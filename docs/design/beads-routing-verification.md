# BeadsOps Routing Verification

This document verifies that each BeadsOps call site correctly targets the intended database (town-level vs rig-level).

## How the Implementation Works

```go
func (b *Implementation) run(args ...string) ([]byte, error) {
    cmd.Dir = b.workDir
    beadsDir := b.beadsDir
    if beadsDir == "" {
        beadsDir = ResolveBeadsDir(b.workDir)  // Follows .beads/redirect
    }
    cmd.Env = append(os.Environ(), "BEADS_DIR="+beadsDir)
    // ...
}
```

**Key behaviors:**
1. `beads.New(workDir)` → `ResolveBeadsDir(workDir)` called at run time
2. `NewWithBeadsDir(workDir, beadsDir)` → Uses explicit beadsDir, NO redirect resolution
3. `ResolveBeadsDir()` follows `.beads/redirect` files (used by polecats/crew)

## Verification Matrix

### Town-Level Operations

These operations need to target `townRoot/.beads` for town-wide data (agents, convoys, mail, roles).

| File:Line | Original cmd.Dir | Original BEADS_DIR | Current Implementation | Operation | Verification |
|-----------|-----------------|-------------------|----------------------|-----------|--------------|
| deacon/stale_hooks.go:125 | townRoot | N/A | `beads.New(townRoot)` | List hooked issues | ✅ townRoot/.beads |
| deacon/stale_hooks.go:202 | townRoot | N/A | `beads.New(townRoot)` | Update issue status | ✅ townRoot/.beads |
| doctor/config_check.go:585 | ctx.TownRoot | N/A | `beads.New(ctx.TownRoot)` | ConfigGet | ✅ townRoot/.beads |
| doctor/config_check.go:661 | c.townRoot | N/A | `beads.New(c.townRoot)` | ConfigSet | ✅ townRoot/.beads |
| doctor/bd_daemon_check.go:32 | ctx.TownRoot | N/A | `beads.New(ctx.TownRoot)` | DaemonStatus | ✅ townRoot/.beads |
| doctor/bd_daemon_check.go:78 | ctx.TownRoot | N/A | `beads.New(ctx.TownRoot)` | DaemonStart | ✅ townRoot/.beads |
| doctor/bd_daemon_check.go:175 | ctx.TownRoot | N/A | `beads.New(ctx.TownRoot)` | Doctor | ✅ townRoot/.beads |
| doctor/role_beads_check.go:94 | ctx.TownRoot | N/A | `beads.New(ctx.TownRoot)` | Create role beads | ✅ townRoot/.beads |
| doctor/beads_check.go:131 | ctx.TownRoot | N/A | `beads.New(ctx.TownRoot)` | SyncFromMain | ✅ townRoot/.beads |
| doctor/beads_check.go:451 | townRoot | N/A | `beads.New(townRoot)` | LabelAdd | ✅ townRoot/.beads |
| daemon/lifecycle.go:653 | d.config.TownRoot | N/A | `beads.New(d.config.TownRoot)` | Show agent bead | ✅ townRoot/.beads |
| daemon/lifecycle.go:771 | d.config.TownRoot | N/A | `beads.New(d.config.TownRoot)` | List agents | ✅ townRoot/.beads |
| daemon/lifecycle.go:855 | d.config.TownRoot | N/A | `beads.New(d.config.TownRoot)` | List agents | ✅ townRoot/.beads |
| cmd/handoff.go:590 | townRoot | SUPERFLUOUS | `beads.New(townRoot)` | Create mail | ✅ townRoot/.beads |
| cmd/formula.go:412 | townBeads | N/A | `beads.New(townBeads)` | Create convoy | ✅ townRoot/.beads |
| web/fetcher.go:39 | f.townBeads | N/A | `beads.New(filepath.Dir(f.townBeads))` | List convoys | ✅ townRoot/.beads |
| web/fetcher.go:239 | townBeads parent | N/A | `beads.New(filepath.Dir(f.townBeads))` | ShowMultiple | ✅ townRoot/.beads |

**All verified correct.**

### Rig-Level Operations

These operations need to target `rigPath/.beads` for rig-specific data (swarms, epics, patrol molecules).

