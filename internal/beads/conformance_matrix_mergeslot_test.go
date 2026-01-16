package beads_test

// CRITICAL INVARIANT: Before changing any Double implementation:
// 1. Verify the real bd CLI behavior by testing manually
// 2. Ensure TrueRawBdOps tests pass against real bd
// 3. Only then update Double to match real bd behavior
// See internal/beads/DOUBLE_TESTING.md for details.
//
// NOTE: bd merge-slot acquire has a known bug ("invalid field for update: holder")
// so TrueRawBdOps tests use stubs that return nil. These tests primarily verify
// Double and Implementation behavior.

import (
	"fmt"
	"testing"

	"github.com/steveyegge/gastown/internal/beads"
)

// TestMatrix_MergeSlotCreate tests creating a merge slot.
func TestMatrix_MergeSlotCreate(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "MergeSlotCreate",
		Operation: "MergeSlotCreate",
		Test: func(ops beads.BeadsOps) error {
			slotID, err := ops.MergeSlotCreate()
			if err != nil {
				return fmt.Errorf("MergeSlotCreate failed: %v", err)
			}
			if slotID == "" {
				return fmt.Errorf("MergeSlotCreate returned empty slot ID")
			}
			return nil
		},
	})
}

// TestMatrix_MergeSlotCheck tests checking merge slot status.
func TestMatrix_MergeSlotCheck(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "MergeSlotCheck",
		Operation: "MergeSlotCheck",
		Test: func(ops beads.BeadsOps) error {
			// First create a slot
			_, err := ops.MergeSlotCreate()
			if err != nil {
				return fmt.Errorf("MergeSlotCreate failed: %v", err)
			}

			// Then check it
			status, err := ops.MergeSlotCheck()
			if err != nil {
				return fmt.Errorf("MergeSlotCheck failed: %v", err)
			}
			if status == nil {
				return fmt.Errorf("MergeSlotCheck returned nil status")
			}
			if status.ID == "" {
				return fmt.Errorf("MergeSlotCheck returned empty slot ID")
			}
			if !status.Available {
				return fmt.Errorf("MergeSlotCheck returned Available=false for fresh slot")
			}
			return nil
		},
	})
}

// TestMatrix_MergeSlotCheck_NotFound tests checking when no slot exists.
func TestMatrix_MergeSlotCheck_NotFound(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "MergeSlotCheck_NotFound",
		Operation: "MergeSlotCheck",
		Test: func(ops beads.BeadsOps) error {
			// Check without creating - should return status with Error="not found"
			status, err := ops.MergeSlotCheck()
			if err != nil {
				return fmt.Errorf("MergeSlotCheck failed: %v", err)
			}
			// bd returns a status with error="not found" rather than a Go error
			if status == nil {
				return fmt.Errorf("MergeSlotCheck returned nil status, want status with Error field")
			}
			if status.Error != "not found" {
				return fmt.Errorf("MergeSlotCheck Error=%q, want 'not found'", status.Error)
			}
			return nil
		},
	})
}

// TestMatrix_MergeSlotAcquire tests acquiring the merge slot.
// NOTE: bd merge-slot acquire has a known bug ("invalid field for update: holder")
// This test only runs against Double. Implementation and RawBd skip due to bd bug.
func TestMatrix_MergeSlotAcquire(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "MergeSlotAcquire",
		Operation: "MergeSlotAcquire",
		DoubleOnly: true, // Skip Implementation and RawBd due to bd bug
		Test: func(ops beads.BeadsOps) error {
			// First create a slot
			_, err := ops.MergeSlotCreate()
			if err != nil {
				return fmt.Errorf("MergeSlotCreate failed: %v", err)
			}

			// Acquire the slot
			status, err := ops.MergeSlotAcquire("test-holder", false)
			if err != nil {
				return fmt.Errorf("MergeSlotAcquire failed: %v", err)
			}
			if status == nil {
				return fmt.Errorf("MergeSlotAcquire returned nil status")
			}
			// After acquire, slot should not be available
			if status.Available {
				return fmt.Errorf("MergeSlotAcquire returned Available=true, want false")
			}
			if status.Holder != "test-holder" {
				return fmt.Errorf("MergeSlotAcquire Holder=%q, want test-holder", status.Holder)
			}
			return nil
		},
	})
}

