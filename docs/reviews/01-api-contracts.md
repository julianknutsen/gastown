# API Contract Changes Review

**Commits:** `bbb768f5..HEAD` (2 commits)
- `5ce06a9f` - refactor: migrate commands to use Agents interface instead of raw tmux
- `e81d9696` - refactor: consolidate agent lifecycle into factory.Start()

This document reviews all interface, type, and function signature changes that affect callers.

---

## 1. Agents Interface (`internal/agent/agent.go`)

### 1.1 Removed Methods

| Method | Old Signature | Status |
|--------|---------------|--------|
| `Start` | `Start(name, workDir, command string) (AgentID, error)` | **REMOVED** |
| `SessionID` | `SessionID(id AgentID) session.SessionID` | **REMOVED** |

### 1.2 Added Methods

| Method | Signature | Purpose |
|--------|-----------|---------|
| `StartWithConfig` | `StartWithConfig(id AgentID, cfg StartConfig) error` | Explicit configuration startup |
| `Respawn` | `Respawn(id AgentID) error` | Atomic kill+restart for handoff |
| `Capture` | `Capture(id AgentID, lines int) (string, error)` | Get recent output |
| `CaptureAll` | `CaptureAll(id AgentID) (string, error)` | Get entire scrollback |
| `List` | `List() ([]AgentID, error)` | List all agents |
| `Attach` | `Attach(id AgentID) error` | Attach to agent session |

### 1.3 Changed Method Semantics

**`Exists(id AgentID) bool`**
- **Old:** Checks if session exists
- **New:** Checks if session exists AND process is alive (zombie detection)

### 1.4 Breaking Change: Start Pattern

```go
// OLD: Name-based start, returns ID
id, err := agents.Start("mayor", workDir, command)

// NEW: ID-based start with explicit config
err := agents.StartWithConfig(agent.MayorAddress, agent.StartConfig{
    WorkDir:   workDir,
    Command:   command,
    EnvVars:   envVars,
    OnCreated: callback,
})
```

**Key differences:**
1. ID is now input, not output (caller provides `AgentID`)
2. Configuration is bundled in `StartConfig` struct
3. Returns `error` only (no ID return - caller already has it)
4. Per-start callbacks via `StartConfig.OnCreated`

---

## 2. AgentID Type (`internal/agent/agent.go`)

### 2.1 Enhanced Type Definition

```go
// OLD: Opaque identifier (doc comment only)
type AgentID string

// NEW: Semantic address with methods
type AgentID string

func (id AgentID) String() string
func (id AgentID) Parse() (role, rig, worker string)
func (id AgentID) Role() string
func (id AgentID) Rig() string
func (id AgentID) Worker() string
```

### 2.2 Address Constants and Factories

```go
// Singleton constants
const MayorAddress AgentID = "mayor"
const DeaconAddress AgentID = "deacon"
const BootAddress AgentID = "boot"

// Factory functions for rig-level agents
func WitnessAddress(rig string) AgentID     // "rigname/witness"
func RefineryAddress(rig string) AgentID    // "rigname/refinery"
func PolecatAddress(rig, name string) AgentID // "rigname/polecat/name"
func CrewAddress(rig, name string) AgentID  // "rigname/crew/name"
```

### 2.3 Self-Identification

```go
// NEW: Agent can identify itself from environment
func Self() (AgentID, error)
var ErrUnknownRole = errors.New("unknown or missing GT_ROLE")
```

---

## 3. StartConfig Type (NEW)

**File:** `internal/agent/start_config.go`

```go
type StartConfig struct {
    WorkDir   string                        // Working directory
    Command   string                        // Command to execute
    EnvVars   map[string]string             // Per-start env vars (merged with Config.EnvVars)
    OnCreated OnSessionCreated              // Per-start callback
}
```

**Callback type:**
```go
type OnSessionCreated func(sess session.Sessions, id session.SessionID) error
```

---

## 4. Sessions Interface (`internal/session/session.go`)

### 4.1 Added Methods

