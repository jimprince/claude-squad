package app

import (
	"claude-squad/keys"
	"claude-squad/log"
	"claude-squad/orchestrator"
	"claude-squad/session"
	"claude-squad/session/git"
	"claude-squad/ui"
	"claude-squad/ui/overlay"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// controller manages instances and orchestrators
type controller struct {
	// newInstanceFinalizer is the finalizer for new instance
	newInstanceFinalizer func()
	// promptAfterName is whether to prompt after name
	promptAfterName bool
	// isOrchestratorPrompt is whether the current prompt is for an orchestrator
	isOrchestratorPrompt bool

	// # UI components

	// list is the list of instances and orchestrators
	list *ui.List
	// tabbedWindow is the tabbed window for preview and diff
	tabbedWindow *ui.TabbedWindow
	// textInputOverlay is the text input overlay
	textInputOverlay *overlay.TextInputOverlay
	// textOverlay is the text overlay
	textOverlay *overlay.TextOverlay
}

func newController(spinner *spinner.Model, autoYes bool) *controller {
	im := &controller{
		list:         ui.NewList(spinner, autoYes),
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewDiffPane()),
	}
	return im
}

// LoadExistingInstances loads instances from storage into the list
func (im *controller) LoadExistingInstances(h *home) error {
	instances, err := h.storage.LoadInstances()
	if err != nil {
		return err
	}

	for _, instance := range instances {
		finalizer := im.list.AddInstance(instance)
		finalizer() // Call finalizer immediately since instance is already started
	}

	return nil
}

func (im *controller) Render(h *home) string {
	listWithPadding := lipgloss.NewStyle().PaddingTop(1).Render(im.list.String())
	previewWithPadding := lipgloss.NewStyle().PaddingTop(1).Render(im.tabbedWindow.String())
	listAndPreview := lipgloss.JoinHorizontal(lipgloss.Top, listWithPadding, previewWithPadding)

	mainView := lipgloss.JoinVertical(
		lipgloss.Center,
		listAndPreview,
		h.menu.String(),
		h.errBox.String(),
	)

	if h.state == tuiStatePrompt {
		if im.textInputOverlay == nil {
			log.ErrorLog.Printf("text input overlay is nil")
		}
		return overlay.PlaceOverlay(0, 0, im.textInputOverlay.Render(), mainView, true, true)
	} else if h.state == tuiStateHelp {
		if im.textOverlay == nil {
			log.ErrorLog.Printf("text overlay is nil")
		}
		return overlay.PlaceOverlay(0, 0, im.textOverlay.Render(), mainView, true, true)
	}

	return mainView
}

func (im *controller) Update(h *home, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case hideErrMsg:
		h.errBox.Clear()
	case previewTickMsg:
		cmd := im.instanceChanged(h)
		return h, tea.Batch(
			cmd,
			func() tea.Msg {
				time.Sleep(100 * time.Millisecond)
				return previewTickMsg{}
			},
		)
	case keyupMsg:
		h.menu.ClearKeydown()
		return h, nil
	case tickUpdateMetadataMessage:
		for _, instance := range im.list.GetInstances() {
			if !instance.Started() || instance.Paused() {
				continue
			}
			updated, prompt := instance.HasUpdated()
			if updated {
				instance.SetStatus(session.Running)
			} else {
				if prompt {
					instance.TapEnter()
				} else {
					instance.SetStatus(session.Ready)
				}
			}
			if err := instance.UpdateDiffStats(); err != nil {
				log.WarningLog.Printf("could not update diff stats: %v", err)
			}
		}
		return h, tickUpdateMetadataCmd
	case tea.MouseMsg:
		// Handle mouse wheel scrolling in the diff view
		if im.tabbedWindow.IsInDiffTab() {
			if msg.Action == tea.MouseActionPress {
				switch msg.Button {
				case tea.MouseButtonWheelUp:
					im.tabbedWindow.ScrollUp()
					return h, im.instanceChanged(h)
				case tea.MouseButtonWheelDown:
					im.tabbedWindow.ScrollDown()
					return h, im.instanceChanged(h)
				}
			}
		}
		return h, nil
	case tea.KeyMsg:
		// Handle key events directly here if we're in prompt state and have a text input overlay
		if h.state == tuiStatePrompt && im.textInputOverlay != nil {
			shouldClose := im.textInputOverlay.HandleKeyPress(msg)
			if shouldClose {
				if im.textInputOverlay.IsSubmitted() {
					if im.isOrchestratorPrompt {
						// Handle orchestrator prompt - generate plan first
						prompt := im.textInputOverlay.GetValue()
						im.textInputOverlay = nil
						im.isOrchestratorPrompt = false
						return im.generateOrchestratorPlan(h, prompt)
					} else {
						// Handle regular prompt for selected instance
						selected := im.list.GetSelectedInstance()
						if selected != nil {
							if err := selected.SendPrompt(im.textInputOverlay.GetValue()); err != nil {
								return h, h.handleError(err)
							}
						}
					}
				}

				// Close the overlay and reset state
				im.textInputOverlay = nil
				im.isOrchestratorPrompt = false
				h.state = tuiStateDefault
				return h, tea.Sequence(
					tea.WindowSize(),
					func() tea.Msg {
						h.menu.SetState(ui.StateDefault)
						h.showHelpScreen(helpTypeInstanceStart, nil, nil, nil)
						return nil
					},
				)
			}
			return h, nil
		}
		return im.handleKeyPress(h, msg)
	case tea.WindowSizeMsg:
		h.updateHandleWindowSizeEvent(msg)
		return h, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		h.spinner, cmd = h.spinner.Update(msg)
		return h, cmd
	}
	return h, nil
}

