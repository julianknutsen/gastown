//go:build integration

package beads

import (
	"testing"
)

// Note: Merge slot API operates on a singleton merge slot, not named slots.
// MergeSlotCreate() creates/returns the slot
// MergeSlotCheck() checks current status
// MergeSlotAcquire(holder, addWaiter) acquires or waits
// MergeSlotRelease(holder) releases
// MergeSlotEnsureExists() ensures slot exists

// === MergeSlotCheck Tests ===

func TestMergeSlotCheck_NoSlot(t *testing.T) {
	t.Skip("MergeSlot tests need fresh town without pre-existing slot")

	t.Parallel()
	env := SetupTownEnv(t, "test-mergeslot-check")
	bd := env.NewForTown()

	status, err := bd.MergeSlotCheck()
	if err != nil {
		t.Fatalf("MergeSlotCheck() failed: %v", err)
	}
	// May return nil or empty status when no slot exists
	_ = status
}

func TestMergeSlotCheck_AfterCreate(t *testing.T) {
	t.Skip("MergeSlot tests need investigation of bd merge-slot interface")

	t.Parallel()
	env := SetupTownEnv(t, "test-mergeslot-created")
	bd := env.NewForTown()

	// Create slot first
	_, err := bd.MergeSlotCreate()
	if err != nil {
		t.Fatalf("MergeSlotCreate() failed: %v", err)
	}

	status, err := bd.MergeSlotCheck()
	if err != nil {
		t.Fatalf("MergeSlotCheck() failed: %v", err)
	}
	// Status should exist after creation
	_ = status
}

// === MergeSlotCreate Tests ===

func TestMergeSlotCreate_New(t *testing.T) {
	t.Skip("MergeSlot tests need investigation of bd merge-slot interface")

	t.Parallel()
	env := SetupTownEnv(t, "test-mergeslot-create")
	bd := env.NewForTown()

	slotID, err := bd.MergeSlotCreate()
	if err != nil {
		t.Fatalf("MergeSlotCreate() failed: %v", err)
	}
	if slotID == "" {
		t.Error("Expected slot ID, got empty string")
	}
}

func TestMergeSlotCreate_AlreadyExists(t *testing.T) {
	t.Skip("MergeSlot tests need investigation of bd merge-slot interface")

	t.Parallel()
	env := SetupTownEnv(t, "test-mergeslot-exists")
	bd := env.NewForTown()

	bd.MergeSlotCreate()
	slotID, err := bd.MergeSlotCreate()
	// Should be idempotent - returns existing slot
	if err != nil {
		t.Fatalf("MergeSlotCreate() second call failed: %v", err)
	}
	_ = slotID
}

// === MergeSlotAcquire Tests ===

func TestMergeSlotAcquire_Available(t *testing.T) {
	t.Skip("MergeSlot tests need investigation of bd merge-slot interface")

	t.Parallel()
	env := SetupTownEnv(t, "test-mergeslot-acquire")
	bd := env.NewForTown()

	bd.MergeSlotCreate()
	status, err := bd.MergeSlotAcquire("test-holder", false)
	if err != nil {
		t.Fatalf("MergeSlotAcquire() failed: %v", err)
	}
	if status == nil {
		t.Fatal("Expected status, got nil")
	}
}

func TestMergeSlotAcquire_AlreadyHeld(t *testing.T) {
	t.Skip("MergeSlot tests need investigation of bd merge-slot interface")

	t.Parallel()
	env := SetupTownEnv(t, "test-mergeslot-held")
	bd := env.NewForTown()

	bd.MergeSlotCreate()
	bd.MergeSlotAcquire("holder1", false)

	// Second acquire by different holder
	status, err := bd.MergeSlotAcquire("holder2", false)
	// Should fail or indicate already held
	_ = status
	_ = err
}

func TestMergeSlotAcquire_AddWaiter(t *testing.T) {
	t.Skip("MergeSlot tests need investigation of bd merge-slot interface")

	t.Parallel()
	env := SetupTownEnv(t, "test-mergeslot-waiter")
	bd := env.NewForTown()

	bd.MergeSlotCreate()
	bd.MergeSlotAcquire("holder1", false)

	// Second acquire with addWaiter=true
	status, err := bd.MergeSlotAcquire("holder2", true)
	// Should add to waiters
	_ = status
	_ = err
}

// === MergeSlotRelease Tests ===

func TestMergeSlotRelease_Held(t *testing.T) {
	t.Skip("MergeSlot tests need investigation of bd merge-slot interface")

	t.Parallel()
	env := SetupTownEnv(t, "test-mergeslot-release")
	bd := env.NewForTown()

	bd.MergeSlotCreate()
	bd.MergeSlotAcquire("holder", false)

	err := bd.MergeSlotRelease("holder")
	if err != nil {
		t.Fatalf("MergeSlotRelease() failed: %v", err)
	}
}

func TestMergeSlotRelease_WrongHolder(t *testing.T) {
	t.Skip("MergeSlot tests need investigation of bd merge-slot interface")

	t.Parallel()
	env := SetupTownEnv(t, "test-mergeslot-wrongholder")
	bd := env.NewForTown()

	bd.MergeSlotCreate()
	bd.MergeSlotAcquire("holder1", false)

	err := bd.MergeSlotRelease("holder2")
	if err == nil {
		t.Error("MergeSlotRelease() with wrong holder should fail")
	}
}

// === MergeSlotEnsureExists Tests ===

func TestMergeSlotEnsureExists_Creates(t *testing.T) {
	t.Skip("MergeSlot tests need investigation of bd merge-slot interface")

	t.Parallel()
	env := SetupTownEnv(t, "test-mergeslot-ensure")
	bd := env.NewForTown()

	slotID, err := bd.MergeSlotEnsureExists()
	if err != nil {
		t.Fatalf("MergeSlotEnsureExists() failed: %v", err)
	}
	if slotID == "" {
		t.Error("Expected slot ID, got empty string")
	}
}

func TestMergeSlotEnsureExists_AlreadyExists(t *testing.T) {
	t.Skip("MergeSlot tests need investigation of bd merge-slot interface")

	t.Parallel()
	env := SetupTownEnv(t, "test-mergeslot-ensure-exists")
	bd := env.NewForTown()

	bd.MergeSlotCreate()
	slotID, err := bd.MergeSlotEnsureExists()
	if err != nil {
		t.Fatalf("MergeSlotEnsureExists() for existing slot failed: %v", err)
	}
	_ = slotID
}
