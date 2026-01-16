package beads_test

import (
	"fmt"
	"testing"

	"github.com/steveyegge/gastown/internal/beads"
)

func TestMatrix_LabelAdd(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "LabelAdd",
		Operation: "LabelAdd",
		Test: func(ops beads.BeadsOps, targetID string) error {
			err := ops.LabelAdd(targetID, "test-label")
			if err != nil {
				return fmt.Errorf("LabelAdd(%s) failed: %v", targetID, err)
			}

			// Verify label added
			issue, err := ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show after LabelAdd failed: %v", err)
			}
			hasLabel := false
			for _, l := range issue.Labels {
				if l == "test-label" {
					hasLabel = true
					break
				}
			}
			if !hasLabel {
				return fmt.Errorf("issue should have test-label")
			}
			return nil
		},
	})
}

func TestMatrix_LabelRemove(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "LabelRemove",
		Operation: "LabelRemove",
		Test: func(ops beads.BeadsOps, targetID string) error {
			// First add a label
			err := ops.LabelAdd(targetID, "test-label")
			if err != nil {
				return fmt.Errorf("LabelAdd failed: %v", err)
			}

			// Verify label added
			issue, err := ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show after LabelAdd failed: %v", err)
			}
			hasLabel := false
			for _, l := range issue.Labels {
				if l == "test-label" {
					hasLabel = true
					break
				}
			}
			if !hasLabel {
				return fmt.Errorf("issue should have test-label after LabelAdd")
			}

			// Remove the label
			err = ops.LabelRemove(targetID, "test-label")
			if err != nil {
				return fmt.Errorf("LabelRemove(%s) failed: %v", targetID, err)
			}

			// Verify label removed
			issue, err = ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show after LabelRemove failed: %v", err)
			}
			for _, l := range issue.Labels {
				if l == "test-label" {
					return fmt.Errorf("issue should not have test-label after LabelRemove")
				}
			}
			return nil
		},
	})
}

func TestMatrix_AgentState(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "AgentState",
		Operation: "AgentState",
		Test: func(ops beads.BeadsOps, targetID string) error {
			// AgentState requires an agent bead (gt:agent label).
			// First add the gt:agent label to make it an agent bead.
			// Note: For cross-rig tests, this tests both LabelAdd and AgentState routing.
			err := ops.LabelAdd(targetID, "gt:agent")
			if err != nil {
				return fmt.Errorf("LabelAdd gt:agent to %s failed: %v", targetID, err)
			}

			// Set agent state to "working"
			err = ops.AgentState(targetID, "working")
			if err != nil {
				return fmt.Errorf("AgentState(%s) failed: %v", targetID, err)
			}

			// Verify state was set
			issue, err := ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show after AgentState failed: %v", err)
			}
			if issue.AgentState != "working" {
				return fmt.Errorf("AgentState = %q, want %q", issue.AgentState, "working")
			}
			return nil
		},
	})
}

func TestMatrix_Comment(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "Comment",
		Operation: "Comment",
		Test: func(ops beads.BeadsOps, targetID string) error {
			err := ops.Comment(targetID, "Test comment")
			if err != nil {
				return fmt.Errorf("Comment(%s) failed: %v", targetID, err)
			}
			return nil
		},
	})
}

func TestMatrix_UpdateAgentActiveMR(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "UpdateAgentActiveMR",
		Operation: "UpdateAgentActiveMR",
		Test: func(ops beads.BeadsOps, targetID string) error {
			// UpdateAgentActiveMR requires an agent bead (gt:agent label).
			// First add the gt:agent label to make it an agent bead.
			err := ops.LabelAdd(targetID, "gt:agent")
			if err != nil {
				return fmt.Errorf("LabelAdd gt:agent to %s failed: %v", targetID, err)
			}

			// Set active MR (stores in description field)
			err = ops.UpdateAgentActiveMR(targetID, "MR-123")
			if err != nil {
				return fmt.Errorf("UpdateAgentActiveMR(%s) failed: %v", targetID, err)
			}

			// Verify active_mr was set by parsing the description
			issue, err := ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show after UpdateAgentActiveMR failed: %v", err)
			}
			fields := beads.ParseAgentFields(issue.Description)
			if fields.ActiveMR != "MR-123" {
				return fmt.Errorf("ActiveMR = %q, want %q", fields.ActiveMR, "MR-123")
			}

			// Test clearing the active MR
			err = ops.UpdateAgentActiveMR(targetID, "")
			if err != nil {
				return fmt.Errorf("UpdateAgentActiveMR (clear) failed: %v", err)
			}
			issue, err = ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show after clear failed: %v", err)
			}
			fields = beads.ParseAgentFields(issue.Description)
			if fields.ActiveMR != "" {
				return fmt.Errorf("ActiveMR = %q after clear, want empty", fields.ActiveMR)
			}

			return nil
		},
	})
}
