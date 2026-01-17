# Deleted Code Audit: bbb768f5..HEAD

**Review Date:** 2026-01-17
**Commits Reviewed:** bbb768f5 to HEAD (e81d9696)
**Focus:** What was removed and whether functionality was migrated

## Summary

This review examines all files deleted between commits bbb768f5 and HEAD, evaluating whether their functionality was properly migrated and identifying any features that may have silently disappeared.

**Verdict:** All deletions appear intentional and well-migrated. The refactoring consolidated agent lifecycle into `factory.Start()` and unified session cycling into a single `cycle.go` file. No functionality appears to be lost.

---

## Deleted Files

### 1. Package: `internal/mayor/`

#### `internal/mayor/manager.go` (DELETED)

**What it did:**
- `Manager` struct for mayor lifecycle operations (Start, Stop, IsRunning, Status)
- Wrapped `agent.Agents` interface for mayor-specific operations
- Used `runtime.EnsureSettingsForRole()` for settings setup
- Called `agents.Start()`, `agents.WaitReady()`, `agents.Stop()`

**Migration Status:** FULLY MIGRATED

The mayor lifecycle is now handled by:
1. `factory.Start(townRoot, agent.MayorAddress, aiRuntime)` - in `internal/factory/factory.go`
2. `factory.Agents().Stop(id, true)` - for stopping
3. Direct `agents.Exists()` calls for status checks

**Evidence:** `internal/cmd/mayor.go` now uses:
```go
factory.Start(townRoot, agent.MayorAddress, agentName)  // Start
factory.Agents().Stop(id, true)                         // Stop
agents.Exists(id)                                       // IsRunning/Status
```

**TODO/FIXME Comments:** None found in deleted code.

---

#### `internal/mayor/manager_test.go` (DELETED)

**What it did:**
- Comprehensive unit tests for mayor manager lifecycle
- Tests for Start, Stop, IsRunning, Status
- Tests for error paths (already running, not running, settings failure)
- Tests for failure injection using `agent.AgentsStub`

**Migration Status:** TESTS NOT YET MIGRATED

**Risk:** The deleted tests provided coverage for:
- `TestManager_Start_CreatesAgent`
- `TestManager_Start_WhenAlreadyRunning_ReturnsError`
- `TestManager_Stop_TerminatesAgent`
- `TestManager_Stop_WhenNotRunning_ReturnsError`
- `TestManager_IsRunning_ReturnsCorrectState`
- `TestManager_Status_ReturnsInfo_WhenRunning`
- `TestManager_Start_UsesCorrectWorkDir`
- `TestManager_Start_UsesCorrectCommand`
- `TestManager_Start_CreatesSettings`
- `TestManager_Start_SettingsFailure_NoAgentCreated`
- Failure injection tests

**Recommendation:** Add equivalent tests to `internal/factory/factory_test.go` or `internal/cmd/mayor_test.go`.

---

### 2. Package: `internal/deacon/`

#### `internal/deacon/manager.go` (DELETED)

**What it did:**
- `Manager` struct for deacon lifecycle (identical pattern to mayor)
- Additional `Nudge(message string)` method for sending messages to deacon
- `Agents()` method to access underlying agent interface

**Migration Status:** FULLY MIGRATED

The deacon lifecycle is now handled by:
1. `factory.Start(townRoot, agent.DeaconAddress, aiRuntime)` - for starting
2. `factory.Agents().Stop(id, true)` - for stopping
3. `agents.Nudge(id, message)` - for nudging (method moved to Agents interface)

**Evidence:** `internal/cmd/deacon.go` now uses:
```go
factory.Start(townRoot, agent.DeaconAddress, agentName)  // Start
agents.Stop(id, true)                                    // Stop
agents.Nudge(id, message)                                // Nudge
```

**Notable:** The `Nudge` method was on the deacon Manager but is now directly available on the Agents interface (`agent.Agents.Nudge()`), which is a better design.

**TODO/FIXME Comments:** None found in deleted code.

---

#### `internal/deacon/manager_test.go` (DELETED)

**What it did:**
- Nearly identical test suite to mayor manager tests
- Additional test: `TestManager_Status_ReturnsError_WhenGetInfoFails`

**Migration Status:** TESTS NOT YET MIGRATED

**Recommendation:** Add equivalent tests to `internal/factory/factory_test.go`.

---

### 3. Package: `internal/connection/` (ENTIRE PACKAGE DELETED)

#### `internal/connection/connection.go` (DELETED)

