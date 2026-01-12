// Package beads provides a wrapper for the bd (beads) CLI.
package beads

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/steveyegge/gastown/internal/runtime"
)

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
	DependencyType string `json:"dependency_type,omitempty"`
}

// ListOptions specifies filters for listing issues.
type ListOptions struct {
	Status     string // "open", "closed", "all"
	Type       string // Deprecated: use Label instead. "task", "bug", "feature", "epic"
	Label      string // Label filter (e.g., "gt:agent", "gt:merge-request")
	Priority   int    // 0-4, -1 for no filter
	Parent     string // filter by parent ID
	Assignee   string // filter by assignee (e.g., "gastown/Toast")
	NoAssignee bool   // filter for issues with no assignee
}

// CreateOptions specifies options for creating an issue.
type CreateOptions struct {
	Title       string
	Type        string   // Deprecated: use Labels instead. "task", "bug", "feature", "epic" - converted to gt:<type> label
	Priority    int      // 0-4
	Description string
	Parent      string
	Actor       string   // Who is creating this issue (populates created_by)
	ID          string   // Optional: explicit ID (enables routing via prefix extraction)
	Labels      []string // Optional: additional labels (e.g., "gt:agent", "gt:convoy")
	BdType      string   // Optional: raw bd --type value (e.g., "convoy", "agent") - NOT converted to label
}

// UpdateOptions specifies options for updating an issue.
type UpdateOptions struct {
	Title        *string
	Status       *string
	Priority     *int
	Description  *string
	Assignee     *string
	AddLabels    []string // Labels to add
	RemoveLabels []string // Labels to remove
	SetLabels    []string // Labels to set (replaces all existing)
}

// SyncStatus represents the sync status of the beads repository.
type SyncStatus struct {
	Branch    string
	Ahead     int
	Behind    int
	Conflicts []string
}

// Beads wraps bd CLI operations with context for town and/or rig operations.
//
// Create with New(townRoot) for town-only operations, or New(townRoot, rigPath)
// for both town and rig operations. All fields are computed at initialization.
//
// Example:
//
//	bd := beads.New(townRoot)              // Town-only: can create convoys, town agents
//	bd := beads.New(townRoot, rigPath)     // Full: can also create rig agents, rig beads
type Beads struct {
	workDir   string // Working directory for bd commands (equals townRoot, for backwards compat)
	townRoot  string // Town root directory (for town operations)
	rigPath   string // Rig directory (empty for town-only)
	rigName   string // Rig name derived from rigPath (empty for town-only)
	rigPrefix string // Rig prefix from routes.jsonl (empty for town-only)
}

// New creates a Beads instance for the given town.
// If rigPath is provided, rig operations are also available.
//
// Example:
//
//	bd := beads.New(townRoot)              // Town-only
//	bd := beads.New(townRoot, rigPath)     // Town + rig
//	bd.CreateTownConvoy(...)               // Always available
//	bd.CreateRigAgent(...)             // Only if rigPath provided
func New(townRoot string, rigPath ...string) *Beads {
	b := &Beads{workDir: townRoot, townRoot: townRoot}
	if len(rigPath) > 0 && rigPath[0] != "" {
		b.rigPath = rigPath[0]
		b.rigName = filepath.Base(rigPath[0])
		b.rigPrefix = GetPrefixForRig(townRoot, b.rigName)
	}
	return b
}

// TownRoot returns the town root directory.
func (b *Beads) TownRoot() string { return b.townRoot }

// RigPath returns the rig directory, or empty for town-only context.
func (b *Beads) RigPath() string { return b.rigPath }

// RigName returns the rig name, or empty for town-only context.
func (b *Beads) RigName() string { return b.rigName }

// RigPrefix returns the rig prefix (without hyphen), or empty for town-only.
func (b *Beads) RigPrefix() string { return b.rigPrefix }

// IsTown returns true - all instances can do town operations.
func (b *Beads) IsTown() bool { return true }

// IsRig returns true if rig operations are available (rigPath was provided).
func (b *Beads) IsRig() bool { return b.rigPath != "" }

