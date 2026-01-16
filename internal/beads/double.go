package beads

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// doubleDep represents a dependency with its type.
type doubleDep struct {
	ID   string // The dependency issue ID
	Type string // "blocks" is blocking; all other types (tracks, related, etc.) are non-blocking
}

// doubleDB is an in-memory database for a single beads directory.
// Double can contain multiple of these to simulate multi-rig setups.
type doubleDB struct {
	prefix   string
	isActive bool

	// Issue storage
	issues   map[string]*Issue
	nextID   int

	// Dependency graph: issue -> dependencies (with type information)
	dependencies map[string][]doubleDep

	// Config storage
	config map[string]string

	// Daemon state
	daemonRunning bool
	daemonPID     int

	// Molecule state
	moleculeSteps map[string]*MolCurrentOutput
	prototypes    map[string]*MoleculeProto

	// Gate state
	gates map[string]*Gate

	// Swarm state
	swarms map[string]*SwarmStatus

	// Formula state
	formulas map[string]*Formula

	// Sync state
	syncBranch string
	syncAhead  int
	syncBehind int
	conflicts  []string
}

// newDoubleDB creates a new in-memory database with the given prefix.
func newDoubleDB(prefix string) *doubleDB {
	return &doubleDB{
		prefix:        prefix,
		isActive:      true,
		issues:        make(map[string]*Issue),
		nextID:        1,
		dependencies:  make(map[string][]doubleDep),
		config:        make(map[string]string),
		moleculeSteps: make(map[string]*MolCurrentOutput),
		prototypes:    make(map[string]*MoleculeProto),
		gates:         make(map[string]*Gate),
		swarms:        make(map[string]*SwarmStatus),
		formulas:      make(map[string]*Formula),
	}
}

// Double is an in-memory test double that implements BeadsOps.
// It faithfully mimics bd behavior including quirks like status transitions
// and dependency blocking.
//
// Double emulates the real bd infrastructure:
// - Multiple databases (like multiple rigs in a town)
// - WorkDir-based database selection (like running bd from different directories)
// - Prefix-based cross-rig routing (like routes.jsonl in real bd)
//
// Usage patterns:
//
//	// Simple single-database test
//	d := NewDouble()
//
//	// Multi-rig test with workDir-based routing (like real bd)
//	d := NewDoubleWithPrefix("gt")
//	d.AddDatabase("ap")
//	d.ConfigureRoute("/path/to/gastown", "gt")
//	d.ConfigureRoute("/path/to/ai_platform", "ap")
//	d.SetWorkDir("/path/to/gastown")  // Now creates gt- issues, lists gt- only
//	d.SetWorkDir("/path/to/ai_platform")  // Now creates ap- issues, lists ap- only
//
//	// Cross-rig operations still work via prefix routing
//	d.Show("ap-123")  // Routes to ap database even when workDir is gastown
type Double struct {
	mu sync.RWMutex

	// Multi-database support
	databases     map[string]*doubleDB // prefix -> database
	currentPrefix string               // which database to use for Create/List

	// WorkDir-based routing (emulates real bd behavior)
	workDirRoutes map[string]string // workDir -> prefix
	currentWorkDir string           // current working directory

	// Legacy compatibility: direct access to single database fields
	// These reference the current database's fields for backwards compatibility
	db *doubleDB
}

// NewDouble creates a new test double with a single "test-" prefix database.
func NewDouble() *Double {
	return NewDoubleWithPrefix("test")
}

// NewDoubleWithPrefix creates a new test double with a specific prefix.
func NewDoubleWithPrefix(prefix string) *Double {
	db := newDoubleDB(prefix)
	d := &Double{
		databases:     make(map[string]*doubleDB),
		currentPrefix: prefix,
		workDirRoutes: make(map[string]string),
		db:            db,
	}
	d.databases[prefix] = db
	return d
}

// AddDatabase adds a new database with the given prefix.
// This enables multi-rig testing scenarios.
func (d *Double) AddDatabase(prefix string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, exists := d.databases[prefix]; !exists {
		d.databases[prefix] = newDoubleDB(prefix)
	}
}

// ConfigureRoute maps a working directory to a database prefix.
// This emulates the routes.jsonl configuration in real bd.
// After calling ConfigureRoute, SetWorkDir(workDir) will select the database
// with the configured prefix.
func (d *Double) ConfigureRoute(workDir, prefix string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.workDirRoutes[workDir] = prefix
	// Ensure database exists
	if _, exists := d.databases[prefix]; !exists {
		d.databases[prefix] = newDoubleDB(prefix)
	}
}

// SetWorkDir sets the current working directory and selects the appropriate
// database based on configured routes. This emulates running bd from different
// directories in a multi-rig setup.
//
// If workDir has a configured route (via ConfigureRoute), uses that database.
// Otherwise falls back to the current database.
func (d *Double) SetWorkDir(workDir string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.currentWorkDir = workDir
	if prefix, ok := d.workDirRoutes[workDir]; ok {
		d.currentPrefix = prefix
		if db, exists := d.databases[prefix]; exists {
			d.db = db
		}
	}
}

// WorkDir returns the current working directory.
func (d *Double) WorkDir() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.currentWorkDir
}

// SetCurrentPrefix sets which database to use for operations that don't
// specify a bead ID (like Create, List). This simulates changing the
// working directory to a different rig.
//
// Prefer SetWorkDir() for more realistic emulation of bd behavior.
func (d *Double) SetCurrentPrefix(prefix string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.currentPrefix = prefix
	if db, ok := d.databases[prefix]; ok {
		d.db = db
	}
}

// getDB returns the database for the given bead ID by extracting its prefix.
// Falls back to current database if prefix not found.
func (d *Double) getDB(beadID string) *doubleDB {
	prefix := extractBeadPrefix(beadID)
	if prefix != "" {
		if db, ok := d.databases[prefix]; ok {
			return db
		}
	}
	return d.db
}

