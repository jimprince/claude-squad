# Continuous Mode Fix & Enhancement Summary

## Problem
Continuous mode was not working because:
1. Content was constantly changing (cursor blinking, timestamps, etc.), preventing stall detection
2. Pattern matching didn't include Claude Code's specific UI elements
3. The logic required complete content freeze, which never happened with modern terminal UIs

## Solution
Updated the stall detection in `session/instance.go`:

### 1. Added Claude Code-specific completion patterns:
- "What's Working Now:"
- "all essential features implemented"
- "auto-accept edits on"
- "workflow complete"
- Terminal prompt detection ("> " at the end)

### 2. Implemented smart content normalization:
- `normalizeContent()` strips out dynamic elements:
  - ANSI escape codes (colors, cursor movements)
  - Timestamps (HH:MM:SS format)
  - Percentages (like "28%")
- `hashContent()` creates consistent hashes for comparison

### 3. Enhanced continuous mode logic:
- Uses normalized content for stability detection
- Only requires 2 seconds of stable content (not 8 seconds)
- Detects completion states even with minor UI changes
- Sends appropriate commands based on detected state

### 4. Improved user feedback:
- Better status message when toggling continuous mode
- Clear indication of what continuous mode does
- Logging for debugging stall detection

## How It Works Now
1. When continuous mode is enabled (Ctrl+G), the system monitors for:
   - Claude Code showing a completion state (task list, "What's Working Now")
   - A terminal prompt waiting for input
   - Confirmation dialogs

2. If detected and content is stable for 2 seconds, it automatically sends:
   - "continue" for completion states
   - Appropriate responses for confirmation dialogs

3. Dynamic UI elements (timestamps, cursor) are filtered out to prevent false negatives

## Latest Enhancement: Informative Continue Messages

### What's New
1. **Sends `/continuous` command** - When auto-continuing, the system now sends the `/continuous` slash command to Claude Code
2. **Time remaining information** - Shows how much time is left in continuous mode
3. **Duration tracking** - Tracks when continuous mode was enabled and can have an optional duration
4. **Automatic expiration** - Continuous mode can automatically disable after a set duration

### How Continue Messages Work
When Claude Code completes a task and shows "What's Working Now:", the system sends:
- For indefinite mode: `/continuous You're in continuous mode (indefinite duration). Keep working on any remaining tasks or improvements. The system will auto-continue when you complete each task.`
- For timed mode: `/continuous You're in continuous mode. Time remaining: 5m 30s. Keep working on any remaining tasks or improvements.`

This lets Claude Code know:
- It's operating in continuous mode
- How much time is remaining (if applicable)
- It should continue with any remaining work

## Testing
To test the fix:
1. Run claude-squad
2. Start a Claude Code session
3. Press Ctrl+G to enable continuous mode
4. Let Claude complete a task
5. When it shows "What's Working Now:" with a task list, it should:
   - Wait 2 seconds for stability
   - Send the `/continuous` command with time info
   - Auto-continue working

## Complete Implementation Details

### Duration Input
When enabling continuous mode (Ctrl+G), users can now:
- Enter a duration like "30m", "2h", "1h30m"
- Press Enter for indefinite duration
- Maximum duration is 24 hours
- Durations over 2 hours trigger a warning in logs

### UI Improvements
1. **List View**: Shows remaining time as `[C:5m30s]` or `[C]` for indefinite
2. **Duration Input**: Clear examples and max duration shown
3. **Expiration Notification**: Shows "⏰ Continuous mode expired" when time runs out

### Thread Safety
- All continuous mode operations are protected by mutex
- Safe concurrent access from UI and watchdog threads
- Proper state management with dedicated flags

### Performance
- Cached duration formatting (updates once per second)
- Optimized expiration checking
- Minimal overhead in update loop

### Integration
- Works seamlessly with pause/resume
- Continuous mode state persists across session saves
- Compatible with existing watchdog features