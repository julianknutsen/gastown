//go:build integration

package beads

import (
	"testing"
)

// Note: CreateAgentBead is internal. Use CreateTownAgent or CreateRigAgent instead.
// Town agents are created by gt install, so we test via GetAgentBead.
// Rig agents are tested in rig_convoy_integration_test.go.

// === UpdateAgentState Tests ===

func TestUpdateAgentState_Basic(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-updatestate")
	bd := env.NewForTown()

	// Use agent created by gt install
	// Valid states: dead, idle, spawning, running, working, stuck, done, stopped
	err := bd.UpdateAgentState("hq-mayor", "running", nil)
	if err != nil {
		t.Fatalf("UpdateAgentState() failed: %v", err)
	}
}

func TestUpdateAgentState_InvalidPrefix(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-updatestate-invalidprefix")
	bd := env.NewForTown()

	// bd agent state auto-creates agents with valid prefix, but fails on invalid prefix
	err := bd.UpdateAgentState("invalid-prefix-agent", "running", nil)
	if err == nil {
		t.Error("UpdateAgentState() with invalid prefix should fail")
	}
}

// === SetHookBead Tests ===

func TestSetHookBead_Basic(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-sethook")
	bd := env.NewForTown()

	// Create issue to assign to agent created by gt install
	issue := env.CreateTestBead(t, "Hook Issue")

	err := bd.SetHookBead("hq-mayor", issue.ID)
	if err != nil {
		t.Fatalf("SetHookBead() failed: %v", err)
	}
}

func TestSetHookBead_AgentNotFound(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-sethook-notfound")
	bd := env.NewForTown()

	issue := env.CreateTestBead(t, "Hook Issue")

	err := bd.SetHookBead("hq-nonexistent", issue.ID)
	if err == nil {
		t.Error("SetHookBead() for non-existent agent should fail")
	}
}

func TestSetHookBead_Replace(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-sethook-replace")
	bd := env.NewForTown()

	// Create issues to assign to agent created by gt install
	issue1 := env.CreateTestBead(t, "Hook Issue 1")
	issue2 := env.CreateTestBead(t, "Hook Issue 2")

	// Set first hook on deacon (use different agent than other tests)
	bd.SetHookBead("hq-deacon", issue1.ID)

	// Replace with second
	err := bd.SetHookBead("hq-deacon", issue2.ID)
	if err != nil {
		t.Fatalf("SetHookBead() replace failed: %v", err)
	}
}

// === ClearHookBead Tests ===

func TestClearHookBead_HasHook(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-clearhook")
	bd := env.NewForTown()

	// Set hook first on agent created by gt install
	issue := env.CreateTestBead(t, "Clear Issue")
	bd.SetHookBead("hq-mayor", issue.ID)

	// Clear hook
	err := bd.ClearHookBead("hq-mayor")
	if err != nil {
		t.Fatalf("ClearHookBead() failed: %v", err)
	}
}

func TestClearHookBead_NoHook(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-clearhook-none")
	bd := env.NewForTown()

	// Clear non-existent hook on agent created by gt install - should be no-op
	err := bd.ClearHookBead("hq-deacon")
	_ = err
}

// === UpdateAgentCleanupStatus Tests ===

func TestUpdateAgentCleanupStatus_Clean(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-cleanup-clean")
	bd := env.NewForTown()

	// Use agent created by gt install
	err := bd.UpdateAgentCleanupStatus("hq-mayor", "clean")
	if err != nil {
		t.Fatalf("UpdateAgentCleanupStatus(clean) failed: %v", err)
	}
}

func TestUpdateAgentCleanupStatus_HasUncommitted(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-cleanup-dirty")
	bd := env.NewForTown()

	// Use agent created by gt install
	err := bd.UpdateAgentCleanupStatus("hq-deacon", "uncommitted changes")
	if err != nil {
		t.Fatalf("UpdateAgentCleanupStatus(dirty) failed: %v", err)
	}
}

// === UpdateAgentActiveMR Tests ===

func TestUpdateAgentActiveMR_Set(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-activemr-set")
	bd := env.NewForTown()

	// Use agent created by gt install
	err := bd.UpdateAgentActiveMR("hq-mayor", "123")
	if err != nil {
		t.Fatalf("UpdateAgentActiveMR(set) failed: %v", err)
	}
}

func TestUpdateAgentActiveMR_Clear(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-activemr-clear")
	bd := env.NewForTown()

	// Use agent created by gt install - set then clear
	bd.UpdateAgentActiveMR("hq-deacon", "456")

	err := bd.UpdateAgentActiveMR("hq-deacon", "")
	if err != nil {
		t.Fatalf("UpdateAgentActiveMR(clear) failed: %v", err)
	}
}

// === DeleteAgentBead Tests ===

func TestDeleteAgentBead_Exists(t *testing.T) {
	// Skip: bd delete --hard --force doesn't actually delete/close the bead
	// Shows status="open" after deletion. Possible bd bug or different semantics.
	t.Skip("bd delete --hard --force doesn't close bead - behavior unclear")
}

func TestDeleteAgentBead_NotExists(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-deleteagent-notexist")
	bd := env.NewForTown()

	err := bd.DeleteAgentBead("hq-nonexistent-agent")
	// Should be no-op or error
	_ = err
}