| File:Line | Original cmd.Dir | Original BEADS_DIR | Current Implementation | Operation | Verification |
|-----------|-----------------|-------------------|----------------------|-----------|--------------|
| swarm/manager.go:44 | m.beadsDir | N/A | `beads.New(m.beadsDir)` | Show epic | ✅ rig/.beads |
| swarm/manager.go:114 | m.beadsDir | N/A | `beads.New(m.beadsDir)` | SwarmStatus | ✅ rig/.beads |
| swarm/manager.go:138 | m.beadsDir | N/A | `beads.New(m.beadsDir)` | SwarmStatus | ✅ rig/.beads |
| swarm/manager.go:176 | m.beadsDir | N/A | `beads.New(m.beadsDir)` | Show epic | ✅ rig/.beads |
| doctor/patrol_check.go:94 | rigPath | N/A | `beads.New(rigPath)` | List molecules | ✅ rig/.beads |
| doctor/patrol_check.go:122 | rigPath | N/A | `beads.New(rigPath)` | MolSeed | ✅ rig/.beads |
| doctor/rig_check.go:836 | c.rigPath | N/A | `beads.New(c.rigPath)` | Stats | ✅ rig/.beads |
| doctor/rig_check.go:885 | c.rigPath | N/A | `beads.New(c.rigPath)` | Sync | ✅ rig/.beads |
| doctor/rig_check.go:1049 | rigPath | N/A | `beads.New(rigPath)` | Init | ✅ rig/.beads |
| doctor/beads_check.go:152 | ctx.RigPath() | N/A | `beads.New(ctx.RigPath())` | SyncFromMain | ✅ rig/.beads |
| rig/manager.go:374 | mayorRigPath | N/A | `beads.New(mayorRigPath)` | Init mayor beads | ✅ mayorRig/.beads |
| rig/manager.go:1030 | rigPath | N/A | `beads.New(rigPath)` | MolSeed | ✅ rig/.beads |
| rig/manager.go:1042 | rigPath | N/A | `beads.New(rigPath)` | MolSeed | ✅ rig/.beads |
| cmd/swarm.go:238 | r.BeadsPath() | N/A | `beads.New(beadsPath)` | Show swarm epic | ✅ rig/.beads |
| cmd/swarm.go:307+ | r.BeadsPath() | N/A | `beads.New(r.BeadsPath())` | Swarm ops | ✅ rig/.beads |

**All verified correct.** Note: `m.beadsDir` and `r.BeadsPath()` return the rig path, not the .beads path.

### Polecat/Crew Operations (with redirect)

These operations run from polecat/crew worktrees that have `.beads/redirect` pointing to shared rig beads.

| File:Line | Original cmd.Dir | Original BEADS_DIR | Current Implementation | Operation | Verification |
|-----------|-----------------|-------------------|----------------------|-----------|--------------|
| daemon/lifecycle.go:603 | workDir | N/A | `beads.New(workDir)` | Sync | ✅ Follows redirect |
| crew/manager.go:413 | dir | N/A | `beads.New(dir)` | Sync | ✅ Follows redirect |
| polecat/session_manager.go:282 | workDir | N/A | `beads.New(workDir)` | Sync | ✅ Follows redirect |
| polecat/session_manager.go:455 | workDir | N/A | `beads.New(workDir)` | Update issue | ✅ Follows redirect |

**All verified correct.** `ResolveBeadsDir(workDir)` follows the redirect to the shared rig beads.

### CWD/Generic Operations

These operations use current working directory and let prefix routing handle cross-rig lookups.

| File:Line | Original cmd.Dir | Original BEADS_DIR | Current Implementation | Operation | Verification |
|-----------|-----------------|-------------------|----------------------|-----------|--------------|
| cmd/handoff.go:660 | N/A (cwd) | N/A | `beads.New(cwd)` | Ready check | ✅ Uses nearest .beads |
| cmd/handoff.go:720 | N/A (cwd) | N/A | `beads.New(cwd)` | Ready check | ✅ Uses nearest .beads |
| doctor/repo_fingerprint_check.go:71 | workDir | N/A | `beads.New(workDir)` | Doctor | ✅ Uses workDir/.beads |
| doctor/repo_fingerprint_check.go:144 | parent of .beads | N/A | `beads.New(workDir)` | Migrate | ✅ Uses workDir/.beads |

**All verified correct.**

### Special Case: Init New .beads (Must Prevent Parent Discovery)

| File:Line | Original cmd.Dir | Original BEADS_DIR | Current Implementation | Operation | Verification |
|-----------|-----------------|-------------------|----------------------|-----------|--------------|
| rig/manager.go:594 | rigPath | beadsDir | `NewWithBeadsDir(rigPath, beadsDir)` | Init NEW .beads | ✅ REQUIRED |

**This is the ONLY legitimate use of NewWithBeadsDir.**

When initializing a NEW .beads directory, we MUST prevent bd from walking up and finding a parent .beads.
Without explicit BEADS_DIR, bd would find the town's .beads and corrupt it.

