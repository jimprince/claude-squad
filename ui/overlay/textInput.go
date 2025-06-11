package overlay

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TextInputOverlay represents a text input overlay with state management.
type TextInputOverlay struct {
	textinput     textinput.Model
	Title         string
	FocusIndex    int // 0 for text input, 1 for enter button
	Submitted     bool
	Canceled      bool
	OnSubmit      func()
	width, height int
}

// NewTextInputOverlay creates a new text input overlay with the given title and initial value.
func NewTextInputOverlay(title string, initialValue string) *TextInputOverlay {
	ti := textinput.New()
	ti.SetValue(initialValue)
	ti.Focus()
	ti.Prompt = ""
	
	// Set placeholder text for duration input
	ti.Placeholder = "e.g., 30m, 2h, 1h30m"

	// Ensure no character limit
	ti.CharLimit = 0

	return &TextInputOverlay{
		textinput:  ti,
		Title:      title,
		FocusIndex: 0,
		Submitted:  false,
		Canceled:   false,
	}
}

func (t *TextInputOverlay) SetSize(width, height int) {
	t.textinput.Width = width - 6 // Account for padding and borders
	t.width = width
	t.height = height
}

// Init initializes the text input overlay model
func (t *TextInputOverlay) Init() tea.Cmd {
	return textinput.Blink
}

// View renders the model's view
func (t *TextInputOverlay) View() string {
	return t.Render()
}

// HandleKeyPress processes a key press and updates the state accordingly.
// Returns true if the overlay should be closed.
func (t *TextInputOverlay) HandleKeyPress(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyTab:
		// Toggle focus between input and enter button.
		t.FocusIndex = (t.FocusIndex + 1) % 2
		if t.FocusIndex == 0 {
			t.textinput.Focus()
		} else {
			t.textinput.Blur()
		}
		return false
	case tea.KeyShiftTab:
		// Toggle focus in reverse.
		t.FocusIndex = (t.FocusIndex + 1) % 2
		if t.FocusIndex == 0 {
			t.textinput.Focus()
		} else {
			t.textinput.Blur()
		}
		return false
	case tea.KeyEsc:
		t.Canceled = true
		return true
	case tea.KeyEnter:
		if t.FocusIndex == 1 {
			// Enter button is focused, so submit.
			t.Submitted = true
			if t.OnSubmit != nil {
				t.OnSubmit()
			}
			return true
		}
		// For single-line input, Enter on the input field should submit
		t.Submitted = true
		if t.OnSubmit != nil {
			t.OnSubmit()
		}
		return true
	default:
		if t.FocusIndex == 0 {
			t.textinput, _ = t.textinput.Update(msg)
		}
		return false
	}
}

// GetValue returns the current value of the text input.
func (t *TextInputOverlay) GetValue() string {
	return t.textinput.Value()
}

// IsSubmitted returns whether the form was submitted.
func (t *TextInputOverlay) IsSubmitted() bool {
	return t.Submitted
}

// IsCanceled returns whether the form was canceled.
func (t *TextInputOverlay) IsCanceled() bool {
	return t.Canceled
}

// SetOnSubmit sets a callback function for form submission.
func (t *TextInputOverlay) SetOnSubmit(onSubmit func()) {
	t.OnSubmit = onSubmit
}

// Render renders the text input overlay.
func (t *TextInputOverlay) Render() string {
	// Create styles
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("62")).
		Bold(true).
		MarginBottom(1)

	buttonStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("7"))

	focusedButtonStyle := buttonStyle
	focusedButtonStyle = focusedButtonStyle.
		Background(lipgloss.Color("62")).
		Foreground(lipgloss.Color("0"))

	// Set textinput width to fit within the overlay
	t.textinput.Width = t.width - 6 // Account for padding and borders

	// Build the view
	content := titleStyle.Render(t.Title) + "\n"
	content += t.textinput.View() + "\n\n"

	// Render enter button with appropriate style
	enterButton := " Enter "
	if t.FocusIndex == 1 {
		enterButton = focusedButtonStyle.Render(enterButton)
	} else {
		enterButton = buttonStyle.Render(enterButton)
	}
	content += enterButton

	return style.Render(content)
}
