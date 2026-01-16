package beads

import "errors"

// Common errors
// ZFC: Only define errors that don't require stderr parsing for decisions.
// ErrNotARepo and ErrSyncConflict were removed - agents should handle these directly.
var (
	ErrNotInstalled = errors.New("bd not installed: run 'pip install beads-cli' or see https://github.com/anthropics/beads")
	ErrNotFound     = errors.New("issue not found")
)

// Issue represents a beads issue.
type Issue struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Priority    int      `json:"priority"`
	Type        string   `json:"issue_type"`
	CreatedAt   string   `json:"created_at"`
	CreatedBy   string   `json:"created_by,omitempty"`
	UpdatedAt   string   `json:"updated_at"`
	ClosedAt    string   `json:"closed_at,omitempty"`
	Parent      string   `json:"parent,omitempty"`
	Assignee    string   `json:"assignee,omitempty"`
	Children    []string `json:"children,omitempty"`
	DependsOn   []string `json:"depends_on,omitempty"`
	Blocks      []string `json:"blocks,omitempty"`
	BlockedBy   []string `json:"blocked_by,omitempty"`
	Labels      []string `json:"labels,omitempty"`

	// Agent bead slots (type=agent only)
	HookBead   string `json:"hook_bead,omitempty"`   // Current work attached to agent's hook
	RoleBead   string `json:"role_bead,omitempty"`   // Role definition bead (shared)
	AgentState string `json:"agent_state,omitempty"` // Agent lifecycle state (spawning, working, done, stuck)

	// Counts from list output
	DependencyCount int `json:"dependency_count,omitempty"`
	DependentCount  int `json:"dependent_count,omitempty"`
	BlockedByCount  int `json:"blocked_by_count,omitempty"`

	// Detailed dependency info from show output
	Dependencies []IssueDep `json:"dependencies,omitempty"`
	Dependents   []IssueDep `json:"dependents,omitempty"`
}

// IssueDep represents a dependency or dependent issue with its relation.
type IssueDep struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Status         string `json:"status"`
	Priority       int    `json:"priority"`
	Type           string `json:"issue_type"`
	Assignee       string `json:"assignee,omitempty"`
	DependencyType string `json:"dependency_type,omitempty"`
}

// ListOptions specifies filters for listing issues.
type ListOptions struct {
	Status       string   // "open", "closed", "all"
	Type         string   // Deprecated: use Label instead. "task", "bug", "feature", "epic"
	Label        string   // Label filter (e.g., "gt:agent", "gt:merge-request")
	Labels       []string // Multiple label filters (all must match)
	Priority     int      // 0-4, -1 for no filter
	Parent       string   // filter by parent ID
	Assignee     string   // filter by assignee (e.g., "gastown/Toast")
	NoAssignee   bool     // filter for issues with no assignee
	Limit        int      // max issues to return (0 = default)
	Tag          string   // tag filter (e.g., "escalation")
	CreatedAfter string   // filter for issues created after this time (e.g., "-1h", "-24h")
	All          bool     // include closed issues
}

// CreateOptions specifies options for creating an issue.
type CreateOptions struct {
	Title       string
	Type        string // "task", "bug", "feature", "epic"
	Priority    int    // 0-4
	Description string
	Parent      string
	Actor       string   // Who is creating this issue (populates created_by)
	Ephemeral   bool     // Create as ephemeral (wisp) - not exported to JSONL
	Labels      []string // Additional labels to add
}

// UpdateOptions specifies options for updating an issue.
type UpdateOptions struct {
	Title        *string
	Status       *string
	Priority     *int
	Description  *string
	Assignee     *string
	Unassign     bool     // Remove assignee
	AddLabels    []string // Labels to add
	RemoveLabels []string // Labels to remove
	SetLabels    []string // Labels to set (replaces all existing)
	Notes        string   // Add notes
}

// SyncStatus represents the sync status of the beads repository.
type SyncStatus struct {
	Branch    string   `json:"branch"`
	Ahead     int      `json:"ahead"`
	Behind    int      `json:"behind"`
	Conflicts []string `json:"conflicts"`
}

// InitOptions specifies options for bd init.
type InitOptions struct {
	Prefix string // Issue ID prefix (e.g., "hq", "toast")
	Quiet  bool   // Suppress output
}

// MigrateOptions specifies options for bd migrate.
type MigrateOptions struct {
	UpdateRepoID bool // Update repository fingerprint
	Yes          bool // Non-interactive mode
}

// DaemonStatus represents the bd daemon status.
type DaemonStatus struct {
	Running bool   `json:"running"`
	PID     int    `json:"pid,omitempty"`
	Uptime  string `json:"uptime,omitempty"`
}

// DaemonHealth represents daemon health information.
type DaemonHealth struct {
	Status    string `json:"status"`
	Latency   int    `json:"latency_ms"`
	QueueSize int    `json:"queue_size"`
}

