# Agent Manager Test Plan

## Overview

This document defines unit tests for all agent managers using `session.Double` as the testable abstraction.

## Manager Categories

| Category | Managers | Characteristics |
|----------|----------|-----------------|
| Town-level Stateless | mayor, deacon | Single instance, no state file |
| Rig-level Stateful | witness, refinery | Single instance per rig, state file |
| Rig-level Multi-instance | crew, polecat | Multiple named instances per rig |

---

## Test Plan by Manager

### Mayor / Deacon (Town-level Stateless)

**Public API:**
- `Start() error`
- `Stop() error`
- `IsRunning() (bool, error)`
- `Status() (*session.Info, error)`
- `SessionName() string`

#### Start() Tests

| Test | Preconditions | Input | Expected Output | Verifications |
|------|---------------|-------|-----------------|---------------|
| S1: Creates session | Dir exists, no session | Start() | nil | Session exists, correct name |
| S2: Already running | Session exists, process alive | Start() | ErrAlreadyRunning | Session unchanged |
| S3: Zombie cleanup | Session exists, process dead | Start() | nil | Old session killed, new created |
| S4: Creates settings | No .claude dir | Start() | nil | .claude/settings.local.json exists |
| S5: Settings failure | Dir not writable | Start() | error | No session created |

#### Stop() Tests

| Test | Preconditions | Input | Expected Output | Verifications |
|------|---------------|-------|-----------------|---------------|
| T1: Terminates session | Session running | Stop() | nil | Session gone |
| T2: Not running | No session | Stop() | ErrNotRunning | - |
| T3: Graceful shutdown | Session running | Stop() | nil | Ctrl-C sent before kill |

#### IsRunning() Tests

| Test | Preconditions | Input | Expected Output |
|------|---------------|-------|-----------------|
| R1: Returns false | No session | IsRunning() | false, nil |
| R2: Returns true | Session exists | IsRunning() | true, nil |
| R3: After stop | Start then Stop | IsRunning() | false, nil |

#### Status() Tests

| Test | Preconditions | Input | Expected Output |
|------|---------------|-------|-----------------|
| U1: Returns info | Session running | Status() | *Info, nil |
| U2: Not running | No session | Status() | nil, ErrNotRunning |

---

### Witness / Refinery (Rig-level Stateful)

**Additional Public API:**
- `Status() (*Witness/*Refinery, error)` - returns state struct
- `SetEnvOverrides([]string)` (witness only)

#### Start() Tests (additions to stateless)

| Test | Preconditions | Input | Expected Output | Verifications |
|------|---------------|-------|-----------------|---------------|
| S6: Updates state | Dir exists | Start() | nil | State.State=Running, StartedAt set |
| S7: State rollback | State save fails | Start() | error | Session cleaned up, state unchanged |
| S8: Dir fallback | Primary dir missing | Start() | nil | Uses fallback dir |

#### Stop() Tests (additions to stateless)

| Test | Preconditions | Input | Expected Output | Verifications |
|------|---------------|-------|-----------------|---------------|
| T4: Updates state | Running | Stop() | nil | State.State=Stopped |
| T5: State=Running, no session | Stale state | Stop() | nil | State updated to Stopped |
| T6: State=Stopped, session exists | Stale session | Stop() | nil | Session killed |
| T7: Returns stop error | Stop fails | Stop() | stop error | State still saved |

#### Status() Tests

| Test | Preconditions | Input | Expected Output |
|------|---------------|-------|-----------------|
| U3: Returns state | Any | Status() | *State, nil |
| U4: Fresh state after start | After Start() | Status() | State.State=Running |
| U5: Fresh state after stop | After Stop() | Status() | State.State=Stopped |

---

### Crew (Rig-level Multi-instance)

**Public API:**
- `Start(name string, opts StartOptions) error`
- `Stop(name string) error`
- `IsRunning(name string) (bool, error)`
- `Add(name string, createBranch bool) (*CrewWorker, error)`
- `Remove(name string, force bool) error`
- `List() ([]*CrewWorker, error)`
- `Get(name string) (*CrewWorker, error)`
- `Rename(oldName, newName string) error`
- `SessionName(name string) string`

