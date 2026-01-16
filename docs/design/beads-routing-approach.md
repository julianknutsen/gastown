# Beads Routing Approach

## Overview

The beads system uses a multi-database architecture where different contexts (town, rigs, crew worktrees) have their own beads databases. Routing determines which database an operation should target.

## Key Concepts

### 1. Database Locations

- **Town beads**: `<townRoot>/.beads/` - Stores hq-* prefixed issues (Mayor, Deacon, role beads)
- **Rig beads**: `<townRoot>/<rigName>/mayor/rig/.beads/` - Canonical location for rig issues
- **Redirects**: `<rigRoot>/.beads/redirect` - Points to canonical location

### 2. routes.jsonl

The `routes.jsonl` file is stored in the town-level `.beads` directory and maps prefixes to rig paths:

```json
{"prefix": "hq-", "path": "."}
{"prefix": "gt-", "path": "gastown/mayor/rig"}
{"prefix": "ap-", "path": "ai_platform/mayor/rig"}
{"prefix": "bd-", "path": "beads/mayor/rig"}
```

This enables:
- Determining which rig a bead belongs to based on its ID prefix
- Finding the correct working directory for operations like `bd update`
- Validating prefix uniqueness (no two rigs should use same prefix)

### 3. Resolution Flow

When performing a beads operation:

1. **Input**: Working directory and/or bead ID
2. **Resolve beadsDir**:
   - If BEADS_DIR env var is set, use it
   - Otherwise, find `.beads/` in workDir
   - Follow redirect chain if `redirect` file exists
3. **Determine prefix**:
   - From bead ID: `ExtractPrefix("ap-123")` → `"ap-"`
   - From routes.jsonl: `GetPrefixForRig(townRoot, "ai_platform")` → `"ap"`
4. **Execute operation** against resolved database

## BeadsOps Interface Approach

### Current Design

BeadsOps **does not** include routing methods. This is intentional:

```go
// Routes are managed separately - not part of BeadsOps
routes, _ := beads.LoadRoutes(beadsDir)
prefix := beads.GetPrefixForRig(townRoot, rigName)
rigPath := beads.GetRigPathForPrefix(townRoot, prefix)

// BeadsOps is created with the resolved directory
impl := beads.New(rigPath)
```

### Rationale

1. **Separation of Concerns**: Routing is a coordination concern, not a database operation
2. **Single-Database Instance**: Each BeadsOps instance operates on ONE database
3. **Pre-resolved Context**: Callers determine the correct database before creating BeadsOps
4. **Testing**: Double can be tested without filesystem routing complexity

### Where Routing Happens

| Component | Routing Responsibility |
|-----------|----------------------|
| Manager (Mayor, Deacon, etc.) | Determines workDir based on role |
| Agent | Uses hookWorkDir from spawn context |
| Command handlers | Parse prefix from bead ID, resolve rig |
| BeadsOps | None - operates on given workDir |

## Route Management Functions

These are standalone functions, not part of BeadsOps:

```go
// Loading routes
routes, err := beads.LoadRoutes(beadsDir)

// Getting prefix for a rig
prefix := beads.GetPrefixForRig(townRoot, "gastown")

// Getting rig path from prefix
rigPath := beads.GetRigPathForPrefix(townRoot, "gt-")

// Resolving hook directory for bd update
hookDir := beads.ResolveHookDir(townRoot, beadID, hookWorkDir)

// Managing routes
beads.AppendRoute(townRoot, beads.Route{Prefix: "new-", Path: "new_rig/mayor/rig"})
beads.RemoveRoute(townRoot, "old-")
```

## Test Doubles

### Double (Single Database)

The basic `Double` simulates one beads database:

```go
double := beads.NewDouble()  // or NewDoubleWithPrefix("custom")
double.Create(beads.CreateOptions{...})  // ID: custom-1
```

### MultiDouble (Multi-Database)

For testing routing scenarios:

```go
multi := beads.NewMultiDouble()
multi.InitDatabase("/town/.beads", "hq")
multi.InitDatabase("/rig/mayor/rig/.beads", "rig")

townOps := beads.NewRoutingDouble(multi, "/town")
rigOps := beads.NewRoutingDouble(multi, "/rig")

// Operations are isolated to their respective databases
townOps.Create(...)  // ID: hq-1
rigOps.Create(...)   // ID: rig-1
```

### Redirect Following

MultiDouble follows the same redirect chain as the real implementation:

```go
// Given crew/.beads/redirect -> ../../.beads
// And rig/.beads/redirect -> mayor/rig/.beads
crewOps := beads.NewRoutingDouble(multi, "/rig/crew/worker1")
// Operations go to /rig/mayor/rig/.beads (following chain)
```

## Production Considerations

### Many Routes

In production, a town may have many rigs:

```json
{"prefix": "hq-", "path": "."}
{"prefix": "gt-", "path": "gastown/mayor/rig"}
{"prefix": "ap-", "path": "ai_platform/mayor/rig"}
{"prefix": "bd-", "path": "beads/mayor/rig"}
{"prefix": "toast-", "path": "toastrig/mayor/rig"}
...potentially dozens more...
```

This is handled efficiently:
- routes.jsonl is small (one line per rig)
- Loaded on-demand, not cached (file may change)
- Prefix lookup is O(n) but n is small (<100 rigs)

### Adding New Rigs

When `gt rig new <name>` is run:
1. Creates rig directory structure
2. Initializes rig-level beads with `bd init`
3. Appends route to town-level routes.jsonl
4. Creates redirect in rig/.beads pointing to mayor/rig/.beads

### Prefix Conflicts

The doctor check (`PrefixConflictCheck`) validates routes.jsonl:
- Detects duplicate prefixes across rigs
- Reports as error since it breaks bead ID → rig resolution

## Why Not Add Routing to BeadsOps?

### Alternative Considered

```go
// NOT implemented - but was considered
type BeadsOps interface {
    // Route management
    LoadRoutes() ([]Route, error)
    AppendRoute(route Route) error
    GetRigPathForPrefix(prefix string) string
    ...
}
```

### Reasons Rejected

1. **Wrong Abstraction Level**: BeadsOps is for issue/bead operations, not file management
2. **Breaks Double**: Double would need filesystem to test routes
3. **Cross-Database Concern**: Routes span multiple databases
4. **Already Works**: Current standalone functions are sufficient
5. **Testing Complexity**: Would require routes.jsonl fixtures

### Current Approach Benefits

1. **Clean Interface**: BeadsOps stays focused on issue operations
2. **Testable**: Double doesn't need filesystem
3. **Flexible**: Callers control routing strategy
4. **Composable**: MultiDouble composes Doubles for testing

## Summary

| Concern | Location | Tested By |
|---------|----------|-----------|
| Issue CRUD | BeadsOps interface | conformance_*_test.go |
| Database isolation | MultiDouble | conformance_routing_test.go |
| Redirect following | ResolveBeadsDir | conformance_routing_test.go |
| Prefix → Rig resolution | routes.go functions | routes_test.go |
| routes.jsonl management | routes.go functions | routes_test.go |

The routing layer is intentionally separate from BeadsOps, allowing each component to be tested independently and composed as needed.
