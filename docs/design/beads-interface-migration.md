# BeadsOps Interface Migration Guide

This document describes the migration from `beads.New()` to the explicit `beads.ForTown()` / `beads.ForRig()` constructors, and provides guidance on using the BeadsOps interface effectively.

## Why the Change

The original `beads.New()` constructor was ambiguous about intent:

```go
// Before: What database does this target?
bd := beads.New()
```

The new constructors make the intent explicit:

```go
// Town-level operations (hq- prefix beads)
bd := beads.ForTown(townRoot)

// Rig-level operations (gt-, ap- prefix beads)
bd := beads.ForRig(rigPath)
```

### Key Differences

| Aspect | `ForTown(townRoot)` | `ForRig(rigPath)` |
|--------|---------------------|-------------------|
| Primary use | Town beads (agents, mail, roles) | Rig beads (tasks, molecules) |
| workDir | Set to townRoot | Set to rigPath |
| townRoot | Set to townRoot | Auto-detected from rigPath |
| Cross-rig routing | Yes | Yes (via auto-detected townRoot) |

Both constructors enable cross-rig routing when townRoot is available.

## Migration Patterns

### Pattern 1: Town-Level Operations

Use `ForTown()` when working with town-level beads (hq- prefix):

```go
// Before
bd := beads.New()
agents, _ := bd.List(beads.ListOptions{Type: "agent"})

// After
bd := beads.ForTown(townRoot)
agents, _ := bd.List(beads.ListOptions{Type: "agent"})
```

Common use cases:
- Agent beads (`hq-agent-*`)
- Mail and messages
- Role beads
- Convoys and synthesis

### Pattern 2: Rig-Level Operations

Use `ForRig()` when working with rig-specific beads:

```go
// Before
bd := beads.New()
tasks, _ := bd.List(beads.ListOptions{Type: "task"})

// After
bd := beads.ForRig(rigPath)
tasks, _ := bd.List(beads.ListOptions{Type: "task"})
```

Common use cases:
- Tasks and issues (`gt-*`, `ap-*`)
- Molecules and swarms
- Merge requests

### Pattern 3: Cross-Rig Operations

Both constructors support cross-rig operations via ID prefix routing:

```go
// From gastown rig, access ai_platform bead
bd := beads.ForRig("/path/to/gastown")
apIssue, _ := bd.Show("ap-123")  // Routes to ai_platform database
```

The `routedImpl()` method handles this transparently for ID-based operations.

## Routing Behavior

### Operations That Route by ID Prefix

These operations automatically route to the correct rig based on bead ID:

- `Show(id)`
- `ShowMultiple(ids)`
- `Update(id, opts)`
- `Close(ids...)`
- `CloseWithReason(reason, ids...)`
- `CloseWithOptions(opts, ids...)`
- `Delete(ids...)`
- `Reopen(id)`
- `AddDependency(issue, dependsOn)`
- `RemoveDependency(issue, dependsOn)`
- `LabelAdd(id, label)`
- `LabelRemove(id, label)`
- `Comment(id, message)`

### Operations That Use workDir

These operations use the workDir set by the constructor:

- `List(opts)` - Lists from current database
- `Create(opts)` - Creates in current database
- `Ready()` - Checks current database
- `Blocked()` - Checks current database

## Error Handling

### Before: String Parsing

```go
_, err := bd.Show(id)
if bdErr, ok := err.(*beads.Error); ok {
    if bdErr.ContainsError("not found") {
        // Handle not found
    }
}
```

### After: Typed Errors

```go
_, err := bd.Show(id)
if errors.Is(err, beads.ErrNotFound) {
    // Handle not found
}
```

Available error constants:
- `beads.ErrNotFound` - Issue not found
- `beads.ErrNotInstalled` - bd CLI not installed

## Run() Removal

The `Run(args ...string)` escape hatch has been removed from the interface. All operations must use typed methods:

```go
// Before: Escape hatch
out, _ := bd.Run("show", id, "--json")

// After: Use typed method
issue, _ := bd.Show(id)
```

If you need an operation not covered by the interface, add a new method rather than using raw command execution.

## Testing with Double

The `beads.Double` test double implements the full `BeadsOps` interface for unit testing:

### Basic Usage

