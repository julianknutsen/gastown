# BeadsOps Double Testing Guidelines

---

## ⚠️ CRITICAL INVARIANTS ⚠️

### Invariant 1: Double MUST Mirror Implementation

**Double should always behave identically to Implementation**, including bugs. If a test passes for Double but fails for Implementation, **Double is wrong**.

- Double is a test double for Implementation, not for real bd
- DoubleOnly tests indicate a design problem and should be avoided
- If Implementation has a bug, Double should have the same bug

### Invariant 2: Never Change Double Without Verifying Real bd

**NEVER change the Double implementation without first:**

1. **Verifying real bd CLI behavior** - Test manually in a temp repo
2. **Ensuring TrueRawBdOps test passes** - The conformance test must pass against real bd
3. **Only THEN updating Double AND Implementation** - Both must match

**WHY:** The Double must faithfully replicate Implementation behavior. The conformance test suite exists to catch mismatches between Double, Implementation, and real bd.

**WORKFLOW:**
```
1. Write conformance test (runs against Double, Implementation, TrueRawBdOps)
2. Test fails on Implementation? → Double should also fail (fix Double if not)
3. Test fails on TrueRawBdOps? → Verify bd behavior manually
4. bd behaves differently? → Fix Implementation AND Double to match bd
5. All three pass? → Behavior is consistent
```

### Invariant 3: No DoubleOnly Tests

If you find yourself needing a DoubleOnly test, something is wrong:
- Either Double doesn't match Implementation (fix Double)
- Or there's a real bug in Implementation (fix Implementation or document as known limitation)

---

## How to Verify bd Behavior

1. **Create a temporary beads repo:**
   ```bash
   cd /tmp && rm -rf bd-test && mkdir bd-test && cd bd-test
   bd init --prefix=test
   ```

2. **Test the specific operation:**
   ```bash
   # Create test data
   bd create --title="Test issue" --json

   # Test the operation in question
   bd list --json
   bd show test-xxx --json
   bd blocked --json
   ```

3. **Compare JSON output** to understand what fields are populated and when.

### Example: bd list vs bd blocked

This bug was discovered during testing:

**Bug:** `Engineer.ListBlockedMRs()` used `bd list` and expected `blocked_by` to be populated.

**Reality:**
- `bd list --json` does NOT populate `blocked_by` - only `dependency_count`
- `bd blocked --json` DOES populate `blocked_by` array
- `bd show --json` populates `dependencies` (detailed objects) but not `blocked_by`

**Fix:** Changed `ListBlockedMRs()` to use `Blocked()` instead of `List()`.

## Common bd CLI Output Differences

| Field | `bd list` | `bd show` | `bd blocked` |
|-------|-----------|-----------|--------------|
| `blocked_by` | No | No | Yes |
| `blocked_by_count` | No | No | Yes |
| `dependency_count` | Yes | No | No |
| `dependent_count` | Yes | No | No |
| `dependencies` (objects) | No | Yes | No |
| `depends_on` (IDs) | No | Yes | No |

## Conformance Test Pattern

When adding new methods to the BeadsOps interface:

1. **Implement in Double** matching assumed behavior
2. **Implement in TrueRawBdOps** calling real bd CLI
3. **Write conformance matrix test** that runs against both
4. **Run with real bd** to verify behavior matches

```go
// Example conformance test
func TestBlockedPopulatesBlockedBy(t *testing.T) {
    // This test runs against both Double and real bd
    runConformanceTest(t, func(ops beads.BeadsOps) {
        // Create blocker
        blocker, _ := ops.Create(beads.CreateOptions{Title: "Blocker"})

        // Create blocked issue
        blocked, _ := ops.Create(beads.CreateOptions{Title: "Blocked"})
        ops.AddDependency(blocked.ID, blocker.ID)

        // Verify Blocked() populates blocked_by
        blockedIssues, _ := ops.Blocked()
        require.Len(t, blockedIssues, 1)
        require.Contains(t, blockedIssues[0].BlockedBy, blocker.ID)
    })
}
```

## Bugs Found During Testing

### 2026-01-16: ListBlockedMRs used wrong bd command

**Location:** `internal/refinery/engineer.go:ListBlockedMRs()`

**Symptom:** Function always returned empty slice.

**Root Cause:** Used `beads.List()` and filtered by `issue.BlockedBy`, but `bd list --json` does not populate the `blocked_by` field.

**Fix:** Changed to use `beads.Blocked()` which calls `bd blocked --json` and properly populates `blocked_by`.

**Lesson:** Always verify what fields bd commands actually populate before writing code that depends on them.

### 2026-01-16: bd merge-slot acquire has "invalid field for update: holder" bug

**Location:** bd CLI command `bd merge-slot acquire --holder=<name>`

**Symptom:** Running `bd merge-slot acquire --holder=test` fails with:
```
Error: failed to acquire slot: invalid field for update: holder
```

**Root Cause:** bd CLI bug in the merge-slot acquire command.

**Workaround:** Tests that require MergeSlotAcquire are marked with `DoubleOnly: true` to skip Implementation and RawBd.

**Tracking:** Conformance tests for MergeSlotAcquire, MergeSlotAcquire_AlreadyHeld, and MergeSlotRelease (which depends on Acquire) only run against Double until bd is fixed.

### 2026-01-16: Double.MergeSlotCheck returned error instead of status

**Location:** `internal/beads/double.go:MergeSlotCheck()`

**Symptom:** Double returned a Go error when slot wasn't found, but bd returns JSON `{error: "not found"}`.

**Root Cause:** Double implementation didn't match real bd behavior.

**Fix:** Updated Double to return `&MergeSlotStatus{Error: "not found"}, nil` instead of `nil, error`.

### 2026-01-16: Implementation.Create converts Type to label instead of --type flag [FIXED]

**Location:** `internal/beads/implementation.go:Create()`

**Symptom:** Creating an issue with `Type: "agent"` results in `issue_type: "task"` with a `gt:agent` label instead of `issue_type: "agent"`.

**Root Cause:** Implementation.Create had a comment "Type is deprecated: convert to gt:<type> label" and used `--labels=gt:type` instead of `--type=type`.

**Impact:** bd slot commands require `issue_type` to be "agent", not just a label.

**Fix:** Changed Implementation.Create to use `--type=` flag. Updated Double and TrueRawBdOps to match. Slot conformance tests now pass.
