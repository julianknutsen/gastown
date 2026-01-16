package beads

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/steveyegge/gastown/internal/runtime"
)

// Implementation wraps bd CLI operations for a working directory.
// It implements the BeadsOps interface.
type Implementation struct {
	workDir  string
	beadsDir string // Optional BEADS_DIR override for cross-database access
	townRoot string // Town root for prefix-based routing (auto-detected)
}

// New creates a new Implementation wrapper for the given directory.
// It automatically detects the town root by walking up from workDir.
// Returns an error if no town root is found (indicated by .beads/routes.jsonl).
func New(workDir string) *Implementation {
	townRoot, _ := FindTownRoot(workDir)
	// Note: We don't error here - townRoot may be empty for standalone use.
	// Routing will fail gracefully if townRoot is empty.
	return &Implementation{workDir: workDir, townRoot: townRoot}
}

// NewWithTownRoot creates an Implementation with an explicit town root.
// Use this when you already know the town root to avoid directory traversal.
func NewWithTownRoot(workDir, townRoot string) *Implementation {
	return &Implementation{workDir: workDir, townRoot: townRoot}
}

// NewWithBeadsDir creates an Implementation wrapper with an explicit BEADS_DIR.
// This is needed when running from a polecat worktree but accessing town-level beads.
func NewWithBeadsDir(workDir, beadsDir string) *Implementation {
	townRoot, _ := FindTownRoot(workDir)
	return &Implementation{workDir: workDir, beadsDir: beadsDir, townRoot: townRoot}
}

// routedImpl returns an Implementation routed to the correct rig for the given bead ID.
// If townRoot is not set, returns the current implementation unchanged.
// This enables transparent cross-rig operations by routing based on bead ID prefix.
func (b *Implementation) routedImpl(beadID string) *Implementation {
	if b.townRoot == "" {
		return b
	}
	workDir := ResolveHookDir(b.townRoot, beadID, b.workDir)
	if workDir == b.workDir {
		return b
	}
	return &Implementation{workDir: workDir, townRoot: b.townRoot}
}

// run executes a bd command and returns stdout.
func (b *Implementation) run(args ...string) ([]byte, error) {
	// Use --no-daemon for faster read operations (avoids daemon IPC overhead)
	// The daemon is primarily useful for write coalescing, not reads.
	// Use --allow-stale to prevent failures when db is out of sync with JSONL
	// (e.g., after daemon is killed during shutdown before syncing).
	fullArgs := append([]string{"--no-daemon", "--allow-stale"}, args...)
	cmd := exec.Command("bd", fullArgs...) //nolint:gosec // G204: bd is a trusted internal tool
	cmd.Dir = b.workDir

	// Always explicitly set BEADS_DIR to prevent inherited env vars from
	// causing prefix mismatches. Use explicit beadsDir if set, otherwise
	// resolve from working directory.
	beadsDir := b.beadsDir
	if beadsDir == "" {
		beadsDir = ResolveBeadsDir(b.workDir)
	}
	cmd.Env = append(os.Environ(), "BEADS_DIR="+beadsDir)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return nil, b.wrapError(err, stderr.String(), args)
	}

	// Handle bd --no-daemon exit code 0 bug: when issue not found,
	// --no-daemon exits 0 but writes error to stderr with empty stdout.
	// Detect this case and treat as error to avoid JSON parse failures.
	if stdout.Len() == 0 && stderr.Len() > 0 {
		return nil, b.wrapError(fmt.Errorf("command produced no output"), stderr.String(), args)
	}

	return stdout.Bytes(), nil
}

// runWithDaemon executes a bd command using the daemon (for writes).
func (b *Implementation) runWithDaemon(args ...string) ([]byte, error) {
	cmd := exec.Command("bd", args...) //nolint:gosec // G204: bd is a trusted internal tool
	cmd.Dir = b.workDir

	beadsDir := b.beadsDir
	if beadsDir == "" {
		beadsDir = ResolveBeadsDir(b.workDir)
	}
	cmd.Env = append(os.Environ(), "BEADS_DIR="+beadsDir)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return nil, b.wrapError(err, stderr.String(), args)
	}

	return stdout.Bytes(), nil
}

// Run executes a bd command and returns stdout.
// This is a public wrapper around the internal run method for cases where
// callers need to run arbitrary bd commands.
func (b *Implementation) Run(args ...string) ([]byte, error) {
	return b.run(args...)
}

