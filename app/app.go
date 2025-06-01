package app

import (
	"claude-squad/app/controller"
	"claude-squad/app/model"
	"context"

	tea "github.com/charmbracelet/bubbletea"
)

const GlobalInstanceLimit = 10

// Run is the main entrypoint into the application.
func Run(ctx context.Context, program string, autoYes bool) error {
	// Create model first
	m := model.NewModel(ctx, program, autoYes)

	// Create controller
	c := controller.NewController(m.GetSpinner(), m.GetAutoYes())

	// Inject controller into model to break circular dependency
	m.SetController(c)

	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(), // Mouse scroll
	)
	_, err := p.Run()
	return err
}
