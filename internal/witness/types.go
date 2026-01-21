// Package witness provides the polecat monitoring agent.
package witness

// WitnessStatusOutput is used for JSON output of witness status command.
// ZFC-compliant: running state comes from tmux, not state files.
type WitnessStatusOutput struct {
	Running           bool     `json:"running"`
	RigName           string   `json:"rig_name"`
	Session           string   `json:"session,omitempty"`
	MonitoredPolecats []string `json:"monitored_polecats,omitempty"`
}
