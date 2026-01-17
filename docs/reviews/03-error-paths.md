# Error Paths & Edge Cases Review

**Commits reviewed:** bbb768f5..HEAD (e81d9696)
**Focus:** Error handling, edge cases, and race conditions in the refactored agent lifecycle code

---

## 1. Zombie Detection Changes

### Before (Start method)
```go
// OLD: internal/agent/agent.go - Start()
sessionID := session.SessionID(name)
exists, _ := a.sess.Exists(sessionID)
if exists {
    if a.sess.IsRunning(sessionID, a.config.ProcessNames...) {
        return AgentID(sessionID), ErrAlreadyRunning
    }
    // Zombie - kill and recreate
    if err := a.sess.Stop(sessionID); err != nil {
        return "", fmt.Errorf("killing zombie session: %w", err)
    }
}
```

### After (StartWithConfig method)
```go
// NEW: internal/agent/start_config.go - StartWithConfig()
sessionID := session.SessionID(id)
exists, _ := a.sess.Exists(sessionID)
if exists {
    if a.sess.IsRunning(sessionID, a.config.ProcessNames...) {
        return ErrAlreadyRunning
    }
    if err := a.sess.Stop(sessionID); err != nil {
        return fmt.Errorf("killing zombie session: %w", err)
    }
}
```

**Analysis:**
- The zombie detection logic is essentially unchanged in structure
- **Good:** Error from zombie cleanup is properly propagated
- **Observation:** The `Exists()` error is silently ignored (`exists, _ := ...`). This is acceptable since a false positive (assuming not exists when exists check fails) leads to a session creation attempt that will fail anyway

### Exists() Semantic Change

The `Exists()` method has been enhanced to incorporate zombie filtering:

```go
// NEW: internal/agent/agent.go - Exists()
func (a *Implementation) Exists(id AgentID) bool {
    sessionID := session.SessionID(id)
    exists, _ := a.sess.Exists(sessionID)
    if !exists {
        return false
    }
    if len(a.config.ProcessNames) == 0 {
        return true  // Can't check process, assume session = agent
    }
    return a.sess.IsRunning(sessionID, a.config.ProcessNames...)
}
```

**Implications:**
- `Exists()` now means "session exists AND agent process is alive"
- If `ProcessNames` is not configured, falls back to session existence only
- This is a behavioral change - callers expecting "session exists" semantics may be affected

---

## 2. Session Start Failure Handling

### Callback Error Cleanup

```go
// internal/agent/start_config.go
if _, err := a.sess.Start(string(id), cfg.WorkDir, command); err != nil {
    return fmt.Errorf("starting session: %w", err)
}

// Run Agents-level callback first (if any)
if a.config.OnSessionCreated != nil {
    if err := a.config.OnSessionCreated(a.sess, sessionID); err != nil {
        _ = a.sess.Stop(sessionID)  // Cleanup on failure
        return fmt.Errorf("session setup: %w", err)
    }
}

// Run per-start callback (if any)
if cfg.OnCreated != nil {
    if err := cfg.OnCreated(a.sess, sessionID); err != nil {
        _ = a.sess.Stop(sessionID)  // Cleanup on failure
        return fmt.Errorf("session setup: %w", err)
    }
}
```

**Good:**
- Session is cleaned up if either callback fails
- Errors are wrapped with context ("session setup:")

**Issue:** Stop() error is silently discarded (`_ = a.sess.Stop(...)`). If stop fails after callback failure:
- The session may be left in an orphaned state
- The original callback error is returned, masking the cleanup failure
- **Recommendation:** Consider logging the cleanup error or returning a compound error

### factory.Start() Error Handling

```go
// internal/factory/factory.go - Start()
if err := agents.StartWithConfig(id, startCfg); err != nil {
    return "", err
}

// Wait for agent to be ready
_ = agents.WaitReady(id)  // Error ignored

return id, nil
```

**Issue:** `WaitReady()` error is silently ignored. If an agent starts but never becomes ready:
- The caller receives success
- The agent may be in a broken state
- **Recommendation:** Either propagate this error or document that Start() returns before readiness is confirmed

---

## 3. AgentID.Parse() Edge Cases

