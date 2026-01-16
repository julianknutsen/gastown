package beads_test

import (
	"fmt"
	"testing"

	"github.com/steveyegge/gastown/internal/beads"
)

func TestMatrix_Show(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "Show",
		Operation: "Show",
		Test: func(ops beads.BeadsOps, targetID string) error {
			issue, err := ops.Show(targetID)
			if err != nil {
				return fmt.Errorf("Show(%s) failed: %v", targetID, err)
			}
			if issue.ID != targetID {
				return fmt.Errorf("Show ID = %q, want %q", issue.ID, targetID)
			}
			return nil
		},
	})
}

func TestMatrix_ShowMultiple(t *testing.T) {
	RunConformanceTest(t, ConformanceTest{
		Name:      "ShowMultiple",
		Operation: "ShowMultiple",
		Test: func(ops beads.BeadsOps, targetID string) error {
			issues, err := ops.ShowMultiple([]string{targetID})
			if err != nil {
				return fmt.Errorf("ShowMultiple([%s]) failed: %v", targetID, err)
			}
			if len(issues) != 1 {
				return fmt.Errorf("ShowMultiple returned %d issues, want 1", len(issues))
			}
			issue, ok := issues[targetID]
			if !ok {
				return fmt.Errorf("ShowMultiple missing issue %s", targetID)
			}
			if issue.ID != targetID {
				return fmt.Errorf("ShowMultiple ID = %q, want %q", issue.ID, targetID)
			}
			return nil
		},
	})
}

func TestMatrix_ShowNotFound(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "ShowNotFound",
		Operation: "Show",
		Test: func(ops beads.BeadsOps) error {
			_, err := ops.Show("nonexistent-id-xyz")
			if err != beads.ErrNotFound {
				return fmt.Errorf("Show(nonexistent) error = %v, want ErrNotFound", err)
			}
			return nil
		},
	})
}
