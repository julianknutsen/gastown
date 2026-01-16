# Distributed Agent Operation Analysis

> Analysis of inter-agent communication mechanisms and required changes for distributed operation across machines, Docker containers, or other abstractions.

## Executive Summary

Gas Town agents communicate through five primary mechanisms: **nudges** (real-time tmux injection), **beads** (git-backed issue database), **mail** (durable message queue on beads), **hooks/slots** (work assignment via beads), and **events/feed** (JSONL activity logs). All mechanisms currently assume local filesystem access and single-machine operation.

**Key finding**: The codebase has **prepared infrastructure** for distributed operation via the `internal/connection/` package, which defines a `Connection` interface abstracting local vs remote operations. However, this interface is not yet integrated into the rest of the codebase.

---

## Communication Mechanisms Overview

| Mechanism | Current Transport | Locality | Storage |
|-----------|------------------|----------|---------|
| **Nudges** | tmux send-keys | Local only | None (ephemeral) |
| **Beads** | bd CLI → SQLite + Git | Shared filesystem | `.beads/` directories |
| **Mail** | Beads issues | Shared filesystem | Town-level beads |
| **Hooks/Slots** | Beads SQLite columns | Shared filesystem | Agent bead records |
| **Events/Feed** | JSONL file append | Local filesystem | `.events.jsonl`, `.feed.jsonl` |
| **Daemon** | File locks + signals | Local process | `daemon.lock`, PID files |
| **Protocol Messages** | Mail + pattern matching | Beads + tmux | Town-level beads |

---

## Detailed Mechanism Analysis

### 1. Nudges - Real-Time Message Injection

**Location**: `internal/tmux/tmux.go:423-454`

Nudges inject messages directly into a Claude agent's tmux session conversation. This is the primary real-time notification mechanism.

**Implementation**:
```go
// NudgeSession sends a message to an agent's tmux session
func (t *Tmux) NudgeSession(session, message string) error {
    // 1. Send text in literal mode (-l flag)
    t.SendKeysLiteral(session, message)
    // 2. Wait 500ms (tested, required)
    time.Sleep(500 * time.Millisecond)
    // 3. Send Escape (handle vim INSERT mode)
    t.SendKeys(session, "Escape")
    // 4. Send Enter with retry (3 attempts, 200ms between)
    return t.SendKeysWithRetry(session, "Enter", 3, 200*time.Millisecond)
}
```

**Local dependencies**:
- tmux server running locally
- tmux socket in `/tmp` or `~/.tmux/`
- Direct subprocess execution of `tmux send-keys`

**For distributed operation**:
- Replace with message queue (agent polls for messages)
- Or RPC endpoint per machine that forwards to local tmux
- Lose synchronous delivery guarantees

---

### 2. Beads - Git-Backed Issue Database

**Location**: `internal/beads/*.go`

Beads is the primary durable storage for all work items, agent state, and mail messages. It uses SQLite locally with JSONL export for git tracking.

**Architecture**:
```
SQLite Database ←→ bd CLI ←→ JSONL Files ←→ Git
     (fast)         (IPC)      (versioned)   (sync)
```

**Key operations**:
```go
// All beads access via subprocess
cmd := exec.Command("bd", "--no-daemon", "--allow-stale", args...)
cmd.Dir = workDir
cmd.Env = append(os.Environ(), "BEADS_DIR="+beadsDir)
```

**Redirect system** (for shared beads):
- Worktrees share rig-level beads via `.beads/redirect` file
- Redirect contains relative path: `../../.beads`
- Resolution follows chain with depth limit of 3

**Local dependencies**:
- bd CLI executable
- SQLite database files
- Git repository access
- Filesystem path resolution

**For distributed operation**:
- Option A: Remote bd execution via SSH
- Option B: bd daemon with network API
- Option C: Replace with distributed database (PostgreSQL, etc.)
- Must handle: sync conflicts, network latency, offline operation

---

### 3. Mail - Durable Message Queue

**Location**: `internal/mail/*.go`

Mail is built on top of beads, providing message routing, mailing lists, work queues, and announce channels.

**Message types**:
- **Direct mail**: Fan-out to individual recipients
- **Mailing lists**: Expand to multiple recipients at send time
- **Work queues**: Single copy, workers claim via `bd update`
- **Announce channels**: Broadcast, read-only, retention-pruned

**Address formats**:
```
Direct:     gastown/Toast, mayor/, gastown/witness
Lists:      list:oncall
Queues:     queue:workers
Announces:  announce:alerts
Groups:     @town, @witnesses, @rig/gastown, @crew/gastown
```