// currentDB returns the current database (for Create, List, etc.).
func (d *Double) currentDB() *doubleDB {
	return d.db
}

// extractBeadPrefix extracts the prefix from a bead ID (e.g., "ap-123" → "ap").
// Returns empty string if no valid prefix pattern is found.
func extractBeadPrefix(beadID string) string {
	idx := strings.Index(beadID, "-")
	if idx == -1 || idx == 0 {
		return ""
	}
	return beadID[:idx]
}

// SetActive sets whether the current database represents an active beads repo.
func (d *Double) SetActive(active bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.db.isActive = active
}

// AddPrototype adds a molecule prototype to the current database.
func (d *Double) AddPrototype(proto *MoleculeProto) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.db.prototypes[proto.ID] = proto
}

// AddFormula adds a formula definition to the current database.
func (d *Double) AddFormula(formula *Formula) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.db.formulas[formula.Name] = formula
}

// AddGate adds a gate to the current database.
func (d *Double) AddGate(gate *Gate) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.db.gates[gate.ID] = gate
}

// AddSwarm adds a swarm status to the current database.
func (d *Double) AddSwarm(swarm *SwarmStatus) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.db.swarms[swarm.ID] = swarm
}

// SetDaemonRunning sets the daemon running state for the current database.
func (d *Double) SetDaemonRunning(running bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.db.daemonRunning = running
	if running {
		d.db.daemonPID = 12345 // Fake PID
	} else {
		d.db.daemonPID = 0
	}
}

// SetSyncStatus sets the sync status for the current database.
func (d *Double) SetSyncStatus(branch string, ahead, behind int, conflicts []string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.db.syncBranch = branch
	d.db.syncAhead = ahead
	d.db.syncBehind = behind
	d.db.conflicts = conflicts
}

// generateID generates the next issue ID using the current database's prefix.
func (d *Double) generateID() string {
	id := fmt.Sprintf("%s-%d", d.db.prefix, d.db.nextID)
	d.db.nextID++
	return id
}

// now returns the current time as a string.
func now() string {
	return time.Now().Format(time.RFC3339)
}

// === Issue Operations ===

