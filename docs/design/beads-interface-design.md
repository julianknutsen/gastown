# Beads Interface Design

## Executive Summary

This document specifies the complete Beads interface API that will enable all 221 direct `bd` command calls to be routed through a testable interface with a high-fidelity Double.

## Current State Analysis

### Existing `beads.Beads` Wrapper

The `internal/beads/beads.go` file already provides a partial wrapper covering:

| Method | bd Command | Status |
|--------|-----------|--------|
| List | `bd list` | ✅ Implemented |
| ListByAssignee | `bd list --assignee` | ✅ Implemented |
| GetAssignedIssue | `bd list --status --assignee` | ✅ Implemented |
| Ready | `bd ready` | ✅ Implemented |
| ReadyWithType | `bd ready --label` | ✅ Implemented |
| Show | `bd show` | ✅ Implemented |
| ShowMultiple | `bd show [ids...]` | ✅ Implemented |
| Blocked | `bd blocked` | ✅ Implemented |
| Create | `bd create` | ✅ Implemented |
| CreateWithID | `bd create --id` | ✅ Implemented |
| Update | `bd update` | ✅ Implemented |
| Close | `bd close` | ✅ Implemented |
| CloseWithReason | `bd close --reason` | ✅ Implemented |
| Release | `bd update --status=open --assignee=` | ✅ Implemented |
| AddDependency | `bd dep add` | ✅ Implemented |
| RemoveDependency | `bd dep remove` | ✅ Implemented |
| Sync | `bd sync` | ✅ Implemented |
| SyncFromMain | `bd sync --from-main` | ✅ Implemented |
| GetSyncStatus | `bd sync --status` | ✅ Implemented |
| Stats | `bd stats` | ✅ Implemented |
| IsBeadsRepo | File check | ✅ Implemented |
| Run | Generic `bd <args>` | ✅ Implemented |

### Direct bd Calls NOT Using Wrapper (61 files, 221 calls)

Categorized by subcommand:

#### 1. Daemon Operations (7 locations)
- `bd daemon health --json` - Health check
- `bd daemon --start` - Start daemon
- `bd daemon --status` - Check status
- `bd daemon --health` - Health endpoint

Files: `internal/beads/daemon.go`, `internal/doctor/bd_daemon_check.go`

#### 2. Config Operations (9 locations)
- `bd config get <key>` - Get config value
- `bd config set <key> <value>` - Set config value

Files: `internal/doctor/config_check.go`, `internal/doctor/rig_check.go`, `internal/rig/manager.go`, `internal/cmd/init.go`, `internal/cmd/install.go`

#### 3. Init Operations (5 locations)
- `bd init --prefix <prefix>` - Initialize beads repo
- `bd init --quiet` - Silent initialization

Files: `internal/doctor/rig_check.go`, `internal/rig/manager.go`, `internal/cmd/install.go`, tests

#### 4. Migrate Operations (4 locations)
- `bd migrate --update-repo-id` - Update repo fingerprint
- `bd migrate --update-repo-id --yes` - Non-interactive

Files: `internal/doctor/bd_daemon_check.go`, `internal/doctor/repo_fingerprint_check.go`, `internal/rig/manager.go`, `internal/cmd/install.go`

#### 5. Label Operations (1 location)
- `bd label add <id> <label>` - Add label to issue

Files: `internal/doctor/beads_check.go`

#### 6. Molecule Operations (6 locations)
- `bd mol seed --patrol` - Seed patrol molecule
- `bd mol wisp gc` - Garbage collect wisps
- `bd mol wisp create <proto> --actor` - Create wisp from proto
- `bd mol wisp list --all --json` - List all wisps
- `bd mol current <id> --json` - Get current step
- `bd mol catalog` - List available prototypes

Files: `internal/doctor/wisp_check.go`, `internal/rig/manager.go`, `internal/cmd/prime_molecule.go`, `internal/cmd/patrol_helpers.go`, `internal/cmd/costs.go`, `internal/cmd/sling_formula.go`

