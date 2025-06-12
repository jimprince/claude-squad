# Claude Squad - Developer Guide

## Overview

Claude Squad is a terminal-based manager for multiple AI agents like Claude Code, Aider, Codex, and Amp. It provides a unified interface to manage multiple AI coding sessions simultaneously.

## Project Structure

```
claude-squad/
├── app/                    # Main application logic
│   ├── app.go             # Core UI and state management
│   ├── app_test.go        # Application tests
│   └── help.go            # Help system
├── cmd/                   # CLI command handling
├── config/                # Configuration management
├── daemon/                # Background process management
├── keys/                  # Keyboard input handling
├── log/                   # Logging utilities
├── session/               # Session management
│   ├── git/               # Git integration
│   └── tmux/              # Terminal multiplexer integration
├── ui/                    # User interface components
│   └── overlay/           # Modal overlays
└── web/                   # Web interface (Next.js)
```

## Building and Installation

### Local Development Build
```bash
go build -o claude-squad-binary
```

### Install as `cs` Command
The project is typically installed as the `cs` command for easy CLI access:

```bash
# Build and install to ~/bin (recommended)
go build -o ~/bin/cs

# Or build and install to system PATH (requires sudo)
go build -o /usr/local/bin/cs
```

### Testing
```bash
# Run all tests
go test ./...

# Run specific package tests
go test -v ./app
go test -v ./session/tmux
go test -v ./session/git
```

## Key Features

- **Multi-session Management**: Handle multiple AI coding sessions simultaneously
- **Terminal Integration**: Built on tmux for robust terminal session management
- **Git Integration**: Automatic git repository detection and branch management
- **Continuous Mode**: Run sessions for specified durations (e.g., "30m", "2h")
- **Auto-yes Mode**: Automatically accept prompts for unattended operation

## Recent Fixes

### Continuous Mode Panic Fix (June 2025)
**Issue**: Application crashed with nil pointer dereference when specifying time durations for continuous mode.

**Root Cause**: Code was accessing `m.continuousModeTarget.Title` after setting `m.continuousModeTarget = nil`.

**Fix**: Capture the target title before setting the pointer to nil:
```go
// Before fix (caused panic):
m.continuousModeTarget = nil
return m.handleError(fmt.Errorf("✓ Continuous mode %s for '%s'", modeText, m.continuousModeTarget.Title))

// After fix:
targetTitle := m.continuousModeTarget.Title
m.continuousModeTarget = nil
return m.handleError(fmt.Errorf("✓ Continuous mode %s for '%s'", modeText, targetTitle))
```

**Test Coverage**: Added `TestContinuousModeFixed` to prevent regression.

### Text Input Character-per-Line Fix (June 2025)
**Issue**: Continuous mode duration input displayed one character per line with newlines between each character.

**Root Cause**: Using `textarea` component (multi-line) instead of `textinput` component (single-line) for duration input.

**Fix**: Replaced `textarea.Model` with `textinput.Model` in `ui/overlay/textInput.go`:
```go
// Before fix (multi-line textarea):
type TextInputOverlay struct {
    textarea textarea.Model
    // ...
}

// After fix (single-line textinput):
type TextInputOverlay struct {
    textinput textinput.Model
    // ...
}
```

**Additional Changes**:
- Updated Enter key behavior: pressing Enter on input field now submits directly
- Added placeholder text for better UX: "e.g., 30m, 2h, 1h30m"
- Simplified focus management for single-line input

**Test Coverage**: Added `TestTextInputSingleLine` to verify single-line behavior.

## Development Workflow

1. **Make Changes**: Edit source code in relevant packages
2. **Add Tests**: Follow TDD - write tests for new functionality
3. **Run Tests**: Ensure all tests pass with `go test ./...`
4. **Build**: Create binary with `go build -o cs`
5. **Test Installation**: Verify the installed binary works correctly
6. **Deploy**: Replace the system `cs` binary

## Common Issues

### Binary Getting Killed on Launch
If the installed binary gets killed immediately:
1. Check file permissions: `ls -la ~/bin/cs`
2. Verify architecture: `file ~/bin/cs`
3. Try clean rebuild: `go clean && go build -o ~/bin/cs`
4. Test directly from build directory first

### TTY Errors in Non-Interactive Environments
The application requires a TTY for terminal interaction. When running from scripts or CI:
- Error: "could not open a new TTY: open /dev/tty: device not configured"
- This is expected behavior - the app needs an interactive terminal

## Git Repository Structure

- **origin**: `https://github.com/jimprince/claude-squad.git` (our controlled fork)
- **upstream**: `https://github.com/smtg-ai/claude-squad.git` (original repo we forked from)
- **Branch Strategy**: Work on feature branches, merge to origin, then upstream

## Configuration

- **Config Directory**: `~/.claude-squad/`
- **Log Files**: `/tmp/claudesquad.log` (or system temp directory)
- **Session Storage**: Managed automatically in config directory

## Usage Examples

```bash
# Start claude-squad
cs

# Start with auto-yes mode
cs --autoyes

# Start with custom program
cs --program "aider --model ollama_chat/gemma3:1b"

# Check version
cs version

# Reset all sessions
cs reset

# Get help
cs --help
```

## Testing Guidelines

When making changes to the codebase:

1. **Write Tests First**: Follow TDD principles
2. **Test State Transitions**: Especially for UI state changes
3. **Test Error Conditions**: Ensure proper error handling
4. **Test Installation**: Always verify the built binary works after deployment
5. **Integration Tests**: Test full workflows, not just unit functionality

## Architecture Notes

- **State Management**: Uses bubble tea framework for TUI state management
- **Session Isolation**: Each AI agent runs in its own tmux session
- **Error Handling**: Centralized error display through UI error box
- **Keyboard Navigation**: Modal-based navigation with overlay system
- **Async Operations**: Non-blocking UI updates for long-running operations

## Contributing

1. Follow the existing code style and patterns
2. Add comprehensive tests for new features
3. Update this CLAUDE.md file when making significant changes
4. Test the installation process before submitting changes
5. Document any breaking changes or new configuration requirements