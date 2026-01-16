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
// agents is the Agents implementation (real or test double) to use for agent lifecycle.
// townRoot is the path to the town root directory.
// agentName is the resolved agent to use (from config.ResolveRoleAgentName or command line).
func NewManager(agents agent.Agents, townRoot, agentName string) *Manager {
	return &Manager{
		townRoot:  townRoot,
		agentName: agentName,
		agents:    agents,
	}
}

// address returns the agent address for the mayor.
func (m *Manager) address() agent.AgentID {
	return agent.MayorAddress()
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
	if err := m.agents.Start(m.address(), mayorDir, startupCmd); err != nil {
		return err // ErrAlreadyRunning or other errors
	}

	// Wait for agent to be ready
	_ = m.agents.WaitReady(m.address())

	return nil
}

// Stop stops the mayor session.
// Returns ErrNotRunning if the agent was not running (for user messaging).
// Always cleans up zombie sessions (tmux exists but process dead).
func (m *Manager) Stop() error {
	wasRunning := m.agents.Exists(m.address())

	// Always call Stop to clean up zombies
	if err := m.agents.Stop(m.address(), true); err != nil {
		return err
	}

	if !wasRunning {
		return ErrNotRunning
	}
	return nil
}

// IsRunning checks if the mayor session is active.
func (m *Manager) IsRunning() (bool, error) {
	return m.agents.Exists(m.address()), nil
}

// Status returns the current mayor status.
func (m *Manager) Status() (*Mayor, error) {
	state := StateStopped
	if m.agents.Exists(m.address()) {
		state = StateRunning
	}
	return &Mayor{
		State: state,
		Name:  m.SessionName(),
	}, nil
}