### All NewWithBeadsDir Usages - Comprehensive Analysis

#### Pattern A: `NewWithBeadsDir(X, ResolveBeadsDir(X))` - REDUNDANT

These call ResolveBeadsDir explicitly, then pass it to NewWithBeadsDir. This is equivalent to `beads.New(X)`.

| File:Line | Current Code | Simplification |
|-----------|-------------|----------------|
| deacon/stuck.go:47 | `NewWithBeadsDir(townRoot, ResolveBeadsDir(townRoot))` | `beads.New(townRoot)` |
| witness/manager.go:244 | `NewWithBeadsDir(townRoot, ResolveBeadsDir(townRoot))` | `beads.New(townRoot)` |
| doctor/patrol_check.go:247 | `NewWithBeadsDir(townRoot, ResolveBeadsDir(townRoot))` | `beads.New(townRoot)` |
| cmd/mail_queue.go:463 | `NewWithBeadsDir(townRoot, ResolveBeadsDir(townRoot))` | `beads.New(townRoot)` |
| cmd/mail_queue.go:510 | `NewWithBeadsDir(townRoot, ResolveBeadsDir(townRoot))` | `beads.New(townRoot)` |
| cmd/mail_queue.go:572 | `NewWithBeadsDir(townRoot, ResolveBeadsDir(townRoot))` | `beads.New(townRoot)` |
| cmd/mail_queue.go:626 | `NewWithBeadsDir(townRoot, ResolveBeadsDir(townRoot))` | `beads.New(townRoot)` |
| rig/manager.go:653 | `NewWithBeadsDir(rigPath, ResolveBeadsDir(rigPath))` | `beads.New(rigPath)` |
| polecat/manager.go:85 | `NewWithBeadsDir(beadsPath, ResolveBeadsDir(beadsPath))` | `beads.New(beadsPath)` |

**Count: 9 redundant usages**

#### Pattern B: `NewWithBeadsDir(beadsDir, beadsDir)` - INCORRECT

These set workDir to the `.beads` directory itself. The command runs FROM INSIDE `.beads`. Works but wrong.

| File:Line | Current Code | Simplification |
|-----------|-------------|----------------|
| cmd/agent_state.go:207 | `NewWithBeadsDir(beadsDir, beadsDir)` | `beads.New(filepath.Dir(beadsDir))` |
| cmd/agent_state.go:246 | `NewWithBeadsDir(beadsDir, beadsDir)` | `beads.New(filepath.Dir(beadsDir))` |
| cmd/molecule_await_signal.go:345 | `NewWithBeadsDir(beadsDir, beadsDir)` | `beads.New(filepath.Dir(beadsDir))` |
| cmd/mail_queue.go:140 | `NewWithBeadsDir(beadsDir, beadsDir)` | `beads.New(filepath.Dir(beadsDir))` |
| cmd/mail_queue.go:204 | `NewWithBeadsDir(beadsDir, beadsDir)` | `beads.New(filepath.Dir(beadsDir))` |
| cmd/mail_queue.go:276 | `NewWithBeadsDir(beadsDir, beadsDir)` | `beads.New(filepath.Dir(beadsDir))` |
| cmd/mail_queue.go:315 | `NewWithBeadsDir(beadsDir, beadsDir)` | `beads.New(filepath.Dir(beadsDir))` |

**Count: 7 incorrect usages**

#### Pattern C: `NewWithBeadsDir(filepath.Dir(beadsDir), beadsDir)` - REDUNDANT

| File:Line | Current Code | Simplification |
|-----------|-------------|----------------|
| mail/router.go:60 | `NewWithBeadsDir(filepath.Dir(beadsDir), beadsDir)` | `beads.New(filepath.Dir(beadsDir))` |

**Count: 1 redundant usage**

#### Pattern D: `NewWithBeadsDir(townRoot, beadsDir)` where beadsDir is resolved earlier - REDUNDANT

