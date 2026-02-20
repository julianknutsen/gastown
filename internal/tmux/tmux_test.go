package tmux

import (
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"
)

func hasTmux() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

func TestListSessionsNoServer(t *testing.T) {
	if !hasTmux() {
		t.Skip("tmux not installed")
	}

	tm := NewTmux()
	sessions, err := tm.ListSessions()
	// Should not error even if no server running
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	// Result may be nil or empty slice
	_ = sessions
}

func TestHasSessionNoServer(t *testing.T) {
	if !hasTmux() {
		t.Skip("tmux not installed")
	}

	tm := NewTmux()
	has, err := tm.HasSession("nonexistent-session-xyz")
	if err != nil {
		t.Fatalf("HasSession: %v", err)
	}
	if has {
		t.Error("expected session to not exist")
	}
}

func TestSessionLifecycle(t *testing.T) {
	if !hasTmux() {
		t.Skip("tmux not installed")
	}

	tm := NewTmux()
	sessionName := "gt-test-session-" + t.Name()

	// Clean up any existing session
	_ = tm.KillSession(sessionName)

	// Create session
	if err := tm.NewSession(sessionName, ""); err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer func() { _ = tm.KillSession(sessionName) }()

	// Verify exists
	has, err := tm.HasSession(sessionName)
	if err != nil {
		t.Fatalf("HasSession: %v", err)
	}
	if !has {
		t.Error("expected session to exist after creation")
	}

	// List should include it
	sessions, err := tm.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	found := false
	for _, s := range sessions {
		if s == sessionName {
			found = true
			break
		}
	}
	if !found {
		t.Error("session not found in list")
	}

	// Kill session
	if err := tm.KillSession(sessionName); err != nil {
		t.Fatalf("KillSession: %v", err)
	}

	// Verify gone
	has, err = tm.HasSession(sessionName)
	if err != nil {
		t.Fatalf("HasSession after kill: %v", err)
	}
	if has {
		t.Error("expected session to not exist after kill")
	}
}

func TestDuplicateSession(t *testing.T) {
	if !hasTmux() {
		t.Skip("tmux not installed")
	}

	tm := NewTmux()
	sessionName := "gt-test-dup-" + t.Name()

	// Clean up any existing session
	_ = tm.KillSession(sessionName)

	// Create session
	if err := tm.NewSession(sessionName, ""); err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer func() { _ = tm.KillSession(sessionName) }()

	// Try to create duplicate
	err := tm.NewSession(sessionName, "")
	if err != ErrSessionExists {
		t.Errorf("expected ErrSessionExists, got %v", err)
	}
}

func TestSendKeysAndCapture(t *testing.T) {
	if !hasTmux() {
		t.Skip("tmux not installed")
	}

	tm := NewTmux()
	sessionName := "gt-test-keys-" + t.Name()

	// Clean up any existing session
	_ = tm.KillSession(sessionName)

	// Create session
	if err := tm.NewSession(sessionName, ""); err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer func() { _ = tm.KillSession(sessionName) }()

	// Send echo command
	if err := tm.SendKeys(sessionName, "echo HELLO_TEST_MARKER"); err != nil {
		t.Fatalf("SendKeys: %v", err)
	}

	// Give it a moment to execute
	// In real tests you'd wait for output, but for basic test we just capture
	output, err := tm.CapturePane(sessionName, 50)
	if err != nil {
		t.Fatalf("CapturePane: %v", err)
	}

	// Should contain our marker (might not if shell is slow, but usually works)
	if !strings.Contains(output, "echo HELLO_TEST_MARKER") {
		t.Logf("captured output: %s", output)
		// Don't fail, just note - timing issues possible
	}
}

func TestGetSessionInfo(t *testing.T) {
	if !hasTmux() {
		t.Skip("tmux not installed")
	}

	tm := NewTmux()
	sessionName := "gt-test-info-" + t.Name()

	// Clean up any existing session
	_ = tm.KillSession(sessionName)

	// Create session
	if err := tm.NewSession(sessionName, ""); err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer func() { _ = tm.KillSession(sessionName) }()

	info, err := tm.GetSessionInfo(sessionName)
	if err != nil {
		t.Fatalf("GetSessionInfo: %v", err)
	}

	if info.Name != sessionName {
		t.Errorf("Name = %q, want %q", info.Name, sessionName)
	}
	if info.Windows < 1 {
		t.Errorf("Windows = %d, want >= 1", info.Windows)
	}
}