#### Start() Tests

| Test | Preconditions | Input | Expected Output | Verifications |
|------|---------------|-------|-----------------|---------------|
| S1: Creates session | Worker exists | Start("foo", {}) | nil | Session "gt-rig-crew-foo" exists |
| S2: Auto-creates worker | Worker not exists | Start("foo", {}) | nil | Worker created, session started |
| S3: Already running | Session exists | Start("foo", {}) | ErrAlreadyRunning | - |
| S4: KillExisting | Session exists | Start("foo", {KillExisting:true}) | nil | Old killed, new started |
| S5: Invalid name | - | Start("", {}) | error | No session |

#### Stop() Tests

| Test | Preconditions | Input | Expected Output |
|------|---------------|-------|-----------------|
| T1: Terminates | Session running | Stop("foo") | nil |
| T2: Not running | No session | Stop("foo") | ErrNotRunning |
| T3: Invalid name | - | Stop("") | error |

#### Add/Remove/List/Get Tests

| Test | Preconditions | Input | Expected Output |
|------|---------------|-------|-----------------|
| A1: Add creates worker | Worker not exists | Add("foo", false) | *Worker, nil |
| A2: Add duplicate | Worker exists | Add("foo", false) | nil, ErrCrewExists |
| A3: Remove deletes | Worker exists, not running | Remove("foo", false) | nil |
| A4: Remove running | Worker running | Remove("foo", false) | ErrSessionRunning or similar |
| A5: Remove force | Worker running | Remove("foo", true) | nil |
| A6: List returns all | Multiple workers | List() | []*Worker, nil |
| A7: Get returns worker | Worker exists | Get("foo") | *Worker, nil |
| A8: Get not found | Worker not exists | Get("foo") | nil, ErrCrewNotFound |

---

### Polecat SessionManager (Rig-level Multi-instance)

**Public API:**
- `Start(polecat string) error`
- `Stop(polecat string, force bool) error`
- `IsRunning(polecat string) (bool, error)`
- `Status(polecat string) (*SessionInfo, error)`
- `List() ([]SessionInfo, error)`
- `Capture(polecat string, lines int) (string, error)`
- `Inject(polecat, message string) error`
- `StopAll(force bool) error`

#### Start() Tests

| Test | Preconditions | Input | Expected Output | Verifications |
|------|---------------|-------|-----------------|---------------|
| S1: Creates session | Polecat in rig config | Start("p1") | nil | Session exists |
| S2: Already running | Session exists | Start("p1") | ErrSessionRunning | - |
| S3: Polecat not found | Not in rig config | Start("unknown") | ErrPolecatNotFound | - |
| S4: Zombie cleanup | Session exists, process dead | Start("p1") | nil | New session |

#### Stop() Tests

| Test | Preconditions | Input | Expected Output |
|------|---------------|-------|-----------------|
| T1: Terminates | Running | Stop("p1", false) | nil |
| T2: Not found | Not running | Stop("p1", false) | ErrSessionNotFound |
| T3: Force stop | Running | Stop("p1", true) | nil (no beads sync) |

#### Capture/Inject Tests

| Test | Preconditions | Input | Expected Output |
|------|---------------|-------|-----------------|
| C1: Capture returns output | Running | Capture("p1", 50) | string, nil |
| C2: Capture not found | Not running | Capture("p1", 50) | "", error |
| I1: Inject sends text | Running | Inject("p1", "hello") | nil |
| I2: Inject not found | Not running | Inject("p1", "hello") | error |

#### List/StopAll Tests

| Test | Preconditions | Input | Expected Output |
|------|---------------|-------|-----------------|
| L1: List returns running | Multiple running | List() | []SessionInfo |
| L2: List empty | None running | List() | [] |
| SA1: StopAll stops all | Multiple running | StopAll(false) | nil, all stopped |

---

## Test Implementation Checklist

