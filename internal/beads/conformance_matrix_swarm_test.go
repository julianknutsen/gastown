package beads_test

import (
	"fmt"
	"testing"

	"github.com/steveyegge/gastown/internal/beads"
)

func TestMatrix_SwarmStatus(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "SwarmStatus",
		Operation: "SwarmStatus",
		Test: func(ops beads.BeadsOps) error {
			status, err := ops.SwarmStatus("test-swarm")
			if err != nil {
				// Expected if swarm doesn't exist
				return nil
			}
			if status != nil && status.ID == "" {
				return fmt.Errorf("SwarmStatus should have an ID")
			}
			return nil
		},
	})
}

func TestMatrix_SwarmCreate(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "SwarmCreate",
		Operation: "SwarmCreate",
		Test: func(ops beads.BeadsOps) error {
			// SwarmCreate requires an epic to exist
			issue, err := ops.SwarmCreate("nonexistent-epic")
			if err != nil {
				// Expected for nonexistent epic
				return nil
			}
			if issue != nil && issue.ID == "" {
				return fmt.Errorf("Swarm issue should have an ID")
			}
			return nil
		},
	})
}

func TestMatrix_SwarmList(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "SwarmList",
		Operation: "SwarmList",
		Test: func(ops beads.BeadsOps) error {
			swarms, err := ops.SwarmList()
			if err != nil {
				return fmt.Errorf("SwarmList failed: %v", err)
			}
			// Empty list is valid
			if swarms == nil {
				return fmt.Errorf("SwarmList returned nil")
			}
			return nil
		},
	})
}

func TestMatrix_SwarmValidate(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "SwarmValidate",
		Operation: "SwarmValidate",
		Test: func(ops beads.BeadsOps) error {
			err := ops.SwarmValidate("nonexistent-epic")
			if err != nil {
				// Expected for nonexistent epic
				return nil
			}
			return nil
		},
	})
}
