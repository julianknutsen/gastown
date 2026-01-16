# BeadsOps WorkDir and BEADS_DIR Audit

This document audits all usages of `NewWithBeadsDir`, `beads.New(workDir)` with non-cwd workDir,
and direct `exec.Command("bd")` calls to identify unnecessary complexity.

## Background

The BeadsOps interface implementation has two constructors:

1. **`beads.New(workDir)`**: Sets workDir, auto-detects townRoot. When running:
   - `cmd.Dir = workDir`
   - `BEADS_DIR = ResolveBeadsDir(workDir)` (finds .beads, follows redirects)

2. **`NewWithBeadsDir(workDir, beadsDir)`**: Same but with explicit beadsDir:
   - `cmd.Dir = workDir`
   - `BEADS_DIR = beadsDir` (uses explicit value, NO redirect resolution)

**Key insight**: Both ALWAYS set BEADS_DIR explicitly. The only difference is whether
ResolveBeadsDir is called (which handles .beads/redirect files).

## Audit Categories

### Category 1: `NewWithBeadsDir` Usages

| File | Line | Pattern | Analysis |
|------|------|---------|----------|
| mail/mailbox.go | 87 | `NewWithBeadsDir(m.workDir, m.beadsDir)` | beadsDir may be empty (see NewMailboxBeads) - works but inconsistent |
| mail/mailbox.go | 221 | `NewWithBeadsDir(m.workDir, beadsDir)` | beadsDir passed as param |
| mail/router.go | 60 | `NewWithBeadsDir(filepath.Dir(beadsDir), beadsDir)` | **REDUNDANT**: Could use `beads.New(filepath.Dir(beadsDir))` |
| rig/manager.go | 594 | `NewWithBeadsDir(rigPath, beadsDir)` | **REQUIRED**: Initializing NEW .beads, must prevent finding parent |
| rig/manager.go | 653 | `NewWithBeadsDir(rigPath, rigBeadsDir)` | rigBeadsDir = ResolveBeadsDir(rigPath) - **REDUNDANT** |
| deacon/stuck.go | 47 | `NewWithBeadsDir(townRoot, ResolveBeadsDir(townRoot))` | **REDUNDANT** |
| witness/manager.go | 244 | `NewWithBeadsDir(townRoot, ResolveBeadsDir(townRoot))` | **REDUNDANT** |
| doctor/patrol_check.go | 247 | `NewWithBeadsDir(townRoot, ResolveBeadsDir(townRoot))` | **REDUNDANT** |
| plugin/recording.go | 66,103 | `NewWithBeadsDir(r.townRoot, beadsDir)` | beadsDir explicitly resolved earlier |
| cmd/agent_state.go | 207,246 | `NewWithBeadsDir(beadsDir, beadsDir)` | **WRONG**: workDir=beadsDir is inside .beads! Should be `New(filepath.Dir(beadsDir))` |
| cmd/molecule_await_signal.go | 345 | `NewWithBeadsDir(beadsDir, beadsDir)` | Same issue |
| cmd/mail_announce.go | 182 | `NewWithBeadsDir(townRoot, beadsDir)` | beadsDir = ResolveBeadsDir(townRoot) - **REDUNDANT** |
| cmd/mail_queue.go | Various | `NewWithBeadsDir(beadsDir, beadsDir)` or `NewWithBeadsDir(townRoot, ResolveBeadsDir(...))` | Mixed patterns |
| cmd/mail_channel.go | 322 | `NewWithBeadsDir(townRoot, beadsDir)` | **REDUNDANT** |
| cmd/rig.go | 1507 | `NewWithBeadsDir(rigPath, rigBeadsDir)` | rigBeadsDir computed separately - could simplify |
| cmd/rig_config.go | 243,275 | `NewWithBeadsDir(townRoot, beadsDir)` | beadsDir = ResolveBeadsDir(townRoot) - **REDUNDANT** |
| polecat/manager.go | 85 | `NewWithBeadsDir(beadsPath, resolvedBeads)` | resolvedBeads = ResolveBeadsDir(beadsPath) |

### Category 2: Patterns That Can Be Simplified

#### Pattern A: `NewWithBeadsDir(X, ResolveBeadsDir(X))`

This is equivalent to `beads.New(X)` because New() internally calls ResolveBeadsDir when BEADS_DIR is empty.

