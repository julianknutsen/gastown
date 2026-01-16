// Package session provides polecat session lifecycle management.
package session

import "time"

// TownSessions wraps a Sessions with town-aware session naming.
//
// This layer is responsible for:
//   - Translating logical session names to unique, collision-free tmux session names
//   - Reversing tmux session names back to logical names on queries
//   - Filtering sessions to only those belonging to this town
//
// Logical name format: "role" for town-level, "rig/role" for rig-level, "rig/role/name" for named.
// Examples: "mayor", "deacon", "myrig/witness", "myrig/polecat/toast"
//
// The underlying Sessions implementation remains generic and knows nothing about
// Gas Town naming conventions or multi-town isolation.
type TownSessions struct {
	sess     Sessions
	townRoot string
}

// Ensure TownSessions implements Sessions
var _ Sessions = (*TownSessions)(nil)

// NewTownSessions creates a town-aware Sessions wrapper.
// If townRoot is empty, behaves like the underlying Sessions (legacy mode).
func NewTownSessions(sess Sessions, townRoot string) *TownSessions {
	return &TownSessions{
		sess:     sess,
		townRoot: townRoot,
	}
}

// TownRoot returns the town root path.
func (t *TownSessions) TownRoot() string {
	return t.townRoot
}

// --- Lifecycle methods ---

// Start creates a new session from a logical name.
// Translates to unique name (including town suffix).
// Returns the logical SessionID (not the unique name).
func (t *TownSessions) Start(name, workDir, command string) (SessionID, error) {
	uniqueName := ToUniqueHumanReadableName(SessionID(name), t.townRoot)
	_, err := t.sess.Start(uniqueName, workDir, command)
	if err != nil {
		return "", err
	}
	return SessionID(name), nil
}

// Stop terminates a session using optimistic-with-retry lookup.
// Returns nil if no matching session exists (idempotent).
func (t *TownSessions) Stop(id SessionID) error {
	actualID, err := t.resolveSession(id)
	if err == ErrSessionNotFound {
		return nil // Nothing to stop
	}
	if err != nil {
		return err
	}
	return t.sess.Stop(actualID)
}

// Exists checks if a session exists using optimistic-with-retry lookup.
func (t *TownSessions) Exists(id SessionID) (bool, error) {
	_, err := t.resolveSession(id)
	if err == ErrSessionNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Respawn atomically kills the session's process and starts a new one.
func (t *TownSessions) Respawn(id SessionID, command string) error {
	actualID, err := t.resolveSession(id)
	if err != nil {
		return err
	}
	return t.sess.Respawn(actualID, command)
}

// --- Communication methods ---

// Send sends text to a session.
func (t *TownSessions) Send(id SessionID, text string) error {
	actualID, err := t.resolveSession(id)
	if err != nil {
		return err
	}
	return t.sess.Send(actualID, text)
}

// SendControl sends a control sequence to a session.
func (t *TownSessions) SendControl(id SessionID, key string) error {
	actualID, err := t.resolveSession(id)
	if err != nil {
		return err
	}
	return t.sess.SendControl(actualID, key)
}

// Nudge sends a message to a running agent reliably.
func (t *TownSessions) Nudge(id SessionID, message string) error {
	actualID, err := t.resolveSession(id)
	if err != nil {
		return err
	}
	return t.sess.Nudge(actualID, message)
}

// --- Observation methods ---

// Capture captures pane content from a session.
func (t *TownSessions) Capture(id SessionID, lines int) (string, error) {
	actualID, err := t.resolveSession(id)
	if err != nil {
		return "", err
	}
	return t.sess.Capture(actualID, lines)
}

// CaptureAll captures the entire scrollback history.
func (t *TownSessions) CaptureAll(id SessionID) (string, error) {
	actualID, err := t.resolveSession(id)
	if err != nil {
		return "", err
	}
	return t.sess.CaptureAll(actualID)
}

// IsRunning checks if a session is running specified processes.
func (t *TownSessions) IsRunning(id SessionID, processNames ...string) bool {
	actualID, err := t.resolveSession(id)
	if err != nil {
		return false
	}
	return t.sess.IsRunning(actualID, processNames...)
}

// WaitFor waits for processes to start in a session.
func (t *TownSessions) WaitFor(id SessionID, timeout time.Duration, processNames ...string) error {
	actualID, err := t.resolveSession(id)
	if err != nil {
		return err
	}
	return t.sess.WaitFor(actualID, timeout, processNames...)
}

// GetStartCommand returns the command that started the session.
func (t *TownSessions) GetStartCommand(id SessionID) (string, error) {
	actualID, err := t.resolveSession(id)
	if err != nil {
		return "", err
	}
	return t.sess.GetStartCommand(actualID)
}

// --- Management methods ---

// List returns all logical session names belonging to this town.
// Filters out sessions from other towns, but includes legacy sessions.
func (t *TownSessions) List() ([]SessionID, error) {
	all, err := t.sess.List()
	if err != nil {
		return nil, err
	}

	var filtered []SessionID
	for _, uniqueID := range all {
		uniqueName := string(uniqueID)
		logicalID, owned := FromUniqueHumanReadableName(uniqueName, t.townRoot)
		if owned {
			filtered = append(filtered, logicalID)
		}
	}
	return filtered, nil
}

// GetInfo returns information about a session.
func (t *TownSessions) GetInfo(id SessionID) (*Info, error) {
	actualID, err := t.resolveSession(id)
	if err != nil {
		return nil, err
	}
	return t.sess.GetInfo(actualID)
}

// Attach attaches to a session.
func (t *TownSessions) Attach(id SessionID) error {
	actualID, err := t.resolveSession(id)
	if err != nil {
		return err
	}
	return t.sess.Attach(actualID)
}

// SwitchTo switches the current tmux client to the target session.
// Only works when inside tmux.
func (t *TownSessions) SwitchTo(id SessionID) error {
	actualID, err := t.resolveSession(id)
	if err != nil {
		return err
	}
	return t.sess.SwitchTo(actualID)
}

// --- Internal helpers ---

// resolveSession implements optimistic-with-retry lookup.
// Given a logical session ID, finds the actual session (new or legacy format).
func (t *TownSessions) resolveSession(id SessionID) (SessionID, error) {
	uniqueName := ToUniqueHumanReadableName(id, t.townRoot)

	// Try the new format first (with town suffix)
	uniqueID := SessionID(uniqueName)
	exists, err := t.sess.Exists(uniqueID)
	if err != nil {
		return "", err
	}
	if exists {
		return uniqueID, nil
	}

	// Fall back to legacy format (without town suffix) for migration
	if t.townRoot != "" {
		legacyName := ToUniqueHumanReadableName(id, "")
		legacyID := SessionID(legacyName)
		exists, err := t.sess.Exists(legacyID)
		if err != nil {
			return "", err
		}
		if exists {
			return legacyID, nil
		}
	}

	return "", ErrSessionNotFound
}