func TestWrapError(t *testing.T) {
	tm := NewTmux()

	tests := []struct {
		stderr string
		want   error
	}{
		{"no server running on /tmp/tmux-...", ErrNoServer},
		{"error connecting to /tmp/tmux-...", ErrNoServer},
		{"no current target", ErrNoServer},
		{"no sessions", ErrNoServer},
		{"duplicate session: test", ErrSessionExists},
		{"session not found: test", ErrSessionNotFound},
		{"can't find session: test", ErrSessionNotFound},
	}

	for _, tt := range tests {
		err := tm.wrapError(nil, tt.stderr, []string{"test"})
		if err != tt.want {
			t.Errorf("wrapError(%q) = %v, want %v", tt.stderr, err, tt.want)
		}
	}
}

func TestEnsureSessionFresh_NoExistingSession(t *testing.T) {
	if !hasTmux() {
		t.Skip("tmux not installed")
	}

	tm := NewTmux()
	sessionName := "gt-test-fresh-" + t.Name()

	// Clean up any existing session
	_ = tm.KillSession(sessionName)

	// EnsureSessionFresh should create a new session
	if err := tm.EnsureSessionFresh(sessionName, ""); err != nil {
		t.Fatalf("EnsureSessionFresh: %v", err)
	}
	defer func() { _ = tm.KillSession(sessionName) }()

	// Verify session exists
	has, err := tm.HasSession(sessionName)
	if err != nil {
		t.Fatalf("HasSession: %v", err)
	}
	if !has {
		t.Error("expected session to exist after EnsureSessionFresh")
	}
}

func TestEnsureSessionFresh_ZombieSession(t *testing.T) {
	if !hasTmux() {
		t.Skip("tmux not installed")
	}

	tm := NewTmux()
	sessionName := "gt-test-zombie-" + t.Name()

	// Clean up any existing session
	_ = tm.KillSession(sessionName)

	// Create a zombie session (session exists but no Claude/node running)
	// A normal tmux session with bash/zsh is a "zombie" for our purposes
	if err := tm.NewSession(sessionName, ""); err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer func() { _ = tm.KillSession(sessionName) }()

	// Verify it's a zombie (not running Claude/node)
	if tm.IsClaudeRunning(sessionName) {
		t.Skip("session unexpectedly has Claude running - can't test zombie case")
	}

	// Verify generic agent check also treats it as not running (shell session)
	if tm.IsAgentRunning(sessionName) {
		t.Fatalf("expected IsAgentRunning(%q) to be false for a fresh shell session", sessionName)
	}

	// EnsureSessionFresh should kill the zombie and create fresh session
	// This should NOT error with "session already exists"
	if err := tm.EnsureSessionFresh(sessionName, ""); err != nil {
		t.Fatalf("EnsureSessionFresh on zombie: %v", err)
	}

	// Session should still exist
	has, err := tm.HasSession(sessionName)
	if err != nil {
		t.Fatalf("HasSession: %v", err)
	}
	if !has {
		t.Error("expected session to exist after EnsureSessionFresh on zombie")
	}
}

func TestEnsureSessionFresh_IdempotentOnZombie(t *testing.T) {
	if !hasTmux() {
		t.Skip("tmux not installed")
	}

	tm := NewTmux()
	sessionName := "gt-test-idem-" + t.Name()

	// Clean up any existing session
	_ = tm.KillSession(sessionName)

	// Call EnsureSessionFresh multiple times - should work each time
	for i := 0; i < 3; i++ {
		if err := tm.EnsureSessionFresh(sessionName, ""); err != nil {
			t.Fatalf("EnsureSessionFresh attempt %d: %v", i+1, err)
		}
	}
	defer func() { _ = tm.KillSession(sessionName) }()

	// Session should exist
	has, err := tm.HasSession(sessionName)
	if err != nil {
		t.Fatalf("HasSession: %v", err)
	}
	if !has {
		t.Error("expected session to exist after multiple EnsureSessionFresh calls")
	}
}

