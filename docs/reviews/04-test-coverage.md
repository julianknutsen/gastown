# Test Coverage Analysis: bbb768f5 to HEAD

## Summary

The refactor to consolidate agent lifecycle into `factory.Start()` has resulted in a **net reduction of 1,319 lines** in test files (1,492 additions vs 2,811 deletions). This reflects:
- Removal of redundant lifecycle tests from manager classes
- Deletion of entire packages (connection, mayor/deacon manager tests)
- Consolidation of Start/Stop tests into `factory_test.go`

---

## Deleted Test Files

| Deleted Test | Lines | What It Covered | Replacement Coverage |
|--------------|-------|-----------------|---------------------|
| `internal/mayor/manager_test.go` | 306 | Mayor Start/Stop/IsRunning/Status, settings creation, error injection | Partially covered by `factory_test.go` - StartWithAgents tests basic Start; **NO coverage** for mayor-specific settings or status |
| `internal/deacon/manager_test.go` | 284 | Deacon Start/Stop/IsRunning/Status, settings creation, error injection | Partially covered by `factory_test.go` - StartWithAgents tests basic Start; **NO coverage** for deacon-specific behavior |
| `internal/connection/address_test.go` | 450 | Address parsing (machine:rig/polecat format), String(), IsLocal(), IsBroadcast(), Equal(), edge cases (unicode, special chars) | **NOT REPLACED** - entire package deleted; functionality moved to `internal/ids/ids.go` with **NO tests** |
| `internal/cmd/polecat_cycle_test.go` | 118 | ParsePolecatSessionName - session name parsing for polecats | **NOT REPLACED** - function appears to be deleted |

**Total: 1,158 lines of deleted tests**

---

## Test Double Status

### agent.Double (internal/agent/double.go)

**Status: FULLY UPDATED for new interfaces**

The Double implements all methods of the `Agents` interface:
- `StartWithConfig()` - stores full StartConfig for spy verification
- `Stop()` - records calls to `StopCalls()` for verification
- `Exists()`, `Respawn()`, `WaitReady()`, `GetInfo()`
- `Nudge()`, `Capture()`, `CaptureAll()`
- `List()`, `Attach()`

Test helper methods properly updated:
- `GetStartConfig()` - returns full StartConfig for verification
- `HasOnCreated()` - verifies callback registration
- `GetEnvVars()` - returns env vars passed to Start
- `NudgeLog()`, `StopCalls()` - spy verification

**Conformance tests**: `internal/agent/conformance_test.go` verifies Double matches Implementation behavior.

### session.Double (internal/session/double.go)

**Status: FULLY UPDATED for new interfaces**

The Double implements all methods of the `Sessions` interface including new methods:
- `Respawn()` - clears buffer, updates command
- `CaptureAll()` - returns entire buffer
- `GetStartCommand()` - returns command from Start/Respawn
- `Attach()`, `SwitchTo()` - session switching support

**Conformance tests**: `internal/session/double_test.go` runs conformance tests.

---

## factory.Start() Test Coverage

The `factory_test.go` provides good coverage for `StartWithAgents()`:

| Test | Coverage |
|------|----------|
| Basic start | Agent exists after start |
| WorkDir | Correct working directory set |
| EnvVars | GT_ROLE, GT_ROOT, GT_RIG, GT_CREW, GT_POLECAT |
| WithTopic | Topic passed to command |
| WithInteractive | --dangerously-skip-permissions removed |
| WithKillExisting | Stops existing agent first |
| AlreadyRunning | Returns ErrAlreadyRunning |
| WithEnvOverrides | Custom env vars added |
| InvalidAgentID | Returns error for unknown role |
| StartError | Error propagation |
| OnCreatedCallback | Callback registered |
| All agent types | Mayor, Deacon, Boot, Witness, Refinery, Polecat, Crew |

**Integration Test Coverage**: **NONE**

No integration tests call `factory.Start()` or `StartWithAgents()`. The factory tests use `agent.Double` exclusively.

---

## Gaps in Coverage

### Critical Gaps

1. **internal/ids/ids.go - NO TESTS**
   - `ParseAddress()` - replaces connection.ParseAddress but has no tests
   - `ParseSessionName()` - complex session name parsing logic untested
   - Edge cases from deleted `address_test.go` not reproduced

2. **Manager lifecycle tests removed without equivalent replacement**
   - Mayor: settings creation, Status(), WaitReady failure handling
   - Deacon: settings creation, Status(), GetInfo error handling
   - Witness: Start/Stop tests removed (now in factory, but less coverage)
   - Refinery: Start/Stop tests removed (now in factory, but less coverage)

