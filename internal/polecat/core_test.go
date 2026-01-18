package polecat

import (
	"os"
	"strings"
	"testing"
)

func TestCoreFileHasTimestampComment(t *testing.T) {
	// Read the core.go file
	content, err := os.ReadFile("core.go")
	if err != nil {
		t.Fatalf("Failed to read core.go: %v", err)
	}

	// Verify the timestamp comment is present
	if !strings.Contains(string(content), "// Remote polecat test:") {
		t.Error("core.go should contain the timestamp comment '// Remote polecat test:'")
	}
}
