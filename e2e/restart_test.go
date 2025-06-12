// +build e2e

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/smtg-ai/claude-squad/log"
	"github.com/smtg-ai/claude-squad/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain sets up the test environment
func TestMain(m *testing.M) {
	// Initialize logger
	log.Initialize(false)
	defer log.Close()

	// Run tests
	code := m.Run()
	
	// Cleanup
	cleanupTestSessions()
	
	os.Exit(code)
}

// cleanupTestSessions removes any test tmux sessions
func cleanupTestSessions() {
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}")
	output, _ := cmd.Output()
	sessions := strings.Split(string(output), "\n")
	
	for _, session := range sessions {
		if strings.HasPrefix(session, "test-") {
			exec.Command("tmux", "kill-session", "-t", session).Run()
		}
	}
}

// TestRestartScenarios tests all 10 restart scenarios from the test plan
func TestRestartScenarios(t *testing.T) {
	// Skip if Claude Code is not available
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("Claude Code not found in PATH")
	}

	// Test 1: Basic Restart Test
	t.Run("basic restart preserves conversation", func(t *testing.T) {
		// Create a test instance
		instance, err := session.NewInstance(session.InstanceOptions{
			Title:   "test-restart-basic",
			Path:    ".",
			Program: "claude code",
		})
		require.NoError(t, err)
		defer instance.Kill()

		// Start the instance
		err = instance.Start(true)
		require.NoError(t, err)

		// Give Claude time to start
		time.Sleep(2 * time.Second)

		// Send a test message via tmux directly
		tmuxCmd := exec.Command("tmux", "send-keys", "-t", instance.Title, "Hello Claude, remember this message for the restart test", "Enter")
		err = tmuxCmd.Run()
		require.NoError(t, err)
		time.Sleep(1 * time.Second)

		// Perform restart
		err = instance.ManualRestart()
		assert.NoError(t, err)

		// Give Claude time to restart
		time.Sleep(3 * time.Second)

		// Verify session is still alive
		assert.True(t, instance.TmuxAlive())
		
		// Capture pane content to verify conversation preserved
		captureCmd := exec.Command("tmux", "capture-pane", "-t", instance.Title, "-p")
		output, err := captureCmd.Output()
		assert.NoError(t, err)
		content := string(output)
		assert.Contains(t, content, "remember this message", "Conversation should be preserved after restart")
	})

	// Test 2: Continuous Mode Preservation
	t.Run("continuous mode preserved after restart", func(t *testing.T) {
		instance, err := session.NewInstance(session.InstanceOptions{
			Title:   "test-restart-continuous",
			Path:    ".",
			Program: "claude code",
		})
		require.NoError(t, err)
		defer instance.Kill()

		err = instance.Start(true)
		require.NoError(t, err)
		time.Sleep(2 * time.Second)

		// Enable continuous mode
		instance.SetContinuousModeDuration(30*time.Minute)
		assert.Greater(t, instance.GetContinuousModeTimeRemaining(), time.Duration(0))

		// Restart
		err = instance.ManualRestart()
		assert.NoError(t, err)
		time.Sleep(3 * time.Second)

		// Verify continuous mode is still enabled
		assert.Greater(t, instance.GetContinuousModeTimeRemaining(), time.Duration(0), "Continuous mode should be preserved after restart")
	})

	// Test 3: Restart Cooldown
	t.Run("restart cooldown prevents rapid restarts", func(t *testing.T) {
		instance, err := session.NewInstance(session.InstanceOptions{
			Title:   "test-restart-cooldown",
			Path:    ".",
			Program: "claude code",
		})
		require.NoError(t, err)
		defer instance.Kill()

		err = instance.Start(true)
		require.NoError(t, err)
		time.Sleep(2 * time.Second)

		// First restart should work
		err = instance.ManualRestart()
		assert.NoError(t, err)

		// Second restart immediately should fail
		err = instance.ManualRestart()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "please wait", "Should enforce cooldown between restarts")
	})

	// Test 4: Cancel Restart (tested via unit tests)
	t.Run("cancel restart via confirmation dialog", func(t *testing.T) {
		t.Skip("Cancellation is tested in unit tests - requires UI interaction")
	})

	// Test 5: Restart During Activity
	t.Run("restart during activity handles gracefully", func(t *testing.T) {
		instance, err := session.NewInstance(session.InstanceOptions{
			Title:   "test-restart-activity",
			Path:    ".",
			Program: "claude code",
		})
		require.NoError(t, err)
		defer instance.Kill()

		err = instance.Start(true)
		require.NoError(t, err)
		time.Sleep(2 * time.Second)

		// Send a command that takes time
		tmuxCmd := exec.Command("tmux", "send-keys", "-t", instance.Title, "Please count from 1 to 100 slowly", "Enter")
		err = tmuxCmd.Run()
		require.NoError(t, err)
		time.Sleep(500 * time.Millisecond)

		// Restart while Claude is working
		err = instance.ManualRestart()
		assert.NoError(t, err)

		// Should complete without hanging
		time.Sleep(3 * time.Second)
		assert.True(t, instance.TmuxAlive())
	})

	// Test 6: Multiple Instance Independence
	t.Run("multiple instances restart independently", func(t *testing.T) {
		// Create two instances
		instance1, err := session.NewInstance(session.InstanceOptions{
			Title:   "test-restart-multi-1",
			Path:    ".",
			Program: "claude code",
		})
		require.NoError(t, err)
		defer instance1.Kill()

		instance2, err := session.NewInstance(session.InstanceOptions{
			Title:   "test-restart-multi-2",
			Path:    ".",
			Program: "claude code",
		})
		require.NoError(t, err)
		defer instance2.Kill()

		// Start both
		err = instance1.Start(true)
		require.NoError(t, err)
		err = instance2.Start(true)
		require.NoError(t, err)
		time.Sleep(2 * time.Second)

		// Restart first instance
		err = instance1.ManualRestart()
		assert.NoError(t, err)

		// Second instance should still be running normally
		assert.True(t, instance2.TmuxAlive())

		// Restart second instance
		err = instance2.ManualRestart()
		assert.NoError(t, err)

		// Both should be alive
		time.Sleep(3 * time.Second)
		assert.True(t, instance1.TmuxAlive())
		assert.True(t, instance2.TmuxAlive())
	})

	// Test 7: Error Handling - Missing Session
	t.Run("restart handles missing session gracefully", func(t *testing.T) {
		instance, err := session.NewInstance(session.InstanceOptions{
			Title:   "test-restart-missing",
			Path:    ".",
			Program: "claude code",
		})
		require.NoError(t, err)
		defer instance.Kill()

		err = instance.Start(true)
		require.NoError(t, err)
		time.Sleep(2 * time.Second)

		// Get the claude session directory
		homeDir, _ := os.UserHomeDir()
		_ = filepath.Join(homeDir, ".claude", "projects")
		
		// Note: We can't easily delete session files in a real test
		// This would be better as a mock test
		t.Skip("Cannot safely delete session files in integration test")
	})

	// Test 8: Paused Instance
	t.Run("restart fails for paused instances", func(t *testing.T) {
		instance, err := session.NewInstance(session.InstanceOptions{
			Title:   "test-restart-paused",
			Path:    ".",
			Program: "claude code",
		})
		require.NoError(t, err)
		defer instance.Kill()

		err = instance.Start(true)
		require.NoError(t, err)
		time.Sleep(2 * time.Second)

		// Pause the instance
		err = instance.Pause()
		require.NoError(t, err)

		// Try to restart
		err = instance.ManualRestart()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "instance is paused")
	})

	// Test 9: Non-Claude Instance
	t.Run("restart only works for Claude instances", func(t *testing.T) {
		instance, err := session.NewInstance(session.InstanceOptions{
			Title:   "test-restart-nonclaude",
			Path:    ".",
			Program: "echo 'not claude'",
		})
		require.NoError(t, err)
		defer instance.Kill()

		// Don't need to start it, just test the validation
		err = instance.ManualRestart()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "restart only supported for Claude Code sessions")
	})

	// Test 10: UI Feedback (tested in unit tests)
	t.Run("UI shows restart progress", func(t *testing.T) {
		t.Skip("UI feedback is tested in unit tests")
	})
}

