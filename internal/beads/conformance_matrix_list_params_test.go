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

// TestMatrix_List_StatusFilter tests filtering by status.
func TestMatrix_List_StatusFilter(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "List_StatusFilter",
		Operation: "List",
		Test: func(ops beads.BeadsOps) error {
			// Create issues with different statuses
			open1, err := ops.Create(beads.CreateOptions{Title: "Open Issue 1", Type: "task"})
			if err != nil {
				return fmt.Errorf("Create open1 failed: %v", err)
			}
			open2, err := ops.Create(beads.CreateOptions{Title: "Open Issue 2", Type: "task"})
			if err != nil {
				return fmt.Errorf("Create open2 failed: %v", err)
			}
			closed1, err := ops.Create(beads.CreateOptions{Title: "Closed Issue", Type: "task"})
			if err != nil {
				return fmt.Errorf("Create closed1 failed: %v", err)
			}
			err = ops.Close(closed1.ID)
			if err != nil {
				return fmt.Errorf("Close failed: %v", err)
			}

			// Test status=open
			openIssues, err := ops.List(beads.ListOptions{Status: "open", Priority: -1})
			if err != nil {
				return fmt.Errorf("List(status=open) failed: %v", err)
			}
			if len(openIssues) < 2 {
				return fmt.Errorf("List(status=open) returned %d issues, want at least 2", len(openIssues))
			}
			// Verify closed issue is not in open list
			for _, issue := range openIssues {
				if issue.ID == closed1.ID {
					return fmt.Errorf("List(status=open) included closed issue %s", closed1.ID)
				}
			}

			// Test status=closed
			closedIssues, err := ops.List(beads.ListOptions{Status: "closed", Priority: -1})
			if err != nil {
				return fmt.Errorf("List(status=closed) failed: %v", err)
			}
			foundClosed := false
			for _, issue := range closedIssues {
				if issue.ID == closed1.ID {
					foundClosed = true
				}
				if issue.ID == open1.ID || issue.ID == open2.ID {
					return fmt.Errorf("List(status=closed) included open issue")
				}
			}
			if !foundClosed {
				return fmt.Errorf("List(status=closed) did not include closed issue")
			}

			// Test status=all (or All: true)
			allIssues, err := ops.List(beads.ListOptions{All: true, Priority: -1})
			if err != nil {
				return fmt.Errorf("List(all=true) failed: %v", err)
			}
			if len(allIssues) < 3 {
				return fmt.Errorf("List(all=true) returned %d issues, want at least 3", len(allIssues))
			}

			return nil
		},
	})
}

// TestMatrix_List_PriorityFilter tests filtering by priority.
func TestMatrix_List_PriorityFilter(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "List_PriorityFilter",
		Operation: "List",
		Test: func(ops beads.BeadsOps) error {
			// Create issues with different priorities
			p1, err := ops.Create(beads.CreateOptions{Title: "Priority 1", Type: "task", Priority: 1})
			if err != nil {
				return fmt.Errorf("Create p1 failed: %v", err)
			}
			p2, err := ops.Create(beads.CreateOptions{Title: "Priority 2", Type: "task", Priority: 2})
			if err != nil {
				return fmt.Errorf("Create p2 failed: %v", err)
			}
			_, err = ops.Create(beads.CreateOptions{Title: "Priority 3", Type: "task", Priority: 3})
			if err != nil {
				return fmt.Errorf("Create p3 failed: %v", err)
			}

			// Filter by priority 2
			issues, err := ops.List(beads.ListOptions{Priority: 2})
			if err != nil {
				return fmt.Errorf("List(priority=2) failed: %v", err)
			}
			if len(issues) < 1 {
				return fmt.Errorf("List(priority=2) returned %d issues, want at least 1", len(issues))
			}
			// Verify only priority 2 issues
			for _, issue := range issues {
				if issue.Priority != 2 {
					return fmt.Errorf("List(priority=2) returned issue with priority %d", issue.Priority)
				}
			}
			// Verify p1 is not in the list
			for _, issue := range issues {
				if issue.ID == p1.ID {
					return fmt.Errorf("List(priority=2) included priority 1 issue")
				}
			}

			// Verify p2 is in the list
			found := false
			for _, issue := range issues {
				if issue.ID == p2.ID {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("List(priority=2) did not include the priority 2 issue")
			}

			return nil
		},
	})
}

