# Claude Code Restart Feature Test Plan

## Feature Overview
The restart feature allows users to restart Claude Code while preserving the conversation context using Ctrl+R.

## Test Scenarios

### 1. Basic Restart Test
1. Start claude-squad
2. Create a new Claude Code session
3. Have a conversation with Claude (ask it to write some code)
4. Press Ctrl+R
5. Confirm the restart dialog
6. **Expected**: Claude restarts and conversation history is preserved

### 2. Continuous Mode Preservation Test
1. Start a Claude Code session
2. Enable continuous mode (Ctrl+G)
3. Let Claude complete a task
4. Press Ctrl+R to restart
5. **Expected**: After restart, continuous mode should still be enabled

### 3. Restart Cooldown Test
1. Start a Claude Code session
2. Press Ctrl+R and confirm restart
3. Immediately press Ctrl+R again
4. **Expected**: Error message "please wait X seconds before restarting again"

### 4. Cancel Restart Test
1. Start a Claude Code session
2. Press Ctrl+R
3. Press Esc or 'n' to cancel
4. **Expected**: Restart is cancelled, session continues normally

### 5. Restart During Activity Test
1. Start a Claude Code session
2. Ask Claude to perform a long task
3. While Claude is working, press Ctrl+R
4. **Expected**: Restart waits gracefully for Claude to finish or times out

### 6. Multiple Instance Test
1. Start two Claude Code sessions
2. Select the first one and restart it
3. Select the second one and restart it
4. **Expected**: Both restart independently without affecting each other

### 7. Error Handling Test
1. Start a Claude Code session
2. Manually delete the session file from ~/.claude/projects/
3. Press Ctrl+R
4. **Expected**: Graceful error message about missing session

### 8. Paused Instance Test
1. Start a Claude Code session
2. Pause it (press 'c' to checkout)
3. Try to restart (Ctrl+R)
4. **Expected**: Error message "cannot restart: instance is paused"

### 9. Non-Claude Instance Test
1. Start an instance with a different program (if supported)
2. Try to restart (Ctrl+R)
3. **Expected**: Error message "restart only supported for Claude Code sessions"

### 10. UI Feedback Test
1. Start a Claude Code session
2. Press Ctrl+R
3. Observe the UI during restart
4. **Expected**: 
   - Confirmation dialog appears
   - Status shows restart progress
   - Menu shows Ctrl+R option

## Verification Checklist
- [ ] Ctrl+R appears in the menu
- [ ] Confirmation dialog works properly
- [ ] Session history is preserved after restart
- [ ] Continuous mode state is preserved
- [ ] Cooldown prevents rapid restarts
- [ ] Mutex prevents concurrent restarts
- [ ] Error messages are clear and helpful
- [ ] No crashes or panics during restart
- [ ] Logs show restart activity
- [ ] Watchdog doesn't interfere with manual restart

## Edge Cases to Monitor
- Very large conversation histories
- Claude Code updates between restarts
- Network issues during restart
- File permission issues
- Concurrent operations during restart