package model

import (
	"claude-squad/instance"
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

// TUI states
const (
	TUIStateDefault = iota
	TUIStatePrompt
	TUIStateHelp
	TUIStateNew
)

// Help types
const (
	HelpTypeGeneral = iota
	HelpTypeInstanceStart
	HelpTypeInstanceCheckout
	HelpTypeInstanceAttach
)

// Global instance limit
const GlobalInstanceLimit = 10

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
	list             *ui.List
	tabbedWindow     *ui.TabbedWindow
	textInputOverlay *overlay.TextInputOverlay
	textOverlay      *overlay.TextOverlay
}

func NewController(spinner *spinner.Model, autoYes bool) *Controller {
	return &Controller{
		list:         ui.NewList(spinner, autoYes),
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewDiffPane()),
	}
}

// LoadExistingInstances loads instances from storage into the list
func (c *Controller) LoadExistingInstances(storage *instance.Storage[instanceInterfaces.Instance]) error {
	instances, err := storage.LoadInstances()
	if err != nil {
		return err
	}

	for _, instance := range instances {
		finalizer := c.list.AddInstance(instance.(*task.Task))
		finalizer() // Call finalizer immediately since instance is already started
	}

	c.instances = instances

	return nil
}

func (c *Controller) Render(model *Model) string {
	listWithPadding := lipgloss.NewStyle().PaddingTop(1).Render(c.list.String())
	previewWithPadding := lipgloss.NewStyle().PaddingTop(1).Render(c.tabbedWindow.String())
	listAndPreview := lipgloss.JoinHorizontal(lipgloss.Top, listWithPadding, previewWithPadding)

	mainView := lipgloss.JoinVertical(
		lipgloss.Center,
		listAndPreview,
		model.GetMenu().String(),
		model.GetErrBox().String(),
	)

	if model.GetState() == TUIStatePrompt {
		if c.textInputOverlay == nil {
			log.ErrorLog.Printf("text input overlay is nil")
		}
		return overlay.PlaceOverlay(0, 0, c.textInputOverlay.Render(), mainView, true, true)
	} else if model.GetState() == TUIStateHelp {
		if c.textOverlay == nil {
			log.ErrorLog.Printf("text overlay is nil")
		}
		return overlay.PlaceOverlay(0, 0, c.textOverlay.Render(), mainView, true, true)
	}

	return mainView
}

func (c *Controller) Update(model *Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case hideErrMsg:
		model.GetErrBox().Clear()
	case previewTickMsg:
		cmd := c.instanceChanged(model)
		return model, tea.Batch(
			cmd,
			func() tea.Msg {
				time.Sleep(100 * time.Millisecond)
				return previewTickMsg{}
			},
		)
	case keyupMsg:
		model.GetMenu().ClearKeydown()
		return model, nil
	case tickUpdateMetadataMessage:
		return model, c.handleMetadataUpdate()
	case tea.MouseMsg:
		return c.handleMouseEvent(model, msg)
	case tea.KeyMsg:
		return c.handleKeyEvent(model, msg)
	case tea.WindowSizeMsg:
		model.UpdateHandleWindowSizeEvent(msg)
		return model, nil
	case spinner.TickMsg:
		spinner := model.GetSpinner()
		var cmd tea.Cmd
		*spinner, cmd = spinner.Update(msg)
		return model, cmd
	}
	return model, nil
}

