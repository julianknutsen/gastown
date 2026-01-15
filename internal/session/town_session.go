// Package session provides polecat session lifecycle management.
package session

import "time"

// TownSessions wraps a Sessions with town-aware session naming.
//
// Behavior:
//   - Start(): Appends town ID suffix to session name
//   - Stop()/Exists()/etc: Uses optimistic-with-retry to find session (new or legacy format)
//   - List(): Filters to only sessions matching this town
//
// Callers provide logical session names (e.g., "hq-mayor") and TownSessions
// handles the town suffix transformation. Use the session name functions
// (MayorSessionName, WitnessSessionName, etc.) to generate logical names.
//
// Note: TownSessions transforms names, so it's not a transparent Sessions wrapper.
// Code using TownSessions must use it consistently for all operations on the same sessions.
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

// --- Lifecycle methods (town-aware) ---

// Start creates a new session with the town ID suffix appended to the name.
// If town root is empty, uses the name as-is (legacy mode).
func (t *TownSessions) Start(name, workDir, command string) (SessionID, error) {
	townName := t.appendTownSuffix(name)
	return t.sess.Start(townName, workDir, command)
}

// Stop terminates a session using optimistic-with-retry lookup.
// Tries the town-suffixed name first, falls back to legacy name.
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
// Tries the town-suffixed name first, falls back to legacy name.
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

// --- Communication methods (pass-through after resolution) ---

// Send sends text to a session (resolves using optimistic-with-retry).
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

// --- Observation methods (pass-through after resolution) ---

// Capture captures pane content from a session.
func (t *TownSessions) Capture(id SessionID, lines int) (string, error) {
	actualID, err := t.resolveSession(id)
	if err != nil {
		return "", err
	}
	return t.sess.Capture(actualID, lines)
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

// --- Management methods ---

// List returns all sessions belonging to this town.
// Filters out sessions from other towns, but includes legacy sessions.
func (t *TownSessions) List() ([]SessionID, error) {
	all, err := t.sess.List()
	if err != nil {
		return nil, err
	}

	// No filtering if no town root (legacy mode)
	if t.townRoot == "" {
		return all, nil
	}

	var filtered []SessionID
	for _, id := range all {
		if MatchesTown(string(id), t.townRoot) {
			filtered = append(filtered, id)
		}
	}
	return filtered, nil
}

// ListAll returns all sessions without filtering.
// Use this when you need to see sessions from all towns.
func (t *TownSessions) ListAll() ([]SessionID, error) {
	return t.sess.List()
}

// GetInfo returns information about a session.
func (t *TownSessions) GetInfo(id SessionID) (*Info, error) {
	actualID, err := t.resolveSession(id)
	if err != nil {
		return nil, err
	}
	return t.sess.GetInfo(actualID)
}

// --- Internal helpers ---

// appendTownSuffix adds the town ID suffix to a session name.
// Returns name unchanged if no town root is configured.
func (t *TownSessions) appendTownSuffix(name string) string {
	if t.townRoot == "" {
		return name
	}
	return name + townSuffix(t.townRoot)
}

// resolveSession implements optimistic-with-retry lookup.
// Given a logical session ID, finds the actual session (new or legacy format).
func (t *TownSessions) resolveSession(id SessionID) (SessionID, error) {
	name := string(id)

	// If the ID already has a town suffix, use it directly
	if ExtractTownID(name) != "" {
		exists, err := t.sess.Exists(id)
		if err != nil {
			return "", err
		}
		if exists {
			return id, nil
		}
		return "", ErrSessionNotFound
	}

	// Try new format first (with town suffix)
	if t.townRoot != "" {
		newID := SessionID(t.appendTownSuffix(name))
		exists, err := t.sess.Exists(newID)
		if err != nil {
			return "", err
		}
		if exists {
			return newID, nil
		}
	}

	// Fall back to legacy format (no suffix)
	exists, err := t.sess.Exists(id)
	if err != nil {
		return "", err
	}
	if exists {
		return id, nil
	}

	return "", ErrSessionNotFound
}
