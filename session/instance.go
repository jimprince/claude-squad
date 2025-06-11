package session

import (
	"claude-squad/log"
	"claude-squad/session/git"
	"claude-squad/session/tmux"
	"path/filepath"

	"crypto/sha256"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/atotto/clipboard"
)

type Status int

const (
	// Running is the status when the instance is running and claude is working.
	Running Status = iota
	// Ready is if the claude instance is ready to be interacted with (waiting for user input).
	Ready
	// Loading is if the instance is loading (if we are starting it up or something).
	Loading
	// Paused is if the instance is paused (worktree removed but branch preserved).
	Paused
)

// Instance is a running instance of claude code.
type Instance struct {
	// Mutex for thread-safe access to continuous mode fields
	mu sync.RWMutex
	
	// Title is the title of the instance.
	Title string
	// Path is the path to the workspace.
	Path string
	// Branch is the branch of the instance.
	Branch string
	// Status is the status of the instance.
	Status Status
	// Program is the program to run in the instance.
	Program string
	// Height is the height of the instance.
	Height int
	// Width is the width of the instance.
	Width int
	// CreatedAt is the time the instance was created.
	CreatedAt time.Time
	// UpdatedAt is the time the instance was last updated.
	UpdatedAt time.Time
	// AutoYes is true if the instance should automatically press enter when prompted.
	AutoYes bool
	// Prompt is the initial prompt to pass to the instance on startup
	Prompt string

	// DiffStats stores the current git diff statistics
	diffStats *git.DiffStats

	// Watchdog functionality
	// LastActivityTime tracks when the session last had meaningful activity
	LastActivityTime time.Time
	// StallCount tracks how many times we've attempted to recover from stalls
	StallCount int
	// WatchdogEnabled determines if watchdog monitoring is active for this instance
	WatchdogEnabled bool
	// ContinuousMode enables more aggressive watchdog monitoring
	ContinuousMode bool
	// ContinuousModeStartTime tracks when continuous mode was enabled
	ContinuousModeStartTime time.Time
	// ContinuousModeDuration is how long continuous mode should run (0 = indefinite)
	ContinuousModeDuration time.Duration
	// LastContentHash tracks content changes to detect stalls
	lastContentHash string
	// RestartAttempts tracks how many times we've tried to restart this session
	RestartAttempts int
	// LastRestartTime tracks when we last attempted a restart
	LastRestartTime time.Time
	// Cache for formatted duration string
	cachedDurationString string
	cachedDurationTime   time.Time

	// The below fields are initialized upon calling Start().

	started bool
	// tmuxSession is the tmux session for the instance.
	tmuxSession *tmux.TmuxSession
	// gitWorktree is the git worktree for the instance.
	gitWorktree *git.GitWorktree
}

// ToInstanceData converts an Instance to its serializable form
func (i *Instance) ToInstanceData() InstanceData {
	data := InstanceData{
		Title:     i.Title,
		Path:      i.Path,
		Branch:    i.Branch,
		Status:    i.Status,
		Height:    i.Height,
		Width:     i.Width,
		CreatedAt: i.CreatedAt,
		UpdatedAt: time.Now(),
		Program:   i.Program,
		AutoYes:   i.AutoYes,
		WatchdogEnabled: i.WatchdogEnabled,
		ContinuousMode: i.ContinuousMode,
		ContinuousModeStartTime: i.ContinuousModeStartTime,
		ContinuousModeDuration: i.ContinuousModeDuration,
		LastActivityTime: i.LastActivityTime,
		StallCount: i.StallCount,
		RestartAttempts: i.RestartAttempts,
		LastRestartTime: i.LastRestartTime,
	}

	// Only include worktree data if gitWorktree is initialized
	if i.gitWorktree != nil {
		data.Worktree = GitWorktreeData{
			RepoPath:      i.gitWorktree.GetRepoPath(),
			WorktreePath:  i.gitWorktree.GetWorktreePath(),
			SessionName:   i.Title,
			BranchName:    i.gitWorktree.GetBranchName(),
			BaseCommitSHA: i.gitWorktree.GetBaseCommitSHA(),
		}
	}

	// Only include diff stats if they exist
	if i.diffStats != nil {
		data.DiffStats = DiffStatsData{
			Added:   i.diffStats.Added,
			Removed: i.diffStats.Removed,
			Content: i.diffStats.Content,
		}
	}

	return data
}