**Local dependencies**:
- Town-level beads database (`~/.beads`)
- tmux for notification nudges
- Filesystem for config (`~/gt/settings/messaging.json`)

**For distributed operation**:
- Same requirements as beads (mail stored in beads)
- Group resolution needs distributed agent registry
- Notification delivery needs machine-aware routing

---

### 4. Hooks/Slots - Work Assignment

**Location**: `internal/beads/beads_agent.go:294-324`

Hooks are the primary mechanism for assigning work to agents. Each agent has a `hook_bead` slot that references their current work.

**Operations**:
```go
// Assign work to agent
bd slot set <agent_bead_id> hook <work_bead_id>

// Clear assignment
bd slot clear <agent_bead_id> hook
```

**GUPP Principle**: "If you find work on your hook, YOU RUN IT" - agents auto-execute hooked work without confirmation.

**Local dependencies**:
- Same as beads (slots stored in SQLite)

**For distributed operation**:
- Atomic slot updates across network
- Conflict resolution for concurrent assignments
- Consider distributed lock for hook operations

---

### 5. Events/Feed - Activity Logging

**Location**: `internal/events/events.go`, `internal/feed/curator.go`

Events provide an audit trail; feed provides a curated user-facing activity stream.

**Storage**:
```
~/gt/.events.jsonl  - Raw events (append-only)
~/gt/.feed.jsonl    - Curated feed (deduplicated, aggregated)
```

**Curator process**:
- Tails events file with 100ms poll interval
- Filters by visibility (audit vs feed)
- Deduplicates repeated events (10-30 second windows)
- Aggregates related events

**Local dependencies**:
- Local filesystem for JSONL files
- File polling (no inotify)
- Mutex for concurrent writes

**For distributed operation**:
- Replace with event streaming service (Kafka, NATS, Redis Streams)
- Or centralized event collector with HTTP POST
- Feed curation could run centrally or per-machine

---

### 6. Daemon - Process Coordination

**Location**: `internal/daemon/daemon.go`

The daemon ensures patrol agents (Deacon, Witnesses, Refineries) are running and handles lifecycle events.

**Coordination mechanisms**:
- **File lock**: `gofrs/flock` on `daemon.lock` (exclusive)
- **PID file**: `daemon.pid` for monitoring
- **State file**: `daemon.state` (atomic writes)
- **Signals**: SIGTERM, SIGKILL, SIGUSR1 (lifecycle)
- **Heartbeat**: 3-minute recovery interval

**Per heartbeat**:
1. Ensure Deacon running
2. Ensure Boot (watchdog) running
3. Check Deacon heartbeat
4. Ensure Witnesses running (per rig)
5. Ensure Refineries running (per rig)
6. Process lifecycle requests
7. Check GUPP violations
8. Check orphaned work
9. Check polecat session health

