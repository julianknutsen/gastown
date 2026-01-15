// Package agent provides the Agents manager for agent processes.
package agent

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/steveyegge/gastown/internal/session"
)

// AgentID identifies an agent managed by Agents.
// This is an opaque identifier returned by Start() and used for subsequent operations.
type AgentID string

// ErrAlreadyRunning is returned when trying to start an already running agent.
var ErrAlreadyRunning = errors.New("agent already running")

// ErrNotRunning is returned when trying to operate on a non-running agent.
var ErrNotRunning = errors.New("agent not running")

// Agents is the interface for managing agent processes.
// Implementations handle lifecycle, readiness detection, and session management.
type Agents interface {
	// Start launches an agent process in a new session.
	Start(name, workDir, command string) (AgentID, error)

	// Stop terminates an agent process.
	Stop(id AgentID, graceful bool) error

	// Exists checks if an agent session exists.
	Exists(id AgentID) bool

	// WaitReady blocks until the agent is ready for input or times out.
	WaitReady(id AgentID) error

	// SessionID returns the underlying session ID for an agent.
	SessionID(id AgentID) session.SessionID

	// GetInfo returns information about an agent's session.
	GetInfo(id AgentID) (*session.Info, error)

	// Nudge sends a message to a running agent reliably.
	// Uses robust delivery (handles vim mode, retries) for inter-agent communication.
	Nudge(id AgentID, message string) error
}

// Implementation is the concrete implementation of the Agents interface.
// It provides runtime-aware lifecycle management.
type Implementation struct {
	sess   session.Sessions
	config *Config
}

// Ensure Implementation implements Agents
var _ Agents = (*Implementation)(nil)

// New creates a new Agents implementation.
func New(sess session.Sessions, config *Config) *Implementation {
	if config == nil {
		config = Claude()
	}
	return &Implementation{
		sess:   sess,
		config: config,
	}
}

// timeout returns the effective timeout for readiness detection.
func (a *Implementation) timeout() time.Duration {
	if a.config.Timeout > 0 {
		return a.config.Timeout
	}
	return 30 * time.Second // Default fallback
}

// prependEnvVars prepends environment variables to a command.
// Returns a command like "VAR1=val1 VAR2=val2 original-command".
// Precondition: envVars is non-empty (caller checks before calling).
func prependEnvVars(envVars map[string]string, command string) string {
	// Sort keys for deterministic output
	keys := make([]string, 0, len(envVars))
	for k := range envVars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, envVars[k]))
	}

	return strings.Join(parts, " ") + " " + command
}

// Start launches an agent process in a new session.
// Takes a name and returns the AgentID (which may differ if TownSessions adds a suffix).
// Handles zombie detection: if a session exists but the agent process is dead, it cleans up first.
// If EnvVars are configured, they are prepended to the command.
func (a *Implementation) Start(name, workDir, command string) (AgentID, error) {
	// Check for existing session and handle zombie detection
	sessionID := session.SessionID(name)
	exists, _ := a.sess.Exists(sessionID)
	if exists {
		// Session exists - check if agent is actually running (healthy vs zombie)
		if a.sess.IsRunning(sessionID, a.config.ProcessNames...) {
			return AgentID(sessionID), ErrAlreadyRunning
		}
		// Zombie - session alive but agent dead. Kill and recreate.
		if err := a.sess.Stop(sessionID); err != nil {
			return "", fmt.Errorf("killing zombie session: %w", err)
		}
	}

	// Prepend env vars to command if configured
	if len(a.config.EnvVars) > 0 {
		command = prependEnvVars(a.config.EnvVars, command)
	}

	// Create the session
	sessionID, err := a.sess.Start(name, workDir, command)
	if err != nil {
		return "", fmt.Errorf("starting session: %w", err)
	}

	// Run post-creation callback (theming, env vars, etc.)
	// This runs synchronously before the command executes
	if a.config.OnSessionCreated != nil {
		if err := a.config.OnSessionCreated(a.sess, sessionID); err != nil {
			// Cleanup session on callback failure
			_ = a.sess.Stop(sessionID)
			return "", fmt.Errorf("session setup: %w", err)
		}
	}

	// AgentID mirrors the session ID but with its own type
	id := AgentID(sessionID)

	// Wait for agent to be ready (non-blocking)
	go a.doWaitForReady(sessionID)

	return id, nil
}

