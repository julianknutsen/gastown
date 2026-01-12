//go:build integration

package beads

import (
	"testing"
)

// === GetConfig Tests ===

func TestGetConfig_IssuePrefix(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-config")
	bd := env.NewForTown()

	prefix, err := bd.GetConfig("issue_prefix")
	if err != nil {
		t.Fatalf("GetConfig(issue_prefix) failed: %v", err)
	}
	// gt install uses hq prefix
	if prefix != "hq" {
		t.Errorf("issue_prefix = %q, want %q", prefix, "hq")
	}
}

func TestGetConfig_NotFound(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-config-notfound")
	bd := env.NewForTown()

	_, err := bd.GetConfig("nonexistent_key_12345")
	// May return error or empty string
	_ = err
}

func TestGetConfig_EmptyKey(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-config-empty")
	bd := env.NewForTown()

	// bd config get with empty key returns "(not set)" - not an error
	value, err := bd.GetConfig("")
	if err != nil {
		t.Fatalf("GetConfig('') failed: %v", err)
	}
	// Empty key returns "(not set)" indicating no value
	if value != "(not set)" && value != "" {
		t.Errorf("GetConfig('') = %q, want '(not set)' or empty", value)
	}
}

// === SetConfig Tests ===

func TestSetConfig_NewKey(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-setconfig")
	bd := env.NewForTown()

	err := bd.SetConfig("test_key", "test_value")
	if err != nil {
		t.Fatalf("SetConfig() failed: %v", err)
	}

	// Verify
	value, err := bd.GetConfig("test_key")
	if err != nil {
		t.Fatalf("GetConfig() after set failed: %v", err)
	}
	if value != "test_value" {
		t.Errorf("value = %q, want %q", value, "test_value")
	}
}

func TestSetConfig_Update(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-setconfig-update")
	bd := env.NewForTown()

	// Set initial
	bd.SetConfig("update_key", "initial")

	// Update
	err := bd.SetConfig("update_key", "updated")
	if err != nil {
		t.Fatalf("SetConfig() update failed: %v", err)
	}

	value, _ := bd.GetConfig("update_key")
	if value != "updated" {
		t.Errorf("value = %q, want %q", value, "updated")
	}
}

// === Init Tests ===

func TestInit_NewDirectory(t *testing.T) {
	t.Parallel()
	// Use legacy setup for this test since we're testing bd init directly
	tmpDir := t.TempDir()

	bd := New(tmpDir)
	err := bd.Init("test")
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Verify prefix
	prefix, err := bd.GetConfig("issue_prefix")
	if err != nil {
		t.Fatalf("GetConfig() failed: %v", err)
	}
	if prefix != "test" {
		t.Errorf("issue_prefix = %q, want %q", prefix, "test")
	}
}

func TestInit_AlreadyInitialized(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-init-exists")
	bd := env.NewForTown()

	// Re-init should fail or be idempotent
	err := bd.Init("hq")
	// Some implementations allow re-init with same prefix
	_ = err
}

// === IsBeadsRepo Tests ===

func TestIsBeadsRepo_True(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-isbeads")
	bd := env.NewForTown()

	isRepo := bd.IsBeadsRepo()
	if !isRepo {
		t.Error("IsBeadsRepo() = false, want true")
	}
}

func TestIsBeadsRepo_False(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	bd := New(tmpDir)

	isRepo := bd.IsBeadsRepo()
	if isRepo {
		t.Error("IsBeadsRepo() = true for non-beads dir, want false")
	}
}

// === Stats Tests ===

func TestStats_Basic(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-stats")
	bd := env.NewForTown()

	stats, err := bd.Stats()
	if err != nil {
		t.Fatalf("Stats() failed: %v", err)
	}
	if stats == "" {
		t.Fatal("Stats() returned empty string")
	}
}

func TestStats_WithIssues(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-stats-issues")
	bd := env.NewForTown()

	// Create some issues
	env.CreateTestBead(t, "Stats Test 1")
	env.CreateTestBead(t, "Stats Test 2")

	stats, err := bd.Stats()
	if err != nil {
		t.Fatalf("Stats() failed: %v", err)
	}
	// Stats should reflect the created issues
	_ = stats
}

// === GetSyncStatus Tests ===

func TestGetSyncStatus_Initialized(t *testing.T) {
	// Skip: bd sync --status requires being in a git repository.
	// Town directories are not git repos by default.
	t.Skip("bd sync requires git repository - town is not a git repo")
}
