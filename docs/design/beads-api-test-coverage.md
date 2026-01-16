# BeadsOps API Test Coverage Matrix

This document enumerates all API methods, their input/context permutations, expected outputs, and test coverage status.

## Legend
- [x] Test exists and passes
- [ ] Test needed
- N/A - Not applicable for Double

## 1. Issue Operations

### List(opts ListOptions) ([]*Issue, error)

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Empty options | Active repo | All open issues, sorted by created_at desc | [x] conformance_issue_test.go |
| Status="open" | Active repo | Only open/in_progress/hooked/pinned issues | [x] conformance_issue_test.go |
| Status="closed" | Active repo | Only closed issues | [x] conformance_issue_test.go |
| Status="all" | Active repo | All issues regardless of status | [x] conformance_issue_test.go |
| Label="gt:agent" | Active repo | Issues with matching label | [x] conformance_issue_test.go |
| Priority=3 | Active repo | Issues with priority 3 | [x] conformance_issue_test.go |
| Priority=-1 | Active repo | All issues (no priority filter) | [x] conformance_issue_test.go |
| Parent="xxx" | Active repo | Issues with specified parent | [x] conformance_issue_test.go |
| Assignee="alice" | Active repo | Issues assigned to alice | [x] conformance_issue_test.go |
| NoAssignee=true | Active repo | Issues with no assignee | [x] conformance_issue_test.go |
| Limit=5 | Active repo | At most 5 issues | [x] conformance_issue_test.go |
| Type="task" | Active repo | Issues with gt:task label | [x] conformance_issue_test.go |
| Combined filters | Active repo | Intersection of all filters | [x] conformance_issue_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_issue_test.go |
| Any | Town context | Town database issues only | [x] conformance_routing_test.go |
| Any | Rig context | Rig database issues only | [x] conformance_routing_test.go |

### Show(id string) (*Issue, error)

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Valid ID | Active repo | Issue with full details + dependencies | [x] conformance_issue_test.go |
| Invalid ID | Active repo | ErrNotFound | [x] conformance_issue_test.go |
| Empty ID | Active repo | ErrNotFound | [x] conformance_issue_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_test.go |

### ShowMultiple(ids []string) (map[string]*Issue, error)

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| All valid IDs | Active repo | Map of all issues | [x] conformance_test.go |
| Some invalid IDs | Active repo | Map of found issues only | [x] conformance_test.go |
| Empty list | Active repo | Empty map | [x] conformance_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_test.go |

### Create(opts CreateOptions) (*Issue, error)

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Title only | Active repo | Issue with auto-generated ID, status=open | [x] conformance_issue_test.go |
| Title + Type | Active repo | Issue with gt:type label | [x] conformance_issue_test.go |
| Title + Priority | Active repo | Issue with specified priority | [x] conformance_issue_test.go |
| Title + Description | Active repo | Issue with description | [x] conformance_issue_test.go |
| Title + Parent | Active repo | Issue as child of parent | [x] conformance_issue_test.go |
| Title + Actor | Active repo | Issue with created_by set | [x] conformance_issue_test.go |
| Title + Labels | Active repo | Issue with specified labels | [x] conformance_issue_test.go |
| Empty Title | Active repo | Issue with empty title (allowed) | [x] conformance_issue_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_test.go |
| Any | Town context | Issue with hq- prefix | [x] conformance_routing_test.go |
| Any | Rig context | Issue with rig- prefix | [x] conformance_routing_test.go |

### CreateWithID(id string, opts CreateOptions) (*Issue, error)

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Unique ID | Active repo | Issue with specified ID | [x] conformance_issue_test.go |
| Duplicate ID | Active repo | Error: issue already exists | [x] conformance_issue_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_test.go |