func (im *controller) handleKeyPress(h *home, msg tea.KeyMsg) (mod tea.Model, cmd tea.Cmd) {
	cmd, returnEarly := h.handleMenuHighlighting(msg)
	if returnEarly {
		return h, cmd
	}

	if h.state == tuiStateHelp {
		// Check if we're showing an orchestrator plan for approval
		if im.orchestratorPlan != "" && im.textOverlay != nil {
			return im.handleOrchestratorPlanKeyPress(h, msg)
		}
		return h.handleHelpState(msg, im.textOverlay)
	}

	if h.state == tuiStateNew {
		// Handle quit commands first. Don't handle q because the user might want to type that.
		if msg.String() == "ctrl+c" {
			h.state = tuiStateDefault
			im.promptAfterName = false
			im.list.Kill()
			return h, tea.Sequence(
				tea.WindowSize(),
				func() tea.Msg {
					h.menu.SetState(ui.StateDefault)
					return nil
				},
			)
		}

		instance := im.list.GetInstances()[im.list.NumInstances()-1]
		switch msg.Type {
		case tea.KeyEnter:
			if len(instance.Title) == 0 {
				return h, h.handleError(fmt.Errorf("title cannot be empty"))
			}

			if err := instance.Start(true); err != nil {
				im.list.Kill()
				h.state = tuiStateDefault
				return h, h.handleError(err)
			}
			// Save after adding new instance
			if err := h.storage.SaveInstances(im.list.GetInstances()); err != nil {
				return h, h.handleError(err)
			}
			// Instance added successfully, call the finalizer.
			im.newInstanceFinalizer()
			if h.autoYes {
				instance.AutoYes = true
			}

			im.newInstanceFinalizer()
			h.state = tuiStateDefault
			if im.promptAfterName {
				h.state = tuiStatePrompt
				h.menu.SetState(ui.StatePrompt)
				// Initialize the text input overlay
				im.textInputOverlay = overlay.NewTextInputOverlay("Enter prompt", "")
				// Set proper size for the overlay
				im.textInputOverlay.SetSize(80, 20) // Match orchestrator overlay size
				im.promptAfterName = false
			} else {
				h.menu.SetState(ui.StateDefault)
				h.showHelpScreen(helpTypeInstanceStart, instance, nil, nil)
			}

			return h, tea.Batch(tea.WindowSize(), im.instanceChanged(h))
		case tea.KeyRunes:
			if len(instance.Title) >= 32 {
				return h, h.handleError(fmt.Errorf("title cannot be longer than 32 characters"))
			}
			if err := instance.SetTitle(instance.Title + string(msg.Runes)); err != nil {
				return h, h.handleError(err)
			}
		case tea.KeyBackspace:
			if len(instance.Title) == 0 {
				return h, nil
			}
			if err := instance.SetTitle(instance.Title[:len(instance.Title)-1]); err != nil {
				return h, h.handleError(err)
			}
		case tea.KeySpace:
			if err := instance.SetTitle(instance.Title + " "); err != nil {
				return h, h.handleError(err)
			}
		case tea.KeyEsc:
			im.list.Kill()
			h.state = tuiStateDefault
			im.instanceChanged(h)

			return h, tea.Sequence(
				tea.WindowSize(),
				func() tea.Msg {
					h.menu.SetState(ui.StateDefault)
					return nil
				},
			)
		default:
		}
		return h, nil
	} else if h.state == tuiStatePrompt {
		// This code is now handled directly in the Update method's tea.KeyMsg case
		return h, nil
	}

	// Handle quit commands first
	if msg.String() == "ctrl+c" || msg.String() == "q" {
		return h.handleQuit()
	}

	name, ok := keys.InstanceModeKeyMap[msg.String()]
	if !ok {
		return h, nil
	}

	switch name {
	case keys.KeyHelp:
		return h.showHelpScreen(helpTypeGeneral, nil, nil, nil)
	case keys.KeyPrompt:
		if im.list.NumInstances() >= GlobalInstanceLimit {
			return h, h.handleError(
				fmt.Errorf("you can't create more than %d instances", GlobalInstanceLimit))
		}
		instance, err := session.NewInstance(session.InstanceOptions{
			Title:   "",
			Path:    ".",
			Program: h.program,
		})
		if err != nil {
			return h, h.handleError(err)
		}

		im.newInstanceFinalizer = im.list.AddInstance(instance)
		im.list.SetSelectedInstance(im.list.NumInstances() - 1)
		h.state = tuiStateNew
		h.menu.SetState(ui.StateNewInstance)
		im.promptAfterName = true

		return h, nil
	case keys.KeyNew:
		if im.list.NumInstances() >= GlobalInstanceLimit {
			return h, h.handleError(
				fmt.Errorf("you can't create more than %d instances", GlobalInstanceLimit))
		}
		instance, err := session.NewInstance(session.InstanceOptions{
			Title:   "",
			Path:    ".",
			Program: h.program,
		})
		if err != nil {
			return h, h.handleError(err)
		}

		im.newInstanceFinalizer = im.list.AddInstance(instance)
		im.list.SetSelectedInstance(im.list.NumInstances() - 1)
		h.state = tuiStateNew
		h.menu.SetState(ui.StateNewInstance)

		return h, nil
	case keys.KeyOrchestrator:
		// Create an orchestrator instance - similar to KeyPrompt but for orchestration
		h.state = tuiStatePrompt
		h.menu.SetState(ui.StatePrompt)
		// Initialize the text input overlay for orchestrator goal
		im.textInputOverlay = overlay.NewTextInputOverlay("Enter orchestration goal", "")
		// Set proper size for the overlay (should match other overlays)
		im.textInputOverlay.SetSize(80, 20) // Width=80, Height=20 to match other overlays
		im.promptAfterName = false
		im.isOrchestratorPrompt = true // New flag to indicate this is orchestrator mode
		return h, nil
	case keys.KeyUp:
		im.list.Up()
		return h, im.instanceChanged(h)
	case keys.KeyDown:
		im.list.Down()
		return h, im.instanceChanged(h)
	case keys.KeyShiftUp:
		if im.tabbedWindow.IsInDiffTab() {
			im.tabbedWindow.ScrollUp()
		}
		return h, im.instanceChanged(h)
	case keys.KeyShiftDown:
		if im.tabbedWindow.IsInDiffTab() {
			im.tabbedWindow.ScrollDown()
		}
		return h, im.instanceChanged(h)
	case keys.KeyTab:
		im.tabbedWindow.Toggle()
		h.menu.SetInDiffTab(im.tabbedWindow.IsInDiffTab())
		return h, im.instanceChanged(h)
	case keys.KeyKill:
		selected := im.list.GetSelectedInstance()
		if selected == nil {
			return h, nil
		}

		worktree, err := selected.GetGitWorktree()
		if err != nil {
			return h, h.handleError(err)
		}

		checkedOut, err := worktree.IsBranchCheckedOut()
		if err != nil {
			return h, h.handleError(err)
		}

		if checkedOut {
			return h, h.handleError(fmt.Errorf("instance %s is currently checked out", selected.Title))
		}

		// Delete from storage first
		if err := h.storage.DeleteInstance(selected.Title); err != nil {
			return h, h.handleError(err)
		}

		// Then kill the instance
		im.list.Kill()
		return h, im.instanceChanged(h)
	case keys.KeySubmit:
		selected := im.list.GetSelectedInstance()
		if selected == nil {
			return h, nil
		}

		// Default commit message with timestamp
		commitMsg := fmt.Sprintf("[claudesquad] update from '%s' on %s", selected.Title, time.Now().Format(time.RFC822))
		worktree, err := selected.GetGitWorktree()
		if err != nil {
			return h, h.handleError(err)
		}
		if err = worktree.PushChanges(commitMsg, true); err != nil {
			return h, h.handleError(err)
		}

		return h, nil
	case keys.KeyCheckout:
		selected := im.list.GetSelectedInstance()
		if selected == nil {
			return h, nil
		}

		// Show help screen before pausing
		h.showHelpScreen(helpTypeInstanceCheckout, selected, nil, func() {
			if err := selected.Pause(); err != nil {
				h.handleError(err)
			}
			im.instanceChanged(h)
		})
		return h, nil
	case keys.KeyResume:
		selected := im.list.GetSelectedInstance()
		if selected == nil {
			return h, nil
		}
		if err := selected.Resume(); err != nil {
			return h, h.handleError(err)
		}
		return h, tea.WindowSize()
	case keys.KeyEnter:
		if im.list.NumInstances() == 0 {
			return h, nil
		}
		selected := im.list.GetSelectedInstance()
		if selected == nil || selected.Paused() || !selected.TmuxAlive() {
			return h, nil
		}
		// Show help screen before attaching
		h.showHelpScreen(helpTypeInstanceAttach, selected, nil, func() {
			ch, err := im.list.Attach()
			if err != nil {
				h.handleError(err)
				return
			}
			<-ch
			h.state = tuiStateDefault
		})
		return h, nil
	case keys.KeyMerge:
		selected := im.list.GetSelectedInstance()
		if selected == nil {
			return h, nil
		}

		// If selected instance is a worker, find its orchestrator and all sibling workers
		var workersToMerge []*session.Instance
		var orchestratorName string

		if selected.IsWorker {
			orchestratorName = selected.ParentOrchestrator
			// Find all workers with the same parent orchestrator
			for _, inst := range im.list.GetInstances() {
				if inst.IsWorker && inst.ParentOrchestrator == orchestratorName {
					workersToMerge = append(workersToMerge, inst)
				}
			}
		} else {
			return h, h.handleError(fmt.Errorf("selected instance is not a worker - select a worker instance to merge"))
		}

		if len(workersToMerge) == 0 {
			return h, h.handleError(fmt.Errorf("no workers found to merge"))
		}

		return h, func() tea.Msg {
			// Create a temporary orchestrator to handle merging
			orch := orchestrator.NewOrchestrator("merge-operation", h.autoYes)
			orch.SetProgram(h.program)

			// Collect diffs from all worker instances
			diffs := make(map[string]*git.DiffStats)
			for _, worker := range workersToMerge {
				if err := worker.UpdateDiffStats(); err != nil {
					fmt.Printf("Warning: could not update diff stats for %s: %v\n", worker.Title, err)
					continue
				}
				diffs[worker.Title] = worker.GetDiffStats()
			}

			// Merge the diffs
			merged, err := orch.MergeDiffs(".", diffs)
			if err != nil {
				return h.handleError(fmt.Errorf("failed to merge worker diffs: %w", err))
			}

			fmt.Printf("Successfully merged %d worker instances:\n%s\n", len(workersToMerge), merged)
			return nil
		}
	default:
		return h, nil
	}
}