| Method | Signature | Purpose |
|--------|-----------|---------|
| `Respawn` | `Respawn(id SessionID, command string) error` | Atomic kill+restart |
| `CaptureAll` | `CaptureAll(id SessionID) (string, error)` | Full scrollback capture |
| `GetStartCommand` | `GetStartCommand(id SessionID) (string, error)` | Get original command |
| `Attach` | `Attach(id SessionID) error` | Blocking attach |
| `SwitchTo` | `SwitchTo(id SessionID) error` | Switch client (in tmux) |

---

## 5. Deleted Types

### 5.1 Manager Types Removed

| Package | Type | Replacement |
|---------|------|-------------|
| `internal/mayor` | `Manager` | `factory.Start()` + `factory.Agents()` |
| `internal/deacon` | `Manager` | `factory.Start()` + `factory.Agents()` |

### 5.2 Deleted Errors (moved/consolidated)

```go
// OLD: Per-package errors
mayor.ErrNotRunning
mayor.ErrAlreadyRunning
deacon.ErrNotRunning
deacon.ErrAlreadyRunning
witness.ErrNotRunning
witness.ErrAlreadyRunning
refinery.ErrNotRunning
refinery.ErrAlreadyRunning

// NEW: Consolidated in agent package
agent.ErrNotRunning
agent.ErrAlreadyRunning
```

---

## 6. Factory Package Changes (`internal/factory/factory.go`)

### 6.1 Removed Functions

| Function | Old Signature |
|----------|---------------|
| `MayorManager` | `MayorManager(townRoot, agentName string) *mayor.Manager` |
| `DeaconManager` | `DeaconManager(townRoot, agentName string) *deacon.Manager` |
| `GenericAgents` | `GenericAgents() agent.Agents` |

### 6.2 Added Functions

```go
// Universal agent accessor
func Agents(townRoot string) agent.Agents

// Universal agent start
func Start(townRoot string, id agent.AgentID, aiRuntime string, opts ...StartOption) (agent.AgentID, error)

// Work directory helpers
func WorkDirForRole(townRoot, role string) (string, error)
func WorkDirForID(townRoot string, id agent.AgentID) (string, error)
```

### 6.3 Added Start Options

```go
type StartOption func(*startConfig)

func WithTopic(topic string) StartOption           // Crew: startup topic
func WithInteractive() StartOption                 // Remove --dangerously-skip-permissions
func WithKillExisting() StartOption                // Stop existing first
func WithEnvOverrides(map[string]string) StartOption // Additional env vars
```

### 6.4 Changed Manager Constructors

**WitnessManager:**
```go
// OLD: Package-level function
factory.WitnessManager(r *rig.Rig, townRoot, agentName string, envOverrides ...string) *witness.Manager

// NEW: Factory method
factory.New(townRoot).WitnessManager(r *rig.Rig, aiRuntime string, envOverrides ...string) *witness.Manager
```

**RefineryManager:**
```go
// OLD
factory.RefineryManager(r *rig.Rig, townRoot, agentName string) *refinery.Manager

// NEW
factory.New(townRoot).RefineryManager(r *rig.Rig, aiRuntime string) *refinery.Manager
```

**PolecatSessionManager:**
```go
// OLD
factory.PolecatSessionManager(r *rig.Rig, agentName string) *polecat.SessionManager

// NEW
factory.New(townRoot).PolecatSessionManager(r *rig.Rig, aiRuntime string) *polecat.SessionManager
```

**CrewManager:**
```go
// OLD
factory.CrewManager(r *rig.Rig, townRoot, agentName string) *crew.Manager

// NEW
factory.New(townRoot).CrewManager(r *rig.Rig, aiRuntime string) *crew.Manager
```

---

## 7. Manager Interface Changes

### 7.1 witness.NewManager

```go
// OLD
func NewManager(r *rig.Rig, agents agent.Agents, agentName string) *Manager

// NEW
func NewManager(agents agent.Agents, r *rig.Rig, _ string) *Manager
```

**Breaking:** Parameter order changed, `agentName` now unused (placeholder `_`).

### 7.2 refinery.NewManager

```go
// OLD
func NewManager(r *rig.Rig, agents agent.Agents, agentName string) *Manager

// NEW
func NewManager(agents agent.Agents, r *rig.Rig, _ string) *Manager
```

### 7.3 crew.NewManager

```go
// OLD
func NewManager(r *rig.Rig, g *git.Git, agents agent.Agents, agentName string) *Manager

// NEW
func NewManager(agents agent.Agents, r *rig.Rig, g *git.Git, _, townRoot string) *Manager
```