```go
// internal/agent/agent.go
func (id AgentID) Parse() (role, rig, worker string) {
    parts := strings.Split(string(id), "/")
    switch len(parts) {
    case 1:
        return parts[0], "", ""      // "mayor" -> ("mayor", "", "")
    case 2:
        return parts[1], parts[0], ""  // "rig/witness" -> ("witness", "rig", "")
    case 3:
        return parts[1], parts[0], parts[2]  // "rig/crew/name" -> ("crew", "rig", "name")
    default:
        return "unknown", "", ""
    }
}
```

**Edge Cases:**

| Input | Output | Issue |
|-------|--------|-------|
| `""` (empty) | `("", "", "")` | Empty role - may cause problems downstream |
| `"/"` | `("", "", "")` | Single slash produces two empty parts |
| `"//"` | `("", "", "")` | Three empty parts |
| `"a/b/c/d"` | `("unknown", "", "")` | 4+ parts return "unknown" - inconsistent |
| `"rig/role/name/extra"` | `("unknown", "", "")` | Extra path segments silently fail |

**Issues:**
1. **Empty string handling:** An empty AgentID produces empty role. This silently passes through - no error returned
2. **No validation:** The function doesn't validate that results make sense (non-empty role, etc.)
3. **Inconsistent "unknown" role:** The 4+ parts case returns "unknown" while 0-3 parts return whatever was parsed
4. **No error return:** Parse() returns values only, caller cannot distinguish valid from invalid

**Recommendation:** Consider adding validation:
```go
func (id AgentID) Parse() (role, rig, worker string, err error)
```

---

## 4. Session Name Translation Edge Cases

### ToUniqueHumanReadableName

```go
// internal/session/names.go
func ToUniqueHumanReadableName(id SessionID, townRoot string) string {
    s := string(id)
    if s == "" {
        return ""
    }
    // ... parsing logic
    switch role {
    case "witness":
        baseName = fmt.Sprintf("%s%s-witness", Prefix, rig)
    case "polecats":  // Note: "polecats" not "polecat"
        if len(parts) < 3 {
            baseName = s // Invalid, return as-is
        }
    // ...
    }
}
```

**Edge Cases:**

1. **Role name mismatch:** The code checks for `"polecats"` (plural) but `AgentID.Parse()` would return `"polecat"` from the address format. The test uses `"gastown/polecats/Toast"` which works, but there may be an inconsistency with how AgentID generates addresses

2. **Unknown format fallthrough:** When the format isn't recognized, the input is returned as-is. This may cause collisions if two different inputs produce the same "as-is" output

3. **Empty parts handling:** `if len(parts) < 2` returns `s` as-is, but doesn't validate that parts[0] and parts[1] are non-empty

### FromUniqueHumanReadableName - Ambiguous Parsing

```go
// internal/session/names.go
// Check for crew: gt-<rig>-crew-<name>
if len(parts) >= 3 && parts[1] == "crew" {
    name := strings.Join(parts[2:], "-")
    return SessionID(fmt.Sprintf("%s/crew/%s", rig, name)), true
}

// Default: polecat gt-<rig>-<name>
name := strings.Join(parts[1:], "-")
return SessionID(fmt.Sprintf("%s/polecats/%s", rig, name)), true
```

**Ambiguity Issue:**
- A session named `gt-myrig-crew-bob` is parsed as crew
- A session named `gt-myrig-bob-crew` is parsed as polecat with name "bob-crew"
- A polecat named "crew" would produce `gt-myrig-crew` which is ambiguous

**Test coverage shows awareness:** The round-trip tests verify known formats work, but don't test ambiguous cases.

---

## 5. Callback Execution Order & Race Conditions

### Callback Ordering

```go
// internal/agent/start_config.go - StartWithConfig()

// 1. Create the session
if _, err := a.sess.Start(...); err != nil { ... }

// 2. Run Agents-level callback first (if any)
if a.config.OnSessionCreated != nil {
    if err := a.config.OnSessionCreated(a.sess, sessionID); err != nil { ... }
}

// 3. Run per-start callback (if any)
if cfg.OnCreated != nil {
    if err := cfg.OnCreated(a.sess, sessionID); err != nil { ... }
}

// 4. Wait for agent to be ready (non-blocking)
go a.doWaitForReady(id)

return nil
```