// List returns issues matching the given options.
func (d *Double) List(opts ListOptions) ([]*Issue, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	db := d.db
	if !db.isActive {
		return nil, fmt.Errorf("not a beads repository")
	}

	var result []*Issue
	for _, issue := range db.issues {
		// Filter by status
		if opts.Status != "" && opts.Status != "all" {
			if opts.Status == "open" {
				// "open" matches open, in_progress, hooked, pinned
				if issue.Status != "open" && issue.Status != "in_progress" &&
					issue.Status != "hooked" && issue.Status != "pinned" {
					continue
				}
			} else if issue.Status != opts.Status {
				continue
			}
		}

		// Filter by label
		if opts.Label != "" {
			found := false
			for _, l := range issue.Labels {
				if l == opts.Label {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Filter by type (deprecated, convert to label check)
		if opts.Type != "" {
			labelToFind := "gt:" + opts.Type
			found := false
			for _, l := range issue.Labels {
				if l == labelToFind {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Filter by priority
		if opts.Priority >= 0 && issue.Priority != opts.Priority {
			continue
		}

		// Filter by parent
		if opts.Parent != "" && issue.Parent != opts.Parent {
			continue
		}

		// Filter by assignee
		if opts.Assignee != "" && issue.Assignee != opts.Assignee {
			continue
		}

		// Make a copy to avoid data races
		issueCopy := *issue
		result = append(result, &issueCopy)
	}

	// Sort by created_at descending (newest first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt > result[j].CreatedAt
	})

	// Apply limit
	if opts.Limit > 0 && len(result) > opts.Limit {
		result = result[:opts.Limit]
	}

	return result, nil
}

// Show returns detailed information about an issue.
func (d *Double) Show(id string) (*Issue, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	db := d.getDB(id)
	if !db.isActive {
		return nil, fmt.Errorf("not a beads repository")
	}

	issue, ok := db.issues[id]
	if !ok {
		return nil, ErrNotFound
	}

	// Make a copy
	issueCopy := *issue

	// Populate dependency info
	if deps, ok := db.dependencies[id]; ok {
		var depIDs []string
		for _, dep := range deps {
			if depIssue, ok := db.issues[dep.ID]; ok {
				issueCopy.Dependencies = append(issueCopy.Dependencies, IssueDep{
					ID:       depIssue.ID,
					Title:    depIssue.Title,
					Status:   depIssue.Status,
					Priority: depIssue.Priority,
					Type:     depIssue.Type,
				})
			}
			depIDs = append(depIDs, dep.ID)
		}
		issueCopy.DependsOn = depIDs
	}

	// Find issues that depend on this one
	for otherID, deps := range db.dependencies {
		for _, dep := range deps {
			if dep.ID == id {
				if depIssue, ok := db.issues[otherID]; ok {
					issueCopy.Dependents = append(issueCopy.Dependents, IssueDep{
						ID:       depIssue.ID,
						Title:    depIssue.Title,
						Status:   depIssue.Status,
						Priority: depIssue.Priority,
						Type:     depIssue.Type,
					})
				}
			}
		}
	}

	return &issueCopy, nil
}

// ShowMultiple fetches multiple issues by ID.
func (d *Double) ShowMultiple(ids []string) (map[string]*Issue, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// For simplicity, use the current db's isActive check
	// Each ID lookup routes to the appropriate db
	if !d.db.isActive {
		return nil, fmt.Errorf("not a beads repository")
	}

	result := make(map[string]*Issue)
	for _, id := range ids {
		db := d.getDB(id)
		if issue, ok := db.issues[id]; ok {
			issueCopy := *issue
			result[id] = &issueCopy
		}
	}

	return result, nil
}

// Create creates a new issue and returns it.
func (d *Double) Create(opts CreateOptions) (*Issue, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	db := d.db
	if !db.isActive {
		return nil, fmt.Errorf("not a beads repository")
	}

	id := d.generateID()
	return d.createIssue(id, opts)
}

// CreateWithID creates an issue with a specific ID.
// Note: Like real bd, this creates in the current database, not routing by ID prefix.
func (d *Double) CreateWithID(id string, opts CreateOptions) (*Issue, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	db := d.db
	if !db.isActive {
		return nil, fmt.Errorf("not a beads repository")
	}

	if _, exists := db.issues[id]; exists {
		return nil, fmt.Errorf("issue %s already exists", id)
	}

	return d.createIssue(id, opts)
}

// createIssue is the internal implementation for creating issues.
// Uses the current database (d.db).
func (d *Double) createIssue(id string, opts CreateOptions) (*Issue, error) {
	db := d.db
	labels := make([]string, 0, len(opts.Labels)+1)
	if opts.Type != "" {
		labels = append(labels, "gt:"+opts.Type)
	}
	labels = append(labels, opts.Labels...)

	issue := &Issue{
		ID:          id,
		Title:       opts.Title,
		Description: opts.Description,
		Status:      "open",
		Priority:    opts.Priority,
		Type:        opts.Type,
		CreatedAt:   now(),
		UpdatedAt:   now(),
		CreatedBy:   opts.Actor,
		Parent:      opts.Parent,
		Labels:      labels,
	}

	db.issues[id] = issue

	// Update parent's children list
	if opts.Parent != "" {
		if parent, ok := db.issues[opts.Parent]; ok {
			parent.Children = append(parent.Children, id)
		}
	}

	// Return a copy
	issueCopy := *issue
	return &issueCopy, nil
}

// Update updates an existing issue.
func (d *Double) Update(id string, opts UpdateOptions) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	db := d.getDB(id)
	if !db.isActive {
		return fmt.Errorf("not a beads repository")
	}

	issue, ok := db.issues[id]
	if !ok {
		return ErrNotFound
	}

	if opts.Title != nil {
		issue.Title = *opts.Title
	}
	if opts.Status != nil {
		issue.Status = *opts.Status
	}
	if opts.Priority != nil {
		issue.Priority = *opts.Priority
	}
	if opts.Description != nil {
		issue.Description = *opts.Description
	}
	if opts.Assignee != nil {
		issue.Assignee = *opts.Assignee
	}
	if opts.Unassign {
		issue.Assignee = ""
	}

	// Handle labels
	if len(opts.SetLabels) > 0 {
		issue.Labels = opts.SetLabels
	} else {
		// Add labels
		for _, label := range opts.AddLabels {
			found := false
			for _, l := range issue.Labels {
				if l == label {
					found = true
					break
				}
			}
			if !found {
				issue.Labels = append(issue.Labels, label)
			}
		}
		// Remove labels
		for _, label := range opts.RemoveLabels {
			for i, l := range issue.Labels {
				if l == label {
					issue.Labels = append(issue.Labels[:i], issue.Labels[i+1:]...)
					break
				}
			}
		}
	}

	issue.UpdatedAt = now()

	return nil
}

// Close closes one or more issues.
func (d *Double) Close(ids ...string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.db.isActive {
		return fmt.Errorf("not a beads repository")
	}

	for _, id := range ids {
		db := d.getDB(id)
		if issue, ok := db.issues[id]; ok {
			issue.Status = "closed"
			issue.ClosedAt = now()
			issue.UpdatedAt = now()
		}
	}

	return nil
}

// CloseWithReason closes issues with a reason.
func (d *Double) CloseWithReason(reason string, ids ...string) error {
	return d.CloseWithOptions(CloseOptions{Reason: reason}, ids...)
}

// CloseWithOptions closes issues with full options (reason, session).
func (d *Double) CloseWithOptions(opts CloseOptions, ids ...string) error {
	// In the Double, session is not used (it's for audit trail in production)
	return d.Close(ids...)
}

// Delete permanently removes issues.
func (d *Double) Delete(ids ...string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.db.isActive {
		return fmt.Errorf("not a beads repository")
	}

	for _, id := range ids {
		db := d.getDB(id)
		delete(db.issues, id)
		delete(db.dependencies, id)

		// Remove from other issues' dependencies
		for otherID, deps := range d.db.dependencies {
			for i, dep := range deps {
				if dep.ID == id {
					d.db.dependencies[otherID] = append(deps[:i], deps[i+1:]...)
					break
				}
			}
		}
	}

	return nil
}

// DeleteWithOptions permanently removes issues with options.
func (d *Double) DeleteWithOptions(opts DeleteOptions, ids ...string) error {
	// Force option is not relevant for the double - just delete
	return d.Delete(ids...)
}

// Reopen reopens a closed issue.
func (d *Double) Reopen(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	db := d.getDB(id)
	if !db.isActive {
		return fmt.Errorf("not a beads repository")
	}

	issue, ok := db.issues[id]
	if !ok {
		return ErrNotFound
	}

	issue.Status = "open"
	issue.ClosedAt = ""
	issue.UpdatedAt = now()

	return nil
}

// === Dependency Operations ===

// Ready returns issues that are ready to work (not blocked).
func (d *Double) Ready() ([]*Issue, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if !d.db.isActive {
		return nil, fmt.Errorf("not a beads repository")
	}

	var result []*Issue
	for id, issue := range d.db.issues {
		// Only open issues can be ready
		if issue.Status != "open" {
			continue
		}

		// Check if blocked by any blocking dependencies (type "blocks")
		// Non-blocking "tracks" and "depends-on" types do NOT cause an issue to be blocked
		blocked := false
		if deps, ok := d.db.dependencies[id]; ok {
			for _, dep := range deps {
				// Only "blocks" type dependencies are blocking
				if dep.Type != "blocks" {
					continue
				}
				if depIssue, ok := d.db.issues[dep.ID]; ok {
					if depIssue.Status != "closed" {
						blocked = true
						break
					}
				}
			}
		}

		if !blocked {
			issueCopy := *issue
			result = append(result, &issueCopy)
		}
	}

	// Sort by priority (higher first), then by created_at
	sort.Slice(result, func(i, j int) bool {
		if result[i].Priority != result[j].Priority {
			return result[i].Priority > result[j].Priority
		}
		return result[i].CreatedAt < result[j].CreatedAt
	})

	return result, nil
}

// ReadyWithLabel returns ready issues filtered by label.
func (d *Double) ReadyWithLabel(label string, limit int) ([]*Issue, error) {
	ready, err := d.Ready()
	if err != nil {
		return nil, err
	}

	var result []*Issue
	for _, issue := range ready {
		for _, l := range issue.Labels {
			if l == label {
				result = append(result, issue)
				break
			}
		}
	}

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result, nil
}

// Blocked returns issues that are blocked by dependencies.
func (d *Double) Blocked() ([]*Issue, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if !d.db.isActive {
		return nil, fmt.Errorf("not a beads repository")
	}

	var result []*Issue
	for id, issue := range d.db.issues {
		// Only open issues can be blocked
		if issue.Status != "open" {
			continue
		}

		// Check if blocked by any blocking dependencies (type "blocks")
		// Non-blocking "tracks" and "depends-on" types do NOT cause an issue to be blocked
		if deps, ok := d.db.dependencies[id]; ok {
			for _, dep := range deps {
				// Only "blocks" type dependencies are blocking
				if dep.Type != "blocks" {
					continue
				}
				if depIssue, ok := d.db.issues[dep.ID]; ok {
					if depIssue.Status != "closed" {
						issueCopy := *issue
						result = append(result, &issueCopy)
						break
					}
				}
			}
		}
	}

	return result, nil
}

// AddDependency adds a blocking dependency: issue depends on dependsOn.
// This creates a "blocks" type dependency (the default blocking type).
func (d *Double) AddDependency(issue, dependsOn string) error {
	return d.addDependencyWithType(issue, dependsOn, "blocks")
}

// AddDependencyWithType adds a typed dependency.
// Type "blocks" is blocking (shows in Blocked()) - this is the default type for dep add.
// All other types (tracks, related, parent-child, etc.) are non-blocking.
func (d *Double) AddDependencyWithType(issue, dependsOn, depType string) error {
	return d.addDependencyWithType(issue, dependsOn, depType)
}

// addDependencyWithType is the internal implementation for adding dependencies.
func (d *Double) addDependencyWithType(issue, dependsOn, depType string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Route both issues by prefix
	issueDB := d.getDB(issue)
	dependsOnDB := d.getDB(dependsOn)

	if !issueDB.isActive {
		return fmt.Errorf("not a beads repository")
	}

	if _, ok := issueDB.issues[issue]; !ok {
		return ErrNotFound
	}
	if _, ok := dependsOnDB.issues[dependsOn]; !ok {
		return ErrNotFound
	}

	// Store dependency in the issue's database
	// Check if already exists
	for _, dep := range issueDB.dependencies[issue] {
		if dep.ID == dependsOn {
			return nil // Already exists
		}
	}

	issueDB.dependencies[issue] = append(issueDB.dependencies[issue], doubleDep{
		ID:   dependsOn,
		Type: depType,
	})

	return nil
}

// RemoveDependency removes a dependency.
func (d *Double) RemoveDependency(issue, dependsOn string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	issueDB := d.getDB(issue)
	if !issueDB.isActive {
		return fmt.Errorf("not a beads repository")
	}

	deps, ok := issueDB.dependencies[issue]
	if !ok {
		return nil
	}

	for i, dep := range deps {
		if dep.ID == dependsOn {
			issueDB.dependencies[issue] = append(deps[:i], deps[i+1:]...)
			return nil
		}
	}

	return nil
}

// === Sync Operations ===

// Sync syncs beads with remote.
func (d *Double) Sync() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.db.isActive {
		return fmt.Errorf("not a beads repository")
	}

	// Simulate sync by resetting ahead/behind
	d.db.syncAhead = 0
	d.db.syncBehind = 0
	d.db.conflicts = nil

	return nil
}

// SyncFromMain syncs beads updates from main branch.
func (d *Double) SyncFromMain() error {
	return d.Sync()
}

// SyncImportOnly syncs only importing changes (no export).
func (d *Double) SyncImportOnly() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.db.isActive {
		return fmt.Errorf("not a beads repository")
	}

	// Only reset behind, keep ahead
	d.db.syncBehind = 0
	d.db.conflicts = nil

	return nil
}

// GetSyncStatus returns the sync status without performing a sync.
func (d *Double) GetSyncStatus() (*SyncStatus, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if !d.db.isActive {
		return nil, fmt.Errorf("not a beads repository")
	}

	return &SyncStatus{
		Branch:    d.db.syncBranch,
		Ahead:     d.db.syncAhead,
		Behind:    d.db.syncBehind,
		Conflicts: d.db.conflicts,
	}, nil
}

// === Config Operations ===

// ConfigGet returns a config value.
func (d *Double) ConfigGet(key string) (string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if !d.db.isActive {
		return "", fmt.Errorf("not a beads repository")
	}

	value, ok := d.db.config[key]
	if !ok {
		return "", nil
	}

	return value, nil
}

// ConfigSet sets a config value.
func (d *Double) ConfigSet(key, value string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.db.isActive {
		return fmt.Errorf("not a beads repository")
	}

	d.db.config[key] = value
	return nil
}

// Init initializes a beads repository.
func (d *Double) Init(opts InitOptions) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if opts.Prefix != "" {
		d.db.prefix = opts.Prefix
	}
	d.db.isActive = true

	return nil
}