func (im *controller) instanceChanged(h *home) tea.Cmd {
	// selected may be nil
	selected := im.list.GetSelectedInstance()

	im.tabbedWindow.UpdateDiff(selected)
	// Update menu with current instance
	h.menu.SetInstance(selected)

	// If there's no selected instance, we don't need to update the preview.
	if err := im.tabbedWindow.UpdatePreview(selected); err != nil {
		return h.handleError(err)
	}
	return nil
}

// generateOrchestratorPlan generates a plan from the user's prompt and shows it for approval
func (im *controller) generateOrchestratorPlan(h *home, prompt string) (tea.Model, tea.Cmd) {
	return h, func() tea.Msg {
		orch := orchestrator.NewOrchestrator(prompt, h.autoYes)
		orch.SetProgram(h.program)

		tasks := orch.DividePrompt()
		orch.Plan = tasks

		// Format the plan for display
		var planText strings.Builder
		planText.WriteString(fmt.Sprintf("Orchestration Plan for: %s\n\n", prompt))
		planText.WriteString("The following tasks will be created:\n\n")

		for i, task := range tasks {
			planText.WriteString(fmt.Sprintf("%d. %s\n", i+1, task.Name))
			planText.WriteString(fmt.Sprintf("   %s\n\n", task.Prompt))
		}

		planText.WriteString("Press Enter to approve this plan, or Escape to cancel.")

		// Store the plan and show it as a text overlay for viewing (not input)
		im.orchestratorPlan = planText.String()
		im.textOverlay = overlay.NewTextOverlay(planText.String())
		h.state = tuiStateHelp // Show the text overlay

		return tea.WindowSize()
	}
}