#### 7. Gate Operations (4 locations)
- `bd gate show <id> --json` - Show gate status
- `bd gate wait <id> --notify <agent>` - Wait for gate with notification

Files: `internal/cmd/gate.go`, `internal/cmd/park.go`, `internal/cmd/resume.go`

#### 8. Swarm Operations (6 locations)
- `bd swarm status <id> --json` - Get swarm status

Files: `internal/swarm/manager.go`, `internal/cmd/swarm.go`

#### 9. Agent State Operations (3 locations)
- `bd agent state <bead> <state>` - Set agent state

Files: `internal/cmd/deacon.go`, `internal/cmd/agent_state.go`

#### 10. Formula Operations (3 locations)
- `bd formula show <name>` - Show formula definition
- `bd cook <formula>` - Execute formula

Files: `internal/cmd/sling_formula.go`, `internal/cmd/sling_helpers.go`, `internal/cmd/sling.go`

#### 11. Doctor Operations (1 location)
- `bd doctor --json` - Run diagnostics

Files: `internal/doctor/repo_fingerprint_check.go`

#### 12. Prime Operations (1 location)
- `bd prime` - Get PRIME.md content

Files: `internal/cmd/prime.go`

#### 13. Slot Operations (1 location - tests only)
- `bd slot show <id>` - Show slot info

Files: `internal/cmd/install_integration_test.go`

#### 14. Version Operations (1 location)
- `bd version` - Get bd version

Files: `internal/deps/beads.go`

#### 15. Delete Operations (1 location)
- `bd delete <ids...>` - Delete issues

Files: `internal/cmd/crew_lifecycle.go`

#### 16. Burn Operations (1 location)
- `bd burn <args>` - Cost tracking

Files: `internal/cmd/costs.go`

#### 17. Reopen Operations (1 location)
- `bd reopen <id>` - Reopen closed issue

Files: `internal/cmd/convoy.go`

#### 18. Bond/Track/Leg Operations (formula-related)
- `bd leg add <formula> <step>` - Add leg to formula
- `bd track <id>` - Track dependency
- `bd bond <parent> <child>` - Create parent-child bond

Files: `internal/cmd/formula.go`, `internal/cmd/sling.go`

#### 19. Comment Operations (1 location)
- `bd comment <id> <message>` - Add comment

Files: `internal/cmd/formula.go`

---

## Interface API Design

### Organizational Structure

Files organized by logical operation group:

```
internal/beads/
├── interface.go       # BeadsOps interface definition
├── types.go          # Shared types (Issue, Options, etc.)
├── implementation.go # Real bd CLI wrapper
├── double.go         # Test double with in-memory state
├── conformance_test.go # Tests that run against both impl
├── stub.go           # BeadsStub for error injection
│
├── issue.go          # List, Show, Create, Update, Close, Delete
├── dependency.go     # AddDep, RemoveDep, Blocked, Ready
├── sync.go           # Sync, SyncFromMain, GetSyncStatus
├── config.go         # ConfigGet, ConfigSet, Init, Migrate
├── daemon.go         # DaemonStart, DaemonStatus, DaemonHealth
├── molecule.go       # MolSeed, MolCurrent, MolCatalog, WispCreate, WispList, WispGC
├── gate.go           # GateShow, GateWait
├── swarm.go          # SwarmStatus
├── formula.go        # FormulaShow, Cook, LegAdd, Track, Bond
├── agent.go          # AgentState
├── label.go          # LabelAdd, LabelRemove
├── misc.go           # Version, Doctor, Prime, Slot, Stats, Burn
```

### Complete Interface Definition