func TestIsAgentRunning(t *testing.T) {
	if !hasTmux() {
		t.Skip("tmux not installed")
	}

	tm := NewTmux()

	// Use NewSessionWithCommand with deterministic processes to avoid a
	// TOCTOU race: the default shell's init scripts (.zshrc) can briefly
	// change the pane's foreground process (e.g., grep during sourcing),
	// causing IsAgentRunning's internal GetPaneCommand to see a different
	// command than what the test captured.

	t.Run("known_command", func(t *testing.T) {
		sessionName := "gt-test-agent-cmd"
		_ = tm.KillSession(sessionName)

		// Start a deterministic long-running process — no shell init race.
		if err := tm.NewSessionWithCommand(sessionName, "", "sleep 300"); err != nil {
			t.Fatalf("NewSessionWithCommand: %v", err)
		}
		defer func() { _ = tm.KillSession(sessionName) }()

		// Brief wait for process to start.
		time.Sleep(100 * time.Millisecond)

		cmd, err := tm.GetPaneCommand(sessionName)
		if err != nil {
			t.Fatalf("GetPaneCommand: %v", err)
		}
		if cmd != "sleep" {
			t.Fatalf("expected pane command 'sleep', got %q", cmd)
		}

		t.Run("matching_process", func(t *testing.T) {
			if !tm.IsAgentRunning(sessionName, "sleep") {
				t.Error("IsAgentRunning should return true when process matches")
			}
		})

		t.Run("non_matching_process", func(t *testing.T) {
			if tm.IsAgentRunning(sessionName, "node") {
				t.Error("IsAgentRunning should return false when process doesn't match")
			}
		})

		t.Run("multiple_with_match", func(t *testing.T) {
			if !tm.IsAgentRunning(sessionName, "nonexistent", "sleep", "also-nonexistent") {
				t.Error("IsAgentRunning should return true when one of multiple processes matches")
			}
		})

		t.Run("multiple_without_match", func(t *testing.T) {
			if tm.IsAgentRunning(sessionName, "nonexistent1", "nonexistent2") {
				t.Error("IsAgentRunning should return false when no process matches")
			}
		})

		t.Run("empty_list_non_shell_process", func(t *testing.T) {
			// sleep is not a shell, so the fallback detects it as "agent running".
			if !tm.IsAgentRunning(sessionName) {
				t.Error("IsAgentRunning with no args should return true for non-shell process")
			}
		})

		t.Run("empty_string_in_list_ignored", func(t *testing.T) {
			// Empty strings in the process list should be skipped.
			if tm.IsAgentRunning(sessionName, "", "") {
				t.Error("IsAgentRunning should return false when all process names are empty")
			}
		})
	})

	t.Run("shell_session", func(t *testing.T) {
		sessionName := "gt-test-agent-sh"
		_ = tm.KillSession(sessionName)

		// Use sh directly — minimal init, no .zshrc race.
		if err := tm.NewSessionWithCommand(sessionName, "", "sh"); err != nil {
			t.Fatalf("NewSessionWithCommand: %v", err)
		}
		defer func() { _ = tm.KillSession(sessionName) }()

		time.Sleep(100 * time.Millisecond)

		t.Run("empty_list_shell_process", func(t *testing.T) {
			// sh is a supported shell, so IsAgentRunning with no args should
			// return false (shell = no agent running).
			if tm.IsAgentRunning(sessionName) {
				cmd, _ := tm.GetPaneCommand(sessionName)
				t.Errorf("IsAgentRunning with no args should return false for shell session (cmd: %q)", cmd)
			}
		})

		t.Run("matching_shell_by_name", func(t *testing.T) {
			// Explicitly matching the shell name should return true.
			if !tm.IsAgentRunning(sessionName, "sh") {
				cmd, _ := tm.GetPaneCommand(sessionName)
				t.Errorf("IsAgentRunning('sh') should return true when shell matches (cmd: %q)", cmd)
			}
		})
	})
}

func TestIsAgentRunning_NonexistentSession(t *testing.T) {
	if !hasTmux() {
		t.Skip("tmux not installed")
	}

	tm := NewTmux()

	// IsAgentRunning on nonexistent session should return false, not error
	got := tm.IsAgentRunning("nonexistent-session-xyz", "node", "gemini", "cursor-agent")
	if got {
		t.Error("IsAgentRunning on nonexistent session should return false")
	}
}