// wrapError wraps bd errors with context.
// ZFC: Avoid parsing stderr to make decisions. Transport errors to agents instead.
// Exception: ErrNotInstalled (exec.ErrNotFound) and ErrNotFound (issue lookup) are
// acceptable as they enable basic error handling without decision-making.
func (b *Implementation) wrapError(err error, stderr string, args []string) error {
	stderr = strings.TrimSpace(stderr)

	// Check for bd not installed
	var execErr *exec.Error
	if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
		return ErrNotInstalled
	}

	// ErrNotFound is widely used for issue lookups - acceptable exception
	// Match various "not found" error patterns from bd
	if strings.Contains(stderr, "not found") || strings.Contains(stderr, "Issue not found") ||
		strings.Contains(stderr, "no issue found") {
		return ErrNotFound
	}

	if stderr != "" {
		return fmt.Errorf("bd %s: %s", strings.Join(args, " "), stderr)
	}
	return fmt.Errorf("bd %s: %w", strings.Join(args, " "), err)
}

// === Issue Operations ===

// List returns issues matching the given options.
func (b *Implementation) List(opts ListOptions) ([]*Issue, error) {
	args := []string{"list", "--json"}

	if opts.Status != "" {
		args = append(args, "--status="+opts.Status)
	}
	// Label filters by label value (e.g., "queue:inbox")
	if opts.Label != "" {
		args = append(args, "--label="+opts.Label)
	}
	// Type filters by issue_type field (e.g., "message", "task")
	if opts.Type != "" {
		args = append(args, "--type="+opts.Type)
	}
	// Multiple labels (all must match)
	for _, label := range opts.Labels {
		args = append(args, "-l", label)
	}
	if opts.Priority >= 0 {
		args = append(args, fmt.Sprintf("--priority=%d", opts.Priority))
	}
	if opts.Parent != "" {
		args = append(args, "--parent="+opts.Parent)
	}
	if opts.Assignee != "" {
		args = append(args, "--assignee="+opts.Assignee)
	}
	if opts.NoAssignee {
		args = append(args, "--no-assignee")
	}
	if opts.Limit > 0 {
		args = append(args, fmt.Sprintf("--limit=%d", opts.Limit))
	}
	if opts.Tag != "" {
		args = append(args, "--tag="+opts.Tag)
	}
	if opts.CreatedAfter != "" {
		args = append(args, "--created-after="+opts.CreatedAfter)
	}
	if opts.All {
		args = append(args, "--all")
	}
	if opts.MolType != "" {
		args = append(args, "--mol-type="+opts.MolType)
	}

	out, err := b.run(args...)
	if err != nil {
		return nil, err
	}

	var issues []*Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("parsing bd list output: %w", err)
	}

	return issues, nil
}

// Show returns detailed information about an issue.
// bd routes this command natively via mayor/town.json -> routes.jsonl
func (b *Implementation) Show(id string) (*Issue, error) {
	out, err := b.run("show", id, "--json")
	if err != nil {
		return nil, err
	}

	// bd show --json returns an array with one element
	var issues []*Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("parsing bd show output: %w", err)
	}

	if len(issues) == 0 {
		return nil, ErrNotFound
	}

	return issues[0], nil
}

// ShowMultiple fetches multiple issues by ID in a single bd call.
// Returns a map of ID to Issue. Missing IDs are not included in the map.
func (b *Implementation) ShowMultiple(ids []string) (map[string]*Issue, error) {
	if len(ids) == 0 {
		return make(map[string]*Issue), nil
	}

	// bd show supports multiple IDs
	args := append([]string{"show", "--json"}, ids...)
	out, err := b.run(args...)
	if err != nil {
		// If bd fails, return empty map (some IDs might not exist)
		return make(map[string]*Issue), nil
	}

	var issues []*Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("parsing bd show output: %w", err)
	}

	result := make(map[string]*Issue, len(issues))
	for _, issue := range issues {
		result[issue.ID] = issue
	}

	return result, nil
}

// Create creates a new issue and returns it.
// If opts.Actor is empty, it defaults to the BD_ACTOR environment variable.
// This ensures created_by is populated for issue provenance tracking.
func (b *Implementation) Create(opts CreateOptions) (*Issue, error) {
	args := []string{"create", "--json"}

	if opts.Title != "" {
		args = append(args, "--title="+opts.Title)
	}
	// Type is deprecated: convert to gt:<type> label
	if opts.Type != "" {
		args = append(args, "--labels=gt:"+opts.Type)
	}
	if opts.Priority >= 0 {
		args = append(args, fmt.Sprintf("--priority=%d", opts.Priority))
	}
	if opts.Description != "" {
		args = append(args, "--description="+opts.Description)
	}
	if opts.Parent != "" {
		args = append(args, "--parent="+opts.Parent)
	}
	if opts.Ephemeral {
		args = append(args, "--ephemeral")
	}
	for _, label := range opts.Labels {
		args = append(args, "--labels="+label)
	}
	// Default Actor from BD_ACTOR env var if not specified
	actor := opts.Actor
	if actor == "" {
		actor = os.Getenv("BD_ACTOR")
	}
	if actor != "" {
		args = append(args, "--actor="+actor)
	}
	if opts.MolType != "" {
		args = append(args, "--mol-type="+opts.MolType)
	}
	// Event-specific fields
	if opts.EventCategory != "" {
		args = append(args, "--event-category="+opts.EventCategory)
	}
	if opts.EventActor != "" {
		args = append(args, "--event-actor="+opts.EventActor)
	}
	if opts.EventPayload != "" {
		args = append(args, "--event-payload="+opts.EventPayload)
	}
	if opts.EventTarget != "" {
		args = append(args, "--event-target="+opts.EventTarget)
	}

	out, err := b.run(args...)
	if err != nil {
		return nil, err
	}

	var issue Issue
	if err := json.Unmarshal(out, &issue); err != nil {
		return nil, fmt.Errorf("parsing bd create output: %w", err)
	}

	return &issue, nil
}