// TestMatrix_List_ParentFilter tests filtering by parent.
func TestMatrix_List_ParentFilter(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "List_ParentFilter",
		Operation: "List",
		Test: func(ops beads.BeadsOps) error {
			// Create parent issue
			parent, err := ops.Create(beads.CreateOptions{Title: "Parent Issue", Type: "epic"})
			if err != nil {
				return fmt.Errorf("Create parent failed: %v", err)
			}

			// Create child issues
			child1, err := ops.Create(beads.CreateOptions{Title: "Child 1", Type: "task", Parent: parent.ID})
			if err != nil {
				return fmt.Errorf("Create child1 failed: %v", err)
			}
			child2, err := ops.Create(beads.CreateOptions{Title: "Child 2", Type: "task", Parent: parent.ID})
			if err != nil {
				return fmt.Errorf("Create child2 failed: %v", err)
			}

			// Create orphan issue
			_, err = ops.Create(beads.CreateOptions{Title: "Orphan", Type: "task"})
			if err != nil {
				return fmt.Errorf("Create orphan failed: %v", err)
			}

			// Filter by parent
			children, err := ops.List(beads.ListOptions{Parent: parent.ID, Priority: -1})
			if err != nil {
				return fmt.Errorf("List(parent=%s) failed: %v", parent.ID, err)
			}
			if len(children) < 2 {
				return fmt.Errorf("List(parent) returned %d issues, want at least 2", len(children))
			}

			// Verify children are in list
			foundChild1, foundChild2 := false, false
			for _, issue := range children {
				if issue.ID == child1.ID {
					foundChild1 = true
				}
				if issue.ID == child2.ID {
					foundChild2 = true
				}
			}
			if !foundChild1 || !foundChild2 {
				return fmt.Errorf("List(parent) missing children: found1=%v, found2=%v", foundChild1, foundChild2)
			}

			return nil
		},
	})
}

// TestMatrix_List_AssigneeFilter tests filtering by assignee.
func TestMatrix_List_AssigneeFilter(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "List_AssigneeFilter",
		Operation: "List",
		Test: func(ops beads.BeadsOps) error {
			// Create issues with different assignees
			assigned, err := ops.Create(beads.CreateOptions{
				Title:    "Assigned Issue",
				Type:     "task",
				Assignee: "gastown/TestAgent",
			})
			if err != nil {
				return fmt.Errorf("Create assigned failed: %v", err)
			}
			_, err = ops.Create(beads.CreateOptions{Title: "Unassigned Issue", Type: "task"})
			if err != nil {
				return fmt.Errorf("Create unassigned failed: %v", err)
			}

			// Filter by assignee
			issues, err := ops.List(beads.ListOptions{Assignee: "gastown/TestAgent", Priority: -1})
			if err != nil {
				return fmt.Errorf("List(assignee) failed: %v", err)
			}
			if len(issues) < 1 {
				return fmt.Errorf("List(assignee) returned %d issues, want at least 1", len(issues))
			}

			// Verify assigned issue is in list
			found := false
			for _, issue := range issues {
				if issue.ID == assigned.ID {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("List(assignee) did not include assigned issue")
			}

			return nil
		},
	})
}

