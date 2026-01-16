package beads_test

// CRITICAL INVARIANT: Before changing any Double implementation:
// 1. Verify the real bd CLI behavior by testing manually
// 2. Ensure TrueRawBdOps tests pass against real bd
// 3. Only then update Double to match real bd behavior
// See internal/beads/DOUBLE_TESTING.md for details.

import (
	"fmt"
	"testing"

	"github.com/steveyegge/gastown/internal/beads"
)

// TestBdRoutingBugs_MapCompleteness verifies that all ID-based operations
// in the BeadsOps interface are tracked in the BdRoutingBugs map.
func TestBdRoutingBugs_MapCompleteness(t *testing.T) {
	// List of all ID-based operations that need routing
	idBasedOps := []string{
		"Show",
		"Update",
		"Close",
		"Delete",
		"DeleteWithOptions",
		"Reopen",
		"Release",
		"ReleaseWithReason",
		"CloseWithOptions",
		"LabelAdd",
		"LabelRemove",
		"AgentState",
		"Comment",
		"UpdateAgentActiveMR",
		"AddDependency",
		"AddDependencyWithType",
		"RemoveDependency",
	}

	for _, op := range idBasedOps {
		_, ok := beads.BdRoutingBugs[op]
		if !ok {
			t.Errorf("BdRoutingBugs missing entry for ID-based operation: %s", op)
		}
	}
}

// TestMatrix_Routing_Show tests Show routing across all contexts.
// This is the fundamental routing test - if Show works cross-rig, other reads should too.
func TestMatrix_Routing_Show(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "Routing_Show",
		Operation: "Show",
		Test: func(ops beads.BeadsOps, targetID string) error {
			issue, err := ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show(%s) failed: %v", targetID, err)
			}
			if issue.ID != targetID {
				return fmt.Errorf("Show returned wrong ID: got %s, want %s", issue.ID, targetID)
			}
			return nil
		},
	})
}

// TestMatrix_Routing_Update tests Update routing across all contexts.
func TestMatrix_Routing_Update(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "Routing_Update",
		Operation: "Update",
		Test: func(ops beads.BeadsOps, targetID string) error {
			newTitle := "Routing Update Test"
			err := ops.Update(targetID, beads.UpdateOptions{Title: &newTitle})
			if err != nil {
				return fmt.Errorf("Update(%s) failed: %v", targetID, err)
			}
			// Verify update was applied
			issue, err := ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show after Update failed: %v", err)
			}
			if issue.Title != newTitle {
				return fmt.Errorf("Title = %q, want %q", issue.Title, newTitle)
			}
			return nil
		},
	})
}

// TestMatrix_Routing_Close tests Close routing across all contexts.
func TestMatrix_Routing_Close(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "Routing_Close",
		Operation: "Close",
		Test: func(ops beads.BeadsOps, targetID string) error {
			err := ops.Close(targetID)
			if err != nil {
				return fmt.Errorf("Close(%s) failed: %v", targetID, err)
			}
			// Verify close was applied
			issue, err := ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show after Close failed: %v", err)
			}
			if issue.Status != "closed" {
				return fmt.Errorf("Status = %q, want 'closed'", issue.Status)
			}
			return nil
		},
	})
}

// TestMatrix_Routing_LabelAdd tests LabelAdd routing across all contexts.
func TestMatrix_Routing_LabelAdd(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "Routing_LabelAdd",
		Operation: "LabelAdd",
		Test: func(ops beads.BeadsOps, targetID string) error {
			label := "routing:test"
			err := ops.LabelAdd(targetID, label)
			if err != nil {
				return fmt.Errorf("LabelAdd(%s) failed: %v", targetID, err)
			}
			// Verify label was added
			issue, err := ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show after LabelAdd failed: %v", err)
			}
			found := false
			for _, l := range issue.Labels {
				if l == label {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("Label %q not found in %v", label, issue.Labels)
			}
			return nil
		},
	})
}

// TestMatrix_Routing_Delete tests Delete routing across all contexts.
func TestMatrix_Routing_Delete(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "Routing_Delete",
		Operation: "Delete",
		Test: func(ops beads.BeadsOps, targetID string) error {
			err := ops.Delete(targetID)
			if err != nil {
				return fmt.Errorf("Delete(%s) failed: %v", targetID, err)
			}
			// Verify delete - Show should return ErrNotFound
			_, err = ops.Show(targetID)
			if err != beads.ErrNotFound {
				return fmt.Errorf("Show after Delete: got err=%v, want ErrNotFound", err)
			}
			return nil
		},
	})
}

// TestMatrix_Routing_Reopen tests Reopen routing across all contexts.
func TestMatrix_Routing_Reopen(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "Routing_Reopen",
		Operation: "Reopen",
		Test: func(ops beads.BeadsOps, targetID string) error {
			// First close the issue
			err := ops.Close(targetID)
			if err != nil {
				return fmt.Errorf("Close failed: %v", err)
			}

			// Now reopen it
			err = ops.Reopen(targetID)
			if err != nil {
				return fmt.Errorf("Reopen(%s) failed: %v", targetID, err)
			}

			// Verify reopened
			issue, err := ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show after Reopen failed: %v", err)
			}
			if issue.Status != "open" {
				return fmt.Errorf("Status = %q, want 'open'", issue.Status)
			}
			return nil
		},
	})
}