// CreateWithID creates an issue with a specific ID.
// This is useful for agent beads, role beads, and other beads that need
// deterministic IDs rather than auto-generated ones.
func (b *Implementation) CreateWithID(id string, opts CreateOptions) (*Issue, error) {
	args := []string{"create", "--json", "--id=" + id}

	if opts.Title != "" {
		args = append(args, "--title="+opts.Title)
	}
	// Type is deprecated: convert to gt:<type> label
	if opts.Type != "" {
		args = append(args, "--labels=gt:"+opts.Type)
	}
	if opts.Priority >= 0 {
		args = append(args, fmt.Sprintf("--priority=%d", opts.Priority))
	}
	if opts.Description != "" {
		args = append(args, "--description="+opts.Description)
	}
	if opts.Parent != "" {
		args = append(args, "--parent="+opts.Parent)
	}
	for _, label := range opts.Labels {
		args = append(args, "--labels="+label)
	}
	// Default Actor from BD_ACTOR env var if not specified
	actor := opts.Actor
	if actor == "" {
		actor = os.Getenv("BD_ACTOR")
	}
	if actor != "" {
		args = append(args, "--actor="+actor)
	}
	if opts.MolType != "" {
		args = append(args, "--mol-type="+opts.MolType)
	}
	// Event-specific fields
	if opts.EventCategory != "" {
		args = append(args, "--event-category="+opts.EventCategory)
	}
	if opts.EventActor != "" {
		args = append(args, "--event-actor="+opts.EventActor)
	}
	if opts.EventPayload != "" {
		args = append(args, "--event-payload="+opts.EventPayload)
	}
	if opts.EventTarget != "" {
		args = append(args, "--event-target="+opts.EventTarget)
	}

	out, err := b.run(args...)
	if err != nil {
		return nil, err
	}

	var issue Issue
	if err := json.Unmarshal(out, &issue); err != nil {
		return nil, fmt.Errorf("parsing bd create output: %w", err)
	}

	return &issue, nil
}

// Update updates an existing issue.
// bd routes this command natively via mayor/town.json -> routes.jsonl
func (b *Implementation) Update(id string, opts UpdateOptions) error {
	args := []string{"update", id}

	if opts.Title != nil {
		args = append(args, "--title="+*opts.Title)
	}
	if opts.Status != nil {
		args = append(args, "--status="+*opts.Status)
	}
	if opts.Priority != nil {
		args = append(args, fmt.Sprintf("--priority=%d", *opts.Priority))
	}
	if opts.Description != nil {
		args = append(args, "--description="+*opts.Description)
	}
	if opts.Assignee != nil {
		args = append(args, "--assignee="+*opts.Assignee)
	}
	if opts.Unassign {
		args = append(args, "--unassign")
	}
	if opts.Notes != "" {
		args = append(args, "--notes="+opts.Notes)
	}
	// Label operations: set-labels replaces all, otherwise use add/remove
	if len(opts.SetLabels) > 0 {
		for _, label := range opts.SetLabels {
			args = append(args, "--set-labels="+label)
		}
	} else {
		for _, label := range opts.AddLabels {
			args = append(args, "--add-label="+label)
		}
		for _, label := range opts.RemoveLabels {
			args = append(args, "--remove-label="+label)
		}
	}

	_, err := b.run(args...)
	return err
}

// Close closes one or more issues.
// bd routes this command natively via mayor/town.json -> routes.jsonl
// If a runtime session ID is set in the environment, it is passed to bd close
// for work attribution tracking (see decision 009-session-events-architecture.md).
func (b *Implementation) Close(ids ...string) error {
	if len(ids) == 0 {
		return nil
	}

	args := append([]string{"close"}, ids...)

	// Pass session ID for work attribution if available
	if sessionID := runtime.SessionIDFromEnv(); sessionID != "" {
		args = append(args, "--session="+sessionID)
	}

	_, err := b.run(args...)
	return err
}

// CloseWithReason closes one or more issues with a reason.
// bd routes this command natively via mayor/town.json -> routes.jsonl
// If a runtime session ID is set in the environment, it is passed to bd close
// for work attribution tracking (see decision 009-session-events-architecture.md).
func (b *Implementation) CloseWithReason(reason string, ids ...string) error {
	return b.CloseWithOptions(CloseOptions{Reason: reason}, ids...)
}