| File:Line | Current Code | Simplification |
|-----------|-------------|----------------|
| plugin/recording.go:66 | `NewWithBeadsDir(r.townRoot, beadsDir)` | `beads.New(r.townRoot)` |
| plugin/recording.go:103 | `NewWithBeadsDir(r.townRoot, beadsDir)` | `beads.New(r.townRoot)` |
| rig/config.go:179 | `NewWithBeadsDir(townRoot, beadsDir)` | `beads.New(townRoot)` |
| cmd/mail_announce.go:182 | `NewWithBeadsDir(townRoot, beadsDir)` | `beads.New(townRoot)` |
| cmd/mail_channel.go:322 | `NewWithBeadsDir(townRoot, beadsDir)` | `beads.New(townRoot)` |
| cmd/rig_config.go:243 | `NewWithBeadsDir(townRoot, beadsDir)` | `beads.New(townRoot)` |
| cmd/rig_config.go:275 | `NewWithBeadsDir(townRoot, beadsDir)` | `beads.New(townRoot)` |
| cmd/mail_queue.go:29 | `NewWithBeadsDir(townRoot, beadsDir)` | `beads.New(townRoot)` |
| cmd/rig.go:1507 | `NewWithBeadsDir(rigPath, rigBeadsDir)` | `beads.New(rigPath)` |

**Count: 9 redundant usages**

#### Pattern E: Mail Package - VERIFIED

The mail package stores beadsDir at construction time and passes it around. Analysis:

| File:Line | Current Code | Analysis |
|-----------|-------------|----------|
| mail/mailbox.go:87 | `NewWithBeadsDir(m.workDir, m.beadsDir)` | ✅ CORRECT - beadsDir may be empty, resolves at run time |
| mail/mailbox.go:221 | `NewWithBeadsDir(m.workDir, beadsDir)` | ✅ CORRECT - beadsDir passed as param from m.beadsDir |
| mail/mailbox.go:308 | `NewWithBeadsDir(m.workDir, beadsDir)` | ✅ CORRECT - beadsDir passed as param from m.beadsDir |
| mail/mailbox.go:349 | `NewWithBeadsDir(m.workDir, beadsDir)` | ✅ CORRECT - beadsDir passed as param from m.beadsDir |

**Constructor Analysis:**
- `NewMailboxBeads(identity, workDir)`: beadsDir is empty → `beadsOps()` calls `NewWithBeadsDir(workDir, "")` → resolves at run time via `ResolveBeadsDir(workDir)` ✅
- `NewMailboxFromAddress(address, workDir)`: beadsDir = `ResolveBeadsDir(workDir)` → pre-resolved, REDUNDANT but works correctly
- `NewMailboxWithBeadsDir(address, workDir, beadsDir)`: beadsDir is explicit → needed when caller wants specific beadsDir

**Key Insight:** When `NewWithBeadsDir(workDir, "")` is called with empty beadsDir, the `run()` method still calls `ResolveBeadsDir(workDir)` at run time. So empty beadsDir works correctly.

**Recommendation:**
- `NewMailboxFromAddress` could be simplified to not call `ResolveBeadsDir` (let it resolve at run time)
- But this is a minor optimization, current code is CORRECT
- Keep `NewMailboxWithBeadsDir` for cases requiring explicit beadsDir

**Count: 4 usages, all CORRECT (some could be simplified but work correctly)**

#### Pattern F: Required - Init New .beads

| File:Line | Current Code | Status |
|-----------|-------------|--------|
| rig/manager.go:594 | `NewWithBeadsDir(rigPath, beadsDir)` | ✅ REQUIRED |

This is initializing a NEW .beads directory. Must prevent bd from finding parent .beads.

#### Pattern G: Migration - VERIFIED

| File:Line | Current Code | Analysis |
|-----------|-------------|----------|
| internal/cmd/migrate_agents.go:106 | `beads.New(sourceBeadsDir)` | ❌ INCORRECT - workDir is .beads path itself! |
| internal/cmd/migrate_agents.go:109 | `NewWithBeadsDir(townRoot, townBeadsDir)` | ⚠️ REDUNDANT - could use `beads.New(townRoot)` |

**Analysis of migrate_agents.go:**

Line 106:
```go
sourceBeadsDir := filepath.Join(townRoot, sourceRigPath, ".beads")
sourceBd := beads.New(sourceBeadsDir)  // WRONG: workDir = /town/rigs/foo/.beads
```
This passes the .beads PATH as workDir. When `run()` calls `ResolveBeadsDir(workDir)`:
- It looks for `/town/rigs/foo/.beads/.beads` (non-existent)
- BEADS_DIR is set to wrong path

**Fix for line 106:**
```go
sourceRigFullPath := filepath.Join(townRoot, sourceRigPath)
sourceBd := beads.New(sourceRigFullPath)  // Correct: workDir = /town/rigs/foo
```

Line 109:
```go
townBeadsDir := filepath.Join(townRoot, ".beads")
targetBd := beads.NewWithBeadsDir(townRoot, townBeadsDir)  // REDUNDANT
```
This is CORRECT but REDUNDANT. Equivalent to `beads.New(townRoot)`.

## Summary

### beads.New() Usage (All Correct)