3. **agent.Self() edge cases**
   - Tests exist for basic cases but `ErrUnknownRole` formatting not tested with invalid role strings

### Moderate Gaps

4. **RigAgentState interface** - No direct tests for implementations
   - `SetRunning()`, `SetStopped()`, `IsRunning()` not unit tested
   - Indirectly tested through manager state file tests

5. **mergeEnvVars()** - Tested indirectly through factory_test.go but no unit tests

6. **Zombie detection** in `StartWithConfig()` - tested via conformance tests but error paths (killing zombie failure) not covered

### Behaviors Only Tested Indirectly

| Behavior | Indirect Test Location |
|----------|----------------------|
| Session name generation from AgentID | `session/identity_test.go` via SessionNameFromAgentID |
| Env var prepending to command | `factory_test.go` - verified via GetCommand() |
| OnCreated callback execution | `factory_test.go` - verified via HasOnCreated() |
| Graceful vs forceful stop | `conformance_test.go` - both paths tested |
| Theme application | Not tested (callback passed but not executed) |

---

## Recommendations

### High Priority

1. **Add tests for internal/ids/ids.go**
   - Port edge cases from deleted `address_test.go`
   - Test `ParseSessionName()` with all session name formats
   - Test error cases and invalid inputs

2. **Add integration tests for factory.Start()**
   - Test actual tmux session creation (not just with Double)
   - Verify env vars are correctly set in real sessions
   - Test theme callback execution

### Medium Priority

3. **Add unit tests for RigAgentState implementations**
   - Test Witness state transitions
   - Test Refinery state transitions

4. **Restore manager-specific tests**
   - Settings file creation/validation
   - Status() method behavior
   - Manager-specific error handling

### Low Priority

5. **Add mergeEnvVars() unit tests**
   - Test precedence (Start-level overrides Agents-level)
   - Test nil handling

---

## Metrics Summary

```
Test files changed:     21
Lines added:         1,492
Lines deleted:       2,811
Net change:         -1,319

Deleted test files:      4
New test files:          0 (factory_test.go already existed)

Test coverage trend: DECREASED
- Manager lifecycle tests removed
- Address parsing tests removed (not replaced)
- Factory tests provide partial replacement
```

---

## Appendix: Deleted Test Functions

### mayor/manager_test.go
- TestManager_Start_CreatesAgent
- TestManager_Start_WhenAlreadyRunning_ReturnsError
- TestManager_Stop_TerminatesAgent
- TestManager_Stop_WhenNotRunning_ReturnsError
- TestManager_IsRunning_ReturnsCorrectState
- TestManager_Status_ReturnsInfo_WhenRunning
- TestManager_Status_ReturnsError_WhenNotRunning
- TestManager_Start_UsesCorrectWorkDir
- TestManager_Start_UsesCorrectCommand
- TestManager_Start_CreatesSettings
- TestManager_Start_SettingsFailure_NoAgentCreated
- TestManager_Start_AgentStartFailure
- TestManager_Stop_AgentStopFailure
- TestManager_Start_WaitReadyFailure

### deacon/manager_test.go
- TestManager_Start_CreatesAgent
- TestManager_Start_WhenAlreadyRunning_ReturnsError
- TestManager_Stop_TerminatesAgent
- TestManager_Stop_WhenNotRunning_ReturnsError
- TestManager_IsRunning_ReturnsCorrectState
- TestManager_Status_ReturnsInfo_WhenRunning
- TestManager_Status_ReturnsError_WhenNotRunning
- TestManager_Status_ReturnsError_WhenGetInfoFails
- TestManager_Start_UsesCorrectWorkDir
- TestManager_Start_UsesCorrectCommand
- TestManager_Start_CreatesSettings
- TestManager_Start_SettingsFailure_NoAgentCreated
- TestManager_Start_AgentStartFailure
- TestManager_Stop_AgentStopFailure
- TestManager_Start_WaitReadyFailure

### connection/address_test.go
- TestParseAddress (basic cases)
- TestAddressString
- TestAddressIsLocal
- TestAddressIsBroadcast
- TestAddressEqual
- TestParseAddress_EdgeCases (50+ edge cases including unicode, special chars)
- TestMustParseAddress_Panics
- TestMustParseAddress_Valid
- TestAddressRigPath

### cmd/polecat_cycle_test.go
- TestParsePolecatSessionName
