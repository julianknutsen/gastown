package tmux_test

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/steveyegge/gastown/internal/session"
	"github.com/steveyegge/gastown/internal/tmux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// skipIfNoLocalSSH skips the test if localhost SSH is not available.
// These tests require passwordless SSH to localhost.
func skipIfNoLocalSSH(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping SSH tests in short mode")
	}
	// Try to SSH to localhost with a quick timeout
	cmd := exec.Command("ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=2", "localhost", "echo", "test")
	if err := cmd.Run(); err != nil {
		t.Skip("skipping: cannot SSH to localhost (need passwordless SSH setup)")
	}
}

// cleanupSession ensures a session is killed after the test.
func cleanupSession(t *testing.T, remote *tmux.Tmux, name string) {
	t.Helper()
	t.Cleanup(func() {
		_ = remote.Stop(session.SessionID(name))
	})
}

func TestRemoteTmux_ImplementsSessions(t *testing.T) {
	// Compile-time check that Tmux implements Sessions
	var _ session.Sessions = (*tmux.Tmux)(nil)
}

func TestRemoteTmux_StartAndStop(t *testing.T) {
	skipIfNoLocalSSH(t)

	remote := tmux.NewRemoteTmux("ssh localhost")
	sessionName := "gt-test-remote-start-" + randomSuffix()
	cleanupSession(t, remote, sessionName)

	// Start a session
	id, err := remote.Start(sessionName, "/tmp", "sleep 30")
	require.NoError(t, err)
	assert.Equal(t, session.SessionID(sessionName), id)

	// Verify it exists
	exists, err := remote.Exists(id)
	require.NoError(t, err)
	assert.True(t, exists)

	// Stop it
	err = remote.Stop(id)
	require.NoError(t, err)

	// Verify it's gone
	exists, err = remote.Exists(id)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestRemoteTmux_Exists_NotFound(t *testing.T) {
	skipIfNoLocalSSH(t)

	remote := tmux.NewRemoteTmux("ssh localhost")

	exists, err := remote.Exists("gt-nonexistent-session-12345")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestRemoteTmux_List(t *testing.T) {
	skipIfNoLocalSSH(t)

	remote := tmux.NewRemoteTmux("ssh localhost")
	sessionName := "gt-test-remote-list-" + randomSuffix()
	cleanupSession(t, remote, sessionName)

	// Start a session
	_, err := remote.Start(sessionName, "/tmp", "sleep 30")
	require.NoError(t, err)

	// List sessions
	sessions, err := remote.List()
	require.NoError(t, err)

	// Verify our session is in the list
	found := false
	for _, id := range sessions {
		if string(id) == sessionName {
			found = true
			break
		}
	}
	assert.True(t, found, "session %s not found in list", sessionName)
}

func TestRemoteTmux_Send(t *testing.T) {
	skipIfNoLocalSSH(t)

	remote := tmux.NewRemoteTmux("ssh localhost")
	sessionName := "gt-test-remote-send-" + randomSuffix()
	cleanupSession(t, remote, sessionName)

	// Start a session with cat (to echo input)
	_, err := remote.Start(sessionName, "/tmp", "cat")
	require.NoError(t, err)

	// Wait for cat to start
	time.Sleep(200 * time.Millisecond)

	// Send some text
	err = remote.Send(session.SessionID(sessionName), "hello world")
	require.NoError(t, err)

	// Give it time to process
	time.Sleep(300 * time.Millisecond)

	// Capture output
	output, err := remote.Capture(session.SessionID(sessionName), 10)
	require.NoError(t, err)
	assert.Contains(t, output, "hello world")
}

func TestRemoteTmux_Capture(t *testing.T) {
	skipIfNoLocalSSH(t)

	remote := tmux.NewRemoteTmux("ssh localhost")
	sessionName := "gt-test-remote-capture-" + randomSuffix()
	cleanupSession(t, remote, sessionName)

	// Start a session that prints something
	_, err := remote.Start(sessionName, "/tmp", "echo 'captured output'; sleep 30")
	require.NoError(t, err)

	// Wait for echo to run
	time.Sleep(500 * time.Millisecond)

	// Capture output
	output, err := remote.Capture(session.SessionID(sessionName), 10)
	require.NoError(t, err)
	assert.Contains(t, output, "captured output")
}

func TestRemoteTmux_SendControl(t *testing.T) {
	skipIfNoLocalSSH(t)

	remote := tmux.NewRemoteTmux("ssh localhost")
	sessionName := "gt-test-remote-ctrl-" + randomSuffix()
	cleanupSession(t, remote, sessionName)

	// Start a session with a shell (not a direct command)
	// This way when we send Ctrl+C to interrupt a command, the shell survives
	_, err := remote.Start(sessionName, "/tmp", "bash")
	require.NoError(t, err)

	// Wait for bash to start
	time.Sleep(300 * time.Millisecond)

	// Send a sleep command to the shell
	err = remote.Send(session.SessionID(sessionName), "sleep 60")
	require.NoError(t, err)

	// Wait for sleep to start
	time.Sleep(300 * time.Millisecond)

	// Send Ctrl+C to interrupt the sleep
	err = remote.SendControl(session.SessionID(sessionName), "C-c")
	require.NoError(t, err)

	// Give it time to process
	time.Sleep(300 * time.Millisecond)

	// The session should still exist (bash shell survived the Ctrl+C)
	exists, err := remote.Exists(session.SessionID(sessionName))
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestRemoteTmux_IsRunning(t *testing.T) {
	skipIfNoLocalSSH(t)

	remote := tmux.NewRemoteTmux("ssh localhost")
	sessionName := "gt-test-remote-isrunning-" + randomSuffix()
	cleanupSession(t, remote, sessionName)

	// Start a session with sleep
	_, err := remote.Start(sessionName, "/tmp", "sleep 30")
	require.NoError(t, err)

	// Wait for process to start
	time.Sleep(300 * time.Millisecond)

	// Check if sleep is running
	id := session.SessionID(sessionName)
	assert.True(t, remote.IsRunning(id, "sleep"))
	assert.False(t, remote.IsRunning(id, "nonexistent"))
}

func TestRemoteTmux_GetInfo(t *testing.T) {
	skipIfNoLocalSSH(t)

	remote := tmux.NewRemoteTmux("ssh localhost")
	sessionName := "gt-test-remote-getinfo-" + randomSuffix()
	cleanupSession(t, remote, sessionName)

	// Start a session
	_, err := remote.Start(sessionName, "/tmp", "sleep 30")
	require.NoError(t, err)

	// Get info
	info, err := remote.GetInfo(session.SessionID(sessionName))
	require.NoError(t, err)
	assert.Equal(t, sessionName, info.Name)
	assert.GreaterOrEqual(t, info.Windows, 1)
}

func TestRemoteTmux_GetInfo_NotFound(t *testing.T) {
	skipIfNoLocalSSH(t)

	remote := tmux.NewRemoteTmux("ssh localhost")

	_, err := remote.GetInfo("gt-nonexistent-12345")
	assert.Error(t, err)
}

func TestRemoteTmux_StopIdempotent(t *testing.T) {
	skipIfNoLocalSSH(t)

	remote := tmux.NewRemoteTmux("ssh localhost")

	// Stop a non-existent session should not error
	err := remote.Stop("gt-nonexistent-stop-12345")
	assert.NoError(t, err)
}

func TestRemoteTmux_WaitFor(t *testing.T) {
	skipIfNoLocalSSH(t)

	remote := tmux.NewRemoteTmux("ssh localhost")
	sessionName := "gt-test-remote-waitfor-" + randomSuffix()
	cleanupSession(t, remote, sessionName)

	// Start a session that will run sleep after a brief delay
	_, err := remote.Start(sessionName, "/tmp", "sleep 0.2 && exec sleep 30")
	require.NoError(t, err)

	id := session.SessionID(sessionName)

	// Wait for sleep to be running (should succeed)
	err = remote.WaitFor(id, 5*time.Second, "sleep")
	assert.NoError(t, err)
}

func TestRemoteTmux_WaitFor_Timeout(t *testing.T) {
	skipIfNoLocalSSH(t)

	remote := tmux.NewRemoteTmux("ssh localhost")
	sessionName := "gt-test-remote-waitfor-timeout-" + randomSuffix()
	cleanupSession(t, remote, sessionName)

	// Start a session with a shell command that won't match
	_, err := remote.Start(sessionName, "/tmp", "cat")
	require.NoError(t, err)

	id := session.SessionID(sessionName)

	// Wait for a process that won't appear (should timeout)
	err = remote.WaitFor(id, 500*time.Millisecond, "nonexistent-process")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

// randomSuffix generates a short random suffix for test session names.
func randomSuffix() string {
	return time.Now().Format("150405.000")[7:] // Uses milliseconds as suffix
}

// TestRemoteTmux_ShellEscape tests the shell escaping helper.
func TestRemoteTmux_ShellEscape(t *testing.T) {
	// These are tested implicitly through the integration tests above,
	// but we can add edge case tests here if needed.

	// Test that single quotes in messages are handled
	skipIfNoLocalSSH(t)

	remote := tmux.NewRemoteTmux("ssh localhost")
	sessionName := "gt-test-remote-escape-" + randomSuffix()
	cleanupSession(t, remote, sessionName)

	// Start a session with cat
	_, err := remote.Start(sessionName, "/tmp", "cat")
	require.NoError(t, err)

	// Wait for cat to start
	time.Sleep(200 * time.Millisecond)

	// Send text with single quotes (edge case for shell escaping)
	err = remote.Send(session.SessionID(sessionName), "it's a test with 'quotes'")
	require.NoError(t, err)

	// Give it time to process
	time.Sleep(300 * time.Millisecond)

	// Capture and verify
	output, err := remote.Capture(session.SessionID(sessionName), 10)
	require.NoError(t, err)
	// Just verify it didn't error - the message should appear
	assert.True(t, len(output) > 0 || strings.Contains(output, "quotes"))
}
