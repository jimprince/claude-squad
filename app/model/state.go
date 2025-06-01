package model

type tuiState int

const (
	tuiStateDefault tuiState = iota
	// tuiStateNew is the state when the user is creating a new instance.
	tuiStateNew
	// tuiStatePrompt is the state when the user is entering a prompt.
	tuiStatePrompt
	// tuiStateHelp is the state when a help screen is displayed.
	tuiStateHelp
)