```go
func TestMyFunction(t *testing.T) {
    d := beads.NewDouble()  // Creates with "test-" prefix

    issue, _ := d.Create(beads.CreateOptions{
        Title: "Test Issue",
        Type:  "task",
    })

    // Your test logic here

    result, _ := d.Show(issue.ID)
    assert.Equal(t, "Test Issue", result.Title)
}
```

### Multi-Rig Testing

```go
func TestCrossRigOperations(t *testing.T) {
    d := beads.NewDoubleWithPrefix("gt")
    d.AddDatabase("ap")
    d.ConfigureRoute("/path/gastown", "gt")
    d.ConfigureRoute("/path/ai_platform", "ap")

    // Create in gastown
    d.SetWorkDir("/path/gastown")
    gtIssue, _ := d.Create(beads.CreateOptions{Title: "GT Task"})
    // gtIssue.ID == "gt-1"

    // Create in ai_platform
    d.SetWorkDir("/path/ai_platform")
    apIssue, _ := d.Create(beads.CreateOptions{Title: "AP Task"})
    // apIssue.ID == "ap-1"

    // Cross-rig access works from any workDir
    d.SetWorkDir("/path/gastown")
    fetched, _ := d.Show(apIssue.ID)  // Routes to ap database
    assert.Equal(t, "AP Task", fetched.Title)
}
```

### Dependency Testing

```go
func TestDependencyBlocking(t *testing.T) {
    d := beads.NewDouble()

    blocker, _ := d.Create(beads.CreateOptions{Title: "Blocker"})
    blocked, _ := d.Create(beads.CreateOptions{Title: "Blocked"})

    d.AddDependency(blocked.ID, blocker.ID)

    ready, _ := d.Ready()
    assert.NotContains(t, issueIDs(ready), blocked.ID)

    d.Close(blocker.ID)

    ready, _ = d.Ready()
    assert.Contains(t, issueIDs(ready), blocked.ID)
}
```

### Setup Helpers

The Double provides setup helpers for complex scenarios:

```go
d.AddPrototype(proto)           // Add molecule prototype
d.SetMoleculeStep(id, step)     // Set molecule execution state
d.AddFormula(formula)           // Add formula definition
d.AddGate(gate)                 // Add gate
d.AddSwarm(swarm)               // Add swarm status
d.SetDaemonRunning(true)        // Set daemon state
d.SetSyncStatus(branch, ahead, behind, conflicts)
```

### Direct Access for Assertions

```go
issue := d.GetIssueByID(id)     // Direct access without Show()
all := d.GetAllIssues()         // Get all issues for verification
```

## New Interface Methods

The following methods were added as part of this migration:

| Method | Purpose |
|--------|---------|
| `ShowMultiple(ids)` | Batch fetch issues (performance) |
| `MessageThread(threadID)` | Query threaded messages |
| `CloseWithOptions(opts, ids...)` | Close with reason/session/force |
| `MolBurn(ids...)` | Batch delete wisps/molecules |
| `AgentState(id, state)` | Set agent lifecycle state |

## ListOptions Enhancements

New filter options available:

```go
opts := beads.ListOptions{
    Status:       "open",
    Type:         "task",
    Label:        "priority:high",
    Labels:       []string{"team:platform", "sprint:42"},
    Assignee:     "gastown/Toast",
    Parent:       "gt-epic-1",
    Tag:          "escalation",
    CreatedAfter: "-24h",
    All:          true,  // Include closed
    MolType:      "swarm",
    DescContains: "urgent",
    SortBy:       "created",
    SortAsc:      false,
    Limit:        100,
}
```

## CreateOptions Enhancements

New create options:

```go
opts := beads.CreateOptions{
    Title:       "New Task",
    Type:        "task",
    Priority:    2,
    Description: "Details here",
    Parent:      "gt-epic-1",
    Assignee:    "gastown/Toast",
    Actor:       "gastown/Mayor",
    Ephemeral:   true,  // Wisp
    Labels:      []string{"team:platform"},
    MolType:     "swarm",
    // Event fields
    EventCategory: "session.ended",
    EventActor:    "gastown/Toast",
    EventPayload:  `{"duration": 3600}`,
    EventTarget:   "gt-task-1",
}
```

## Removed APIs

| Removed | Replacement |
|---------|-------------|
| `beads.New()` | `beads.ForTown()` or `beads.ForRig()` |
| `Run(args...)` | Use typed methods |
| `ListOptions.NoAssignee` | Filter results for empty assignee |
| `internal/mail/bd.go` | Use BeadsOps directly |
