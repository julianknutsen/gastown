// Package beads provides a wrapper for the bd (beads) CLI.
package beads

// BeadsOps defines the interface for beads operations.
// All methods match the Implementation and are tested against both
// the real bd CLI and the Double.
type BeadsOps interface {
	// === Issue Operations ===

	// List returns issues matching the given options.
	List(opts ListOptions) ([]*Issue, error)

	// Show returns detailed information about an issue.
	Show(id string) (*Issue, error)

	// ShowMultiple fetches multiple issues by ID in a single call.
	ShowMultiple(ids []string) (map[string]*Issue, error)

	// Create creates a new issue and returns it.
	Create(opts CreateOptions) (*Issue, error)

	// CreateWithID creates an issue with a specific ID.
	CreateWithID(id string, opts CreateOptions) (*Issue, error)

	// Update updates an existing issue.
	Update(id string, opts UpdateOptions) error

	// Close closes one or more issues.
	Close(ids ...string) error

	// CloseWithReason closes issues with a reason.
	CloseWithReason(reason string, ids ...string) error

	// CloseWithOptions closes issues with full options (reason, session).
	CloseWithOptions(opts CloseOptions, ids ...string) error

	// Delete permanently removes issues.
	Delete(ids ...string) error

	// DeleteWithOptions permanently removes issues with options (e.g., --force).
	DeleteWithOptions(opts DeleteOptions, ids ...string) error

	// Reopen reopens a closed issue.
	Reopen(id string) error

	// === Dependency Operations ===

	// Ready returns issues that are ready to work (not blocked).
	Ready() ([]*Issue, error)

	// ReadyWithLabel returns ready issues filtered by label.
	ReadyWithLabel(label string, limit int) ([]*Issue, error)

	// Blocked returns issues that are blocked by dependencies.
	Blocked() ([]*Issue, error)

	// AddDependency adds a dependency: issue depends on dependsOn.
	AddDependency(issue, dependsOn string) error

	// AddDependencyWithType adds a typed dependency (e.g., "tracks").
	AddDependencyWithType(issue, dependsOn, depType string) error

	// RemoveDependency removes a dependency.
	RemoveDependency(issue, dependsOn string) error

	// === Sync Operations ===

	// Sync syncs beads with remote.
	Sync() error

	// SyncFromMain syncs beads updates from main branch.
	SyncFromMain() error

	// SyncImportOnly syncs only importing changes (no export).
	SyncImportOnly() error

	// GetSyncStatus returns the sync status without performing a sync.
	GetSyncStatus() (*SyncStatus, error)

	// === Config Operations ===

	// ConfigGet returns a config value.
	ConfigGet(key string) (string, error)

	// ConfigSet sets a config value.
	ConfigSet(key, value string) error

	// Init initializes a beads repository.
	Init(opts InitOptions) error

	// Migrate runs database migrations.
	Migrate(opts MigrateOptions) error

	// === Daemon Operations ===

	// DaemonStart starts the bd daemon.
	DaemonStart() error

	// DaemonStop stops the bd daemon.
	DaemonStop() error

	// DaemonStatus returns the daemon status.
	DaemonStatus() (*DaemonStatus, error)

	// DaemonHealth returns daemon health info.
	DaemonHealth() (*DaemonHealth, error)

	// === Molecule Operations ===

	// MolSeed seeds molecules (e.g., patrol).
	MolSeed(opts MolSeedOptions) error

	// MolCurrent returns the current step of a molecule.
	MolCurrent(moleculeID string) (*MolCurrentOutput, error)

	// MolCatalog lists available molecule prototypes.
	MolCatalog() ([]*MoleculeProto, error)

	// WispCreate creates a wisp from a prototype.
	WispCreate(protoID, actor string) (*Issue, error)

	// WispCreateWithOptions creates a wisp with full options including variables.
	WispCreateWithOptions(opts WispCreateOptions) (*Issue, error)

	// WispList lists all wisps.
	WispList(all bool) ([]*Issue, error)

	// WispGC garbage collects old wisps.
	WispGC() error

	// MolBond bonds a wisp to a bead, creating a compound.
	MolBond(wispID, beadID string) (*Issue, error)

	// === Gate Operations ===

	// GateShow returns gate status.
	GateShow(gateID string) (*Gate, error)

	// GateWait waits for a gate to open, optionally notifying an agent.
	GateWait(gateID, notifyAgent string) error

	// GateList returns all gates, optionally including closed ones.
	GateList(all bool) ([]*Gate, error)

	// GateResolve manually closes/resolves a gate.
	GateResolve(gateID string) error

	// GateAddWaiter adds a waiter to a gate.
	GateAddWaiter(gateID, waiterID string) error

	// GateCheck evaluates all open gates and closes resolved ones.
	GateCheck() error

	// === Swarm Operations ===

	// SwarmStatus returns swarm status.
	SwarmStatus(swarmID string) (*SwarmStatus, error)

	// SwarmCreate creates a swarm molecule from an epic.
	SwarmCreate(epicID string) (*Issue, error)

	// SwarmList returns all swarm molecules.
	SwarmList() ([]*Issue, error)

	// SwarmValidate validates epic structure for swarming.
	SwarmValidate(epicID string) error

	// === Formula Operations ===

	// FormulaShow returns formula definition.
	FormulaShow(name string) (*Formula, error)

	// FormulaList returns all available formulas.
	FormulaList() ([]*Formula, error)

	// Cook executes a formula.
	Cook(formulaName string) (*Issue, error)

	// LegAdd adds a leg (step) to a formula instance.
	LegAdd(formulaID, stepName string) error

	// === Agent Operations ===

	// AgentState sets the state of an agent bead.
	AgentState(beadID, state string) error

	// === Label Operations ===

	// LabelAdd adds a label to an issue.
	LabelAdd(id, label string) error

	// LabelRemove removes a label from an issue.
	LabelRemove(id, label string) error

	// === Slot Operations ===

	// SlotShow returns slot information.
	SlotShow(id string) (*Slot, error)

	// SlotSet sets a slot on an agent bead.
	SlotSet(agentID, slotName, beadID string) error

	// SlotClear clears a slot on an agent bead.
	SlotClear(agentID, slotName string) error

	// === Search Operations ===

	// Search searches issues by text query.
	Search(query string, opts SearchOptions) ([]*Issue, error)

	// === Miscellaneous Operations ===

	// Version returns the bd version.
	Version() (string, error)

	// Doctor runs diagnostics.
	Doctor() (*DoctorReport, error)

	// Prime returns the PRIME.md content.
	Prime() (string, error)

	// Stats returns repository statistics as text.
	Stats() (string, error)

	// StatsJSON returns repository statistics as structured data.
	StatsJSON() (*RepoStats, error)

	// Flush flushes pending writes to the database.
	Flush() error

	// Burn records cost information.
	Burn(opts BurnOptions) error

	// Comment adds a comment to an issue.
	Comment(id, message string) error

	// IsBeadsRepo checks if the working directory is a beads repository.
	IsBeadsRepo() bool

	// Run executes an arbitrary bd command (escape hatch).
	Run(args ...string) ([]byte, error)
}

// Compile-time check that Implementation implements BeadsOps.
var _ BeadsOps = (*Implementation)(nil)
