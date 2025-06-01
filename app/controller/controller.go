package controller

import (
	appInterfaces "claude-squad/app/interfaces"
	instanceInterfaces "claude-squad/instance/interfaces"
	"claude-squad/instance/orchestrator"
	"claude-squad/instance/task"
	"claude-squad/keys"
	"claude-squad/log"
	"claude-squad/ui"
	"claude-squad/ui/overlay"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TUI states - these should match the ones in model
const (
	TUIStateDefault = iota
	TUIStatePrompt
	TUIStateHelp
	TUIStateNew
)

// Help types - these should match the ones in model
const (
	HelpTypeGeneral = iota
	HelpTypeInstanceStart
	HelpTypeInstanceCheckout
	HelpTypeInstanceAttach
)

// Message types
type hideErrMsg struct{}
type previewTickMsg struct{}
type keyupMsg struct{}
type tickUpdateMetadataMessage struct{}

// Global instance limit
const GlobalInstanceLimit = 10

// Commands
var tickUpdateMetadataCmd = func() tea.Msg {
	time.Sleep(1 * time.Second)
	return tickUpdateMetadataMessage{}
}

type orchestratorState int

const (
	// orchestratorStateDefault is the default state for orchestrator
	orchestratorStateDefault orchestratorState = iota
	// orchestratorStatePrompt is the state when the user is entering a prompt for orchestrator
	orchestratorStatePrompt
	// orchestratorStatePlan is the state when the orchestrator plan is being displayed
	orchestratorStatePlan
)

// Controller manages instances and orchestrators
type Controller struct {
	// newInstanceFinalizer is the finalizer for new instance
	newInstanceFinalizer func()
	// promptAfterName is whether to prompt after name
	promptAfterName bool
	// orchestratorState is the state of the orchestrator
	orchestratorState orchestratorState

	// instances is the list of instances being managed
	instances []instanceInterfaces.Instance

	// UI components
	List             *ui.List
	TabbedWindow     *ui.TabbedWindow
	textInputOverlay *overlay.TextInputOverlay
	textOverlay      *overlay.TextOverlay
}

func NewController(spinner *spinner.Model, autoYes bool) *Controller {
	return &Controller{
		List:         ui.NewList(spinner, autoYes),
		TabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewDiffPane()),
	}
}

// LoadExistingInstances loads instances from storage into the list
func (im *Controller) LoadExistingInstances(storage interface{}) error {
	// Type assert to get the actual storage interface
	if s, ok := storage.(appInterfaces.StorageInterface); ok {
		instances, err := s.LoadInstances()
		if err != nil {
			return err
		}

		for _, instance := range instances {
			finalizer := im.List.AddInstance(instance.(*task.Task))
			finalizer() // Call finalizer immediately since instance is already started
		}
	}

	return nil
}

func (im *Controller) Render(h interface{}) string {
	// Type assert to get the model interface
	model, ok := h.(appInterfaces.ModelInterface)
	if !ok {
		return "Invalid model"
	}
	listWithPadding := lipgloss.NewStyle().PaddingTop(1).Render(im.List.String())
	previewWithPadding := lipgloss.NewStyle().PaddingTop(1).Render(im.TabbedWindow.String())
	listAndPreview := lipgloss.JoinHorizontal(lipgloss.Top, listWithPadding, previewWithPadding)

	mainView := lipgloss.JoinVertical(
		lipgloss.Center,
		listAndPreview,
		model.GetMenu().String(),
		model.GetErrBox().String(),
	)

	if model.GetState() == TUIStatePrompt {
		if im.textInputOverlay == nil {
			log.ErrorLog.Printf("text input overlay is nil")
		}
		return overlay.PlaceOverlay(0, 0, im.textInputOverlay.Render(), mainView, true, true)
	} else if model.GetState() == TUIStateHelp {
		if im.textOverlay == nil {
			log.ErrorLog.Printf("text overlay is nil")
		}
		return overlay.PlaceOverlay(0, 0, im.textOverlay.Render(), mainView, true, true)
	}

	return mainView
}

