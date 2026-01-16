package deacon

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
	ErrNotRunning = errors.New("deacon not running")
	// ErrAlreadyRunning is re-exported from agent package
	ErrAlreadyRunning = agent.ErrAlreadyRunning
)

// Manager handles deacon lifecycle operations.
type Manager struct {
	townRoot  string
	agentName string
	agents    agent.Agents
}

// NewManager creates a new deacon manager for a town.
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

// address returns the agent address for the deacon.
func (m *Manager) address() agent.AgentID {
	return agent.DeaconAddress()
}

// SessionName returns the tmux session name for the deacon.
// This is a package-level function for convenience.
func SessionName() string {
	return session.DeaconSessionName()
}

// SessionName returns the tmux session name for the deacon.
func (m *Manager) SessionName() string {
	return SessionName()
}

// deaconDir returns the working directory for the deacon.
func (m *Manager) deaconDir() string {
	return filepath.Join(m.townRoot, "deacon")
}

// Start starts the deacon session.
// Restarts are handled by daemon via ensureDeaconRunning on each heartbeat.
func (m *Manager) Start() error {
	deaconDir := m.deaconDir()

	// Ensure runtime settings exist
	runtimeConfig := config.LoadRuntimeConfig(m.townRoot)
	if err := runtime.EnsureSettingsForRole(deaconDir, "deacon", runtimeConfig); err != nil {
		return fmt.Errorf("ensuring runtime settings: %w", err)
	}

	// Build startup command (env vars are prepended by the Agents layer)
	startupCmd := config.BuildAgentCommand(m.agentName, "")

	// Start the agent (handles zombie detection, env vars, theming, and readiness)
	if err := m.agents.Start(m.address(), deaconDir, startupCmd); err != nil {
		return err // ErrAlreadyRunning or other errors
	}

	// Wait for agent to be ready
	_ = m.agents.WaitReady(m.address())

	// Propulsion is handled by the CLI prompt ("gt prime") passed at startup.
	// No need for post-startup nudges which are unreliable (text arrives before input is ready).

	return nil
}

// Stop stops the deacon session.
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

// IsRunning checks if the deacon session is active.
func (m *Manager) IsRunning() (bool, error) {
	return m.agents.Exists(m.address()), nil
}

// Status returns the current deacon status.
func (m *Manager) Status() (*Deacon, error) {
	state := StateStopped
	if m.agents.Exists(m.address()) {
		state = StateRunning
	}
	return &Deacon{
		State: state,
		Name:  m.SessionName(),
	}, nil
}

// Nudge sends a message to the deacon agent reliably.
func (m *Manager) Nudge(message string) error {
	if !m.agents.Exists(m.address()) {
		return ErrNotRunning
	}
	return m.agents.Nudge(m.address(), message)
}

// Agents returns the underlying Agents interface.
// Use this for operations not directly supported by Manager.
func (m *Manager) Agents() agent.Agents {
	return m.agents
}
