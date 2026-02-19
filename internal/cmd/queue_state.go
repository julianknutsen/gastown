package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// QueueState represents the runtime operational state of the work queue.
// Stored at <townRoot>/.runtime/queue-state.json.
// Follows the pattern of deacon/redispatch-state.json for daemon operational state.
type QueueState struct {
	Paused            bool   `json:"paused"`
	PausedBy          string `json:"paused_by,omitempty"`
	PausedAt          string `json:"paused_at,omitempty"`
	LastDispatchAt    string `json:"last_dispatch_at,omitempty"`
	LastDispatchCount int    `json:"last_dispatch_count,omitempty"`
}

// queueStateFile returns the path to the queue state file.
func queueStateFile(townRoot string) string {
	return filepath.Join(townRoot, ".runtime", "queue-state.json")
}

// LoadQueueState loads the queue runtime state, returning a zero-value state if the file
// doesn't exist. This is intentional: absence means "not paused, never dispatched."
func LoadQueueState(townRoot string) (*QueueState, error) {
	path := queueStateFile(townRoot)
	data, err := os.ReadFile(path) //nolint:gosec // G304: path is constructed internally
	if err != nil {
		if os.IsNotExist(err) {
			return &QueueState{}, nil
		}
		return nil, err
	}

	var state QueueState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// SaveQueueState writes the queue runtime state to disk atomically.
// Uses write-to-temp + rename to prevent corruption from concurrent writers
// (e.g., dispatch RecordDispatch racing with gt queue pause).
func SaveQueueState(townRoot string, state *QueueState) error {
	path := queueStateFile(townRoot)
	dir := filepath.Dir(path)

	// Ensure .runtime directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	// Atomic write: temp file + rename
	tmp, err := os.CreateTemp(dir, ".queue-state-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) // clean up on rename failure
		return err
	}
	return nil
}

// SetPaused marks the queue as paused by the given actor.
func (s *QueueState) SetPaused(by string) {
	s.Paused = true
	s.PausedBy = by
	s.PausedAt = time.Now().UTC().Format(time.RFC3339)
}

// SetResumed marks the queue as resumed (not paused).
func (s *QueueState) SetResumed() {
	s.Paused = false
	s.PausedBy = ""
	s.PausedAt = ""
}

// RecordDispatch records a dispatch event.
func (s *QueueState) RecordDispatch(count int) {
	s.LastDispatchAt = time.Now().UTC().Format(time.RFC3339)
	s.LastDispatchCount = count
}
