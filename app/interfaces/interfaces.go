package interfaces

import (
	instanceInterfaces "claude-squad/instance/interfaces"
	"claude-squad/ui"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// ModelInterface defines what the controller needs from the model
type ModelInterface interface {
	// Implement tea.Model interface
	tea.Model

	// Storage operations
	GetStorage() StorageInterface

	// State management
	GetState() int
	SetState(state int)

	// UI components
	GetMenu() MenuInterface
	GetErrBox() ErrBoxInterface
	GetSpinner() *spinner.Model

	// Configuration
	GetProgram() string
	GetAutoYes() bool

	// Event handlers
	HandleError(err error) tea.Cmd
	ShowHelpScreen(helpType int, instance interface{}, data interface{}, callback func())
	HandleMenuHighlighting(msg tea.KeyMsg) (tea.Cmd, bool)
	UpdateHandleWindowSizeEvent(msg tea.WindowSizeMsg)
	HandleQuit() (tea.Model, tea.Cmd)
	HandleHelpState(msg tea.KeyMsg, textOverlay interface{}) (tea.Model, tea.Cmd)
	KeydownCallback(name string) tea.Cmd
}

// StorageInterface defines storage operations needed by controller
type StorageInterface interface {
	LoadInstances() ([]instanceInterfaces.Instance, error)
	SaveInstances(instances []instanceInterfaces.Instance) error
	DeleteInstance(title string) error
}

// MenuInterface defines menu operations needed by controller
type MenuInterface interface {
	SetState(state ui.MenuState)
	SetInstance(instance interface{})
	SetInDiffTab(inDiffTab bool)
	ClearKeydown()
	String() string
}

// ErrBoxInterface defines error box operations needed by controller
type ErrBoxInterface interface {
	Clear()
	String() string
}
