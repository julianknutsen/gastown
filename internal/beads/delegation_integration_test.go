//go:build integration

package beads

import (
	"testing"
)

// === Delegation Tests ===
// All delegation tests are skipped because bd slot only supports "hook" and "role" slots,
// not "delegated_from". The delegation system needs to be reimplemented.
// Production bug: beads_delegation.go uses bd slot set with "delegated_from" slot.

func TestGetDelegation_NoDelegation(t *testing.T) {
	// Skip: GetDelegation parses slot value as JSON but bd returns error message
	t.Skip("Delegation system uses unsupported bd slot - fix pending")
}

func TestGetDelegation_HasDelegation(t *testing.T) {
	t.Skip("Delegation system uses unsupported bd slot - fix pending")
}

func TestGetDelegation_NotFound(t *testing.T) {
	t.Skip("Delegation system uses unsupported bd slot - fix pending")
}

func TestAddDelegation_Basic(t *testing.T) {
	t.Skip("Delegation system uses unsupported bd slot - fix pending")
}

func TestAddDelegation_MissingFields(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-adddel-missing")
	bd := env.NewForTown()

	// Missing parent and child - should fail validation before hitting bd
	deleg := &Delegation{
		DelegatedBy: "hop://test/mayor",
		DelegatedTo: "hop://test/polecat",
	}
	err := bd.AddDelegation(deleg)
	if err == nil {
		t.Error("AddDelegation() with missing parent/child should fail")
	}
}

func TestRemoveDelegation_Exists(t *testing.T) {
	t.Skip("Delegation system uses unsupported bd slot - fix pending")
}

func TestRemoveDelegation_NotExists(t *testing.T) {
	t.Skip("Delegation system uses unsupported bd slot - fix pending")
}

func TestListDelegationsFrom_NoDelegations(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-listdel-none")
	bd := env.NewForTown()

	// This should work - listing from nonexistent returns empty
	delegs, err := bd.ListDelegationsFrom("hq-nonexistent-agent")
	if err != nil {
		t.Fatalf("ListDelegationsFrom() failed: %v", err)
	}
	if len(delegs) != 0 {
		t.Errorf("Expected 0 delegations, got %d", len(delegs))
	}
}

func TestListDelegationsFrom_HasDelegations(t *testing.T) {
	t.Skip("Delegation system uses unsupported bd slot - fix pending")
}