// Migrate runs database migrations.
func (d *Double) Migrate(opts MigrateOptions) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.db.isActive {
		return fmt.Errorf("not a beads repository")
	}

	// Migration is a no-op for the double
	return nil
}

// === Daemon Operations ===

// DaemonStart starts the bd daemon.
func (d *Double) DaemonStart() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.db.daemonRunning = true
	d.db.daemonPID = 12345

	return nil
}

// DaemonStop stops the bd daemon.
func (d *Double) DaemonStop() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.db.daemonRunning = false
	d.db.daemonPID = 0

	return nil
}

// DaemonStatus returns the daemon status.
func (d *Double) DaemonStatus() (*DaemonStatus, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return &DaemonStatus{
		Running: d.db.daemonRunning,
		PID:     d.db.daemonPID,
		Uptime:  "1h30m",
	}, nil
}

// DaemonHealth returns daemon health info.
func (d *Double) DaemonHealth() (*DaemonHealth, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if !d.db.daemonRunning {
		return nil, fmt.Errorf("daemon not running")
	}

	return &DaemonHealth{
		Status:    "healthy",
		Latency:   5,
		QueueSize: 0,
	}, nil
}

// === Molecule Operations ===

// MolSeed seeds molecules (e.g., patrol).
func (d *Double) MolSeed(opts MolSeedOptions) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.db.isActive {
		return fmt.Errorf("not a beads repository")
	}

	// Seeding is a no-op for the double unless specifically set up
	return nil
}

