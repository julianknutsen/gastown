# Architectural Verification Report

Post-merge verification that the refactored code correctly implements the agent abstraction architecture.

## Architecture Summary

The target architecture has three layers:

```
┌─────────────────────────────────────────────────────────┐
│                  Role Managers                           │
│   (mayor, deacon, witness, refinery, polecat, crew)     │
│   - Use m.agents.Start(), Stop(), Exists()              │
│   - Take agent.Agents via constructor (DI)               │
└─────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────┐
│                    agent.Agents                          │
│   - Start(), Stop(), Exists(), WaitReady()              │
│   - Zombie detection centralized here                   │
│   - Created by factory with proper config               │
└─────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────┐
│                  session.Sessions                        │
│   - Low-level tmux operations                           │
│   - Start(), Stop(), Exists(), Send(), Capture()        │
└─────────────────────────────────────────────────────────┘
```

## Verification Results

### ✅ Core Managers Correctly Use agent.Agents

| Manager | Takes Agents | Uses m.agents.* | Status |
|---------|--------------|-----------------|--------|
| mayor.Manager | ✅ `agents agent.Agents` | ✅ Start/Stop/Exists | **PASS** |
| deacon.Manager | ✅ `agents agent.Agents` | ✅ Start/Stop/Exists | **PASS** |
| witness.Manager | ✅ `agents agent.Agents` | ✅ Start/Stop/Exists | **PASS** |
| refinery.Manager | ✅ `agents agent.Agents` | ✅ Start/Stop/Exists | **PASS** |
| polecat.SessionManager | ✅ `agents agent.Agents` | ✅ Start/Stop/Exists | **PASS** |
| crew.Manager | ✅ `agents agent.Agents` | ✅ Start/Stop/Exists | **PASS** |

### ✅ Factory Creates Properly Configured Managers

All factory functions follow the correct pattern:

```go
func DeaconManager(townRoot, agentName string) *deacon.Manager {
    envVars := config.AgentEnv(...)
    sessionCfg := tmux.SessionConfigForRole("deacon", "").WithEnvVars(envVars)
    t := tmux.NewTmux().WithSessionConfig(sessionCfg)
    agents := agent.New(t, agent.FromPreset(agentName).WithEnvVars(envVars))
    return deacon.NewManager(townRoot, agentName, agents)
}
```

Verified factory functions:
- ✅ `factory.MayorManager()`
- ✅ `factory.DeaconManager()`
- ✅ `factory.WitnessManager()`
- ✅ `factory.RefineryManager()`
- ✅ `factory.PolecatSessionManager()`
- ✅ `factory.CrewManager()`
- ✅ `factory.GenericAgents()` (for status/health checks)

### ✅ Test Doubles Exist

| Layer | Double | Stub | Location |
|-------|--------|------|----------|
| agent.Agents | ✅ agent.Double | ✅ agent.AgentsStub | internal/agent/double.go |
| session.Sessions | ✅ session.Double | - | internal/session/double.go |

### ✅ Daemon Uses Factory Functions (FIXED)

The daemon had inline agent creation code that duplicated factory logic.

**Before (violation):**
```go
// ensureDeaconRunning() - duplicated factory logic inline
envVars := config.AgentEnv(config.AgentEnvConfig{Role: "deacon", TownRoot: d.config.TownRoot})
sessionCfg := tmux.SessionConfigForRole("deacon", "").WithEnvVars(envVars)
t := tmux.NewTmux().WithSessionConfig(sessionCfg)
agents := agent.New(t, agent.FromPreset(agentName).WithEnvVars(envVars))
mgr := deacon.NewManager(d.config.TownRoot, agentName, agents)
```

**After (fixed):**
```go
// ensureDeaconRunning() - uses factory
mgr := factory.DeaconManager(d.config.TownRoot, agentName)
```

Functions fixed:
- ✅ `daemon.ensureDeaconRunning()` - now uses `factory.DeaconManager()`
- ✅ `daemon.ensureWitnessRunning()` - now uses `factory.WitnessManager()`
- ✅ `daemon.ensureRefineryRunning()` - already used `factory.RefineryManager()`

### ✅ Legitimate Direct tmux.NewTmux() Usages

The following usages are **architecturally correct** - they don't create agents, they perform utility operations:

| Location | Usage | Justification |
|----------|-------|---------------|
| `factory/*.go` | Create agents | **This is the correct place** |
| `daemon/daemon.go:91` | Session existence checks | Daemon's own tmux instance for monitoring |
| `doctor/*.go` | Health diagnostics | Need direct tmux access for checks |
| `cmd/*.go` | List/attach/status/nudge | CLI operations on existing sessions |
| `boot/boot.go` | Boot watchdog | Special case - not a regular agent |
| `connection/local.go` | Connection handler | Session interaction |
| `mail/router.go` | Status checks | Query session state |
| `deacon/stale_hooks.go` | Session checks | Cleanup operations |
| `witness/handlers.go` | Polecat operations | Session management |
| `polecat/pending.go` | Status checks | Query polecat sessions |

**Rule:** Direct `tmux.NewTmux()` is allowed for:
1. Factory functions that create agents
2. Read-only operations (list, status, capture)
3. Doctor/diagnostic checks
4. Session interaction utilities (send, attach, nudge)

**Rule:** Direct `tmux.NewTmux()` is NOT allowed for:
1. Role manager lifecycle (Start/Stop) - must use agent.Agents via factory

## Summary

✅ **ARCHITECTURE VERIFIED**

All role managers correctly use the agent.Agents interface for lifecycle operations. The daemon has been fixed to use factory functions. Test doubles exist at both agent and session levels, enabling unit testing without real tmux.