### Agent (internal/agent/agent_test.go)
- [x] Start creates session
- [x] Start when already running returns ErrAlreadyRunning
- [x] Start detects and cleans up zombies
- [x] Start callback error cleans up session
- [x] Start callback success preserves session
- [x] Stop terminates session
- [x] Stop when not running returns nil (idempotent)
- [x] Stop graceful succeeds
- [x] SendControl logs control sequences
- [x] Exists returns correct state
- [x] Restart stops and starts
- [x] Full lifecycle test

### Mayor
- [x] S1: Creates session
- [x] S2: Already running
- [x] S3: Zombie cleanup
- [x] S4: Creates settings
- [x] S5: Settings failure
- [x] T1: Terminates session
- [x] T2: Not running
- [x] T3: Graceful shutdown (via failure injection)
- [x] R1-R3: IsRunning
- [x] U1-U2: Status
- [x] Failure injection: Start/Stop/WaitReady

### Deacon
- [x] S1: Creates session
- [x] S2: Already running
- [x] S3: Zombie cleanup
- [x] S4: Creates settings
- [x] S5: Settings failure
- [x] T1: Terminates session
- [x] T2: Not running
- [x] T3: Graceful shutdown (via failure injection)
- [x] R1-R3: IsRunning
- [x] U1-U2: Status
- [x] Failure injection: Start/Stop/WaitReady

### Witness (internal/witness/manager_test.go)
- [x] S1: Creates session
- [x] S2: Already running
- [x] S3: Zombie cleanup
- [x] S6: Updates state to running
- [x] T1: Terminates session
- [x] T2: Not running
- [x] T4: Updates state to stopped
- [x] T5: State=Running but no session succeeds
- [x] T6: State=Stopped but session exists succeeds
- [x] U3-U5: Status tests
- [x] Full lifecycle test
- [x] SessionName format

### Refinery (internal/refinery/manager_test.go)
- [x] S1: Creates session
- [x] S2: Already running
- [x] S3: Zombie cleanup
- [x] S6: Updates state to running
- [x] T1: Terminates session
- [x] T2: Not running
- [x] T4: Updates state to stopped
- [x] T5: State=Running but no session succeeds
- [x] T6: State=Stopped but session exists succeeds
- [x] U3-U5: Status tests
- [x] Full lifecycle test
- [x] SessionName format
- [x] MR business logic tests (GetMR, Retry, RegisterMR)

### Crew (internal/crew/manager_test.go)
- [x] S1: Start creates session
- [x] S2: Auto-creates worker (TestManagerStart_AutoCreatesWorker)
- [x] S3: Already running
- [x] T1: Stop terminates
- [x] T2: Stop not running returns error
- [x] R1-R2: IsRunning true/false
- [x] A1-A8: Add/Remove/List/Get tests (existing tests)

### Polecat SessionManager (internal/polecat/session_manager_*_test.go)
- [x] S1: Creates session (TestStart_CreatesSessionWithCorrectCommand)
- [x] S2: Already running (TestStart_RejectsAlreadyRunningSession)
- [x] S3: Polecat not found (TestStartPolecatNotFound)
- [x] T1: Terminates session (TestStop_TerminatesSession)
- [x] T2: Not found (TestStopNotFound)
- [x] T3: Force stop (TestStop_ForceSkipsGracefulShutdown)
- [x] C1: Capture returns output (TestCapture_ReturnsOutput)
- [x] C2: Capture not found (TestCaptureNotFound)
- [x] I1: Inject sends text (TestInject_SendsMessage)
- [x] I2: Inject not found (TestInjectNotFound)
- [x] L1: List filters by rig (TestList_FiltersByRigPrefix)
- [x] L2: List empty (TestSessionManagerListEmpty)
- [x] SA1: StopAll stops all (TestStopAll_StopsAllSessions)
- [x] R1-R2: IsRunning true/false
- [x] Status tests (TestStatus_PopulatesSessionInfo)
- [x] SessionName format (TestSessionName)
- [x] Env vars format (TestPolecatCommandFormat)