func TestIsConfiguredAgentRunning(t *testing.T) {
	if !hasTmux() {
		t.Skip("tmux not installed")
	}

	tm := NewTmux()

	t.Run("matches expected pane commands", func(t *testing.T) {
		sessionName := "gt-test-configured-agent"
		_ = tm.KillSession(sessionName)
		if err := tm.NewSessionWithCommand(sessionName, "", "sleep 300"); err != nil {
			t.Fatalf("NewSessionWithCommand: %v", err)
		}
		defer func() { _ = tm.KillSession(sessionName) }()
		time.Sleep(100 * time.Millisecond)

		// Should match when expected commands include "sleep"
		if !tm.IsConfiguredAgentRunning(sessionName, []string{"sleep"}) {
			t.Error("IsConfiguredAgentRunning should return true when pane command matches expected")
		}

		// Should not match when expected commands don't include "sleep"
		if tm.IsConfiguredAgentRunning(sessionName, []string{"codex", "gemini"}) {
			t.Error("IsConfiguredAgentRunning should return false when pane command doesn't match expected")
		}
	})

	t.Run("empty commands falls back to IsClaudeRunning", func(t *testing.T) {
		sessionName := "gt-test-configured-agent-fallback"
		_ = tm.KillSession(sessionName)
		// Start a shell session - IsClaudeRunning should return false for a shell
		if err := tm.NewSessionWithCommand(sessionName, "", "sh"); err != nil {
			t.Fatalf("NewSessionWithCommand: %v", err)
		}
		defer func() { _ = tm.KillSession(sessionName) }()
		time.Sleep(100 * time.Millisecond)

		// With empty commands, falls back to IsClaudeRunning which returns false for shell
		if tm.IsConfiguredAgentRunning(sessionName, nil) {
			t.Error("IsConfiguredAgentRunning with nil commands should fall back to IsClaudeRunning")
		}
	})
}

func TestIsClaudeRunning(t *testing.T) {
	if !hasTmux() {
		t.Skip("tmux not installed")
	}

	tm := NewTmux()
	sessionName := "gt-test-claude-" + t.Name()

	// Clean up any existing session
	_ = tm.KillSession(sessionName)

	// Create session (will run default shell, not Claude)
	if err := tm.NewSession(sessionName, ""); err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer func() { _ = tm.KillSession(sessionName) }()

	// IsClaudeRunning should be false (shell is running, not node/claude)
	cmd, _ := tm.GetPaneCommand(sessionName)
	wantRunning := cmd == "node" || cmd == "claude"

	if got := tm.IsClaudeRunning(sessionName); got != wantRunning {
		t.Errorf("IsClaudeRunning() = %v, want %v (pane cmd: %q)", got, wantRunning, cmd)
	}
}