// run executes a bd command from townRoot and returns stdout.
// For rig-specific operations, use runRig() instead.
func (b *Beads) run(args ...string) ([]byte, error) {
	return b.runAt(b.townRoot, args...)
}

// runRig executes a bd command from rigPath and returns stdout.
// Panics if rigPath is not set.
func (b *Beads) runRig(args ...string) ([]byte, error) {
	if b.rigPath == "" {
		panic("runRig called without rig context")
	}
	return b.runAt(b.rigPath, args...)
}

// runAt executes a bd command from the specified directory.
func (b *Beads) runAt(workDir string, args ...string) ([]byte, error) {
	// Use --no-daemon for faster read operations (avoids daemon IPC overhead)
	// The daemon is primarily useful for write coalescing, not reads
	fullArgs := append([]string{"--no-daemon"}, args...)
	cmd := exec.Command("bd", fullArgs...) //nolint:gosec // G204: bd is a trusted internal tool
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return nil, b.wrapError(err, stderr.String(), args)
	}

	return stdout.Bytes(), nil
}

// Run executes a bd command from townRoot and returns stdout.
// This is a public wrapper for cases where callers need arbitrary bd commands.
func (b *Beads) Run(args ...string) ([]byte, error) {
	return b.run(args...)
}

// wrapError wraps bd errors with context.
// ZFC: Avoid parsing stderr to make decisions. Transport errors to agents instead.
// Exception: ErrNotInstalled (exec.ErrNotFound) and ErrNotFound (issue lookup) are
// acceptable as they enable basic error handling without decision-making.
func (b *Beads) wrapError(err error, stderr string, args []string) error {
	stderr = strings.TrimSpace(stderr)

	// Check for bd not installed
	if execErr, ok := err.(*exec.Error); ok && errors.Is(execErr.Err, exec.ErrNotFound) {
		return ErrNotInstalled
	}

	// ErrNotFound is widely used for issue lookups - acceptable exception
	if strings.Contains(stderr, "not found") || strings.Contains(stderr, "Issue not found") {
		return ErrNotFound
	}

	if stderr != "" {
		return fmt.Errorf("bd %s: %s", strings.Join(args, " "), stderr)
	}
	return fmt.Errorf("bd %s: %w", strings.Join(args, " "), err)
}

