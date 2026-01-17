# Final Consolidated Review: Factory Consolidation Refactor

**Date:** 2026-01-17
**Commits:** bbb768f5..HEAD (e81d9696)
**Reviewers:** 5 parallel review agents + consolidation

---

## Executive Summary

This refactor consolidates agent lifecycle management from scattered per-role managers into a unified `factory.Start()` function, introducing `AgentID` as a semantic address type.

**Key Changes:**
- **Unified startup**: All agents now start via `factory.Start(townRoot, agentID, runtime)`
- **AgentID addressing**: Logical addresses (`"mayor"`, `"gastown/witness"`, `"gastown/crew/joe"`) replace session name construction
- **Deleted packages**: `mayor.Manager`, `deacon.Manager`, `connection/` package removed
- **Consolidated cycling**: Three `*_cycle.go` files merged into unified `cycle.go`
- **Enhanced zombie detection**: `Agents.Exists()` now checks process liveness, not just session existence

**Impact:** -2,500 net lines of code, cleaner architecture, but some test coverage gaps and a potentially breaking behavior change around propulsion nudges.

---

## Critical Issues (Must-Fix Before Merge)

### 1. Propulsion Nudges Removed from Agent Restarts
**Source:** Review 2 (Behavior Deltas)
**Severity:** HIGH
**Files:** `internal/daemon/lifecycle.go`, `internal/factory/factory.go`

The old daemon's `restartSession()` sent two nudges after starting an agent:
```go
// OLD CODE (deleted)
_ = session.StartupNudge(d.tmux, sessionName, ...)   // Predecessor discovery
time.Sleep(2 * time.Second)
_ = d.tmux.NudgeSession(sessionName, PropulsionNudge)  // Trigger autonomous work
```

The new `factory.Start()` does NOT send these nudges. Agents restarted by the daemon may not automatically begin work.

**Impact:** Agents may sit idle after restart instead of picking up work from their hook.

**Recommendation:** Either:
1. Add propulsion nudge to `factory.Start()` for roles that need it
2. Ensure all agents have hook-based work discovery that doesn't require nudges
3. Document this as intentional and verify agents self-start via other mechanisms

---

### 2. Test Coverage Gap: `internal/ids/ids.go` Has No Tests
**Source:** Review 4 (Test Coverage)
**Severity:** HIGH
**Files:** `internal/ids/ids.go` (new), `internal/connection/address_test.go` (deleted)

The `connection/address.go` parsing logic was moved to `internal/ids/ids.go`, but its 450 lines of tests were deleted without replacement. This code handles:
- `ParseAddress()` - agent address parsing
- `ParseSessionName()` - session name to AgentID conversion
- Edge cases (unicode, special characters, malformed inputs)

**Impact:** Parsing bugs could cause agents to start with wrong identities or fail to resolve.

**Recommendation:** Port the critical test cases from the deleted `address_test.go` to a new `ids_test.go`.

---

## Important Issues (Should-Fix, Risk If Not Addressed)

### 3. WaitReady Error Silently Ignored
**Source:** Review 3 (Error Paths)
**Severity:** MEDIUM
**File:** `internal/factory/factory.go:117`

```go
_ = agents.WaitReady(id)  // Error ignored!
return id, nil
```

If an agent starts but never becomes ready (e.g., Claude fails to initialize), the caller receives success.

**Impact:** Agents may appear started but be non-functional.

**Recommendation:** Either propagate the error or add documentation that `Start()` doesn't guarantee readiness.

---

### 4. AgentID.Parse() Lacks Validation
**Source:** Review 3 (Error Paths)
**Severity:** MEDIUM
**File:** `internal/agent/agent.go`

`Parse()` returns values without validation:
- Empty string → `("", "", "")` - empty role
- `"a/b/c/d"` → `("unknown", "", "")` - inconsistent handling
- No error return - caller cannot distinguish valid from invalid

**Impact:** Malformed AgentIDs silently produce invalid results that may cause downstream failures.

**Recommendation:** Add `AgentID.Validate()` method or change `Parse()` to return an error.

---

### 5. Session Name Parsing Ambiguity
**Source:** Review 3 (Error Paths)
**Severity:** MEDIUM
**File:** `internal/session/names.go`

The `FromUniqueHumanReadableName()` function has ambiguous cases:
- A polecat named "crew" produces `gt-myrig-crew` which parses as crew member
- Names with hyphens may be misinterpreted

**Impact:** Rare edge cases where agent identity is misdetected.

**Recommendation:** Add test cases for ambiguous names; consider using a more structured format (e.g., `gt:rig:role:name`).