// TestMatrix_MergeSlotAcquire_AlreadyHeld tests acquiring when already held.
// NOTE: bd merge-slot acquire has a known bug ("invalid field for update: holder")
// This test only runs against Double. Implementation and RawBd skip due to bd bug.
func TestMatrix_MergeSlotAcquire_AlreadyHeld(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:       "MergeSlotAcquire_AlreadyHeld",
		Operation:  "MergeSlotAcquire",
		DoubleOnly: true, // Skip Implementation and RawBd due to bd bug
		Test: func(ops beads.BeadsOps) error {
			// First create and acquire a slot
			_, err := ops.MergeSlotCreate()
			if err != nil {
				return fmt.Errorf("MergeSlotCreate failed: %v", err)
			}

			_, err = ops.MergeSlotAcquire("first-holder", false)
			if err != nil {
				return fmt.Errorf("First MergeSlotAcquire failed: %v", err)
			}

			// Try to acquire again as a different holder
			status, err := ops.MergeSlotAcquire("second-holder", false)
			if err != nil {
				return fmt.Errorf("Second MergeSlotAcquire failed: %v", err)
			}
			// Should return the held status, not acquire
			if status == nil {
				return fmt.Errorf("Second MergeSlotAcquire returned nil status")
			}
			if status.Holder != "first-holder" {
				return fmt.Errorf("MergeSlotAcquire Holder=%q, want first-holder", status.Holder)
			}
			return nil
		},
	})
}

// TestMatrix_MergeSlotRelease tests releasing the merge slot.
// NOTE: bd merge-slot acquire has a known bug ("invalid field for update: holder")
// This test depends on Acquire, so only runs against Double.
func TestMatrix_MergeSlotRelease(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:       "MergeSlotRelease",
		Operation:  "MergeSlotRelease",
		DoubleOnly: true, // Skip Implementation and RawBd due to bd bug in Acquire
		Test: func(ops beads.BeadsOps) error {
			// Create and acquire a slot
			_, err := ops.MergeSlotCreate()
			if err != nil {
				return fmt.Errorf("MergeSlotCreate failed: %v", err)
			}

			_, err = ops.MergeSlotAcquire("test-holder", false)
			if err != nil {
				return fmt.Errorf("MergeSlotAcquire failed: %v", err)
			}

			// Release the slot
			err = ops.MergeSlotRelease("test-holder")
			if err != nil {
				return fmt.Errorf("MergeSlotRelease failed: %v", err)
			}

			// Verify slot is available again
			status, err := ops.MergeSlotCheck()
			if err != nil {
				return fmt.Errorf("MergeSlotCheck after release failed: %v", err)
			}
			if status == nil {
				return fmt.Errorf("MergeSlotCheck returned nil")
			}
			if !status.Available {
				return fmt.Errorf("After release, Available=%v, want true", status.Available)
			}
			return nil
		},
	})
}

// TestMatrix_MergeSlotEnsureExists tests idempotent slot creation.
func TestMatrix_MergeSlotEnsureExists(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "MergeSlotEnsureExists",
		Operation: "MergeSlotEnsureExists",
		Test: func(ops beads.BeadsOps) error {
			// First call should create the slot
			slotID1, err := ops.MergeSlotEnsureExists()
			if err != nil {
				return fmt.Errorf("First MergeSlotEnsureExists failed: %v", err)
			}
			if slotID1 == "" {
				return fmt.Errorf("First MergeSlotEnsureExists returned empty slot ID")
			}

			// Second call should be idempotent
			slotID2, err := ops.MergeSlotEnsureExists()
			if err != nil {
				return fmt.Errorf("Second MergeSlotEnsureExists failed: %v", err)
			}
			if slotID1 != slotID2 {
				return fmt.Errorf("MergeSlotEnsureExists not idempotent: %s != %s", slotID1, slotID2)
			}
			return nil
		},
	})
}