// List returns issues matching the given options.
func (b *Beads) List(opts ListOptions) ([]*Issue, error) {
	args := []string{"list", "--json"}

	if opts.Status != "" {
		args = append(args, "--status="+opts.Status)
	}
	// Prefer Label over Type (Type is deprecated)
	if opts.Label != "" {
		args = append(args, "--label="+opts.Label)
	} else if opts.Type != "" {
		// Deprecated: convert type to label for backward compatibility
		args = append(args, "--label=gt:"+opts.Type)
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

// ListByAssignee returns all issues assigned to a specific assignee.
// The assignee is typically in the format "rig/polecatName" (e.g., "gastown/Toast").
func (b *Beads) ListByAssignee(assignee string) ([]*Issue, error) {
	return b.List(ListOptions{
		Status:   "all", // Include both open and closed for state derivation
		Assignee: assignee,
		Priority: -1, // No priority filter
	})
}

// GetAssignedIssue returns the first open issue assigned to the given assignee.
// Returns nil if no open issue is assigned.
func (b *Beads) GetAssignedIssue(assignee string) (*Issue, error) {
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

// Ready returns issues that are ready to work (not blocked).
func (b *Beads) Ready() ([]*Issue, error) {
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

// ReadyWithType returns ready issues filtered by label.
// Uses bd ready --label flag for server-side filtering.
// The issueType is converted to a gt:<type> label (e.g., "molecule" -> "gt:molecule").
func (b *Beads) ReadyWithType(issueType string) ([]*Issue, error) {
	out, err := b.run("ready", "--json", "--label", "gt:"+issueType, "-n", "100")
	if err != nil {
		return nil, err
	}

	var issues []*Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("parsing bd ready output: %w", err)
	}

	return issues, nil
}

// Show returns detailed information about an issue.
func (b *Beads) Show(id string) (*Issue, error) {
	out, err := b.run("show", id, "--json")
	if err != nil {
		return nil, err
	}

	// bd show returns empty output for non-existent beads (exit 0)
	if len(bytes.TrimSpace(out)) == 0 {
		return nil, ErrNotFound
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
func (b *Beads) ShowMultiple(ids []string) (map[string]*Issue, error) {
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

// Blocked returns issues that are blocked by dependencies.
func (b *Beads) Blocked() ([]*Issue, error) {
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

// Create creates a new issue and returns it.
// If opts.Actor is empty, it defaults to the BD_ACTOR environment variable.
// This ensures created_by is populated for issue provenance tracking.
//
// If opts.ID is set, the working directory is determined by the ID prefix:
// - Town prefix (hq-): uses townRoot
// - Rig prefix: uses rigPath (requires rig context)
func (b *Beads) Create(opts CreateOptions) (*Issue, error) {
	args := []string{"create", "--json"}

	// Determine workDir based on ID prefix.
	//
	// BD BUG WORKAROUND: When both --prefix and --id are passed to bd create,
	// bd ignores the explicit ID and generates a new one. To work around this,
	// we set cmd.Dir (via runAt) to target the correct database directly instead
	// of using --prefix for routing. The workDir determines which beads database
	// bd discovers, so we set it to townRoot for hq- prefix or rigPath for rig prefix.
	//
	// Without an explicit ID, we default to townRoot so that Create() from any
	// context creates in the town database (enabling convoy creation from rigs).
	workDir := b.townRoot
	if opts.ID != "" {
		args = append(args, "--id="+opts.ID)

		idPrefix := extractPrefix(opts.ID)
		if idPrefix == TownBeadsPrefix {
			// Town operation - use townRoot
			workDir = b.townRoot
		} else if idPrefix != "" && b.IsRig() && idPrefix == b.rigPrefix {
			// Rig operation - use rigPath
			workDir = b.rigPath
		} else if idPrefix != "" && !b.IsRig() {
			// ID has rig prefix but no rig context
			return nil, fmt.Errorf("ID %q has rig prefix %q but no rig context provided", opts.ID, idPrefix)
		} else if idPrefix != "" && idPrefix != b.rigPrefix {
			// ID prefix doesn't match our rig
			return nil, fmt.Errorf("ID prefix mismatch: ID %q has prefix %q but rig has %q", opts.ID, idPrefix, b.rigPrefix)
		}
	}

	if opts.Title != "" {
		args = append(args, "--title="+opts.Title)
	}
	// BdType is the raw bd --type value (e.g., "convoy", "agent")
	if opts.BdType != "" {
		args = append(args, "--type="+opts.BdType)
	}
	// Type is deprecated: convert to gt:<type> label
	if opts.Type != "" {
		args = append(args, "--labels=gt:"+opts.Type)
	}
	// Add explicit labels
	for _, label := range opts.Labels {
		args = append(args, "--labels="+label)
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
	// Default Actor from BD_ACTOR env var if not specified
	actor := opts.Actor
	if actor == "" {
		actor = os.Getenv("BD_ACTOR")
	}
	if actor != "" {
		args = append(args, "--actor="+actor)
	}

	out, err := b.runAt(workDir, args...)
	if err != nil {
		return nil, err
	}

	var issue Issue
	if err := json.Unmarshal(out, &issue); err != nil {
		return nil, fmt.Errorf("parsing bd create output: %w", err)
	}

	return &issue, nil
}

// extractPrefix extracts the prefix from a beads ID.
// For "hq-cv-xxx" returns "hq", for "gt-abc" returns "gt".
// Returns empty string if no hyphen found.
func extractPrefix(id string) string {
	idx := strings.Index(id, "-")
	if idx < 0 {
		return ""
	}
	return id[:idx]
}

// addPrefixForRouting adds --prefix to args for routing, with conflict detection.
// If --prefix is already in args:
//   - and matches the extracted prefix from id: no-op
//   - and differs: returns error
//
// If --prefix is not in args: adds --prefix=<extracted>
// Returns the updated args and any error.
func addPrefixForRouting(args []string, id string) ([]string, error) {
	extractedPrefix := extractPrefix(id)
	if extractedPrefix == "" {
		return args, nil
	}

	// Check if --prefix is already specified
	for _, arg := range args {
		if strings.HasPrefix(arg, "--prefix=") {
			existingPrefix := strings.TrimPrefix(arg, "--prefix=")
			if existingPrefix != extractedPrefix {
				return nil, fmt.Errorf("prefix mismatch: ID %q has prefix %q but --prefix=%s was specified", id, extractedPrefix, existingPrefix)
			}
			// Already specified and matches, no-op
			return args, nil
		}
	}

	// Not specified, add it
	return append(args, "--prefix="+extractedPrefix), nil
}

// CreateWithID creates an issue with a specific ID.
// This is useful for agent beads, role beads, and other beads that need
// deterministic IDs rather than auto-generated ones.
//
// NOTE: This method extracts the prefix from the ID and passes --prefix to bd
// to enable routing via routes.jsonl. Without --prefix, bd create --id validates
// against the LOCAL database prefix and fails if they don't match.
//
// Deprecated: Use Create with opts.ID set instead.
func (b *Beads) CreateWithID(id string, opts CreateOptions) (*Issue, error) {
	opts.ID = id
	return b.Create(opts)
}

// Update updates an existing issue.
func (b *Beads) Update(id string, opts UpdateOptions) error {
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
// If a runtime session ID is set in the environment, it is passed to bd close
// for work attribution tracking (see decision 009-session-events-architecture.md).
func (b *Beads) Close(ids ...string) error {
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
// If a runtime session ID is set in the environment, it is passed to bd close
// for work attribution tracking (see decision 009-session-events-architecture.md).
func (b *Beads) CloseWithReason(reason string, ids ...string) error {
	if len(ids) == 0 {
		return nil
	}

	args := append([]string{"close"}, ids...)
	args = append(args, "--reason="+reason)

	// Pass session ID for work attribution if available
	if sessionID := runtime.SessionIDFromEnv(); sessionID != "" {
		args = append(args, "--session="+sessionID)
	}

	_, err := b.run(args...)
	return err
}

// Release moves an in_progress issue back to open status.
// This is used to recover stuck steps when a worker dies mid-task.
// It clears the assignee so the step can be claimed by another worker.
func (b *Beads) Release(id string) error {
	return b.ReleaseWithReason(id, "")
}

// ReleaseWithReason moves an in_progress issue back to open status with a reason.
// The reason is added as a note to the issue for tracking purposes.
func (b *Beads) ReleaseWithReason(id, reason string) error {
	args := []string{"update", id, "--status=open", "--assignee="}

	// Add reason as a note if provided
	if reason != "" {
		args = append(args, "--notes=Released: "+reason)
	}

	_, err := b.run(args...)
	return err
}

// AddDependency adds a dependency: issue depends on dependsOn.
func (b *Beads) AddDependency(issue, dependsOn string) error {
	_, err := b.run("dep", "add", issue, dependsOn)
	return err
}

// RemoveDependency removes a dependency.
func (b *Beads) RemoveDependency(issue, dependsOn string) error {
	_, err := b.run("dep", "remove", issue, dependsOn)
	return err
}

// Sync syncs beads with remote.
func (b *Beads) Sync() error {
	_, err := b.run("sync")
	return err
}

// SyncFromMain syncs beads updates from main branch.
func (b *Beads) SyncFromMain() error {
	_, err := b.run("sync", "--from-main")
	return err
}

// GetSyncStatus returns the sync status without performing a sync.
func (b *Beads) GetSyncStatus() (*SyncStatus, error) {
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

// Stats returns repository statistics.
func (b *Beads) Stats() (string, error) {
	out, err := b.run("stats")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// IsBeadsRepo checks if the town root is a beads repository.
// ZFC: Check file existence directly instead of parsing bd errors.
func (b *Beads) IsBeadsRepo() bool {
	beadsDir := ResolveBeadsDir(b.townRoot)
	info, err := os.Stat(beadsDir)
	return err == nil && info.IsDir()
}

// primeContent is the Gas Town PRIME.md content that provides essential context
// for crew workers. This is the fallback if the SessionStart hook fails.
const primeContent = `# Gas Town Worker Context

> **Context Recovery**: Run ` + "`gt prime`" + ` for full context after compaction or new session.

## The Propulsion Principle (GUPP)

**If you find work on your hook, YOU RUN IT.**

No confirmation. No waiting. No announcements. The hook having work IS the assignment.
This is physics, not politeness. Gas Town is a steam engine - you are a piston.

**Failure mode we're preventing:**
- Agent starts with work on hook
- Agent announces itself and waits for human to say "ok go"
- Human is AFK / trusting the engine to run
- Work sits idle. The whole system stalls.

## Startup Protocol

1. Check your hook: ` + "`gt mol status`" + `
2. If work is hooked → EXECUTE (no announcement, no waiting)
3. If hook empty → Check mail: ` + "`gt mail inbox`" + `
4. Still nothing? Wait for user instructions

## Key Commands

- ` + "`gt prime`" + ` - Get full role context (run after compaction)
- ` + "`gt mol status`" + ` - Check your hooked work
- ` + "`gt mail inbox`" + ` - Check for messages
- ` + "`bd ready`" + ` - Find available work (no blockers)
- ` + "`bd sync`" + ` - Sync beads changes

## Session Close Protocol

Before saying "done":
1. git status (check what changed)
2. git add <files> (stage code changes)
3. bd sync (commit beads changes)
4. git commit -m "..." (commit code)
5. bd sync (commit any new beads changes)
6. git push (push to remote)

**Work is not done until pushed.**
`

// ProvisionPrimeMD writes the Gas Town PRIME.md file to the specified beads directory.
// This provides essential Gas Town context (GUPP, startup protocol) as a fallback
// if the SessionStart hook fails. The PRIME.md is read by bd prime.
//
// The beadsDir should be the actual beads directory (after following any redirect).
// Returns nil if PRIME.md already exists (idempotent).
func ProvisionPrimeMD(beadsDir string) error {
	primePath := filepath.Join(beadsDir, "PRIME.md")

	// Check if already exists - don't overwrite customizations
	if _, err := os.Stat(primePath); err == nil {
		return nil // Already exists, don't overwrite
	}

	// Create .beads directory if it doesn't exist
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		return fmt.Errorf("creating beads dir: %w", err)
	}

	// Write PRIME.md
	if err := os.WriteFile(primePath, []byte(primeContent), 0644); err != nil {
		return fmt.Errorf("writing PRIME.md: %w", err)
	}

	return nil
}

// ProvisionPrimeMDForWorktree provisions PRIME.md for a worktree by following its redirect.
// This is the main entry point for crew/polecat provisioning.
func ProvisionPrimeMDForWorktree(worktreePath string) error {
	// Resolve the beads directory (follows redirect chain)
	beadsDir := ResolveBeadsDir(worktreePath)

	// Provision PRIME.md in the target directory
	return ProvisionPrimeMD(beadsDir)
}

// Init initializes a new beads database with the given prefix.
// This creates the .beads/ directory and sets up the database configuration.
func (b *Beads) Init(prefix string) error {
	args := []string{"init", "--prefix", prefix}
	_, err := b.run(args...)
	return err
}

// GetConfig retrieves a configuration value from the beads database.
// Common keys: "issue_prefix", "types.custom", "sync-branch"
func (b *Beads) GetConfig(key string) (string, error) {
	args := []string{"config", "get", key}
	out, err := b.run(args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// SetConfig sets a configuration value in the beads database.
func (b *Beads) SetConfig(key, value string) error {
	args := []string{"config", "set", key, value}
	_, err := b.run(args...)
	return err
}

// CreateWithPrefix creates a new issue with an explicit prefix.
// This is useful when you need to create a bead in a different database
// than the one discovered from workDir (routing via routes.jsonl).
// Note: Actor is always taken from BD_ACTOR env var (set by config/env.go).
func (b *Beads) CreateWithPrefix(prefix string, opts CreateOptions) (*Issue, error) {
	args := []string{"create", "--json", "--prefix=" + prefix}

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
	// Actor comes from BD_ACTOR env var (set by config/env.go for all roles)
	if actor := os.Getenv("BD_ACTOR"); actor != "" {
		args = append(args, "--actor="+actor)
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
