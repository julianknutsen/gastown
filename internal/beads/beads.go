// Package beads provides a wrapper for the bd (beads) CLI.
//
// This file provides backward-compatibility type aliases.
// See interface.go for the BeadsOps interface and implementation.go
// for the real bd CLI wrapper.
package beads

import (
	"fmt"
	"os"
	"path/filepath"
)

// Beads is a backward-compatibility alias for Implementation.
// New code should use BeadsOps interface or Implementation directly.
type Beads = Implementation

// primeContent is the Gas Town PRIME.md content that provides essential context
// for crew workers. This is the fallback if the SessionStart hook fails.
const primeContent = `# Gas Town Worker Context

> **Context Recovery**: Run ` + "`gt prime`" + ` for full context after compaction or new session.

## The Propulsion Principle (GUPP)

**If you find work on your hook, YOU RUN IT.**

No confirmation. No waiting. No announcements. The hook having work IS the assignment.
This is physics, not politeness. Gas Town is a steam engine - you are a piston.

**Failure mode we're preventing:**
- Agent starts with work on hook
- Agent announces itself and waits for human to say "ok go"
- Human is AFK / trusting the engine to run
- Work sits idle. The whole system stalls.

## Startup Protocol

1. Check your hook: ` + "`gt mol status`" + `
2. If work is hooked → EXECUTE (no announcement, no waiting)
3. If hook empty → Check mail: ` + "`gt mail inbox`" + `
4. Still nothing? Wait for user instructions

## Key Commands

- ` + "`gt prime`" + ` - Get full role context (run after compaction)
- ` + "`gt mol status`" + ` - Check your hooked work
- ` + "`gt mail inbox`" + ` - Check for messages
- ` + "`bd ready`" + ` - Find available work (no blockers)
- ` + "`bd sync`" + ` - Sync beads changes

## Session Close Protocol

Before signaling completion:
1. git status (check what changed)
2. git add <files> (stage code changes)
3. bd sync (commit beads changes)
4. git commit -m "..." (commit code)
5. bd sync (commit any new beads changes)
6. git push (push to remote)
7. ` + "`gt done`" + ` (submit to merge queue and exit)

**Polecats MUST call ` + "`gt done`" + ` - this submits work and exits the session.**
`

// ProvisionPrimeMD writes the Gas Town PRIME.md file to the specified beads directory.
// This provides essential Gas Town context (GUPP, startup protocol) as a fallback
// if the SessionStart hook fails. The PRIME.md is read by bd prime.
//
// The beadsDir should be the actual beads directory (after following any redirect).
// Returns nil if PRIME.md already exists (idempotent).
func ProvisionPrimeMD(beadsDir string) error {
	primePath := filepath.Join(beadsDir, "PRIME.md")

	// Check if already exists - don't overwrite customizations
	if _, err := os.Stat(primePath); err == nil {
		return nil // Already exists, don't overwrite
	}

	// Create .beads directory if it doesn't exist
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		return fmt.Errorf("creating beads dir: %w", err)
	}

	// Write PRIME.md
	if err := os.WriteFile(primePath, []byte(primeContent), 0644); err != nil {
		return fmt.Errorf("writing PRIME.md: %w", err)
	}

	return nil
}

// ProvisionPrimeMDForWorktree provisions PRIME.md for a worktree by following its redirect.
// This is the main entry point for crew/polecat provisioning.
func ProvisionPrimeMDForWorktree(worktreePath string) error {
	// Resolve the beads directory (follows redirect chain)
	beadsDir := ResolveBeadsDir(worktreePath)

	// Provision PRIME.md in the target directory
	return ProvisionPrimeMD(beadsDir)
}
