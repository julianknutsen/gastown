# fix: Correct bootstrap file placement and settings.local.json migration

## Summary

This PR fixes critical issues with Claude Code configuration file placement that caused hooks not to fire and agents to miss project instructions:

1. **settings.json → settings.local.json**: Claude Code does NOT traverse parent directories for settings files (only for CLAUDE.md). Settings must be placed directly in each agent's working directory. We now use `settings.local.json` which is gitignored by default, preventing pollution of user repos.

2. **Bootstrap file placement**: CLAUDE.md and AGENTS.md are now placed at agent directory level (e.g., `<rig>/refinery/CLAUDE.md`) where Claude can find them via upward traversal, rather than inside git worktrees where they would pollute source repos.

3. **Documentation fixes**: Updated stale references to polecat path structure (`polecats/<name>/<rigname>/`) and other directory layouts to match actual code.

## Fixed Issues

- **Hooks not firing for polecats/crew**: Settings placed in parent directories were never found because Claude Code doesn't traverse upward for settings. Now each agent provisions `settings.local.json` in their working directory at startup.

- **Source repo pollution**: Writing to CLAUDE.md or AGENTS.md inside git worktrees would leak Gas Town internals if merged. Bootstrap files are now outside worktrees.

## Related Issues (Not Fixed)

- **#392 - Polecats/crew exclude essential instructions**: Project-specific CLAUDE.md is still excluded by sparse checkout. This is intentional (prevents Gas Town context from conflicting with project instructions), but means agents don't see project-specific instructions. A future solution might merge project instructions into `gt prime` output.

## Breaking Changes

| Before | After |
|--------|-------|
| `<rig>/polecats/.claude/settings.json` | `<rig>/polecats/<name>/<rigname>/.claude/settings.local.json` |
| `<rig>/crew/.claude/settings.json` | `<rig>/crew/<name>/.claude/settings.local.json` |
| `<rig>/refinery/.claude/settings.json` | `<rig>/refinery/rig/.claude/settings.local.json` |
| `<rig>/witness/.claude/settings.json` | `<rig>/witness/.claude/settings.local.json` |
| (no CLAUDE.md at agent level) | `<rig>/refinery/CLAUDE.md`, `<rig>/witness/CLAUDE.md`, etc. |

**Migration**: Run `gt doctor --fix` to update existing installations.

## Expected Impact

### Positive
- Hooks now fire correctly for all agent types
- User repos stay clean (no Gas Town files committed)
- Multi-provider support via AGENTS.md (Codex compatibility)
- Bootstrap files placed correctly for Claude's upward traversal

### Risk Assessment
- **Low risk**: Changes are backwards-compatible for new installations
- **Medium risk for existing installs**: Requires `gt doctor --fix` to migrate
- Old settings files in parent directories will be ignored (harmless but unused)

## Changes by Category

### Core Logic
- `internal/claude/settings.go`: Use `settings.local.json` instead of `settings.json`
- `internal/rig/manager.go`: Install CLAUDE.md/AGENTS.md at agent directory level, remove settings installation from parent dirs
- `internal/polecat/manager.go`: Add `EnsureSettingsForRole` call, remove AGENTS.md copy blocks
- `internal/agent/bootstrap.go`: New file - centralized bootstrap pointer generation

### Doctor Checks
- `internal/doctor/claude_settings_check.go`: Check for settings.local.json in working directories
- `internal/doctor/priming_check.go`: Verify CLAUDE.md exists at agent level
- `internal/doctor/config_check.go`: Updated paths

### Documentation
- `docs/reference.md`: Complete rewrite of Settings and CLAUDE.md sections
- `docs/design/architecture.md`: Added Settings Placement section with clear table
- `README.md`: Updated to reference settings.local.json

### Tests
- `internal/cmd/rig_integration_test.go`: Added `TestAgentWorktreesStayClean` - verifies no Gas Town files pollute user repos
- `internal/doctor/priming_check_test.go`: Comprehensive tests for bootstrap file placement
- `internal/polecat/manager_test.go`: Fixed NewManager signature, added cleanliness tests

### Cleanup
- Deleted `templates/polecat-CLAUDE.md` (orphaned file, not used by any code)

## Directory Structure After This PR

```
~/gt/
├── mayor/                      # Town-level mayor
│   ├── CLAUDE.md               # Bootstrap pointer
│   ├── AGENTS.md               # Multi-provider support
│   └── .claude/settings.local.json
├── deacon/                     # Background supervisor
│   └── .claude/settings.local.json
└── <rig>/                      # Project container
    ├── witness/                # Witness agent home
    │   ├── CLAUDE.md           # Bootstrap pointer (found via upward traversal)
    │   ├── AGENTS.md
    │   └── .claude/settings.local.json
    ├── refinery/               # Refinery parent
    │   ├── CLAUDE.md           # Bootstrap pointer
    │   ├── AGENTS.md
    │   └── rig/                # Worktree (source repo)
    │       └── .claude/settings.local.json
    ├── crew/                   # Crew parent
    │   ├── CLAUDE.md           # Shared by all crew via traversal
    │   ├── AGENTS.md
    │   └── <name>/             # Crew worktree
    │       └── .claude/settings.local.json
    └── polecats/               # Polecat parent
        ├── CLAUDE.md           # Shared by all polecats via traversal
        ├── AGENTS.md
        └── <name>/<rigname>/   # Polecat worktree (new structure)
            └── .claude/settings.local.json
```

## Test Plan

- [x] `go build ./...` passes
- [x] `go test ./...` passes
- [x] `TestAgentWorktreesStayClean` verifies no Gas Town files in user repos
- [x] `TestRigAddCreatesCorrectStructure` verifies CLAUDE.md/AGENTS.md placement
- [ ] Manual test: Fresh `gt install` + `gt rig add` creates correct structure
- [ ] Manual test: `gt doctor --fix` migrates existing installation
- [ ] Manual test: Polecat hooks fire correctly after spawn

## Related Links

- Claude Code settings behavior: Parent directory traversal only works for CLAUDE.md
- Issue #392: https://github.com/steveyegge/gastown/issues/392

---

Generated with [Claude Code](https://claude.ai/code)