// FromInstanceData creates a new Instance from serialized data
func FromInstanceData(data InstanceData) (*Instance, error) {
	instance := &Instance{
		Title:     data.Title,
		Path:      data.Path,
		Branch:    data.Branch,
		Status:    data.Status,
		Height:    data.Height,
		Width:     data.Width,
		CreatedAt: data.CreatedAt,
		UpdatedAt: data.UpdatedAt,
		Program:   data.Program,
		WatchdogEnabled: data.WatchdogEnabled,
		ContinuousMode: data.ContinuousMode,
		ContinuousModeStartTime: data.ContinuousModeStartTime,
		ContinuousModeDuration: data.ContinuousModeDuration,
		LastActivityTime: data.LastActivityTime,
		StallCount: data.StallCount,
		RestartAttempts: data.RestartAttempts,
		LastRestartTime: data.LastRestartTime,
		gitWorktree: git.NewGitWorktreeFromStorage(
			data.Worktree.RepoPath,
			data.Worktree.WorktreePath,
			data.Worktree.SessionName,
			data.Worktree.BranchName,
			data.Worktree.BaseCommitSHA,
		),
		diffStats: &git.DiffStats{
			Added:   data.DiffStats.Added,
			Removed: data.DiffStats.Removed,
			Content: data.DiffStats.Content,
		},
	}

	if instance.Paused() {
		instance.started = true
		instance.tmuxSession = tmux.NewTmuxSession(instance.Title, instance.Program)
	} else {
		if err := instance.Start(false); err != nil {
			return nil, err
		}
	}

	return instance, nil
}

// Options for creating a new instance
type InstanceOptions struct {
	// Title is the title of the instance.
	Title string
	// Path is the path to the workspace.
	Path string
	// Program is the program to run in the instance (e.g. "claude", "aider --model ollama_chat/gemma3:1b")
	Program string
	// If AutoYes is true, then
	AutoYes bool
}

func NewInstance(opts InstanceOptions) (*Instance, error) {
	t := time.Now()

	// Convert path to absolute
	absPath, err := filepath.Abs(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	return &Instance{
		Title:     opts.Title,
		Status:    Ready,
		Path:      absPath,
		Program:   opts.Program,
		Height:    0,
		Width:     0,
		CreatedAt: t,
		UpdatedAt: t,
		AutoYes:   false,
	}, nil
}

func (i *Instance) RepoName() (string, error) {
	if !i.started {
		return "", fmt.Errorf("cannot get repo name for instance that has not been started")
	}
	return i.gitWorktree.GetRepoName(), nil
}

func (i *Instance) SetStatus(status Status) {
	i.Status = status
}

// firstTimeSetup is true if this is a new instance. Otherwise, it's one loaded from storage.
func (i *Instance) Start(firstTimeSetup bool) error {
	if i.Title == "" {
		return fmt.Errorf("instance title cannot be empty")
	}

	tmuxSession := tmux.NewTmuxSession(i.Title, i.Program)
	i.tmuxSession = tmuxSession

	if firstTimeSetup {
		gitWorktree, branchName, err := git.NewGitWorktree(i.Path, i.Title)
		if err != nil {
			return fmt.Errorf("failed to create git worktree: %w", err)
		}
		i.gitWorktree = gitWorktree
		i.Branch = branchName
	}

	// Setup error handler to cleanup resources on any error
	var setupErr error
	defer func() {
		if setupErr != nil {
			if cleanupErr := i.Kill(); cleanupErr != nil {
				setupErr = fmt.Errorf("%v (cleanup error: %v)", setupErr, cleanupErr)
			}
		} else {
			i.started = true
			// Initialize watchdog for restored instances if enabled
			if i.WatchdogEnabled {
				i.InitializeWatchdog(true)
			}
		}
	}()

	if !firstTimeSetup {
		// Reuse existing session
		if err := tmuxSession.Restore(); err != nil {
			setupErr = fmt.Errorf("failed to restore existing session: %w", err)
			return setupErr
		}
	} else {
		// Setup git worktree first
		if err := i.gitWorktree.Setup(); err != nil {
			setupErr = fmt.Errorf("failed to setup git worktree: %w", err)
			return setupErr
		}

		// Create new session
		if err := i.tmuxSession.Start(i.gitWorktree.GetWorktreePath()); err != nil {
			// Cleanup git worktree if tmux session creation fails
			if cleanupErr := i.gitWorktree.Cleanup(); cleanupErr != nil {
				err = fmt.Errorf("%v (cleanup error: %v)", err, cleanupErr)
			}
			setupErr = fmt.Errorf("failed to start new session: %w", err)
			return setupErr
		}
	}

	i.SetStatus(Running)

	return nil
}

// Kill terminates the instance and cleans up all resources
func (i *Instance) Kill() error {
	if !i.started {
		// If instance was never started, just return success
		return nil
	}

	var errs []error

	// Always try to cleanup both resources, even if one fails
	// Clean up tmux session first since it's using the git worktree
	if i.tmuxSession != nil {
		if err := i.tmuxSession.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close tmux session: %w", err))
		}
	}

	// Then clean up git worktree
	if i.gitWorktree != nil {
		if err := i.gitWorktree.Cleanup(); err != nil {
			errs = append(errs, fmt.Errorf("failed to cleanup git worktree: %w", err))
		}
	}

	return i.combineErrors(errs)
}

// combineErrors combines multiple errors into a single error
func (i *Instance) combineErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}

	errMsg := "multiple cleanup errors occurred:"
	for _, err := range errs {
		errMsg += "\n  - " + err.Error()
	}
	return fmt.Errorf("%s", errMsg)
}

