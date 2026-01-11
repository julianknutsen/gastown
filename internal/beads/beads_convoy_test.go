package beads

import (
	"strings"
	"testing"
)

func TestFormatConvoyDescription(t *testing.T) {
	tests := []struct {
		name         string
		trackedCount int
		fields       *ConvoyFields
		want         string
	}{
		{
			name:         "no fields",
			trackedCount: 5,
			fields:       nil,
			want:         "Convoy tracking 5 issues",
		},
		{
			name:         "empty fields",
			trackedCount: 3,
			fields:       &ConvoyFields{},
			want:         "Convoy tracking 3 issues",
		},
		{
			name:         "with notify",
			trackedCount: 2,
			fields:       &ConvoyFields{Notify: "team@example.com"},
			want:         "Convoy tracking 2 issues\nNotify: team@example.com",
		},
		{
			name:         "with molecule",
			trackedCount: 4,
			fields:       &ConvoyFields{Molecule: "mol-xyz"},
			want:         "Convoy tracking 4 issues\nMolecule: mol-xyz",
		},
		{
			name:         "all fields",
			trackedCount: 10,
			fields: &ConvoyFields{
				Notify:   "webhook://example.com/notify",
				Molecule: "mol-abc123",
			},
			want: "Convoy tracking 10 issues\nNotify: webhook://example.com/notify\nMolecule: mol-abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatConvoyDescription(tt.trackedCount, tt.fields)
			if got != tt.want {
				t.Errorf("FormatConvoyDescription() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestParseConvoyFields(t *testing.T) {
	tests := []struct {
		name        string
		description string
		wantNil     bool
		wantFields  *ConvoyFields
	}{
		{
			name:        "empty description",
			description: "",
			wantNil:     true,
		},
		{
			name:        "no convoy fields",
			description: "Convoy tracking 5 issues",
			wantNil:     true,
		},
		{
			name:        "with notify",
			description: "Convoy tracking 3 issues\nNotify: team@example.com",
			wantFields:  &ConvoyFields{Notify: "team@example.com"},
		},
		{
			name:        "with molecule",
			description: "Convoy tracking 2 issues\nMolecule: mol-xyz",
			wantFields:  &ConvoyFields{Molecule: "mol-xyz"},
		},
		{
			name:        "all fields",
			description: "Convoy tracking 10 issues\nNotify: webhook://example.com\nMolecule: mol-abc",
			wantFields: &ConvoyFields{
				Notify:   "webhook://example.com",
				Molecule: "mol-abc",
			},
		},
		{
			name:        "case insensitive",
			description: "NOTIFY: test@example.com\nMOLECULE: mol-test",
			wantFields: &ConvoyFields{
				Notify:   "test@example.com",
				Molecule: "mol-test",
			},
		},
		{
			name:        "extra whitespace",
			description: "  Notify:   spaced@example.com  \n  Molecule:  mol-spaced  ",
			wantFields: &ConvoyFields{
				Notify:   "spaced@example.com",
				Molecule: "mol-spaced",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := ParseConvoyFields(tt.description)

			if tt.wantNil {
				if fields != nil {
					t.Errorf("ParseConvoyFields() = %+v, want nil", fields)
				}
				return
			}

			if fields == nil {
				t.Fatal("ParseConvoyFields() = nil, want non-nil")
			}

			if fields.Notify != tt.wantFields.Notify {
				t.Errorf("Notify = %q, want %q", fields.Notify, tt.wantFields.Notify)
			}
			if fields.Molecule != tt.wantFields.Molecule {
				t.Errorf("Molecule = %q, want %q", fields.Molecule, tt.wantFields.Molecule)
			}
		})
	}
}

func TestConvoyFieldsRoundTrip(t *testing.T) {
	original := &ConvoyFields{
		Notify:   "webhook://example.com/convoy",
		Molecule: "mol-roundtrip",
	}

	// Format to string
	formatted := FormatConvoyDescription(5, original)

	// Parse back
	parsed := ParseConvoyFields(formatted)

	if parsed == nil {
		t.Fatal("round-trip parse returned nil")
	}

	if parsed.Notify != original.Notify {
		t.Errorf("round-trip Notify = %q, want %q", parsed.Notify, original.Notify)
	}
	if parsed.Molecule != original.Molecule {
		t.Errorf("round-trip Molecule = %q, want %q", parsed.Molecule, original.Molecule)
	}
}

func TestConvoyID(t *testing.T) {
	tests := []struct {
		shortID string
		want    string
	}{
		{"ab12cd", "hq-cv-ab12cd"},
		{"xyz789", "hq-cv-xyz789"},
		{"a1b2c3", "hq-cv-a1b2c3"},
	}

	for _, tt := range tests {
		t.Run(tt.shortID, func(t *testing.T) {
			got := ConvoyID(tt.shortID)
			if got != tt.want {
				t.Errorf("ConvoyID(%q) = %q, want %q", tt.shortID, got, tt.want)
			}

			// Verify the prefix is extractable
			prefix := extractPrefix(got)
			if prefix != "hq" {
				t.Errorf("extractPrefix(ConvoyID(%q)) = %q, want 'hq'", tt.shortID, prefix)
			}

			// Verify the format
			if !strings.HasPrefix(got, "hq-cv-") {
				t.Errorf("ConvoyID(%q) should have 'hq-cv-' prefix", tt.shortID)
			}
		})
	}
}