| Category | Count | Status |
|----------|-------|--------|
| Town-level operations | 17 | ✅ All correct |
| Rig-level operations | 15 | ✅ All correct |
| Polecat/Crew operations | 4 | ✅ All correct (redirects work) |
| CWD/Generic operations | 4 | ✅ All correct |

### NewWithBeadsDir Usage

| Pattern | Count | Status | Action |
|---------|-------|--------|--------|
| Pattern A: `NewWithBeadsDir(X, ResolveBeadsDir(X))` | 9 | ⚠️ REDUNDANT | Replace with `beads.New(X)` |
| Pattern B: `NewWithBeadsDir(beadsDir, beadsDir)` | 7 | ❌ INCORRECT | Replace with `beads.New(filepath.Dir(beadsDir))` |
| Pattern C: `NewWithBeadsDir(filepath.Dir(beadsDir), beadsDir)` | 1 | ⚠️ REDUNDANT | Replace with `beads.New(filepath.Dir(beadsDir))` |
| Pattern D: `NewWithBeadsDir(townRoot, beadsDir)` (pre-resolved) | 9 | ⚠️ REDUNDANT | Replace with `beads.New(townRoot)` |
| Pattern E: Mail Package | 4 | ✅ CORRECT | Keep (supports empty beadsDir resolution) |
| Pattern F: Init New .beads | 1 | ✅ REQUIRED | Keep (must prevent parent discovery) |
| Pattern G: migrate_agents.go line 106 | 1 | ❌ INCORRECT | Fix: use rig path, not .beads path |
| Pattern G: migrate_agents.go line 109 | 1 | ⚠️ REDUNDANT | Replace with `beads.New(townRoot)` |

### Totals

| Status | Count | Files Affected |
|--------|-------|----------------|
| ✅ CORRECT | 45 | All beads.New() calls + mail package |
| ⚠️ REDUNDANT (can simplify) | 20 | Patterns A, C, D + migrate_agents.go:109 |
| ❌ INCORRECT (should fix) | 8 | Pattern B + migrate_agents.go:106 |
| ✅ REQUIRED (must keep) | 1 | rig/manager.go:594 |

## Conclusion

The migration to BeadsOps is **functionally correct** for all `beads.New()` usages. The implementation properly:
1. Resolves the correct `.beads` directory via `ResolveBeadsDir()`
2. Follows redirect files for polecats/crew worktrees
3. Sets BEADS_DIR explicitly to prevent inherited env var issues

### Required Changes

**Must Fix (8 incorrect usages):**
- Pattern B: 7 places use `NewWithBeadsDir(beadsDir, beadsDir)` which runs commands FROM INSIDE `.beads`
- `migrate_agents.go:106`: Uses `.beads` path as workDir instead of parent

**Should Simplify (20 redundant usages):**
- Patterns A, C, D: Can be replaced with `beads.New()` for cleaner code
- No behavioral change needed

**Must Keep (1 required usage):**
- `rig/manager.go:594`: Must prevent parent `.beads` discovery during initialization

## Detailed Analysis: Is workDir Required or Superfluous?

### How Prefix Routing Works

1. **routedImpl (GasTop level)**: For operations like Delete, Reopen, LabelAdd, Comment - GasTop uses `routedImpl()` which extracts prefix from bead ID, looks up path in `townRoot/.beads/routes.jsonl`, and routes to correct rig.
   - **Key**: `townRoot` is found by `FindTownRoot(workDir)` which **walks UP** the directory tree
   - So `beads.New(cwd)` works from anywhere in the town - FindTownRoot will find townRoot
   - Operations using routedImpl are **workDir-agnostic** (as long as cwd is within the town)

2. **bd native routing**: For operations like Show, List, Create, Update - relies on bd's internal routing. bd needs access to `routes.jsonl` in BEADS_DIR to route prefixed bead IDs.
   - **Question**: Does bd walk up to find routes.jsonl, or only look in BEADS_DIR?
   - If bd only looks in BEADS_DIR, then BEADS_DIR must point to a database with routes.jsonl

3. **Key insight**:
   - Operations using **routedImpl** work from any workDir within the town
   - Operations relying on **bd native routing** may need BEADS_DIR to point to town/.beads (which has routes.jsonl)

### Pattern-by-Pattern Analysis

#### Pattern A: `NewWithBeadsDir(X, ResolveBeadsDir(X))` → `beads.New(X)`

