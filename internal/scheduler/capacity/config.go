// Package capacity provides types and pure functions for the capacity-controlled
// dispatch scheduler. The impure orchestration (dispatch loop, enqueue, epic/convoy
// resolution) stays in cmd but uses types and pure functions from this package.
package capacity

import "time"

// SchedulerConfig configures the capacity scheduler for polecat dispatch.
// This is a town-wide setting (not per-rig) because capacity control is host-wide:
// API rate limits, memory, and CPU are shared resources across all rigs.
type SchedulerConfig struct {
	// Enabled controls whether the daemon auto-dispatches scheduled work.
	// Default: false (must opt in).
	Enabled bool `json:"enabled"`

	// MaxPolecats is the max concurrent polecats across ALL rigs.
	// Includes both scheduler-dispatched and directly-slung polecats.
	// nil/absent = default (10). Explicit 0 = unlimited (no cap).
	MaxPolecats *int `json:"max_polecats,omitempty"`

	// BatchSize is the number of beads to dispatch per heartbeat tick.
	// Limits spawn rate per 3-minute cycle.
	// nil/absent = default (3). Explicit 0 is rejected by config setter.
	BatchSize *int `json:"batch_size,omitempty"`

	// SpawnDelay is the delay between spawns to prevent Dolt lock contention.
	// Default: "2s".
	SpawnDelay string `json:"spawn_delay,omitempty"`
}

// DefaultSchedulerConfig returns a SchedulerConfig with sensible defaults.
func DefaultSchedulerConfig() *SchedulerConfig {
	defaultMax := 10
	defaultBatch := 3
	return &SchedulerConfig{
		Enabled:     false,
		MaxPolecats: &defaultMax,
		BatchSize:   &defaultBatch,
		SpawnDelay:  "2s",
	}
}

// GetMaxPolecats returns MaxPolecats or the default (10) if unset.
// Returns 0 if explicitly set to 0 (unlimited).
func (c *SchedulerConfig) GetMaxPolecats() int {
	if c == nil || c.MaxPolecats == nil {
		return 10
	}
	return *c.MaxPolecats
}

// GetBatchSize returns BatchSize or the default (3) if unset.
func (c *SchedulerConfig) GetBatchSize() int {
	if c == nil || c.BatchSize == nil {
		return 3
	}
	return *c.BatchSize
}

// GetSpawnDelay returns SpawnDelay as a duration, defaulting to 2s.
func (c *SchedulerConfig) GetSpawnDelay() time.Duration {
	if c == nil || c.SpawnDelay == "" {
		return 2 * time.Second
	}
	return ParseDurationOrDefault(c.SpawnDelay, 2*time.Second)
}

// ParseDurationOrDefault parses a Go duration string, returning fallback on error or empty input.
func ParseDurationOrDefault(s string, fallback time.Duration) time.Duration {
	if s == "" {
		return fallback
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return fallback
	}
	return d
}
