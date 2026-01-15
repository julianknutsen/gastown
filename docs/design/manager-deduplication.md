# Manager Deduplication Design

## Overview

The role managers (mayor, deacon, witness, refinery) share significant common code patterns. This document proposes a design to extract shared logic into a base abstraction while preserving role-specific behavior.

## Current State (After Sessions Simplification)

The architecture has been simplified:
- **Sessions → Agents → Managers** (no intermediate Processes layer)
- Managers take `session.Sessions` directly
- `IsRunning` and `WaitFor` are on the Sessions interface

### Manager Categories

| Category | Managers | Common Fields |
|----------|----------|---------------|
| **Town-level** | mayor, deacon | townRoot, agentName, sess |
| **Rig-level** | witness, refinery | rig, workDir |
| **Worker-level** | polecat, crew | rig, git, (various) |

### Accidental Differences: Witness/Refinery vs Mayor/Deacon

Analysis reveals that witness and refinery managers manually implement what mayor/deacon
get automatically via `agent.Agents`:

| Aspect | Mayor/Deacon (Unified) | Witness/Refinery (Manual) |
|--------|------------------------|---------------------------|
| **DI** | Takes `session.Sessions` | Creates `tmux.NewTmux()` internally |
| **Session Creation** | Via `agent.Agents.Start()` | Direct `t.NewSession()` / `t.Start()` |
| **Zombie Detection** | Handled by Agents | Manual duplicate code |
| **AlreadyRunning** | Handled by Agents | Manual duplicate code |
| **State File** | None | `witness.json` / `refinery.json` |

**Recommendation**: Migrate witness/refinery to use `agent.Agents` pattern for consistency.

### Shared Code in Start()

All role managers follow this pattern in `Start()`:

```go
func (m *Manager) Start() error {
    // 1. Ensure directory exists
    os.MkdirAll(workDir, 0755)

    // 2. Ensure runtime settings (Claude, OpenCode, etc.)
    claude.EnsureSettingsForRole(workDir, role)

    // 3. Build startup command
    config.BuildAgentStartupCommandWithAgentOverride(...)

    // 4. Create Agents instance and start
    agents := agent.New(sess, agent.FromPreset(agentName))
    agentID, err := agents.Start(sessionName, workDir, command)

    // 5. Set environment variables
    for k, v := range envVars {
        sess.SetEnv(sessionID, k, v)
    }

    // 6. Apply theming (role-specific)
    if t, ok := sess.(*tmux.Tmux); ok {
        t.ConfigureGasTownSession(sessionID, theme, rig, name, role)
    }

    // 7. Wait for readiness
    agents.WaitReady(agentID)

    // 8. Send nudges (role-specific)
    // ...

    return nil
}
```

### Differences Between Managers

| Aspect | Mayor | Deacon | Witness | Refinery |
|--------|-------|--------|---------|----------|
| **Session name** | gt-mayor | gt-deacon | gt-{rig}-witness | gt-{rig}-refinery |
| **Work dir** | {town}/mayor | {town}/deacon | {rig}/witness | {rig}/refinery |
| **Theme** | MayorTheme | DeaconTheme | WitnessTheme | RefineryTheme |
| **Nudges** | cold-start + propulsion | gt prime | startup beacon | startup beacon |
| **Env vars** | Role=mayor, TownRoot | Role=deacon, TownRoot | Role=witness, Rig, TownRoot | Role=refinery, Rig, TownRoot |

## Proposed Design

### Option A: Base Manager with Composition

```go
// agent/role_manager.go

// RoleConfig defines role-specific behavior
type RoleConfig struct {
    Role        string                    // "mayor", "deacon", "witness", "refinery"
    SessionName func() string             // Returns session name
    WorkDir     func() string             // Returns working directory
    Theme       func() tmux.Theme         // Returns theming
    EnvVars     func() map[string]string  // Returns environment variables
    OnStarted   func(agentID AgentID) error // Post-start hook (nudges, etc.)
}

// RoleManager provides common lifecycle operations for role-based agents
type RoleManager struct {
    procs     *processes.Processes
    agentName string
    config    RoleConfig
}

func NewRoleManager(procs *processes.Processes, agentName string, cfg RoleConfig) *RoleManager

func (m *RoleManager) Start() error
func (m *RoleManager) Stop() error
func (m *RoleManager) IsRunning() (bool, error)
func (m *RoleManager) Status() (*session.Info, error)
func (m *RoleManager) SessionName() string
```

**Usage in mayor package:**

```go
package mayor

type Manager struct {
    *agent.RoleManager
    townRoot string
}

func NewManager(townRoot, agentName string, procs *processes.Processes) *Manager {
    cfg := agent.RoleConfig{
        Role:        "mayor",
        SessionName: session.MayorSessionName,
        WorkDir:     func() string { return filepath.Join(townRoot, "mayor") },
        Theme:       tmux.MayorTheme,
        EnvVars: func() map[string]string {
            return config.AgentEnv(config.AgentEnvConfig{
                Role:     "mayor",
                TownRoot: townRoot,
            })
        },
        OnStarted: func(agentID agent.AgentID) error {
            // Mayor-specific nudges
            return nil
        },
    }
    return &Manager{
        RoleManager: agent.NewRoleManager(procs, agentName, cfg),
        townRoot:    townRoot,
    }
}
```