// MolCurrent returns the current step of a molecule.
func (d *Double) MolCurrent(moleculeID string) (*MolCurrentOutput, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if !d.db.isActive {
		return nil, fmt.Errorf("not a beads repository")
	}

	step, ok := d.db.moleculeSteps[moleculeID]
	if !ok {
		return nil, ErrNotFound
	}

	return step, nil
}

// MolCatalog lists available molecule prototypes.
func (d *Double) MolCatalog() ([]*MoleculeProto, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if !d.db.isActive {
		return nil, fmt.Errorf("not a beads repository")
	}

	result := []*MoleculeProto{} // Return empty slice, not nil
	for _, proto := range d.db.prototypes {
		result = append(result, proto)
	}

	return result, nil
}

// WispCreate creates a wisp from a prototype.
func (d *Double) WispCreate(protoID, actor string) (*Issue, error) {
	return d.WispCreateWithOptions(WispCreateOptions{
		ProtoID: protoID,
		Actor:   actor,
	})
}

// WispCreateWithOptions creates a wisp with full options including variables.
func (d *Double) WispCreateWithOptions(opts WispCreateOptions) (*Issue, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.db.isActive {
		return nil, fmt.Errorf("not a beads repository")
	}

	proto, ok := d.db.prototypes[opts.ProtoID]
	if !ok {
		return nil, ErrNotFound
	}

	// Build title with variable substitution if provided
	title := proto.Name + " wisp"
	if feature, ok := opts.Variables["feature"]; ok {
		title = feature
	}

	id := d.generateID()
	issue := &Issue{
		ID:        id,
		Title:     title,
		Status:    "open",
		CreatedAt: now(),
		UpdatedAt: now(),
		CreatedBy: opts.Actor,
		Labels:    []string{"gt:wisp"},
	}

	d.db.issues[id] = issue

	issueCopy := *issue
	return &issueCopy, nil
}

// WispList lists all wisps.
func (d *Double) WispList(all bool) ([]*Issue, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if !d.db.isActive {
		return nil, fmt.Errorf("not a beads repository")
	}

	result := []*Issue{} // Return empty slice, not nil
	for _, issue := range d.db.issues {
		for _, label := range issue.Labels {
			if label == "gt:wisp" {
				if all || issue.Status != "closed" {
					issueCopy := *issue
					result = append(result, &issueCopy)
				}
				break
			}
		}
	}

	return result, nil
}

// WispGC garbage collects old wisps.
func (d *Double) WispGC() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.db.isActive {
		return fmt.Errorf("not a beads repository")
	}

	// GC closed wisps older than some threshold
	// For the double, just remove closed wisps
	for id, issue := range d.db.issues {
		if issue.Status == "closed" {
			for _, label := range issue.Labels {
				if label == "gt:wisp" {
					delete(d.db.issues, id)
					break
				}
			}
		}
	}

	return nil
}

// MolBurn burns (deletes) wisps/molecules.
func (d *Double) MolBurn(ids ...string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.db.isActive {
		return fmt.Errorf("not a beads repository")
	}

	for _, id := range ids {
		db := d.getDB(id)
		delete(db.issues, id)
	}

	return nil
}

// MolBond bonds a wisp to a bead, creating a compound.
func (d *Double) MolBond(wispID, beadID string) (*Issue, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	wispDB := d.getDB(wispID)
	beadDB := d.getDB(beadID)

	if !wispDB.isActive {
		return nil, fmt.Errorf("not a beads repository")
	}

	wisp, ok := wispDB.issues[wispID]
	if !ok {
		return nil, fmt.Errorf("wisp not found: %s", wispID)
	}

	bead, ok := beadDB.issues[beadID]
	if !ok {
		return nil, fmt.Errorf("bead not found: %s", beadID)
	}

	// Create a compound by adding dependency and label
	wisp.Labels = append(wisp.Labels, "gt:bonded")
	wispDB.dependencies[wispID] = append(wispDB.dependencies[wispID], doubleDep{
		ID:   beadID,
		Type: "depends-on", // Bond is a blocking dependency
	})

	// Return the updated wisp as the compound
	issueCopy := *wisp
	issueCopy.Parent = bead.ID
	return &issueCopy, nil
}

