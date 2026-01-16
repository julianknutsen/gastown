package witness

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/steveyegge/gastown/internal/agent"
	"github.com/steveyegge/gastown/internal/config"
	"github.com/steveyegge/gastown/internal/rig"
	"github.com/steveyegge/gastown/internal/runtime"
)

// Common errors
var (
	ErrNotRunning = errors.New("witness not running")
	// ErrAlreadyRunning is re-exported from agent package
	ErrAlreadyRunning = agent.ErrAlreadyRunning
)

// Manager handles witness lifecycle and monitoring operations.
type Manager struct {
	rig          *rig.Rig
	agents       agent.Agents
	agentName    string
	workDir      string
	stateManager *agent.StateManager[Witness]
}

// NewManager creates a new witness manager for a rig.
// agents is the Agents implementation (real or test double) to use for agent lifecycle.
// agentName is the resolved agent to use (from config.ResolveRoleAgentName or command line).
func NewManager(agents agent.Agents, r *rig.Rig, agentName string) *Manager {
	return &Manager{
		rig:       r,
		agents:    agents,
		agentName: agentName,
		workDir:   r.Path,
		stateManager: agent.NewStateManager[Witness](r.Path, "witness.json", func() *Witness {
			return &Witness{
				RigName: r.Name,
				State:   StateStopped,
			}
		}),
	}
}

// address returns the agent address for the witness.
func (m *Manager) address() agent.AgentID {
	return agent.WitnessAddress(m.rig.Name)
}

// stateFile returns the path to the witness state file.
func (m *Manager) stateFile() string {
	return m.stateManager.StateFile()
}

// loadState loads witness state from disk.
func (m *Manager) loadState() (*Witness, error) {
	return m.stateManager.Load()
}

// saveState persists witness state to disk using atomic write.
func (m *Manager) saveState(w *Witness) error {
	return m.stateManager.Save(w)
}

// SessionName returns the tmux session name for this witness.
func (m *Manager) SessionName() string {
	return fmt.Sprintf("gt-%s-witness", m.rig.Name)
}

// ID returns the AgentID for the witness.
func (m *Manager) ID() agent.AgentID {
	return m.address()
}

// IsRunning checks if the witness session is currently active.
func (m *Manager) IsRunning() bool {
	return m.agents.Exists(m.address())
}

// Status returns the current witness status.
// Reconciles persisted state with actual agent existence.
func (m *Manager) Status() (*Witness, error) {
	w, err := m.loadState()
	if err != nil {
		return nil, err
	}

	// Reconcile state with reality (don't persist, just report accurately)
	if w.State == StateRunning && !m.agents.Exists(m.address()) {
		w.State = StateStopped // Agent crashed
	}

	// Update monitored polecats list (still useful for display)
	w.MonitoredPolecats = m.rig.Polecats

	return w, nil
}

// witnessDir returns the working directory for the witness.
// Prefers witness/rig/, falls back to witness/, then rig root.
func (m *Manager) witnessDir() string {
	witnessRigDir := filepath.Join(m.rig.Path, "witness", "rig")
	if _, err := os.Stat(witnessRigDir); err == nil {
		return witnessRigDir
	}

	witnessDir := filepath.Join(m.rig.Path, "witness")
	if _, err := os.Stat(witnessDir); err == nil {
		return witnessDir
	}

	return m.rig.Path
}

// Start starts the witness in a tmux session.
func (m *Manager) Start() error {
	w, err := m.loadState()
	if err != nil {
		return err
	}

	// Working directory
	witnessDir := m.witnessDir()

	// Ensure runtime settings exist in the working directory.
	runtimeConfig := config.LoadRuntimeConfig(m.rig.Path)
	if err := runtime.EnsureSettingsForRole(witnessDir, "witness", runtimeConfig); err != nil {
		return fmt.Errorf("ensuring runtime settings: %w", err)
	}

	// Build startup command (env vars are prepended by the Agents layer)
	command := config.BuildAgentCommand(m.agentName, "")

	// Start the agent (handles zombie detection, env vars, theming, and readiness)
	if err := m.agents.Start(m.address(), witnessDir, command); err != nil {
		return err // ErrAlreadyRunning or other errors
	}

	// Update state to running
	now := time.Now()
	w.State = StateRunning
	w.StartedAt = &now
	w.MonitoredPolecats = m.rig.Polecats
	if err := m.saveState(w); err != nil {
		_ = m.agents.Stop(m.address(), false) // best-effort cleanup on state save failure
		return fmt.Errorf("saving state: %w", err)
	}

	// Wait for agent to be ready
	_ = m.agents.WaitReady(m.address())

	// Propulsion is handled by the CLI prompt ("gt prime") passed at startup.

	return nil
}

// Stop stops the witness.
// Returns ErrNotRunning if the agent was not running (for user messaging).
// Always cleans up zombie sessions (tmux exists but process dead).
func (m *Manager) Stop() error {
	w, err := m.loadState()
	if err != nil {
		return err
	}

	stateWasRunning := w.State == StateRunning
	processWasRunning := m.agents.Exists(m.address())

	// Always call Stop to clean up zombies
	stopErr := m.agents.Stop(m.address(), true)

	w.State = StateStopped
	if err := m.saveState(w); err != nil {
		return err
	}

	if stopErr != nil {
		return stopErr
	}

	// Only report "not running" if neither state nor process indicated running
	if !stateWasRunning && !processWasRunning {
		return ErrNotRunning
	}
	return nil
}
