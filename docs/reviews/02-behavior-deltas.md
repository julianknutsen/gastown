# Behavior Deltas Review: bbb768f5..HEAD

This document analyzes behavioral changes in the agent lifecycle system between commit `bbb768f5` and HEAD.

## Executive Summary

The refactoring introduces a unified `factory.Start()` function that replaces per-role manager Start methods. This consolidates startup behavior but also changes several implicit behaviors around propulsion nudges, session naming, and hook timing.

---

## 1. Agent Startup: Old vs New

### Old Architecture (Manager.Start())

Each role had its own `Manager.Start()` method with role-specific logic:

```go
// Old: crew.Manager.Start()
func (m *Manager) Start(name string, opts StartOptions) error {
    // 1. Validate name
    // 2. Get or create worker workspace
    // 3. Handle KillExisting option
    // 4. Ensure runtime settings
    // 5. Build startup beacon
    // 6. Store context for callback (m.startCrewName, m.startOpts)
    // 7. Build startup command with env vars
    // 8. Call m.agents.Start(sessionName, workDir, command)
}
```

**Callback Flow (Old):**
- `agent.New()` was configured with `OnSessionCreated` at factory creation time
- The callback had to access manager state via closure or stored fields
- Example: `crew.Manager` stored `m.startCrewName`, `m.startOpts`, `m.startWorker` for the callback

### New Architecture (factory.Start())

All roles now use `factory.Start()`:

```go
// New: factory.Start()
func Start(townRoot string, id agent.AgentID, aiRuntime string, opts ...StartOption) (agent.AgentID, error) {
    // 1. Parse AgentID to get role, rig, worker
    // 2. Compute workDir via WorkDirForID()
    // 3. Build env vars from parsed components
    // 4. Apply env overrides from options
    // 5. Ensure runtime settings
    // 6. Build startup command
    // 7. Create agents with production configuration
    // 8. Build OnCreated callback (captures context via closure)
    // 9. Call agents.StartWithConfig(id, startCfg)
    // 10. WaitReady (blocking, was async before)
}
```

**Key Differences:**
| Aspect | Old | New |
|--------|-----|-----|
| Entry point | Role-specific `Manager.Start()` | Unified `factory.Start()` |
| Callback configuration | At Agents creation time | Per-start via `StartConfig.OnCreated` |
| Context capture | Stored in manager fields | Closure captures at call time |
| WaitReady | Async goroutine in `agents.Start()` | Blocking call after `StartWithConfig()` |
| Zombie handling | Varied by role | Consistent in `StartWithConfig()` |

---

## 2. Session Naming: Old Patterns vs AgentID Addressing

### Old Pattern (Session Name Based)

```go
// Old: Each role constructed its own session names
func (m *Manager) SessionName() string {
    return fmt.Sprintf("gt-%s-witness", m.rig.Name)
}

// Old: daemon used identity-to-session conversion
func (d *Daemon) identityToSession(identity string) string {
    switch parsed.RoleType {
    case "witness":
        return fmt.Sprintf("gt-%s-%s", parsed.RigName, parsed.RoleType)
    case "crew":
        return fmt.Sprintf("gt-%s-crew-%s", parsed.RigName, parsed.AgentName)
    }
}
```

### New Pattern (AgentID Based)

```go
// New: AgentID is the canonical address
type AgentID string  // Format: "role" | "rig/role" | "rig/role/name"

const MayorAddress AgentID = "mayor"
const DeaconAddress AgentID = "deacon"

func WitnessAddress(rig string) AgentID {
    return AgentID(fmt.Sprintf("%s/witness", rig))
}

func CrewAddress(rig, name string) AgentID {
    return AgentID(fmt.Sprintf("%s/crew/%s", rig, name))
}
```

**Translation Layer:**

The `session.TownSessions` layer now handles translation:
- `ToUniqueHumanReadableName(id, townRoot)` - AgentID to tmux session name
- `FromUniqueHumanReadableName(name, townRoot)` - tmux name back to AgentID

