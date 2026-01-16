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

// TestMatrix_SlotSet tests setting a slot on an agent bead.
// NOTE: bd slot commands require issue_type="agent", but CreateOptions.Type is
// converted to a gt:<type> label by both Implementation and Double. This test
// will fail until Implementation.Create is fixed to use --type= flag.
func TestMatrix_SlotSet(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "SlotSet",
		Operation: "SlotSet",
		Test: func(ops beads.BeadsOps) error {
			// Create an agent issue
			agent, err := ops.Create(beads.CreateOptions{
				Title: "Test Agent",
				Type:  "agent",
			})
			if err != nil {
				return fmt.Errorf("Create agent failed: %v", err)
			}

			// Create a task to attach to the hook
			task, err := ops.Create(beads.CreateOptions{
				Title: "Test Task",
				Type:  "task",
			})
			if err != nil {
				return fmt.Errorf("Create task failed: %v", err)
			}

			// Set the hook slot
			err = ops.SlotSet(agent.ID, "hook", task.ID)
			if err != nil {
				return fmt.Errorf("SlotSet(%s, hook, %s) failed: %v", agent.ID, task.ID, err)
			}

			// Verify by showing the slot
			slot, err := ops.SlotShow(agent.ID)
			if err != nil {
				return fmt.Errorf("SlotShow after SlotSet failed: %v", err)
			}
			if slot == nil {
				return fmt.Errorf("SlotShow returned nil")
			}
			if slot.IssueID != task.ID {
				return fmt.Errorf("SlotShow.IssueID=%q, want %q", slot.IssueID, task.ID)
			}
			return nil
		},
	})
}

// TestMatrix_SlotClear tests clearing a slot on an agent bead.
// NOTE: See SlotSet for why this test fails.
func TestMatrix_SlotClear(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "SlotClear",
		Operation: "SlotClear",
		Test: func(ops beads.BeadsOps) error {
			// Create an agent issue
			agent, err := ops.Create(beads.CreateOptions{
				Title: "Test Agent for Clear",
				Type:  "agent",
			})
			if err != nil {
				return fmt.Errorf("Create agent failed: %v", err)
			}

			// Create a task to attach to the hook
			task, err := ops.Create(beads.CreateOptions{
				Title: "Test Task for Clear",
				Type:  "task",
			})
			if err != nil {
				return fmt.Errorf("Create task failed: %v", err)
			}

			// Set the hook slot
			err = ops.SlotSet(agent.ID, "hook", task.ID)
			if err != nil {
				return fmt.Errorf("SlotSet failed: %v", err)
			}

			// Clear the hook slot
			err = ops.SlotClear(agent.ID, "hook")
			if err != nil {
				return fmt.Errorf("SlotClear(%s, hook) failed: %v", agent.ID, err)
			}

			// Verify by showing - IssueID should be empty
			slot, err := ops.SlotShow(agent.ID)
			if err != nil {
				return fmt.Errorf("SlotShow after SlotClear failed: %v", err)
			}
			if slot == nil {
				return fmt.Errorf("SlotShow returned nil")
			}
			if slot.IssueID != "" {
				return fmt.Errorf("SlotShow.IssueID=%q after clear, want empty", slot.IssueID)
			}
			return nil
		},
	})
}

// TestMatrix_SlotShow_WithSlots tests showing slot information with actual slots.
// This extends the basic SlotShow test in conformance_matrix_misc_test.go
// NOTE: See SlotSet for why this test fails.
func TestMatrix_SlotShow_WithSlots(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "SlotShow_WithSlots",
		Operation: "SlotShow",
		Test: func(ops beads.BeadsOps) error {
			// Create an agent issue
			agent, err := ops.Create(beads.CreateOptions{
				Title: "Test Agent for Show",
				Type:  "agent",
			})
			if err != nil {
				return fmt.Errorf("Create agent failed: %v", err)
			}

			// Show the slot (should have empty IssueID initially)
			slot, err := ops.SlotShow(agent.ID)
			if err != nil {
				return fmt.Errorf("SlotShow(%s) failed: %v", agent.ID, err)
			}
			if slot == nil {
				return fmt.Errorf("SlotShow returned nil")
			}
			if slot.ID != agent.ID {
				return fmt.Errorf("SlotShow.ID=%q, want %q", slot.ID, agent.ID)
			}
			return nil
		},
	})
}