// doWaitForReady implements the readiness wait logic.
func (a *Implementation) doWaitForReady(sessionID session.SessionID) error {
	// Run startup hook if defined (e.g., dismiss dialogs)
	if a.config.StartupHook != nil {
		_ = a.config.StartupHook(a.sess, sessionID) // Non-fatal
	}

	// Use checker if available
	if a.config.Checker != nil {
		return WaitForReady(a.sess, sessionID, a.timeout(), a.config.Checker)
	}

	// Fall back to startup delay
	if a.config.StartupDelay > 0 {
		time.Sleep(a.config.StartupDelay)
	}

	return nil
}

// IsReady checks if an agent is ready for input.
func (a *Implementation) IsReady(id AgentID) bool {
	sessionID := session.SessionID(id)

	// Check session exists first
	exists, _ := a.sess.Exists(sessionID)
	if !exists {
		return false
	}

	// Check output if we have a checker
	if a.config.Checker == nil {
		return false
	}
	output, err := a.sess.Capture(sessionID, 50)
	if err != nil {
		return false
	}
	return a.config.Checker.IsReady(output)
}

// WaitReady blocks until the agent is ready for input or times out.
func (a *Implementation) WaitReady(id AgentID) error {
	sessionID := session.SessionID(id)

	// Check session exists first
	exists, _ := a.sess.Exists(sessionID)
	if !exists {
		return ErrNotRunning
	}

	return a.doWaitForReady(sessionID)
}

// Stop terminates an agent process.
func (a *Implementation) Stop(id AgentID, graceful bool) error {
	sessionID := session.SessionID(id)

	// Check if session exists
	exists, _ := a.sess.Exists(sessionID)
	if !exists {
		return nil // Idempotent - nothing to stop
	}

	if graceful {
		_ = a.sess.SendControl(sessionID, "C-c")
		time.Sleep(100 * time.Millisecond)
	}

	if err := a.sess.Stop(sessionID); err != nil {
		return fmt.Errorf("stopping session: %w", err)
	}

	return nil
}

// Restart stops and restarts an agent with a new command.
func (a *Implementation) Restart(id AgentID, workDir, command string) error {
	if err := a.Stop(id, true); err != nil {
		return fmt.Errorf("stopping for restart: %w", err)
	}

	_, err := a.Start(string(id), workDir, command)
	return err
}

// Send sends text to an agent.
func (a *Implementation) Send(id AgentID, text string) error {
	sessionID := session.SessionID(id)
	return a.sess.Send(sessionID, text)
}

// SendControl sends a control sequence to an agent.
func (a *Implementation) SendControl(id AgentID, key string) error {
	sessionID := session.SessionID(id)
	return a.sess.SendControl(sessionID, key)
}

// Capture returns the last N lines from an agent's session.
func (a *Implementation) Capture(id AgentID, lines int) (string, error) {
	sessionID := session.SessionID(id)
	return a.sess.Capture(sessionID, lines)
}

// Sessions returns the underlying Sessions for role-specific configuration.
// Use this for operations like setting environment variables or applying theming.
func (a *Implementation) Sessions() session.Sessions {
	return a.sess
}

// SessionID returns the underlying session ID for an agent.
func (a *Implementation) SessionID(id AgentID) session.SessionID {
	return session.SessionID(id)
}

// Exists checks if an agent session exists.
func (a *Implementation) Exists(id AgentID) bool {
	exists, _ := a.sess.Exists(session.SessionID(id))
	return exists
}

// GetInfo returns information about an agent's session.
func (a *Implementation) GetInfo(id AgentID) (*session.Info, error) {
	sessionID := session.SessionID(id)
	return a.sess.GetInfo(sessionID)
}

// Nudge sends a message to a running agent reliably.
func (a *Implementation) Nudge(id AgentID, message string) error {
	sessionID := session.SessionID(id)
	return a.sess.Nudge(sessionID, message)
}