```go
// BeadsOps defines the interface for beads operations.
// All methods are tested against both the real bd CLI and the Double.
type BeadsOps interface {
    // === Issue Operations ===

    // List returns issues matching the given options.
    List(opts ListOptions) ([]*Issue, error)

    // Show returns detailed information about an issue.
    Show(id string) (*Issue, error)

    // ShowMultiple fetches multiple issues by ID in a single call.
    ShowMultiple(ids []string) (map[string]*Issue, error)

    // Create creates a new issue and returns it.
    Create(opts CreateOptions) (*Issue, error)

    // CreateWithID creates an issue with a specific ID.
    CreateWithID(id string, opts CreateOptions) (*Issue, error)

    // Update updates an existing issue.
    Update(id string, opts UpdateOptions) error

    // Close closes one or more issues.
    Close(ids ...string) error

    // CloseWithReason closes issues with a reason.
    CloseWithReason(reason string, ids ...string) error

    // Delete permanently removes issues.
    Delete(ids ...string) error

    // Reopen reopens a closed issue.
    Reopen(id string) error

    // === Dependency Operations ===

    // Ready returns issues that are ready to work (not blocked).
    Ready() ([]*Issue, error)

    // ReadyWithLabel returns ready issues filtered by label.
    ReadyWithLabel(label string, limit int) ([]*Issue, error)

    // Blocked returns issues that are blocked by dependencies.
    Blocked() ([]*Issue, error)

    // AddDependency adds a dependency: issue depends on dependsOn.
    AddDependency(issue, dependsOn string) error

    // RemoveDependency removes a dependency.
    RemoveDependency(issue, dependsOn string) error

    // === Sync Operations ===

    // Sync syncs beads with remote.
    Sync() error

    // SyncFromMain syncs beads updates from main branch.
    SyncFromMain() error

    // SyncImportOnly syncs only importing changes (no export).
    SyncImportOnly() error

    // GetSyncStatus returns the sync status without performing a sync.
    GetSyncStatus() (*SyncStatus, error)

    // === Config Operations ===

    // ConfigGet returns a config value.
    ConfigGet(key string) (string, error)

    // ConfigSet sets a config value.
    ConfigSet(key, value string) error

    // Init initializes a beads repository.
    Init(opts InitOptions) error

    // Migrate runs database migrations.
    Migrate(opts MigrateOptions) error

    // === Daemon Operations ===

    // DaemonStart starts the bd daemon.
    DaemonStart() error

    // DaemonStop stops the bd daemon.
    DaemonStop() error

    // DaemonStatus returns the daemon status.
    DaemonStatus() (*DaemonStatus, error)

    // DaemonHealth returns daemon health info.
    DaemonHealth() (*DaemonHealth, error)

    // === Molecule Operations ===

    // MolSeed seeds molecules (e.g., patrol).
    MolSeed(opts MolSeedOptions) error

    // MolCurrent returns the current step of a molecule.
    MolCurrent(moleculeID string) (*MoleculeStep, error)

    // MolCatalog lists available molecule prototypes.
    MolCatalog() ([]*MoleculeProto, error)

    // WispCreate creates a wisp from a prototype.
    WispCreate(protoID, actor string) (*Issue, error)

    // WispList lists all wisps.
    WispList(all bool) ([]*Issue, error)

    // WispGC garbage collects old wisps.
    WispGC() error

    // === Gate Operations ===

    // GateShow returns gate status.
    GateShow(gateID string) (*Gate, error)

    // GateWait waits for a gate to open, optionally notifying an agent.
    GateWait(gateID, notifyAgent string) error

    // === Swarm Operations ===

    // SwarmStatus returns swarm status.
    SwarmStatus(swarmID string) (*SwarmStatus, error)

    // === Formula Operations ===

    // FormulaShow returns formula definition.
    FormulaShow(name string) (*Formula, error)

    // Cook executes a formula.
    Cook(formulaName string) (*Issue, error)

    // LegAdd adds a leg (step) to a formula instance.
    LegAdd(formulaID, stepName string) error

    // Track tracks a dependency for a formula.
    Track(issueID, dependencyID string) error

    // Bond creates a parent-child relationship.
    Bond(parentID, childID string) error

    // === Agent Operations ===

    // AgentState sets the state of an agent bead.
    AgentState(beadID, state string) error

    // === Label Operations ===

    // LabelAdd adds a label to an issue.
    LabelAdd(id, label string) error

    // LabelRemove removes a label from an issue.
    LabelRemove(id, label string) error

    // === Miscellaneous Operations ===

    // Version returns the bd version.
    Version() (string, error)

    // Doctor runs diagnostics.
    Doctor() (*DoctorReport, error)

    // Prime returns the PRIME.md content.
    Prime() (string, error)

    // SlotShow returns slot information.
    SlotShow(id string) (*Slot, error)

    // Stats returns repository statistics.
    Stats() (string, error)

    // StatsJSON returns repository statistics as structured data.
    StatsJSON() (*RepoStats, error)

    // Burn records cost information.
    Burn(opts BurnOptions) error

    // Comment adds a comment to an issue.
    Comment(id, message string) error

    // === Repository Checks ===

    // IsBeadsRepo checks if the working directory is a beads repository.
    IsBeadsRepo() bool

    // Run executes an arbitrary bd command (escape hatch).
    Run(args ...string) ([]byte, error)
}
```