// Close is an alias for Kill to maintain backward compatibility
func (i *Instance) Close() error {
	if !i.started {
		return fmt.Errorf("cannot close instance that has not been started")
	}
	return i.Kill()
}

func (i *Instance) Preview() (string, error) {
	if !i.started || i.Status == Paused {
		return "", nil
	}
	return i.tmuxSession.CapturePaneContent()
}

func (i *Instance) HasUpdated() (updated bool, hasPrompt bool) {
	if !i.started {
		return false, false
	}
	return i.tmuxSession.HasUpdated()
}

// TapEnter sends an enter key press to the tmux session if AutoYes is enabled.
func (i *Instance) TapEnter() {
	if !i.started || !i.AutoYes {
		return
	}
	if err := i.tmuxSession.TapEnter(); err != nil {
		log.ErrorLog.Printf("error tapping enter: %v", err)
	}
}

func (i *Instance) Attach() (chan struct{}, error) {
	if !i.started {
		return nil, fmt.Errorf("cannot attach instance that has not been started")
	}
	return i.tmuxSession.Attach()
}

func (i *Instance) SetPreviewSize(width, height int) error {
	if !i.started || i.Status == Paused {
		return fmt.Errorf("cannot set preview size for instance that has not been started or " +
			"is paused")
	}
	return i.tmuxSession.SetDetachedSize(width, height)
}

// GetGitWorktree returns the git worktree for the instance
func (i *Instance) GetGitWorktree() (*git.GitWorktree, error) {
	if !i.started {
		return nil, fmt.Errorf("cannot get git worktree for instance that has not been started")
	}
	return i.gitWorktree, nil
}

func (i *Instance) Started() bool {
	return i.started
}

// SetTitle sets the title of the instance. Returns an error if the instance has started.
// We cant change the title once it's been used for a tmux session etc.
func (i *Instance) SetTitle(title string) error {
	if i.started {
		return fmt.Errorf("cannot change title of a started instance")
	}
	i.Title = title
	return nil
}

func (i *Instance) Paused() bool {
	return i.Status == Paused
}

// TmuxAlive returns true if the tmux session is alive. This is a sanity check before attaching.
func (i *Instance) TmuxAlive() bool {
	return i.tmuxSession.DoesSessionExist()
}

// Pause stops the tmux session and removes the worktree, preserving the branch
func (i *Instance) Pause() error {
	if !i.started {
		return fmt.Errorf("cannot pause instance that has not been started")
	}
	if i.Status == Paused {
		return fmt.Errorf("instance is already paused")
	}

	var errs []error

	// Check if there are any changes to commit
	if dirty, err := i.gitWorktree.IsDirty(); err != nil {
		errs = append(errs, fmt.Errorf("failed to check if worktree is dirty: %w", err))
		log.ErrorLog.Print(err)
	} else if dirty {
		// Commit changes with timestamp
		commitMsg := fmt.Sprintf("[claudesquad] update from '%s' on %s (paused)", i.Title, time.Now().Format(time.RFC822))
		if err := i.gitWorktree.PushChanges(commitMsg, false); err != nil {
			errs = append(errs, fmt.Errorf("failed to commit changes: %w", err))
			log.ErrorLog.Print(err)
			// Return early if we can't commit changes to avoid corrupted state
			return i.combineErrors(errs)
		}
	}

	// Close tmux session first since it's using the git worktree
	if err := i.tmuxSession.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close tmux session: %w", err))
		log.ErrorLog.Print(err)
		// Return early if we can't close tmux to avoid corrupted state
		return i.combineErrors(errs)
	}

	// Check if worktree exists before trying to remove it
	if _, err := os.Stat(i.gitWorktree.GetWorktreePath()); err == nil {
		// Remove worktree but keep branch
		if err := i.gitWorktree.Remove(); err != nil {
			errs = append(errs, fmt.Errorf("failed to remove git worktree: %w", err))
			log.ErrorLog.Print(err)
			return i.combineErrors(errs)
		}

		// Only prune if remove was successful
		if err := i.gitWorktree.Prune(); err != nil {
			errs = append(errs, fmt.Errorf("failed to prune git worktrees: %w", err))
			log.ErrorLog.Print(err)
			return i.combineErrors(errs)
		}
	}

	if err := i.combineErrors(errs); err != nil {
		log.ErrorLog.Print(err)
		return err
	}

	i.SetStatus(Paused)
	_ = clipboard.WriteAll(i.gitWorktree.GetBranchName())
	return nil
}

