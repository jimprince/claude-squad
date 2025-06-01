package model

import (
	appInterfaces "claude-squad/app/interfaces"
	"claude-squad/instance"
	instanceInterfaces "claude-squad/instance/interfaces"
	"claude-squad/instance/task"
	"claude-squad/keys"
	"claude-squad/ui"
	"claude-squad/ui/overlay"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// Interface implementations for ModelInterface

// storageAdapter wraps the generic storage to implement StorageInterface
type storageAdapter struct {
	storage *instance.Storage[instanceInterfaces.Instance]
}

func (s *storageAdapter) LoadInstances() ([]instanceInterfaces.Instance, error) {
	return s.storage.LoadInstances()
}

func (s *storageAdapter) SaveInstances(instances []instanceInterfaces.Instance) error {
	return s.storage.SaveInstances(instances)
}

func (s *storageAdapter) DeleteInstance(title string) error {
	return s.storage.DeleteInstance(title)
}

// GetStorage returns the storage interface
func (m *Model) GetStorage() appInterfaces.StorageInterface {
	return &storageAdapter{storage: m.storage}
}

// GetState returns the current state as an int
func (m *Model) GetState() int {
	return int(m.state)
}

// SetState sets the current state from an int
func (m *Model) SetState(state int) {
	m.state = tuiState(state)
}

// GetMenu returns the menu interface
func (m *Model) GetMenu() appInterfaces.MenuInterface {
	return &menuWrapper{menu: m.menu}
}

// menuWrapper wraps ui.Menu to implement MenuInterface
type menuWrapper struct {
	menu *ui.Menu
}

func (w *menuWrapper) SetState(state ui.MenuState) {
	w.menu.SetState(state)
}

func (w *menuWrapper) SetInstance(instance interface{}) {
	if task, ok := instance.(*task.Task); ok {
		w.menu.SetInstance(task)
	} else {
		w.menu.SetInstance(nil)
	}
}

func (w *menuWrapper) SetInDiffTab(inDiffTab bool) {
	w.menu.SetInDiffTab(inDiffTab)
}

func (w *menuWrapper) ClearKeydown() {
	w.menu.ClearKeydown()
}

func (w *menuWrapper) String() string {
	return w.menu.String()
}

// GetErrBox returns the error box interface
func (m *Model) GetErrBox() appInterfaces.ErrBoxInterface {
	return m.errBox
}

// GetSpinner returns the spinner model
func (m *Model) GetSpinner() *spinner.Model {
	return &m.spinner
}

// GetProgram returns the program string
func (m *Model) GetProgram() string {
	return m.program
}

// GetAutoYes returns the autoYes setting
func (m *Model) GetAutoYes() bool {
	return m.autoYes
}

// HandleError handles errors by calling the internal handleError method
func (m *Model) HandleError(err error) tea.Cmd {
	return m.handleError(err)
}

// ShowHelpScreen shows a help screen by calling the internal showHelpScreen method
func (m *Model) ShowHelpScreen(helpTypeInt int, instance interface{}, data interface{}, callback func()) {
	var taskPtr *task.Task
	var overlayPtr *overlay.TextOverlay

	if instance != nil {
		if t, ok := instance.(*task.Task); ok {
			taskPtr = t
		}
	}
	if data != nil {
		if o, ok := data.(*overlay.TextOverlay); ok {
			overlayPtr = o
		}
	}

	m.showHelpScreen(helpType(helpTypeInt), taskPtr, overlayPtr, callback)
}

// HandleMenuHighlighting handles menu highlighting by calling the internal method
func (m *Model) HandleMenuHighlighting(msg tea.KeyMsg) (tea.Cmd, bool) {
	return m.handleMenuHighlighting(msg)
}

// UpdateHandleWindowSizeEvent handles window size events
func (m *Model) UpdateHandleWindowSizeEvent(msg tea.WindowSizeMsg) {
	m.updateHandleWindowSizeEvent(msg)
}

// HandleQuit handles quit events
func (m *Model) HandleQuit() (tea.Model, tea.Cmd) {
	return m.handleQuit()
}

// HandleHelpState handles help state events
func (m *Model) HandleHelpState(msg tea.KeyMsg, textOverlay interface{}) (tea.Model, tea.Cmd) {
	if overlay, ok := textOverlay.(*overlay.TextOverlay); ok {
		return m.handleHelpState(msg, overlay)
	}
	return m, nil
}

// KeydownCallback handles keydown callbacks
func (m *Model) KeydownCallback(name string) tea.Cmd {
	if keyName, ok := keys.InstanceModeKeyMap[name]; ok {
		return m.keydownCallback(keyName)
	}
	return nil
}
