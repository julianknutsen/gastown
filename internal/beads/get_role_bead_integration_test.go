//go:build integration

package beads

import (
	"testing"
)

// === GetRoleConfig Tests ===
// GetRoleConfig(roleBeadID) returns (*RoleConfig, error)
// RoleConfig contains: SessionPattern, WorkDirPattern, NeedsPreSync, StartCommand, etc.

func TestGetRoleBead_Exists(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-getrolebead")
	bd := env.NewForTown()

	// Create role bead using API
	_, err := bd.Create(CreateOptions{
		ID:     "hq-role-test",
		Title:  "Test Role",
		Labels: []string{"gt:role"},
	})
	if err != nil {
		t.Fatalf("Create role bead failed: %v", err)
	}

	roleConfig, err := bd.GetRoleConfig("hq-role-test")
	if err != nil {
		t.Fatalf("GetRoleConfig() failed: %v", err)
	}
	// RoleConfig may be nil or have default values for basic role
	_ = roleConfig
}

func TestGetRoleBead_NotFound(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-getrolebead-notfound")
	bd := env.NewForTown()

	// GetRoleConfig returns nil, nil for not found (backwards compat)
	roleConfig, err := bd.GetRoleConfig("hq-nonexistent-role")
	if err != nil {
		t.Fatalf("GetRoleConfig() should return nil, nil for not found, got error: %v", err)
	}
	if roleConfig != nil {
		t.Error("Expected nil roleConfig for non-existent bead")
	}
}

func TestGetRoleBead_NotRole(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-getrolebead-notrole")
	bd := env.NewForTown()

	// Create non-role bead (missing gt:role label)
	created := env.CreateTestBead(t, "Not Role")

	// GetRoleConfig checks for gt:role label and returns error if missing
	_, err := bd.GetRoleConfig(created.ID)
	if err == nil {
		t.Error("GetRoleConfig() should fail for bead without gt:role label")
	}
}

func TestGetRoleBead_EmptyID(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-getrolebead-empty")
	bd := env.NewForTown()

	// GetRoleConfig with empty ID returns nil, nil (same as not found)
	config, err := bd.GetRoleConfig("")
	if err != nil {
		t.Fatalf("GetRoleConfig('') returned error: %v", err)
	}
	if config != nil {
		t.Error("Expected nil config for empty ID")
	}
}

func TestGetRoleBead_ParsesConfig(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-getrolebead-config")
	bd := env.NewForTown()

	// Create role with description containing config
	// RoleConfig parses: session_pattern, work_dir_pattern, needs_pre_sync, start_command
	desc := `session_pattern: hq-{role}
work_dir_pattern: {town}/{rig}
needs_pre_sync: true`
	_, err := bd.Create(CreateOptions{
		ID:          "hq-role-config",
		Title:       "Role with Config",
		Labels:      []string{"gt:role"},
		Description: desc,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	roleConfig, err := bd.GetRoleConfig("hq-role-config")
	if err != nil {
		t.Fatalf("GetRoleConfig() failed: %v", err)
	}
	if roleConfig == nil {
		t.Fatal("Expected RoleConfig to be parsed")
	}
	if roleConfig.SessionPattern != "hq-{role}" {
		t.Errorf("SessionPattern = %q, want %q", roleConfig.SessionPattern, "hq-{role}")
	}
}
