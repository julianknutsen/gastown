# Reverse Rebase Plan

## Overview

This branch (`refactor/agents-clean`) contains a large architectural refactor. Traditional rebasing onto `upstream:main` causes excessive conflicts. Instead, we use a **reverse rebase** approach: cherry-pick each new upstream commit onto this branch, adapting the intent to fit the refactored architecture.

## Current Status

- **Base commit**: `9cd2696a` (last upstream commit incorporated into this branch)
- **Upstream HEAD**: `0db2bda6`
- **Commits to process**: 23

## Workflow

### 1. Identify New Upstream Commits

```bash
# Fetch latest from upstream
git fetch upstream

# Find commits on upstream:main that aren't in this branch
git log --oneline HEAD..upstream/main

# Or from the known base commit
git log --oneline 9cd2696a..upstream/main

# Get exact count
git rev-list --count 9cd2696a..upstream/main
```

Note: Both `HEAD..upstream/main` and `9cd2696a..upstream/main` should return the same commits since 9cd2696a is our sync point.

### 2. Process Each Commit (Oldest First)

For each commit, starting from the oldest:

```bash
# View the commit details
git show <commit-hash>

# Attempt cherry-pick
git cherry-pick <commit-hash>
```

### 3. Handle Each Commit

**If cherry-pick applies cleanly:**
- Review the changes to ensure they make sense in refactored context
- Run tests: `go test ./...`
- If tests pass, continue to next commit

**If cherry-pick has conflicts:**
1. Abort the cherry-pick: `git cherry-pick --abort`
2. Analyze the commit's **intent** (what problem does it solve?)
3. Manually apply the intent to the refactored code
4. Create a new commit with message: `<original message> (adapted from <hash>)`
5. Run tests to verify no regressions

**If commit is obsolete:**
- If the commit touches deleted code or is superseded by the refactor
- Document in commit log: `skip: <hash> - <reason>`
- Continue to next commit

### 4. Verification After Each Commit

```bash
# Run tests
go test ./...

# Run linter
golangci-lint run

# Build
go build ./...
```

## Critical Rules

**NEVER REMOVE FUNCTIONALITY.** When processing commits:

1. **All new features must exist** - If upstream adds a feature, our branch must have equivalent functionality. The implementation may differ (e.g., factory.Start() vs BuildStartupCommand) but the capability must be preserved.

2. **All new tests must pass** - If upstream adds tests, either:
   - Add them directly (if the tested function exists)
   - Add equivalent tests for our architecture (if function was refactored)
   - Document why the test is not applicable (only if the underlying feature doesn't apply)

3. **No regressions** - After processing each commit, verify:
   - `go build ./...` passes
   - `go test ./...` passes (or at least the affected packages)
   - The intent of the commit is preserved

4. **When in doubt, implement** - If a commit adds functionality and you're unsure how to adapt it, implement it in our architecture rather than skipping.

## Commit Categories

| Category | Action |
|----------|--------|
| **CLEAN** | Cherry-pick applies cleanly, tests pass |
| **ADAPT** | Conflicts - manually apply intent to refactored code |
| **SKIP** | Commit obsoleted by refactor (document why) |
| **DEFER** | Complex change requiring discussion |

## Tracking Progress

After processing commits, update this section:

### Last Synced
- **Upstream commit**: `a6102830`
- **Date**: `2026-01-20`
- **Commits processed**: `5/23`

### Commit Log

## Commit 1: 37f465bd - feat(hooks): add gt block-pr-workflow command
**Status**: CLEAN
**Files**: internal/cmd/block_pr_workflow.go (new)
**Commentary**: New file addition for CLI command. No conflicts with agent refactor.

## Commit 2: dcf7b810 - refactor(hooks): rename to gt tap guard pr-workflow
**Status**: CLEAN
**Files**: internal/cmd/block_pr_workflow.go (deleted), internal/cmd/tap.go, internal/cmd/tap_guard.go (new)
**Commentary**: Restructures previous commit. Pure file operations, no conflicts.

## Commit 3: b8eb9362 - fix(sling): prevent agent self-interruption during tests
**Status**: CLEAN (auto-merge)
**Files**: internal/cmd/sling_formula.go
**Commentary**: Small fix adding GT_TEST_NO_NUDGE check. Auto-merged cleanly.

## Commit 4: 544cacf3 - feat(roles): add config-based role definition system (Phase 1)
**Status**: CLEAN
**Files**: internal/config/roles.go, internal/config/roles/*.toml (new), internal/config/roles_test.go, internal/cmd/role.go
**Commentary**: All new files for role config infrastructure. No conflicts.

## Commit 5: a6102830 - feat(roles): switch daemon to config-based roles (Phase 2+3)
**Status**: ADAPT
**Files**: internal/daemon/lifecycle.go (conflicts), +15 others
**Commentary**: This commit switches daemon from role beads to config-based roles.
CONFLICT: Our refactor has `restartAgent()` using `factory.Start()` instead of
`restartSession()` with manual session setup. The upstream helpers (`getWorkDir`,
`getStartCommand`, `setSessionEnvironment`) are obsolete because `factory.Start()`
handles all that. RESOLUTION: Keep our refactored functions (`identityToAgentID`,
`restartAgent`, `isAgentRunning`, `stopAgent`). The upstream's intent (use
`LoadRoleDefinition()` for config) is already satisfied by our `getRoleConfigForIdentity()`
which uses `config.LoadRoleDefinition()` at line 282.

---

For each commit processed, log the outcome with commentary:

```
## Commit: <hash> - <short message>
**Status**: CLEAN | ADAPT | SKIP
**Files**: <list of files affected>
**Commentary**: <what the commit does, why it applied cleanly or had conflicts,
               how the intent was preserved in our refactored architecture>
```

### Skipped Commits
| Hash | Reason |
|------|--------|
| | |

### Adapted Commits
| Original Hash | New Hash | Notes |
|---------------|----------|-------|
| | | |

## Commands Reference

```bash
# See what's new on upstream
git log --oneline HEAD..upstream/main

# See what's new with file changes
git log --oneline --stat HEAD..upstream/main

# Cherry-pick with commit reference
git cherry-pick -x <hash>  # -x adds "cherry picked from" to message

# If conflict, see what files conflict
git status

# Abort failed cherry-pick
git cherry-pick --abort

# Continue after resolving conflicts
git cherry-pick --continue
```