// CloseWithOptions closes one or more issues with full options.
// If Session is empty, falls back to runtime.SessionIDFromEnv().
func (b *Implementation) CloseWithOptions(opts CloseOptions, ids ...string) error {
	if len(ids) == 0 {
		return nil
	}

	args := append([]string{"close"}, ids...)
	if opts.Reason != "" {
		args = append(args, "--reason="+opts.Reason)
	}
	if opts.Force {
		args = append(args, "--force")
	}

	// Use explicit session if provided, otherwise fall back to environment
	sessionID := opts.Session
	if sessionID == "" {
		sessionID = runtime.SessionIDFromEnv()
	}
	if sessionID != "" {
		args = append(args, "--session="+sessionID)
	}

	_, err := b.run(args...)
	return err
}

// Delete permanently removes issues.
// bd doesn't route this by prefix, so we use routedImpl.
func (b *Implementation) Delete(ids ...string) error {
	if len(ids) == 0 {
		return nil
	}
	impl := b.routedImpl(ids[0])
	// Use --force to bypass confirmation and --hard to permanently delete
	// (bypass tombstones so Show returns ErrNotFound after delete)
	args := []string{"delete", "--force", "--hard"}
	args = append(args, ids...)
	_, err := impl.run(args...)
	return err
}

// DeleteWithOptions permanently removes issues with options.
// bd doesn't route this by prefix, so we use routedImpl.
func (b *Implementation) DeleteWithOptions(opts DeleteOptions, ids ...string) error {
	if len(ids) == 0 {
		return nil
	}
	impl := b.routedImpl(ids[0])
	args := []string{"delete", "--hard"}
	if opts.Force {
		args = append(args, "--force")
	}
	args = append(args, ids...)
	_, err := impl.run(args...)
	return err
}

// Reopen reopens a closed issue.
// bd doesn't route this by prefix, so we use routedImpl.
func (b *Implementation) Reopen(id string) error {
	impl := b.routedImpl(id)
	_, err := impl.run("reopen", id)
	return err
}

// === Dependency Operations ===

// Ready returns issues that are ready to work (not blocked).
func (b *Implementation) Ready() ([]*Issue, error) {
	out, err := b.run("ready", "--json")
	if err != nil {
		return nil, err
	}

	var issues []*Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("parsing bd ready output: %w", err)
	}

	return issues, nil
}

// ReadyWithLabel returns ready issues filtered by label.
// Uses bd ready --label flag for server-side filtering.
func (b *Implementation) ReadyWithLabel(label string, limit int) ([]*Issue, error) {
	args := []string{"ready", "--json", "--label", label}
	if limit > 0 {
		args = append(args, "-n", fmt.Sprintf("%d", limit))
	}

	out, err := b.run(args...)
	if err != nil {
		return nil, err
	}

	var issues []*Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("parsing bd ready output: %w", err)
	}

	return issues, nil
}

// Blocked returns issues that are blocked by dependencies.
func (b *Implementation) Blocked() ([]*Issue, error) {
	out, err := b.run("blocked", "--json")
	if err != nil {
		return nil, err
	}

	var issues []*Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("parsing bd blocked output: %w", err)
	}

	return issues, nil
}

// AddDependency adds a dependency: issue depends on dependsOn.
// bd now routes this command by prefix natively, but we keep routedImpl for reliability.
func (b *Implementation) AddDependency(issue, dependsOn string) error {
	impl := b.routedImpl(issue)
	_, err := impl.run("dep", "add", issue, dependsOn)
	return err
}

// AddDependencyWithType adds a typed dependency (e.g., "tracks").
func (b *Implementation) AddDependencyWithType(issue, dependsOn, depType string) error {
	impl := b.routedImpl(issue)
	_, err := impl.run("dep", "add", issue, dependsOn, "--type="+depType)
	return err
}

// RemoveDependency removes a dependency.
// bd now routes this command by prefix natively, but we keep routedImpl for reliability.
func (b *Implementation) RemoveDependency(issue, dependsOn string) error {
	impl := b.routedImpl(issue)
	_, err := impl.run("dep", "remove", issue, dependsOn)
	return err
}

// === Sync Operations ===

// Sync syncs beads with remote.
func (b *Implementation) Sync() error {
	_, err := b.runWithDaemon("sync")
	return err
}

// SyncFromMain syncs beads updates from main branch.
func (b *Implementation) SyncFromMain() error {
	_, err := b.run("sync", "--from-main")
	return err
}

// SyncImportOnly syncs only importing changes (no export).
func (b *Implementation) SyncImportOnly() error {
	_, err := b.run("sync", "--import-only")
	return err
}

