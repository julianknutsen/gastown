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

// TestMatrix_Create_WithTitle tests creating with just a title.
func TestMatrix_Create_WithTitle(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Create_WithTitle",
		Operation: "Create",
		Test: func(ops beads.BeadsOps) error {
			issue, err := ops.Create(beads.CreateOptions{Title: "Test Title Only"})
			if err != nil {
				return fmt.Errorf("Create failed: %v", err)
			}
			if issue.Title != "Test Title Only" {
				return fmt.Errorf("Title = %q, want %q", issue.Title, "Test Title Only")
			}
			return nil
		},
	})
}

// TestMatrix_Create_WithType tests creating with different types.
func TestMatrix_Create_WithType(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Create_WithType",
		Operation: "Create",
		Test: func(ops beads.BeadsOps) error {
			// Test task type
			task, err := ops.Create(beads.CreateOptions{Title: "Task Type", Type: "task"})
			if err != nil {
				return fmt.Errorf("Create task failed: %v", err)
			}
			taskIssue, err := ops.Show(task.ID)
			if err != nil {
				return fmt.Errorf("Show task failed: %v", err)
			}
			// Verify gt:task label exists
			hasLabel := false
			for _, l := range taskIssue.Labels {
				if l == "gt:task" {
					hasLabel = true
					break
				}
			}
			if !hasLabel {
				return fmt.Errorf("task issue should have gt:task label, got: %v", taskIssue.Labels)
			}

			// Test agent type
			agent, err := ops.Create(beads.CreateOptions{Title: "Agent Type", Type: "agent"})
			if err != nil {
				return fmt.Errorf("Create agent failed: %v", err)
			}
			agentIssue, err := ops.Show(agent.ID)
			if err != nil {
				return fmt.Errorf("Show agent failed: %v", err)
			}
			hasLabel = false
			for _, l := range agentIssue.Labels {
				if l == "gt:agent" {
					hasLabel = true
					break
				}
			}
			if !hasLabel {
				return fmt.Errorf("agent issue should have gt:agent label, got: %v", agentIssue.Labels)
			}

			return nil
		},
	})
}

// TestMatrix_Create_WithPriority tests creating with different priorities.
func TestMatrix_Create_WithPriority(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Create_WithPriority",
		Operation: "Create",
		Test: func(ops beads.BeadsOps) error {
			// Test priority 1 (high)
			p1, err := ops.Create(beads.CreateOptions{Title: "Priority 1", Type: "task", Priority: 1})
			if err != nil {
				return fmt.Errorf("Create p1 failed: %v", err)
			}
			p1Issue, err := ops.Show(p1.ID)
			if err != nil {
				return fmt.Errorf("Show p1 failed: %v", err)
			}
			if p1Issue.Priority != 1 {
				return fmt.Errorf("Priority = %d, want 1", p1Issue.Priority)
			}

			// Test priority 4 (low)
			p4, err := ops.Create(beads.CreateOptions{Title: "Priority 4", Type: "task", Priority: 4})
			if err != nil {
				return fmt.Errorf("Create p4 failed: %v", err)
			}
			p4Issue, err := ops.Show(p4.ID)
			if err != nil {
				return fmt.Errorf("Show p4 failed: %v", err)
			}
			if p4Issue.Priority != 4 {
				return fmt.Errorf("Priority = %d, want 4", p4Issue.Priority)
			}

			return nil
		},
	})
}

// TestMatrix_Create_WithDescription tests creating with a description.
func TestMatrix_Create_WithDescription(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Create_WithDescription",
		Operation: "Create",
		Test: func(ops beads.BeadsOps) error {
			desc := "This is a detailed description\nwith multiple lines\nand special chars: <>&\""
			issue, err := ops.Create(beads.CreateOptions{
				Title:       "With Description",
				Type:        "task",
				Description: desc,
			})
			if err != nil {
				return fmt.Errorf("Create failed: %v", err)
			}

			// Verify description was set
			shown, err := ops.Show(issue.ID)
			if err != nil {
				return fmt.Errorf("Show failed: %v", err)
			}
			if shown.Description != desc {
				return fmt.Errorf("Description = %q, want %q", shown.Description, desc)
			}

			return nil
		},
	})
}

// TestMatrix_Create_WithParent tests creating with a parent.
func TestMatrix_Create_WithParent(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Create_WithParent",
		Operation: "Create",
		Test: func(ops beads.BeadsOps) error {
			// Create parent
			parent, err := ops.Create(beads.CreateOptions{Title: "Parent Epic", Type: "epic"})
			if err != nil {
				return fmt.Errorf("Create parent failed: %v", err)
			}

			// Create child with parent
			child, err := ops.Create(beads.CreateOptions{
				Title:  "Child Task",
				Type:   "task",
				Parent: parent.ID,
			})
			if err != nil {
				return fmt.Errorf("Create child failed: %v", err)
			}

			// Verify parent is set
			shown, err := ops.Show(child.ID)
			if err != nil {
				return fmt.Errorf("Show child failed: %v", err)
			}
			if shown.Parent != parent.ID {
				return fmt.Errorf("Parent = %q, want %q", shown.Parent, parent.ID)
			}

			return nil
		},
	})
}