// handleOrchestratorPlanApproval handles when user approves the orchestrator plan
func (im *controller) handleOrchestratorPlanApproval(h *home) (tea.Model, tea.Cmd) {
	// For testing purposes, just show a success message
	return h, func() tea.Msg {
		// Show success message
		successMessage := "Plan Approved\n\nOrchestration plan has been approved. For testing purposes, no workers will be created."
		im.textOverlay = overlay.NewTextOverlay(successMessage)

		h.state = tuiStateHelp // Show the text overlay
		return tea.WindowSize()
	}
}

// handleOrchestratorPlanKeyPress handles key presses when showing orchestrator plan for approval
func (im *controller) handleOrchestratorPlanKeyPress(h *home, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// User approved the plan
		im.orchestratorPlan = ""
		return im.handleOrchestratorPlanApproval(h)
	case "esc", "q":
		// User cancelled the plan
		im.orchestratorPlan = ""
		im.textOverlay = nil
		h.state = tuiStateDefault
		return h, tea.Sequence(
			tea.WindowSize(),
			func() tea.Msg {
				h.menu.SetState(ui.StateDefault)
				return nil
			},
		)
	default:
		// Any other key shows help about the plan approval
		return h, nil
	}
}

func (im *controller) HandleQuit(h *home) (tea.Model, tea.Cmd) {
	if err := h.storage.SaveInstances(im.list.GetInstances()); err != nil {
		return h, h.handleError(err)
	}
	return h, tea.Quit
}

