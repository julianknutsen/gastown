package polecat

// Compile-time interface compliance check.
var _ Backend = (*Manager)(nil)

// Backend abstracts polecat creation operations.
// Manager implements this interface and handles both local and remote polecats
// via the ManagerDeps abstraction (TargetFS, LocalFS, GitOps).
//
// This interface enables the same spawn code path to work for both
// local and remote polecats.
type Backend interface {
	// AllocateName allocates a name from the name pool.
	AllocateName() (string, error)

	// Exists checks if a polecat exists.
	Exists(name string) bool

	// Get returns a specific polecat by name.
	Get(name string) (*Polecat, error)

	// List returns all polecats in this rig.
	List() ([]*Polecat, error)

	// AddWithOptions creates a new polecat with the specified options.
	AddWithOptions(name string, opts AddOptions) (*Polecat, error)

	// Remove deletes a polecat.
	Remove(name string, force bool) error

	// RemoveWithOptions removes a polecat with control over safety checks.
	// force: bypass uncommitted changes check (but not stashes/unpushed)
	// nuclear: bypass ALL safety checks including stashes and unpushed commits
	RemoveWithOptions(name string, force, nuclear bool) error

	// Start starts the polecat's session.
	// The agent is resolved from config based on the rig's role_agents setting.
	Start(name string) error

	// RigName returns the rig name for this backend.
	RigName() string

	// SessionName returns the tmux session name for a polecat.
	SessionName(name string) string

	// GitState returns the git state of a polecat's worktree.
	// Used for pre-kill verification to ensure no work is lost.
	GitState(name string) (*GitState, error)

	// Sync runs bd sync in the polecat's worktree.
	// fromMain: only pull changes, don't push
	Sync(name string, fromMain bool) error

	// CleanupStaleBranches removes orphaned polecat branches.
	// Returns the number of branches deleted.
	CleanupStaleBranches() (int, error)

	// DetectStalePolecats identifies polecats that may need cleanup.
	// threshold: number of commits behind main to consider stale
	DetectStalePolecats(threshold int) ([]*StalenessInfo, error)
}

// GitState represents the git state of a polecat's worktree.
type GitState struct {
	Clean            bool     `json:"clean"`
	UncommittedFiles []string `json:"uncommitted_files"`
	UnpushedCommits  int      `json:"unpushed_commits"`
	StashCount       int      `json:"stash_count"`
}