// Resume recreates the worktree and restarts the tmux session
func (i *Instance) Resume() error {
	if !i.started {
		return fmt.Errorf("cannot resume instance that has not been started")
	}
	if i.Status != Paused {
		return fmt.Errorf("can only resume paused instances")
	}

	// Check if branch is checked out
	if checked, err := i.gitWorktree.IsBranchCheckedOut(); err != nil {
		log.ErrorLog.Print(err)
		return fmt.Errorf("failed to check if branch is checked out: %w", err)
	} else if checked {
		return fmt.Errorf("cannot resume: branch is checked out, please switch to a different branch")
	}

	// Setup git worktree
	if err := i.gitWorktree.Setup(); err != nil {
		log.ErrorLog.Print(err)
		return fmt.Errorf("failed to setup git worktree: %w", err)
	}

	// Create new tmux session
	if err := i.tmuxSession.Start(i.gitWorktree.GetWorktreePath()); err != nil {
		log.ErrorLog.Print(err)
		// Cleanup git worktree if tmux session creation fails
		if cleanupErr := i.gitWorktree.Cleanup(); cleanupErr != nil {
			err = fmt.Errorf("%v (cleanup error: %v)", err, cleanupErr)
			log.ErrorLog.Print(err)
		}
		return fmt.Errorf("failed to start new session: %w", err)
	}

	i.SetStatus(Running)
	return nil
}

// UpdateDiffStats updates the git diff statistics for this instance
func (i *Instance) UpdateDiffStats() error {
	if !i.started {
		i.diffStats = nil
		return nil
	}

	if i.Status == Paused {
		// Keep the previous diff stats if the instance is paused
		return nil
	}

	stats := i.gitWorktree.Diff()
	if stats.Error != nil {
		if strings.Contains(stats.Error.Error(), "base commit SHA not set") {
			// Worktree is not fully set up yet, not an error
			i.diffStats = nil
			return nil
		}
		return fmt.Errorf("failed to get diff stats: %w", stats.Error)
	}

	i.diffStats = stats
	return nil
}

// GetDiffStats returns the current git diff statistics
func (i *Instance) GetDiffStats() *git.DiffStats {
	return i.diffStats
}

// SendPrompt sends a prompt to the tmux session
func (i *Instance) SendPrompt(prompt string) error {
	if !i.started {
		return fmt.Errorf("instance not started")
	}
	if i.tmuxSession == nil {
		return fmt.Errorf("tmux session not initialized")
	}
	if err := i.tmuxSession.SendKeys(prompt); err != nil {
		return fmt.Errorf("error sending keys to tmux session: %w", err)
	}

	// Brief pause to prevent carriage return from being interpreted as newline
	time.Sleep(100 * time.Millisecond)
	if err := i.tmuxSession.TapEnter(); err != nil {
		return fmt.Errorf("error tapping enter: %w", err)
	}

	return nil
}

// Watchdog functionality

