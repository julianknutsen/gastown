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

// TestMatrix_Release tests that Release moves an in_progress issue back to open.
func TestMatrix_Release(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "Release",
		Operation: "Release",
		Test: func(ops beads.BeadsOps, targetID string) error {
			// First set to in_progress
			inProgress := "in_progress"
			err := ops.Update(targetID, beads.UpdateOptions{Status: &inProgress})
			if err != nil {
				return fmt.Errorf("Update to in_progress failed: %v", err)
			}

			// Verify in_progress
			issue, err := ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show after Update failed: %v", err)
			}
			if issue.Status != "in_progress" {
				return fmt.Errorf("Status = %q, want in_progress", issue.Status)
			}

			// Release back to open
			err = ops.Release(targetID)
			if err != nil {
				return fmt.Errorf("Release(%s) failed: %v", targetID, err)
			}

			// Verify back to open
			issue, err = ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show after Release failed: %v", err)
			}
			if issue.Status != "open" {
				return fmt.Errorf("Status after Release = %q, want open", issue.Status)
			}
			return nil
		},
	})
}

// TestMatrix_ReleaseWithReason tests Release with a reason comment.
func TestMatrix_ReleaseWithReason(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "ReleaseWithReason",
		Operation: "ReleaseWithReason",
		Test: func(ops beads.BeadsOps, targetID string) error {
			// First set to in_progress
			inProgress := "in_progress"
			err := ops.Update(targetID, beads.UpdateOptions{Status: &inProgress})
			if err != nil {
				return fmt.Errorf("Update to in_progress failed: %v", err)
			}

			// Release with reason
			err = ops.ReleaseWithReason(targetID, "blocked on dependency")
			if err != nil {
				return fmt.Errorf("ReleaseWithReason(%s) failed: %v", targetID, err)
			}

			// Verify back to open
			issue, err := ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show after ReleaseWithReason failed: %v", err)
			}
			if issue.Status != "open" {
				return fmt.Errorf("Status after ReleaseWithReason = %q, want open", issue.Status)
			}
			return nil
		},
	})
}

// TestMatrix_ListByAssignee tests listing issues by assignee.
func TestMatrix_ListByAssignee(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "ListByAssignee",
		Operation: "ListByAssignee",
		Test: func(ops beads.BeadsOps) error {
			assignee := "test-worker"

			// Create issues with different assignees
			issue1, err := ops.Create(beads.CreateOptions{
				Title:    "Assigned Task",
				Type:     "task",
				Assignee: assignee,
			})
			if err != nil {
				return fmt.Errorf("Create assigned task failed: %v", err)
			}

			_, err = ops.Create(beads.CreateOptions{
				Title: "Unassigned Task",
				Type:  "task",
			})
			if err != nil {
				return fmt.Errorf("Create unassigned task failed: %v", err)
			}

			// List by assignee
			issues, err := ops.ListByAssignee(assignee)
			if err != nil {
				return fmt.Errorf("ListByAssignee(%s) failed: %v", assignee, err)
			}

			// Should find exactly one issue
			if len(issues) != 1 {
				return fmt.Errorf("ListByAssignee returned %d issues, want 1", len(issues))
			}
			if issues[0].ID != issue1.ID {
				return fmt.Errorf("ListByAssignee returned ID %s, want %s", issues[0].ID, issue1.ID)
			}
			return nil
		},
	})
}

// TestMatrix_GetAssignedIssue tests getting the first open issue for an assignee.
func TestMatrix_GetAssignedIssue(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "GetAssignedIssue",
		Operation: "GetAssignedIssue",
		Test: func(ops beads.BeadsOps) error {
			assignee := "test-worker-2"

			// Create an assigned open issue
			issue1, err := ops.Create(beads.CreateOptions{
				Title:    "Open Assigned Task",
				Type:     "task",
				Assignee: assignee,
			})
			if err != nil {
				return fmt.Errorf("Create open assigned task failed: %v", err)
			}

			// Create and close another assigned issue
			issue2, err := ops.Create(beads.CreateOptions{
				Title:    "Closed Assigned Task",
				Type:     "task",
				Assignee: assignee,
			})
			if err != nil {
				return fmt.Errorf("Create closed assigned task failed: %v", err)
			}
			if err := ops.Close(issue2.ID); err != nil {
				return fmt.Errorf("Close failed: %v", err)
			}

			// Get assigned issue - should return the open one
			issue, err := ops.GetAssignedIssue(assignee)
			if err != nil {
				return fmt.Errorf("GetAssignedIssue(%s) failed: %v", assignee, err)
			}
			if issue == nil {
				return fmt.Errorf("GetAssignedIssue returned nil, want issue")
			}
			if issue.ID != issue1.ID {
				return fmt.Errorf("GetAssignedIssue returned ID %s, want %s", issue.ID, issue1.ID)
			}
			return nil
		},
	})
}

// TestMatrix_GetAssignedIssue_NoMatch tests GetAssignedIssue when no issue matches.
func TestMatrix_GetAssignedIssue_NoMatch(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "GetAssignedIssue_NoMatch",
		Operation: "GetAssignedIssue",
		Test: func(ops beads.BeadsOps) error {
			// Get assigned issue for non-existent assignee
			issue, err := ops.GetAssignedIssue("nonexistent-worker")
			if err != nil {
				return fmt.Errorf("GetAssignedIssue failed: %v", err)
			}
			// Should return nil when no match
			if issue != nil {
				return fmt.Errorf("GetAssignedIssue returned %v, want nil", issue)
			}
			return nil
		},
	})
}

// TestMatrix_DeleteWithOptions tests Delete with force option.
func TestMatrix_DeleteWithOptions(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "DeleteWithOptions",
		Operation: "DeleteWithOptions",
		Test: func(ops beads.BeadsOps, targetID string) error {
			// Delete with force option
			err := ops.DeleteWithOptions(beads.DeleteOptions{Force: true}, targetID)
			if err != nil {
				return fmt.Errorf("DeleteWithOptions(%s) failed: %v", targetID, err)
			}

			// Verify deleted
			_, err = ops.Show(targetID)
			if err != beads.ErrNotFound {
				return fmt.Errorf("Show after DeleteWithOptions returned err=%v, want ErrNotFound", err)
			}
			return nil
		},
	})
}

// TestMatrix_CloseWithOptions tests Close with session option.
func TestMatrix_CloseWithOptions(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "CloseWithOptions",
		Operation: "CloseWithOptions",
		Test: func(ops beads.BeadsOps, targetID string) error {
			// Close with options
			err := ops.CloseWithOptions(beads.CloseOptions{
				Reason:  "completed",
				Session: "test-session-123",
			}, targetID)
			if err != nil {
				return fmt.Errorf("CloseWithOptions(%s) failed: %v", targetID, err)
			}

			// Verify closed
			issue, err := ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show after CloseWithOptions failed: %v", err)
			}
			if issue.Status != "closed" {
				return fmt.Errorf("Status = %q, want closed", issue.Status)
			}
			return nil
		},
	})
}