**Behavioral Delta:**
- Town suffix is now appended for collision prevention (e.g., `hq-mayor-abc123`)
- Crew sessions use `gt-<rig>-c-<name>` instead of `gt-<rig>-crew-<name>` (shorter prefix)
- Polecat sessions still use `gt-<rig>-<name>` pattern

---

## 3. Callback/Hook Timing Changes

### OnSessionCreated Timing

**Old:**
```go
// Called synchronously after session creation, before Start() returns
agentCfg := agent.FromPreset(agentName).WithOnSessionCreated(func(sess, id) error {
    return mgr.OnSessionCreated(sess, id)
})
agents := agent.New(t, agentCfg)
```

**New:**
```go
// Per-start callback, explicit in StartConfig
startCfg := agent.StartConfig{
    WorkDir:   workDir,
    Command:   startupCmd,
    EnvVars:   envVars,
    OnCreated: buildSessionConfigurer(id, envVars),  // Per-start closure
}
agents.StartWithConfig(id, startCfg)
```

**Key Timing Change:** Two callbacks can now run:
1. Agents-level `Config.OnSessionCreated` (if configured)
2. Per-start `StartConfig.OnCreated`

They run in that order, both synchronously before WaitReady.

### Startup Hook (Readiness Detection)

**Old:**
```go
// Started async in goroutine
go a.doWaitForReady(sessionID)
```

**New:**
```go
// doWaitForReady called async, but WaitReady blocks at the end of Start()
go a.doWaitForReady(id)  // Still async in StartWithConfig
// BUT factory.Start() then calls:
_ = agents.WaitReady(id)  // Blocking!
```

**Behavioral Delta:** `factory.Start()` now blocks until the agent is ready, whereas the old manager-based `Start()` methods returned immediately and let readiness happen in background.

---

## 4. Side Effect Changes

### Environment Variables

**Old:** Env vars were set two ways:
1. Prepended to command string: `GT_ROLE=... GT_RIG=... claude ...`
2. Set via tmux `set-environment` in OnSessionCreated callback

**New:** Same two-phase approach, but consolidated:
1. `config.AgentEnv()` called in `factory.Start()`
2. `buildSessionConfigurer()` calls `t.SetEnvVars(sessionID, envVars)`

**New Env Var:** `BEADS_NO_DAEMON` is now set for all rig-level agents:
```go
if rigName != "" {
    envCfg.Rig = rigName
    envCfg.BeadsNoDaemon = true // Rig-level agents don't use daemon
}
```

### Theming

**Old:** Each role had custom theming logic in its OnSessionCreated:
```go
// In crew.Manager.OnSessionCreated
t.ConfigureGasTownSession(id, tmux.AssignTheme(m.rig.Name), m.rig.Name, m.startCrewName, "crew")
```

**New:** Centralized in `buildSessionConfigurer()`:
```go
// Compute theming parameters based on AgentID components
if rigName == "" {
    // Town-level: use role-specific theme
    switch role {
    case constants.RoleMayor:
        theme = tmux.MayorTheme()
    case constants.RoleDeacon:
        theme = tmux.DeaconTheme()
    }
} else {
    theme = tmux.AssignTheme(rigName)
}
```

**New Behavior:** Mayor and Deacon now have explicit theme functions (`MayorTheme()`, `DeaconTheme()`) instead of `DefaultTheme()`.

### Role-Specific Hooks

**Old:** Set in role manager callbacks:
```go
// crew.Manager.OnSessionCreated
t.SetCrewCycleBindings(string(id))

// polecat set via different mechanism
```

**New:** Centralized in `buildSessionConfigurer()`:
```go
switch role {
case constants.RolePolecat:
    t.SetPaneDiedHook(sessionID, fmt.Sprintf("%s/%s", rigName, worker))
case constants.RoleCrew:
    t.SetCrewCycleBindings("")  // Empty string - sets global bindings
}
```

---

## 5. Propulsion/Nudge Mechanism Changes