// MolCurrentOutput represents the output from bd mol current --json.
// Note: This is different from MoleculeStep in molecule.go which represents
// parsed step definitions from molecule markdown.
type MolCurrentOutput struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	StepNum  int    `json:"step_num"`
	StepName string `json:"step_name"`
}

// MoleculeProto represents a molecule prototype.
type MoleculeProto struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Steps       []string `json:"steps"`
}

// MolSeedOptions specifies options for bd mol seed.
type MolSeedOptions struct {
	Patrol bool // Seed patrol molecule
}

// WispCreateOptions specifies options for creating a wisp.
type WispCreateOptions struct {
	ProtoID   string            // Proto/formula ID to create wisp from
	Actor     string            // Actor name for audit trail
	Variables map[string]string // Variable substitutions (--var key=value)
}

// CloseOptions specifies options for closing issues.
type CloseOptions struct {
	Reason  string // Reason for closing (--reason)
	Session string // Session ID for audit trail (--session)
	Force   bool   // Force close even if pinned (--force)
}

// Gate represents a gate for synchronization.
type Gate struct {
	ID          string   `json:"id"`
	Status      string   `json:"status"` // "open", "closed"
	CloseReason string   `json:"close_reason,omitempty"`
	Waiters     []string `json:"waiters,omitempty"`
	OpenedAt    string   `json:"opened_at,omitempty"`
	ClosedAt    string   `json:"closed_at,omitempty"`
}

// SwarmStatus represents the status of a swarm.
type SwarmStatus struct {
	ID         string         `json:"id"`
	Status     string         `json:"status"`
	TotalTasks int            `json:"total_tasks"`
	Completed  int            `json:"completed"`
	InProgress int            `json:"in_progress"`
	Blocked    int            `json:"blocked"`
	Ready      int            `json:"ready"`
	Workers    []*SwarmWorker `json:"workers,omitempty"`
	// Task arrays with details (from swarm status --json)
	ReadyTasks      []*SwarmTask `json:"ready,omitempty"`
	ActiveTasks     []*SwarmTask `json:"active,omitempty"`
	BlockedTasks    []*SwarmTask `json:"blocked,omitempty"`
	CompletedTasks  []*SwarmTask `json:"completed,omitempty"`
}

// SwarmTask represents a task in a swarm with basic info.
type SwarmTask struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// SwarmWorker represents a worker in a swarm.
type SwarmWorker struct {
	ID     string `json:"id"`
	Task   string `json:"task"`
	Status string `json:"status"`
}

// Formula represents a workflow formula definition.
type Formula struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Steps       []string `json:"steps"`
	Tracked     []string `json:"tracked"`
}

// DoctorReport represents the output of bd doctor.
type DoctorReport struct {
	Status   string         `json:"status"`
	Checks   []*DoctorCheck `json:"checks"`
	Warnings []string       `json:"warnings"`
	Errors   []string       `json:"errors"`
}

// DoctorCheck represents a single doctor check result.
type DoctorCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// Slot represents an agent hook slot.
type Slot struct {
	ID       string `json:"id"`
	IssueID  string `json:"issue_id"`
	Agent    string `json:"agent"`
	HookedAt string `json:"hooked_at"`
}

// RepoStats represents repository statistics from bd stats --json.
// The bd output wraps stats in a "summary" field.
type RepoStats struct {
	Summary RepoStatsSummary `json:"summary"`
	// Legacy fields at root level for backwards compatibility
	TotalIssues    int            `json:"total_issues"`
	OpenIssues     int            `json:"open_issues"`
	ClosedIssues   int            `json:"closed_issues"`
	IssuesByType   map[string]int `json:"issues_by_type"`
	IssuesByStatus map[string]int `json:"issues_by_status"`
}

// RepoStatsSummary contains the actual stats from bd stats --json.
type RepoStatsSummary struct {
	TotalIssues    int `json:"total_issues"`
	OpenIssues     int `json:"open_issues"`
	InProgressIssues int `json:"in_progress_issues"`
	ClosedIssues   int `json:"closed_issues"`
	BlockedIssues  int `json:"blocked_issues"`
	DeferredIssues int `json:"deferred_issues"`
	ReadyIssues    int `json:"ready_issues"`
	TombstoneIssues int `json:"tombstone_issues"`
	PinnedIssues   int `json:"pinned_issues"`
}

// BurnOptions specifies options for bd burn (cost tracking).
type BurnOptions struct {
	SessionID string
	Tokens    int
	Cost      float64
	Model     string
}

// SearchOptions specifies options for bd search.
type SearchOptions struct {
	Status string // "open", "closed", "all"
	Type   string // "task", "bug", "feature", "epic", etc.
	Limit  int    // max results (0 = default)
}