### Additional Types Needed

```go
// InitOptions for bd init
type InitOptions struct {
    Prefix string
    Quiet  bool
}

// MigrateOptions for bd migrate
type MigrateOptions struct {
    UpdateRepoID bool
    Yes          bool // Non-interactive
}

// DaemonStatus from bd daemon --status
type DaemonStatus struct {
    Running bool   `json:"running"`
    PID     int    `json:"pid,omitempty"`
    Uptime  string `json:"uptime,omitempty"`
}

// DaemonHealth from bd daemon health --json
type DaemonHealth struct {
    Status    string `json:"status"`
    Latency   int    `json:"latency_ms"`
    QueueSize int    `json:"queue_size"`
}

// MoleculeStep from bd mol current
type MoleculeStep struct {
    ID       string `json:"id"`
    Title    string `json:"title"`
    Status   string `json:"status"`
    StepNum  int    `json:"step_num"`
    StepName string `json:"step_name"`
}

// MoleculeProto from bd mol catalog
type MoleculeProto struct {
    ID          string   `json:"id"`
    Name        string   `json:"name"`
    Description string   `json:"description"`
    Steps       []string `json:"steps"`
}

// MolSeedOptions for bd mol seed
type MolSeedOptions struct {
    Patrol bool
}

// Gate from bd gate show
type Gate struct {
    ID        string   `json:"id"`
    Status    string   `json:"status"` // "open", "closed"
    Waiters   []string `json:"waiters"`
    OpenedAt  string   `json:"opened_at,omitempty"`
    ClosedAt  string   `json:"closed_at,omitempty"`
}

// SwarmStatus from bd swarm status
type SwarmStatus struct {
    ID           string         `json:"id"`
    Status       string         `json:"status"`
    TotalTasks   int            `json:"total_tasks"`
    Completed    int            `json:"completed"`
    InProgress   int            `json:"in_progress"`
    Blocked      int            `json:"blocked"`
    Workers      []*SwarmWorker `json:"workers"`
}

// SwarmWorker represents a worker in a swarm
type SwarmWorker struct {
    ID     string `json:"id"`
    Task   string `json:"task"`
    Status string `json:"status"`
}

// Formula from bd formula show
type Formula struct {
    Name        string   `json:"name"`
    Description string   `json:"description"`
    Steps       []string `json:"steps"`
    Tracked     []string `json:"tracked"`
}

// DoctorReport from bd doctor --json
type DoctorReport struct {
    Status   string          `json:"status"`
    Checks   []*DoctorCheck  `json:"checks"`
    Warnings []string        `json:"warnings"`
    Errors   []string        `json:"errors"`
}

// DoctorCheck represents a single doctor check
type DoctorCheck struct {
    Name    string `json:"name"`
    Status  string `json:"status"`
    Message string `json:"message,omitempty"`
}

// Slot from bd slot show
type Slot struct {
    ID       string `json:"id"`
    IssueID  string `json:"issue_id"`
    Agent    string `json:"agent"`
    HookedAt string `json:"hooked_at"`
}

// RepoStats from bd stats --json
type RepoStats struct {
    TotalIssues    int            `json:"total_issues"`
    OpenIssues     int            `json:"open_issues"`
    ClosedIssues   int            `json:"closed_issues"`
    IssuesByType   map[string]int `json:"issues_by_type"`
    IssuesByStatus map[string]int `json:"issues_by_status"`
}

// BurnOptions for bd burn
type BurnOptions struct {
    SessionID string
    Tokens    int
    Cost      float64
    Model     string
}
```