### Update(id string, opts UpdateOptions) error

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Title change | Active repo | Success, title updated | [x] conformance_issue_test.go |
| Status change | Active repo | Success, status updated | [x] conformance_issue_test.go |
| Priority change | Active repo | Success, priority updated | [x] conformance_issue_test.go |
| Description change | Active repo | Success, description updated | [x] conformance_issue_test.go |
| Assignee change | Active repo | Success, assignee updated | [x] conformance_issue_test.go |
| Unassign=true | Active repo | Success, assignee cleared | [x] conformance_issue_test.go |
| AddLabels | Active repo | Success, labels added | [x] conformance_issue_test.go |
| RemoveLabels | Active repo | Success, labels removed | [x] conformance_issue_test.go |
| SetLabels | Active repo | Success, labels replaced | [x] conformance_issue_test.go |
| Invalid ID | Active repo | ErrNotFound | [x] conformance_issue_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_test.go |

### Close(ids ...string) error

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Single valid ID | Active repo | Success, status=closed, closed_at set | [x] conformance_issue_test.go |
| Multiple valid IDs | Active repo | Success, all closed | [x] conformance_issue_test.go |
| Invalid ID | Active repo | Silently ignored | [x] conformance_issue_test.go |
| Empty list | Active repo | Success (no-op) | [x] conformance_issue_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_test.go |

### CloseWithReason(reason string, ids ...string) error

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Valid ID + reason | Active repo | Success, status=closed | [x] conformance_issue_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_test.go |

### Delete(ids ...string) error

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Single valid ID | Active repo | Issue removed from database | [x] conformance_issue_test.go |
| Multiple IDs | Active repo | All removed | [x] conformance_issue_test.go |
| ID with dependencies | Active repo | Also removes from dependencies | [x] conformance_issue_test.go |
| Invalid ID | Active repo | Silently ignored | [x] conformance_issue_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_test.go |

### Reopen(id string) error

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Closed issue | Active repo | Success, status=open, closed_at cleared | [x] conformance_issue_test.go |
| Open issue | Active repo | Success (no change) | [x] conformance_issue_test.go |
| Invalid ID | Active repo | ErrNotFound | [x] conformance_issue_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_test.go |

## 2. Dependency Operations

### Ready() ([]*Issue, error)

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| No dependencies | Active repo | All open issues | [x] conformance_dependency_test.go |
| Some blocked | Active repo | Only unblocked open issues | [x] conformance_dependency_test.go |
| All blocked | Active repo | Empty list | [x] conformance_dependency_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_dependency_test.go |

### ReadyWithLabel(label string, limit int) ([]*Issue, error)

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Matching label | Active repo | Ready issues with that label | [x] conformance_dependency_test.go |
| No matches | Active repo | Empty list | [x] conformance_dependency_test.go |
| Limit=2 | Active repo | At most 2 issues | [x] conformance_dependency_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_dependency_test.go |

### Blocked() ([]*Issue, error)

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Some blocked | Active repo | Only blocked open issues | [x] conformance_dependency_test.go |
| None blocked | Active repo | Empty list | [x] conformance_dependency_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_dependency_test.go |

### AddDependency(issue, dependsOn string) error

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Both valid | Active repo | Success, dependency added | [x] conformance_dependency_test.go |
| Issue invalid | Active repo | ErrNotFound | [x] conformance_dependency_test.go |
| DependsOn invalid | Active repo | ErrNotFound | [x] conformance_dependency_test.go |
| Already exists | Active repo | Success (idempotent) | [x] conformance_dependency_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_dependency_test.go |

### RemoveDependency(issue, dependsOn string) error

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Existing dependency | Active repo | Success, dependency removed | [x] conformance_dependency_test.go |
| Non-existent dependency | Active repo | Success (no-op) | [x] conformance_dependency_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_dependency_test.go |

## 3. Sync Operations

### Sync() error

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Clean state | Active repo | Success | [x] conformance_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_test.go |

### SyncFromMain() error

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Any | Active repo | Success | [x] conformance_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_test.go |

### SyncImportOnly() error

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Any | Active repo | Success | [x] conformance_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_test.go |

### GetSyncStatus() (*SyncStatus, error)

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Any | Active repo | SyncStatus with branch, ahead, behind, conflicts | [x] conformance_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_test.go |

## 4. Config Operations

### ConfigGet(key string) (string, error)

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Existing key | Active repo | Value string | [x] conformance_config_test.go |
| Non-existent key | Active repo | Empty string (no error) | [x] conformance_config_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_config_test.go |