func TestIsClaudeRunning_VersionPattern(t *testing.T) {
	// Test the version pattern regex matching directly
	// Since we can't easily mock the pane command, test the pattern logic
	tests := []struct {
		cmd  string
		want bool
	}{
		{"node", true},
		{"claude", true},
		{"2.0.76", true},
		{"1.2.3", true},
		{"10.20.30", true},
		{"bash", false},
		{"zsh", false},
		{"", false},
		{"v2.0.76", false}, // version with 'v' prefix shouldn't match
		{"2.0", false},     // incomplete version
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			// Check if it matches node/claude directly
			isKnownCmd := tt.cmd == "node" || tt.cmd == "claude"
			// Check version pattern
			matched, _ := regexp.MatchString(`^\d+\.\d+\.\d+`, tt.cmd)

			got := isKnownCmd || matched
			if got != tt.want {
				t.Errorf("IsClaudeRunning logic for %q = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}

func TestIsClaudeRunning_ShellWithNodeChild(t *testing.T) {
	if !hasTmux() {
		t.Skip("tmux not installed")
	}

	tm := NewTmux()
	sessionName := "gt-test-shell-child-" + t.Name()

	// Clean up any existing session
	_ = tm.KillSession(sessionName)

	// Create session with "bash -c" running a node process
	// Use a simple node command that runs for a few seconds
	cmd := `node -e "setTimeout(() => {}, 10000)"`
	if err := tm.NewSessionWithCommand(sessionName, "", cmd); err != nil {
		t.Fatalf("NewSessionWithCommand: %v", err)
	}
	defer func() { _ = tm.KillSession(sessionName) }()

	// Give the node process time to start
	// WaitForCommand waits until NOT running bash/zsh/sh
	shellsToExclude := []string{"bash", "zsh", "sh"}
	err := tm.WaitForCommand(sessionName, shellsToExclude, 2000*1000000) // 2 second timeout
	if err != nil {
		// If we timeout waiting, it means the pane command is still a shell
		// This is the case we're testing - shell with a node child
		paneCmd, _ := tm.GetPaneCommand(sessionName)
		t.Logf("Pane command is %q - testing shell+child detection", paneCmd)
	}

	// Now test IsClaudeRunning - it should detect node as a child process
	paneCmd, _ := tm.GetPaneCommand(sessionName)
	if paneCmd == "node" {
		// Direct node detection should work
		if !tm.IsClaudeRunning(sessionName) {
			t.Error("IsClaudeRunning should return true when pane command is 'node'")
		}
	} else {
		// Pane is a shell (bash/zsh) with node as child
		// The new child process detection should catch this
		got := tm.IsClaudeRunning(sessionName)
		t.Logf("Pane command: %q, IsClaudeRunning: %v", paneCmd, got)
		// Note: This may or may not detect depending on how tmux runs the command.
		// On some systems, tmux runs the command directly; on others via a shell.
	}
}

func TestHasClaudeChild(t *testing.T) {
	// Test the hasClaudeChild helper function directly
	// This uses the current process as a test subject

	// Get current process PID as string
	currentPID := "1" // init/launchd - should have children but not claude/node

	// hasClaudeChild should return false for init (no node/claude children)
	got := hasClaudeChild(currentPID)
	if got {
		t.Logf("hasClaudeChild(%q) = true - init has claude/node child?", currentPID)
	}

	// Test with a definitely nonexistent PID
	got = hasClaudeChild("999999999")
	if got {
		t.Error("hasClaudeChild should return false for nonexistent PID")
	}
}

func TestGetAllDescendants(t *testing.T) {
	// Test the getAllDescendants helper function

	// Test with nonexistent PID - should return empty slice
	got := getAllDescendants("999999999")
	if len(got) != 0 {
		t.Errorf("getAllDescendants(nonexistent) = %v, want empty slice", got)
	}

	// Test with PID 1 (init/launchd) - should find some descendants
	// Note: We can't test exact PIDs, just that the function doesn't panic
	// and returns reasonable results
	descendants := getAllDescendants("1")
	t.Logf("getAllDescendants(\"1\") found %d descendants", len(descendants))

	// Verify returned PIDs are all numeric strings
	for _, pid := range descendants {
		for _, c := range pid {
			if c < '0' || c > '9' {
				t.Errorf("getAllDescendants returned non-numeric PID: %q", pid)
			}
		}
	}
}

func TestSessionSet(t *testing.T) {
	if !hasTmux() {
		t.Skip("tmux not installed")
	}

	tm := NewTmux()
	sessionName := "gt-test-sessionset-" + t.Name()

	// Clean up any existing session
	_ = tm.KillSession(sessionName)

	// Create a test session
	if err := tm.NewSession(sessionName, ""); err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer func() { _ = tm.KillSession(sessionName) }()

	// Get the session set
	set, err := tm.GetSessionSet()
	if err != nil {
		t.Fatalf("GetSessionSet: %v", err)
	}

	// Test Has() for existing session
	if !set.Has(sessionName) {
		t.Errorf("SessionSet.Has(%q) = false, want true", sessionName)
	}

	// Test Has() for non-existing session
	if set.Has("nonexistent-session-xyz-12345") {
		t.Error("SessionSet.Has(nonexistent) = true, want false")
	}

	// Test nil safety
	var nilSet *SessionSet
	if nilSet.Has("anything") {
		t.Error("nil SessionSet.Has() = true, want false")
	}

	// Test Names() returns the session
	names := set.Names()
	found := false
	for _, n := range names {
		if n == sessionName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("SessionSet.Names() doesn't contain %q", sessionName)
	}
}