**What it did:**
- Defined `Connection` interface for abstracting local vs remote operations
- Supported file operations: ReadFile, WriteFile, MkdirAll, Remove, Stat, Glob, Exists
- Supported command execution: Exec, ExecDir, ExecEnv
- Supported tmux operations: TmuxNewSession, TmuxKillSession, TmuxSendKeys, etc.
- Defined error types: ConnectionError, NotFoundError, PermissionError
- Defined `FileInfo` interface and `BasicFileInfo` struct

**Migration Status:** NOT MIGRATED (INTENTIONALLY REMOVED)

This was infrastructure for remote machine support that was never fully implemented. The comment in `registry.go` confirms: `"ssh connections not yet implemented"`.

**Risk:** LOW - This was scaffolding for a future feature. No production code depended on it.

---

#### `internal/connection/address.go` (DELETED)

**What it did:**
- `Address` struct for parsing agent/rig addresses
- Format: `[machine:]rig[/polecat]`
- Methods: ParseAddress, String, IsLocal, IsBroadcast, RigPath, Equal, Validate

**Migration Status:** NOT MIGRATED (INTENTIONALLY REMOVED)

This address format was for cross-machine addressing (e.g., `vm:gastown/rictus`). The current system uses `agent.AgentID` for local addressing only.

**Risk:** LOW - Cross-machine support was never shipped.

---

#### `internal/connection/registry.go` (DELETED)

**What it did:**
- `MachineRegistry` for managing machine configurations
- JSON-based persistence of machine configs
- Methods: Get, Add, Remove, List, Connection, LocalConnection
- Machine struct with Name, Type, Host, KeyPath, TownPath fields

**Migration Status:** NOT MIGRATED (INTENTIONALLY REMOVED)

**Risk:** LOW - Federation feature was not implemented.

---

#### `internal/connection/local.go` (DELETED)

**What it did:**
- `LocalConnection` implementing `Connection` interface
- Wrapped `os` package calls for file operations
- Wrapped `exec.Command` for command execution
- Wrapped `tmux.Tmux` for tmux operations

**Migration Status:** NOT NEEDED

Local operations are performed directly via `os`, `exec`, and `tmux.Tmux` packages without the Connection abstraction layer.

---

#### `internal/connection/address_test.go` (DELETED)

**What it did:**
- Comprehensive tests for address parsing (150+ test cases)
- Edge cases: unicode, special characters, malformed inputs
- Tests for Equal, IsLocal, IsBroadcast, RigPath methods

**Migration Status:** TESTS NOT NEEDED (feature removed)

---

### 4. Command Files

#### `internal/cmd/session_helpers.go` (DELETED)

**What it did:**
```go
var sessionsProvider = func() session.Sessions {
    return tmux.NewTmux()
}

func newSessions() session.Sessions {
    return sessionsProvider()
}

func newTmux() *tmux.Tmux {
    return tmux.NewTmux()
}
```

This was a test seam for injecting session doubles.

**Migration Status:** SUPERSEDED

Commands now use `factory.Agents()` which provides the same testability through the `agent.Agents` interface.

**Risk:** None - the abstraction layer moved up to `factory.Agents()`.

---

#### `internal/cmd/polecat_cycle.go` (DELETED)

**What it did:**
- `cyclePolecatSession(direction, sessionOverride)` - cycle between polecat sessions
- `parsePolecatSessionName(sessionName)` - extract rig/polecat from session name
- `findRigPolecatSessions(rigName)` - find all polecats in a rig

**Migration Status:** FULLY MIGRATED

Functionality consolidated into `internal/cmd/cycle.go`:
```go
case constants.RolePolecat:
    siblings = filterByRoleAndRig(allIDs, constants.RolePolecat, rig)
```

The new implementation is cleaner - it uses `agent.AgentID` parsing instead of string manipulation on session names.

---

#### `internal/cmd/polecat_cycle_test.go` (DELETED)

**What it did:**
- Tests for `parsePolecatSessionName()` function
- 15 test cases covering valid polecats, crew exclusion, special sessions

**Migration Status:** TESTS NOT YET MIGRATED

The test logic should be adapted for the new `agent.AgentID` based cycling, but may not be necessary if `AgentID.Parse()` is well-tested.

---

#### `internal/cmd/town_cycle.go` (DELETED)

**What it did:**
- `cycleTownSession(direction, sessionOverride)` - cycle between mayor/deacon
- `isTownLevelSession(sessionName)` - check if session is town-level
- `findRunningTownSessions()` - find running mayor/deacon sessions
- Cobra commands: `gt town next`, `gt town prev`

**Migration Status:** FULLY MIGRATED

