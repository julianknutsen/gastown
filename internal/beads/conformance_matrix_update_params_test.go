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

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

// TestMatrix_Update_Title tests updating the title field.
func TestMatrix_Update_Title(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "Update_Title",
		Operation: "Update",
		Test: func(ops beads.BeadsOps, targetID string) error {
			newTitle := "Updated Title"
			err := ops.Update(targetID, beads.UpdateOptions{Title: strPtr(newTitle)})
			if err != nil {
				return fmt.Errorf("Update failed: %v", err)
			}

			issue, err := ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show failed: %v", err)
			}
			if issue.Title != newTitle {
				return fmt.Errorf("Title = %q, want %q", issue.Title, newTitle)
			}
			return nil
		},
	})
}

// TestMatrix_Update_Status tests updating the status field.
func TestMatrix_Update_Status(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "Update_Status",
		Operation: "Update",
		Test: func(ops beads.BeadsOps, targetID string) error {
			// Update to in_progress
			err := ops.Update(targetID, beads.UpdateOptions{Status: strPtr("in_progress")})
			if err != nil {
				return fmt.Errorf("Update to in_progress failed: %v", err)
			}

			issue, err := ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show failed: %v", err)
			}
			if issue.Status != "in_progress" {
				return fmt.Errorf("Status = %q, want %q", issue.Status, "in_progress")
			}

			// Update back to open
			err = ops.Update(targetID, beads.UpdateOptions{Status: strPtr("open")})
			if err != nil {
				return fmt.Errorf("Update to open failed: %v", err)
			}
			issue, err = ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show after reopen failed: %v", err)
			}
			if issue.Status != "open" {
				return fmt.Errorf("Status = %q, want %q", issue.Status, "open")
			}

			return nil
		},
	})
}

// TestMatrix_Update_Priority tests updating the priority field.
func TestMatrix_Update_Priority(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "Update_Priority",
		Operation: "Update",
		Test: func(ops beads.BeadsOps, targetID string) error {
			// Update to priority 3
			err := ops.Update(targetID, beads.UpdateOptions{Priority: intPtr(3)})
			if err != nil {
				return fmt.Errorf("Update failed: %v", err)
			}

			issue, err := ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show failed: %v", err)
			}
			if issue.Priority != 3 {
				return fmt.Errorf("Priority = %d, want 3", issue.Priority)
			}
			return nil
		},
	})
}

// TestMatrix_Update_Description tests updating the description field.
func TestMatrix_Update_Description(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "Update_Description",
		Operation: "Update",
		Test: func(ops beads.BeadsOps, targetID string) error {
			newDesc := "Updated description with new content"
			err := ops.Update(targetID, beads.UpdateOptions{Description: strPtr(newDesc)})
			if err != nil {
				return fmt.Errorf("Update failed: %v", err)
			}

			issue, err := ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show failed: %v", err)
			}
			if issue.Description != newDesc {
				return fmt.Errorf("Description = %q, want %q", issue.Description, newDesc)
			}
			return nil
		},
	})
}

// TestMatrix_Update_Assignee tests updating the assignee field.
func TestMatrix_Update_Assignee(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "Update_Assignee",
		Operation: "Update",
		Test: func(ops beads.BeadsOps, targetID string) error {
			newAssignee := "gastown/UpdatedAgent"
			err := ops.Update(targetID, beads.UpdateOptions{Assignee: strPtr(newAssignee)})
			if err != nil {
				return fmt.Errorf("Update failed: %v", err)
			}

			issue, err := ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show failed: %v", err)
			}
			if issue.Assignee != newAssignee {
				return fmt.Errorf("Assignee = %q, want %q", issue.Assignee, newAssignee)
			}
			return nil
		},
	})
}

// TestMatrix_Update_Unassign tests removing the assignee.
// NOTE: bd CLI doesn't support --unassign flag, so we use --assignee="" to clear.
func TestMatrix_Update_Unassign(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "Update_Unassign",
		Operation: "Update",
		Test: func(ops beads.BeadsOps, targetID string) error {
			// First set an assignee
			err := ops.Update(targetID, beads.UpdateOptions{Assignee: strPtr("gastown/ToBeRemoved")})
			if err != nil {
				return fmt.Errorf("Update to set assignee failed: %v", err)
			}

			// Verify assignee is set
			issue, err := ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show after assign failed: %v", err)
			}
			if issue.Assignee != "gastown/ToBeRemoved" {
				return fmt.Errorf("Assignee not set: got %q", issue.Assignee)
			}

			// Now unassign
			err = ops.Update(targetID, beads.UpdateOptions{Unassign: true})
			if err != nil {
				return fmt.Errorf("Update to unassign failed: %v", err)
			}

			issue, err = ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show after unassign failed: %v", err)
			}
			if issue.Assignee != "" {
				return fmt.Errorf("Assignee = %q after unassign, want empty", issue.Assignee)
			}
			return nil
		},
	})
}

