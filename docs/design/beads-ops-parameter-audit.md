# BeadsOps Parameter Usage Audit

Analysis of 343+ production calls across 67+ files (excluding tests).

## Executive Summary

The BeadsOps interface carries **14+ unused or underused parameters** across its option structs. Removing these would improve API clarity and maintainability.

## Unused Parameters (Safe to Remove)

### Completely Unused (0 production calls)

| Struct | Field | Notes |
|--------|-------|-------|
| `ListOptions` | `NoAssignee` | Callers filter assignee differently |
| `ListOptions` | `SortAsc` | Never used |
| `UpdateOptions` | `Unassign` | Callers use `Assignee = nil` instead |
| `UpdateOptions` | `AddLabels` | Callers use `SetLabels` instead |
| `UpdateOptions` | `RemoveLabels` | Callers use `SetLabels` instead |
| `CloseOptions` | `Force` | Never used |
| `CreateOptions` | `MolType` | Never used |

### Almost Never Used (1-2 calls - remove)

| Struct | Field | Uses | Location |
|--------|-------|------|----------|
| `ListOptions` | `Tag` | 1 | beads_dog.go (likely dead code) |
| `ListOptions` | `CreatedAfter` | 1 | plugin/recording.go |
| `ListOptions` | `All` | 1 | plugin/recording.go |
| `ListOptions` | `MolType` | 2 | implementation.go only |
| `ListOptions` | `DescContains` | 1 | implementation.go only |
| `ListOptions` | `SortBy` | 1 | implementation.go only |

## Actively Used Parameters (Keep)

### ListOptions (126 calls)

| Field | Uses | Notes |
|-------|------|-------|
| `Status` | 111 | Core - "open", "closed", "all", "hooked", "in_progress" |
| `Type` | 52 | Core - "agent", "molecule", "task", "merge-request" |
| `Priority` | 27 | Common - -1 for no filter, 0-4 values |
| `Label/Labels` | 30+ | Moderate use |
| `Assignee` | 7 | Light use |
| `Parent` | 3 | Light use |
| `Limit` | 9 | Rarely set |

### CreateOptions (23 calls)

| Field | Uses | Notes |
|-------|------|-------|
| `Type` | 52 | Required |
| `Title` | 16 | Required |
| `Priority` | 27 | Common |
| `Description` | 6 | Moderate |
| `Assignee` | 7 | Light |
| `Ephemeral` | 4 | Wisps |
| `EventCategory` | 4 | costs.go |
| `Labels` | 3+ | mail/router.go |
| `Parent` | 3 | handoff.go, molecule.go only |

### UpdateOptions (68 calls)

| Field | Uses | Notes |
|-------|------|-------|
| `Status` | 111 | Core |
| `Title` | 16 | Moderate |
| `Priority` | 27 | Moderate |
| `Description` | 6 | Light |
| `Assignee` | 7 | Light |
| `SetLabels` | 1 | Replaces AddLabels/RemoveLabels |
| `Notes` | 2 | Minimal |

### CloseOptions (12 calls)

| Field | Uses | Notes |
|-------|------|-------|
| `Reason` | 11 | "merged", "retention pruning", "rejected" |
| `Session` | 2 | Audit trail |

## Methods with Good Design (No Changes Needed)

These methods have no options structs - clean, simple APIs:

- `Show(id string)` - 109 calls, most used method
- `Ready()` - 4 calls
- `ReadyWithLabel(label, limit)` - 1 call
- `Close(ids ...string)` - basic variant

## Redundant Method Variants

| Method | Calls | Issue |
|--------|-------|-------|
| `CloseWithOptions` | 1 | Always called with empty options, use `CloseWithReason` instead |
| `DeleteWithOptions` | 0 | Production uses `Delete()` directly |

## Recommendations

### High Priority (Remove unused fields)

```go
// ListOptions: Remove these fields
- NoAssignee bool
- SortAsc bool
- Tag string
- CreatedAfter time.Time
- All bool
- MolType string
- DescContains string
- SortBy string

// UpdateOptions: Remove these fields
- Unassign bool
- AddLabels []string
- RemoveLabels []string

// CloseOptions: Remove this field
- Force bool

// CreateOptions: Remove this field
- MolType string
```

### Medium Priority (Design improvements)

1. **CreateOptions.Parent** - Only 3 uses. Consider separate `CreateChildIssue()` method.
2. **CloseWithOptions** - Consider removing, always called with empty options.
3. **DeleteWithOptions** - Consider removing, zero production uses.

### Low Priority

1. Review if `ListOptions.Limit` default behavior is correct (rarely set).

## Call Distribution by File

### Top List() Callers
- mailbox.go (10)
- rig.go (9)
- molecule_status.go (7)
- convoy.go (5)

### Top Update() Callers
- handoff.go (6)
- engineer.go (5)
- beads_channel.go (5)
- beads_agent.go (5)

### Top Show() Callers
- Distributed across validation code
- hook_check.go
- engineer.go
- handoff.go

## Conclusion

**14 fields can be removed** from option structs with zero impact on production code. The `Show()` method's design (no options, single parameter) should be the template for other simple operations.
