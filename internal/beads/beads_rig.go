// Package beads provides rig identity bead management.
package beads

import (
	"fmt"
	"strings"
)

// RigFields contains the fields specific to rig identity beads.
type RigFields struct {
	Repo   string // Git URL for the rig's repository
	Prefix string // Beads prefix for this rig (e.g., "gt", "bd")
	State  string // Operational state: active, archived, maintenance
}

// FormatRigDescription formats the description field for a rig identity bead.
func FormatRigDescription(name string, fields *RigFields) string {
	if fields == nil {
		return ""
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("Rig identity bead for %s.", name))
	lines = append(lines, "")

	if fields.Repo != "" {
		lines = append(lines, fmt.Sprintf("repo: %s", fields.Repo))
	}
	if fields.Prefix != "" {
		lines = append(lines, fmt.Sprintf("prefix: %s", fields.Prefix))
	}
	if fields.State != "" {
		lines = append(lines, fmt.Sprintf("state: %s", fields.State))
	}

	return strings.Join(lines, "\n")
}

// ParseRigFields extracts rig fields from an issue's description.
func ParseRigFields(description string) *RigFields {
	fields := &RigFields{}

	for _, line := range strings.Split(description, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		colonIdx := strings.Index(line, ":")
		if colonIdx == -1 {
			continue
		}

		key := strings.TrimSpace(line[:colonIdx])
		value := strings.TrimSpace(line[colonIdx+1:])
		if value == "null" || value == "" {
			value = ""
		}

		switch strings.ToLower(key) {
		case "repo":
			fields.Repo = value
		case "prefix":
			fields.Prefix = value
		case "state":
			fields.State = value
		}
	}

	return fields
}

// CreateRigBead creates a rig identity bead for tracking rig metadata.
// The ID format is: <prefix>-rig-<name> (e.g., gt-rig-gastown)
// Use RigBeadID() helper to generate correct IDs.
// The created_by field is populated from BD_ACTOR env var for provenance tracking.
//
// NOTE: This method extracts the prefix from the ID and passes --prefix to bd
// to enable routing via routes.jsonl.
func (b *Beads) CreateRigBead(id, title string, fields *RigFields) (*Issue, error) {
	description := FormatRigDescription(title, fields)

	return b.Create(CreateOptions{
		ID:          id,
		Title:       title,
		Description: description,
		Labels:      []string{"gt:rig"},
	})
}

// RigBeadIDWithPrefix generates a rig identity bead ID using the specified prefix.
// Format: <prefix>-rig-<name> (e.g., gt-rig-gastown)
func RigBeadIDWithPrefix(prefix, name string) string {
	return fmt.Sprintf("%s-rig-%s", prefix, name)
}

// RigBeadID generates a rig identity bead ID using "gt" prefix.
// For non-gastown rigs, use RigBeadIDWithPrefix with the rig's configured prefix.
func RigBeadID(name string) string {
	return RigBeadIDWithPrefix("gt", name)
}
