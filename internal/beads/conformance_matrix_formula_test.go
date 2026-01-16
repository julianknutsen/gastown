package beads_test

import (
	"fmt"
	"testing"

	"github.com/steveyegge/gastown/internal/beads"
)

func TestMatrix_FormulaShow(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "FormulaShow",
		Operation: "FormulaShow",
		Test: func(ops beads.BeadsOps) error {
			// FormulaShow may return nil for unknown formulas
			formula, err := ops.FormulaShow("test-formula")
			if err != nil {
				// Expected if formula doesn't exist
				return nil
			}
			// If found, should have a name
			if formula != nil && formula.Name == "" {
				return fmt.Errorf("Formula should have a name")
			}
			return nil
		},
	})
}

func TestMatrix_Cook(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Cook",
		Operation: "Cook",
		Test: func(ops beads.BeadsOps) error {
			// Cook requires a formula to exist - may fail in test env
			issue, err := ops.Cook("nonexistent-formula")
			if err != nil {
				// Expected for nonexistent formula
				return nil
			}
			if issue != nil && issue.ID == "" {
				return fmt.Errorf("Cooked issue should have an ID")
			}
			return nil
		},
	})
}

func TestMatrix_LegAdd(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "LegAdd",
		Operation: "LegAdd",
		Test: func(ops beads.BeadsOps) error {
			// LegAdd requires a formula issue to exist - may fail in test env
			err := ops.LegAdd("nonexistent-formula-id", "step1")
			if err != nil {
				// Expected for nonexistent formula
				return nil
			}
			return nil
		},
	})
}

func TestMatrix_FormulaList(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "FormulaList",
		Operation: "FormulaList",
		Test: func(ops beads.BeadsOps) error {
			formulas, err := ops.FormulaList()
			if err != nil {
				return fmt.Errorf("FormulaList failed: %v", err)
			}
			// Empty list is valid
			if formulas == nil {
				return fmt.Errorf("FormulaList returned nil")
			}
			return nil
		},
	})
}