| File:Line | X value | Bead Prefix | Operation | workDir REQUIRED? | Reason |
|-----------|---------|-------------|-----------|-------------------|--------|
| deacon/stuck.go:47 | townRoot | `hq-` | GetRoleConfig (Show) | **REQUIRED** | hq- prefix routes to town beads; townRoot ensures routes.jsonl available |
| witness/manager.go:244 | townRoot | `hq-` | GetRoleConfig (Show) | **REQUIRED** | Same as above |
| doctor/patrol_check.go:247 | townRoot | `hq-` | GetRoleConfig (Show) | **REQUIRED** | Same as above |
| cmd/mail_queue.go:463 | townRoot | `hq-q-*` | CreateQueueBead | **REQUIRED** | Town-level queue beads need townRoot |
| cmd/mail_queue.go:510 | townRoot | `hq-q-*` | GetQueueBead (Show) | **REQUIRED** | Same as above |
| cmd/mail_queue.go:572 | townRoot | `hq-q-*` | Update queue | **REQUIRED** | Same as above |
| cmd/mail_queue.go:626 | townRoot | `hq-q-*` | Update queue | **REQUIRED** | Same as above |
| rig/manager.go:653 | rigPath | `<prefix>-*` | CreateAgentBead | **REQUIRED** | Rig-level agents need rig's .beads |
| polecat/manager.go:85 | beadsPath | `<prefix>-*` | Various | **REQUIRED** | Polecats operate on rig beads |

**Conclusion for Pattern A**: All 9 usages have REQUIRED workDir. The `NewWithBeadsDir(X, ResolveBeadsDir(X))` can be simplified to `beads.New(X)` but X itself is necessary - cannot use cwd instead.

#### Pattern B: `NewWithBeadsDir(beadsDir, beadsDir)` → `beads.New(filepath.Dir(beadsDir))`

**UPDATED after bd routing investigation:**

| File:Line | How beadsDir obtained | Operation | bd routing? | workDir REQUIRED? | Reason |
|-----------|----------------------|-----------|-------------|-------------------|--------|
| cmd/agent_state.go:207,246 | `ResolveBeadsDir(cwd)` | Update, Show | **Yes (walks up)** | **SUPERFLUOUS** | bd routing handles it |
| cmd/molecule_await_signal.go:345 | `ResolveBeadsDir(cwd)` | Update | **Yes (walks up)** | **SUPERFLUOUS** | bd routing handles it |
| cmd/mail_queue.go:140 | `ResolveBeadsDir(townRoot)` | **List** | **No** | **REQUIRED** | List has NO routing |
| cmd/mail_queue.go:204 | `ResolveBeadsDir(townRoot)` | LabelAdd | Yes (routedImpl) | **SUPERFLUOUS** | GasTop routedImpl handles it |
| cmd/mail_queue.go:276 | `ResolveBeadsDir(townRoot)` | Show | **Yes (walks up)** | **SUPERFLUOUS** | bd routing handles it |
| cmd/mail_queue.go:315 | `ResolveBeadsDir(townRoot)` | LabelRemove | Yes (routedImpl) | **SUPERFLUOUS** | GasTop routedImpl handles it |

**Analysis**:

1. **agent_state.go, molecule_await_signal.go**:
   - Operations (Update, Show) have **bd native routing** that walks up to find routes.jsonl
   - Fix: `beads.New(filepath.Dir(beadsDir))` - technically **SUPERFLUOUS** but cleaner than broken code

2. **mail_queue.go line 140 (List)**:
   - List has **NO routing support** in bd - searches only local BEADS_DIR
   - `beads.New(cwd)` would be WRONG if cwd is in a rig (would list rig's .beads instead of town's)
   - Fix: `beads.New(townRoot)` - **REQUIRED** to target town beads

3. **mail_queue.go lines 204, 276, 315 (LabelAdd, Show, LabelRemove)**:
   - LabelAdd/LabelRemove use GasTop routedImpl (walks up)
   - Show uses bd native routing (walks up)
   - Fix: `beads.New(townRoot)` - **SUPERFLUOUS** for routing, but **consistent** with List

**The current code is BROKEN** (workDir = .beads path), so any fix improves it.

**Recommendation for Pattern B**:
- For agent_state.go and molecule_await_signal.go: Fix to `beads.New(filepath.Dir(beadsDir))` - cleaner code
- For mail_queue.go: Fix to `beads.New(townRoot)` - **REQUIRED for List** (no routing), consistent for others

#### Pattern C: `NewWithBeadsDir(filepath.Dir(beadsDir), beadsDir)` → `beads.New(filepath.Dir(beadsDir))`

