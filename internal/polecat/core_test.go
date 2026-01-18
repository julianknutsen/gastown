package polecat

import "testing"

func TestCoreVersion(t *testing.T) {
	if CoreVersion == "" {
		t.Error("CoreVersion should not be empty")
	}
}

func TestCoreVersion_Value(t *testing.T) {
	expected := "1.0.0"
	if CoreVersion != expected {
		t.Errorf("CoreVersion = %q, want %q", CoreVersion, expected)
	}
}
