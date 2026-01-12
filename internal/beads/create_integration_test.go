//go:build integration

package beads

import (
	"strings"
	"testing"
)

// === Create Tests ===

func TestCreate_Basic(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-create")
	bd := env.NewForTown()

	issue, err := bd.Create(CreateOptions{
		Title: "Test Create",
		Type:  "task",
	})
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}
	if issue == nil {
		t.Fatal("Create() returned nil")
	}
	if issue.Title != "Test Create" {
		t.Errorf("Title = %q, want %q", issue.Title, "Test Create")
	}
	// gt install uses hq- prefix
	if !strings.HasPrefix(issue.ID, "hq-") {
		t.Errorf("ID = %q, should start with 'hq-'", issue.ID)
	}
}

func TestCreate_WithLabels(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-create-labels")
	bd := env.NewForTown()

	issue, err := bd.Create(CreateOptions{
		Title:  "Test With Labels",
		Type:   "task",
		Labels: []string{"test:label", "feature:new"},
	})
	if err != nil {
		t.Fatalf("Create() with labels failed: %v", err)
	}
	if issue == nil {
		t.Fatal("Create() returned nil")
	}

	// Verify labels via Show (create response may not include labels)
	shown, err := bd.Show(issue.ID)
	if err != nil {
		t.Fatalf("Show() failed: %v", err)
	}

	hasLabel := false
	for _, l := range shown.Labels {
		if l == "test:label" {
			hasLabel = true
			break
		}
	}
	if !hasLabel {
		t.Errorf("Missing test:label in issue labels: %v", shown.Labels)
	}
}

func TestCreate_WithDescription(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-create-desc")
	bd := env.NewForTown()

	issue, err := bd.Create(CreateOptions{
		Title:       "Test With Description",
		Type:        "task",
		Description: "This is a detailed description",
	})
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	// Verify description via Show
	shown, err := bd.Show(issue.ID)
	if err != nil {
		t.Fatalf("Show() failed: %v", err)
	}
	if shown.Description != "This is a detailed description" {
		t.Errorf("Description = %q, want %q", shown.Description, "This is a detailed description")
	}
}

func TestCreate_WithType(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-create-type")
	bd := env.NewForTown()

	// BdType sets the actual issue_type field; Type is deprecated and adds gt:label
	issue, err := bd.Create(CreateOptions{
		Title:  "Test Bug Type",
		BdType: "bug",
	})
	if err != nil {
		t.Fatalf("Create(type=bug) failed: %v", err)
	}

	shown, err := bd.Show(issue.ID)
	if err != nil {
		t.Fatalf("Show() failed: %v", err)
	}
	if shown.Type != "bug" {
		t.Errorf("Type = %q, want %q", shown.Type, "bug")
	}
}

func TestCreate_EmptyTitle_Error(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-create-empty")
	bd := env.NewForTown()

	_, err := bd.Create(CreateOptions{
		Title: "",
		Type:  "task",
	})
	if err == nil {
		t.Error("Create() with empty title should fail")
	}
}

// === CreateWithID Tests ===

func TestCreateWithID_ExplicitID(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-createwithid")
	bd := env.NewForTown()

	// gt install uses hq- prefix, so use hq- prefix for explicit ID
	issue, err := bd.Create(CreateOptions{
		ID:    "hq-explicit-test-id",
		Title: "Explicit ID Test",
		Type:  "task",
	})
	if err != nil {
		t.Fatalf("Create() with ID failed: %v", err)
	}
	if issue.ID != "hq-explicit-test-id" {
		t.Errorf("ID = %q, want %q", issue.ID, "hq-explicit-test-id")
	}
}

// === CreateWithPrefix Tests ===

func TestCreateWithPrefix_Basic(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-createprefix")
	bd := env.NewForTown()

	// Use the default prefix from gt install (hq)
	issue, err := bd.CreateWithPrefix("hq", CreateOptions{
		Title: "Prefixed Issue",
	})
	if err != nil {
		t.Fatalf("CreateWithPrefix() failed: %v", err)
	}
	if issue == nil {
		t.Fatal("Expected issue, got nil")
	}
	if !strings.HasPrefix(issue.ID, "hq-") {
		t.Errorf("ID = %q, should start with 'hq-'", issue.ID)
	}
}

// === Update Tests ===

func TestUpdate_Title(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-update-title")
	bd := env.NewForTown()

	// Create then update
	created := env.CreateTestBead(t, "Original Title")

	newTitle := "Updated Title"
	err := bd.Update(created.ID, UpdateOptions{
		Title: &newTitle,
	})
	if err != nil {
		t.Fatalf("Update() failed: %v", err)
	}

	// Verify update
	shown, err := bd.Show(created.ID)
	if err != nil {
		t.Fatalf("Show() failed: %v", err)
	}
	if shown.Title != "Updated Title" {
		t.Errorf("Title = %q, want %q", shown.Title, "Updated Title")
	}
}

func TestUpdate_AddLabels(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-update-labels")
	bd := env.NewForTown()

	created := env.CreateTestBead(t, "Label Test")

	addLabels := []string{"new:label"}
	err := bd.Update(created.ID, UpdateOptions{
		AddLabels: addLabels,
	})
	if err != nil {
		t.Fatalf("Update() with AddLabels failed: %v", err)
	}

	shown, err := bd.Show(created.ID)
	if err != nil {
		t.Fatalf("Show() failed: %v", err)
	}

	hasLabel := false
	for _, l := range shown.Labels {
		if l == "new:label" {
			hasLabel = true
			break
		}
	}
	if !hasLabel {
		t.Errorf("Missing new:label after update: %v", shown.Labels)
	}
}

// === Close Tests ===

func TestClose_Basic(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-close")
	bd := env.NewForTown()

	created := env.CreateTestBead(t, "Close Test")

	err := bd.Close(created.ID)
	if err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	shown, err := bd.Show(created.ID)
	if err != nil {
		t.Fatalf("Show() failed: %v", err)
	}
	if shown.Status != "closed" {
		t.Errorf("Status = %q, want %q", shown.Status, "closed")
	}
}

func TestCloseWithReason_Basic(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-close-reason")
	bd := env.NewForTown()

	created := env.CreateTestBead(t, "Close Reason Test")

	// CloseWithReason(reason, ids...) - reason first, then IDs
	err := bd.CloseWithReason("completed successfully", created.ID)
	if err != nil {
		t.Fatalf("CloseWithReason() failed: %v", err)
	}

	shown, err := bd.Show(created.ID)
	if err != nil {
		t.Fatalf("Show() failed: %v", err)
	}
	if shown.Status != "closed" {
		t.Errorf("Status = %q, want %q", shown.Status, "closed")
	}
}

// === Release Tests ===

func TestRelease_Basic(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-release")
	bd := env.NewForTown()

	created := env.CreateTestBead(t, "Release Test")

	err := bd.Release(created.ID)
	if err != nil {
		t.Fatalf("Release() failed: %v", err)
	}
}