Functionality consolidated into `internal/cmd/cycle.go`:
```go
case constants.RoleMayor, constants.RoleDeacon:
    siblings = filterByRoles(allIDs, constants.RoleMayor, constants.RoleDeacon)
```

The commands are now `gt cycle next` and `gt cycle prev` (unified).

---

#### `internal/cmd/crew_cycle.go` (DELETED)

**What it did:**
- `cycleCrewSession(direction, sessionOverride)` - cycle between crew sessions
- Used `parseCrewSessionName()` and `findRigCrewSessions()`

**Migration Status:** FULLY MIGRATED

Functionality consolidated into `internal/cmd/cycle.go`:
```go
case constants.RoleCrew:
    siblings = filterByRoleAndRig(allIDs, constants.RoleCrew, rig)
```

---

### 5. Session Files

#### `internal/session/town_session.go` (DELETED)

**What it did:**
- `TownSessions` wrapper that added town ID suffix to session names
- Implemented optimistic-with-retry session resolution (new vs legacy format)
- Methods: Start, Stop, Exists, Send, SendControl, Nudge, Capture, etc.
- `appendTownSuffix()`, `resolveSession()` internal helpers

**Migration Status:** SUPERSEDED BY AgentID

The town isolation is now handled at the `agent.AgentID` level rather than session name transformation. The `AgentID` structure encodes role, rig, and worker identity without needing name suffixes.

**Risk:** MEDIUM - If there are any sessions using the old town-suffix naming convention, they may become orphaned.

**Recommendation:** Verify no production sessions use the `<name>-<townid>` format. If they do, migration tooling may be needed.

---

#### `internal/session/town_session_test.go` (DELETED)

**What it did:**
- Tests for TownSessions wrapper behavior
- Tests for suffix handling and session resolution

**Migration Status:** TESTS NOT NEEDED (feature removed)

---

## TODO/FIXME Comments in Deleted Code

**None found.** The deleted code was clean of outstanding work items.

---

## Features That May Have Silently Disappeared

### 1. Remote Machine Support (INTENTIONAL)

The entire `connection` package provided scaffolding for:
- SSH-based remote operations
- Cross-machine agent addressing (`vm:gastown/rictus`)
- Machine registry with JSON persistence

**Status:** Feature was never implemented (`"ssh connections not yet implemented"`). Removal is intentional cleanup.

### 2. Town-Suffixed Session Names (SUPERSEDED)

The `TownSessions` wrapper added town ID suffixes to allow multiple towns on one machine.

**Status:** Superseded by `agent.AgentID` which encodes identity without name transformation. This may require verification that no production sessions use the old format.

### 3. Session Override Flag for Cycling (MINOR)

The old `--session` flag allowed tmux key bindings to pass session names explicitly:
```go
var townCycleSession string
townNextCmd.Flags().StringVar(&townCycleSession, "session", "", "...")
```

**Status:** The new unified `cycle.go` uses `agent.Self()` which reads from environment variables. This should work correctly from tmux bindings since env vars are inherited.

---

## Test Coverage Gaps

The following test files were deleted without equivalent replacements:

| Deleted Test File | Coverage Gap | Priority |
|-------------------|--------------|----------|
| `mayor/manager_test.go` | Mayor lifecycle via factory.Start() | HIGH |
| `deacon/manager_test.go` | Deacon lifecycle via factory.Start() | HIGH |
| `cmd/polecat_cycle_test.go` | Session name parsing for polecats | LOW |
| `connection/address_test.go` | N/A (feature removed) | NONE |
| `session/town_session_test.go` | N/A (feature removed) | NONE |

---

## Recommendations

1. **Add factory.Start() tests** - The mayor and deacon manager tests provided valuable coverage for lifecycle operations. Equivalent tests should be added to `internal/factory/factory_test.go`.

2. **Verify no town-suffix sessions exist** - Run `tmux list-sessions` in production to ensure no sessions use the old `<name>-<townid>` format.

3. **Document the connection package removal** - If remote machine support is planned for the future, note that the scaffolding was removed and would need to be rebuilt.

4. **Consider adding cycle.go tests** - The new unified cycling code has no dedicated tests. The old tests verified edge cases like "not a polecat session" and "single session, nothing to cycle to".

---

## Conclusion

The deleted code represents a well-executed consolidation refactoring:

- **Mayor/Deacon managers** -> `factory.Start()` + `factory.Agents().Stop()`
- **Cycle commands (3 files)** -> unified `cycle.go`
- **Connection package** -> removed (feature never implemented)
- **TownSessions** -> superseded by AgentID

The main concern is test coverage - the manager tests provided good coverage for lifecycle operations that should be replicated for the new factory-based approach.