// GetSyncStatus returns the sync status without performing a sync.
func (b *Implementation) GetSyncStatus() (*SyncStatus, error) {
	out, err := b.run("sync", "--status", "--json")
	if err != nil {
		// If sync branch doesn't exist, return empty status
		if strings.Contains(err.Error(), "does not exist") {
			return &SyncStatus{}, nil
		}
		return nil, err
	}

	var status SyncStatus
	if err := json.Unmarshal(out, &status); err != nil {
		return nil, fmt.Errorf("parsing bd sync status output: %w", err)
	}

	return &status, nil
}

// === Config Operations ===

// ConfigGet returns a config value.
// Returns empty string for keys that don't exist.
func (b *Implementation) ConfigGet(key string) (string, error) {
	out, err := b.run("config", "get", key)
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(out))
	// bd returns "key (not set)" for missing keys - normalize to empty string
	if strings.HasSuffix(value, "(not set)") {
		return "", nil
	}
	return value, nil
}

// ConfigSet sets a config value.
func (b *Implementation) ConfigSet(key, value string) error {
	_, err := b.run("config", "set", key, value)
	return err
}

// Init initializes a beads repository.
func (b *Implementation) Init(opts InitOptions) error {
	args := []string{"init"}

	if opts.Prefix != "" {
		args = append(args, "--prefix", opts.Prefix)
	}
	if opts.Quiet {
		args = append(args, "--quiet")
	}

	_, err := b.run(args...)
	return err
}

// Migrate runs database migrations.
func (b *Implementation) Migrate(opts MigrateOptions) error {
	args := []string{"migrate"}

	if opts.UpdateRepoID {
		args = append(args, "--update-repo-id")
	}
	if opts.Yes {
		args = append(args, "--yes")
	}

	_, err := b.run(args...)
	return err
}

// === Daemon Operations ===

// DaemonStart starts the bd daemon.
func (b *Implementation) DaemonStart() error {
	_, err := b.runWithDaemon("daemon", "--start")
	return err
}

// DaemonStop stops the bd daemon.
func (b *Implementation) DaemonStop() error {
	_, err := b.runWithDaemon("daemon", "--stop")
	return err
}

// DaemonStatus returns the daemon status.
func (b *Implementation) DaemonStatus() (*DaemonStatus, error) {
	out, err := b.runWithDaemon("daemon", "--status", "--json")
	if err != nil {
		// Daemon not running is not an error, return stopped status
		return &DaemonStatus{Running: false}, nil
	}

	var status DaemonStatus
	if err := json.Unmarshal(out, &status); err != nil {
		return nil, fmt.Errorf("parsing bd daemon status output: %w", err)
	}

	return &status, nil
}

// DaemonHealth returns daemon health info.
func (b *Implementation) DaemonHealth() (*DaemonHealth, error) {
	out, err := b.runWithDaemon("daemon", "health", "--json")
	if err != nil {
		return nil, err
	}

	var health DaemonHealth
	if err := json.Unmarshal(out, &health); err != nil {
		return nil, fmt.Errorf("parsing bd daemon health output: %w", err)
	}

	return &health, nil
}

// === Molecule Operations ===

// MolSeed seeds molecules (e.g., patrol).
func (b *Implementation) MolSeed(opts MolSeedOptions) error {
	args := []string{"mol", "seed"}

	if opts.Patrol {
		args = append(args, "--patrol")
	}

	_, err := b.run(args...)
	return err
}

// MolCurrent returns the current step of a molecule.
func (b *Implementation) MolCurrent(moleculeID string) (*MolCurrentOutput, error) {
	out, err := b.run("mol", "current", moleculeID, "--json")
	if err != nil {
		return nil, err
	}

	var step MolCurrentOutput
	if err := json.Unmarshal(out, &step); err != nil {
		return nil, fmt.Errorf("parsing bd mol current output: %w", err)
	}

	return &step, nil
}

// MolCatalog lists available molecule prototypes.
func (b *Implementation) MolCatalog() ([]*MoleculeProto, error) {
	out, err := b.run("mol", "catalog", "--json")
	if err != nil {
		// Catalog command may not exist yet, return empty list
		return []*MoleculeProto{}, nil
	}

	// Try parsing as wrapped object first
	var wrapper struct {
		Protos []*MoleculeProto `json:"protos"`
	}
	if err := json.Unmarshal(out, &wrapper); err == nil && wrapper.Protos != nil {
		return wrapper.Protos, nil
	}

	// Fall back to array format
	var protos []*MoleculeProto
	if err := json.Unmarshal(out, &protos); err != nil {
		// If parsing fails, return empty list (command may output help text)
		return []*MoleculeProto{}, nil
	}

	return protos, nil
}

// WispCreate creates a wisp from a prototype.
func (b *Implementation) WispCreate(protoID, actor string) (*Issue, error) {
	return b.WispCreateWithOptions(WispCreateOptions{
		ProtoID: protoID,
		Actor:   actor,
	})
}

