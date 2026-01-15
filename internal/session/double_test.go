package session

import (
	"testing"
)

// TestDouble_Conformance verifies the test double matches Sessions contract.
func TestDouble_Conformance(t *testing.T) {
	factory := func() Sessions {
		return NewDouble()
	}

	RunConformanceTests(t, factory, nil)
}