| File:Line | beadsDir | Operation | workDir REQUIRED? | Reason |
|-----------|----------|-----------|-------------------|--------|
| mail/router.go:60 | `townRoot/.beads` | Send mail | **REQUIRED** | Mail operations need town beads |

**Analysis**: The router needs townRoot to send mail to town-level beads. `filepath.Dir(beadsDir)` = townRoot is required.

#### Pattern D: `NewWithBeadsDir(townRoot, beadsDir)` where beadsDir is pre-resolved

| File:Line | Bead Prefix | Operation | workDir REQUIRED? | Reason |
|-----------|-------------|-----------|-------------------|--------|
| plugin/recording.go:66,103 | Various | Create/Update convoy | **REQUIRED** | Convoys are town-level |
| rig/config.go:179 | `hq-` | ConfigGet | **REQUIRED** | Town-level config |
| cmd/mail_announce.go:182 | `hq-` | Create message | **REQUIRED** | Town-level mail |
| cmd/mail_channel.go:322 | `hq-ch-*` | Channel ops | **REQUIRED** | Town-level channels |
| cmd/rig_config.go:243,275 | `hq-` | ConfigGet/Set | **REQUIRED** | Town-level config |
| cmd/mail_queue.go:29 | `hq-q-*` | Queue ops | **REQUIRED** | Town-level queues |
| cmd/rig.go:1507 | `<prefix>-*` | Rig ops | **REQUIRED** | Rig-level operations |

**Conclusion for Pattern D**: All 9 usages have REQUIRED workDir (townRoot or rigPath). The simplification to `beads.New(townRoot)` or `beads.New(rigPath)` is just cleaner code.

#### Pattern G: migrate_agents.go

| Line | Current Code | Analysis |
|------|-------------|----------|
| 106 | `beads.New(sourceBeadsDir)` where sourceBeadsDir = `.beads` path | **BROKEN** - should be `beads.New(filepath.Join(townRoot, sourceRigPath))` |
| 109 | `NewWithBeadsDir(townRoot, townBeadsDir)` | **REQUIRED** but redundant - `beads.New(townRoot)` works |

**Analysis of line 106**: The migration reads from a rig's .beads. The correct fix is:
```go
sourceRigPath := filepath.Join(townRoot, sourceRigPath)
sourceBd := beads.New(sourceRigPath)
```
The workDir (rig path) is **REQUIRED** because we're reading rig-level agent beads.

## Summary Table

| Pattern | Count | workDir Assessment | Recommended Change |
|---------|-------|-------------------|-------------------|
| A | 9 | All **REQUIRED** (bd-native ops need correct BEADS_DIR) | Simplify to `beads.New(X)`, keep X |
| B | 7 | 3 **SUPERFLUOUS**, 4 **REQUIRED** (2 for bd-native, 2 for consistency) | Fix broken code; use `cwd` for agent_state/molecule, `townRoot` for mail_queue |
| C | 1 | **REQUIRED** (mail routing needs town beads) | Simplify to `beads.New(filepath.Dir(beadsDir))` |
| D | 9 | All **REQUIRED** (bd-native ops need correct BEADS_DIR) | Simplify to `beads.New(townRoot/rigPath)` |
| E (mail) | 4 | All **CORRECT** | No change needed |
| F (init) | 1 | **REQUIRED** (must keep NewWithBeadsDir) | No change |
| G | 2 | 1 broken + 1 redundant | Fix line 106; simplify line 109 |

### Key Distinction: routedImpl vs bd-native operations

**UPDATED after investigating bd implementation in ~/repos/beads:**

bd **does walk up** the directory tree to find `routes.jsonl` for certain operations:

| Operation Type | GasTop routedImpl? | bd native routing? | workDir matters? |
|---------------|-------------------|-------------------|------------------|
| LabelAdd, LabelRemove | Yes | N/A | **No** | GasTop FindTownRoot walks up |
| Delete, Reopen, Comment | Yes | No | **No** | GasTop FindTownRoot walks up |
| **Show** | No | **Yes (walks up)** | **No** | bd `findTownRoutes()` walks up |
| **Update** | No | **Yes (walks up)** | **No** | bd `needsRouting()` walks up |
| **Close** | No | **Yes (needsRouting)** | **No** | bd `resolveAndGetIssueWithRouting()` |
| **List** | No | **No** | **Yes** | Searches only local BEADS_DIR |
| **Delete** | No | **No** | **Yes** | No routing in delete.go |
| Create | No | Partial (--rig flag) | **Depends** | Can route with --rig/--prefix |

**Key insight**: bd's Show and Update operations use `resolveAndGetIssueWithRouting()` which calls `findTownRoutes()` → `findTownRoot()` to walk up the directory tree looking for `mayor/town.json` as a town marker, then loads routes from `townRoot/.beads/routes.jsonl`.