---

## Migration Checklist

### Phase 1: Interface Foundation
- [ ] Create `internal/beads/interface.go` with BeadsOps interface
- [ ] Create `internal/beads/types.go` with all new types
- [ ] Rename existing `beads.go` to `implementation.go`
- [ ] Make Implementation implement BeadsOps interface

### Phase 2: Double Implementation
- [ ] Create `internal/beads/double.go` with in-memory state
- [ ] Implement all BeadsOps methods on Double
- [ ] Create `internal/beads/stub.go` for error injection
- [ ] Ensure Double mimics bd quirks (exit codes, error messages)

### Phase 3: Conformance Tests
- [ ] Create `internal/beads/conformance_test.go`
- [ ] Write tests for each method category:
  - [ ] Issue operations (List, Show, Create, Update, Close, Delete, Reopen)
  - [ ] Dependency operations (Ready, Blocked, AddDep, RemoveDep)
  - [ ] Sync operations (Sync, SyncFromMain, GetSyncStatus)
  - [ ] Config operations (ConfigGet, ConfigSet, Init, Migrate)
  - [ ] Daemon operations (Start, Stop, Status, Health)
  - [ ] Molecule operations (MolSeed, MolCurrent, MolCatalog, Wisp*)
  - [ ] Gate operations (GateShow, GateWait)
  - [ ] Swarm operations (SwarmStatus)
  - [ ] Formula operations (FormulaShow, Cook, LegAdd, Track, Bond)
  - [ ] Agent operations (AgentState)
  - [ ] Label operations (LabelAdd, LabelRemove)
  - [ ] Misc operations (Version, Doctor, Prime, Slot, Stats, Burn, Comment)

### Phase 4: Implementation Expansion
Add missing methods to Implementation:
- [ ] Config operations (ConfigGet, ConfigSet)
- [ ] Init operation
- [ ] Migrate operation
- [ ] Daemon operations (all 4)
- [ ] Molecule operations (all 6)
- [ ] Gate operations (both)
- [ ] Swarm operations
- [ ] Formula operations (all 5)
- [ ] Agent operations
- [ ] Label operations (explicit, not via Update)
- [ ] Delete, Reopen, Version, Doctor, Prime, Slot, StatsJSON, Burn, Comment

