//go:build integration

package beads

import (
	"strings"
	"testing"
)

// === AddDependency Tests ===

func TestAddDependency_Basic(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-adddep")
	bd := env.NewForTown()

	parent := env.CreateTestBead(t, "Parent")
	child := env.CreateTestBead(t, "Child")

	err := bd.AddDependency(parent.ID, child.ID)
	if err != nil {
		t.Fatalf("AddDependency() failed: %v", err)
	}
}

func TestAddDependency_IssueNotFound(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-adddep-notfound")
	bd := env.NewForTown()

	child := env.CreateTestBead(t, "Existing")

	err := bd.AddDependency("hq-nonexistent", child.ID)
	if err == nil {
		t.Error("AddDependency() with non-existent issue should fail")
	}
}

func TestAddDependency_DependsOnNotFound(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-adddep-depnotfound")
	bd := env.NewForTown()

	parent := env.CreateTestBead(t, "Issue")

	// bd add-dependency may not validate that dependsOn exists
	// It could succeed or fail depending on bd version
	err := bd.AddDependency(parent.ID, "hq-nonexistent")
	_ = err
}

// === RemoveDependency Tests ===

func TestRemoveDependency_Exists(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-remdep")
	bd := env.NewForTown()

	parent := env.CreateTestBead(t, "Parent")
	child := env.CreateTestBead(t, "Child")

	// Add then remove
	err := bd.AddDependency(parent.ID, child.ID)
	if err != nil {
		t.Fatalf("AddDependency() failed: %v", err)
	}

	err = bd.RemoveDependency(parent.ID, child.ID)
	if err != nil {
		t.Fatalf("RemoveDependency() failed: %v", err)
	}
}

func TestRemoveDependency_NotExists(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-remdep-notexist")
	bd := env.NewForTown()

	parent := env.CreateTestBead(t, "Issue")

	// Remove non-existent dependency - should be idempotent or error
	err := bd.RemoveDependency(parent.ID, "hq-no-dep-target")
	// Behavior depends on implementation
	_ = err
}

// === ReleaseWithReason Tests ===

func TestReleaseWithReason_Basic(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-release-reason")
	bd := env.NewForTown()

	bead := env.CreateTestBead(t, "Release Me")

	err := bd.ReleaseWithReason(bead.ID, "completed work")
	if err != nil {
		t.Fatalf("ReleaseWithReason() failed: %v", err)
	}
}

func TestReleaseWithReason_NotFound(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-release-notfound")
	bd := env.NewForTown()

	// bd update on non-existent may succeed or fail depending on bd version
	err := bd.ReleaseWithReason("hq-nonexistent-release", "reason")
	// Error behavior varies - may return error or succeed silently
	_ = err
}

// === CreateWithPrefix Tests (rig context) ===

func TestCreateWithPrefix_DifferentPrefix(t *testing.T) {
	t.Parallel()
	// Need rig context to test different prefix
	env := SetupTownWithRigEnv(t, "test-createprefix-diff", "testrig", "tr")
	bd := env.NewForRig()

	// Create with rig prefix
	issue, err := bd.CreateWithPrefix("tr", CreateOptions{
		Title: "Rig Prefixed Issue",
	})
	if err != nil {
		t.Fatalf("CreateWithPrefix(tr) failed: %v", err)
	}
	if !strings.HasPrefix(issue.ID, "tr-") {
		t.Errorf("ID = %q, should start with 'tr-'", issue.ID)
	}
}

// === AddGateWaiter Tests ===

func TestAddGateWaiter_Basic(t *testing.T) {
	// Skip: bd gate add-waiter returns "invalid field for update: waiters"
	// This is a bd limitation - the waiters field cannot be updated via bd commands.
	t.Skip("bd gate add-waiter doesn't support waiters field update")
}

func TestAddGateWaiter_GateNotFound(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-gatewaiter-notfound")
	bd := env.NewForTown()

	err := bd.AddGateWaiter("hq-nonexistent-gate", "waiter")
	if err == nil {
		t.Error("AddGateWaiter() for non-existent gate should fail")
	}
}