### Option B: Interface with Shared Implementation

```go
// agent/role.go

// Role defines the contract for role-based agents
type Role interface {
    Name() string
    SessionName() string
    WorkDir() string
    Theme() tmux.Theme
    EnvVars() map[string]string
    OnStarted(agentID AgentID) error
}

// StartRole starts any Role using the common lifecycle
func StartRole(procs *processes.Processes, agentName string, role Role) (AgentID, error) {
    // Common implementation...
}

func StopRole(procs *processes.Processes, role Role) error
func IsRoleRunning(procs *processes.Processes, role Role) bool
func RoleStatus(procs *processes.Processes, role Role) (*session.Info, error)
```

**Usage in mayor package:**

```go
package mayor

type mayorRole struct {
    townRoot string
}

func (r *mayorRole) Name() string        { return "mayor" }
func (r *mayorRole) SessionName() string { return session.MayorSessionName() }
func (r *mayorRole) WorkDir() string     { return filepath.Join(r.townRoot, "mayor") }
// ...

type Manager struct {
    procs     *processes.Processes
    agentName string
    role      *mayorRole
}

func (m *Manager) Start() error {
    return agent.StartRole(m.procs, m.agentName, m.role)
}
```

### Option C: Functional Approach with Builder

```go
// agent/builder.go

type StartOptions struct {
    Procs       *processes.Processes
    AgentName   string
    Role        string
    SessionName string
    WorkDir     string
    Theme       func() tmux.Theme
    EnvVars     map[string]string
    OnStarted   func(AgentID) error
}

func StartWithOptions(opts StartOptions) (AgentID, error)
```

**Usage:**

```go
func (m *Manager) Start() error {
    _, err := agent.StartWithOptions(agent.StartOptions{
        Procs:       m.procs,
        AgentName:   m.agentName,
        Role:        "mayor",
        SessionName: session.MayorSessionName(),
        WorkDir:     m.mayorDir(),
        Theme:       tmux.MayorTheme,
        EnvVars:     m.envVars(),
        OnStarted:   m.sendNudges,
    })
    return err
}
```

## Recommendation

**Option A (Base Manager with Composition)** is recommended because:

1. **Type safety**: Compile-time verification of configuration
2. **Embedding**: Manager types can embed RoleManager and add role-specific methods
3. **Testability**: RoleConfig can be constructed with test doubles
4. **Gradual adoption**: Existing managers can migrate incrementally
5. **Clear ownership**: Each package still owns its Manager type

## Migration Path

### Phase 0: Unify Witness/Refinery with Agent.Agents (prerequisite)

Before deduplicating, align witness/refinery with mayor/deacon pattern:

1. **Witness**:
   - Add `sess session.Sessions` parameter to NewManager
   - Replace `t := tmux.NewTmux()` with injected sessions
   - Use `agent.Agents` for start/stop/zombie detection
   - Keep state file for now (remove in Phase 5)

2. **Refinery**:
   - Same pattern as Witness
   - Keep foreground mode (deprecated but still used)

### Phase 1: Create `agent.RoleManager` with shared logic

Extract common patterns from the now-unified managers.

### Phase 2: Migrate deacon manager (simplest - no nudges)

### Phase 3: Migrate mayor manager (adds nudge handling)

### Phase 4: Migrate witness/refinery managers (rig-scoped)

### Phase 5: Cleanup

- Remove duplicated code from individual managers
- Consider removing/unifying state files (witness.json, refinery.json)

## Non-Goals

- Polecat/crew managers have different lifecycles (worker vs infrastructure)
- Rig/dog/swarm managers are not agent managers
- This design does not address agent configuration (handled by agent.FromPreset)

## Session-Level DI and Theming

### The Theming Challenge

Currently, theming requires access to tmux-specific APIs:

```go
if t, ok := sess.(*tmux.Tmux); ok {
    t.ConfigureGasTownSession(sessionID, theme, rig, name, role)
}
```

If we push DI further (higher-level creates sessions), managers can't apply theming
without the concrete tmux type.

### Options for Theming with Full DI

1. **Apply theming at session creation** (recommended)
   - Caller creates session AND applies theme before passing to manager
   - Manager is simpler - just configures the agent

2. **Type assertion with graceful degradation**
   - Manager tries `sess.(*tmux.Tmux)` and skips theming if unavailable
   - Works today, tests can use Double without theming

3. **Theming callback parameter**
   - Pass `OnSessionCreated func(SessionID)` that applies theme
   - Flexible but adds parameter noise

4. **Sessions.Themed(theme) wrapper**
   - Add theme configuration to Sessions interface
   - Pollutes interface with tmux-specific concern

### Recommended Approach

Keep **Option 2 (type assertion)** for now. The theming is non-critical and can fail
gracefully. This preserves testability while allowing full theming in production.

For future consideration: if we add alternative session backends (zellij, etc.),
we'd need a theming abstraction anyway.

## Open Questions

1. Should `RoleConfig.OnStarted` be synchronous or return a channel?
2. Should nudges be part of this abstraction or remain role-specific?
3. How to handle witness's `StartOptions` (forceRestart, initialPrompt, beadIDs)?
4. Should state files (witness.json, refinery.json) be unified or removed?