// === Gate Operations ===

// GateShow returns gate status.
func (d *Double) GateShow(gateID string) (*Gate, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if !d.db.isActive {
		return nil, fmt.Errorf("not a beads repository")
	}

	gate, ok := d.db.gates[gateID]
	if !ok {
		return nil, ErrNotFound
	}

	return gate, nil
}

// GateWait waits for a gate to open.
func (d *Double) GateWait(gateID, notifyAgent string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.db.isActive {
		return fmt.Errorf("not a beads repository")
	}

	gate, ok := d.db.gates[gateID]
	if !ok {
		return ErrNotFound
	}

	// If gate is already open, return immediately
	if gate.Status == "open" {
		return nil
	}

	// Add to waiters
	if notifyAgent != "" {
		gate.Waiters = append(gate.Waiters, notifyAgent)
	}

	return nil
}

// GateList returns all gates.
func (d *Double) GateList(all bool) ([]*Gate, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if !d.db.isActive {
		return nil, fmt.Errorf("not a beads repository")
	}

	gates := []*Gate{} // Return empty slice, not nil
	for _, gate := range d.db.gates {
		if all || gate.Status != "closed" {
			gates = append(gates, gate)
		}
	}

	return gates, nil
}

// GateResolve manually closes/resolves a gate.
func (d *Double) GateResolve(gateID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.db.isActive {
		return fmt.Errorf("not a beads repository")
	}

	gate, ok := d.db.gates[gateID]
	if !ok {
		return ErrNotFound
	}

	gate.Status = "closed"
	return nil
}

// GateAddWaiter adds a waiter to a gate.
func (d *Double) GateAddWaiter(gateID, waiterID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.db.isActive {
		return fmt.Errorf("not a beads repository")
	}

	gate, ok := d.db.gates[gateID]
	if !ok {
		return ErrNotFound
	}

	gate.Waiters = append(gate.Waiters, waiterID)
	return nil
}

// GateCheck evaluates all open gates and closes resolved ones.
func (d *Double) GateCheck() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.db.isActive {
		return fmt.Errorf("not a beads repository")
	}

	// In Double, this is a no-op since we don't have actual gate resolution logic
	return nil
}

// === Swarm Operations ===

// SwarmStatus returns swarm status.
func (d *Double) SwarmStatus(swarmID string) (*SwarmStatus, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if !d.db.isActive {
		return nil, fmt.Errorf("not a beads repository")
	}

	status, ok := d.db.swarms[swarmID]
	if !ok {
		return nil, ErrNotFound
	}

	return status, nil
}

// SwarmCreate creates a swarm molecule from an epic.
func (d *Double) SwarmCreate(epicID string) (*Issue, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.db.isActive {
		return nil, fmt.Errorf("not a beads repository")
	}

	// Verify epic exists
	epic, ok := d.db.issues[epicID]
	if !ok {
		return nil, ErrNotFound
	}
	if epic.Type != "epic" {
		return nil, fmt.Errorf("issue %s is not an epic", epicID)
	}

	// Create a swarm molecule issue
	swarmID := d.generateID()
	swarmIssue := &Issue{
		ID:     swarmID,
		Title:  "Swarm: " + epic.Title,
		Type:   "swarm",
		Status: "open",
		Parent: epicID,
	}
	d.db.issues[swarmID] = swarmIssue

	// Initialize swarm status
	d.db.swarms[swarmID] = &SwarmStatus{
		ID:     swarmID,
		Status: "active",
	}

	return swarmIssue, nil
}

// SwarmList returns all swarm molecules.
func (d *Double) SwarmList() ([]*Issue, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if !d.db.isActive {
		return nil, fmt.Errorf("not a beads repository")
	}

	swarms := []*Issue{} // Return empty slice, not nil
	for _, issue := range d.db.issues {
		if issue.Type == "swarm" {
			swarms = append(swarms, issue)
		}
	}

	return swarms, nil
}

// SwarmValidate validates epic structure for swarming.
func (d *Double) SwarmValidate(epicID string) error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if !d.db.isActive {
		return fmt.Errorf("not a beads repository")
	}

	epic, ok := d.db.issues[epicID]
	if !ok {
		return ErrNotFound
	}
	if epic.Type != "epic" {
		return fmt.Errorf("issue %s is not an epic", epicID)
	}

	// In Double, always valid
	return nil
}

// === Formula Operations ===

// FormulaShow returns detailed formula definition.
func (d *Double) FormulaShow(name string) (*FormulaDetails, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if !d.db.isActive {
		return nil, fmt.Errorf("not a beads repository")
	}

	formula, ok := d.db.formulas[name]
	if !ok {
		// Try with mol- prefix
		formula, ok = d.db.formulas["mol-"+name]
		if !ok {
			return nil, ErrNotFound
		}
	}

	// Convert FormulaListEntry to FormulaDetails for test double
	return &FormulaDetails{
		Name:        formula.Name,
		Description: formula.Description,
		Type:        formula.Type,
		Steps:       []*FormulaStep{}, // Test double doesn't track detailed steps
	}, nil
}

// FormulaList returns all available formulas.
func (d *Double) FormulaList() ([]*FormulaListEntry, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if !d.db.isActive {
		return nil, fmt.Errorf("not a beads repository")
	}

	result := []*FormulaListEntry{}
	for _, formula := range d.db.formulas {
		result = append(result, formula)
	}

	return result, nil
}

// Cook executes a formula.
func (d *Double) Cook(formulaName string) (*Issue, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.db.isActive {
		return nil, fmt.Errorf("not a beads repository")
	}

	formula, ok := d.db.formulas[formulaName]
	if !ok {
		formula, ok = d.db.formulas["mol-"+formulaName]
		if !ok {
			return nil, ErrNotFound
		}
	}

	id := d.generateID()
	issue := &Issue{
		ID:        id,
		Title:     formula.Name + " instance",
		Status:    "open",
		CreatedAt: now(),
		UpdatedAt: now(),
		Labels:    []string{"gt:formula", "formula:" + formula.Name},
	}

	d.db.issues[id] = issue

	issueCopy := *issue
	return &issueCopy, nil
}