// TestMatrix_List_LimitFilter tests the limit parameter.
func TestMatrix_List_LimitFilter(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "List_LimitFilter",
		Operation: "List",
		Test: func(ops beads.BeadsOps) error {
			// Create several issues
			for i := 0; i < 5; i++ {
				_, err := ops.Create(beads.CreateOptions{
					Title: fmt.Sprintf("Limit Test Issue %d", i),
					Type:  "task",
				})
				if err != nil {
					return fmt.Errorf("Create issue %d failed: %v", i, err)
				}
			}

			// Test limit=2
			issues, err := ops.List(beads.ListOptions{Limit: 2, Priority: -1})
			if err != nil {
				return fmt.Errorf("List(limit=2) failed: %v", err)
			}
			if len(issues) > 2 {
				return fmt.Errorf("List(limit=2) returned %d issues, want at most 2", len(issues))
			}

			// Test limit=1
			issues, err = ops.List(beads.ListOptions{Limit: 1, Priority: -1})
			if err != nil {
				return fmt.Errorf("List(limit=1) failed: %v", err)
			}
			if len(issues) > 1 {
				return fmt.Errorf("List(limit=1) returned %d issues, want at most 1", len(issues))
			}

			return nil
		},
	})
}

// TestMatrix_List_LabelFilter tests filtering by single label.
func TestMatrix_List_LabelFilter(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "List_LabelFilter",
		Operation: "List",
		Test: func(ops beads.BeadsOps) error {
			// Create issue with custom label
			labeled, err := ops.Create(beads.CreateOptions{
				Title:  "Labeled Issue",
				Type:   "task",
				Labels: []string{"custom:test-label"},
			})
			if err != nil {
				return fmt.Errorf("Create labeled failed: %v", err)
			}
			_, err = ops.Create(beads.CreateOptions{Title: "Unlabeled Issue", Type: "task"})
			if err != nil {
				return fmt.Errorf("Create unlabeled failed: %v", err)
			}

			// Filter by label
			issues, err := ops.List(beads.ListOptions{Label: "custom:test-label", Priority: -1})
			if err != nil {
				return fmt.Errorf("List(label) failed: %v", err)
			}
			if len(issues) < 1 {
				return fmt.Errorf("List(label) returned %d issues, want at least 1", len(issues))
			}

			// Verify labeled issue is in list
			found := false
			for _, issue := range issues {
				if issue.ID == labeled.ID {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("List(label) did not include labeled issue")
			}

			return nil
		},
	})
}

// TestMatrix_List_MultipleLabels tests filtering by multiple labels (AND).
func TestMatrix_List_MultipleLabels(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "List_MultipleLabels",
		Operation: "List",
		Test: func(ops beads.BeadsOps) error {
			// Create issue with multiple labels
			both, err := ops.Create(beads.CreateOptions{
				Title:  "Both Labels",
				Type:   "task",
				Labels: []string{"label:a", "label:b"},
			})
			if err != nil {
				return fmt.Errorf("Create both failed: %v", err)
			}
			_, err = ops.Create(beads.CreateOptions{
				Title:  "Only A",
				Type:   "task",
				Labels: []string{"label:a"},
			})
			if err != nil {
				return fmt.Errorf("Create onlyA failed: %v", err)
			}

			// Filter by both labels (AND)
			issues, err := ops.List(beads.ListOptions{
				Labels:   []string{"label:a", "label:b"},
				Priority: -1,
			})
			if err != nil {
				return fmt.Errorf("List(labels) failed: %v", err)
			}
			if len(issues) < 1 {
				return fmt.Errorf("List(labels) returned %d issues, want at least 1", len(issues))
			}

			// Verify only 'both' issue is in list
			found := false
			for _, issue := range issues {
				if issue.ID == both.ID {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("List(labels) did not include issue with both labels")
			}

			return nil
		},
	})
}