// TestMatrix_Create_WithAssignee tests creating with an assignee.
func TestMatrix_Create_WithAssignee(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Create_WithAssignee",
		Operation: "Create",
		Test: func(ops beads.BeadsOps) error {
			assignee := "gastown/TestAgent"
			issue, err := ops.Create(beads.CreateOptions{
				Title:    "Assigned Task",
				Type:     "task",
				Assignee: assignee,
			})
			if err != nil {
				return fmt.Errorf("Create failed: %v", err)
			}

			// Verify assignee is set
			shown, err := ops.Show(issue.ID)
			if err != nil {
				return fmt.Errorf("Show failed: %v", err)
			}
			if shown.Assignee != assignee {
				return fmt.Errorf("Assignee = %q, want %q", shown.Assignee, assignee)
			}

			return nil
		},
	})
}

// TestMatrix_Create_WithActor tests creating with an actor (created_by).
func TestMatrix_Create_WithActor(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Create_WithActor",
		Operation: "Create",
		Test: func(ops beads.BeadsOps) error {
			actor := "gastown/CreatorAgent"
			issue, err := ops.Create(beads.CreateOptions{
				Title: "Created By Actor",
				Type:  "task",
				Actor: actor,
			})
			if err != nil {
				return fmt.Errorf("Create failed: %v", err)
			}

			// Verify created_by is set
			shown, err := ops.Show(issue.ID)
			if err != nil {
				return fmt.Errorf("Show failed: %v", err)
			}
			if shown.CreatedBy != actor {
				return fmt.Errorf("CreatedBy = %q, want %q", shown.CreatedBy, actor)
			}

			return nil
		},
	})
}

// TestMatrix_Create_WithLabels tests creating with custom labels.
func TestMatrix_Create_WithLabels(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Create_WithLabels",
		Operation: "Create",
		Test: func(ops beads.BeadsOps) error {
			labels := []string{"custom:label1", "custom:label2"}
			issue, err := ops.Create(beads.CreateOptions{
				Title:  "Labeled Task",
				Type:   "task",
				Labels: labels,
			})
			if err != nil {
				return fmt.Errorf("Create failed: %v", err)
			}

			// Verify labels are set
			shown, err := ops.Show(issue.ID)
			if err != nil {
				return fmt.Errorf("Show failed: %v", err)
			}
			for _, wantLabel := range labels {
				found := false
				for _, gotLabel := range shown.Labels {
					if gotLabel == wantLabel {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("Missing label %q in %v", wantLabel, shown.Labels)
				}
			}

			return nil
		},
	})
}

// TestMatrix_Create_CombinedOptions tests creating with multiple options combined.
func TestMatrix_Create_CombinedOptions(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Create_AllOptions",
		Operation: "Create",
		Test: func(ops beads.BeadsOps) error {
			// Create parent first
			parent, err := ops.Create(beads.CreateOptions{Title: "Parent for AllOptions", Type: "epic"})
			if err != nil {
				return fmt.Errorf("Create parent failed: %v", err)
			}

			// Create with all options
			opts := beads.CreateOptions{
				Title:       "Full Options Task",
				Type:        "task",
				Priority:    2,
				Description: "Full description here",
				Parent:      parent.ID,
				Assignee:    "gastown/FullTest",
				Actor:       "gastown/Creator",
				Labels:      []string{"custom:full", "custom:test"},
			}
			issue, err := ops.Create(opts)
			if err != nil {
				return fmt.Errorf("Create failed: %v", err)
			}

			// Verify all fields
			shown, err := ops.Show(issue.ID)
			if err != nil {
				return fmt.Errorf("Show failed: %v", err)
			}
			if shown.Title != opts.Title {
				return fmt.Errorf("Title = %q, want %q", shown.Title, opts.Title)
			}
			if shown.Priority != opts.Priority {
				return fmt.Errorf("Priority = %d, want %d", shown.Priority, opts.Priority)
			}
			if shown.Description != opts.Description {
				return fmt.Errorf("Description = %q, want %q", shown.Description, opts.Description)
			}
			if shown.Parent != opts.Parent {
				return fmt.Errorf("Parent = %q, want %q", shown.Parent, opts.Parent)
			}
			if shown.Assignee != opts.Assignee {
				return fmt.Errorf("Assignee = %q, want %q", shown.Assignee, opts.Assignee)
			}
			if shown.CreatedBy != opts.Actor {
				return fmt.Errorf("CreatedBy = %q, want %q", shown.CreatedBy, opts.Actor)
			}

			// Verify labels (includes gt:task and custom labels)
			for _, wantLabel := range opts.Labels {
				found := false
				for _, gotLabel := range shown.Labels {
					if gotLabel == wantLabel {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("Missing label %q in %v", wantLabel, shown.Labels)
				}
			}

			return nil
		},
	})
}
