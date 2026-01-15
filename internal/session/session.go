// Package session provides abstractions for running agents in terminal sessions.
// The primary implementation is tmux, but this abstraction allows for
// testing with mocks and potentially other terminal multiplexers (e.g., zellij).
package session

import "time"

// SessionID identifies a session.
// This is an opaque identifier returned by List() and Start(), and passed to
// other methods to specify which session to operate on.
type SessionID string

// Info contains information about a session.
type Info struct {
	Name         string // Session name
	Created      string // Creation time (format varies by implementation)
	Attached     bool   // Whether someone is attached
	Windows      int    // Number of windows
	Activity     string // Last activity timestamp
	LastAttached string // Last time the session was attached
}

// Sessions is the portable interface for managing a collection of terminal sessions.
// It abstracts the underlying session manager (tmux, zellij, etc.).
//
// This interface manages a collection of named sessions. Methods that operate
// on a specific session take a SessionID parameter. Use List() to get existing
// session IDs, or Start() to create a new session and get its ID.
//
// Agent-specific behavior (readiness, hooks) is handled by the agent.Agents layer.
//
// Implementation-specific extensions (like tmux theming) should be handled directly
// by the implementation (e.g., *tmux.Tmux has ConfigureGasTownSession method).
type Sessions interface {
	// Lifecycle
	Start(name, workDir, command string) (SessionID, error)
	Stop(id SessionID) error
	Exists(id SessionID) (bool, error)

	// Communication
	Send(id SessionID, text string) error       // Send text to session (appends Enter)
	SendControl(id SessionID, key string) error // Send control sequence (no Enter, e.g., "C-c", "Down")
	Nudge(id SessionID, message string) error   // Robust message delivery (handles vim mode, retries)

	// Observation
	Capture(id SessionID, lines int) (string, error)
	IsRunning(id SessionID, processNames ...string) bool
	WaitFor(id SessionID, timeout time.Duration, processNames ...string) error

	// Management
	List() ([]SessionID, error)
	GetInfo(id SessionID) (*Info, error)
}
