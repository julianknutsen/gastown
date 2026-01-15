# Agent Manager Consolidation Summary

This document summarizes the functional differences between agent managers before and after the consolidation, and highlights bugs fixed as a result.

## Changes Made

### 1. OnSessionCreated Callback Architecture

Added a new callback to `agent.Config` that runs immediately after session creation but before waiting for readiness:

```go
type OnSessionCreated func(sess session.Sessions, id session.SessionID) error

func (c *Config) WithOnSessionCreated(fn OnSessionCreated) *Config
```

This enables managers to:
- Set environment variables
- Apply theming
- Run any synchronous setup

The callback uses type assertion for theming, enabling graceful degradation when non-tmux sessions are injected (e.g., in tests):

```go
if t, ok := sess.(*tmux.Tmux); ok {
    theme := tmux.AssignTheme(rigName)
    _ = t.ConfigureGasTownSession(id, theme, rigName, "witness", "witness")
}
```

### 2. Witness Manager Migration

**Before:**
- Created its own `tmux.NewTmux()` internally
- Manually handled session creation via `t.Start()`
- Manually set env vars one by one
- Manually applied theming
- Manually waited for readiness (WaitForRuntimeReady, AcceptBypassPermissionsWarning, etc.)
- No zombie detection (session exists but agent dead)

**After:**
- Takes `session.Sessions` as dependency injection
- Uses `agent.Agents` for session lifecycle
- Uses `OnSessionCreated` callback for env vars and theming
- Zombie detection handled by `agent.Agents.Start()`
- Readiness waiting handled by agent package

**Signature Change:**
```go
// Before
func NewManager(r *rig.Rig, t *tmux.Tmux, agentName string) *Manager

// After
func NewManager(r *rig.Rig, sess session.Sessions, agentName string) *Manager
```

### 3. Refinery Manager Migration

**Before:**
- Created its own `tmux.NewTmux()` internally
- Manually handled session creation via `t.Start()`
- Manually set env vars one by one
- Manually applied theming
- Had custom zombie detection (check IsAgentRunning, kill and recreate)
- Manually waited for readiness

**After:**
- Takes `session.Sessions` as dependency injection
- Uses `agent.Agents` for session lifecycle
- Uses `OnSessionCreated` callback for env vars and theming
- Zombie detection centralized in `agent.Agents`
- Readiness waiting handled by agent package

**Signature Change:**
```go
// Before
func NewManager(r *rig.Rig) *Manager

// After
func NewManager(r *rig.Rig, sess session.Sessions, agentName string) *Manager
```

## Bugs Fixed

### 1. Missing Zombie Detection in Witness

**Before:** Witness manager did not detect zombie sessions (tmux session exists but Claude process dead). If a witness session died unexpectedly but the tmux session remained, starting the witness would return "already running" even though the agent was dead.

**After:** Both witness and refinery use `agent.Agents.Start()` which includes consistent zombie detection - if a session exists but the expected process isn't running, the session is killed and recreated.

### 2. Inconsistent Readiness Waiting

**Before:** Different managers implemented readiness waiting differently:
- Refinery: WaitForRuntimeReady + AcceptBypassPermissionsWarning + SleepForReadyDelay + RunStartupFallback
- Witness: Similar but could have subtle differences

**After:** All managers delegate to `agent.Agents` which provides a consistent readiness pattern through the `StartupHook` and `Checker` configuration.

### 3. Theming Failed Silently Without Graceful Degradation

**Before:** Theming code assumed `*tmux.Tmux` was always available. In test scenarios or with alternative session implementations, this could fail unexpectedly.

**After:** Theming uses type assertion with graceful degradation:
```go
if t, ok := sess.(*tmux.Tmux); ok {
    // Apply theming
}
```
Non-tmux sessions simply skip theming without error.

### 4. State Management Inconsistency

**Before:** Each manager had its own `loadState`/`saveState` implementation with duplicated JSON marshaling and file operations.

**After:** Managers use `agent.StateManager[T]` which provides:
- Consistent atomic writes
- Type-safe state loading
- Default state factory pattern

## Testing Improvements

The dependency injection pattern enables better testing:

```go
// Before: Hard to test, creates real tmux sessions
mgr := witness.NewManager(r, tmux.NewTmux(), agentName)

// After: Testable with mock sessions
mockSess := session.NewDouble()
mgr := witness.NewManager(r, mockSess, agentName)
```

## Architecture Pattern

All role managers now follow a consistent pattern:

1. **Constructor** takes `session.Sessions` for DI
2. **agents()** method creates configured `agent.Agents`
3. **onSessionCreated()** callback handles env vars and theming
4. **Start()** delegates to `agents().Start()`
5. **Stop()** delegates to `agents().Stop()`

This pattern is implemented in:
- `mayor.Manager`
- `deacon.Manager`
- `witness.Manager`
- `refinery.Manager`

## RoleManager Base (Not Implemented)

The original plan included creating a `RoleManager` base type. This was deemed unnecessary because:

1. The callback architecture already provides sufficient deduplication
2. Each manager has role-specific differences (working directory, env vars, commands)
3. Go's composition model makes embedding awkward for this use case
4. The current pattern is clear and explicit
