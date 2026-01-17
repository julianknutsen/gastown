# Code Review: Factory Consolidation Refactor (bbb768f5..HEAD)

**Date**: 2026-01-17
**Reviewer**: Claude Opus 4.5
**Commits**: bbb768f5 → HEAD (e81d9696)

## Overview

This is a significant architectural refactor that consolidates agent lifecycle management into a unified `factory.Start()` pattern and introduces `AgentID` as a logical address type. The changes touch 112 files with ~7,700 lines removed and ~5,200 added.

### Key Changes

1. **New `AgentID` type**: Logical addresses like `"mayor"`, `"gastown/witness"`, `"gastown/crew/joe"` that abstract away tmux session naming
2. **Centralized `factory.Start()`**: Single entry point for starting any agent with consistent env vars, theming, and hooks
3. **Deleted manager packages**: `mayor.Manager`, `deacon.Manager` removed - their Start() logic moved to factory
4. **`TownSessions` refactored**: Now translates logical IDs to unique human-readable tmux session names
5. **`agent.Respawn()`**: New method for atomic handoff (reuses original command)
6. **`agent.Self()`**: Returns current agent's ID from GT_* environment variables

---

## Behavior Changes That May Be Untested

### 1. **Startup Beacon Missing for Some Agents** (HIGH RISK)
**File**: `internal/factory/factory.go:86-94`

```go
func buildCommand(role, aiRuntime string, cfg *startConfig) string {
    beacon := ""
    if cfg.topic != "" {
        beacon = cfg.topic
    }
    cmd := config.BuildAgentCommand(aiRuntime, beacon)
    ...
}
```

In the old code, agents like witness/refinery received startup nudges (GUPP: Gas Town Universal Propulsion Principle). The new `factory.Start()` only passes a beacon for crew when `WithTopic()` is used. **Singleton agents (mayor, deacon, witness, refinery) may no longer receive their startup propulsion nudge.**

The old `daemon/lifecycle.go` had explicit post-start nudges:
```go
// Old code (deleted):
_ = session.StartupNudge(d.tmux, sessionName, ...)
_ = d.tmux.NudgeSession(sessionName, session.PropulsionNudgeForRole(parsed.RoleType, workDir))
```

**Impact**: Agents started via `factory.Start()` may wait indefinitely for input if they rely on the propulsion nudge.

### 2. **Pre-Sync Workspace Removed** (MEDIUM RISK)
**File**: `internal/daemon/lifecycle.go` (deleted code)

Old daemon had:
```go
if needsPreSync {
    d.logger.Printf("Pre-syncing workspace for %s at %s", identity, workDir)
    d.syncWorkspace(workDir)
}
```

The new `restartAgent()` in daemon calls `factory.Start()` which doesn't do workspace syncing. Agents with git worktrees (polecats) may start with stale code after a restart.

### 3. **`agent.Exists()` Behavior Change** (MEDIUM RISK)
**File**: `internal/agent/agent.go:265-280`

```go
func (a *Implementation) Exists(id AgentID) bool {
    // ...
    if len(a.config.ProcessNames) == 0 {
        return true  // NEW: assumes session = running if no process names
    }
    return a.sess.IsRunning(sessionID, a.config.ProcessNames...)
}
```

Previously `Exists()` just checked session existence. Now it checks if the agent process is alive. **This affects callers that expected `Exists()` to return true for zombie sessions.**

Callers affected:
- `internal/cmd/start.go:264` - `startConfiguredCrew` now relies on this for zombie detection
- `internal/polecat/session_manager.go:135` - Stop() checks `Exists()` first

### 4. **Handoff Self-Respawn Path** (HIGH RISK)
**File**: `internal/cmd/handoff.go:173-183`

```go
// Write handoff marker for successor detection
if cwd, err := os.Getwd(); err == nil {
    markerPath := filepath.Join(runtimeDir, constants.FileHandoffMarker)
    _ = os.WriteFile(markerPath, []byte(targetID), 0644)  // WAS: currentSession
}

return agents.Respawn(targetID)
```

