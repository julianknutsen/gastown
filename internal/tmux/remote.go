// Package tmux provides a wrapper for tmux session operations via subprocess.
// This file implements RemoteTmux which executes tmux commands on a remote machine via SSH.
package tmux

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/steveyegge/gastown/internal/constants"
	"github.com/steveyegge/gastown/internal/ids"
	"github.com/steveyegge/gastown/internal/session"
)

// Compile-time check that *RemoteTmux implements session.Sessions.
var _ session.Sessions = (*RemoteTmux)(nil)

// RemoteTmux wraps tmux operations on a remote machine via SSH.
// All tmux commands are executed through SSH, allowing agents to run
// on remote machines while being controlled from the local machine.
type RemoteTmux struct {
	sshCmd   string // SSH command prefix, e.g., "ssh user@remote" or "ssh -o ControlPath=/tmp/ssh user@remote"
	localSSH string // SSH command for remote to call back to local (for bd-wrapper), e.g., "ssh user@local-ip"
}

// NewRemoteTmux creates a new RemoteTmux wrapper.
// sshCmd is the SSH command prefix used to connect to the remote machine.
// Example: "ssh user@devbox" or "ssh -o ControlMaster=auto -o ControlPath=/tmp/gt-ssh user@devbox"
func NewRemoteTmux(sshCmd string) *RemoteTmux {
	return &RemoteTmux{sshCmd: sshCmd}
}

// NewRemoteTmuxWithCallback creates a RemoteTmux with bd-wrapper callback support.
// sshCmd: SSH command to reach remote (e.g., "ssh user@remote")
// localSSH: SSH command for remote to call back (e.g., "ssh -i ~/.ssh/key user@local-ip")
//
// When localSSH is set, Start() automatically prepends GT_LOCAL_SSH to commands,
// enabling bd-wrapper on the remote to proxy beads operations back to local.
func NewRemoteTmuxWithCallback(sshCmd, localSSH string) *RemoteTmux {
	return &RemoteTmux{sshCmd: sshCmd, localSSH: localSSH}
}

