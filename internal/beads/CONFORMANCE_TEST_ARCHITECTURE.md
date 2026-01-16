# Conformance Test Architecture

This document describes the architecture of the beads conformance tests. These tests
verify that all BeadsOps implementations behave consistently.

## Overview

The conformance tests run every test case against **three implementations**:

1. **Double** - In-memory test double that defines the expected behavior
2. **Implementation** - Production wrapper around bd CLI with workarounds for known bugs
3. **RawBd** - Direct bd CLI calls without workarounds (tests bd's native behavior)

## Key Files

```
internal/beads/
├── conformance_matrix_common_test.go   # Shared infrastructure
├── conformance_matrix_*_test.go        # Domain-specific tests
├── bd_routing_bugs.go                  # Bug tracking registry
└── conformance_routing_test.go         # Double/routing-specific tests
```

## Test Types

### SimpleConformanceTest

For operations that **don't take a bead ID** (Config, Daemon, Version, etc.):

```go
RunSimpleConformanceTest(t, SimpleConformanceTest{
    Name:      "Config",
    Operation: "ConfigGet",  // Must match key in BdRoutingBugs
    Test: func(ops beads.BeadsOps) error {
        val, err := ops.ConfigGet("key")
        if err != nil { return err }
        // ... validate ...
        return nil
    },
})
```

### ConformanceTest

For **ID-based operations** that may require cross-rig routing:

```go
RunConformanceTest(t, ConformanceTest{
    Name:      "Show",
    Operation: "Show",  // Must match key in BdRoutingBugs
    Test: func(ops beads.BeadsOps, targetID string) error {
        issue, err := ops.Show(targetID)
        if err != nil { return err }
        // ... validate ...
        return nil
    },
})
```

ID-based tests run in two contexts:
- **same-rig**: Target bead is in the same database as workDir (gt → gt)
- **cross-rig**: Target bead is in a different database (gt → ap)

## Bug Tracking System

The `bd_routing_bugs.go` file maintains a registry of known bugs in the bd CLI:

```go
var BdRoutingBugs = map[string]bool{
    "Show":     true,   // bd routes Show correctly
    "Update":   true,   // bd routes Update correctly
    "Delete":   false,  // bd has routing bug for Delete
    "Reopen":   false,  // bd has routing bug for Reopen
    // ...
}
```

- `true` = Operation works correctly in bd (no workaround needed)
- `false` = Operation has a bug (Implementation adds workaround)

### How Bug Tracking Works in Tests

When running tests:

1. **Double**: Always expected to pass (defines correct behavior)
2. **Implementation**: Always expected to pass (has workarounds)
3. **RawBd** with `BdRoutingBugs[op] = true`: Expected to pass
4. **RawBd** with `BdRoutingBugs[op] = false`: Expected to fail in cross-rig context

When a RawBd test unexpectedly passes for a known-buggy operation, the test logs:
```
WARNING: bd may have fixed Show routing! If confirmed, update BdRoutingBugs["Show"] = true...
```

## Test Environment

Each test gets a fresh environment simulating a town with multiple rigs:

```
<townRoot>/
├── mayor/town.json              # Town marker
├── .beads/                      # Town-level beads (prefix: hq)
│   ├── issues.jsonl
│   └── routes.jsonl
├── gastown/                     # Rig 1
│   ├── .beads/redirect → mayor/rig/.beads
│   └── mayor/rig/.beads/        # Actual beads (prefix: gt)
│       └── issues.jsonl
└── ai_platform/                 # Rig 2
    ├── .beads/redirect → mayor/rig/.beads
    └── mayor/rig/.beads/        # Actual beads (prefix: ap)
        └── issues.jsonl
```

The routes.jsonl enables cross-rig operations by mapping prefixes to paths:
```jsonl
{"prefix": "gt-", "path": "gastown/mayor/rig"}
{"prefix": "ap-", "path": "ai_platform/mayor/rig"}
{"prefix": "hq-", "path": "."}
```

## TrueRawBdOps

`TrueRawBdOps` is a BeadsOps implementation that calls bd directly without:
- Setting `BEADS_DIR` environment variable
- Any routing workarounds

This tests bd's **native behavior** and reveals routing bugs.

```go
type TrueRawBdOps struct {
    workDir  string  // Working directory for bd commands
    testHome string  // Isolated HOME to avoid polluting real config
}

func (r *TrueRawBdOps) run(args ...string) ([]byte, error) {
    cmd := exec.Command("bd", args...)
    cmd.Dir = r.workDir
    cmd.Env = append(os.Environ(), "HOME="+r.testHome)
    // No BEADS_DIR set - tests native routing
    return cmd.Output()
}
```

## Adding New Tests

### 1. Choose the Test Type

- **SimpleConformanceTest**: For operations without bead ID (stats, config, daemon)
- **ConformanceTest**: For operations with bead ID that may need routing

### 2. Add to Appropriate Domain File

Create tests in the domain-specific file:
- `conformance_matrix_config_test.go` - Config, Init, Migrate
- `conformance_matrix_daemon_test.go` - DaemonStart, DaemonStop, etc.
- `conformance_matrix_show_test.go` - Show, ShowMultiple
- `conformance_matrix_update_test.go` - Update, Close, Delete, Reopen
- `conformance_matrix_create_test.go` - Create, List
- `conformance_matrix_label_test.go` - LabelAdd, LabelRemove, Comment
- `conformance_matrix_dependency_test.go` - AddDependency, Ready, Blocked
- `conformance_matrix_misc_test.go` - Stats, Version, Search, etc.
- `conformance_matrix_formula_test.go` - FormulaShow, Cook, LegAdd
- `conformance_matrix_gate_test.go` - GateShow, GateList, etc.
- `conformance_matrix_molecule_test.go` - MolSeed, WispCreate, etc.
- `conformance_matrix_swarm_test.go` - SwarmStatus, SwarmCreate, etc.

### 3. Register Operation in BdRoutingBugs

If testing a new operation, add it to `bd_routing_bugs.go`:
```go
"NewOperation": true,  // true if bd handles it correctly
```

### 4. Implement in TrueRawBdOps (if needed)

If the operation needs real bd testing, implement it in `TrueRawBdOps`:
```go
func (r *TrueRawBdOps) NewOperation(args) error {
    _, err := r.run("new-cmd", args)
    return err
}
```

## Running Tests

```bash
# Run all matrix tests
go test ./internal/beads/... -run 'TestMatrix_' -v

# Run specific domain
go test ./internal/beads/... -run 'TestMatrix_Show' -v

# Run specific target
go test ./internal/beads/... -run 'TestMatrix_Show/RawBd' -v
```

## Known Behavioral Differences

Some operations behave differently between implementations. These are handled
by normalizing bd's behavior in the Implementation layer:

### 1. Wrapped JSON Output

bd often returns wrapped objects instead of arrays:
```json
{"swarms": [...]}     // SwarmList
{"wisps": [...]}      // WispList
{"protos": [...]}     // MolCatalog (if implemented)
```

**Fix:** Implementation parses the wrapper and returns the inner array.

### 2. Null vs Empty Array

bd returns `null` instead of `[]` for empty lists (e.g., GateList).

**Fix:** Implementation normalizes `null` to empty slice `[]*Type{}`.

### 3. ConfigGet for Missing Keys

bd returns `"key (not set)"` for missing config keys.

**Fix:** Implementation strips the "(not set)" suffix and returns empty string.

## Routing Tests

The `conformance_routing_test.go` file tests **Double-specific functionality** not
covered by BeadsOps interface tests:

- `ResolveBeadsDir` - Redirect chain resolution
- `ConfigureRoute` / `SetWorkDir` - Multi-database routing
- `SetActive` - Database activation
- Town/rig isolation with shared databases

These tests verify the Double's internal routing logic, not the BeadsOps interface.
