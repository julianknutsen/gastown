package polecat

import (
	"os"
	"sort"
	"testing"

	"github.com/steveyegge/gastown/internal/rig"
)

// TestReconcilePoolWithOpenAgentBeads verifies that names with open agent beads are reserved.
// This prevents fresh polecat spawns from reusing names that have:
// - Active hooked work (polecat working or crashed)
// - Finished work waiting for nuke (gt done ran, Witness hasn't nuked yet)
// Names become available only after the agent bead is closed (via nuke).
func TestReconcilePoolWithOpenAgentBeads(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		namesWithDirs     []string
		namesWithSessions []string
		namesWithHooks    []string // names with open agent beads (working OR waiting for nuke)
		wantInUse         []string // names that should be marked in-use (unavailable)
	}{
		{
			name:              "no dirs, no sessions, no open agents - all available",
			namesWithDirs:     []string{},
			namesWithSessions: []string{},
			namesWithHooks:    []string{},
			wantInUse:         []string{},
		},
		{
			name:              "open agent without dir - reserved (crashed or waiting for nuke)",
			namesWithDirs:     []string{},
			namesWithSessions: []string{},
			namesWithHooks:    []string{"furiosa"},
			wantInUse:         []string{"furiosa"},
		},
		{
			name:              "open agent with dir - in use (both reasons)",
			namesWithDirs:     []string{"furiosa"},
			namesWithSessions: []string{},
			namesWithHooks:    []string{"furiosa"},
			wantInUse:         []string{"furiosa"},
		},
		{
			name:              "mixed: one with dir, one with open agent only",
			namesWithDirs:     []string{"toast"},
			namesWithSessions: []string{},
			namesWithHooks:    []string{"nux"},
			wantInUse:         []string{"nux", "toast"},
		},
		{
			name:              "orphan session does not block, but open agent does",
			namesWithDirs:     []string{},
			namesWithSessions: []string{"capable"},
			namesWithHooks:    []string{"capable"},
			wantInUse:         []string{"capable"},
		},
		{
			name:              "multiple open agents reserve multiple names",
			namesWithDirs:     []string{},
			namesWithSessions: []string{},
			namesWithHooks:    []string{"furiosa", "nux", "slit"},
			wantInUse:         []string{"furiosa", "nux", "slit"},
		},
		{
			name:              "complex: dirs, sessions, and open agents",
			namesWithDirs:     []string{"toast"},
			namesWithSessions: []string{"toast", "capable"},
			namesWithHooks:    []string{"nux", "slit"},
			wantInUse:         []string{"nux", "slit", "toast"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "reconcile-hook-test-*")
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = os.RemoveAll(tmpDir) }()

			// Use "myrig" which hashes to mad-max theme
			r := &rig.Rig{
				Name: "myrig",
				Path: tmpDir,
			}
			m := NewManager(r, nil, nil)

			// Call ReconcilePoolWith with all three sources
			m.ReconcilePoolWith(tt.namesWithDirs, tt.namesWithSessions, tt.namesWithHooks)

			// Verify in-use names
			gotInUse := m.namePool.ActiveNames()
			sort.Strings(gotInUse)
			sort.Strings(tt.wantInUse)

			if len(gotInUse) != len(tt.wantInUse) {
				t.Errorf("in-use count: got %d (%v), want %d (%v)",
					len(gotInUse), gotInUse, len(tt.wantInUse), tt.wantInUse)
				return
			}
			for i := range tt.wantInUse {
				if i >= len(gotInUse) || gotInUse[i] != tt.wantInUse[i] {
					t.Errorf("in-use names: got %v, want %v", gotInUse, tt.wantInUse)
					break
				}
			}
		})
	}
}