**However**, List has **NO routing support** - it only searches the local BEADS_DIR database.

## Final Recommendations

### Must Fix (truly broken code - workDir = .beads path):
1. **Pattern B (agent_state.go:207,246)**: Replace `NewWithBeadsDir(beadsDir, beadsDir)` with `beads.New(cwd)` - workDir is **SUPERFLUOUS** (same BEADS_DIR)
2. **Pattern B (molecule_await_signal.go:345)**: Replace `NewWithBeadsDir(beadsDir, beadsDir)` with `beads.New(cwd)` - workDir is **SUPERFLUOUS** (same BEADS_DIR)
3. **Pattern B (mail_queue.go:140,204,276,315)**: Replace `NewWithBeadsDir(beadsDir, beadsDir)` with `beads.New(townRoot)` - workDir is **REQUIRED** for List/Show (bd-native ops targeting town beads)
4. **migrate_agents.go:106**: Replace `beads.New(sourceBeadsDir)` with `beads.New(filepath.Join(townRoot, sourceRigPath))` - workDir is **REQUIRED** (bd-native ops targeting rig beads)

### Can Simplify (cleaner code, same semantics - workDir still required):
1. **All Pattern A usages (9)**: Replace `NewWithBeadsDir(X, ResolveBeadsDir(X))` with `beads.New(X)` - X is **REQUIRED** (bd-native ops need correct BEADS_DIR)
2. **Pattern C (mail/router.go:60)**: Replace `NewWithBeadsDir(filepath.Dir(beadsDir), beadsDir)` with `beads.New(filepath.Dir(beadsDir))` - **REQUIRED** (mail targets town beads)
3. **All Pattern D usages (9)**: Replace `NewWithBeadsDir(townRoot, beadsDir)` with `beads.New(townRoot)` - **REQUIRED** (bd-native ops need correct BEADS_DIR)
4. **migrate_agents.go:109**: Replace `NewWithBeadsDir(townRoot, townBeadsDir)` with `beads.New(townRoot)` - **REQUIRED** (bd-native ops targeting town beads)

### Keep As-Is:
1. **rig/manager.go:594**: Required for init - must use `NewWithBeadsDir` to prevent parent .beads discovery
2. **mail package (Pattern E)**: Correct design - supports both empty and explicit beadsDir

## GasTop routedImpl Analysis

**Question**: Are any `routedImpl()` usages in GasTop superfluous given bd's native routing?

**Investigation Results** (verified against bd source code in ~/repos/beads):

| Operation | GasTop Comment | bd Reality (verified) | Verdict |
|-----------|----------------|----------------------|---------|
| **Delete, DeleteWithOptions** | "bd doesn't route this by prefix" | NO routing (delete.go) | **REQUIRED** ✓ Comment accurate |
| **Reopen** | "bd doesn't route this by prefix" | NO routing (reopen.go) | **REQUIRED** ✓ Comment accurate |
| **AddDependency, RemoveDependency** | "bd now routes this natively..." | NO routing (dep.go) | **REQUIRED** ✓ Comment was INCORRECT (fixed) |
| **LabelAdd, LabelRemove** | "bd now routes this natively..." | NO routing (label.go) | **REQUIRED** ✓ Comment was INCORRECT (fixed) |
| **Comment** | "bd now routes this natively..." | NO routing (comments.go) | **REQUIRED** ✓ Comment was INCORRECT (fixed) |

### bd Commands WITH Routing Support

These commands use `needsRouting()` and `resolveAndGetIssueWithRouting()`:
- **show.go** - Uses routing to resolve prefixed bead IDs
- **update.go** - Uses routing to find correct database
- **close.go** - Uses routing via `needsRouting()` check

### bd Commands WITHOUT Routing Support

These commands do NOT use routing infrastructure:
- **delete.go** - NO routing
- **reopen.go** - NO routing
- **dep.go** (dependencies) - NO routing
- **label.go** - NO routing
- **comments.go** - NO routing

### Conclusion

**ALL `routedImpl()` usages in GasTop are REQUIRED.** None are superfluous.

The comments in implementation.go that claimed "bd now routes this command by prefix natively, but we keep routedImpl for reliability" were INCORRECT for AddDependency, RemoveDependency, LabelAdd, LabelRemove, and Comment. These comments have been corrected to "bd doesn't route this by prefix, so we use routedImpl."

GasTop's `routedImpl()` is fixing REAL gaps in bd's routing support for these operations.