**Ordering guarantees:**
1. Session is created before any callback runs
2. Agents-level callback runs before per-start callback
3. Both callbacks complete before readiness waiting begins

**Potential Race Condition:**

```go
go a.doWaitForReady(id)
return nil
```

The `doWaitForReady` goroutine starts but doesn't synchronize with the return. If the caller immediately tries to interact with the agent:
- The agent may not be ready
- The `WaitReady()` method exists for explicit synchronization
- **This is documented behavior** - callers must use `WaitReady()` if they need guaranteed readiness

### Respawn Race Condition

```go
// internal/agent/agent.go - Respawn()
if err := a.sess.Respawn(sessionID, command); err != nil {
    return fmt.Errorf("respawning session: %w", err)
}

// Only reached for remote handoff (respawning a different agent)
go a.doWaitForReady(id)

return nil
```

**Good:** The comment explicitly documents that for self-handoff, the code after `Respawn()` never executes because the process is terminated.

**Issue:** For remote handoff, there's a window between `Respawn()` returning and `doWaitForReady()` starting where the agent state is undefined.

---

## 6. Error Propagation Patterns

### Errors Properly Propagated

| Location | Error | Handling |
|----------|-------|----------|
| `StartWithConfig` | session start failure | Wrapped and returned |
| `StartWithConfig` | callback failure | Wrapped and returned (with cleanup) |
| `StartWithConfig` | zombie cleanup failure | Wrapped and returned |
| `Stop` | session stop failure | Wrapped and returned |
| `Respawn` | session not running | `ErrNotRunning` returned |
| `Respawn` | get start command failure | Wrapped and returned |
| `TownSessions.resolveSession` | session not found | `ErrSessionNotFound` returned |

### Errors Silently Ignored

| Location | Error | Why Ignored | Risk |
|----------|-------|-------------|------|
| `Exists()` | sess.Exists() | Fall through to false | Low - fails safe |
| `StartWithConfig` | Stop() cleanup | Original error more important | Medium - orphan sessions |
| `factory.Start` | WaitReady() | Non-blocking by design | Low - documented |
| `doWaitForReady` | StartupHook() | Explicitly non-fatal | Low - documented |
| `Respawn` | ClearHistory() | Non-fatal cleanup | Low |
| `IsRunning` | resolveSession() | Returns false on error | Low - fails safe |

---

## 7. Double/Stub Test Coverage

The test doubles have been updated to match the new interface:

### Double (in-memory fake)
```go
// OnCreated callback handling
if cfg.OnCreated != nil {
    _ = cfg.OnCreated(nil, "")  // Note: passes nil for Sessions!
}
```

**Issue:** The Double passes `nil` for the Sessions parameter in callbacks. Tests that use callbacks expecting a real Sessions will fail. This is documented as a limitation.

### AgentsStub (error injection)
```go
func (s *AgentsStub) StartWithConfig(id AgentID, cfg StartConfig) error {
    if s.StartErr != nil {
        return s.StartErr
    }
    return s.Agents.StartWithConfig(id, cfg)
}
```

**Good:** The stub properly supports error injection for the new interface.

---

## 8. Summary of Findings

### High Priority
1. **WaitReady error ignored in factory.Start()** - Agents may appear started but be non-functional

### Medium Priority
2. **AgentID.Parse() lacks validation** - Empty/malformed IDs silently produce potentially invalid results
3. **Cleanup errors discarded** - When callback fails and cleanup also fails, only callback error is visible
4. **Session name ambiguity** - Polecats named "crew" or with hyphens may cause parsing ambiguity

### Low Priority (Documented Behavior)
5. **Callback receives nil Sessions in Double** - Test limitation, documented
6. **Non-blocking readiness wait** - Design choice, WaitReady() available for synchronization
7. **Exists() semantic change** - Now includes zombie filtering, may affect callers

### Recommendations
1. Add `AgentID.Validate()` method or error return to `Parse()`
2. Log cleanup errors in callback failure paths
3. Consider returning `WaitReady()` error from `factory.Start()` or add explicit documentation
4. Add test cases for ambiguous session name parsing
5. Consider using a more structured session name format to avoid parsing ambiguity (e.g., `gt:rig:role:name` instead of `gt-rig-role-name`)
