package cmd

import (
	"fmt"
	"testing"
)

func TestSplitVars(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty", "", nil},
		{"single", "a=1", []string{"a=1"}},
		{"two newline-separated", "a=1\nb=2", []string{"a=1", "b=2"}},
		{"three newline-separated", "x=hello\ny=world\nz=42", []string{"x=hello", "y=world", "z=42"}},
		{"blank lines filtered", "a=1\n\nb=2\n", []string{"a=1", "b=2"}},
		{"whitespace trimmed", "  a=1  \n  b=2  ", []string{"a=1", "b=2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitVars(tt.input)
			if tt.want == nil {
				if got != nil {
					t.Errorf("splitVars(%q) = %v, want nil", tt.input, got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("splitVars(%q) = %v (len %d), want %v (len %d)",
					tt.input, got, len(got), tt.want, len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("splitVars(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestComputeDispatchCount(t *testing.T) {
	tests := []struct {
		name       string
		capacity   int // 0 = unlimited
		batchSize  int
		readyCount int
		want       int
	}{
		{"unlimited capacity, batch constrains", 0, 3, 10, 3},
		{"unlimited capacity, ready constrains", 0, 5, 2, 2},
		{"capacity constrains", 4, 10, 20, 4},
		{"batch constrains", 10, 3, 20, 3},
		{"ready constrains", 10, 5, 2, 2},
		{"all equal", 3, 3, 3, 3},
		{"zero ready", 10, 5, 0, 0},
		{"capacity 1", 1, 5, 10, 1},
		{"single bead", 0, 3, 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeDispatchCount(tt.capacity, tt.batchSize, tt.readyCount)
			if got != tt.want {
				t.Errorf("computeDispatchCount(%d, %d, %d) = %d, want %d",
					tt.capacity, tt.batchSize, tt.readyCount, got, tt.want)
			}
		})
	}
}

func TestCircuitBreakerMetadataFiltering(t *testing.T) {
	// Verifies that beads with dispatch_failures >= maxDispatchFailures
	// are correctly identified via metadata parsing.
	tests := []struct {
		name          string
		failures      int
		shouldBeSkipped bool
	}{
		{"zero failures", 0, false},
		{"below threshold", maxDispatchFailures - 1, false},
		{"at threshold", maxDispatchFailures, true},
		{"above threshold", maxDispatchFailures + 5, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := &QueueMetadata{
				TargetRig:        "test-rig",
				EnqueuedAt:       "2026-01-15T10:00:00Z",
				DispatchFailures: tt.failures,
			}
			if tt.failures > 0 {
				meta.LastFailure = "test error"
			}

			// Format → parse round-trip
			formatted := FormatQueueMetadata(meta)
			parsed := ParseQueueMetadata(formatted)
			if parsed == nil {
				t.Fatal("ParseQueueMetadata returned nil")
			}

			skipped := parsed.DispatchFailures >= maxDispatchFailures
			if skipped != tt.shouldBeSkipped {
				t.Errorf("failures=%d: skipped=%v, want %v (parsed.DispatchFailures=%d, max=%d)",
					tt.failures, skipped, tt.shouldBeSkipped, parsed.DispatchFailures, maxDispatchFailures)
			}
		})
	}
}

func TestCircuitBreakerCorruptedValue(t *testing.T) {
	// Verifies that corrupted dispatch_failures doesn't crash and defaults to 0
	desc := "---gt:queue:v1---\ntarget_rig: rig1\ndispatch_failures: not_a_number\nenqueued_at: 2026-01-15T10:00:00Z"
	parsed := ParseQueueMetadata(desc)
	if parsed == nil {
		t.Fatal("ParseQueueMetadata returned nil for corrupted failures")
	}
	if parsed.DispatchFailures != 0 {
		t.Errorf("corrupted dispatch_failures should default to 0, got %d", parsed.DispatchFailures)
	}
}

func TestStripQueueMetadata_DoubleDelimiter(t *testing.T) {
	desc := "Task desc\n---gt:queue:v1---\ntarget_rig: rig1\n---gt:queue:v1---\ntarget_rig: rig2"
	stripped := StripQueueMetadata(desc)
	if stripped != "Task desc" {
		t.Errorf("StripQueueMetadata with double delimiter: got %q, want %q", stripped, "Task desc")
	}
}

func TestStripQueueMetadata_DelimiterOnly(t *testing.T) {
	desc := "---gt:queue:v1---\ntarget_rig: rig1\nenqueued_at: 2026-01-15T10:00:00Z"
	stripped := StripQueueMetadata(desc)
	if stripped != "" {
		t.Errorf("StripQueueMetadata with delimiter-only: got %q, want empty", stripped)
	}
}

func TestStripQueueMetadata_DelimiterInUserContent(t *testing.T) {
	// If user content contains the delimiter text on a line by itself,
	// StripQueueMetadata strips from the first occurrence. This is a known
	// edge case — the delimiter is chosen to be unlikely in real descriptions.
	desc := "User wrote ---gt:queue:v1--- as text\n---gt:queue:v1---\ntarget_rig: rig1"
	stripped := StripQueueMetadata(desc)
	// Strips from the first line containing the delimiter
	if stripped != "User wrote " {
		t.Errorf("StripQueueMetadata: got %q, want %q", stripped, "User wrote ")
	}
}

func TestCapacityDisplayFormat(t *testing.T) {
	// Verify the capacity display string logic
	tests := []struct {
		maxPolecats int
		capacity    int
		wantStr     string
	}{
		{0, 0, "unlimited"},
		{10, 7, "7 free of 10"},
		{5, 1, "1 free of 5"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("max=%d", tt.maxPolecats), func(t *testing.T) {
			capStr := "unlimited"
			if tt.maxPolecats > 0 {
				capStr = fmt.Sprintf("%d free of %d", tt.capacity, tt.maxPolecats)
			}
			if capStr != tt.wantStr {
				t.Errorf("capacity display: got %q, want %q", capStr, tt.wantStr)
			}
		})
	}
}
