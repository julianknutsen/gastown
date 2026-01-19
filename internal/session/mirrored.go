package session

import (
	"time"

	"github.com/steveyegge/gastown/internal/ids"
)

// MirroredSessions wraps remote and local Sessions for remote polecats.
// The remote session is the source of truth (where the agent actually runs).
// The local mirror provides better UX for attach (no SSH latency).
//
// Operation routing:
//   - Start: creates session on remote, creates local mirror that attaches to remote
//   - Attach: attaches to local mirror (better UX)
//   - Stop: stops both local mirror and remote session
//   - Query ops (Exists, Capture, IsRunning, etc.): delegate to remote
type MirroredSessions struct {
	remote  Sessions // Source of truth (remote tmux via SSH)
	local   Sessions // Local mirror (for attach UX)
	sshCmd  string   // SSH command for creating the mirror
}

// NewMirroredSessions creates a MirroredSessions wrapper.
// sshCmd is used to create the local mirror that attaches to the remote session.
func NewMirroredSessions(remote, local Sessions, sshCmd string) *MirroredSessions {
	return &MirroredSessions{
		remote: remote,
		local:  local,
		sshCmd: sshCmd,
	}
}

// mirrorName returns the local mirror session name for a given session.
func mirrorName(name string) string {
	return name + "-mirror"
}

// Start creates the session on the remote and a local mirror for attach UX.
func (m *MirroredSessions) Start(name, workDir, command string) (SessionID, error) {
	// 1. Start the real session on remote
	id, err := m.remote.Start(name, workDir, command)
	if err != nil {
		return "", err
	}

	// 2. Create local mirror that SSH-attaches to the remote session
	// The mirror session runs: ssh <host> tmux attach -t <session>
	// This gives us a local tmux session we can attach to with no additional latency
	mirrorCmd := m.sshCmd + " -t tmux attach-session -t " + name
	_, err = m.local.Start(mirrorName(name), "", mirrorCmd)
	if err != nil {
		// Mirror creation failed - non-fatal, remote session still works
		// User can still attach via direct SSH
	}

	return id, nil
}

// Stop stops both the local mirror and remote session.
func (m *MirroredSessions) Stop(id SessionID) error {
	name := string(id)

	// Stop local mirror first (it's attached to remote, will disconnect gracefully)
	_ = m.local.Stop(SessionID(mirrorName(name)))

	// Stop the remote session
	return m.remote.Stop(id)
}

// Exists checks if the session exists on the remote (source of truth).
func (m *MirroredSessions) Exists(id SessionID) (bool, error) {
	return m.remote.Exists(id)
}

// Respawn respawns the session on the remote.
func (m *MirroredSessions) Respawn(id SessionID, command string) error {
	return m.remote.Respawn(id, command)
}

// Send sends text to the remote session.
func (m *MirroredSessions) Send(id SessionID, text string) error {
	return m.remote.Send(id, text)
}

// SendControl sends a control sequence to the remote session.
func (m *MirroredSessions) SendControl(id SessionID, key string) error {
	return m.remote.SendControl(id, key)
}

// Nudge sends a message to the remote session with retry logic.
func (m *MirroredSessions) Nudge(id SessionID, message string) error {
	return m.remote.Nudge(id, message)
}

// Capture captures output from the remote session.
func (m *MirroredSessions) Capture(id SessionID, lines int) (string, error) {
	return m.remote.Capture(id, lines)
}

// CaptureAll captures entire scrollback from the remote session.
func (m *MirroredSessions) CaptureAll(id SessionID) (string, error) {
	return m.remote.CaptureAll(id)
}

// IsRunning checks if processes are running in the remote session.
func (m *MirroredSessions) IsRunning(id SessionID, processNames ...string) bool {
	return m.remote.IsRunning(id, processNames...)
}

// WaitFor waits for processes in the remote session.
func (m *MirroredSessions) WaitFor(id SessionID, timeout time.Duration, processNames ...string) error {
	return m.remote.WaitFor(id, timeout, processNames...)
}

// GetStartCommand gets the start command from the remote session.
func (m *MirroredSessions) GetStartCommand(id SessionID) (string, error) {
	return m.remote.GetStartCommand(id)
}

// List lists sessions from the remote.
func (m *MirroredSessions) List() ([]SessionID, error) {
	return m.remote.List()
}

// GetInfo gets session info from the remote.
func (m *MirroredSessions) GetInfo(id SessionID) (*Info, error) {
	return m.remote.GetInfo(id)
}

// Attach attaches to the local mirror for better UX.
// If the mirror doesn't exist, falls back to remote attach.
func (m *MirroredSessions) Attach(id SessionID) error {
	name := string(id)
	mirrorID := SessionID(mirrorName(name))

	// Try to attach to local mirror first
	exists, err := m.local.Exists(mirrorID)
	if err == nil && exists {
		return m.local.Attach(mirrorID)
	}

	// Fall back to remote attach
	return m.remote.Attach(id)
}

// SwitchTo switches to the local mirror if it exists.
func (m *MirroredSessions) SwitchTo(id SessionID) error {
	name := string(id)
	mirrorID := SessionID(mirrorName(name))

	// Try local mirror first
	exists, err := m.local.Exists(mirrorID)
	if err == nil && exists {
		return m.local.SwitchTo(mirrorID)
	}

	// Fall back to remote
	return m.remote.SwitchTo(id)
}

// SessionIDForAgent converts an agent ID to session ID (delegates to remote).
func (m *MirroredSessions) SessionIDForAgent(id ids.AgentID) SessionID {
	return m.remote.SessionIDForAgent(id)
}

// Compile-time check that MirroredSessions implements Sessions.
var _ Sessions = (*MirroredSessions)(nil)
