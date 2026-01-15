# Agent Manager Test Coverage Handoff

## Summary

We're implementing missing tests from `internal/agent/TEST_PLAN.md` to ensure all manager types have proper test coverage.

## What Was Done

1. **Created factory package** (`internal/factory/factory.go`) - Centralized manager creation for cmd and daemon

2. **Added tests to mayor_test.go**:
   - `TestManager_Start_CreatesSettings` (S4) - verifies settings file creation
   - `TestManager_Start_SettingsFailure_NoAgentCreated` (S5) - verifies no agent on settings failure
   - `TestManager_Start_AgentStartFailure` - failure injection
   - `TestManager_Stop_AgentStopFailure` - failure injection
   - `TestManager_Start_WaitReadyFailure` - documents current behavior (WaitReady errors ignored)
   - `TestManager_Start_UsesCorrectWorkDir` - verifies workDir parameter
   - `TestManager_Start_UsesCorrectCommand` - verifies command parameter

3. **Added getters to agent.Double** (`internal/agent/double.go`):
   - `GetWorkDir(id)` - returns workDir passed to Start
   - `GetCommand(id)` - returns command passed to Start

4. **Added tests to deacon_test.go**:
   - `TestManager_Start_UsesCorrectWorkDir` - verifies workDir parameter
   - `TestManager_Start_UsesCorrectCommand` - verifies command parameter
   - `TestManager_Start_CreatesSettings` (S4) - verifies settings file creation
   - `TestManager_Start_SettingsFailure_NoAgentCreated` (S5) - verifies no agent on settings failure
   - `TestManager_Start_AgentStartFailure` - failure injection
   - `TestManager_Stop_AgentStopFailure` - failure injection
   - `TestManager_Start_WaitReadyFailure` - documents current behavior (WaitReady errors ignored)

5. **Updated TEST_PLAN.md** - Marked mayor and deacon tests as complete

## Current Test Status

All manager tests pass:
```bash
go test ./internal/mayor/... ./internal/deacon/... ./internal/witness/... ./internal/refinery/...
```

## Remaining Work

### Crew
- [ ] Review existing tests for coverage with agent.Double

### Polecat
- [ ] Review existing tests for coverage

## Key Files

- `internal/agent/TEST_PLAN.md` - Test checklist (source of truth)
- `internal/agent/double.go` - Test double with GetWorkDir/GetCommand getters
- `internal/mayor/manager_test.go` - Reference for test patterns
- `internal/deacon/manager_test.go` - Reference for test patterns
- `internal/refinery/manager_test.go` - Already uses agent.Double

## Test Pattern

All manager tests follow this pattern:

```go
func TestManager_Start_CreatesSettings(t *testing.T) {
    townRoot := t.TempDir()
    // Create manager dir WITHOUT .claude/settings.local.json
    managerDir := filepath.Join(townRoot, "manager_name")
    require.NoError(t, os.MkdirAll(managerDir, 0755))

    agents := agent.NewDouble()
    mgr := manager.NewManager(townRoot, "claude", agents)

    err := mgr.Start()
    require.NoError(t, err)

    // Verify settings were created
    settingsPath := filepath.Join(managerDir, ".claude", "settings.local.json")
    _, err = os.Stat(settingsPath)
    assert.NoError(t, err, "settings file should be created by Start")
}
```

## Notes

- Zombie detection is tested in `agent/agent_test.go`, not manager tests
- Graceful shutdown (T3) can be tested via `agents.SetStopErr()` failure injection
- WaitReady failures are currently ignored by managers (documented behavior)
