// Package beads provides convoy bead management.
package beads

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strings"
)

// generateShortID generates a short random ID (5 lowercase chars).
func generateShortID() string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return strings.ToLower(base32.StdEncoding.EncodeToString(b)[:5])
}

// ConvoyFields contains the fields specific to convoy beads.
// These are stored in the description.
type ConvoyFields struct {
	Notify   string // Notification address (e.g., email, webhook URL)
	Molecule string // Associated molecule ID for coordinated work
}

// FormatConvoyDescription formats the description field for a convoy bead.
func FormatConvoyDescription(trackedCount int, fields *ConvoyFields) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("Convoy tracking %d issues", trackedCount))

	if fields != nil {
		if fields.Notify != "" {
			lines = append(lines, fmt.Sprintf("Notify: %s", fields.Notify))
		}
		if fields.Molecule != "" {
			lines = append(lines, fmt.Sprintf("Molecule: %s", fields.Molecule))
		}
	}

	return strings.Join(lines, "\n")
}

// ParseConvoyFields extracts convoy fields from an issue's description.
func ParseConvoyFields(description string) *ConvoyFields {
	fields := &ConvoyFields{}
	hasFields := false

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
		if value == "" {
			continue
		}

		switch strings.ToLower(key) {
		case "notify":
			fields.Notify = value
			hasFields = true
		case "molecule":
			fields.Molecule = value
			hasFields = true
		}
	}

	if !hasFields {
		return nil
	}
	return fields
}

// CreateConvoy creates a convoy bead with an explicit ID.
// For new code using ForTown() context, prefer CreateTownConvoy() which auto-generates the ID.
//
// The ID format is: hq-cv-<shortid> (e.g., hq-cv-ab12cd)
//
// NOTE: This method extracts the prefix from the ID and passes --prefix to bd
// to enable routing via routes.jsonl.
func (b *Beads) CreateConvoy(id, title string, trackedCount int, fields *ConvoyFields) (*Issue, error) {
	description := FormatConvoyDescription(trackedCount, fields)

	return b.Create(CreateOptions{
		ID:          id,
		Title:       title,
		Description: description,
		BdType:      "convoy",
	})
}

// CreateTownConvoy creates a convoy bead in town beads with auto-generated ID.
// Can be called on any Beads instance (town operations are always available).
//
// ID format: {TownBeadsPrefix}-cv-{random}
// Example: New(townRoot).CreateTownConvoy("My Convoy", 5, fields)
// Creates: hq-cv-abc123
func (b *Beads) CreateTownConvoy(title string, trackedCount int, fields *ConvoyFields) (*Issue, error) {
	// All Beads instances can do town operations, no context check needed
	id := ConvoyID(generateShortID())
	return b.CreateConvoy(id, title, trackedCount, fields)
}

// ConvoyID generates a convoy bead ID using the town prefix.
// Format: hq-cv-<shortid> (e.g., hq-cv-ab12cd)
func ConvoyID(shortID string) string {
	return fmt.Sprintf("%s-cv-%s", TownBeadsPrefix, shortID)
}
