package beads_test

import (
	"fmt"
	"testing"

	"github.com/steveyegge/gastown/internal/beads"
)

func TestMatrix_Create(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Create",
		Operation: "Create",
		Test: func(ops beads.BeadsOps) error {
			issue, err := ops.Create(beads.CreateOptions{
				Title:       "Test Issue",
				Type:        "task",
				Priority:    2,
				Description: "Test description",
			})
			if err != nil {
				return fmt.Errorf("Create failed: %v", err)
			}
			if issue.ID == "" {
				return fmt.Errorf("Create returned issue with empty ID")
			}
			if issue.Title != "Test Issue" {
				return fmt.Errorf("Title = %q, want %q", issue.Title, "Test Issue")
			}
			if issue.Status != "open" {
				return fmt.Errorf("Status = %q, want %q", issue.Status, "open")
			}
			return nil
		},
	})
}

func TestMatrix_CreateWithID(t *testing.T) {
	// Use unique counter for each test target
	var idCounter int
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "CreateWithID",
		Operation: "CreateWithID",
		Test: func(ops beads.BeadsOps) error {
			// Use prefix-appropriate ID (gt- for the gastown test database)
			// Each invocation gets a unique ID to avoid conflicts between targets.
			idCounter++
			customID := fmt.Sprintf("gt-custom-test-%d", idCounter)
			issue, err := ops.CreateWithID(customID, beads.CreateOptions{
				Title: "Custom ID Issue",
				Type:  "task",
			})
			if err != nil {
				return fmt.Errorf("CreateWithID failed: %v", err)
			}
			if issue == nil {
				return fmt.Errorf("CreateWithID returned nil issue")
			}
			if issue.ID != customID {
				return fmt.Errorf("ID = %q, want %q", issue.ID, customID)
			}
			return nil
		},
	})
}

func TestMatrix_List(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "List",
		Operation: "List",
		Test: func(ops beads.BeadsOps) error {
			// Create multiple issues
			_, err := ops.Create(beads.CreateOptions{Title: "Task 1", Type: "task", Priority: 0})
			if err != nil {
				return fmt.Errorf("Create failed: %v", err)
			}
			_, err = ops.Create(beads.CreateOptions{Title: "Task 2", Type: "task", Priority: 0})
			if err != nil {
				return fmt.Errorf("Create failed: %v", err)
			}
			closed, err := ops.Create(beads.CreateOptions{Title: "Closed", Type: "task", Priority: 0})
			if err != nil {
				return fmt.Errorf("Create failed: %v", err)
			}
			_ = ops.Close(closed.ID)

			// List open issues
			open, err := ops.List(beads.ListOptions{Status: "open", Priority: -1})
			if err != nil {
				return fmt.Errorf("List failed: %v", err)
			}
			if len(open) < 2 {
				return fmt.Errorf("List(open) returned %d issues, want at least 2", len(open))
			}

			// Verify closed issue is not in open list
			for _, issue := range open {
				if issue.ID == closed.ID {
					return fmt.Errorf("Closed issue should not be in open list")
				}
			}
			return nil
		},
	})
}

func TestMatrix_List_EmptyRepo(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "List_EmptyRepo",
		Operation: "List",
		Test: func(ops beads.BeadsOps) error {
			// List on empty repo should return empty slice, not error
			issues, err := ops.List(beads.ListOptions{Priority: -1})
			if err != nil {
				return fmt.Errorf("List on empty repo failed: %v", err)
			}
			if len(issues) != 0 {
				return fmt.Errorf("List on empty repo returned %d issues, want 0", len(issues))
			}
			return nil
		},
	})
}

func TestMatrix_List_FilterByStatus(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "List_FilterByStatus",
		Operation: "List",
		Test: func(ops beads.BeadsOps) error {
			// Create issues in different states
			_, _ = ops.Create(beads.CreateOptions{Title: "Open 1", Type: "task"})
			_, _ = ops.Create(beads.CreateOptions{Title: "Open 2", Type: "task"})
			closed1, _ := ops.Create(beads.CreateOptions{Title: "Closed 1", Type: "task"})
			_ = ops.Close(closed1.ID)

			// Test status=open
			openIssues, err := ops.List(beads.ListOptions{Status: "open", Priority: -1})
			if err != nil {
				return fmt.Errorf("List(open) failed: %v", err)
			}
			if len(openIssues) != 2 {
				return fmt.Errorf("List(open) returned %d issues, want 2", len(openIssues))
			}

			// Test status=closed
			closedIssues, err := ops.List(beads.ListOptions{Status: "closed", Priority: -1})
			if err != nil {
				return fmt.Errorf("List(closed) failed: %v", err)
			}
			if len(closedIssues) != 1 {
				return fmt.Errorf("List(closed) returned %d issues, want 1", len(closedIssues))
			}

			// Test status=all
			allIssues, err := ops.List(beads.ListOptions{Status: "all", Priority: -1})
			if err != nil {
				return fmt.Errorf("List(all) failed: %v", err)
			}
			if len(allIssues) != 3 {
				return fmt.Errorf("List(all) returned %d issues, want 3", len(allIssues))
			}
			return nil
		},
	})
}

func TestMatrix_List_FilterByLabel(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "List_FilterByLabel",
		Operation: "List",
		Test: func(ops beads.BeadsOps) error {
			// Create issues with different types (which become labels)
			task, _ := ops.Create(beads.CreateOptions{Title: "Task", Type: "task"})
			_, _ = ops.Create(beads.CreateOptions{Title: "Bug", Type: "bug"})
			_, _ = ops.Create(beads.CreateOptions{Title: "Feature", Type: "feature"})

			// Filter by gt:task label
			tasks, err := ops.List(beads.ListOptions{Label: "gt:task", Priority: -1})
			if err != nil {
				return fmt.Errorf("List(label=gt:task) failed: %v", err)
			}
			if len(tasks) != 1 {
				return fmt.Errorf("List(label=gt:task) returned %d issues, want 1", len(tasks))
			}
			if len(tasks) > 0 && tasks[0].ID != task.ID {
				return fmt.Errorf("List(label=gt:task) returned wrong issue")
			}
			return nil
		},
	})
}

func TestMatrix_Create_AllOptions(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Create_AllOptions",
		Operation: "Create",
		Test: func(ops beads.BeadsOps) error {
			issue, err := ops.Create(beads.CreateOptions{
				Title:       "Full Options Issue",
				Type:        "feature",
				Priority:    3,
				Description: "Detailed description here",
			})
			if err != nil {
				return fmt.Errorf("Create failed: %v", err)
			}
			if issue.Title != "Full Options Issue" {
				return fmt.Errorf("Title mismatch")
			}
			if issue.Priority != 3 {
				return fmt.Errorf("Priority = %d, want 3", issue.Priority)
			}
			return nil
		},
	})
}