// LegAdd adds a leg (step) to a formula instance.
func (d *Double) LegAdd(formulaID, stepName string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	db := d.getDB(formulaID)
	if !db.isActive {
		return fmt.Errorf("not a beads repository")
	}

	if _, ok := db.issues[formulaID]; !ok {
		return ErrNotFound
	}

	// Create a step issue as a child
	id := d.generateID()
	step := &Issue{
		ID:        id,
		Title:     stepName,
		Status:    "open",
		Parent:    formulaID,
		CreatedAt: now(),
		UpdatedAt: now(),
		Labels:    []string{"gt:step"},
	}

	db.issues[id] = step

	// Add to parent's children
	if parent, ok := db.issues[formulaID]; ok {
		parent.Children = append(parent.Children, id)
	}

	return nil
}

// === Agent Operations ===

// AgentState sets the state of an agent bead.
func (d *Double) AgentState(beadID, state string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	db := d.getDB(beadID)
	if !db.isActive {
		return fmt.Errorf("not a beads repository")
	}

	issue, ok := db.issues[beadID]
	if !ok {
		return ErrNotFound
	}

	issue.AgentState = state
	issue.UpdatedAt = now()

	return nil
}

// === Label Operations ===

// LabelAdd adds a label to an issue.
func (d *Double) LabelAdd(id, label string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	db := d.getDB(id)
	if !db.isActive {
		return fmt.Errorf("not a beads repository")
	}

	issue, ok := db.issues[id]
	if !ok {
		return ErrNotFound
	}

	// Check if already exists
	for _, l := range issue.Labels {
		if l == label {
			return nil
		}
	}

	issue.Labels = append(issue.Labels, label)
	issue.UpdatedAt = now()

	return nil
}

// LabelRemove removes a label from an issue.
func (d *Double) LabelRemove(id, label string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	db := d.getDB(id)
	if !db.isActive {
		return fmt.Errorf("not a beads repository")
	}

	issue, ok := db.issues[id]
	if !ok {
		return ErrNotFound
	}

	for i, l := range issue.Labels {
		if l == label {
			issue.Labels = append(issue.Labels[:i], issue.Labels[i+1:]...)
			issue.UpdatedAt = now()
			return nil
		}
	}

	return nil
}

// === Miscellaneous Operations ===

// Version returns the bd version.
func (d *Double) Version() (string, error) {
	return "1.0.0-double", nil
}

// Doctor runs diagnostics.
func (d *Double) Doctor() (*DoctorReport, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if !d.db.isActive {
		return nil, fmt.Errorf("not a beads repository")
	}

	return &DoctorReport{
		Status: "healthy",
		Checks: []*DoctorCheck{
			{Name: "database", Status: "ok"},
			{Name: "sync", Status: "ok"},
		},
	}, nil
}

// Prime returns the PRIME.md content.
func (d *Double) Prime() (string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if !d.db.isActive {
		return "", fmt.Errorf("not a beads repository")
	}

	return primeContent, nil
}

// SlotShow returns slot information.
func (d *Double) SlotShow(id string) (*Slot, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	db := d.getDB(id)
	if !db.isActive {
		return nil, fmt.Errorf("not a beads repository")
	}

	issue, ok := db.issues[id]
	if !ok {
		return nil, ErrNotFound
	}

	return &Slot{
		ID:       id,
		IssueID:  issue.HookBead,
		Agent:    issue.Assignee,
		HookedAt: issue.UpdatedAt,
	}, nil
}

// SlotSet sets a slot on an agent bead.
func (d *Double) SlotSet(agentID, slotName, beadID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	db := d.getDB(agentID)
	if !db.isActive {
		return fmt.Errorf("not a beads repository")
	}

	agent, ok := db.issues[agentID]
	if !ok {
		return ErrNotFound
	}

	// Verify the bead exists (may be in different db)
	beadDB := d.getDB(beadID)
	if _, ok := beadDB.issues[beadID]; !ok {
		return ErrNotFound
	}

	switch slotName {
	case "hook":
		agent.HookBead = beadID
	case "role":
		agent.RoleBead = beadID
	default:
		return fmt.Errorf("unknown slot: %s", slotName)
	}

	return nil
}

// SlotClear clears a slot on an agent bead.
func (d *Double) SlotClear(agentID, slotName string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	db := d.getDB(agentID)
	if !db.isActive {
		return fmt.Errorf("not a beads repository")
	}

	agent, ok := db.issues[agentID]
	if !ok {
		return ErrNotFound
	}

	switch slotName {
	case "hook":
		agent.HookBead = ""
	case "role":
		agent.RoleBead = ""
	default:
		return fmt.Errorf("unknown slot: %s", slotName)
	}

	return nil
}

// Search searches issues by text query.
func (d *Double) Search(query string, opts SearchOptions) ([]*Issue, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if !d.db.isActive {
		return nil, fmt.Errorf("not a beads repository")
	}

	var results []*Issue
	query = strings.ToLower(query)

	for _, issue := range d.db.issues {
		// Filter by status
		if opts.Status != "" && opts.Status != "all" {
			if opts.Status == "open" && issue.Status == "closed" {
				continue
			}
			if opts.Status == "closed" && issue.Status != "closed" {
				continue
			}
		}

		// Filter by type
		if opts.Type != "" && issue.Type != opts.Type {
			continue
		}

		// Search in title and description
		if strings.Contains(strings.ToLower(issue.Title), query) ||
			strings.Contains(strings.ToLower(issue.Description), query) {
			results = append(results, issue)
		}

		// Apply limit
		if opts.Limit > 0 && len(results) >= opts.Limit {
			break
		}
	}

	return results, nil
}