### ConfigSet(key, value string) error

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| New key | Active repo | Success, key created | [x] conformance_config_test.go |
| Existing key | Active repo | Success, key overwritten | [x] conformance_config_test.go |
| Empty value | Active repo | Success, value set to empty | [x] conformance_config_test.go |
| Special characters | Active repo | Success | [x] conformance_config_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_config_test.go |

### Init(opts InitOptions) error

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Empty options | Inactive repo | Success, repo initialized | [x] conformance_config_test.go |
| Prefix specified | Any | Success, prefix set | [x] conformance_config_test.go |
| Quiet=true | Any | Success, no output | [x] conformance_config_test.go |
| Already initialized | Active repo | Success (idempotent) | [x] conformance_config_test.go |

### Migrate(opts MigrateOptions) error

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Empty options | Active repo | Success | [x] conformance_config_test.go |
| UpdateRepoID=true | Active repo | Success | [x] conformance_config_test.go |
| Yes=true | Active repo | Success | [x] conformance_config_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_config_test.go |

## 5. Daemon Operations

### DaemonStart() error

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Daemon not running | Any | Success, daemon started | [x] conformance_daemon_test.go |
| Daemon running | Any | Success (idempotent) | [x] conformance_daemon_test.go |

### DaemonStop() error

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Daemon running | Any | Success, daemon stopped | [x] conformance_daemon_test.go |
| Daemon not running | Any | Success (idempotent) | [x] conformance_daemon_test.go |

### DaemonStatus() (*DaemonStatus, error)

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Daemon running | Any | DaemonStatus{Running: true, PID: >0} | [x] conformance_daemon_test.go |
| Daemon stopped | Any | DaemonStatus{Running: false} | [x] conformance_daemon_test.go |

### DaemonHealth() (*DaemonHealth, error)

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Daemon running | Any | DaemonHealth{Status: "healthy"} | [x] conformance_daemon_test.go |
| Daemon stopped | Any | Error: daemon not running | [x] conformance_daemon_test.go |

## 6. Molecule Operations

### MolSeed(opts MolSeedOptions) error

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Patrol=true | Active repo | Success | [x] conformance_molecule_test.go |
| Empty options | Active repo | Success (no-op) | [x] conformance_molecule_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_molecule_test.go |

### MolCurrent(moleculeID string) (*MolCurrentOutput, error)

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Valid ID | Active repo | MolCurrentOutput with step info | [x] conformance_molecule_test.go |
| Invalid ID | Active repo | ErrNotFound | [x] conformance_molecule_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_molecule_test.go |

### MolCatalog() ([]*MoleculeProto, error)

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Protos exist | Active repo | List of prototypes | [x] conformance_molecule_test.go |
| No protos | Active repo | Empty list | [x] conformance_molecule_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_molecule_test.go |

### WispCreate(protoID, actor string) (*Issue, error)

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Valid proto | Active repo | Issue with gt:wisp label | [x] conformance_molecule_test.go |
| Invalid proto | Active repo | ErrNotFound | [x] conformance_molecule_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_molecule_test.go |

### WispList(all bool) ([]*Issue, error)

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| all=false | Active repo | Open wisps only | [x] conformance_molecule_test.go |
| all=true | Active repo | All wisps including closed | [x] conformance_molecule_test.go |
| No wisps | Active repo | Empty list | [x] conformance_molecule_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_molecule_test.go |

### WispGC() error

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Closed wisps exist | Active repo | Closed wisps removed | [x] conformance_molecule_test.go |
| No closed wisps | Active repo | Success (no-op) | [x] conformance_molecule_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_molecule_test.go |

## 7. Gate Operations

### GateShow(gateID string) (*Gate, error)

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Valid ID | Active repo | Gate with status, waiters | [x] conformance_gate_test.go |
| Invalid ID | Active repo | ErrNotFound | [x] conformance_gate_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_gate_test.go |

