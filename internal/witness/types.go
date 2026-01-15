// Package witness provides the polecat monitoring agent.
package witness

import (
	"time"

	"github.com/steveyegge/gastown/internal/agent"
)

// State is an alias for agent.State for backwards compatibility.
type State = agent.State

// State constants - re-exported from agent package for backwards compatibility.
const (
	StateStopped = agent.StateStopped
	StateRunning = agent.StateRunning
	StatePaused  = agent.StatePaused
)

// Witness represents a rig's polecat monitoring agent.
type Witness struct {
	// RigName is the rig this witness monitors.
	RigName string `json:"rig_name"`

	// State is the current running state.
	State State `json:"state"`

	// StartedAt is when the witness was started.
	StartedAt *time.Time `json:"started_at,omitempty"`

	// MonitoredPolecats tracks polecats being monitored.
	MonitoredPolecats []string `json:"monitored_polecats,omitempty"`
}


