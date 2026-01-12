//go:build integration

package beads

import (
	"testing"
)

// === GetAgentNotificationLevel Tests ===

func TestGetAgentNotificationLevel_Default(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-getnotif-default")
	bd := env.NewForTown()

	// Use agent created by gt install
	level, err := bd.GetAgentNotificationLevel("hq-mayor")
	if err != nil {
		t.Fatalf("GetAgentNotificationLevel() failed: %v", err)
	}
	// Default is typically "normal" or empty
	_ = level
}

func TestGetAgentNotificationLevel_Set(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-getnotif-set")
	bd := env.NewForTown()

	// Use agent created by gt install
	// Valid levels: verbose, normal, muted
	err := bd.UpdateAgentNotificationLevel("hq-deacon", "verbose")
	if err != nil {
		t.Fatalf("UpdateAgentNotificationLevel() failed: %v", err)
	}

	// Get it back
	level, err := bd.GetAgentNotificationLevel("hq-deacon")
	if err != nil {
		t.Fatalf("GetAgentNotificationLevel() failed: %v", err)
	}
	if level != "verbose" {
		t.Errorf("level = %q, want %q", level, "verbose")
	}
}

func TestGetAgentNotificationLevel_NotFound(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-getnotif-notfound")
	bd := env.NewForTown()

	_, err := bd.GetAgentNotificationLevel("hq-nonexistent-agent")
	if err == nil {
		t.Error("GetAgentNotificationLevel() for non-existent should fail")
	}
}

func TestGetAgentNotificationLevel_NotAgent(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-getnotif-notagent")
	bd := env.NewForTown()

	bead := env.CreateTestBead(t, "Not Agent")

	_, err := bd.GetAgentNotificationLevel(bead.ID)
	if err == nil {
		t.Error("GetAgentNotificationLevel() for non-agent should fail")
	}
}

func TestGetAgentNotificationLevel_EmptyID(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-getnotif-empty")
	bd := env.NewForTown()

	_, err := bd.GetAgentNotificationLevel("")
	if err == nil {
		t.Error("GetAgentNotificationLevel('') should fail")
	}
}

// === UpdateAgentNotificationLevel Tests ===

func TestUpdateAgentNotificationLevel_Basic(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-updatenotif")
	bd := env.NewForTown()

	// Use agent created by gt install
	// Valid levels: verbose, normal, muted
	err := bd.UpdateAgentNotificationLevel("hq-mayor", "muted")
	if err != nil {
		t.Fatalf("UpdateAgentNotificationLevel() failed: %v", err)
	}

	// Verify
	level, _ := bd.GetAgentNotificationLevel("hq-mayor")
	if level != "muted" {
		t.Errorf("level = %q, want %q", level, "muted")
	}
}

func TestUpdateAgentNotificationLevel_NotFound(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-updatenotif-notfound")
	bd := env.NewForTown()

	err := bd.UpdateAgentNotificationLevel("hq-nonexistent", "verbose")
	if err == nil {
		t.Error("UpdateAgentNotificationLevel() for non-existent should fail")
	}
}

func TestUpdateAgentNotificationLevel_Levels(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-updatenotif-levels")
	bd := env.NewForTown()

	// Use agent created by gt install
	// Test all valid levels: verbose, normal, muted
	levels := []string{"verbose", "normal", "muted"}
	for _, level := range levels {
		err := bd.UpdateAgentNotificationLevel("hq-deacon", level)
		if err != nil {
			t.Errorf("UpdateAgentNotificationLevel(%q) failed: %v", level, err)
		}
	}
}