// TestMatrix_Routing_AddDependency tests AddDependency routing.
// Note: This test is simplified to not create cross-database dependencies.
// Cross-database dependencies (e.g., ap bead depending on gt bead) are a separate
// feature from cross-rig routing and have their own complexity.
func TestMatrix_Routing_AddDependency(t *testing.T) {
	// Use SimpleConformanceTest since the dependency target needs to be in the same db
	// as the source bead to avoid cross-database dependency complexity
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Routing_AddDependency",
		Operation: "AddDependency",
		Test: func(ops beads.BeadsOps) error {
			// Create both source and target in the same database
			source, err := ops.Create(beads.CreateOptions{
				Title: "Dependency Source",
				Type:  "task",
			})
			if err != nil {
				return fmt.Errorf("Create source failed: %v", err)
			}
			depTarget, err := ops.Create(beads.CreateOptions{
				Title: "Dependency Target",
				Type:  "task",
			})
			if err != nil {
				return fmt.Errorf("Create dependency target failed: %v", err)
			}

			// Add dependency
			err = ops.AddDependency(source.ID, depTarget.ID)
			if err != nil {
				return fmt.Errorf("AddDependency(%s, %s) failed: %v", source.ID, depTarget.ID, err)
			}

			// Verify dependency was added
			issue, err := ops.Show(source.ID)
			if err != nil {
				return fmt.Errorf("Show after AddDependency failed: %v", err)
			}
			found := false
			for _, dep := range issue.Dependencies {
				if dep.ID == depTarget.ID {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("Dependency %s not found in dependencies", depTarget.ID)
			}
			return nil
		},
	})
}

// TestMatrix_Routing_CrossRigDependency tests adding a dependency between beads
// in different databases (e.g., ap bead depends on gt bead).
// This is a complex scenario that requires both beads to be resolvable from the
// current working context.
func TestMatrix_Routing_CrossRigDependency(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "Routing_CrossRigDependency",
		Operation: "AddDependency",
		Test: func(ops beads.BeadsOps, targetID string) error {
			// targetID is in ap database (from cross-rig setup)
			// We need to create a dependency target in gt database

			// For Double, we need to create the target in gt
			if d, ok := ops.(*beads.Double); ok {
				d.SetCurrentPrefix("gt")
				depTarget, err := d.Create(beads.CreateOptions{
					Title: "CrossRig Dep Target",
					Type:  "task",
				})
				if err != nil {
					return fmt.Errorf("Create gt target failed: %v", err)
				}
				d.SetCurrentPrefix("gt") // Ensure we're in gt context

				// Now add dependency from ap bead to gt bead
				err = d.AddDependency(targetID, depTarget.ID)
				if err != nil {
					return fmt.Errorf("AddDependency(%s, %s) failed: %w", targetID, depTarget.ID, err)
				}

				// Verify dependency
				issue, err := d.Show(targetID)
				if err != nil {
					return fmt.Errorf("Show after AddDependency failed: %v", err)
				}
				found := false
				for _, dep := range issue.Dependencies {
					if dep.ID == depTarget.ID {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("Cross-rig dependency %s not found in dependencies", depTarget.ID)
				}
				return nil
			}

			// For Implementation/RawBd, this is more complex as we need different contexts
			// to create beads in different databases. For now, just verify the operation
			// can be attempted (may fail due to routing issues).

			// Create dependency target in the current context (which is gt for Implementation)
			depTarget, err := ops.Create(beads.CreateOptions{
				Title: "CrossRig Dep Target",
				Type:  "task",
			})
			if err != nil {
				return fmt.Errorf("Create target failed: %v", err)
			}

			// Try to add cross-rig dependency
			err = ops.AddDependency(targetID, depTarget.ID)
			if err != nil {
				return fmt.Errorf("AddDependency(%s, %s) failed: %w", targetID, depTarget.ID, err)
			}

			// Verify dependency
			issue, err := ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show after AddDependency failed: %v", err)
			}
			found := false
			for _, dep := range issue.Dependencies {
				if dep.ID == depTarget.ID {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("Cross-rig dependency %s not found in dependencies", depTarget.ID)
			}
			return nil
		},
	})
}

// TestBrokenOperations verifies the list of broken operations is accurate.
func TestBrokenOperations(t *testing.T) {
	broken := beads.BrokenOperations()
	if len(broken) == 0 {
		t.Skip("No broken operations reported")
	}

	t.Logf("Current broken operations in bd: %v", broken)

	// Verify each broken operation is actually marked as broken
	for _, op := range broken {
		if beads.IsBdFixed(op) {
			t.Errorf("BrokenOperations() includes %s but IsBdFixed(%s) returns true", op, op)
		}
	}
}