**Occurrences**:
- deacon/stuck.go:47
- witness/manager.go:244
- doctor/patrol_check.go:247
- rig/manager.go:653
- cmd/mail_announce.go:182
- cmd/mail_channel.go:322
- cmd/rig_config.go:243,275

**Fix**: Replace with `beads.New(X)`

#### Pattern B: `NewWithBeadsDir(beadsDir, beadsDir)` where beadsDir IS the .beads path

This sets workDir to the .beads directory itself (e.g., `/home/user/gt/.beads`), which is unusual.
It works because BEADS_DIR is explicitly set, but it's cleaner to run from the parent.

**Occurrences**:
- cmd/agent_state.go:207,246
- cmd/molecule_await_signal.go:345
- cmd/mail_queue.go:140,204,276,315

**Fix**: Replace with `beads.New(filepath.Dir(beadsDir))`

#### Pattern C: `NewWithBeadsDir(filepath.Dir(beadsDir), beadsDir)`

This explicitly sets workDir to parent of .beads and beadsDir to .beads path.
Equivalent to `beads.New(filepath.Dir(beadsDir))`.

**Occurrences**:
- mail/router.go:60

**Fix**: Replace with `beads.New(filepath.Dir(beadsDir))`

### Category 3: Legitimate Uses of NewWithBeadsDir

These are cases where NewWithBeadsDir is actually necessary:

1. **rig/manager.go:594** - Initializing a NEW .beads directory
   - Must prevent bd from walking up and finding parent .beads
   - NewWithBeadsDir is REQUIRED here

2. **Cases where redirect should NOT be followed**
   - If code explicitly wants to use a specific .beads and ignore redirects
   - Review case-by-case

### Category 4: Direct `exec.Command("bd")` Calls (Not Using BeadsOps)

These still exist and should eventually be converted:

| File | Line | Purpose |
|------|------|---------|
| daemon/convoy_watcher.go | 88 | `bd activity --follow --town --json` (streaming) |
| deps/beads.go | 40 | `bd version` (dependency check) |
| beads/daemon.go | 41,127 | Daemon health/start (special daemon handling) |
| cmd/molecule_await_signal.go | 241 | `bd activity --follow` (streaming) |
| cmd/beads_version.go | 97 | `bd version` |
| tui/feed/*.go | Various | Streaming activity feeds |
| tui/convoy/model.go | Various | TUI convoy display |

**Note**: Streaming commands (`--follow`) and TUI commands may need special handling
and might be appropriate to keep as direct exec.Command calls.

### Category 5: BeadsOps Migration Comments

These comments document the original `cmd.Dir` settings before conversion to BeadsOps.
They serve as documentation for why a particular workDir was chosen.

Most follow patterns:
- `cmd.Dir=townRoot (REQUIRED - town beads)` → `beads.New(townRoot)`
- `cmd.Dir=rigPath (REQUIRED - rig beads)` → `beads.New(rigPath)`
- `cmd.Dir N/A, BEADS_DIR N/A (runs from cwd)` → `beads.New(cwd)`

## Recommendations

### High Priority Simplifications

1. **Replace `NewWithBeadsDir(X, ResolveBeadsDir(X))` with `beads.New(X)`**
   - ~10 occurrences
   - No behavioral change, just cleaner code

2. **Replace `NewWithBeadsDir(beadsDir, beadsDir)` with `beads.New(filepath.Dir(beadsDir))`**
   - ~6 occurrences
   - Fixes weird pattern of running from inside .beads directory

3. **Replace `NewWithBeadsDir(filepath.Dir(beadsDir), beadsDir)` with `beads.New(filepath.Dir(beadsDir))`**
   - ~1 occurrence
   - Equivalent behavior, simpler code

### Keep As-Is

1. **rig/manager.go:594** - Necessary to prevent parent .beads discovery during init

2. **Direct exec.Command for streaming** - May need special handling for `--follow` commands

### Questions to Resolve

1. **mail/router.go:resolveBeadsDir()** doesn't use `beads.ResolveBeadsDir()`, so it ignores redirects.
   Is this intentional? All mail goes to `townRoot/.beads` without following redirects.

2. **mail/mailbox.go** stores beadsDir at construction time via `ResolveBeadsDir()`.
   This means if filesystem changes after construction, it uses stale path.
   Is this a problem in practice?