// TestMatrix_Update_AddLabels tests adding labels.
func TestMatrix_Update_AddLabels(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "Update_AddLabels",
		Operation: "Update",
		Test: func(ops beads.BeadsOps, targetID string) error {
			labels := []string{"custom:update-add1", "custom:update-add2"}
			err := ops.Update(targetID, beads.UpdateOptions{AddLabels: labels})
			if err != nil {
				return fmt.Errorf("Update failed: %v", err)
			}

			issue, err := ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show failed: %v", err)
			}
			for _, wantLabel := range labels {
				found := false
				for _, gotLabel := range issue.Labels {
					if gotLabel == wantLabel {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("Missing label %q in %v", wantLabel, issue.Labels)
				}
			}
			return nil
		},
	})
}

// TestMatrix_Update_RemoveLabels tests removing labels.
func TestMatrix_Update_RemoveLabels(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "Update_RemoveLabels",
		Operation: "Update",
		Test: func(ops beads.BeadsOps, targetID string) error {
			// First add a label
			err := ops.Update(targetID, beads.UpdateOptions{AddLabels: []string{"custom:to-remove"}})
			if err != nil {
				return fmt.Errorf("Update to add label failed: %v", err)
			}

			// Verify label exists
			issue, err := ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show after add failed: %v", err)
			}
			found := false
			for _, l := range issue.Labels {
				if l == "custom:to-remove" {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("Label not added: %v", issue.Labels)
			}

			// Now remove the label
			err = ops.Update(targetID, beads.UpdateOptions{RemoveLabels: []string{"custom:to-remove"}})
			if err != nil {
				return fmt.Errorf("Update to remove label failed: %v", err)
			}

			issue, err = ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show after remove failed: %v", err)
			}
			for _, l := range issue.Labels {
				if l == "custom:to-remove" {
					return fmt.Errorf("Label still present after removal: %v", issue.Labels)
				}
			}
			return nil
		},
	})
}

// TestMatrix_Update_SetLabels tests replacing all labels.
func TestMatrix_Update_SetLabels(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "Update_SetLabels",
		Operation: "Update",
		Test: func(ops beads.BeadsOps, targetID string) error {
			// First add some labels
			err := ops.Update(targetID, beads.UpdateOptions{AddLabels: []string{"custom:old1", "custom:old2"}})
			if err != nil {
				return fmt.Errorf("Update to add labels failed: %v", err)
			}

			// Now set entirely new labels
			newLabels := []string{"custom:new1", "custom:new2"}
			err = ops.Update(targetID, beads.UpdateOptions{SetLabels: newLabels})
			if err != nil {
				return fmt.Errorf("Update to set labels failed: %v", err)
			}

			issue, err := ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show failed: %v", err)
			}

			// Verify new labels exist
			for _, wantLabel := range newLabels {
				found := false
				for _, gotLabel := range issue.Labels {
					if gotLabel == wantLabel {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("Missing new label %q in %v", wantLabel, issue.Labels)
				}
			}

			// Verify old labels removed
			for _, l := range issue.Labels {
				if l == "custom:old1" || l == "custom:old2" {
					return fmt.Errorf("Old label %q still present in %v", l, issue.Labels)
				}
			}

			return nil
		},
	})
}

// TestMatrix_Update_MultipleFields tests updating multiple fields at once.
func TestMatrix_Update_MultipleFields(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "Update_MultipleFields",
		Operation: "Update",
		Test: func(ops beads.BeadsOps, targetID string) error {
			newTitle := "Multi-Update Title"
			newPriority := 4
			newDesc := "Multi-update description"
			newAssignee := "gastown/MultiAgent"

			err := ops.Update(targetID, beads.UpdateOptions{
				Title:       strPtr(newTitle),
				Priority:    intPtr(newPriority),
				Description: strPtr(newDesc),
				Assignee:    strPtr(newAssignee),
				AddLabels:   []string{"custom:multi"},
			})
			if err != nil {
				return fmt.Errorf("Update failed: %v", err)
			}

			issue, err := ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show failed: %v", err)
			}

			if issue.Title != newTitle {
				return fmt.Errorf("Title = %q, want %q", issue.Title, newTitle)
			}
			if issue.Priority != newPriority {
				return fmt.Errorf("Priority = %d, want %d", issue.Priority, newPriority)
			}
			if issue.Description != newDesc {
				return fmt.Errorf("Description = %q, want %q", issue.Description, newDesc)
			}
			if issue.Assignee != newAssignee {
				return fmt.Errorf("Assignee = %q, want %q", issue.Assignee, newAssignee)
			}
			found := false
			for _, l := range issue.Labels {
				if l == "custom:multi" {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("Missing label custom:multi in %v", issue.Labels)
			}

			return nil
		},
	})
}
