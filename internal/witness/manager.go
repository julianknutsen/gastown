package witness

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/steveyegge/gastown/internal/agent"
	"github.com/steveyegge/gastown/internal/rig"
	"github.com/steveyegge/gastown/internal/session"
)

// Manager handles witness status and monitoring operations.
// Start/Stop operations are handled via factory.Start()/factory.Agents().Stop().
// ZFC-compliant: tmux session existence is the source of truth.
type Manager struct {
	agents  agent.AgentObserver // Only needs Exists(), GetInfo() for status checks
	rigName string
	address agent.AgentID
	rig     *rig.Rig
}

// NewManager creates a new witness manager for a rig.
// The manager handles status queries.
// Lifecycle operations (Start/Stop) should use factory.Start()/factory.Agents().Stop().
//
// The agents parameter only needs to implement AgentObserver (Exists, GetInfo, List).
// In production, pass factory.Agents(). In tests, use agent.NewObserverDouble().
func NewManager(agents agent.AgentObserver, r *rig.Rig) *Manager {
	return &Manager{
		agents:  agents,
		rigName: r.Name,
		address: agent.WitnessAddress(r.Name),
		rig:     r,
	}
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

// Status returns the tmux session info if running, nil otherwise.
// ZFC-compliant: tmux session is the source of truth.
func (m *Manager) Status() (*session.Info, error) {
	if !m.agents.Exists(m.address) {
		return nil, nil
	}
	return m.agents.GetInfo(m.address)
}

// SessionName returns the tmux session name for this witness.
func (m *Manager) SessionName() string {
	return fmt.Sprintf("gt-%s-witness", m.rigName)
}

// IsRunning checks if the witness session is currently active.
// ZFC-compliant: checks tmux session existence.
func (m *Manager) IsRunning() bool {
	return m.agents.Exists(m.address)
}

// Address returns the agent's AgentID.
func (m *Manager) Address() agent.AgentID {
	return m.address
}

// Rig returns the rig this witness monitors.
func (m *Manager) Rig() *rig.Rig {
	return m.rig
}
