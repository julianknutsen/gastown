package polecat

// Compile-time interface compliance checks.
var (
	_ Backend = (*Manager)(nil)
	_ Backend = (*RemoteManager)(nil)
)

// Backend abstracts polecat creation operations.
// LocalBackend (Manager) uses local filesystem and git commands.
// RemoteBackend (RemoteManager) uses SSH for remote operations.
//
// This interface enables the same spawn code path to work for both
// local and remote polecats by abstracting filesystem and git operations.
type Backend interface {
	// AllocateName allocates a name from the name pool.
	AllocateName() (string, error)

	// Exists checks if a polecat exists.
	Exists(name string) bool

	// Get returns a specific polecat by name.
	Get(name string) (*Polecat, error)

	// AddWithOptions creates a new polecat with the specified options.
	AddWithOptions(name string, opts AddOptions) (*Polecat, error)

	// Remove deletes a polecat.
	Remove(name string, force bool) error

	// Start starts the polecat's session.
	// The agent is resolved from config based on the rig's role_agents setting.
	Start(name string) error

	// RigName returns the rig name for this backend.
	RigName() string

	// SessionName returns the tmux session name for a polecat.
	SessionName(name string) string
}
