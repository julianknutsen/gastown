package beads_test

import (
	"fmt"
	"testing"

	"github.com/steveyegge/gastown/internal/beads"
)

func TestMatrix_Update(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "Update",
		Operation: "Update",
		Test: func(ops beads.BeadsOps, targetID string) error {
			newTitle := "Updated Title"
			err := ops.Update(targetID, beads.UpdateOptions{Title: &newTitle})
			if err != nil {
				return fmt.Errorf("Update(%s) failed: %v", targetID, err)
			}

			// Verify update
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

func TestMatrix_Close(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "Close",
		Operation: "Close",
		Test: func(ops beads.BeadsOps, targetID string) error {
			err := ops.Close(targetID)
			if err != nil {
				return fmt.Errorf("Close(%s) failed: %v", targetID, err)
			}

			// Verify closed
			issue, err := ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show after Close failed: %v", err)
			}
			if issue.Status != "closed" {
				return fmt.Errorf("Status = %q, want %q", issue.Status, "closed")
			}
			return nil
		},
	})
}

func TestMatrix_CloseWithReason(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "CloseWithReason",
		Operation: "CloseWithReason",
		Test: func(ops beads.BeadsOps, targetID string) error {
			err := ops.CloseWithReason("test-reason", targetID)
			if err != nil {
				return fmt.Errorf("CloseWithReason(%s) failed: %v", targetID, err)
			}

			// Verify closed
			issue, err := ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show after CloseWithReason failed: %v", err)
			}
			if issue.Status != "closed" {
				return fmt.Errorf("Status = %q, want %q", issue.Status, "closed")
			}
			return nil
		},
	})
}

func TestMatrix_Delete(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "Delete",
		Operation: "Delete",
		Test: func(ops beads.BeadsOps, targetID string) error {
			err := ops.Delete(targetID)
			if err != nil {
				return fmt.Errorf("Delete(%s) failed: %v", targetID, err)
			}

			// Verify deleted - Show should return ErrNotFound
			_, err = ops.Show(targetID)
			if err != beads.ErrNotFound {
				return fmt.Errorf("Show after Delete returned err=%v, want ErrNotFound", err)
			}
			return nil
		},
	})
}

func TestMatrix_Reopen(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "Reopen",
		Operation: "Reopen",
		Test: func(ops beads.BeadsOps, targetID string) error {
			// First close the target
			err := ops.Close(targetID)
			if err != nil {
				return fmt.Errorf("Close(%s) failed: %v", targetID, err)
			}

			// Verify closed
			issue, err := ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show after Close failed: %v", err)
			}
			if issue.Status != "closed" {
				return fmt.Errorf("Status after Close = %q, want %q", issue.Status, "closed")
			}

			// Reopen
			err = ops.Reopen(targetID)
			if err != nil {
				return fmt.Errorf("Reopen(%s) failed: %v", targetID, err)
			}

			// Verify reopened
			issue, err = ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show after Reopen failed: %v", err)
			}
			if issue.Status != "open" {
				return fmt.Errorf("Status after Reopen = %q, want %q", issue.Status, "open")
			}
			return nil
		},
	})
}

func TestMatrix_Update_LabelOperations(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Update_LabelOperations",
		Operation: "Update",
		Test: func(ops beads.BeadsOps) error {
			issue, _ := ops.Create(beads.CreateOptions{Title: "Label Test", Type: "task"})

			// Add labels via Update
			err := ops.Update(issue.ID, beads.UpdateOptions{
				AddLabels: []string{"label1", "label2"},
			})
			if err != nil {
				return fmt.Errorf("Update AddLabels failed: %v", err)
			}

			// Verify
			updated, _ := ops.Show(issue.ID)
			found1, found2 := false, false
			for _, l := range updated.Labels {
				if l == "label1" {
					found1 = true
				}
				if l == "label2" {
					found2 = true
				}
			}
			if !found1 || !found2 {
				return fmt.Errorf("Labels not added correctly")
			}

			// Remove a label
			err = ops.Update(issue.ID, beads.UpdateOptions{
				RemoveLabels: []string{"label1"},
			})
			if err != nil {
				return fmt.Errorf("Update RemoveLabels failed: %v", err)
			}

			// Verify removal
			updated, _ = ops.Show(issue.ID)
			for _, l := range updated.Labels {
				if l == "label1" {
					return fmt.Errorf("label1 should have been removed")
				}
			}
			return nil
		},
	})
}

func TestMatrix_Close_Multiple(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Close_Multiple",
		Operation: "Close",
		Test: func(ops beads.BeadsOps) error {
			// Create multiple issues
			issue1, _ := ops.Create(beads.CreateOptions{Title: "Issue 1", Type: "task"})
			issue2, _ := ops.Create(beads.CreateOptions{Title: "Issue 2", Type: "task"})
			issue3, _ := ops.Create(beads.CreateOptions{Title: "Issue 3", Type: "task"})

			// Close all at once
			err := ops.Close(issue1.ID, issue2.ID, issue3.ID)
			if err != nil {
				return fmt.Errorf("Close multiple failed: %v", err)
			}

			// Verify all closed
			for _, id := range []string{issue1.ID, issue2.ID, issue3.ID} {
				issue, _ := ops.Show(id)
				if issue.Status != "closed" {
					return fmt.Errorf("Issue %s should be closed", id)
				}
			}
			return nil
		},
	})
}

func TestMatrix_Close_Idempotent(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Close_Idempotent",
		Operation: "Close",
		Test: func(ops beads.BeadsOps) error {
			issue, _ := ops.Create(beads.CreateOptions{Title: "Idempotent", Type: "task"})

			// Close twice - should not error
			_ = ops.Close(issue.ID)
			err := ops.Close(issue.ID)
			if err != nil {
				return fmt.Errorf("Second Close should not error: %v", err)
			}

			// Still closed
			updated, _ := ops.Show(issue.ID)
			if updated.Status != "closed" {
				return fmt.Errorf("Issue should still be closed")
			}
			return nil
		},
	})
}