// DetectStall checks if the session appears to be stalled based on content and timing
func (i *Instance) DetectStall(stallTimeoutSeconds, continuousModeTimeoutSeconds int) bool {
	if !i.started || i.Status == Paused || !i.WatchdogEnabled {
		return false
	}

	// Get current content
	content, err := i.tmuxSession.CapturePaneContent()
	if err != nil {
		log.WarningLog.Printf("failed to capture pane content for stall detection: %v", err)
		return false
	}

	// Check for common stall patterns in Claude Code
	stallPatterns := []string{
		"I need confirmation to proceed",
		"Should I continue?", 
		"Do you want me to continue?",
		"Would you like me to proceed?",
		"Press any key to continue",
		"Continue? (y/n)",
		"Proceed? (y/n)",
		"[y/n]",
		"(y/n)",
		"Type 'continue' to proceed",
		"waiting for confirmation",
		"Claude Code is waiting",
		"Do you want to proceed?",
		"1. Yes",
		"> 1. Yes",
	}

	// Claude Code specific completion patterns
	completionPatterns := []string{
		"What's Working Now:",
		"The medical dictation app now has all essential features implemented",
		"all essential features implemented and working",
		"auto-accept edits on",
		"Context left until auto-compact:",
		"All UI elements functional and responsive",
		"Settings management implemented",
		"workflow complete",
	}

	hasStallPattern := false
	hasCompletionPattern := false
	contentLower := strings.ToLower(content)
	
	// First check explicit patterns
	for _, pattern := range stallPatterns {
		if strings.Contains(contentLower, strings.ToLower(pattern)) {
			hasStallPattern = true
			break
		}
	}
	
	// Check for completion patterns (Claude Code specific)
	for _, pattern := range completionPatterns {
		if strings.Contains(contentLower, strings.ToLower(pattern)) {
			hasCompletionPattern = true
			break
		}
	}
	
	// Also check for common confirmation prompt structures
	if !hasStallPattern {
		// Check for "Do you want to [action]?" pattern
		if strings.Contains(contentLower, "do you want to") && strings.Contains(contentLower, "?") {
			hasStallPattern = true
		}
		// Check for numbered options with Yes/No
		if strings.Contains(contentLower, "1.") && strings.Contains(contentLower, "yes") &&
		   strings.Contains(contentLower, "2.") && strings.Contains(contentLower, "no") {
			hasStallPattern = true
		}
		// Check for (y/n) or similar patterns anywhere in content
		if strings.Contains(contentLower, "(y/n)") || strings.Contains(contentLower, "(yes/no)") ||
		   strings.Contains(contentLower, "[y/n]") || strings.Contains(contentLower, "(esc)") {
			hasStallPattern = true
		}
		// Check for the terminal prompt at the bottom
		if strings.Contains(content, "\n> ") || strings.HasSuffix(strings.TrimSpace(content), ">") {
			hasStallPattern = true
		}
	}

	// For continuous mode, use different detection logic
	if i.ContinuousMode {
		// In continuous mode, we care more about completion patterns and prompts
		if hasCompletionPattern || hasStallPattern {
			// Check if we've been in this state for at least 2 seconds
			timeSinceActivity := time.Since(i.LastActivityTime)
			stabilityThreshold := 2 * time.Second
			
			// Use normalized content for comparison (strip timestamps and dynamic elements)
			normalizedContent := i.normalizeContent(content)
			normalizedHash := i.hashContent(normalizedContent)
			
			// If normalized content hasn't changed for stability threshold, it's a stall
			if i.lastContentHash == normalizedHash && timeSinceActivity > stabilityThreshold {
				log.WarningLog.Printf("continuous mode stall detected for instance '%s': completion_pattern=%v, stall_pattern=%v, stable_for=%v", 
					i.Title, hasCompletionPattern, hasStallPattern, timeSinceActivity)
				return true
			}
			
			// Update hash if it changed
			if i.lastContentHash != normalizedHash {
				i.lastContentHash = normalizedHash
				i.LastActivityTime = time.Now()
			}
			
			return false
		}
	}

	// Regular mode detection (unchanged logic for backward compatibility)
	// Calculate content hash to detect if content has changed
	currentHash := i.hashContent(content)
	contentUnchanged := i.lastContentHash == currentHash
	
	// Update hash for next check
	i.lastContentHash = currentHash

	// If content changed, update last activity time
	if !contentUnchanged {
		i.LastActivityTime = time.Now()
		return false
	}

	// Check if we've been inactive for too long
	timeSinceActivity := time.Since(i.LastActivityTime)
	
	// Use continuous mode timeout if enabled, otherwise use normal timeout
	timeoutSeconds := stallTimeoutSeconds
	if i.ContinuousMode {
		timeoutSeconds = continuousModeTimeoutSeconds
	}
	stallTimeout := time.Duration(timeoutSeconds) * time.Second
	
	// Only consider it a stall if:
	// 1. We have a stall pattern in the content, OR
	// 2. We've had no activity for the configured timeout
	if hasStallPattern || timeSinceActivity > stallTimeout {
		log.WarningLog.Printf("stall detected for instance '%s': pattern=%v, inactive_for=%v", 
			i.Title, hasStallPattern, timeSinceActivity)
		return true
	}

	return false
}

// normalizeContent strips out dynamic elements like timestamps and cursor positions
func (i *Instance) normalizeContent(content string) string {
	// Remove ANSI escape codes (colors, cursor movements, etc)
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	normalized := ansiRegex.ReplaceAllString(content, "")
	
	// Remove timestamp patterns (common formats)
	// Example: 13:54:48, 2024-01-15, etc.
	timeRegex := regexp.MustCompile(`\d{1,2}:\d{2}:\d{2}|\d{4}-\d{2}-\d{2}`)
	normalized = timeRegex.ReplaceAllString(normalized, "")
	
	// Remove percentage patterns that might change (like "28%")
	percentRegex := regexp.MustCompile(`\d+%`)
	normalized = percentRegex.ReplaceAllString(normalized, "")
	
	// Normalize whitespace
	normalized = strings.TrimSpace(normalized)
	
	return normalized
}

// hashContent creates a hash of the content
func (i *Instance) hashContent(content string) string {
	hasher := sha256.New()
	hasher.Write([]byte(content))
	return fmt.Sprintf("%x", hasher.Sum(nil))
}

