package cmd

import (
	"fmt"
	"strings"
	"time"
)

// QueueMetadata holds queue dispatch parameters stored in a bead's description.
// Delimited by ---gt:queue:v1--- so it can be cleanly parsed without conflicting
// with existing description content. The namespaced delimiter avoids collision
// with user content that might contain generic markdown separators.
type QueueMetadata struct {
	TargetRig   string `json:"target_rig"`
	Formula     string `json:"formula,omitempty"`
	Args        string `json:"args,omitempty"`
	Vars        string `json:"vars,omitempty"` // newline-separated key=value pairs
	EnqueuedAt  string `json:"enqueued_at"`
	Merge       string `json:"merge,omitempty"`
	Convoy      string `json:"convoy,omitempty"`
	BaseBranch  string `json:"base_branch,omitempty"`
	NoMerge     bool   `json:"no_merge,omitempty"`
	Account     string `json:"account,omitempty"`
	Agent       string `json:"agent,omitempty"`
	HookRawBead      bool   `json:"hook_raw_bead,omitempty"`
	Owned            bool   `json:"owned,omitempty"`
	DispatchFailures int    `json:"dispatch_failures,omitempty"`
	LastFailure      string `json:"last_failure,omitempty"`
}

const queueMetadataDelimiter = "---gt:queue:v1---"

// FormatQueueMetadata formats metadata as key-value lines for bead description.
func FormatQueueMetadata(m *QueueMetadata) string {
	var lines []string
	lines = append(lines, queueMetadataDelimiter)

	if m.TargetRig != "" {
		lines = append(lines, fmt.Sprintf("target_rig: %s", m.TargetRig))
	}
	if m.Formula != "" {
		lines = append(lines, fmt.Sprintf("formula: %s", m.Formula))
	}
	if m.Args != "" {
		lines = append(lines, fmt.Sprintf("args: %s", m.Args))
	}
	// Vars are stored as repeated "var:" lines to avoid lossy delimiters.
	// Values may contain commas, so one line per var is the safe format.
	for _, v := range strings.Split(m.Vars, "\n") {
		v = strings.TrimSpace(v)
		if v != "" {
			lines = append(lines, fmt.Sprintf("var: %s", v))
		}
	}
	if m.EnqueuedAt != "" {
		lines = append(lines, fmt.Sprintf("enqueued_at: %s", m.EnqueuedAt))
	}
	if m.Merge != "" {
		lines = append(lines, fmt.Sprintf("merge: %s", m.Merge))
	}
	if m.Convoy != "" {
		lines = append(lines, fmt.Sprintf("convoy: %s", m.Convoy))
	}
	if m.BaseBranch != "" {
		lines = append(lines, fmt.Sprintf("base_branch: %s", m.BaseBranch))
	}
	if m.NoMerge {
		lines = append(lines, "no_merge: true")
	}
	if m.Account != "" {
		lines = append(lines, fmt.Sprintf("account: %s", m.Account))
	}
	if m.Agent != "" {
		lines = append(lines, fmt.Sprintf("agent: %s", m.Agent))
	}
	if m.HookRawBead {
		lines = append(lines, "hook_raw_bead: true")
	}
	if m.Owned {
		lines = append(lines, "owned: true")
	}
	if m.DispatchFailures > 0 {
		lines = append(lines, fmt.Sprintf("dispatch_failures: %d", m.DispatchFailures))
	}
	if m.LastFailure != "" {
		lines = append(lines, fmt.Sprintf("last_failure: %s", m.LastFailure))
	}

	return strings.Join(lines, "\n")
}

// ParseQueueMetadata extracts queue metadata from a bead description.
// Returns nil if no ---queue--- section is found.
func ParseQueueMetadata(description string) *QueueMetadata {
	idx := strings.Index(description, queueMetadataDelimiter)
	if idx < 0 {
		return nil
	}

	section := description[idx+len(queueMetadataDelimiter):]
	m := &QueueMetadata{}
	var varLines []string

	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Stop at a second delimiter or non-kv line
		if line == queueMetadataDelimiter {
			break
		}

		parts := strings.SplitN(line, ": ", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "target_rig":
			m.TargetRig = val
		case "formula":
			m.Formula = val
		case "args":
			m.Args = val
		case "var":
			varLines = append(varLines, val)
		case "vars":
			// Legacy: comma-separated format for backward compatibility
			varLines = append(varLines, strings.Split(val, ",")...)
		case "enqueued_at":
			m.EnqueuedAt = val
		case "merge":
			m.Merge = val
		case "convoy":
			m.Convoy = val
		case "base_branch":
			m.BaseBranch = val
		case "no_merge":
			m.NoMerge = val == "true"
		case "account":
			m.Account = val
		case "agent":
			m.Agent = val
		case "hook_raw_bead":
			m.HookRawBead = val == "true"
		case "no_boot":
			// Legacy: ignored. Dispatch always sets NoBoot=true.
		case "owned":
			m.Owned = val == "true"
		case "dispatch_failures":
			fmt.Sscanf(val, "%d", &m.DispatchFailures)
		case "last_failure":
			m.LastFailure = val
		}
	}

	if len(varLines) > 0 {
		m.Vars = strings.Join(varLines, "\n")
	}

	return m
}

// StripQueueMetadata removes the ---queue--- section from a bead description.
// Used when dequeuing a bead for dispatch (clean up the metadata).
func StripQueueMetadata(description string) string {
	idx := strings.Index(description, queueMetadataDelimiter)
	if idx < 0 {
		return description
	}
	return strings.TrimRight(description[:idx], "\n")
}

// NewQueueMetadata creates a QueueMetadata with the current timestamp.
func NewQueueMetadata(rigName string) *QueueMetadata {
	return &QueueMetadata{
		TargetRig:  rigName,
		EnqueuedAt: time.Now().UTC().Format(time.RFC3339),
	}
}