**Local dependencies**:
- tmux for session management
- File locks (don't work over NFS)
- Process signals (local only)
- Direct subprocess execution

**For distributed operation**:
- Distributed lock service (etcd, Consul, ZooKeeper)
- RPC health checks to host agents
- Signal routing via message queue
- Consider: one coordinator + host agents per machine

---

### 7. Protocol Messages - Inter-Agent Coordination

**Location**: `internal/protocol/types.go`, `internal/witness/protocol.go`

Structured messages for specific workflows (merge coordination, lifecycle events).

**Message types**:
```go
TypeMergeReady    // Witness → Refinery: branch ready
TypeMerged        // Refinery → Witness: merge succeeded
TypeMergeFailed   // Refinery → Witness: merge failed
TypeReworkRequest // Refinery → Witness: rebase needed
```

**Pattern matching** in witness for lifecycle:
- `POLECAT_DONE` - work completion
- `LIFECYCLE:Shutdown` - daemon-triggered shutdown
- `HELP:` - intervention request
- `MERGED`, `MERGE_FAILED` - refinery status
- `HANDOFF` - session continuity

**Local dependencies**:
- Mail system (beads-based)
- tmux nudges for notifications

**For distributed operation**:
- Same as mail system
- Consider dedicated protocol channel for reliability

---

## Existing Infrastructure for Distribution

### Connection Package (`internal/connection/`)

**Already implemented** - abstracts local vs remote operations:

```go
// Connection interface (connection.go:13-78)
type Connection interface {
    // Identification
    Name() string
    IsLocal() bool

    // File operations
    ReadFile(path string) ([]byte, error)
    WriteFile(path string, data []byte, perm fs.FileMode) error
    MkdirAll(path string, perm fs.FileMode) error
    Remove(path string) error
    Stat(path string) (FileInfo, error)
    Glob(pattern string) ([]string, error)
    Exists(path string) (bool, error)

    // Command execution
    Exec(cmd string, args ...string) ([]byte, error)
    ExecDir(dir, cmd string, args ...string) ([]byte, error)
    ExecEnv(env map[string]string, cmd string, args ...string) ([]byte, error)

    // Tmux operations
    TmuxNewSession(name, dir string) error
    TmuxKillSession(name string) error
    TmuxSendKeys(session, keys string) error
    TmuxCapturePane(session string, lines int) (string, error)
    TmuxHasSession(name string) (bool, error)
    TmuxListSessions() ([]string, error)
}
```

**Machine Registry** (`registry.go`):
```go
type Machine struct {
    Name     string `json:"name"`
    Type     string `json:"type"`      // "local", "ssh"
    Host     string `json:"host"`      // for ssh: user@host
    KeyPath  string `json:"key_path"`  // SSH private key path
    TownPath string `json:"town_path"` // Path to town root on remote
}
```

**Address Parsing** (`address.go`):
```go
// Format: [machine:]rig[/polecat]
// Examples:
//   - "gastown/rictus"        -> local, gastown rig, rictus polecat
//   - "vm:gastown/rictus"     -> vm machine, gastown rig, rictus polecat
//   - "vm:gastown/"           -> vm machine, gastown rig, broadcast
```

**Status**:
- `LocalConnection` fully implemented
- `SSHConnection` stubbed: `return nil, fmt.Errorf("ssh connections not yet implemented")`
- **Package not yet integrated** into rest of codebase

---

### Federation Design (`docs/design/federation.md`)

Design spec for multi-workspace coordination:

**URI schemes**:
```
hop://entity/chain/rig/issue-id     # Full work unit reference
beads://platform/org/repo/issue-id  # Cross-repo reference
gp-xyz                              # Local short form
```

**Implementation status**:
- [x] Agent identity in git commits (`GIT_AUTHOR_NAME`)
- [x] BD_ACTOR in beads operations
- [x] Workspace metadata (`.town.json`)
- [x] URI scheme defined
- [ ] Remote registration
- [ ] Cross-workspace queries
- [ ] Delegation primitives

---

## Required Changes by Priority

### High Priority - Direct Local Dependencies

| Component | File | Current | Required Change |
|-----------|------|---------|-----------------|
| Tmux sessions | `internal/tmux/tmux.go` | Local subprocess | Route through `Connection` interface |
| Nudge delivery | `tmux.go:423-454` | `tmux send-keys` | Message queue or RPC per machine |
| Process detection | `tmux.go:763-839` | `tmux list-panes`, `pgrep` | Agent heartbeat or remote health RPC |
| Session lifecycle | `daemon/lifecycle.go` | Direct tmux calls | Machine registry routing |

### Medium Priority - Filesystem State

| Component | File | Current | Required Change |
|-----------|------|---------|-----------------|
| Beads access | `internal/beads/beads.go` | `exec.Command("bd")` | Remote exec via Connection or bd network API |
| Redirect resolution | `beads_redirect.go` | Filesystem paths | Network-aware resolution |
| Events/Feed | `events/events.go`, `feed/curator.go` | JSONL append + polling | Event streaming service |

### Medium Priority - Coordination

| Component | File | Current | Required Change |
|-----------|------|---------|-----------------|
| Daemon lock | `daemon/daemon.go` | `gofrs/flock` | Distributed lock (etcd/Consul) |
| State files | `daemon/daemon.go` | Local files | Distributed state or per-machine agents |
| Heartbeat | `daemon/daemon.go:196-244` | Local poll | RPC to host agents |

### Lower Priority - Configuration

| Component | File | Current | Required Change |
|-----------|------|---------|-----------------|
| Messaging config | `~/gt/settings/messaging.json` | Local file | Replicate or centralize |
| @Group resolution | `mail/router.go` | Local beads query | Distributed agent registry |
| Escalation routing | `internal/witness/*.go` | Local beads + tmux | Machine-aware routing |

---

## Architectural Options

### Option A: SSH-Based Distribution (Minimal Change)

Complete the `SSHConnection` implementation:

```
Local Machine                    Remote Machine
┌──────────────┐    SSH         ┌──────────────┐
│ gt daemon    │ ───────────→   │ tmux server  │
│ Connection   │                │ bd CLI       │
│ interface    │                │ local files  │
└──────────────┘                └──────────────┘
```

**Implementation**:
1. Implement `SSHConnection` in `internal/connection/`
2. Update daemon to use `Connection` interface
3. Update tmux wrapper to accept `Connection`
4. Update beads wrapper for remote execution

**Pros**:
- Minimal code change
- Reuses existing abstractions
- No new infrastructure

**Cons**:
- SSH latency per operation
- No parallelism
- Single point of failure
- Connection management complexity

**Effort**: ~2-3 weeks

---

### Option B: Host Agent Architecture (Recommended)

Deploy lightweight agent per machine with RPC:

```
Central Daemon                   Host Agent (per machine)
┌──────────────┐    gRPC/HTTP   ┌──────────────┐
│ Coordinator  │ ─────────────→ │ Local tmux   │
│ Message Queue│                │ Local beads  │
│ Session Reg. │ ←───────────── │ Health check │
└──────────────┘                └──────────────┘
```

**New components**:
1. `internal/host_agent/` - RPC server per machine
2. Message queue for nudges (Redis/NATS)
3. Session registry (etcd or custom)
4. Health aggregation service

**Pros**:
- Better performance (persistent connections)
- Parallel operations
- Resilient (agents operate independently)
- Clean separation of concerns

**Cons**:
- New infrastructure (message queue, registry)
- More complex deployment
- State synchronization challenges

**Effort**: ~4-6 weeks

---

### Option C: Full Distributed Database (Most Scalable)

Replace beads with distributed database:

```
All Machines                     Central Services
┌──────────────┐                ┌──────────────┐
│ Agent process│ ─────────────→ │ PostgreSQL   │
│              │                │ Event stream │
│              │                │ Lock service │
└──────────────┘                └──────────────┘
```

**Changes**:
1. Replace beads with PostgreSQL/CockroachDB
2. Event streaming via Kafka/NATS
3. Distributed locks via etcd
4. Remove git-based sync

**Pros**:
- True horizontal scaling
- No single point of failure
- Standard infrastructure

**Cons**:
- Significant rewrite
- Lose git-native benefits
- Complex migration path
- Operational overhead

**Effort**: ~8-12 weeks

---

## Recommended Implementation Path

### Phase 1: SSH Connection (Foundation)

1. Implement `SSHConnection` type in `internal/connection/registry.go`
2. Add connection pooling for SSH sessions
3. Create integration tests for remote operations
4. Document machine registration workflow

### Phase 2: Integration (Incremental)

1. Update `internal/tmux/tmux.go` to accept `Connection` parameter
2. Update `internal/daemon/daemon.go` to route via Connection
3. Update `internal/beads/beads.go` for remote execution
4. Add machine selection to CLI commands (`--machine=vm`)

### Phase 3: Host Agents (Performance)

1. Design host agent RPC protocol
2. Implement `internal/host_agent/` package
3. Add message queue for async nudges
4. Implement session registry
5. Migrate daemon to use host agents

### Phase 4: Distributed State (Scale)

1. Evaluate distributed database options
2. Design migration path from beads
3. Implement event streaming
4. Add distributed locking
5. Full testing and rollout

---

## Key Files to Modify

| File | Purpose | Changes |
|------|---------|---------|
| `internal/connection/registry.go:179` | SSH implementation | Implement `SSHConnection` type |
| `internal/tmux/tmux.go` | Session management | Accept `Connection` parameter |
| `internal/daemon/daemon.go` | Coordination | Use `Connection` interface, distributed locks |
| `internal/beads/beads.go` | Issue database | Remote execution via Connection |
| `internal/mail/router.go` | Message routing | Machine-aware address resolution |
| `internal/events/events.go` | Activity logging | Event streaming integration |
| `internal/feed/curator.go` | Feed curation | Subscribe to event stream |

---

## Summary

The codebase is **architecturally prepared** for distributed operation:

1. **Connection interface** exists and defines the abstraction needed
2. **Address parsing** supports cross-machine references
3. **Federation design** provides conceptual framework
4. **Clean separation** between coordination (daemon) and execution (tmux/beads)

**Main gaps**:
1. SSH Connection implementation (stubbed)
2. Integration of Connection interface (not yet used)
3. Distributed alternatives for file-based state
4. Network transport for real-time nudges

**Recommended approach**: Start with SSH-based distribution (Option A) for basic multi-machine support, then evolve toward host agents (Option B) for better performance. This provides a working distributed system quickly while building toward a more robust architecture.
