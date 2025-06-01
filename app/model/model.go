package model

import (
	"claude-squad/config"
	"claude-squad/instance"
	instanceInterfaces "claude-squad/instance/interfaces"
	"claude-squad/instance/types"
	"claude-squad/keys"
	"claude-squad/registry"
	"claude-squad/ui"
	"context"
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// Forward declaration to avoid circular dependency
type Controller struct{}

type Model struct {
	ctx context.Context

	// # State

	// state is the current discrete state of the app
	state tuiState
	// program is the program to use for instances and orchestrators
	program string
	// autoYes is whether to automatically approve actions
	autoYes bool
	// keySent is used to manage underlining menu items
	keySent bool

	// # UI components

	// menu is the menu UI component
	menu *ui.Menu
	// errBox is the error box UI component
	errBox *ui.ErrBox
	// spinner is the global spinner instance. We plumb this down to where it's needed
	spinner spinner.Model

	// # Storage and Configuration

	// storage is the interface for saving/loading data to/from the app's state
	storage *instance.Storage[instanceInterfaces.Instance]
	// appConfig stores persistent application configuration
	appConfig *config.Config
	// appState stores persistent application state like seen help screens
	appState config.AppState

	// Controller will be injected after creation to avoid circular dependency
	controller ControllerInterface
}

// ControllerInterface defines what we need from the controller to avoid circular dependency
type ControllerInterface interface {
	LoadExistingInstances(storage interface{}) error
	Render(m interface{}) string
	Update(m interface{}, msg tea.Msg) (tea.Model, tea.Cmd)
	HandleQuit(m interface{})
	GetList() *ui.List
	GetTabbedWindow() *ui.TabbedWindow
}

func NewModel(ctx context.Context, program string, autoYes bool) *Model {
	appConfig := config.LoadConfig()
	appState := config.LoadState()

	// Create serialization functions for Instance interface
	toData := func(i instanceInterfaces.Instance) ([]byte, error) {
		return registry.MarshalInstanceWithType(i)
	}

	fromData := func(data []byte) (instanceInterfaces.Instance, error) {
		// Unmarshal the instance with type information from the registry
		var tagged types.TaggedInstance
		if err := json.Unmarshal(data, &tagged); err != nil {
			return nil, err
		}
		return registry.UnmarshalInstanceWithType(tagged)
	}

	getTitle := func(i instanceInterfaces.Instance) string {
		return i.StatusText()
	}

	storage := instance.NewStorage(appState, toData, fromData, getTitle)

	h := &Model{
		ctx:       ctx,
		spinner:   spinner.New(spinner.WithSpinner(spinner.MiniDot)),
		menu:      ui.NewMenu(),
		errBox:    ui.NewErrBox(),
		storage:   storage,
		appConfig: appConfig,
		program:   program,
		autoYes:   autoYes,
		state:     tuiStateDefault,
		appState:  appState,
	}

	return h
}

// SetController injects the controller after creation to avoid circular dependency
func (m *Model) SetController(controller ControllerInterface) {
	m.controller = controller
	if err := controller.LoadExistingInstances(m.storage); err != nil {
		fmt.Printf("Warning: Failed to load existing instances: %v\n", err)
	} else {
		fmt.Printf("Successfully loaded existing instances\n")
	}
}

// View renders the UI using the controller
func (m *Model) View() string {
	if m.controller != nil {
		return m.controller.Render(m)
	}
	return "Loading..."
}

// updateHandleWindowSizeEvent sets the sizes of the components.
// The components will try to render inside their bounds.
func (m *Model) updateHandleWindowSizeEvent(msg tea.WindowSizeMsg) {
	// Menu takes 10% of height, list and window take 90%
	contentHeight := int(float32(msg.Height) * 0.9)
	menuHeight := msg.Height - contentHeight - 1     // minus 1 for error box
	m.errBox.SetSize(int(float32(msg.Width)*0.9), 1) // error box takes 1 row

	m.menu.SetSize(msg.Width, menuHeight)

	// Set sizes for instance mode components
	if m.controller != nil {
		// Split the content width between list and preview
		// List takes ~40% of width, preview takes ~60%
		listWidth := int(float32(msg.Width) * 0.4)
		previewWidth := msg.Width - listWidth

		m.controller.GetList().SetSize(listWidth, contentHeight)
		m.controller.GetTabbedWindow().SetSize(previewWidth, contentHeight)
	}
}

func (m *Model) Init() tea.Cmd {
	// Upon starting, we want to start the spinner. Whenever we get a spinner.TickMsg, we
	// update the spinner, which sends a new spinner.TickMsg. I think this lasts forever lol.
	return tea.Batch(
		m.spinner.Tick,
	)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.controller != nil {
		return m.controller.Update(m, msg)
	}
	return m, nil
}

func (m *Model) handleQuit() (tea.Model, tea.Cmd) {
	if m.controller != nil {
		m.controller.HandleQuit(m)
	}
	return m, tea.Quit
}

func (m *Model) handleMenuHighlighting(msg tea.KeyMsg) (cmd tea.Cmd, returnEarly bool) {
	// Handle menu highlighting when you press a button. We intercept it here and immediately return to
	// update the ui while re-sending the keypress. Then, on the next call to this, we actually handle the keypress.
	if m.keySent {
		m.keySent = false
		return nil, false
	}
	if m.state == tuiStatePrompt || m.state == tuiStateHelp {
		return nil, false
	}
	// If it's in the instance mode keymap, we should try to highlight it.
	name, ok := keys.InstanceModeKeyMap[msg.String()]
	if !ok {
		return nil, false
	}

	m.keySent = true
	return tea.Batch(
		func() tea.Msg { return msg },
		m.keydownCallback(name)), true
}
