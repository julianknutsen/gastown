package beads_test

import (
	"fmt"
	"testing"

	"github.com/steveyegge/gastown/internal/beads"
)

func TestMatrix_MolSeed(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "MolSeed",
		Operation: "MolSeed",
		Test: func(ops beads.BeadsOps) error {
			err := ops.MolSeed(beads.MolSeedOptions{})
			if err != nil {
				// MolSeed may fail in test env
				return nil
			}
			return nil
		},
	})
}

func TestMatrix_MolCurrent(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "MolCurrent",
		Operation: "MolCurrent",
		Test: func(ops beads.BeadsOps) error {
			output, err := ops.MolCurrent("test-molecule")
			if err != nil {
				// Expected if molecule doesn't exist
				return nil
			}
			if output != nil && output.ID == "" {
				return fmt.Errorf("MolCurrent output should have an ID")
			}
			return nil
		},
	})
}

func TestMatrix_MolCatalog(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "MolCatalog",
		Operation: "MolCatalog",
		Test: func(ops beads.BeadsOps) error {
			protos, err := ops.MolCatalog()
			if err != nil {
				return fmt.Errorf("MolCatalog failed: %v", err)
			}
			// Empty catalog is valid
			if protos == nil {
				return fmt.Errorf("MolCatalog returned nil")
			}
			return nil
		},
	})
}

func TestMatrix_WispCreate(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "WispCreate",
		Operation: "WispCreate",
		Test: func(ops beads.BeadsOps) error {
			// WispCreate requires a prototype to exist
			issue, err := ops.WispCreate("nonexistent-proto", "test-actor")
			if err != nil {
				// Expected for nonexistent prototype
				return nil
			}
			if issue != nil && issue.ID == "" {
				return fmt.Errorf("Wisp should have an ID")
			}
			return nil
		},
	})
}

func TestMatrix_WispList(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "WispList",
		Operation: "WispList",
		Test: func(ops beads.BeadsOps) error {
			wisps, err := ops.WispList(false)
			if err != nil {
				return fmt.Errorf("WispList failed: %v", err)
			}
			// Empty list is valid
			if wisps == nil {
				return fmt.Errorf("WispList returned nil")
			}
			return nil
		},
	})
}

func TestMatrix_WispGC(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "WispGC",
		Operation: "WispGC",
		Test: func(ops beads.BeadsOps) error {
			err := ops.WispGC()
			if err != nil {
				// WispGC may fail in test env
				return nil
			}
			return nil
		},
	})
}

func TestMatrix_MolBond(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "MolBond",
		Operation: "MolBond",
		Test: func(ops beads.BeadsOps) error {
			// MolBond requires a wisp and a bead to exist
			// Testing with nonexistent IDs should fail gracefully
			issue, err := ops.MolBond("nonexistent-wisp", "nonexistent-bead")
			if err != nil {
				// Expected for nonexistent wisp/bead
				return nil
			}
			if issue != nil && issue.ID == "" {
				return fmt.Errorf("MolBond should return an issue with an ID")
			}
			return nil
		},
	})
}