// TestAllocateNameSkipsNamesWithOpenAgentBeads verifies that AllocateName()
// skips names where the agent bead is still open (working or waiting for nuke).
func TestAllocateNameSkipsNamesWithOpenAgentBeads(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "allocate-hook-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Use "myrig" which hashes to mad-max theme
	r := &rig.Rig{
		Name: "myrig",
		Path: tmpDir,
	}
	m := NewManager(r, nil, nil)

	// furiosa (first name in mad-max) has an open agent bead - should be skipped
	// This simulates either: polecat working, polecat crashed, or gt done waiting for nuke
	m.ReconcilePoolWith([]string{}, []string{}, []string{"furiosa"})

	// Allocate should skip furiosa and return nux (second name)
	name, err := m.namePool.Allocate()
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}

	if name == "furiosa" {
		t.Errorf("allocated reserved name 'furiosa', should have skipped")
	}
	if name != "nux" {
		t.Errorf("expected 'nux' (second name, first available), got %q", name)
	}
}

// TestAllocateNameReusesNameAfterAgentClosed verifies that names become
// available again after the agent bead is closed (via nuke).
func TestAllocateNameReusesNameAfterAgentClosed(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "allocate-cleared-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Use "myrig" which hashes to mad-max theme
	r := &rig.Rig{
		Name: "myrig",
		Path: tmpDir,
	}
	m := NewManager(r, nil, nil)

	// First: furiosa has an open agent bead (working or waiting for nuke)
	m.ReconcilePoolWith([]string{}, []string{}, []string{"furiosa"})

	// Verify furiosa is reserved
	name, _ := m.namePool.Allocate()
	if name == "furiosa" {
		t.Errorf("furiosa should be reserved (open agent bead)")
	}
	m.namePool.Release(name) // Release what we got

	// Now: simulate nuke closing the agent bead - no more open agent beads
	m.ReconcilePoolWith([]string{}, []string{}, []string{})

	// furiosa should now be available (agent bead closed)
	name, err = m.namePool.Allocate()
	if err != nil {
		t.Fatalf("Allocate after nuke: %v", err)
	}

	if name != "furiosa" {
		t.Errorf("expected 'furiosa' (first available after agent closed), got %q", name)
	}
}

// TestAllocateNameMultipleOpenAgentsReservesAll verifies that multiple names
// with open agent beads are all reserved.
func TestAllocateNameMultipleOpenAgentsReservesAll(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "allocate-multi-agent-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Use "myrig" which hashes to mad-max theme
	r := &rig.Rig{
		Name: "myrig",
		Path: tmpDir,
	}
	m := NewManager(r, nil, nil)

	// First three names have open agent beads
	m.ReconcilePoolWith([]string{}, []string{}, []string{"furiosa", "nux", "slit"})

	// Should get rictus (4th name)
	name, err := m.namePool.Allocate()
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}

	// Should skip all names with open agents
	if name == "furiosa" || name == "nux" || name == "slit" {
		t.Errorf("allocated reserved name %q, should have skipped", name)
	}
	if name != "rictus" {
		t.Errorf("expected 'rictus' (4th name, first available), got %q", name)
	}
}

// TestExtractPolecatNameFromID verifies the helper extracts polecat names correctly.
func TestExtractPolecatNameFromID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		agentID  string
		wantName string
	}{
		{"gastown/polecats/furiosa", "furiosa"},
		{"gastown/polecats/nux", "nux"},
		{"myrig/polecats/toast", "toast"},
		{"gastown/witness", ""},           // Not a polecat
		{"gastown/polecats", ""},          // Incomplete
		{"furiosa", ""},                   // No structure
		{"a/b/c/d", ""},                   // Too many parts
		{"gastown/mayor/furiosa", ""},     // Wrong middle part
	}

	for _, tt := range tests {
		t.Run(tt.agentID, func(t *testing.T) {
			got := extractPolecatNameFromID(tt.agentID)
			if got != tt.wantName {
				t.Errorf("extractPolecatNameFromID(%q) = %q, want %q", tt.agentID, got, tt.wantName)
			}
		})
	}
}
