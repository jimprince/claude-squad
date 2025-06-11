# Claude Squad Watchdog

Claude Squad now includes intelligent watchdog functionality that automatically monitors your AI coding sessions and recovers from common stall conditions.

## 🔍 What the Watchdog Does

The watchdog continuously monitors all running Claude Code sessions and:

1. **Detects Stalls**: Identifies when sessions are waiting for user input or appear frozen
2. **Auto-Recovery**: Automatically sends appropriate commands to continue stalled sessions  
3. **Pattern Recognition**: Recognizes common stall patterns like "Should I continue?" prompts
4. **Timeout Monitoring**: Detects sessions that haven't had activity for a configurable timeout period

## ⚡ Key Features

- **Real-time Monitoring**: Integrated into the existing 500ms polling loop
- **Intelligent Detection**: Combines pattern matching with activity timeout detection
- **Configurable Recovery**: Customize timeout periods, retry attempts, and continue commands
- **Session Persistence**: Watchdog state is saved and restored with sessions
- **Non-intrusive**: Only intervenes when sessions are genuinely stalled

## 🛠️ Configuration

The watchdog is configured via the `~/.claude-squad/config.json` file:

```json
{
  "watchdog_enabled": true,
  "stall_timeout_seconds": 300,
  "max_continue_attempts": 3,
  "continue_commands": [
    "continue",
    "yes", 
    "y",
    "proceed",
    "\n"
  ]
}
```

### Configuration Options

| Option | Default | Description |
|--------|---------|-------------|
| `watchdog_enabled` | `true` | Enable/disable watchdog for new sessions |
| `stall_timeout_seconds` | `300` | Seconds of inactivity before considering a session stalled |
| `max_continue_attempts` | `3` | Maximum recovery attempts before giving up |
| `continue_commands` | `["continue", "yes", "y", "proceed", "\n"]` | Commands to try when recovering from stalls |

## 🎯 Stall Detection Patterns

The watchdog recognizes these common Claude Code stall patterns:

- "I need confirmation to proceed"
- "Should I continue?"
- "Do you want me to continue?"  
- "Would you like me to proceed?"
- "Press any key to continue"
- "Continue? (y/n)"
- "Proceed? (y/n)"
- "[y/n]" or "(y/n)"
- "Type 'continue' to proceed"
- "waiting for confirmation"
- "Claude Code is waiting"

## 🔄 How It Works

1. **Content Monitoring**: Every 500ms, the watchdog captures the current terminal content
2. **Change Detection**: Uses content hashing to detect when output has changed
3. **Pattern Analysis**: Scans content for known stall patterns
4. **Timeout Tracking**: Tracks time since last meaningful activity
5. **Smart Recovery**: Attempts recovery using configured continue commands
6. **Retry Logic**: Implements exponential backoff with configurable retry limits

## 📊 Session Status

Each session tracks its watchdog status:

- **Enabled**: Whether watchdog monitoring is active
- **Last Activity**: Timestamp of last detected activity
- **Stall Count**: Number of recovery attempts made

You can view this information in the session details within the Claude Squad interface.

## 🚀 Getting Started

### For New Sessions

New sessions automatically inherit the global `watchdog_enabled` setting from your config.

### For Existing Sessions  

Existing sessions will have the watchdog initialized when you:
- Resume a paused session
- Restart Claude Squad

### Manual Control

You can temporarily disable the watchdog for a specific session by modifying the session's watchdog settings.

## 🐛 Troubleshooting

### Watchdog Not Working

1. **Check Configuration**: Verify `watchdog_enabled: true` in config
2. **Check Logs**: Look for watchdog messages in the Claude Squad logs
3. **Verify Patterns**: Ensure your stall pattern matches the recognized patterns
4. **Timeout Settings**: Adjust `stall_timeout_seconds` if needed

### Too Many Interventions

1. **Increase Timeout**: Raise `stall_timeout_seconds` for less aggressive monitoring
2. **Reduce Attempts**: Lower `max_continue_attempts` to prevent spam
3. **Customize Commands**: Modify `continue_commands` for your specific use case

### False Positives

1. **Pattern Conflicts**: Check if normal output contains stall patterns
2. **Activity Detection**: Verify that legitimate activity is being detected
3. **Logging**: Enable debug logging to see what the watchdog is detecting

## 📝 Example Usage

### Basic Setup
```bash
# Enable watchdog globally
./cs debug  # Check current config
# Edit ~/.claude-squad/config.json to enable watchdog

# Start a new session (watchdog auto-enabled)  
./cs -p "claude"
```

### Custom Configuration
```json
{
  "watchdog_enabled": true,
  "stall_timeout_seconds": 180,
  "max_continue_attempts": 5,
  "continue_commands": [
    "continue",
    "proceed", 
    "yes",
    "y"
  ]
}
```

## 🔧 Advanced Features

### Per-Session Control
Each session can have its watchdog individually enabled/disabled while preserving the global default for new sessions.

### Logging Integration
All watchdog activity is logged through the existing Claude Squad logging system:
- **Info**: Normal watchdog operations
- **Warning**: Stall detection and recovery attempts  
- **Error**: Failed recovery attempts

### State Persistence
Watchdog state (activity time, stall count) is automatically saved and restored with session data.

## 🤝 Contributing

The watchdog functionality is implemented in:
- `session/instance.go` - Core watchdog logic
- `config/config.go` - Configuration handling
- `app/app.go` - Integration with main polling loop

To extend the watchdog:
1. Add new stall patterns to `DetectStall()`
2. Implement custom recovery strategies in `InjectContinue()`
3. Add new configuration options to `Config` struct
4. Update documentation and tests

## 📚 Technical Details

### Architecture
- **Detection**: Pattern matching + timeout-based activity monitoring
- **Recovery**: Command injection via tmux session
- **Integration**: Hooks into existing 500ms metadata polling cycle
- **Persistence**: JSON serialization with session data

### Performance
- **Overhead**: Minimal - leverages existing polling infrastructure  
- **Memory**: Low - uses content hashing for change detection
- **CPU**: Efficient - pattern matching only on content changes

### Reliability
- **Error Handling**: Graceful degradation on detection failures
- **Resource Safety**: Automatic cleanup on session termination
- **State Consistency**: Atomic operations for state updates