// MessageThread returns all messages in a thread.
func (d *Double) MessageThread(threadID string) ([]*Issue, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if !d.db.isActive {
		return nil, fmt.Errorf("not a beads repository")
	}

	var results []*Issue
	for _, issue := range d.db.issues {
		// Check if issue has thread:<threadID> label
		for _, label := range issue.Labels {
			if label == "thread:"+threadID {
				issueCopy := *issue
				results = append(results, &issueCopy)
				break
			}
		}
	}

	// Sort by created_at ascending (oldest first for thread view)
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt < results[j].CreatedAt
	})

	return results, nil
}

// Flush flushes pending writes to the database.
func (d *Double) Flush() error {
	// Double has no pending writes to flush
	return nil
}

// Stats returns repository statistics as text.
func (d *Double) Stats() (string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if !d.db.isActive {
		return "", fmt.Errorf("not a beads repository")
	}

	open := 0
	closed := 0
	for _, issue := range d.db.issues {
		if issue.Status == "closed" {
			closed++
		} else {
			open++
		}
	}

	return fmt.Sprintf("Total: %d, Open: %d, Closed: %d", len(d.db.issues), open, closed), nil
}

// StatsJSON returns repository statistics as structured data.
func (d *Double) StatsJSON() (*RepoStats, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if !d.db.isActive {
		return nil, fmt.Errorf("not a beads repository")
	}

	stats := &RepoStats{
		TotalIssues:    len(d.db.issues),
		IssuesByType:   make(map[string]int),
		IssuesByStatus: make(map[string]int),
	}

	for _, issue := range d.db.issues {
		if issue.Status == "closed" {
			stats.ClosedIssues++
		} else {
			stats.OpenIssues++
		}

		stats.IssuesByStatus[issue.Status]++
		if issue.Type != "" {
			stats.IssuesByType[issue.Type]++
		}
	}

	// Also populate Summary field to match bd stats --json output format
	stats.Summary = RepoStatsSummary{
		TotalIssues:  stats.TotalIssues,
		OpenIssues:   stats.OpenIssues,
		ClosedIssues: stats.ClosedIssues,
	}

	return stats, nil
}

// Burn records cost information.
func (d *Double) Burn(opts BurnOptions) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.db.isActive {
		return fmt.Errorf("not a beads repository")
	}

	// Burn is a no-op for the double
	return nil
}

// Comment adds a comment to an issue.
func (d *Double) Comment(id, message string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	db := d.getDB(id)
	if !db.isActive {
		return fmt.Errorf("not a beads repository")
	}

	issue, ok := db.issues[id]
	if !ok {
		return ErrNotFound
	}

	// For the double, append comment to description
	if issue.Description != "" {
		issue.Description += "\n\n---\n" + message
	} else {
		issue.Description = message
	}
	issue.UpdatedAt = now()

	return nil
}

// === Repository Checks ===

// IsBeadsRepo checks if the working directory is a beads repository.
func (d *Double) IsBeadsRepo() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.db.isActive
}

// Compile-time check that Double implements BeadsOps.
var _ BeadsOps = (*Double)(nil)

// === Convenience Methods (matching Implementation) ===

// ListByAssignee returns all issues assigned to a specific assignee.
func (d *Double) ListByAssignee(assignee string) ([]*Issue, error) {
	return d.List(ListOptions{
		Status:   "all",
		Assignee: assignee,
		Priority: -1,
	})
}

// GetAssignedIssue returns the first open issue assigned to the given assignee.
func (d *Double) GetAssignedIssue(assignee string) (*Issue, error) {
	issues, err := d.List(ListOptions{
		Status:   "open",
		Assignee: assignee,
		Priority: -1,
	})
	if err != nil {
		return nil, err
	}

	if len(issues) == 0 {
		issues, err = d.List(ListOptions{
			Status:   "in_progress",
			Assignee: assignee,
			Priority: -1,
		})
		if err != nil {
			return nil, err
		}
	}

	if len(issues) == 0 {
		return nil, nil
	}

	return issues[0], nil
}

// ReadyWithType returns ready issues filtered by type label.
func (d *Double) ReadyWithType(issueType string) ([]*Issue, error) {
	return d.ReadyWithLabel("gt:"+issueType, 100)
}

// Release moves an in_progress issue back to open status.
func (d *Double) Release(id string) error {
	return d.ReleaseWithReason(id, "")
}

// ReleaseWithReason moves an in_progress issue back to open status with a reason.
func (d *Double) ReleaseWithReason(id, reason string) error {
	status := "open"
	assignee := ""
	opts := UpdateOptions{
		Status:   &status,
		Assignee: &assignee,
	}

	if reason != "" {
		opts.Notes = "Released: " + reason
	}

	return d.Update(id, opts)
}

// GetIssueByID returns an issue by ID (direct access for testing).
func (d *Double) GetIssueByID(id string) *Issue {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if issue, ok := d.db.issues[id]; ok {
		issueCopy := *issue
		return &issueCopy
	}
	return nil
}

// GetAllIssues returns all issues (for testing).
func (d *Double) GetAllIssues() []*Issue {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]*Issue, 0, len(d.db.issues))
	for _, issue := range d.db.issues {
		issueCopy := *issue
		result = append(result, &issueCopy)
	}
	return result
}

// SetMoleculeStep sets a molecule step for testing.
func (d *Double) SetMoleculeStep(moleculeID string, step *MolCurrentOutput) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.db.moleculeSteps[moleculeID] = step
}

// primeContent is duplicated here for Double.Prime() to return
// (it's also in beads.go but Double doesn't depend on that)
func init() {
	// Ensure primeContent is accessible
	_ = strings.TrimSpace(primeContent)
}
