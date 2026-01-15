package mayor

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/steveyegge/gastown/internal/agent"
	"github.com/steveyegge/gastown/internal/config"
	"github.com/steveyegge/gastown/internal/runtime"
	"github.com/steveyegge/gastown/internal/session"
)

// Common errors
var (
	ErrNotRunning = errors.New("mayor not running")
	// ErrAlreadyRunning is re-exported from agent package
	ErrAlreadyRunning = agent.ErrAlreadyRunning
)

// Manager handles mayor lifecycle operations.
type Manager struct {
	townRoot  string
	agentName string
	agents    agent.Agents
}

// NewManager creates a new mayor manager for a town.
// The Agents should be backed by a Sessions pre-configured with session settings
// (e.g., tmux.WithSessionConfig). For tests, pass an agent.Double.
func NewManager(townRoot, agentName string, agents agent.Agents) *Manager {
	return &Manager{
		townRoot:  townRoot,
		agentName: agentName,
		agents:    agents,
	}
}

// SessionName returns the tmux session name for the mayor.
// This is a package-level function for convenience.
func SessionName() string {
	return session.MayorSessionName()
}

// SessionName returns the tmux session name for the mayor.
func (m *Manager) SessionName() string {
	return SessionName()
}

// mayorDir returns the working directory for the mayor.
func (m *Manager) mayorDir() string {
	return filepath.Join(m.townRoot, "mayor")
}

// Start starts the mayor session.
func (m *Manager) Start() error {
	mayorDir := m.mayorDir()

	// Ensure runtime settings exist
	runtimeConfig := config.LoadRuntimeConfig(m.townRoot)
	if err := runtime.EnsureSettingsForRole(mayorDir, "mayor", runtimeConfig); err != nil {
		return fmt.Errorf("ensuring runtime settings: %w", err)
	}

	// Build startup command (env vars are set on the session by tmux.WithSessionConfig)
	startupCmd := config.BuildAgentCommand(m.agentName, "")

	// Start the agent (handles zombie detection and readiness)
	agentID, err := m.agents.Start(m.SessionName(), mayorDir, startupCmd)
	if err != nil {
		return err // ErrAlreadyRunning or other errors
	}

	// Wait for agent to be ready
	_ = m.agents.WaitReady(agentID)

	return nil
}

// Stop stops the mayor session.
func (m *Manager) Stop() error {
	agentID := agent.AgentID(m.SessionName())

	// Check if agent exists
	if !m.agents.Exists(agentID) {
		return ErrNotRunning
	}

	// Stop gracefully
	return m.agents.Stop(agentID, true)
}

// IsRunning checks if the mayor session is active.
func (m *Manager) IsRunning() (bool, error) {
	return m.agents.Exists(agent.AgentID(m.SessionName())), nil
}

// Status returns information about the mayor session.
func (m *Manager) Status() (*session.Info, error) {
	agentID := agent.AgentID(m.SessionName())

	if !m.agents.Exists(agentID) {
		return nil, ErrNotRunning
	}

	return m.agents.GetInfo(agentID)
}