// WispCreateWithOptions creates a wisp with full options including variables.
func (b *Implementation) WispCreateWithOptions(opts WispCreateOptions) (*Issue, error) {
	// bd routes this command natively via mayor/town.json -> routes.jsonl
	// Use shorthand: bd mol wisp <proto> --var key=value ...
	args := []string{"mol", "wisp", opts.ProtoID}
	if opts.Actor != "" {
		args = append(args, "--actor", opts.Actor)
	}
	for key, value := range opts.Variables {
		args = append(args, "--var", key+"="+value)
	}

	out, err := b.run(args...)
	if err != nil {
		return nil, err
	}

	var issue Issue
	if err := json.Unmarshal(out, &issue); err != nil {
		return nil, fmt.Errorf("parsing bd mol wisp output: %w", err)
	}

	return &issue, nil
}

// WispList lists all wisps.
func (b *Implementation) WispList(all bool) ([]*Issue, error) {
	args := []string{"mol", "wisp", "list", "--json"}
	if all {
		args = append(args, "--all")
	}

	out, err := b.run(args...)
	if err != nil {
		return []*Issue{}, nil
	}

	// bd returns {"wisps": [...], "count": N} wrapper
	var wrapper struct {
		Wisps []*Issue `json:"wisps"`
	}
	if err := json.Unmarshal(out, &wrapper); err != nil {
		return nil, fmt.Errorf("parsing bd mol wisp list output: %w", err)
	}

	if wrapper.Wisps == nil {
		return []*Issue{}, nil
	}
	return wrapper.Wisps, nil
}

// WispGC garbage collects old wisps.
func (b *Implementation) WispGC() error {
	_, err := b.run("mol", "wisp", "gc")
	return err
}

// MolBurn burns (deletes) wisps/molecules with --force.
func (b *Implementation) MolBurn(ids ...string) error {
	if len(ids) == 0 {
		return nil
	}
	args := []string{"mol", "burn", "--force"}
	args = append(args, ids...)
	_, err := b.run(args...)
	return err
}

// MolBond bonds a wisp to a bead, creating a compound.
func (b *Implementation) MolBond(wispID, beadID string) (*Issue, error) {
	out, err := b.run("mol", "bond", wispID, beadID, "--json")
	if err != nil {
		return nil, err
	}

	var issue Issue
	if err := json.Unmarshal(out, &issue); err != nil {
		return nil, fmt.Errorf("parsing bd mol bond output: %w", err)
	}

	return &issue, nil
}

// === Gate Operations ===

// GateShow returns gate status.
func (b *Implementation) GateShow(gateID string) (*Gate, error) {
	// bd routes this command natively via mayor/town.json -> routes.jsonl
	out, err := b.run("gate", "show", gateID, "--json")
	if err != nil {
		return nil, err
	}

	var gate Gate
	if err := json.Unmarshal(out, &gate); err != nil {
		return nil, fmt.Errorf("parsing bd gate show output: %w", err)
	}

	return &gate, nil
}

// GateWait waits for a gate to open, optionally notifying an agent.
// bd routes this command natively via mayor/town.json -> routes.jsonl
func (b *Implementation) GateWait(gateID, notifyAgent string) error {
	args := []string{"gate", "wait", gateID}
	if notifyAgent != "" {
		args = append(args, "--notify", notifyAgent)
	}

	_, err := b.runWithDaemon(args...)
	return err
}

// GateList returns all gates, optionally including closed ones.
func (b *Implementation) GateList(all bool) ([]*Gate, error) {
	args := []string{"gate", "list", "--json"}
	if all {
		args = append(args, "--all")
	}

	out, err := b.run(args...)
	if err != nil {
		return []*Gate{}, nil
	}

	var gates []*Gate
	if err := json.Unmarshal(out, &gates); err != nil {
		return nil, fmt.Errorf("parsing gate list output: %w", err)
	}

	// bd returns null for empty list, normalize to empty slice
	if gates == nil {
		return []*Gate{}, nil
	}
	return gates, nil
}

// GateResolve manually closes/resolves a gate.
func (b *Implementation) GateResolve(gateID string) error {
	_, err := b.runWithDaemon("gate", "resolve", gateID)
	return err
}

// GateAddWaiter adds a waiter to a gate.
func (b *Implementation) GateAddWaiter(gateID, waiterID string) error {
	_, err := b.runWithDaemon("gate", "add-waiter", gateID, waiterID)
	return err
}

// GateCheck evaluates all open gates and closes resolved ones.
func (b *Implementation) GateCheck() error {
	_, err := b.runWithDaemon("gate", "check")
	return err
}

// === Swarm Operations ===

// SwarmStatus returns swarm status.
// bd routes this command natively via mayor/town.json -> routes.jsonl
func (b *Implementation) SwarmStatus(swarmID string) (*SwarmStatus, error) {
	out, err := b.run("swarm", "status", swarmID, "--json")
	if err != nil {
		return nil, err
	}

	var status SwarmStatus
	if err := json.Unmarshal(out, &status); err != nil {
		return nil, fmt.Errorf("parsing bd swarm status output: %w", err)
	}

	return &status, nil
}