---

### 6. Manager Test Coverage Not Migrated
**Source:** Reviews 4 & 5 (Test Coverage, Deleted Code)
**Severity:** MEDIUM
**Files:** Deleted `mayor/manager_test.go` (306 lines), `deacon/manager_test.go` (284 lines)

These tests covered:
- Settings file creation
- Error injection (start failure, stop failure, settings failure)
- Status/GetInfo behavior
- WorkDir and Command correctness

The new `factory_test.go` provides basic coverage but not the same depth.

**Recommendation:** Add factory.Start() tests for error paths and settings creation.

---

## Minor Issues (Nice-to-Fix, Code Quality)

### 7. Cleanup Errors Discarded
**Source:** Review 3 (Error Paths)
**File:** `internal/agent/start_config.go:84,92`

When a callback fails and cleanup also fails, only the callback error is returned:
```go
if err := cfg.OnCreated(...); err != nil {
    _ = a.sess.Stop(sessionID)  // Cleanup error discarded
    return fmt.Errorf("session setup: %w", err)
}
```

**Recommendation:** Log the cleanup error or return a compound error.

---

### 8. Unused Parameters in Manager Constructors
**Source:** Review 1 (API Contracts)
**Files:** `witness.NewManager`, `refinery.NewManager`, `crew.NewManager`, `polecat.NewSessionManager`

Several manager constructors have unused `_` parameters:
```go
func NewManager(agents agent.Agents, r *rig.Rig, _ string) *Manager
```

**Recommendation:** Remove unused parameters or document why they exist.

---

### 9. Blocking WaitReady Changes Startup Behavior
**Source:** Review 2 (Behavior Deltas)
**File:** `internal/factory/factory.go`

Old manager `Start()` methods returned immediately; new `factory.Start()` blocks on `WaitReady()`.

**Impact:** Batch startups may be slower.

**Recommendation:** Document this change; consider `WithAsync()` option if needed.

---

## Indirect Bug Fixes

The refactor appears to have fixed these issues:

### 1. Inconsistent Zombie Detection
**Before:** Each manager had slightly different zombie handling logic.
**After:** Centralized in `StartWithConfig()` with consistent behavior.

### 2. Env Var Duplication
**Before:** Env vars were both prepended to command AND set via `t.SetEnvVars()`.
**After:** Single-pass through `buildSessionConfigurer()`.

### 3. Race in Crew Callback Context
**Before:** `crew.Manager` stored `m.startCrewName` etc. for callback access (potential race if Start called concurrently).
**After:** Closures capture context at call time, thread-safe.

### 4. GUPP False Positives
**Before:** `IsClaudeRunning()` could return true for zombie sessions.
**After:** `factory.Agents().Exists()` includes zombie filtering.

---

## Recommendations (Prioritized)

### Immediate (Before Merge)
1. **Investigate propulsion nudge removal** - Verify agents have alternative work discovery mechanisms or restore nudges
2. **Add tests for `internal/ids/ids.go`** - Port critical edge cases from deleted `address_test.go`

### Short-Term (Next Sprint)
3. **Add factory.Start() error path tests** - Settings failure, callback failure, zombie cleanup failure
4. **Add integration tests** - Test actual tmux session creation, not just with Double
5. **Document WaitReady error behavior** - Either propagate error or document that Start() doesn't guarantee readiness

### Medium-Term
6. **Add AgentID validation** - Validate() method or error return from Parse()
7. **Add session name ambiguity tests** - Cover edge cases like polecat named "crew"
8. **Clean up unused parameters** - Remove `_` parameters or document their purpose

---

## Metrics

```
Files Changed:        112
Lines Added:        5,187
Lines Deleted:      7,719
Net Change:        -2,532

Test Files Changed:    21
Test Lines Added:   1,492
Test Lines Deleted: 2,811
Test Net Change:   -1,319

Deleted Packages:       2 (mayor, connection)
Deleted Files:         12
New Types:              3 (AgentID methods, StartConfig, StartOption)
```

---

## Conclusion

This is a well-designed refactor that consolidates scattered lifecycle logic into a clean, unified API. The `factory.Start()` + `AgentID` pattern is more testable and maintainable than the previous per-role managers.

**Main concerns:**
1. The propulsion nudge removal could break autonomous work pickup - needs investigation
2. Test coverage decreased, particularly for the `ids` package and error paths
3. Some subtle behavioral changes (blocking WaitReady, stricter Exists) may affect callers

**Recommendation:** Address the critical issues (propulsion nudges, ids tests) before merging. The important issues can be addressed as fast-follows.