### GateWait(gateID, notifyAgent string) error

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Gate open | Active repo | Success (immediate return) | [x] conformance_gate_test.go |
| Gate closed + agent | Active repo | Success, agent added to waiters | [x] conformance_gate_test.go |
| Gate closed + empty | Active repo | Success, no waiter added | [x] conformance_gate_test.go |
| Invalid gate | Active repo | ErrNotFound | [x] conformance_gate_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_gate_test.go |

## 8. Swarm Operations

### SwarmStatus(swarmID string) (*SwarmStatus, error)

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Valid ID | Active repo | SwarmStatus with counts, workers | [x] conformance_swarm_test.go |
| Invalid ID | Active repo | ErrNotFound | [x] conformance_swarm_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_swarm_test.go |

## 9. Formula Operations

### FormulaShow(name string) (*Formula, error)

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Valid name | Active repo | Formula with steps, tracked | [x] conformance_formula_test.go |
| With mol- prefix | Active repo | Found without prefix | [x] conformance_formula_test.go |
| Invalid name | Active repo | ErrNotFound | [x] conformance_formula_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_formula_test.go |

### Cook(formulaName string) (*Issue, error)

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Valid formula | Active repo | Issue with gt:formula label | [x] conformance_formula_test.go |
| With mol- prefix | Active repo | Works without prefix | [x] conformance_formula_test.go |
| Invalid formula | Active repo | ErrNotFound | [x] conformance_formula_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_formula_test.go |

### LegAdd(formulaID, stepName string) error

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Valid formula ID | Active repo | Success, step issue created as child | [x] conformance_formula_test.go |
| Invalid ID | Active repo | ErrNotFound | [x] conformance_formula_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_formula_test.go |

### Track(issueID, dependencyID string) error

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Both valid | Active repo | Success, dependency added | [x] conformance_formula_test.go |
| Issue invalid | Active repo | ErrNotFound | [x] conformance_formula_test.go |
| Dependency invalid | Active repo | ErrNotFound | [x] conformance_formula_test.go |
| Any | Inactive repo | Error implied by delegate | [x] conformance_formula_test.go |

### Bond(parentID, childID string) error

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Both valid | Active repo | Success, parent-child linked | [x] conformance_formula_test.go |
| Parent invalid | Active repo | ErrNotFound | [x] conformance_formula_test.go |
| Child invalid | Active repo | ErrNotFound | [x] conformance_formula_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_formula_test.go |

## 10. Agent Operations

### AgentState(beadID, state string) error

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Valid ID | Active repo | Success, agent_state updated | [x] conformance_misc_test.go |
| Invalid ID | Active repo | ErrNotFound | [x] conformance_misc_test.go |
| Various states | Active repo | All valid (spawning, working, done, stuck) | [x] conformance_misc_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_misc_test.go |

## 11. Label Operations

### LabelAdd(id, label string) error

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Valid ID | Active repo | Success, label added | [x] conformance_misc_test.go |
| Label exists | Active repo | Success (idempotent) | [x] conformance_misc_test.go |
| Invalid ID | Active repo | ErrNotFound | [x] conformance_misc_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_misc_test.go |

### LabelRemove(id, label string) error

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Label exists | Active repo | Success, label removed | [x] conformance_misc_test.go |
| Label not exists | Active repo | Success (no-op) | [x] conformance_misc_test.go |
| Invalid ID | Active repo | ErrNotFound | [x] conformance_misc_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_misc_test.go |

## 12. Miscellaneous Operations

### Version() (string, error)

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Any | Any | Version string (non-empty) | [x] conformance_misc_test.go |

### Doctor() (*DoctorReport, error)

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Any | Active repo | DoctorReport with status, checks | [x] conformance_misc_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_misc_test.go |

### Prime() (string, error)

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Any | Active repo | PRIME.md content (non-empty) | [x] conformance_misc_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_misc_test.go |

### SlotShow(id string) (*Slot, error)

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Valid ID | Active repo | Slot info | [x] conformance_misc_test.go |
| Invalid ID | Active repo | ErrNotFound | [x] conformance_misc_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_misc_test.go |