// SwarmCreate creates a swarm molecule from an epic.
func (b *Implementation) SwarmCreate(epicID string) (*Issue, error) {
	out, err := b.runWithDaemon("swarm", "create", epicID, "--json")
	if err != nil {
		return nil, err
	}

	var issue Issue
	if err := json.Unmarshal(out, &issue); err != nil {
		return nil, fmt.Errorf("parsing bd swarm create output: %w", err)
	}

	return &issue, nil
}

// SwarmList returns all swarm molecules.
func (b *Implementation) SwarmList() ([]*Issue, error) {
	out, err := b.run("swarm", "list", "--json")
	if err != nil {
		return nil, err
	}

	// bd returns {"swarms": [...]} wrapper
	var wrapper struct {
		Swarms []*Issue `json:"swarms"`
	}
	if err := json.Unmarshal(out, &wrapper); err != nil {
		return nil, fmt.Errorf("parsing bd swarm list output: %w", err)
	}

	if wrapper.Swarms == nil {
		return []*Issue{}, nil
	}
	return wrapper.Swarms, nil
}

// SwarmValidate validates epic structure for swarming.
func (b *Implementation) SwarmValidate(epicID string) error {
	_, err := b.run("swarm", "validate", epicID)
	return err
}

// === Formula Operations ===

// FormulaShow returns formula definition.
func (b *Implementation) FormulaShow(name string) (*Formula, error) {
	out, err := b.run("formula", "show", name)
	if err != nil {
		return nil, err
	}

	var formula Formula
	if err := json.Unmarshal(out, &formula); err != nil {
		return nil, fmt.Errorf("parsing bd formula show output: %w", err)
	}

	return &formula, nil
}

// FormulaList returns all available formulas.
func (b *Implementation) FormulaList() ([]*Formula, error) {
	out, err := b.run("formula", "list", "--json")
	if err != nil {
		// formula list may not be implemented, return empty list
		return []*Formula{}, nil
	}

	var formulas []*Formula
	if err := json.Unmarshal(out, &formulas); err != nil {
		// If parsing fails, return empty list
		return []*Formula{}, nil
	}

	if formulas == nil {
		return []*Formula{}, nil
	}
	return formulas, nil
}

// Cook executes a formula.
func (b *Implementation) Cook(formulaName string) (*Issue, error) {
	out, err := b.run("cook", formulaName, "--json")
	if err != nil {
		return nil, err
	}

	var issue Issue
	if err := json.Unmarshal(out, &issue); err != nil {
		return nil, fmt.Errorf("parsing bd cook output: %w", err)
	}

	return &issue, nil
}

// LegAdd adds a leg (step) to a formula instance.
// bd routes this command natively via mayor/town.json -> routes.jsonl
func (b *Implementation) LegAdd(formulaID, stepName string) error {
	_, err := b.run("leg", "add", formulaID, stepName)
	return err
}

// === Agent Operations ===

// AgentState sets the state of an agent bead.
// bd routes this command natively via mayor/town.json -> routes.jsonl
func (b *Implementation) AgentState(beadID, state string) error {
	_, err := b.run("agent", "state", beadID, state)
	return err
}

// === Label Operations ===

// LabelAdd adds a label to an issue.
// bd now routes this command by prefix natively, but we keep routedImpl for reliability.
func (b *Implementation) LabelAdd(id, label string) error {
	impl := b.routedImpl(id)
	_, err := impl.run("label", "add", id, label)
	return err
}

// LabelRemove removes a label from an issue.
// bd now routes this command by prefix natively, but we keep routedImpl for reliability.
func (b *Implementation) LabelRemove(id, label string) error {
	impl := b.routedImpl(id)
	_, err := impl.run("label", "remove", id, label)
	return err
}

// === Miscellaneous Operations ===

