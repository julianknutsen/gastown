package doctor

import (
	"os"
)

// ShellBeadsDirCheck warns if BEADS_DIR is set in the shell environment.
// With cwd-based discovery, BEADS_DIR should not be set globally.
type ShellBeadsDirCheck struct {
	BaseCheck
}

// NewShellBeadsDirCheck creates a new shell BEADS_DIR check.
func NewShellBeadsDirCheck() *ShellBeadsDirCheck {
	return &ShellBeadsDirCheck{
		BaseCheck: BaseCheck{
			CheckName:        "shell-beads-dir",
			CheckDescription: "Warn if BEADS_DIR is set in shell environment",
			CheckCategory:    CategoryConfig,
		},
	}
}

// Run checks if BEADS_DIR is set in the environment.
func (c *ShellBeadsDirCheck) Run(ctx *CheckContext) *CheckResult {
	beadsDir := os.Getenv("BEADS_DIR")

	if beadsDir != "" {
		return &CheckResult{
			Name:    c.Name(),
			Status:  StatusWarning,
			Message: "BEADS_DIR is set in shell environment",
			Details: []string{
				"BEADS_DIR=" + beadsDir,
				"",
				"Gas Town now uses cwd-based beads discovery and no longer sets BEADS_DIR globally.",
				"Having BEADS_DIR set may cause unexpected behavior.",
			},
			FixHint: "Remove BEADS_DIR from your shell profile (~/.bashrc, ~/.zshrc, etc.) and restart your shell",
		}
	}

	return &CheckResult{
		Name:    c.Name(),
		Status:  StatusOK,
		Message: "BEADS_DIR is not set (using cwd-based discovery)",
	}
}