### Stats() (string, error)

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Issues exist | Active repo | Summary text with counts | [x] conformance_misc_test.go |
| No issues | Active repo | Summary text (zeros) | [x] conformance_misc_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_misc_test.go |

### StatsJSON() (*RepoStats, error)

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Issues exist | Active repo | RepoStats with accurate counts | [x] conformance_misc_test.go |
| No issues | Active repo | RepoStats (zeros) | [x] conformance_misc_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_misc_test.go |

### Burn(opts BurnOptions) error

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Valid options | Active repo | Success | [x] conformance_misc_test.go |
| Empty options | Active repo | Success (no-op) | [x] conformance_misc_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_misc_test.go |

### Comment(id, message string) error

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| Valid ID | Active repo | Success, message appended to description | [x] conformance_misc_test.go |
| Invalid ID | Active repo | ErrNotFound | [x] conformance_misc_test.go |
| Empty message | Active repo | Success | [x] conformance_misc_test.go |
| Any | Inactive repo | Error: not a beads repository | [x] conformance_misc_test.go |

### IsBeadsRepo() bool

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| N/A | Active repo | true | [x] conformance_routing_test.go |
| N/A | Inactive repo | false | [x] conformance_routing_test.go |
| N/A | Directory with .beads | true | [x] conformance_routing_test.go |
| N/A | Directory without .beads | false | [x] conformance_routing_test.go |
| N/A | Directory with redirect | true (if target exists) | [x] conformance_routing_test.go |

### Run(args ...string) ([]byte, error)

| Input Permutation | Context | Expected Output | Test |
|-------------------|---------|-----------------|------|
| "list" | Active repo | JSON output | [x] conformance_misc_test.go |
| "show", id | Active repo | JSON output | [x] conformance_misc_test.go |
| "ready" | Active repo | JSON output | [x] conformance_misc_test.go |
| "blocked" | Active repo | JSON output | [x] conformance_misc_test.go |
| "version" | Any | Version string | [x] conformance_misc_test.go |
| Unknown command | Any | Error: unknown command | [x] conformance_misc_test.go |
| No args | Any | Error: no command specified | [x] conformance_misc_test.go |

## 13. Routing Context Tests

### Town/Rig Isolation

| Scenario | Expected Behavior | Test |
|----------|-------------------|------|
| Create in town, list from town | Sees town issues | [x] conformance_routing_test.go |
| Create in rig, list from rig | Sees rig issues | [x] conformance_routing_test.go |
| Create in town, list from rig | Does NOT see town issues | [x] conformance_routing_test.go |
| Prefix from town beads | Uses hq- prefix | [x] conformance_routing_test.go |
| Prefix from rig beads | Uses rig- prefix | [x] conformance_routing_test.go |

### Redirect Following

| Scenario | Expected Behavior | Test |
|----------|-------------------|------|
| Crew dir with redirect | Follows to rig beads | [x] conformance_routing_test.go |
| Chain: crew -> rig -> mayor | Follows full chain | [x] conformance_routing_test.go |
| Create from crew, list from rig | Both see same issues | [x] conformance_routing_test.go |

### BEADS_DIR Override

| Scenario | Expected Behavior | Test |
|----------|-------------------|------|
| BEADS_DIR set | Overrides all redirect logic | [x] conformance_routing_test.go |
| BEADS_DIR cleared | Resumes normal resolution | [x] conformance_routing_test.go |

## Coverage Summary

| Category | Total Tests | Coverage |
|----------|-------------|----------|
| Issue Operations | 40+ | 100% |
| Dependency Operations | 15+ | 100% |
| Sync Operations | 8+ | 100% |
| Config Operations | 15+ | 100% |
| Daemon Operations | 15+ | 100% |
| Molecule Operations | 20+ | 100% |
| Gate Operations | 10+ | 100% |
| Swarm Operations | 7+ | 100% |
| Formula Operations | 20+ | 100% |
| Agent Operations | 5+ | 100% |
| Label Operations | 8+ | 100% |
| Miscellaneous Operations | 30+ | 100% |
| Routing Tests | 15+ | 100% |

**Total: 200+ conformance tests covering all API methods and permutations.**
