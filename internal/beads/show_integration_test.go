//go:build integration

package beads

import (
	"errors"
	"testing"
)

// === Show Tests ===

func TestShow_ExistingBead(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-show")
	bd := env.NewForTown()

	// Create a test bead using Beads API
	created := env.CreateTestBead(t, "Test Show Bead")

	issue, err := bd.Show(created.ID)
	if err != nil {
		t.Fatalf("Show() failed: %v", err)
	}
	if issue == nil {
		t.Fatal("Show() returned nil issue")
	}
	if issue.ID != created.ID {
		t.Errorf("ID = %q, want %q", issue.ID, created.ID)
	}
	if issue.Title != "Test Show Bead" {
		t.Errorf("Title = %q, want %q", issue.Title, "Test Show Bead")
	}
}

func TestShow_NonExistent_ReturnsErrNotFound(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-show-notfound")
	bd := env.NewForTown()

	_, err := bd.Show("hq-nonexistent")
	if err == nil {
		t.Fatal("Show() for non-existent should return error")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Expected ErrNotFound, got: %v", err)
	}
}

func TestShow_EmptyID_Error(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-show-empty")
	bd := env.NewForTown()

	_, err := bd.Show("")
	if err == nil {
		t.Error("Show('') should return error")
	}
}

// === ShowMultiple Tests ===

func TestShowMultiple_AllExist(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-showmulti")
	bd := env.NewForTown()

	// Create test beads using Beads API
	bead1 := env.CreateTestBead(t, "Multi 1")
	bead2 := env.CreateTestBead(t, "Multi 2")

	result, err := bd.ShowMultiple([]string{bead1.ID, bead2.ID})
	if err != nil {
		t.Fatalf("ShowMultiple() failed: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("Expected 2 results, got %d", len(result))
	}
	if result[bead1.ID] == nil {
		t.Errorf("Missing %s in results", bead1.ID)
	}
	if result[bead2.ID] == nil {
		t.Errorf("Missing %s in results", bead2.ID)
	}
}

func TestShowMultiple_SomeMissing(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-showmulti-missing")
	bd := env.NewForTown()

	// Create only one bead
	bead1 := env.CreateTestBead(t, "Exists")

	result, err := bd.ShowMultiple([]string{bead1.ID, "hq-missing"})
	// ShowMultiple should not error for missing - just not include them
	if err != nil {
		t.Fatalf("ShowMultiple() failed: %v", err)
	}
	if result[bead1.ID] == nil {
		t.Errorf("Missing %s in results", bead1.ID)
	}
	// hq-missing should not be in result (not an error)
}

func TestShowMultiple_EmptyList(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-showmulti-empty")
	bd := env.NewForTown()

	result, err := bd.ShowMultiple([]string{})
	if err != nil {
		t.Fatalf("ShowMultiple([]) failed: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("Expected empty result, got %d items", len(result))
	}
}

// === List Tests ===

func TestList_All(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-list-all")
	bd := env.NewForTown()

	// Create test beads using Beads API
	env.CreateTestBead(t, "List 1")
	env.CreateTestBead(t, "List 2")

	issues, err := bd.List(ListOptions{})
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}
	if len(issues) < 2 {
		t.Errorf("Expected at least 2 issues, got %d", len(issues))
	}
}

func TestList_ByStatus(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-list-status")
	bd := env.NewForTown()

	env.CreateTestBead(t, "Status Test")

	issues, err := bd.List(ListOptions{Status: "open"})
	if err != nil {
		t.Fatalf("List(status=open) failed: %v", err)
	}
	// All new beads are open by default
	if len(issues) == 0 {
		t.Error("Expected at least 1 open issue")
	}
}

func TestList_ByLabel(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-list-label")
	bd := env.NewForTown()

	// Create bead with label using Beads API
	env.CreateTestBead(t, "Labeled", "test:label")

	issues, err := bd.List(ListOptions{Label: "test:label"})
	if err != nil {
		t.Fatalf("List(label) failed: %v", err)
	}
	if len(issues) == 0 {
		t.Error("Expected at least 1 issue with label")
	}
}

// === Ready/Blocked Tests ===

func TestReady_ReturnsOpenIssues(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-ready")
	bd := env.NewForTown()

	env.CreateTestBead(t, "Ready Test")

	issues, err := bd.Ready()
	if err != nil {
		t.Fatalf("Ready() failed: %v", err)
	}
	// Ready should return issues that are open and not blocked
	_ = issues // May be empty or have items depending on state
}

func TestBlocked_ReturnsBlockedIssues(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-blocked")
	bd := env.NewForTown()

	issues, err := bd.Blocked()
	if err != nil {
		t.Fatalf("Blocked() failed: %v", err)
	}
	// Should return blocked issues (likely empty in fresh env)
	_ = issues
}