// Version returns the bd version.
func (b *Implementation) Version() (string, error) {
	out, err := b.run("version")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Doctor runs diagnostics.
func (b *Implementation) Doctor() (*DoctorReport, error) {
	out, err := b.run("doctor", "--json")
	if err != nil {
		return nil, err
	}

	var report DoctorReport
	if err := json.Unmarshal(out, &report); err != nil {
		return nil, fmt.Errorf("parsing bd doctor output: %w", err)
	}

	return &report, nil
}

// Prime returns the PRIME.md content.
func (b *Implementation) Prime() (string, error) {
	out, err := b.run("prime")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// SlotShow returns slot information.
// bd routes this command natively via mayor/town.json -> routes.jsonl
func (b *Implementation) SlotShow(id string) (*Slot, error) {
	out, err := b.run("slot", "show", id, "--json")
	if err != nil {
		return nil, err
	}

	var slot Slot
	if err := json.Unmarshal(out, &slot); err != nil {
		return nil, fmt.Errorf("parsing bd slot show output: %w", err)
	}

	return &slot, nil
}

// SlotSet sets a slot on an agent bead.
func (b *Implementation) SlotSet(agentID, slotName, beadID string) error {
	_, err := b.runWithDaemon("slot", "set", agentID, slotName, beadID)
	return err
}

// SlotClear clears a slot on an agent bead.
func (b *Implementation) SlotClear(agentID, slotName string) error {
	_, err := b.runWithDaemon("slot", "clear", agentID, slotName)
	return err
}

// Search searches issues by text query.
func (b *Implementation) Search(query string, opts SearchOptions) ([]*Issue, error) {
	args := []string{"search", query, "--json"}
	if opts.Status != "" {
		args = append(args, "--status", opts.Status)
	}
	if opts.Type != "" {
		args = append(args, "--type", opts.Type)
	}
	if opts.Limit > 0 {
		args = append(args, "--limit", fmt.Sprintf("%d", opts.Limit))
	}

	out, err := b.run(args...)
	if err != nil {
		return nil, err
	}

	var issues []*Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("parsing bd search output: %w", err)
	}

	return issues, nil
}

// Flush flushes pending writes to the database.
func (b *Implementation) Flush() error {
	_, err := b.runWithDaemon("daemon", "flush")
	return err
}

// Stats returns repository statistics as text.
func (b *Implementation) Stats() (string, error) {
	out, err := b.run("stats")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// StatsJSON returns repository statistics as structured data.
func (b *Implementation) StatsJSON() (*RepoStats, error) {
	out, err := b.run("stats", "--json")
	if err != nil {
		return nil, err
	}

	var stats RepoStats
	if err := json.Unmarshal(out, &stats); err != nil {
		return nil, fmt.Errorf("parsing bd stats output: %w", err)
	}

	return &stats, nil
}

// Burn records cost information.
func (b *Implementation) Burn(opts BurnOptions) error {
	args := []string{"burn"}

	if opts.SessionID != "" {
		args = append(args, "--session="+opts.SessionID)
	}
	if opts.Tokens > 0 {
		args = append(args, fmt.Sprintf("--tokens=%d", opts.Tokens))
	}
	if opts.Cost > 0 {
		args = append(args, fmt.Sprintf("--cost=%.6f", opts.Cost))
	}
	if opts.Model != "" {
		args = append(args, "--model="+opts.Model)
	}

	_, err := b.run(args...)
	return err
}

// Comment adds a comment to an issue.
// bd now routes this command by prefix natively, but we keep routedImpl for reliability.
func (b *Implementation) Comment(id, message string) error {
	impl := b.routedImpl(id)
	_, err := impl.run("comment", id, message)
	return err
}

// === Repository Checks ===

// IsBeadsRepo checks if the working directory is a beads repository.
// ZFC: Check file existence directly instead of parsing bd errors.
func (b *Implementation) IsBeadsRepo() bool {
	beadsDir := ResolveBeadsDir(b.workDir)
	info, err := os.Stat(beadsDir)
	return err == nil && info.IsDir()
}

// === Convenience Methods (not part of interface) ===

// ListByAssignee returns all issues assigned to a specific assignee.
// The assignee is typically in the format "rig/polecatName" (e.g., "gastown/Toast").
func (b *Implementation) ListByAssignee(assignee string) ([]*Issue, error) {
	return b.List(ListOptions{
		Status:   "all", // Include both open and closed for state derivation
		Assignee: assignee,
		Priority: -1, // No priority filter
	})
}

// GetAssignedIssue returns the first open issue assigned to the given assignee.
// Returns nil if no open issue is assigned.
func (b *Implementation) GetAssignedIssue(assignee string) (*Issue, error) {
	issues, err := b.List(ListOptions{
		Status:   "open",
		Assignee: assignee,
		Priority: -1,
	})
	if err != nil {
		return nil, err
	}

	// Also check in_progress status explicitly
	if len(issues) == 0 {
		issues, err = b.List(ListOptions{
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
// Deprecated: Use ReadyWithLabel instead.
func (b *Implementation) ReadyWithType(issueType string) ([]*Issue, error) {
	return b.ReadyWithLabel("gt:"+issueType, 100)
}

// Release moves an in_progress issue back to open status.
// This is used to recover stuck steps when a worker dies mid-task.
// It clears the assignee so the step can be claimed by another worker.
func (b *Implementation) Release(id string) error {
	return b.ReleaseWithReason(id, "")
}

// ReleaseWithReason moves an in_progress issue back to open status with a reason.
// The reason is added as a note to the issue for tracking purposes.
func (b *Implementation) ReleaseWithReason(id, reason string) error {
	opts := UpdateOptions{
		Status:   strPtr("open"),
		Assignee: strPtr(""),
	}

	// Add reason as a note if provided
	if reason != "" {
		opts.Notes = "Released: " + reason
	}

	return b.Update(id, opts)
}

// strPtr returns a pointer to the string.
func strPtr(s string) *string {
	return &s
}