// InjectContinue attempts to send commands to unstall the session
func (i *Instance) InjectContinue(continueCommands []string) error {
	if !i.started || i.Status == Paused {
		return fmt.Errorf("cannot inject continue: instance not running")
	}

	// Default continue commands if none provided
	if len(continueCommands) == 0 {
		continueCommands = []string{
			"1",      // For numbered prompts
			"continue",
			"yes", 
			"y",
			"proceed",
			"\n", // Just press enter
		}
	}

	log.WarningLog.Printf("attempting to unstall instance '%s' (attempt %d)", i.Title, i.StallCount+1)

	// Get current content to make intelligent decision
	content, err := i.tmuxSession.CapturePaneContent()
	if err == nil {
		contentLower := strings.ToLower(content)
		
		// Special handling for continuous mode with Claude Code
		if i.ContinuousMode {
			// If Claude Code is showing completion status, send /continuous command
			if strings.Contains(contentLower, "what's working now:") ||
			   strings.Contains(contentLower, "all essential features implemented") ||
			   strings.Contains(contentLower, "auto-accept edits on") {
				// Build the continuous mode message
				var continuousMsg string
				remaining := i.GetContinuousModeTimeRemaining()
				if remaining > 0 {
					// Format remaining time nicely
					hours := int(remaining.Hours())
					minutes := int(remaining.Minutes()) % 60
					seconds := int(remaining.Seconds()) % 60
					
					if hours > 0 {
						continuousMsg = fmt.Sprintf("/continuous You're in continuous mode. Time remaining: %dh %dm %ds. Keep working on any remaining tasks or improvements.", hours, minutes, seconds)
					} else if minutes > 0 {
						continuousMsg = fmt.Sprintf("/continuous You're in continuous mode. Time remaining: %dm %ds. Keep working on any remaining tasks or improvements.", minutes, seconds)
					} else {
						continuousMsg = fmt.Sprintf("/continuous You're in continuous mode. Time remaining: %ds. Keep working on any remaining tasks or improvements.", seconds)
					}
				} else {
					continuousMsg = "/continuous You're in continuous mode (indefinite duration). Keep working on any remaining tasks or improvements. The system will auto-continue when you complete each task."
				}
				
				continueCommands = []string{continuousMsg, "continue", "\n"}
				log.InfoLog.Printf("continuous mode: detected Claude Code completion state, sending continuous mode message")
			}
		}
		
		// If there's a "don't ask again" option, prefer that
		if strings.Contains(contentLower, "don't ask again") {
			// Usually option 2 for "don't ask again"
			if strings.Contains(contentLower, "2.") && strings.Contains(contentLower, "don't ask again") {
				continueCommands = []string{"2", "yes", "1", "y", "continue"}
			}
		}
		
		// If it's asking to create a file, might want to say yes
		if strings.Contains(contentLower, "do you want to create") {
			continueCommands = []string{"1", "yes", "y"}
		}
	}

	// Try each continue command
	for _, cmd := range continueCommands {
		if err := i.SendPrompt(cmd); err != nil {
			log.WarningLog.Printf("failed to send continue command '%s': %v", cmd, err)
			continue
		}
		
		// Increment stall count and update activity time
		i.StallCount++
		i.LastActivityTime = time.Now()
		
		log.WarningLog.Printf("sent continue command '%s' to instance '%s'", cmd, i.Title)
		return nil
	}

	return fmt.Errorf("failed to send any continue commands to instance '%s'", i.Title)
}

// InitializeWatchdog sets up the watchdog state for a new or resumed instance
func (i *Instance) InitializeWatchdog(enabled bool) {
	i.WatchdogEnabled = enabled
	i.LastActivityTime = time.Now()
	i.StallCount = 0
	i.lastContentHash = ""
}

// GetWatchdogStatus returns current watchdog state information
func (i *Instance) GetWatchdogStatus() (enabled bool, lastActivity time.Time, stallCount int) {
	return i.WatchdogEnabled, i.LastActivityTime, i.StallCount
}

// ToggleContinuousMode toggles continuous mode for more aggressive monitoring
func (i *Instance) ToggleContinuousMode() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	
	i.ContinuousMode = !i.ContinuousMode
	if i.ContinuousMode {
		i.ContinuousModeStartTime = time.Now()
		// Set default duration if not specified (0 = indefinite)
		if i.ContinuousModeDuration == 0 {
			i.ContinuousModeDuration = 0 // Run indefinitely
		}
	} else {
		// Clear the start time when disabling
		i.ContinuousModeStartTime = time.Time{}
	}
	if log.WarningLog != nil {
		log.WarningLog.Printf("continuous mode %s for instance '%s'", 
			map[bool]string{true: "enabled", false: "disabled"}[i.ContinuousMode], i.Title)
	}
	return i.ContinuousMode
}

// SetContinuousModeDuration sets the duration for continuous mode
func (i *Instance) SetContinuousModeDuration(duration time.Duration) {
	i.mu.Lock()
	defer i.mu.Unlock()
	
	i.ContinuousModeDuration = duration
	if i.ContinuousMode {
		// Reset start time when duration changes
		i.ContinuousModeStartTime = time.Now()
	}
}

// IsContinuousMode returns whether continuous mode is enabled
func (i *Instance) IsContinuousMode() bool {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.ContinuousMode
}

// DisableContinuousMode safely disables continuous mode
func (i *Instance) DisableContinuousMode() {
	i.mu.Lock()
	defer i.mu.Unlock()
	
	if i.ContinuousMode {
		i.ContinuousMode = false
		i.ContinuousModeStartTime = time.Time{}
		if log.InfoLog != nil {
			log.InfoLog.Printf("continuous mode disabled for instance '%s'", i.Title)
		}
	}
}

