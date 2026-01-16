package plugin

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/steveyegge/gastown/internal/beads"
)

// RunResult represents the outcome of a plugin execution.
type RunResult string

const (
	ResultSuccess RunResult = "success"
	ResultFailure RunResult = "failure"
	ResultSkipped RunResult = "skipped"
)

// PluginRunRecord represents data for creating a plugin run bead.
type PluginRunRecord struct {
	PluginName string
	RigName    string
	Result     RunResult
	Body       string
}

// PluginRunBead represents a recorded plugin run from the ledger.
type PluginRunBead struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	Labels    []string  `json:"labels"`
	Result    RunResult `json:"-"` // Parsed from labels
}

// Recorder handles plugin run recording and querying.
type Recorder struct {
	townRoot string
}

// NewRecorder creates a new plugin run recorder.
func NewRecorder(townRoot string) *Recorder {
	return &Recorder{townRoot: townRoot}
}

// RecordRun creates an ephemeral bead for a plugin run.
// This is pure data writing - the caller decides what result to record.
func (r *Recorder) RecordRun(record PluginRunRecord) (string, error) {
	title := fmt.Sprintf("Plugin run: %s", record.PluginName)

	// Build labels
	labels := []string{
		"type:plugin-run",
		fmt.Sprintf("plugin:%s", record.PluginName),
		fmt.Sprintf("result:%s", record.Result),
	}
	if record.RigName != "" {
		labels = append(labels, fmt.Sprintf("rig:%s", record.RigName))
	}

	// BeadsOps Migration: cmd.Dir=r.townRoot, BEADS_DIR was set explicitly to prevent
	// prefix mismatches when redirects are in play
	beadsDir := beads.ResolveBeadsDir(r.townRoot)
	b := beads.NewWithBeadsDir(r.townRoot, beadsDir)

	issue, err := b.Create(beads.CreateOptions{
		Title:       title,
		Description: record.Body,
		Ephemeral:   true,
		Labels:      labels,
	})
	if err != nil {
		return "", fmt.Errorf("creating plugin run bead: %w", err)
	}

	return issue.ID, nil
}

// GetLastRun returns the most recent run for a plugin.
// Returns nil if no runs found.
func (r *Recorder) GetLastRun(pluginName string) (*PluginRunBead, error) {
	runs, err := r.queryRuns(pluginName, 1, "")
	if err != nil {
		return nil, err
	}
	if len(runs) == 0 {
		return nil, nil
	}
	return runs[0], nil
}

// GetRunsSince returns all runs for a plugin since the given duration.
// Duration format: "1h", "24h", "7d", etc.
func (r *Recorder) GetRunsSince(pluginName string, since string) ([]*PluginRunBead, error) {
	return r.queryRuns(pluginName, 0, since)
}

// queryRuns queries plugin run beads from the ledger.
func (r *Recorder) queryRuns(pluginName string, limit int, since string) ([]*PluginRunBead, error) {
	beadsDir := beads.ResolveBeadsDir(r.townRoot)
	b := beads.NewWithBeadsDir(r.townRoot, beadsDir)

	// Convert duration like "1h" to created-after format
	// bd supports relative dates with - prefix (e.g., -1h, -24h)
	createdAfter := ""
	if since != "" {
		if !strings.HasPrefix(since, "-") {
			createdAfter = "-" + since
		} else {
			createdAfter = since
		}
	}

	issues, err := b.List(beads.ListOptions{
		Labels:       []string{"type:plugin-run", fmt.Sprintf("plugin:%s", pluginName)},
		Limit:        limit,
		CreatedAfter: createdAfter,
		All:          true, // Include closed beads too
	})
	if err != nil {
		// Empty result is OK (no runs found)
		return nil, nil
	}

	// Convert to PluginRunBead with parsed result
	runs := make([]*PluginRunBead, 0, len(issues))
	for _, issue := range issues {
		run := &PluginRunBead{
			ID:     issue.ID,
			Title:  issue.Title,
			Labels: issue.Labels,
		}

		// Parse created_at
		if t, err := time.Parse(time.RFC3339, issue.CreatedAt); err == nil {
			run.CreatedAt = t
		}

		// Extract result from labels
		for _, label := range issue.Labels {
			if len(label) > 7 && label[:7] == "result:" {
				run.Result = RunResult(label[7:])
				break
			}
		}

		runs = append(runs, run)
	}

	return runs, nil
}

// parseJSON is a helper to unmarshal JSON.
func parseJSON(data []byte, v interface{}) error {
	if len(data) == 0 || string(data) == "[]\n" {
		return nil
	}
	return json.Unmarshal(data, v)
}

// CountRunsSince returns the count of runs for a plugin since the given duration.
// This is useful for cooldown gate evaluation.
func (r *Recorder) CountRunsSince(pluginName string, since string) (int, error) {
	runs, err := r.GetRunsSince(pluginName, since)
	if err != nil {
		return 0, err
	}
	return len(runs), nil
}
