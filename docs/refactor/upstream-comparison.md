# Upstream vs Refactor Comparison Report

This report compares `working-tree` (pre-merge refactor) and `refactor/agents` (post-merge) to verify all changes are intentional architectural decisions or legitimate upstream features.

## Summary

**ARCHITECTURE CHANGE IS INTENTIONAL:**
The refactoring changes how managers work - from internal tmux creation to dependency-injected agent.Agents. This is correct and enables testability.

**SOME UPSTREAM FEATURES NEED REVIEW:**
Several upstream performance and bug fixes may need to be ported to the refactored architecture.

---

## Architectural Differences (INTENTIONAL)

### Factory Pattern Change

**Before (working-tree):**
```go
// Factory had a monolithic Start() function for all agent types
func Start(townRoot string, id agent.AgentID, aiRuntime string, opts ...StartOption) (agent.AgentID, error)

// Managers took AgentObserver (read-only)
func NewManager(agents agent.AgentObserver, r *rig.Rig) *Manager
```

**After (refactor/agents):**
```go
// Factory creates managers with DI
func MayorManager(townRoot, agentName string) *mayor.Manager
func DeaconManager(townRoot, agentName string) *deacon.Manager

// Managers take full Agents interface and own their lifecycle
func NewManager(townRoot, agentName string, agents agent.Agents) *Manager
```

**Why this is correct:**
- Managers now own their lifecycle (Start/Stop) instead of factory
- DI enables unit testing with agent.Double
- Factory is now just wiring (~135 lines vs ~560 lines)
- Each manager is self-contained

---

## Upstream Features Analysis

### 1. Parallel Startup (MISSING from refactor)

**Upstream has:**
```go
// internal/cmd/start.go - parallel startup with goroutines
func startCoreAgents(townRoot string, agentOverride string, mu *sync.Mutex) error {
    var wg sync.WaitGroup

    wg.Add(1)
    go func() { /* start Mayor */ }()

    wg.Add(1)
    go func() { /* start Deacon */ }()

    wg.Wait()
}
```

**Refactor has:**
```go
// Sequential startup
func startCoreAgents(townRoot string, agentOverride string) error {
    mayorMgr := factory.MayorManager(townRoot, mayorAgent)
    mayorMgr.Start()

    deaconMgr := factory.DeaconManager(townRoot, deaconAgent)
    deaconMgr.Start()
}
```

**Impact:** Performance regression on startup. Mayor and Deacon start sequentially instead of in parallel.

**Recommendation:** Can be added to refactored architecture. The managers support concurrent Start() calls.

---

### 2. WaitForCommand Fatal Behavior (DIFFERENT in refactor)

**Upstream mayor.Start():**
```go
// Fatal if Claude fails to launch - kills the session
if err := t.WaitForCommand(sessionID, constants.SupportedShells, constants.ClaudeStartTimeout); err != nil {
    _ = t.KillSessionWithProcesses(sessionID)
    return fmt.Errorf("waiting for mayor to start: %w", err)
}
```

**Refactor mayor.Start():**
```go
// Non-fatal - ignores WaitReady errors
_ = m.agents.WaitReady(agentID)
return nil
```

**Impact:** Refactored version may report success even if Claude fails to start.

**Recommendation:** Consider making WaitReady failures fatal or at least logged.

---

### 3. Working Directory (DIFFERENT in refactor)

**Upstream mayor.Start():**
```go
// Creates session in townRoot per issue #280
if err := t.NewSessionWithCommand(sessionID, m.townRoot, startupCmd); err != nil
```

**Refactor mayor.Start():**
```go
// Uses mayorDir
agentID, err := m.agents.Start(m.SessionName(), mayorDir, startupCmd)
```

**Impact:** May break tools that expect Mayor to run from town root.

**Issue Reference:** https://github.com/anthropics/gastown/issues/280

**Recommendation:** Verify this doesn't break Mayor functionality.

---

### 4. KillSessionWithProcesses (PRESENT in refactor ✅)

**Both versions have this function in tmux.go:**
```go
func (t *Tmux) KillSessionWithProcesses(name string) error
```

**Status:** No issue - feature is present.

---

### 5. Startup Beacon (DIFFERENT in refactor)

**Upstream mayor.Start():**
```go
// Includes startup beacon with cold-start instructions
beacon := session.FormatStartupNudge(session.StartupNudgeConfig{
    Recipient: "mayor",
    Sender:    "human",
    Topic:     "cold-start",
})
startupCmd, err := config.BuildAgentStartupCommandWithAgentOverride(
    "mayor", "", m.townRoot, "", beacon, agentOverride)
```

**Refactor mayor.Start():**
```go
// Simple command without beacon
startupCmd := config.BuildAgentCommand(m.agentName, "")
```

**Impact:** Mayor may not receive initial instructions on cold start.

**Recommendation:** Verify if cold-start beacon is needed or handled elsewhere.

---

## Files Changed Analysis

### New Files in refactor/agents (correct - refactoring additions)
- `internal/mayor/manager.go` - NEW: Mayor lifecycle manager
- `internal/mayor/manager_test.go` - NEW: Unit tests
- `internal/deacon/manager.go` - NEW: Deacon lifecycle manager
- `internal/deacon/manager_test.go` - NEW: Unit tests

### Modified Files (architectural changes)
- `internal/witness/manager.go` - Changed from AgentObserver to full Agents
- `internal/refinery/manager.go` - Changed from AgentObserver to full Agents
- `internal/polecat/session_manager.go` - Changed to use Agents
- `internal/crew/manager.go` - Changed to use Agents
- `internal/factory/factory.go` - Simplified from ~560 to ~135 lines

### Test Files Updated
- `internal/mayor/manager_test.go` - Uses agent.Double
- `internal/deacon/manager_test.go` - Uses agent.Double
- `internal/witness/manager_test.go` - Updated for new signature
- `internal/refinery/manager_test.go` - Updated for new signature
- `internal/polecat/session_manager_interface_test.go` - Uses agent.Double

---

## Verification Checklist

| Aspect | Status | Notes |
|--------|--------|-------|
| Zombie detection | ✅ Present | In agent.Implementation.Start() |
| Env vars in command | ✅ Present | Via config.EnvVars in agent.Config |
| Theming | ✅ Present | Via OnSessionCreated callback |
| ErrAlreadyRunning | ✅ Present | Re-exported from agent package |
| Parallel startup | ❌ Missing | Performance optimization not ported |
| Fatal WaitForCommand | ⚠️ Changed | Refactor ignores errors |
| Working directory | ⚠️ Changed | Uses mayorDir not townRoot |
| Startup beacon | ⚠️ Changed | Not included in refactor |

---

## Recommendations

1. **Parallel Startup**: Add goroutine-based parallel startup to `cmd/start.go`. The refactored managers support this.

2. **WaitReady Error Handling**: Consider making WaitReady errors fatal in manager.Start() or at least logging them.

3. **Working Directory Verification**: Test that Mayor works correctly when started in mayorDir vs townRoot. If issue #280 is relevant, may need to change.

4. **Startup Beacon**: Verify if cold-start beacon is handled elsewhere or if it needs to be added back.

---

## Conclusion

The architectural refactoring is correct and achieves the testability goals. Some upstream performance optimizations and bug fixes may need to be ported. The differences are:

1. **Intentional architecture changes** - factory pattern, DI, managers owning lifecycle
2. **Missing upstream features** - parallel startup
3. **Behavioral differences** - error handling, working directory, startup beacon

None of these appear to be merge errors - they're either intentional architectural decisions or upstream features that weren't part of the original refactor commits.