// GetContinuousModeTimeRemaining returns the time remaining in continuous mode
// Returns 0 if continuous mode is indefinite or not enabled
func (i *Instance) GetContinuousModeTimeRemaining() time.Duration {
	i.mu.RLock()
	defer i.mu.RUnlock()
	
	if !i.ContinuousMode || i.ContinuousModeDuration == 0 {
		return 0
	}
	
	elapsed := time.Since(i.ContinuousModeStartTime)
	remaining := i.ContinuousModeDuration - elapsed
	
	if remaining < 0 {
		return 0
	}
	return remaining
}

// GetContinuousModeTimeRemainingFormatted returns a formatted string of remaining time
// Uses caching to avoid repeated formatting
func (i *Instance) GetContinuousModeTimeRemainingFormatted() string {
	i.mu.RLock()
	defer i.mu.RUnlock()
	
	if !i.ContinuousMode {
		return ""
	}
	
	// Check cache validity (update every second)
	if time.Since(i.cachedDurationTime) < time.Second && i.cachedDurationString != "" {
		return i.cachedDurationString
	}
	
	// Need to temporarily unlock for GetContinuousModeTimeRemaining call
	i.mu.RUnlock()
	remaining := i.GetContinuousModeTimeRemaining()
	i.mu.RLock()
	
	if remaining == 0 {
		i.cachedDurationString = ""
		i.cachedDurationTime = time.Now()
		return ""
	}
	
	// Format remaining time
	hours := int(remaining.Hours())
	minutes := int(remaining.Minutes()) % 60
	seconds := int(remaining.Seconds()) % 60
	
	var timeStr string
	if hours > 0 {
		timeStr = fmt.Sprintf("%dh%dm", hours, minutes)
	} else if minutes > 0 {
		timeStr = fmt.Sprintf("%dm%ds", minutes, seconds)
	} else {
		timeStr = fmt.Sprintf("%ds", seconds)
	}
	
	// Cache the result
	i.cachedDurationString = timeStr
	i.cachedDurationTime = time.Now()
	
	return timeStr
}

// ManualRestart allows user to manually restart Claude Code with session restore
func (i *Instance) ManualRestart() error {
	// Acquire mutex to prevent concurrent restarts
	i.mu.Lock()
	defer i.mu.Unlock()
	
	// Validate state
	if !i.started {
		return fmt.Errorf("cannot restart: instance not started")
	}
	if i.Status == Paused {
		return fmt.Errorf("cannot restart: instance is paused")
	}
	if !strings.Contains(strings.ToLower(i.Program), "claude") {
		return fmt.Errorf("restart only supported for Claude Code sessions")
	}

	// Check if we're already restarting
	const restartCooldown = 10 * time.Second
	if time.Since(i.LastRestartTime) < restartCooldown {
		return fmt.Errorf("please wait %v before restarting again", 
			restartCooldown - time.Since(i.LastRestartTime))
	}

	// Save current state
	i.LastRestartTime = time.Now()
	i.RestartAttempts++

	// Log the restart
	log.InfoLog.Printf("user initiated restart for instance '%s'", i.Title)

	// Perform the restart
	if err := i.restartClaudeWithResume(); err != nil {
		return fmt.Errorf("failed to restart Claude Code: %w", err)
	}

	return nil
}

// DetectCrashAndRestart detects if Claude Code crashed and restarts it with --resume
func (i *Instance) DetectCrashAndRestart() bool {
	if !i.started || i.Status == Paused {
		return false
	}

	// Only handle Claude Code crashes
	if !strings.Contains(strings.ToLower(i.Program), "claude") {
		return false
	}

	// Check if we've tried too many restarts recently
	const maxRestartAttempts = 3
	const restartCooldown = 5 * time.Minute
	
	if i.RestartAttempts >= maxRestartAttempts {
		timeSinceLastRestart := time.Since(i.LastRestartTime)
		if timeSinceLastRestart < restartCooldown {
			// Too many restart attempts, give up for now
			return false
		}
		// Reset counter after cooldown
		i.RestartAttempts = 0
	}

	// Try to capture pane content - if this fails, the session likely crashed
	_, err := i.tmuxSession.CapturePaneContent()
	if err != nil {
		// Check if it's an exit status 1 error (session crashed)
		if strings.Contains(err.Error(), "exit status 1") || 
		   strings.Contains(err.Error(), "no session found") ||
		   strings.Contains(err.Error(), "can't find session") {
			
			log.WarningLog.Printf("detected crashed Claude Code session '%s' (attempt %d/%d)", 
				i.Title, i.RestartAttempts+1, maxRestartAttempts)
			
			i.RestartAttempts++
			i.LastRestartTime = time.Now()
			
			if err := i.restartClaudeWithResume(); err != nil {
				log.ErrorLog.Printf("failed to restart Claude Code session '%s': %v", i.Title, err)
				return false
			}
			return true
		}
	}
	return false
}