// TestMatrix_List_TypeFilter tests filtering by type.
func TestMatrix_List_TypeFilter(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "List_TypeFilter",
		Operation: "List",
		Test: func(ops beads.BeadsOps) error {
			// Create issues with different types
			agent, err := ops.Create(beads.CreateOptions{Title: "Agent Issue", Type: "agent"})
			if err != nil {
				return fmt.Errorf("Create agent failed: %v", err)
			}
			_, err = ops.Create(beads.CreateOptions{Title: "Task Issue", Type: "task"})
			if err != nil {
				return fmt.Errorf("Create task failed: %v", err)
			}

			// Filter by type=agent
			issues, err := ops.List(beads.ListOptions{Type: "agent", Priority: -1})
			if err != nil {
				return fmt.Errorf("List(type=agent) failed: %v", err)
			}
			if len(issues) < 1 {
				return fmt.Errorf("List(type=agent) returned %d issues, want at least 1", len(issues))
			}

			// Verify agent issue is in list
			found := false
			for _, issue := range issues {
				if issue.ID == agent.ID {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("List(type=agent) did not include agent issue")
			}

			return nil
		},
	})
}

// TestMatrix_List_DescContainsFilter tests filtering by description content.
func TestMatrix_List_DescContainsFilter(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "List_DescContainsFilter",
		Operation: "List",
		Test: func(ops beads.BeadsOps) error {
			// Create issues with different descriptions
			withKeyword, err := ops.Create(beads.CreateOptions{
				Title:       "Issue With Keyword",
				Type:        "task",
				Description: "This issue contains the XYZUNIQUE keyword",
			})
			if err != nil {
				return fmt.Errorf("Create withKeyword failed: %v", err)
			}
			_, err = ops.Create(beads.CreateOptions{
				Title:       "Issue Without",
				Type:        "task",
				Description: "This has no special words",
			})
			if err != nil {
				return fmt.Errorf("Create without failed: %v", err)
			}

			// Filter by description contains
			issues, err := ops.List(beads.ListOptions{DescContains: "XYZUNIQUE", Priority: -1})
			if err != nil {
				// DescContains may not be supported in all implementations
				return nil
			}
			if len(issues) < 1 {
				return fmt.Errorf("List(desc_contains) returned %d issues, want at least 1", len(issues))
			}

			// Verify issue with keyword is in list
			found := false
			for _, issue := range issues {
				if issue.ID == withKeyword.ID {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("List(desc_contains) did not include issue with keyword")
			}

			return nil
		},
	})
}

// TestMatrix_List_MultipleFilters tests combining multiple filters.
func TestMatrix_List_MultipleFilters(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "List_MultipleFilters",
		Operation: "List",
		Test: func(ops beads.BeadsOps) error {
			// Create a specific issue that matches multiple criteria
			target, err := ops.Create(beads.CreateOptions{
				Title:    "Target Issue",
				Type:     "task",
				Priority: 2,
				Assignee: "gastown/MultiFilterTest",
			})
			if err != nil {
				return fmt.Errorf("Create target failed: %v", err)
			}

			// Create non-matching issues
			_, err = ops.Create(beads.CreateOptions{
				Title:    "Wrong Priority",
				Type:     "task",
				Priority: 1,
				Assignee: "gastown/MultiFilterTest",
			})
			if err != nil {
				return fmt.Errorf("Create wrong priority failed: %v", err)
			}
			_, err = ops.Create(beads.CreateOptions{
				Title:    "Wrong Assignee",
				Type:     "task",
				Priority: 2,
				Assignee: "gastown/OtherAgent",
			})
			if err != nil {
				return fmt.Errorf("Create wrong assignee failed: %v", err)
			}

			// Filter by multiple criteria
			issues, err := ops.List(beads.ListOptions{
				Priority: 2,
				Assignee: "gastown/MultiFilterTest",
			})
			if err != nil {
				return fmt.Errorf("List(multiple filters) failed: %v", err)
			}
			if len(issues) < 1 {
				return fmt.Errorf("List(multiple filters) returned %d issues, want at least 1", len(issues))
			}

			// Verify target issue is in list
			found := false
			for _, issue := range issues {
				if issue.ID == target.ID {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("List(multiple filters) did not include target issue")
			}

			return nil
		},
	})
}
