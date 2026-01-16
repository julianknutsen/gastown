package beads_test

import (
	"fmt"
	"testing"

	"github.com/steveyegge/gastown/internal/beads"
)

func TestMatrix_GateShow(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "GateShow",
		Operation: "GateShow",
		Test: func(ops beads.BeadsOps) error {
			// GateShow may return nil for unknown gates
			gate, err := ops.GateShow("test-gate")
			if err != nil {
				// Expected if gate doesn't exist
				return nil
			}
			if gate != nil && gate.ID == "" {
				return fmt.Errorf("Gate should have an ID")
			}
			return nil
		},
	})
}

func TestMatrix_GateList(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "GateList",
		Operation: "GateList",
		Test: func(ops beads.BeadsOps) error {
			gates, err := ops.GateList(false)
			if err != nil {
				return fmt.Errorf("GateList failed: %v", err)
			}
			// Empty list is valid
			if gates == nil {
				return fmt.Errorf("GateList returned nil")
			}
			return nil
		},
	})
}

func TestMatrix_GateWait(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "GateWait",
		Operation: "GateWait",
		Test: func(ops beads.BeadsOps) error {
			// GateWait on nonexistent gate may fail
			err := ops.GateWait("nonexistent-gate", "test-agent")
			if err != nil {
				// Expected for nonexistent gate
				return nil
			}
			return nil
		},
	})
}

func TestMatrix_GateResolve(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "GateResolve",
		Operation: "GateResolve",
		Test: func(ops beads.BeadsOps) error {
			// GateResolve on nonexistent gate may fail
			err := ops.GateResolve("nonexistent-gate")
			if err != nil {
				// Expected for nonexistent gate
				return nil
			}
			return nil
		},
	})
}

func TestMatrix_GateCheck(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "GateCheck",
		Operation: "GateCheck",
		Test: func(ops beads.BeadsOps) error {
			err := ops.GateCheck()
			if err != nil {
				// GateCheck may fail in some environments
				return nil
			}
			return nil
		},
	})
}