// restartClaudeWithResume restarts Claude Code with --resume and the session ID
func (i *Instance) restartClaudeWithResume() error {
	// Save state before restart
	wasInContinuousMode := i.ContinuousMode
	continuousModeStartTime := i.ContinuousModeStartTime
	continuousModeDuration := i.ContinuousModeDuration
	
	// First, get the Claude session list to find the session number
	sessionNumber, err := i.findClaudeSessionNumber()
	if err != nil {
		return fmt.Errorf("failed to find Claude session number: %w", err)
	}

	// Gracefully close the existing tmux session if it's still running
	if i.tmuxSession != nil {
		// Try to send exit command first for graceful shutdown
		_ = i.tmuxSession.SendKeys("exit")
		time.Sleep(500 * time.Millisecond)
		
		if err := i.tmuxSession.Close(); err != nil {
			log.ErrorLog.Printf("failed to close tmux session during restart: %v", err)
		}
	}

	// Create resume command with session number
	baseProgram := strings.Split(i.Program, " ")[0] // Get just "claude" without args
	resumeProgram := fmt.Sprintf("%s -r %s", baseProgram, sessionNumber)

	log.WarningLog.Printf("restarting with command: %s", resumeProgram)

	// Create new tmux session with resume command
	tmuxSession := tmux.NewTmuxSession(i.Title, resumeProgram)
	i.tmuxSession = tmuxSession

	// Start the new session in the existing worktree
	if err := i.tmuxSession.Start(i.gitWorktree.GetWorktreePath()); err != nil {
		return fmt.Errorf("failed to restart Claude Code with --resume: %w", err)
	}

	log.WarningLog.Printf("successfully restarted Claude Code session '%s' with session %s", i.Title, sessionNumber)
	
	// Wait for Claude to be ready with exponential backoff
	maxRetries := 5
	for retry := 0; retry < maxRetries; retry++ {
		time.Sleep(time.Duration(1<<uint(retry)) * time.Second) // 1s, 2s, 4s, 8s, 16s
		
		// Try to capture content to see if Claude is ready
		if content, err := i.tmuxSession.CapturePaneContent(); err == nil {
			contentLower := strings.ToLower(content)
			// Check if Claude is ready (shows prompt or waiting)
			if strings.Contains(contentLower, "claude") || 
			   strings.Contains(contentLower, ">") ||
			   strings.Contains(contentLower, "continue") {
				// Claude is ready, send continue
				if err := i.SendPrompt("continue"); err != nil {
					log.ErrorLog.Printf("failed to send initial continue after restart: %v", err)
				} else {
					log.InfoLog.Printf("sent initial 'continue' to resumed session '%s'", i.Title)
				}
				break
			}
		}
		
		if retry == maxRetries-1 {
			log.WarningLog.Printf("Claude may not be fully ready after restart, proceeding anyway")
		}
	}
	
	// Reset activity tracking for fresh monitoring
	i.LastActivityTime = time.Now()
	i.lastContentHash = ""
	
	// Restore continuous mode state if it was enabled
	if wasInContinuousMode {
		i.ContinuousMode = true
		i.ContinuousModeStartTime = continuousModeStartTime
		i.ContinuousModeDuration = continuousModeDuration
		log.InfoLog.Printf("restored continuous mode state after restart")
	}
	
	return nil
}

// findClaudeSessionNumber finds the Claude session number for this workspace
func (i *Instance) findClaudeSessionNumber() (string, error) {
	// Claude doesn't have a --list command, so go directly to file-based discovery
	return i.findClaudeSessionFromFiles()
}

// findClaudeSessionFromFiles finds Claude session by looking at session files directly
func (i *Instance) findClaudeSessionFromFiles() (string, error) {
	// Claude sessions are stored in ~/.claude/projects/
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	projectsDir := filepath.Join(homeDir, ".claude", "projects")
	
	// Use the worktree path since Claude was run from there
	currentDir := i.gitWorktree.GetWorktreePath()
	// Remove leading slash and replace all / with -
	dirKey := strings.TrimPrefix(currentDir, "/")
	dirKey = strings.ReplaceAll(dirKey, "/", "-")
	
	// Look for session files in the project directory (not in a sessions subdirectory)
	sessionDir := filepath.Join(projectsDir, dirKey)
	
	log.InfoLog.Printf("looking for sessions in: %s", sessionDir)
	
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		log.WarningLog.Printf("failed to read session directory %s: %v", sessionDir, err)
		return "", fmt.Errorf("failed to read session directory %s: %w", sessionDir, err)
	}

	// Find the most recent session
	var mostRecentSession string
	var mostRecentTime time.Time
	
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".jsonl") {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			
			if info.ModTime().After(mostRecentTime) {
				mostRecentTime = info.ModTime()
				// Remove .jsonl extension to get session ID
				mostRecentSession = strings.TrimSuffix(entry.Name(), ".jsonl")
			}
		}
	}

	if mostRecentSession == "" {
		return "", fmt.Errorf("no Claude session files found in %s", sessionDir)
	}

	log.InfoLog.Printf("found Claude session from files: %s", mostRecentSession)
	return mostRecentSession, nil
}
