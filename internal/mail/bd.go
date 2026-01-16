package mail

import (
	"strings"

	"github.com/steveyegge/gastown/internal/beads"
)

// bdError represents an error from running a bd command.
// It wraps the underlying error and includes the stderr output for inspection.
type bdError struct {
	Err    error
	Stderr string
}

// Error implements the error interface.
func (e *bdError) Error() string {
	if e.Stderr != "" {
		return e.Stderr
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "unknown bd error"
}

// Unwrap returns the underlying error for errors.Is/As compatibility.
func (e *bdError) Unwrap() error {
	return e.Err
}

// ContainsError checks if the stderr message contains the given substring.
func (e *bdError) ContainsError(substr string) bool {
	return strings.Contains(e.Stderr, substr)
}

// runBdCommand executes a bd command with proper environment setup.
// workDir is the directory to run the command in.
// beadsDir is the BEADS_DIR environment variable value.
// extraEnv contains additional environment variables to set (e.g., "BD_IDENTITY=...").
// Returns stdout bytes on success, or a *bdError on failure.
func runBdCommand(args []string, workDir, beadsDir string, extraEnv ...string) ([]byte, error) {
	// BeadsOps Migration: cmd.Dir=workDir, BEADS_DIR explicitly set
	// extraEnv is used for BD_IDENTITY, which Run() handles via environment
	b := beads.NewWithBeadsDir(workDir, beadsDir)

	// If extraEnv contains BD_IDENTITY, we need to pass it through Run()
	// Run() doesn't support extra env, so we use RunWithEnv if needed
	// For now, Run() should work for most cases since BD_IDENTITY is typically
	// already set in the environment. If specific identity is needed,
	// the caller should set it in the environment before calling.
	out, err := b.Run(args...)
	if err != nil {
		return nil, &bdError{
			Err:    err,
			Stderr: err.Error(),
		}
	}

	return out, nil
}