### CRITICAL: Propulsion Nudges Removed from factory.Start()

**Old (daemon lifecycle restart):**
```go
// In daemon.restartSession():
// GUPP: Gas Town Universal Propulsion Principle
_ = session.StartupNudge(d.tmux, sessionName, session.StartupNudgeConfig{
    Recipient: recipient,
    Sender:    "deacon",
    Topic:     "lifecycle-restart",
})

// Wait for beacon processing
time.Sleep(2 * time.Second)

// Send propulsion nudge
_ = d.tmux.NudgeSession(sessionName, session.PropulsionNudgeForRole(parsed.RoleType, workDir))
```

**New (daemon uses factory.Start()):**
```go
// In daemon.restartAgent():
func (d *Daemon) restartAgent(identity string) error {
    // ...
    case "witness":
        _, err := factory.Start(d.config.TownRoot, witnessID, agentName)
        return err
    // ...
}
```

**Behavioral Delta:** The new `factory.Start()` does NOT send propulsion nudges!
- No `StartupNudge` for predecessor discovery
- No `PropulsionNudge` for autonomous execution
- The 2-second wait between nudges is gone

**Impact:** Agents started via `factory.Start()` will NOT automatically begin autonomous work unless they have other mechanisms (hooks, mail polling) to trigger execution.

### GUPP Violation Detection

**Old:**
```go
if d.tmux.IsClaudeRunning(sessionName) {
    // Check if stuck...
}
```

**New:**
```go
polecatID := agentpkg.PolecatAddress(rigName, polecatName)
running := factory.Agents(d.config.TownRoot).Exists(polecatID)
if running {
    // Check if stuck...
}
```

**Behavioral Delta:** GUPP detection now uses `factory.Agents().Exists()` which includes zombie detection (checks both session existence AND process liveness). This is stricter - zombie sessions won't trigger false GUPP violations.

---

## 6. Implicit to Explicit Behavior Changes

### Made Explicit

| Previously Implicit | Now Explicit |
|---------------------|--------------|
| Session name format | `AgentID.Parse()` extracts role/rig/worker |
| Callback context capture | Closure captures at call time, no manager state |
| Zombie detection | `Exists()` checks both session AND process |
| WorkDir resolution | `WorkDirForID()` centralizes all lookup logic |
| Runtime settings path | `runtime.EnsureSettingsForRole()` called in one place |

### Made Implicit (or Removed)

| Previously Explicit | Now Implicit/Removed |
|---------------------|---------------------|
| Propulsion nudges after restart | **REMOVED** - agents must self-start |
| Per-role startup beacon | Simplified to `WithTopic()` option only |
| Pre-sync for workspace | Moved to role bead config, not in factory.Start() |
| Agent name resolution | Caller must resolve and pass `aiRuntime` |

---

## 7. Summary of Critical Behavioral Changes

1. **Startup blocking:** `factory.Start()` blocks until agent is ready; old managers returned immediately.

2. **Propulsion nudges removed:** Agents restarted via factory won't receive startup/propulsion nudges. This could break autonomous work pickup.

3. **Zombie filtering:** `Agents.Exists()` now checks process liveness, not just session existence. More reliable but stricter.

4. **Callback per-start:** OnCreated callback can now be different per start, enabling context capture via closures instead of manager state.

5. **Town suffix:** Session names now include town suffix for multi-town collision prevention.

6. **Daemon delegates to factory:** All daemon lifecycle restarts now go through `factory.Start()`, ensuring consistent env vars and theming but losing the propulsion nudge mechanism.

---

## Recommendations

1. **Verify propulsion mechanism:** Ensure agents have alternative ways to start autonomous work (hooks, mail polling, /resume behavior).

2. **Test multi-town:** Verify session name translation works correctly with town suffixes.

3. **Measure startup latency:** The blocking `WaitReady()` call may slow down batch startups.

4. **Check zombie handling:** Verify the stricter `Exists()` check doesn't cause false negatives for slow-starting agents.