// TestRestartWithLargeHistory tests restart with a large conversation history
func TestRestartWithLargeHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large history test in short mode")
	}

	instance, err := session.NewInstance(session.InstanceOptions{
		Title:   "test-restart-large-history",
		Path:    ".",
		Program: "claude code",
	})
	require.NoError(t, err)
	defer instance.Kill()

	err = instance.Start(true)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Send many messages to build up history
	for i := 0; i < 50; i++ {
		msg := fmt.Sprintf("Message %d: This is a test message to build up conversation history", i)
		tmuxCmd := exec.Command("tmux", "send-keys", "-t", instance.Title, msg, "Enter")
		err = tmuxCmd.Run()
		require.NoError(t, err)
		time.Sleep(100 * time.Millisecond)
	}

	// Wait for Claude to process
	time.Sleep(5 * time.Second)

	// Perform restart
	startTime := time.Now()
	err = instance.ManualRestart()
	assert.NoError(t, err)
	restartDuration := time.Since(startTime)

	// Restart should complete in reasonable time even with large history
	assert.Less(t, restartDuration, 30*time.Second, "Restart should complete within 30 seconds")

	// Verify session is alive
	time.Sleep(3 * time.Second)
	assert.True(t, instance.TmuxAlive())
}

// TestRestartRaceConditions tests for race conditions during restart
func TestRestartRaceConditions(t *testing.T) {
	instance, err := session.NewInstance(session.InstanceOptions{
		Title:   "test-restart-race",
		Path:    ".",
		Program: "claude code",
	})
	require.NoError(t, err)
	defer instance.Kill()

	err = instance.Start(true)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Try to restart from multiple goroutines
	errors := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func() {
			errors <- instance.ManualRestart()
		}()
	}

	// Collect results
	successCount := 0
	cooldownErrors := 0
	for i := 0; i < 5; i++ {
		err := <-errors
		if err == nil {
			successCount++
		} else if strings.Contains(err.Error(), "please wait") {
			cooldownErrors++
		}
	}

	// Only one should succeed, others should get cooldown error
	assert.Equal(t, 1, successCount, "Only one restart should succeed")
	assert.Equal(t, 4, cooldownErrors, "Other restarts should fail with cooldown error")
}