### Phase 5: Consumer Migration
Migrate each file to use BeadsOps interface:
- [ ] `internal/beads/daemon.go` → Use interface
- [ ] `internal/doctor/beads_check.go`
- [ ] `internal/doctor/bd_daemon_check.go`
- [ ] `internal/doctor/config_check.go`
- [ ] `internal/doctor/repo_fingerprint_check.go`
- [ ] `internal/doctor/rig_check.go`
- [ ] `internal/doctor/wisp_check.go`
- [ ] `internal/doctor/patrol_check.go`
- [ ] `internal/daemon/lifecycle.go`
- [ ] `internal/swarm/manager.go`
- [ ] `internal/rig/manager.go`
- [ ] `internal/web/fetcher.go`
- [ ] `internal/mail/bd.go`
- [ ] `internal/plugin/recording.go`
- [ ] `internal/polecat/session_manager.go`
- [ ] `internal/crew/manager.go`
- [ ] `internal/deacon/stale_hooks.go`
- [ ] `internal/deps/beads.go`
- [ ] `internal/cmd/agent_state.go`
- [ ] `internal/cmd/convoy.go`
- [ ] `internal/cmd/costs.go`
- [ ] `internal/cmd/crew_lifecycle.go`
- [ ] `internal/cmd/deacon.go`
- [ ] `internal/cmd/formula.go`
- [ ] `internal/cmd/gate.go`
- [ ] `internal/cmd/handoff.go`
- [ ] `internal/cmd/hook.go`
- [ ] `internal/cmd/init.go`
- [ ] `internal/cmd/install.go`
- [ ] `internal/cmd/mail_announce.go`
- [ ] `internal/cmd/mail_channel.go`
- [ ] `internal/cmd/mail_queue.go`
- [ ] `internal/cmd/molecule_await_signal.go`
- [ ] `internal/cmd/molecule_step.go`
- [ ] `internal/cmd/park.go`
- [ ] `internal/cmd/patrol_helpers.go`
- [ ] `internal/cmd/polecat.go`
- [ ] `internal/cmd/polecat_identity.go`
- [ ] `internal/cmd/prime.go`
- [ ] `internal/cmd/prime_molecule.go`
- [ ] `internal/cmd/resume.go`
- [ ] `internal/cmd/rig_dock.go`
- [ ] `internal/cmd/sling.go`
- [ ] `internal/cmd/sling_batch.go`
- [ ] `internal/cmd/sling_convoy.go`
- [ ] `internal/cmd/sling_formula.go`
- [ ] `internal/cmd/sling_helpers.go`
- [ ] `internal/cmd/swarm.go`
- [ ] `internal/cmd/synthesis.go`

### Phase 6: Test Migration
- [ ] Update integration tests to use Double where appropriate
- [ ] Keep integration tests for actual bd behavior verification
- [ ] Ensure all new unit tests pass

---

## Double Implementation Notes

### State Model

```go
type Double struct {
    mu sync.RWMutex

    // Issue storage
    issues      map[string]*Issue
    nextID      int
    prefix      string

    // Dependency graph
    dependencies map[string][]string // issue -> depends_on

    // Config storage
    config map[string]string

    // Daemon state
    daemonRunning bool
    daemonPID     int

    // Molecule state
    molecules    map[string]*MoleculeInstance
    prototypes   map[string]*MoleculeProto

    // Gate state
    gates map[string]*Gate

    // Swarm state
    swarms map[string]*SwarmStatus

    // Formula state
    formulas map[string]*Formula

    // Sync state
    syncBranch   string
    syncAhead    int
    syncBehind   int
    conflicts    []string
}
```

### bd Quirks to Replicate

1. **Exit codes**: bd exits 0 even on some errors with `--no-daemon`
2. **JSON output**: Some commands return array, some return object
3. **Empty stdout with stderr**: Detect "not found" via empty stdout
4. **Prefix handling**: IDs have prefix (e.g., "hq-123", "toast-456")
5. **Status transitions**: open → in_progress → closed (or hooked/pinned)
6. **Dependency blocking**: Issue blocked if any dependency not closed
7. **Assignee format**: "rig/polecat" format (e.g., "gastown/Toast")

### Routing Behavior

The Double must handle routing correctly:
- Operations from town root use town beads (`hq-` prefix)
- Operations from rig worktree use rig beads (rig prefix)
- BEADS_DIR environment variable can override

---

## File Organization

Final file structure after migration:

```
internal/beads/
├── interface.go          # BeadsOps interface (50 lines)
├── types.go              # All shared types (200 lines)
├── implementation.go     # Real bd CLI wrapper (800 lines)
├── double.go             # Test double (600 lines)
├── stub.go               # BeadsStub for error injection (50 lines)
├── conformance_test.go   # Tests for both impl (1000+ lines)
├── resolve.go            # BEADS_DIR resolution (existing)
├── daemon.go             # Daemon-specific helpers (existing)
└── prime.go              # PRIME.md provisioning (move from beads.go)
```