func (im *Controller) Update(h interface{}, msg tea.Msg) (tea.Model, tea.Cmd) {
	// Type assert to get the model interface
	m, ok := h.(appInterfaces.ModelInterface)
	if !ok {
		return h.(tea.Model), nil
	}
	switch msg := msg.(type) {
	case hideErrMsg:
		m.GetErrBox().Clear()
	case previewTickMsg:
		cmd := im.instanceChanged(m)
		return m, tea.Batch(
			cmd,
			func() tea.Msg {
				time.Sleep(100 * time.Millisecond)
				return previewTickMsg{}
			},
		)
	case keyupMsg:
		m.GetMenu().ClearKeydown()
		return m, nil
	case tickUpdateMetadataMessage:
		return m, im.handleMetadataUpdate(m)
	case tea.MouseMsg:
		return im.handleMouseEvent(m, msg)
	case tea.KeyMsg:
		return im.handleKeyEvent(m, msg)
	case tea.WindowSizeMsg:
		m.UpdateHandleWindowSizeEvent(msg)
		return m, nil
	case spinner.TickMsg:
		spinner := m.GetSpinner()
		var cmd tea.Cmd
		*spinner, cmd = spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (im *Controller) handleMetadataUpdate(h appInterfaces.ModelInterface) tea.Cmd {
	for _, instance := range im.List.GetInstances() {
		if !instance.Started() || instance.Paused() {
			continue
		}
		updated, prompt := instance.HasUpdated()
		if updated {
			instance.SetStatus(task.Running)
		} else {
			if prompt {
				instance.TapEnter()
			} else {
				instance.SetStatus(task.Ready)
			}
		}
		if err := instance.UpdateDiffStats(); err != nil {
			log.WarningLog.Printf("could not update diff stats: %v", err)
		}
	}
	return tickUpdateMetadataCmd
}

func (im *Controller) handleMouseEvent(h appInterfaces.ModelInterface, msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Handle mouse wheel scrolling in the diff view
	if im.TabbedWindow.IsInDiffTab() {
		if msg.Action == tea.MouseActionPress {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				im.TabbedWindow.ScrollUp()
				return h, im.instanceChanged(h)
			case tea.MouseButtonWheelDown:
				im.TabbedWindow.ScrollDown()
				return h, im.instanceChanged(h)
			default:
				break
			}
		}
	}
	return h, nil
}

func (im *Controller) handleKeyEvent(h appInterfaces.ModelInterface, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle prompt state key events
	if h.GetState() == TUIStatePrompt && im.textInputOverlay != nil {
		return im.handlePromptKeyEvent(h, msg)
	}

	// Handle other key events
	return im.handleKeyPress(h, msg)
}

func (im *Controller) handlePromptKeyEvent(h appInterfaces.ModelInterface, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	shouldClose := im.textInputOverlay.HandleKeyPress(msg)
	if !shouldClose {
		return h, nil
	}

	if im.textInputOverlay.IsSubmitted() {
		if im.orchestratorState == orchestratorStatePrompt {
			// Handle orchestrator prompt - generate plan first
			prompt := im.textInputOverlay.GetValue()
			im.textInputOverlay = nil
			im.orchestratorState = orchestratorStatePrompt
			return im.generateOrchestratorPlan(h, prompt)
		} else {
			// Handle regular prompt for selected instance
			selected := im.List.GetSelectedInstance()
			if selected != nil {
				if err := selected.SendPrompt(im.textInputOverlay.GetValue()); err != nil {
					return h, h.HandleError(err)
				}
			}
		}
	}

	// Close the overlay and reset state
	im.textInputOverlay = nil
	// im.isOrchestratorPrompt = false
	h.SetState(TUIStateDefault)
	return h, tea.Sequence(
		tea.WindowSize(),
		func() tea.Msg {
			h.GetMenu().SetState(ui.StateDefault)
			h.ShowHelpScreen(HelpTypeInstanceStart, nil, nil, nil)
			return nil
		},
	)
}

func (im *Controller) handleKeyPress(h appInterfaces.ModelInterface, msg tea.KeyMsg) (mod tea.Model, cmd tea.Cmd) {
	cmd, returnEarly := h.HandleMenuHighlighting(msg)
	if returnEarly {
		return h, cmd
	}

	if h.GetState() == TUIStateHelp {
		// // Check if we're showing an orchestrator plan for approval
		// if im.orchestratorPlan != "" && im.textOverlay != nil {
		// 	return im.handleOrchestratorPlanKeyPress(h, msg)
		// }
		return h.HandleHelpState(msg, im.textOverlay)
	}

	if h.GetState() == TUIStateNew {
		return im.handleNewInstanceState(h, msg)
	}

	// Handle quit commands first
	if msg.String() == "ctrl+c" || msg.String() == "q" {
		return h.HandleQuit()
	}

	name, ok := keys.InstanceModeKeyMap[msg.String()]
	if !ok {
		return h, nil
	}

	switch name {
	case keys.KeyHelp:
		return h, tea.Cmd(func() tea.Msg {
			h.ShowHelpScreen(HelpTypeGeneral, nil, nil, nil)
			return nil
		})
	case keys.KeyPrompt, keys.KeyNew:
		return im.handleNewInstance(h, name == keys.KeyPrompt)
	case keys.KeyOrchestrator:
		return im.handleNewOrchestrator(h)
	case keys.KeyUp:
		im.List.Up()
		return h, im.instanceChanged(h)
	case keys.KeyDown:
		im.List.Down()
		return h, im.instanceChanged(h)
	case keys.KeyShiftUp:
		if im.TabbedWindow.IsInDiffTab() {
			im.TabbedWindow.ScrollUp()
		}
		return h, im.instanceChanged(h)
	case keys.KeyShiftDown:
		if im.TabbedWindow.IsInDiffTab() {
			im.TabbedWindow.ScrollDown()
		}
		return h, im.instanceChanged(h)
	case keys.KeyTab:
		im.TabbedWindow.Toggle()
		h.GetMenu().SetInDiffTab(im.TabbedWindow.IsInDiffTab())
		return h, im.instanceChanged(h)
	case keys.KeyKill:
		return im.handleKillInstance(h)
	case keys.KeySubmit:
		return im.handleSubmitChanges(h)
	case keys.KeyCheckout:
		return im.handleCheckoutInstance(h)
	case keys.KeyResume:
		return im.handleResumeInstance(h)
	case keys.KeyEnter:
		return im.handleAttachInstance(h)
	default:
		return h, nil
	}
}

func (im *Controller) handleNewInstanceState(h appInterfaces.ModelInterface, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle quit commands first. Don't handle q because the user might want to type that.
	if msg.String() == "ctrl+c" {
		h.SetState(TUIStateDefault)
		im.promptAfterName = false
		im.List.Kill()
		return h, tea.Sequence(
			tea.WindowSize(),
			func() tea.Msg {
				h.GetMenu().SetState(ui.StateDefault)
				return nil
			},
		)
	}

	instance := im.List.GetInstances()[im.List.NumInstances()-1]
	switch msg.Type {
	case tea.KeyEnter:
		return im.finalizeNewInstance(h, instance)
	case tea.KeyRunes:
		if len(instance.Title) >= 32 {
			return h, h.HandleError(fmt.Errorf("title cannot be longer than 32 characters"))
		}
		if err := instance.SetTitle(instance.Title + string(msg.Runes)); err != nil {
			return h, h.HandleError(err)
		}
	case tea.KeyBackspace:
		if len(instance.Title) == 0 {
			return h, nil
		}
		if err := instance.SetTitle(instance.Title[:len(instance.Title)-1]); err != nil {
			return h, h.HandleError(err)
		}
	case tea.KeySpace:
		if err := instance.SetTitle(instance.Title + " "); err != nil {
			return h, h.HandleError(err)
		}
	case tea.KeyEsc:
		im.List.Kill()
		h.SetState(TUIStateDefault)
		im.instanceChanged(h)

		return h, tea.Sequence(
			tea.WindowSize(),
			func() tea.Msg {
				h.GetMenu().SetState(ui.StateDefault)
				return nil
			},
		)
	default:
	}
	return h, nil
}

func (im *Controller) finalizeNewInstance(h appInterfaces.ModelInterface, instance *task.Task) (tea.Model, tea.Cmd) {
	if len(instance.Title) == 0 {
		return h, h.HandleError(fmt.Errorf("title cannot be empty"))
	}

	if err := instance.Start(true); err != nil {
		im.List.Kill()
		h.SetState(TUIStateDefault)
		return h, h.HandleError(err)
	}
	// Save after adding new instance
	if err := h.GetStorage().SaveInstances(im.instances); err != nil {
		return h, h.HandleError(err)
	}
	// Instance added successfully, call the finalizer.
	im.newInstanceFinalizer()
	if h.GetAutoYes() {
		instance.AutoYes = true
	}

	im.newInstanceFinalizer()
	h.SetState(TUIStateDefault)
	if im.promptAfterName {
		h.SetState(TUIStatePrompt)
		h.GetMenu().SetState(ui.StatePrompt)
		// Initialize the text input overlay
		im.textInputOverlay = overlay.NewTextInputOverlay("Enter prompt", "")
		// Set proper size for the overlay
		im.textInputOverlay.SetSize(80, 20) // Match orchestrator overlay size
		im.promptAfterName = false
	} else {
		h.GetMenu().SetState(ui.StateDefault)
		h.ShowHelpScreen(HelpTypeInstanceStart, instance, nil, nil)
	}

	return h, tea.Batch(tea.WindowSize(), im.instanceChanged(h))
}

func (im *Controller) handleNewInstance(h appInterfaces.ModelInterface, promptAfter bool) (tea.Model, tea.Cmd) {
	if im.List.NumInstances() >= GlobalInstanceLimit {
		return h, h.HandleError(
			fmt.Errorf("you can't create more than %d instances", GlobalInstanceLimit))
	}
	instance, err := task.NewTask(task.TaskOptions{
		Title:   "",
		Path:    ".",
		Program: h.GetProgram(),
	})
	if err != nil {
		return h, h.HandleError(err)
	}

	im.newInstanceFinalizer = im.List.AddInstance(instance)
	im.List.SetSelectedInstance(im.List.NumInstances() - 1)
	h.SetState(TUIStateNew)
	h.GetMenu().SetState(ui.StateNewInstance)
	im.promptAfterName = promptAfter

	return h, nil
}

func (im *Controller) handleNewOrchestrator(h appInterfaces.ModelInterface) (tea.Model, tea.Cmd) {
	// Create an orchestrator instance - similar to KeyPrompt but for orchestration
	h.SetState(TUIStatePrompt)
	h.GetMenu().SetState(ui.StatePrompt)
	// Initialize the text input overlay for orchestrator goal
	im.textInputOverlay = overlay.NewTextInputOverlay("Enter orchestration goal", "")
	// Set proper size for the overlay (should match other overlays)
	im.textInputOverlay.SetSize(80, 20)
	im.promptAfterName = false
	// im.isOrchestratorPrompt = true
	return h, nil
}

func (im *Controller) handleKillInstance(h appInterfaces.ModelInterface) (tea.Model, tea.Cmd) {
	selected := im.List.GetSelectedInstance()
	if selected == nil {
		return h, nil
	}

	worktree, err := selected.GetGitWorktree()
	if err != nil {
		return h, h.HandleError(err)
	}

	checkedOut, err := worktree.IsBranchCheckedOut()
	if err != nil {
		return h, h.HandleError(err)
	}

	if checkedOut {
		return h, h.HandleError(fmt.Errorf("instance %s is currently checked out", selected.Title))
	}

	// Delete from storage first
	if err := h.GetStorage().DeleteInstance(selected.Title); err != nil {
		return h, h.HandleError(err)
	}

	// Then kill the instance
	im.List.Kill()
	return h, im.instanceChanged(h)
}

func (im *Controller) handleSubmitChanges(h appInterfaces.ModelInterface) (tea.Model, tea.Cmd) {
	selected := im.List.GetSelectedInstance()
	if selected == nil {
		return h, nil
	}

	// Default commit message with timestamp
	commitMsg := fmt.Sprintf("[claudesquad] update from '%s' on %s", selected.Title, time.Now().Format(time.RFC822))
	worktree, err := selected.GetGitWorktree()
	if err != nil {
		return h, h.HandleError(err)
	}
	if err = worktree.PushChanges(commitMsg, true); err != nil {
		return h, h.HandleError(err)
	}

	return h, nil
}

func (im *Controller) handleCheckoutInstance(h appInterfaces.ModelInterface) (tea.Model, tea.Cmd) {
	selected := im.List.GetSelectedInstance()
	if selected == nil {
		return h, nil
	}

	// Show help screen before pausing
	h.ShowHelpScreen(HelpTypeInstanceCheckout, selected, nil, func() {
		if err := selected.Pause(); err != nil {
			h.HandleError(err)
		}
		im.instanceChanged(h)
	})
	return h, nil
}

func (im *Controller) handleResumeInstance(h appInterfaces.ModelInterface) (tea.Model, tea.Cmd) {
	selected := im.List.GetSelectedInstance()
	if selected == nil {
		return h, nil
	}
	if err := selected.Resume(); err != nil {
		return h, h.HandleError(err)
	}
	return h, tea.WindowSize()
}

func (im *Controller) handleAttachInstance(h appInterfaces.ModelInterface) (tea.Model, tea.Cmd) {
	if im.List.NumInstances() == 0 {
		return h, nil
	}
	selected := im.List.GetSelectedInstance()
	if selected == nil || selected.Paused() || !selected.TmuxAlive() {
		return h, nil
	}
	// Show help screen before attaching
	h.ShowHelpScreen(HelpTypeInstanceAttach, selected, nil, func() {
		ch, err := im.List.Attach()
		if err != nil {
			h.HandleError(err)
			return
		}
		<-ch
		h.SetState(TUIStateDefault)
	})
	return h, nil
}

func (im *Controller) instanceChanged(h appInterfaces.ModelInterface) tea.Cmd {
	// selected may be nil
	selected := im.List.GetSelectedInstance()

	im.TabbedWindow.UpdateDiff(selected)
	// Update menu with current instance
	h.GetMenu().SetInstance(selected)

	// If there's no selected instance, we don't need to update the preview.
	if err := im.TabbedWindow.UpdatePreview(selected); err != nil {
		return h.HandleError(err)
	}
	return nil
}

// generateOrchestratorPlan generates a plan from the user's prompt and shows it for approval
func (im *Controller) generateOrchestratorPlan(h appInterfaces.ModelInterface, prompt string) (tea.Model, tea.Cmd) {
	return h, func() tea.Msg {
		orch := orchestrator.NewOrchestrator(h.GetProgram(), prompt)
		im.instances = append(im.instances, orch)

		orch.ForumulatePlan()

		return tea.WindowSize()
	}
}

// handleOrchestratorPlanApproval handles when user approves the orchestrator plan
func (im *Controller) handleOrchestratorPlanApproval(h appInterfaces.ModelInterface) (tea.Model, tea.Cmd) {
	// For testing purposes, just show a success message
	return h, func() tea.Msg {
		// Show success message
		successMessage := "Plan Approved\n\nOrchestration plan has been approved. For testing purposes, no workers will be created."
		im.textOverlay = overlay.NewTextOverlay(successMessage)

		h.SetState(TUIStateHelp) // Show the text overlay
		return tea.WindowSize()
	}
}

// handleOrchestratorPlanKeyPress handles key presses when showing orchestrator plan for approval
func (im *Controller) handleOrchestratorPlanKeyPress(h appInterfaces.ModelInterface, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// User approved the plan
		// im.orchestratorPlan = ""
		return im.handleOrchestratorPlanApproval(h)
	case "esc", "q":
		// User cancelled the plan
		// im.orchestratorPlan = ""
		im.textOverlay = nil
		h.SetState(TUIStateDefault)
		return h, tea.Sequence(
			tea.WindowSize(),
			func() tea.Msg {
				h.GetMenu().SetState(ui.StateDefault)
				return nil
			},
		)
	default:
		// Any other key shows help about the plan approval
		return h, nil
	}
}

func (im *Controller) HandleQuit(h interface{}) {
	if m, ok := h.(appInterfaces.ModelInterface); ok {
		if err := m.GetStorage().SaveInstances(im.instances); err != nil {
			m.HandleError(err)
		}
	}
}

// GetList returns the List component
func (im *Controller) GetList() *ui.List {
	return im.List
}

// GetTabbedWindow returns the TabbedWindow component
func (im *Controller) GetTabbedWindow() *ui.TabbedWindow {
	return im.TabbedWindow
}