// run executes a tmux command on the remote machine via SSH.
func (t *RemoteTmux) run(args ...string) (string, error) {
	// Build the remote tmux command
	tmuxCmd := "tmux " + shellJoin(args)

	// Execute via SSH
	cmd := exec.Command("sh", "-c", t.sshCmd+" "+shellEscape(tmuxCmd))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", t.wrapError(err, stderr.String(), args)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// wrapError wraps tmux errors with context.
func (t *RemoteTmux) wrapError(err error, stderr string, args []string) error {
	stderr = strings.TrimSpace(stderr)

	// Detect specific error types (same as local Tmux)
	if strings.Contains(stderr, "no server running") ||
		strings.Contains(stderr, "error connecting to") {
		return ErrNoServer
	}
	if strings.Contains(stderr, "duplicate session") {
		return ErrSessionExists
	}
	if strings.Contains(stderr, "session not found") ||
		strings.Contains(stderr, "can't find session") {
		return ErrSessionNotFound
	}

	if stderr != "" {
		return fmt.Errorf("remote tmux %s: %s", args[0], stderr)
	}
	return fmt.Errorf("remote tmux %s: %w", args[0], err)
}

// Start creates a new detached terminal session on the remote machine.
// The command runs directly as the initial process of the pane.
// If localSSH was configured via NewRemoteTmuxWithCallback, GT_LOCAL_SSH
// is automatically prepended to enable bd-wrapper callback.
func (t *RemoteTmux) Start(name, workDir, command string) (session.SessionID, error) {
	// Prepend GT_LOCAL_SSH if configured (enables bd-wrapper callback)
	if t.localSSH != "" {
		command = "GT_LOCAL_SSH='" + t.localSSH + "' " + command
	}

	args := []string{"new-session", "-d", "-s", name}
	if workDir != "" {
		args = append(args, "-c", workDir)
	}
	// Add the command as the last argument
	args = append(args, command)

	_, err := t.run(args...)
	if err != nil {
		return "", err
	}

	return session.SessionID(name), nil
}

// Stop terminates a terminal session on the remote machine.
// For remote sessions, we use a simpler approach than local: just kill the session.
// The remote machine's tmux will handle process cleanup.
func (t *RemoteTmux) Stop(id session.SessionID) error {
	name := string(id)

	// First try to get pane PID and kill descendants (best effort)
	pid, err := t.getPanePID(name)
	if err == nil && pid != "" {
		// Kill descendants on remote using a single SSH command
		// This uses a shell one-liner to find and kill descendants
		killCmd := fmt.Sprintf(
			"for p in $(pgrep -P %s 2>/dev/null); do kill -TERM $p 2>/dev/null; done; "+
				"sleep 0.1; "+
				"for p in $(pgrep -P %s 2>/dev/null); do kill -KILL $p 2>/dev/null; done",
			pid, pid)
		_ = t.sshRun(killCmd) // Best effort
	}

	// Kill the tmux session
	_, err = t.run("kill-session", "-t", name)
	if errors.Is(err, ErrSessionNotFound) {
		return nil // Session already gone
	}
	return err
}

// Respawn atomically kills the session's process and starts a new one.
func (t *RemoteTmux) Respawn(id session.SessionID, command string) error {
	// Get pane ID for the session
	paneID, err := t.getPaneID(string(id))
	if err != nil {
		return fmt.Errorf("getting pane ID: %w", err)
	}

	// Clear scrollback history (best effort)
	_, _ = t.run("clear-history", "-t", paneID)

	// Prepend GT_LOCAL_SSH if configured
	if t.localSSH != "" {
		command = "GT_LOCAL_SSH='" + t.localSSH + "' " + command
	}

	// Atomically kill current process and start new one
	_, err = t.run("respawn-pane", "-k", "-t", paneID, command)
	return err
}

// getPaneID returns the pane ID for a session on the remote machine.
func (t *RemoteTmux) getPaneID(sess string) (string, error) {
	out, err := t.run("list-panes", "-t", sess, "-F", "#{pane_id}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// Exists checks if a session exists on the remote machine (exact match).
func (t *RemoteTmux) Exists(id session.SessionID) (bool, error) {
	_, err := t.run("has-session", "-t", "="+string(id))
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) || errors.Is(err, ErrNoServer) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Send sends text to a session on the remote machine and presses Enter.
func (t *RemoteTmux) Send(id session.SessionID, text string) error {
	// Send text using literal mode (-l) to handle special chars
	if _, err := t.run("send-keys", "-t", string(id), "-l", text); err != nil {
		return err
	}
	// Wait for paste to be processed
	time.Sleep(time.Duration(constants.DefaultDebounceMs) * time.Millisecond)
	// Send Enter separately
	_, err := t.run("send-keys", "-t", string(id), "Enter")
	return err
}

// SendControl sends a control sequence to a session on the remote machine.
// Used for terminal control like "C-c" (interrupt), "Down" (arrow key), "Escape", etc.
func (t *RemoteTmux) SendControl(id session.SessionID, key string) error {
	_, err := t.run("send-keys", "-t", string(id), key)
	return err
}

// Nudge sends a message to a Claude Code session on the remote machine reliably.
// Uses: literal mode + 500ms debounce + ESC (for vim mode) + separate Enter with retry.
func (t *RemoteTmux) Nudge(id session.SessionID, message string) error {
	sess := string(id)

	// 1. Send text in literal mode (handles special characters)
	if _, err := t.run("send-keys", "-t", sess, "-l", message); err != nil {
		return err
	}

	// 2. Wait 500ms for paste to complete (tested, required)
	time.Sleep(500 * time.Millisecond)

	// 3. Send Escape to exit vim INSERT mode if enabled (harmless in normal mode)
	_, _ = t.run("send-keys", "-t", sess, "Escape")
	time.Sleep(100 * time.Millisecond)

	// 4. Send Enter with retry (critical for message submission)
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(200 * time.Millisecond)
		}
		if _, err := t.run("send-keys", "-t", sess, "Enter"); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return fmt.Errorf("failed to send Enter after 3 attempts: %w", lastErr)
}

// Capture captures the visible content of a pane on the remote machine.
func (t *RemoteTmux) Capture(id session.SessionID, lines int) (string, error) {
	return t.run("capture-pane", "-p", "-t", string(id), "-S", fmt.Sprintf("-%d", lines))
}

// CaptureAll captures all scrollback history from a pane on the remote machine.
func (t *RemoteTmux) CaptureAll(id session.SessionID) (string, error) {
	return t.run("capture-pane", "-p", "-t", string(id), "-S", "-")
}

// GetStartCommand returns the command that started the session.
func (t *RemoteTmux) GetStartCommand(id session.SessionID) (string, error) {
	output, err := t.run("display-message", "-t", string(id), "-p", "#{pane_start_command}")
	if err != nil {
		return "", fmt.Errorf("getting start command: %w", err)
	}
	return strings.TrimSpace(output), nil
}

// IsRunning checks if any of the given process names match the pane's current command
// on the remote machine.
func (t *RemoteTmux) IsRunning(id session.SessionID, processNames ...string) bool {
	if len(processNames) == 0 {
		return false
	}
	cmd, err := t.getPaneCommand(string(id))
	if err != nil {
		return false
	}
	for _, name := range processNames {
		if cmd == name {
			return true
		}
	}
	return false
}

// WaitFor polls until one of the given process names is running in the session
// on the remote machine.
func (t *RemoteTmux) WaitFor(id session.SessionID, timeout time.Duration, processNames ...string) error {
	if len(processNames) == 0 {
		return nil
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if t.IsRunning(id, processNames...) {
			return nil
		}
		time.Sleep(constants.PollInterval)
	}
	return fmt.Errorf("timeout waiting for process %v on remote", processNames)
}

// List returns all session IDs on the remote machine.
func (t *RemoteTmux) List() ([]session.SessionID, error) {
	out, err := t.run("list-sessions", "-F", "#{session_name}")
	if err != nil {
		if errors.Is(err, ErrNoServer) {
			return nil, nil // No server = no sessions
		}
		return nil, err
	}

	if out == "" {
		return nil, nil
	}

	names := strings.Split(out, "\n")
	ids := make([]session.SessionID, len(names))
	for i, name := range names {
		ids[i] = session.SessionID(name)
	}
	return ids, nil
}

// Attach attaches to a session on the remote machine.
// For remote sessions, this opens an interactive SSH connection with tmux attach.
func (t *RemoteTmux) Attach(id session.SessionID) error {
	// Build command: ssh <remote> -t tmux attach -t <session>
	cmd := exec.Command("sh", "-c", t.sshCmd+" -t tmux attach -t "+shellEscape(string(id)))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// SwitchTo is not supported for remote sessions.
// Remote sessions cannot be switched to from local tmux.
func (t *RemoteTmux) SwitchTo(id session.SessionID) error {
	return fmt.Errorf("SwitchTo not supported for remote sessions")
}

// SessionIDForAgent converts an agent address to its SessionID.
func (t *RemoteTmux) SessionIDForAgent(id ids.AgentID) session.SessionID {
	return session.SessionID(session.SessionNameFromAgentID(id))
}

// GetInfo returns detailed information about a session on the remote machine.
func (t *RemoteTmux) GetInfo(id session.SessionID) (*session.Info, error) {
	name := string(id)
	format := "#{session_name}|#{session_windows}|#{session_created_string}|#{session_attached}|#{session_activity}|#{session_last_attached}"
	out, err := t.run("list-sessions", "-F", format, "-f", fmt.Sprintf("#{==:#{session_name},%s}", name))
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, ErrSessionNotFound
	}

	parts := strings.Split(out, "|")
	if len(parts) < 4 {
		return nil, fmt.Errorf("unexpected session info format: %s", out)
	}

	windows := 0
	_, _ = fmt.Sscanf(parts[1], "%d", &windows)

	info := &session.Info{
		Name:     parts[0],
		Windows:  windows,
		Created:  parts[2],
		Attached: parts[3] == "1",
	}

	if len(parts) > 4 {
		info.Activity = parts[4]
	}
	if len(parts) > 5 {
		info.LastAttached = parts[5]
	}

	return info, nil
}

// Helper methods

// getPaneCommand returns the current command running in a pane on the remote machine.
func (t *RemoteTmux) getPaneCommand(sess string) (string, error) {
	out, err := t.run("list-panes", "-t", sess, "-F", "#{pane_current_command}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// getPanePID returns the PID of the pane's main process on the remote machine.
func (t *RemoteTmux) getPanePID(sess string) (string, error) {
	out, err := t.run("list-panes", "-t", sess, "-F", "#{pane_pid}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// sshRun executes an arbitrary command on the remote machine via SSH.
// Used for non-tmux commands like killing processes.
func (t *RemoteTmux) sshRun(remoteCmd string) error {
	cmd := exec.Command("sh", "-c", t.sshCmd+" "+shellEscape(remoteCmd))
	return cmd.Run()
}

// shellEscape escapes a string for safe use in shell commands.
func shellEscape(s string) string {
	// Use single quotes to escape, replacing any single quotes in the string
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// shellJoin joins arguments into a shell command string with proper escaping.
func shellJoin(args []string) string {
	escaped := make([]string, len(args))
	for i, arg := range args {
		escaped[i] = shellEscape(arg)
	}
	return strings.Join(escaped, " ")
}