// createTestWorker creates a dummy worker instance for testing purposes
func (im *controller) createTestWorker(h *home) (tea.Model, tea.Cmd) {
	return h, func() tea.Msg {
		// Create a dummy worker instance with a test name
		testName := fmt.Sprintf("test-worker-%d", time.Now().Unix())

		// Create a new instance with proper options
		opts := session.InstanceOptions{
			Title:   testName,
			Path:    ".",
			Program: "claude", // Use the default program
			AutoYes: h.autoYes,
		}

		// Create the instance
		instance, err := session.NewInstance(opts)
		if err != nil {
			return h.handleError(fmt.Errorf("failed to create test worker: %w", err))
		}

		// Set worker properties
		instance.IsWorker = true
		instance.ParentOrchestrator = "test-orchestrator"

		// Add the instance to the list
		finalizer := im.list.AddInstance(instance)

		// Start the instance
		if err := instance.Start(true); err != nil {
			return h.handleError(fmt.Errorf("failed to start test worker: %w", err))
		}

		// Call the finalizer
		finalizer()

		// Save all instances
		if err := h.storage.SaveInstances(im.list.GetInstances()); err != nil {
			return h.handleError(fmt.Errorf("failed to save instances: %w", err))
		}

		return tea.WindowSize() // Refresh UI to show new worker
	}
}
