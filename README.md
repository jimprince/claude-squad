# Claude Squad [![CI](https://github.com/smtg-ai/claude-squad/actions/workflows/build.yml/badge.svg)](https://github.com/smtg-ai/claude-squad/actions/workflows/build.yml) [![GitHub Release](https://img.shields.io/github/v/release/smtg-ai/claude-squad)](https://github.com/smtg-ai/claude-squad/releases/latest)

[Claude Squad](https://smtg-ai.github.io/claude-squad/) is a terminal app that manages multiple [Claude Code](https://github.com/anthropics/claude-code), [Codex](https://github.com/openai/codex) (and other local agents including [Aider](https://github.com/Aider-AI/aider)) in separate workspaces, allowing you to work on multiple tasks simultaneously.


![Claude Squad Screenshot](assets/screenshot.png)

### Highlights
- Complete tasks in the background (including yolo / auto-accept mode!)
- **🤖 Intelligent Watchdog**: Automatically detects and recovers from stalled sessions
- Manage instances and tasks in one terminal window
- Review changes before applying them, checkout changes before pushing them
- Each task gets its own isolated git workspace, so no conflicts

<br />

https://github.com/user-attachments/assets/aef18253-e58f-4525-9032-f5a3d66c975a

<br />

### Installation

Claude Squad installs as `cs` on your system and can be installed using several methods:

#### Option 1: Quick Install Script (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/smtg-ai/claude-squad/main/install.sh | sh
```

This automatically detects your platform and installs the latest binary to `~/bin/cs`.

#### Option 2: Go Install (For Go developers)

```bash
go install github.com/smtg-ai/claude-squad@latest
```

This installs the `claude-squad` binary to your `$GOPATH/bin`. You may want to create a symlink:
```bash
ln -s "$GOPATH/bin/claude-squad" "$GOPATH/bin/cs"
```

#### Option 3: Download from GitHub Releases

Download the appropriate binary for your platform from the [latest release](https://github.com/smtg-ai/claude-squad/releases/latest):

```bash
# Example for macOS ARM64
curl -L -o cs https://github.com/smtg-ai/claude-squad/releases/latest/download/cs-darwin-arm64
chmod +x cs
mv cs ~/bin/cs  # or /usr/local/bin/cs
```

#### Option 4: Homebrew

```bash
brew install claude-squad
ln -s "$(brew --prefix)/bin/claude-squad" "$(brew --prefix)/bin/cs"
```

#### Option 5: Build from Source

```bash
git clone https://github.com/smtg-ai/claude-squad.git
cd claude-squad
make install  # Installs to ~/bin/cs
```

### Prerequisites

- [tmux](https://github.com/tmux/tmux/wiki/Installing)
- [gh](https://cli.github.com/)

### Usage

```
Usage:
  cs [flags]
  cs [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  debug       Print debug information like config paths
  help        Help about any command
  reset       Reset all stored instances
  version     Print the version number of claude-squad

Flags:
  -y, --autoyes          [experimental] If enabled, all instances will automatically accept prompts for claude code & aider
  -h, --help             help for claude-squad
  -p, --program string   Program to run in new instances (e.g. 'aider --model ollama_chat/gemma3:1b')
```

Run the application with:

```bash
cs
```

<br />

<b>Using Claude Squad with other AI assistants:</b>
- For [Codex](https://github.com/openai/codex): Set your API key with `export OPENAI_API_KEY=<your_key>`
- Launch with specific assistants:
   - Codex: `cs -p "codex"`
   - Aider: `cs -p "aider ..."`
- Make this the default, by modifying the config file (locate with `cs debug`)

### 🤖 Intelligent Watchdog

Claude Squad includes an intelligent watchdog that automatically monitors your AI sessions and recovers from stalls:

- **Auto-Detection**: Recognizes when sessions are waiting for confirmation or appear frozen
- **Smart Recovery**: Automatically sends appropriate "continue" commands to unstall sessions
- **Configurable**: Customize timeout periods, retry attempts, and recovery commands
- **Non-intrusive**: Only intervenes when sessions are genuinely stalled

The watchdog is enabled by default and can be configured in `~/.claude-squad/config.json`. For detailed configuration options and troubleshooting, see [WATCHDOG.md](WATCHDOG.md).

<br />

#### Menu
The menu at the bottom of the screen shows available commands: 

##### Instance/Session Management
- `n` - Create a new session
- `N` - Create a new session with a prompt
- `D` - Kill (delete) the selected session
- `↑/j`, `↓/k` - Navigate between sessions

##### Actions
- `↵/o` - Attach to the selected session to reprompt
- `ctrl-q` - Detach from session
- `s` - Commit and push branch to github
- `c` - Checkout. Commits changes and pauses the session
- `r` - Resume a paused session
- `?` - Show help menu

##### Navigation
- `tab` - Switch between preview tab and diff tab
- `q` - Quit the application
- `shift-↓/↑` - scroll in diff view

### How It Works

1. **tmux** to create isolated terminal sessions for each agent
2. **git worktrees** to isolate codebases so each session works on its own branch
3. A simple TUI interface for easy navigation and management

### License

[AGPL-3.0](LICENSE.md)

### Star History

[![Star History Chart](https://api.star-history.com/svg?repos=smtg-ai/claude-squad&type=Date)](https://www.star-history.com/#smtg-ai/claude-squad&Date)