**Breaking:**
- Parameter order changed
- Added `townRoot` parameter
- `agentName` now unused

### 7.4 polecat.NewSessionManager

```go
// OLD
func NewSessionManager(agents agent.Agents, sess session.Sessions, r *rig.Rig, agentName string) *SessionManager

// NEW
func NewSessionManager(agents agent.Agents, r *rig.Rig, _, townRoot string) *SessionManager
```

**Breaking:**
- Removed `sess session.Sessions` parameter
- Added `townRoot` parameter
- `agentName` now unused

---

## 8. Command Migration Patterns

### 8.1 Starting Singleton Agents

```go
// OLD (mayor/deacon)
mgr, err := getMayorManager(agentOverride)
if err := mgr.Start(); err != nil {
    if err == mayor.ErrAlreadyRunning { ... }
}

// NEW
townRoot, _ := workspace.FindFromCwdOrError()
agentName := config.ResolveAgentForRole("mayor", townRoot, "", override)
if _, err := factory.Start(townRoot, agent.MayorAddress, agentName); err != nil {
    if err == agent.ErrAlreadyRunning { ... }
}
```

### 8.2 Starting Rig-Level Agents

```go
// OLD (witness)
mgr, err := getWitnessManager(rigName, agentOverride, envOverrides...)
if err := mgr.Start(); err != nil { ... }

// NEW
id := agent.WitnessAddress(rigName)
opts := []factory.StartOption{}
if len(envOverrides) > 0 {
    opts = append(opts, factory.WithEnvOverrides(envMap))
}
if _, err := factory.Start(townRoot, id, agentName, opts...); err != nil { ... }
```

### 8.3 Stopping Agents

```go
// OLD
mgr.Stop()

// NEW
agents := factory.Agents(townRoot)
agents.Stop(id, true)  // true = graceful
```

---

## 9. Test Double Changes (`internal/agent/double.go`)

### 9.1 Removed Methods

```go
// OLD
func (d *Double) Start(name, workDir, command string) (AgentID, error)
```

### 9.2 Added Methods

```go
// NEW
func (d *Double) StartWithConfig(id AgentID, cfg StartConfig) error
func (d *Double) Respawn(id AgentID) error
func (d *Double) Capture(id AgentID, lines int) (string, error)
func (d *Double) CaptureAll(id AgentID) (string, error)
func (d *Double) List() ([]AgentID, error)
func (d *Double) Attach(id AgentID) error
```

### 9.3 Added Test Helpers

```go
type StopCall struct {
    ID       AgentID
    Graceful bool
}

// Available via field access: double.stopCalls
```

---

## 10. Summary of Breaking Changes

| Category | Change | Migration |
|----------|--------|-----------|
| `Agents.Start` removed | Must use `StartWithConfig` | Use `StartWithConfig(id, cfg)` |
| `Agents.SessionID` removed | Internal implementation detail | Use `session.SessionID(id)` if needed |
| `mayor.Manager` deleted | Lifecycle consolidated | Use `factory.Start()` + `factory.Agents()` |
| `deacon.Manager` deleted | Lifecycle consolidated | Use `factory.Start()` + `factory.Agents()` |
| Manager constructors reordered | `agents` is now first param | Update call sites |
| Manager `agentName` param | Now unused placeholder | Can pass empty string |
| Factory functions | Now methods on `*Factory` | Use `factory.New(townRoot).XxxManager(...)` |
| Error constants moved | Per-package errors removed | Use `agent.ErrAlreadyRunning` etc. |

---

## 11. Non-Breaking Additions

These additions extend the API without breaking existing code:

1. **AgentID methods** - `Parse()`, `Role()`, `Rig()`, `Worker()`, `String()`
2. **Address constants** - `MayorAddress`, `DeaconAddress`, `BootAddress`
3. **Address factories** - `WitnessAddress()`, `RefineryAddress()`, etc.
4. **Self-identification** - `agent.Self()` for runtime identity
5. **StartOption pattern** - Functional options for `factory.Start()`
6. **Sessions methods** - `Respawn()`, `CaptureAll()`, `GetStartCommand()`, `Attach()`, `SwitchTo()`