func (c *Controller) handleMetadataUpdate() tea.Cmd {
	for _, instance := range c.list.GetInstances() {
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

func (c *Controller) handleMouseEvent(model *Model, msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Handle mouse wheel scrolling in the diff view
	if c.tabbedWindow.IsInDiffTab() {
		if msg.Action == tea.MouseActionPress {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				c.tabbedWindow.ScrollUp()
				return model, c.instanceChanged(model)
			case tea.MouseButtonWheelDown:
				c.tabbedWindow.ScrollDown()
				return model, c.instanceChanged(model)
			default:
				break
			}
		}
	}
	return model, nil
}

func (c *Controller) handleKeyEvent(model *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle prompt state key events
	if model.GetState() == TUIStatePrompt && c.textInputOverlay != nil {
		return c.handlePromptKeyEvent(model, msg)
	}

	// Handle other key events
	return c.handleKeyPress(model, msg)
}

func (c *Controller) handlePromptKeyEvent(model *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	shouldClose := c.textInputOverlay.HandleKeyPress(msg)
	if !shouldClose {
		return model, nil
	}

	if c.textInputOverlay.IsSubmitted() {
		if c.orchestratorState == orchestratorStatePrompt {
			// Handle orchestrator prompt - generate plan first
			prompt := c.textInputOverlay.GetValue()
			c.textInputOverlay = nil
			c.orchestratorState = orchestratorStatePrompt
			return c.generateOrchestratorPlan(model, prompt)
		} else {
			// Handle regular prompt for selected instance
			selected := c.list.GetSelectedInstance()
			if selected != nil {
				if err := selected.SendPrompt(c.textInputOverlay.GetValue()); err != nil {
					return model, model.HandleError(err)
				}
			}
		}
	}

	// Close the overlay and reset state
	c.textInputOverlay = nil
	// c.isOrchestratorPrompt = false
	model.SetState(TUIStateDefault)
	return model, tea.Sequence(
		tea.WindowSize(),
		func() tea.Msg {
			model.GetMenu().SetState(ui.StateDefault)
			model.ShowHelpScreen(HelpTypeInstanceStart, nil, nil, nil)
			return nil
		},
	)
}

func (c *Controller) handleKeyPress(model *Model, msg tea.KeyMsg) (mod tea.Model, cmd tea.Cmd) {
	cmd, returnEarly := model.HandleMenuHighlighting(msg)
	if returnEarly {
		return model, cmd
	}

	if model.GetState() == TUIStateHelp {
		// // Check if we're showing an orchestrator plan for approval
		// if c.orchestratorPlan != "" && c.textOverlay != nil {
		// 	return c.handleOrchestratorPlanKeyPress(model, msg)
		// }
		return model.HandleHelpState(msg, c.textOverlay)
	}

	if model.GetState() == TUIStateNew {
		return c.handleNewInstanceState(model, msg)
	}

	// Handle quit commands first
	if msg.String() == "ctrl+c" || msg.String() == "q" {
		return model.HandleQuit()
	}

	name, ok := keys.InstanceModeKeyMap[msg.String()]
	if !ok {
		return model, nil
	}

	switch name {
	case keys.KeyHelp:
		return model, tea.Cmd(func() tea.Msg {
			model.ShowHelpScreen(HelpTypeGeneral, nil, nil, nil)
			return nil
		})
	case keys.KeyPrompt, keys.KeyNew:
		return c.handleNewInstance(model, name == keys.KeyPrompt)
	case keys.KeyOrchestrator:
		return c.handleNewOrchestrator(model)
	case keys.KeyUp:
		c.list.Up()
		return model, c.instanceChanged(model)
	case keys.KeyDown:
		c.list.Down()
		return model, c.instanceChanged(model)
	case keys.KeyShiftUp:
		if c.tabbedWindow.IsInDiffTab() {
			c.tabbedWindow.ScrollUp()
		}
		return model, c.instanceChanged(model)
	case keys.KeyShiftDown:
		if c.tabbedWindow.IsInDiffTab() {
			c.tabbedWindow.ScrollDown()
		}
		return model, c.instanceChanged(model)
	case keys.KeyTab:
		c.tabbedWindow.Toggle()
		model.GetMenu().SetInDiffTab(c.tabbedWindow.IsInDiffTab())
		return model, c.instanceChanged(model)
	case keys.KeyKill:
		return c.handleKillInstance(model)
	case keys.KeySubmit:
		return c.handleSubmitChanges(model)
	case keys.KeyCheckout:
		return c.handleCheckoutInstance(model)
	case keys.KeyResume:
		return c.handleResumeInstance(model)
	case keys.KeyEnter:
		return c.handleAttachInstance(model)
	default:
		return model, nil
	}
}

func (c *Controller) handleNewInstanceState(model *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle quit commands first. Don't handle q because the user might want to type that.
	if msg.String() == "ctrl+c" {
		model.SetState(TUIStateDefault)
		c.promptAfterName = false
		c.list.Kill()
		return model, tea.Sequence(
			tea.WindowSize(),
			func() tea.Msg {
				model.GetMenu().SetState(ui.StateDefault)
				return nil
			},
		)
	}

	instance := c.list.GetInstances()[c.list.NumInstances()-1]
	switch msg.Type {
	case tea.KeyEnter:
		return c.finalizeNewInstance(model, instance)
	case tea.KeyRunes:
		if len(instance.Title) >= 32 {
			return model, model.HandleError(fmt.Errorf("title cannot be longer than 32 characters"))
		}
		if err := instance.SetTitle(instance.Title + string(msg.Runes)); err != nil {
			return model, model.HandleError(err)
		}
	case tea.KeyBackspace:
		if len(instance.Title) == 0 {
			return model, nil
		}
		if err := instance.SetTitle(instance.Title[:len(instance.Title)-1]); err != nil {
			return model, model.HandleError(err)
		}
	case tea.KeySpace:
		if err := instance.SetTitle(instance.Title + " "); err != nil {
			return model, model.HandleError(err)
		}
	case tea.KeyEsc:
		c.list.Kill()
		model.SetState(TUIStateDefault)
		c.instanceChanged(model)

		return model, tea.Sequence(
			tea.WindowSize(),
			func() tea.Msg {
				model.GetMenu().SetState(ui.StateDefault)
				return nil
			},
		)
	default:
	}
	return model, nil
}

func (c *Controller) finalizeNewInstance(model *Model, instance *task.Task) (tea.Model, tea.Cmd) {
	if len(instance.Title) == 0 {
		return model, model.HandleError(fmt.Errorf("title cannot be empty"))
	}

	if err := instance.Start(true); err != nil {
		c.list.Kill()
		model.SetState(TUIStateDefault)
		return model, model.HandleError(err)
	}

	c.instances = append(c.instances, instance)
	// Save after adding new instance
	if err := model.GetStorage().SaveInstances(c.instances); err != nil {
		return model, model.HandleError(err)
	}
	// Instance added successfully, call the finalizer.
	c.newInstanceFinalizer()
	if model.GetAutoYes() {
		instance.AutoYes = true
	}

	c.newInstanceFinalizer()
	model.SetState(TUIStateDefault)
	if c.promptAfterName {
		model.SetState(TUIStatePrompt)
		model.GetMenu().SetState(ui.StatePrompt)
		// Initialize the text input overlay
		c.textInputOverlay = overlay.NewTextInputOverlay("Enter prompt", "")
		// Set proper size for the overlay
		c.textInputOverlay.SetSize(80, 20) // Match orchestrator overlay size
		c.promptAfterName = false
	} else {
		model.GetMenu().SetState(ui.StateDefault)
		model.ShowHelpScreen(HelpTypeInstanceStart, instance, nil, nil)
	}

	return model, tea.Batch(tea.WindowSize(), c.instanceChanged(model))
}

func (c *Controller) handleNewInstance(model *Model, promptAfter bool) (tea.Model, tea.Cmd) {
	if c.list.NumInstances() >= GlobalInstanceLimit {
		return model, model.HandleError(
			fmt.Errorf("you can't create more than %d instances", GlobalInstanceLimit))
	}
	instance, err := task.NewTask(task.TaskOptions{
		Title:   "",
		Path:    ".",
		Program: model.GetProgram(),
	})
	if err != nil {
		return model, model.HandleError(err)
	}

	c.newInstanceFinalizer = c.list.AddInstance(instance)
	c.list.SetSelectedInstance(c.list.NumInstances() - 1)
	model.SetState(TUIStateNew)
	model.GetMenu().SetState(ui.StateNewInstance)
	c.promptAfterName = promptAfter

	return model, nil
}

func (c *Controller) handleNewOrchestrator(model *Model) (tea.Model, tea.Cmd) {
	// Create an orchestrator instance - similar to KeyPrompt but for orchestration
	model.SetState(TUIStatePrompt)
	model.GetMenu().SetState(ui.StatePrompt)
	// Initialize the text input overlay for orchestrator goal
	c.textInputOverlay = overlay.NewTextInputOverlay("Enter orchestration goal", "")
	// Set proper size for the overlay (should match other overlays)
	c.textInputOverlay.SetSize(80, 20)
	c.promptAfterName = false
	// c.isOrchestratorPrompt = true
	return model, nil
}

func (c *Controller) handleKillInstance(model *Model) (tea.Model, tea.Cmd) {
	selected := c.list.GetSelectedInstance()
	if selected == nil {
		return model, nil
	}

	worktree, err := selected.GetGitWorktree()
	if err != nil {
		return model, model.HandleError(err)
	}

	checkedOut, err := worktree.IsBranchCheckedOut()
	if err != nil {
		return model, model.HandleError(err)
	}

	if checkedOut {
		return model, model.HandleError(fmt.Errorf("instance %s is currently checked out", selected.Title))
	}

	// Delete from storage first
	if err := model.GetStorage().DeleteInstance(selected.Title); err != nil {
		return model, model.HandleError(err)
	}

	// Then kill the instance
	c.list.Kill()
	return model, c.instanceChanged(model)
}

func (c *Controller) handleSubmitChanges(model *Model) (tea.Model, tea.Cmd) {
	selected := c.list.GetSelectedInstance()
	if selected == nil {
		return model, nil
	}

	// Default commit message with timestamp
	commitMsg := fmt.Sprintf("[claudesquad] update from '%s' on %s", selected.Title, time.Now().Format(time.RFC822))
	worktree, err := selected.GetGitWorktree()
	if err != nil {
		return model, model.HandleError(err)
	}
	if err = worktree.PushChanges(commitMsg, true); err != nil {
		return model, model.HandleError(err)
	}

	return model, nil
}

func (c *Controller) handleCheckoutInstance(model *Model) (tea.Model, tea.Cmd) {
	selected := c.list.GetSelectedInstance()
	if selected == nil {
		return model, nil
	}

	// Show help screen before pausing
	model.ShowHelpScreen(HelpTypeInstanceCheckout, selected, nil, func() {
		if err := selected.Pause(); err != nil {
			model.HandleError(err)
		}
		c.instanceChanged(model)
	})
	return model, nil
}

func (c *Controller) handleResumeInstance(model *Model) (tea.Model, tea.Cmd) {
	selected := c.list.GetSelectedInstance()
	if selected == nil {
		return model, nil
	}
	if err := selected.Resume(); err != nil {
		return model, model.HandleError(err)
	}
	return model, tea.WindowSize()
}

func (c *Controller) handleAttachInstance(model *Model) (tea.Model, tea.Cmd) {
	if c.list.NumInstances() == 0 {
		return model, nil
	}
	selected := c.list.GetSelectedInstance()
	if selected == nil || selected.Paused() || !selected.TmuxAlive() {
		return model, nil
	}
	// Show help screen before attaching
	model.ShowHelpScreen(HelpTypeInstanceAttach, selected, nil, func() {
		ch, err := c.list.Attach()
		if err != nil {
			model.HandleError(err)
			return
		}
		<-ch
		model.SetState(TUIStateDefault)
	})
	return model, nil
}

func (c *Controller) instanceChanged(model *Model) tea.Cmd {
	// selected may be nil
	selected := c.list.GetSelectedInstance()

	c.tabbedWindow.UpdateDiff(selected)
	// Update menu with current instance
	model.GetMenu().SetInstance(selected)

	// If there's no selected instance, we don't need to update the preview.
	if err := c.tabbedWindow.UpdatePreview(selected); err != nil {
		return model.HandleError(err)
	}
	return nil
}

// generateOrchestratorPlan generates a plan from the user's prompt and shows it for approval
func (c *Controller) generateOrchestratorPlan(model *Model, prompt string) (tea.Model, tea.Cmd) {
	return model, func() tea.Msg {
		orch := orchestrator.NewOrchestrator(model.GetProgram(), prompt)
		c.instances = append(c.instances, orch)

		orch.ForumulatePlan()

		return tea.WindowSize()
	}
}

// handleOrchestratorPlanApproval handles when user approves the orchestrator plan
func (c *Controller) handleOrchestratorPlanApproval(model *Model) (tea.Model, tea.Cmd) {
	// For testing purposes, just show a success message
	return model, func() tea.Msg {
		// Show success message
		successMessage := "Plan Approved\n\nOrchestration plan has been approved. For testing purposes, no workers will be created."
		c.textOverlay = overlay.NewTextOverlay(successMessage)

		model.SetState(TUIStateHelp) // Show the text overlay
		return tea.WindowSize()
	}
}

// handleOrchestratorPlanKeyPress handles key presses when showing orchestrator plan for approval
func (c *Controller) handleOrchestratorPlanKeyPress(model *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// User approved the plan
		// c.orchestratorPlan = ""
		return c.handleOrchestratorPlanApproval(model)
	case "esc", "q":
		// User cancelled the plan
		// c.orchestratorPlan = ""
		c.textOverlay = nil
		model.SetState(TUIStateDefault)
		return model, tea.Sequence(
			tea.WindowSize(),
			func() tea.Msg {
				model.GetMenu().SetState(ui.StateDefault)
				return nil
			},
		)
	default:
		// Any other key shows help about the plan approval
		return model, nil
	}
}

func (c *Controller) HandleQuit(model *Model) {
	if err := model.GetStorage().SaveInstances(c.instances); err != nil {
		model.HandleError(err)
	}
}

// GetList returns the list component
func (c *Controller) GetList() *ui.List {
	return c.list
}

// GetTabbedWindow returns the tabbedWindow component
func (c *Controller) GetTabbedWindow() *ui.TabbedWindow {
	return c.tabbedWindow
}
