# BeadsOps Parameter Usage Audit

Analysis of 343+ production calls across 67+ files (excluding tests).

## Executive Summary

After thorough analysis and attempted removal of "unused" parameters, only **1 field** was confirmed as truly unused: `ListOptions.NoAssignee`. All other fields that appeared unused in grep analysis were actually used in production.

## Removed Parameters

### ListOptions.NoAssignee
- **Status**: Removed
- **Reason**: Zero production callers. Callers filter assignee differently (checking for empty assignee in results).

## Parameters That Appeared Unused But Are Actually Used

The initial audit incorrectly identified these as unused. Build failures revealed actual production usage:

| Struct | Field | Actual Usage |
|--------|-------|--------------|
| `ListOptions.Tag` | prime.go, beads_dog.go |
| `ListOptions.CreatedAfter` | plugin/recording.go |
| `ListOptions.All` | plugin/recording.go |
| `ListOptions.MolType` | swarm.go |
| `ListOptions.DescContains` | mail/router.go |
| `ListOptions.SortBy` | mail/router.go |
| `ListOptions.SortAsc` | mail/router.go |
| `UpdateOptions.Unassign` | crew_lifecycle.go |
| `UpdateOptions.AddLabels` | beads_escalation.go |
| `UpdateOptions.RemoveLabels` | beads_escalation.go |
| `CloseOptions.Force` | hook.go |
| `CreateOptions.MolType` | swarm.go |

## Actively Used Parameters (All Kept)

### ListOptions
| Field | Notes |
|-------|-------|
| `Status` | Core - "open", "closed", "all", "hooked", "in_progress" |
| `Type` | Core - "agent", "molecule", "task", "merge-request" |
| `Priority` | Common - -1 for no filter, 0-4 values |
| `Label/Labels` | Moderate use |
| `Assignee` | Light use |
| `Parent` | Light use |
| `Limit` | Rarely set |
| `Tag` | prime.go |
| `CreatedAfter` | plugin/recording.go |
| `All` | plugin/recording.go |
| `MolType` | swarm.go |
| `DescContains` | mail/router.go |
| `SortBy` | mail/router.go |
| `SortAsc` | mail/router.go |

### CreateOptions
| Field | Notes |
|-------|-------|
| `Title` | Required |
| `Type` | Required |
| `Priority` | Common |
| `Description` | Moderate |
| `Parent` | handoff.go, molecule.go |
| `Assignee` | Light |
| `Actor` | Provenance tracking |
| `Ephemeral` | Wisps |
| `Labels` | mail/router.go |
| `MolType` | swarm.go |
| `EventCategory` | costs.go |
| `EventActor` | costs.go |
| `EventPayload` | costs.go |
| `EventTarget` | costs.go |

### UpdateOptions
| Field | Notes |
|-------|-------|
| `Title` | Moderate |
| `Status` | Core |
| `Priority` | Moderate |
| `Description` | Light |
| `Assignee` | Light |
| `Unassign` | crew_lifecycle.go |
| `AddLabels` | beads_escalation.go |
| `RemoveLabels` | beads_escalation.go |
| `SetLabels` | Various |
| `Notes` | Minimal |

### CloseOptions
| Field | Notes |
|-------|-------|
| `Reason` | "merged", "retention pruning", "rejected" |
| `Session` | Audit trail |
| `Force` | hook.go (closing pinned issues) |

## Redundant Method Variants

| Method | Status | Notes |
|--------|--------|-------|
| `DeleteWithOptions` | Keep | Part of interface, may be needed for future use |
| `CloseWithOptions` | Keep | Used in swarm.go, polecat.go, hook.go, crew_lifecycle.go, synthesis.go |

## Methods with Good Design

These methods have no options structs - clean, simple APIs:

- `Show(id string)` - 109 calls, most used method
- `Ready()` - 4 calls
- `ReadyWithLabel(label, limit)` - 1 call
- `Close(ids ...string)` - basic variant

## Lessons Learned

1. **Grep-based analysis is insufficient**: Static grep analysis missed callers because:
   - Field names appear in struct literals without explicit field names
   - Callers may use different variable names
   - Some files were excluded from the initial search

2. **Build verification is essential**: Attempting to remove "unused" fields immediately reveals actual dependencies through build failures.

3. **Conservative approach preferred**: Keep parameters that have any production usage, even if infrequent.

## Conclusion

Only `ListOptions.NoAssignee` was confirmed as truly unused and removed. All other option struct fields are used in production and remain in the API.