The handoff marker now writes the **AgentID** (logical name) instead of the tmux session name. If any code reading this marker expects the session name format, it will break.

Also, `Respawn()` relies on `GetStartCommand()` from Sessions:
```go
command, err := a.sess.GetStartCommand(sessionID)
```

**This method is new and may not be implemented correctly for all session backends.** The `TownSessions` wrapper calls through, but the underlying `tmux.Tmux` implementation needs to correctly return the original startup command.

### 5. **Session Name Translation Edge Cases**
**File**: `internal/session/names.go:89-145`

`ToUniqueHumanReadableName()` maps logical IDs to tmux session names:
```go
case "polecats":
    if len(parts) < 3 {
        baseName = s // Invalid, return as-is
    } else {
        name := parts[2]
        baseName = fmt.Sprintf("%s%s-%s", Prefix, rig, name)  // "gt-rig-name"
    }
```

But the **inverse function** `FromUniqueHumanReadableName()` has a different assumption:
```go
// Default: polecat gt-<rig>-<name>
name := strings.Join(parts[1:], "-")
return SessionID(fmt.Sprintf("%s/polecats/%s", rig, name)), true
```

This assumes any unknown session is a polecat, which could misclassify sessions. The tests in `names_test.go` may not cover all edge cases with hyphens in names.

---

## Potential Bugs Indirectly Fixed

### 1. **Zombie Detection for Crew**
**Old**: `startConfiguredCrew()` had inline zombie detection with `t.IsAgentRunning()`
**New**: Uses `crew.Manager.Start()` → `factory.Start()` which has zombie handling in `StartWithConfig()`

This consolidation likely fixes inconsistent zombie handling across different start paths.

### 2. **Env Var Duplication**
Old pattern prepended env vars AND set them via `t.SetEnvVars()`. New pattern in `buildSessionConfigurer()` sets them once via `t.SetEnvVars()` after session creation, avoiding potential duplication.

### 3. **Crew Cycle Bindings Race**
Old `crew.Manager.OnSessionCreated()` could race with command execution. New `buildSessionConfigurer()` runs synchronously before the command starts.

---

## Code Quality Issues

### 1. **Unused Parameter**
**File**: `internal/polecat/session_manager.go:27`
```go
func NewSessionManager(agents agent.Agents, r *rig.Rig, _, townRoot string) *SessionManager {
```
Blank identifier `_` for an unused parameter - should be removed or documented.

### 2. **Duplicate Comment**
**File**: `internal/crew/manager.go:454`
```go
// Stop terminates a crew member's tmux session.
// Stop stops a crew member's session.
```

### 3. **Missing Error Handling**
**File**: `internal/factory/factory.go:101-102`
```go
_ = agents.Stop(id, true)  // Ignores error
```
When `WithKillExisting()` is set, stop errors are silently ignored.

---

## Test Coverage Gaps

1. **Deleted tests not replaced**:
   - `internal/deacon/manager_test.go` (284 lines) - deacon lifecycle testing gone
   - `internal/mayor/manager_test.go` (306 lines) - mayor lifecycle testing gone
   - `internal/cmd/polecat_cycle_test.go` (118 lines)

2. **New code without tests**:
   - `factory.Start()` has minimal test coverage in `factory_test.go` (newly added, check coverage)
   - `agent.Respawn()` needs integration tests for self-handoff path
   - `agent.Self()` environment variable parsing needs tests

3. **Conformance tests need update**: `internal/agent/conformance_test.go` changed but needs to verify new `StartWithConfig` behavior matches `Start` semantics.

---

## Recommendations

1. **Add startup propulsion nudge** to `factory.Start()` for roles that need it (or document why it's no longer needed)

2. **Add integration test** for handoff self-respawn path - verify `GetStartCommand()` returns correct command with env vars

3. **Add test coverage** for `ToUniqueHumanReadableName` / `FromUniqueHumanReadableName` round-trip with edge cases (hyphens in names, crew vs polecat disambiguation)

4. **Document the behavior change** in `Exists()` - callers expecting zombie sessions to return `true` may be affected

5. **Consider re-adding workspace pre-sync** or document why it's safe to remove
