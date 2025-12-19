package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/config"
	"github.com/steveyegge/gastown/internal/git"
	"github.com/steveyegge/gastown/internal/refinery"
	"github.com/steveyegge/gastown/internal/rig"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/workspace"
)

// MQ command flags
var (
	// Submit flags
	mqSubmitBranch   string
	mqSubmitIssue    string
	mqSubmitEpic     string
	mqSubmitPriority int

	// Retry flags
	mqRetryNow bool

	// Reject flags
	mqRejectReason string
	mqRejectNotify bool

	// List command flags
	mqListReady  bool
	mqListStatus string
	mqListWorker string
	mqListEpic   string
	mqListJSON   bool

	// Status command flags
	mqStatusJSON bool
)

var mqCmd = &cobra.Command{
	Use:   "mq",
	Short: "Merge queue operations",
	Long: `Manage the merge queue for a rig.

The merge queue tracks work branches from polecats waiting to be merged.
Use these commands to view, submit, retry, and manage merge requests.`,
}

var mqSubmitCmd = &cobra.Command{
	Use:   "submit",
	Short: "Submit current branch to the merge queue",
	Long: `Submit the current branch to the merge queue.

Creates a merge-request bead that will be processed by the Engineer.

Auto-detection:
  - Branch: current git branch
  - Issue: parsed from branch name (e.g., polecat/Nux/gt-xyz → gt-xyz)
  - Worker: parsed from branch name
  - Rig: detected from current directory
  - Target: main (or integration/<epic> if --epic specified)
  - Priority: inherited from source issue

Examples:
  gt mq submit                           # Auto-detect everything
  gt mq submit --issue gt-abc            # Explicit issue
  gt mq submit --epic gt-xyz             # Target integration branch
  gt mq submit --priority 0              # Override priority (P0)`,
	RunE: runMqSubmit,
}

var mqRetryCmd = &cobra.Command{
	Use:   "retry <rig> <mr-id>",
	Short: "Retry a failed merge request",
	Long: `Retry a failed merge request.

Resets a failed MR so it can be processed again by the refinery.
The MR must be in a failed state (open with an error).

Examples:
  gt mq retry gastown gt-mr-abc123
  gt mq retry gastown gt-mr-abc123 --now`,
	Args: cobra.ExactArgs(2),
	RunE: runMQRetry,
}

var mqListCmd = &cobra.Command{
	Use:   "list <rig>",
	Short: "Show the merge queue",
	Long: `Show the merge queue for a rig.

Lists all pending merge requests waiting to be processed.

Output format:
  ID          STATUS       PRIORITY  BRANCH                    WORKER  AGE
  gt-mr-001   ready        P0        polecat/Nux/gt-xyz        Nux     5m
  gt-mr-002   in_progress  P1        polecat/Toast/gt-abc      Toast   12m
  gt-mr-003   blocked      P1        polecat/Capable/gt-def    Capable 8m
              (waiting on gt-mr-001)

Examples:
  gt mq list gastown
  gt mq list gastown --ready
  gt mq list gastown --status=open
  gt mq list gastown --worker=Nux`,
	Args: cobra.ExactArgs(1),
	RunE: runMQList,
}

var mqRejectCmd = &cobra.Command{
	Use:   "reject <rig> <mr-id-or-branch>",
	Short: "Reject a merge request",
	Long: `Manually reject a merge request.

This closes the MR with a 'rejected' status without merging.
The source issue is NOT closed (work is not done).

Examples:
  gt mq reject gastown polecat/Nux/gt-xyz --reason "Does not meet requirements"
  gt mq reject gastown mr-Nux-12345 --reason "Superseded by other work" --notify`,
	Args: cobra.ExactArgs(2),
	RunE: runMQReject,
}

var mqStatusCmd = &cobra.Command{
	Use:   "status <id>",
	Short: "Show detailed merge request status",
	Long: `Display detailed information about a merge request.

Shows all MR fields, current status with timestamps, dependencies,
blockers, and processing history.

Example:
  gt mq status gt-mr-abc123`,
	Args: cobra.ExactArgs(1),
	RunE: runMqStatus,
}

var mqIntegrationCmd = &cobra.Command{
	Use:   "integration",
	Short: "Manage integration branches for epics",
	Long: `Manage integration branches for batch work on epics.

Integration branches allow multiple MRs for an epic to target a shared
branch instead of main. After all epic work is complete, the integration
branch is landed to main as a single atomic unit.

Commands:
  create  Create an integration branch for an epic
  land    Merge integration branch to main (not yet implemented)
  status  Show integration branch status (not yet implemented)`,
}

var mqIntegrationCreateCmd = &cobra.Command{
	Use:   "create <epic-id>",
	Short: "Create an integration branch for an epic",
	Long: `Create an integration branch for batch work on an epic.

Creates a branch named integration/<epic-id> from main and pushes it
to origin. Future MRs for this epic's children can target this branch.

Actions:
  1. Verify epic exists
  2. Create branch integration/<epic-id> from main
  3. Push to origin
  4. Store integration branch info in epic metadata

Example:
  gt mq integration create gt-auth-epic
  # Creates integration/gt-auth-epic from main`,
	Args: cobra.ExactArgs(1),
	RunE: runMqIntegrationCreate,
}

func init() {
	// Submit flags
	mqSubmitCmd.Flags().StringVar(&mqSubmitBranch, "branch", "", "Source branch (default: current branch)")
	mqSubmitCmd.Flags().StringVar(&mqSubmitIssue, "issue", "", "Source issue ID (default: parse from branch name)")
	mqSubmitCmd.Flags().StringVar(&mqSubmitEpic, "epic", "", "Target epic's integration branch instead of main")
	mqSubmitCmd.Flags().IntVarP(&mqSubmitPriority, "priority", "p", -1, "Override priority (0-4, default: inherit from issue)")

	// Retry flags
	mqRetryCmd.Flags().BoolVar(&mqRetryNow, "now", false, "Immediately process instead of waiting for refinery loop")

	// List flags
	mqListCmd.Flags().BoolVar(&mqListReady, "ready", false, "Show only ready-to-merge (no blockers)")
	mqListCmd.Flags().StringVar(&mqListStatus, "status", "", "Filter by status (open, in_progress, closed)")
	mqListCmd.Flags().StringVar(&mqListWorker, "worker", "", "Filter by worker name")
	mqListCmd.Flags().StringVar(&mqListEpic, "epic", "", "Show MRs targeting integration/<epic>")
	mqListCmd.Flags().BoolVar(&mqListJSON, "json", false, "Output as JSON")

	// Reject flags
	mqRejectCmd.Flags().StringVarP(&mqRejectReason, "reason", "r", "", "Reason for rejection (required)")
	mqRejectCmd.Flags().BoolVar(&mqRejectNotify, "notify", false, "Send mail notification to worker")
	_ = mqRejectCmd.MarkFlagRequired("reason")

	// Status flags
	mqStatusCmd.Flags().BoolVar(&mqStatusJSON, "json", false, "Output as JSON")

	// Add subcommands
	mqCmd.AddCommand(mqSubmitCmd)
	mqCmd.AddCommand(mqRetryCmd)
	mqCmd.AddCommand(mqListCmd)
	mqCmd.AddCommand(mqRejectCmd)
	mqCmd.AddCommand(mqStatusCmd)

	// Integration branch subcommands
	mqIntegrationCmd.AddCommand(mqIntegrationCreateCmd)
	mqCmd.AddCommand(mqIntegrationCmd)

	rootCmd.AddCommand(mqCmd)
}

// branchInfo holds parsed branch information.
type branchInfo struct {
	Branch string // Full branch name
	Issue  string // Issue ID extracted from branch
	Worker string // Worker name (polecat name)
}

// parseBranchName extracts issue ID and worker from a branch name.
// Supports formats:
//   - polecat/<worker>/<issue>  → issue=<issue>, worker=<worker>
//   - <issue>                   → issue=<issue>, worker=""
func parseBranchName(branch string) branchInfo {
	info := branchInfo{Branch: branch}

	// Try polecat/<worker>/<issue> format
	if strings.HasPrefix(branch, "polecat/") {
		parts := strings.SplitN(branch, "/", 3)
		if len(parts) == 3 {
			info.Worker = parts[1]
			info.Issue = parts[2]
			return info
		}
	}

	// Try to find an issue ID pattern in the branch name
	// Common patterns: prefix-xxx, prefix-xxx.n (subtask)
	issuePattern := regexp.MustCompile(`([a-z]+-[a-z0-9]+(?:\.[0-9]+)?)`)
	if matches := issuePattern.FindStringSubmatch(branch); len(matches) > 1 {
		info.Issue = matches[1]
	}

	return info
}

// findCurrentRig determines the current rig from the working directory.
// Returns the rig name and rig object, or an error if not in a rig.
func findCurrentRig(townRoot string) (string, *rig.Rig, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", nil, fmt.Errorf("getting current directory: %w", err)
	}

	// Get relative path from town root to cwd
	relPath, err := filepath.Rel(townRoot, cwd)
	if err != nil {
		return "", nil, fmt.Errorf("computing relative path: %w", err)
	}

	// The first component of the relative path should be the rig name
	parts := strings.Split(relPath, string(filepath.Separator))
	if len(parts) == 0 || parts[0] == "" || parts[0] == "." {
		return "", nil, fmt.Errorf("not inside a rig directory")
	}

	rigName := parts[0]

	// Load rig manager and get the rig
	rigsConfigPath := filepath.Join(townRoot, "mayor", "rigs.json")
	rigsConfig, err := config.LoadRigsConfig(rigsConfigPath)
	if err != nil {
		rigsConfig = &config.RigsConfig{Rigs: make(map[string]config.RigEntry)}
	}

	g := git.NewGit(townRoot)
	rigMgr := rig.NewManager(townRoot, rigsConfig, g)
	r, err := rigMgr.GetRig(rigName)
	if err != nil {
		return "", nil, fmt.Errorf("rig '%s' not found: %w", rigName, err)
	}

	return rigName, r, nil
}

func runMqSubmit(cmd *cobra.Command, args []string) error {
	// Find workspace
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return fmt.Errorf("not in a Gas Town workspace: %w", err)
	}

	// Find current rig
	rigName, _, err := findCurrentRig(townRoot)
	if err != nil {
		return err
	}

	// Initialize git for the current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}
	g := git.NewGit(cwd)

	// Get current branch
	branch := mqSubmitBranch
	if branch == "" {
		branch, err = g.CurrentBranch()
		if err != nil {
			return fmt.Errorf("getting current branch: %w", err)
		}
	}

	if branch == "main" || branch == "master" {
		return fmt.Errorf("cannot submit main/master branch to merge queue")
	}

	// Parse branch info
	info := parseBranchName(branch)

	// Override with explicit flags
	issueID := mqSubmitIssue
	if issueID == "" {
		issueID = info.Issue
	}
	worker := info.Worker

	if issueID == "" {
		return fmt.Errorf("cannot determine source issue from branch '%s'; use --issue to specify", branch)
	}

	// Determine target branch
	target := "main"
	if mqSubmitEpic != "" {
		target = "integration/" + mqSubmitEpic
	}

	// Initialize beads
	bd := beads.New(cwd)

	// Get source issue for priority inheritance
	var priority int
	if mqSubmitPriority >= 0 {
		priority = mqSubmitPriority
	} else {
		// Try to inherit from source issue
		sourceIssue, err := bd.Show(issueID)
		if err != nil {
			// Issue not found, use default priority
			priority = 2
		} else {
			priority = sourceIssue.Priority
		}
	}

	// Build title
	title := fmt.Sprintf("Merge: %s", issueID)

	// Build description with MR fields
	mrFields := &beads.MRFields{
		Branch:      branch,
		Target:      target,
		SourceIssue: issueID,
		Worker:      worker,
		Rig:         rigName,
	}
	description := beads.FormatMRFields(mrFields)

	// Create the merge-request issue
	createOpts := beads.CreateOptions{
		Title:       title,
		Type:        "merge-request",
		Priority:    priority,
		Description: description,
	}

	issue, err := bd.Create(createOpts)
	if err != nil {
		return fmt.Errorf("creating merge request: %w", err)
	}

	// Success output
	fmt.Printf("%s Created merge request\n", style.Bold.Render("✓"))
	fmt.Printf("  MR ID: %s\n", style.Bold.Render(issue.ID))
	fmt.Printf("  Source: %s\n", branch)
	fmt.Printf("  Target: %s\n", target)
	fmt.Printf("  Issue: %s\n", issueID)
	if worker != "" {
		fmt.Printf("  Worker: %s\n", worker)
	}
	fmt.Printf("  Priority: P%d\n", priority)

	return nil
}

func runMQRetry(cmd *cobra.Command, args []string) error {
	rigName := args[0]
	mrID := args[1]

	mgr, _, err := getRefineryManager(rigName)
	if err != nil {
		return err
	}

	// Get the MR first to show info
	mr, err := mgr.GetMR(mrID)
	if err != nil {
		if err == refinery.ErrMRNotFound {
			return fmt.Errorf("merge request '%s' not found in rig '%s'", mrID, rigName)
		}
		return fmt.Errorf("getting merge request: %w", err)
	}

	// Show what we're retrying
	fmt.Printf("Retrying merge request: %s\n", mrID)
	fmt.Printf("  Branch: %s\n", mr.Branch)
	fmt.Printf("  Worker: %s\n", mr.Worker)
	if mr.Error != "" {
		fmt.Printf("  Previous error: %s\n", style.Dim.Render(mr.Error))
	}

	// Perform the retry
	if err := mgr.Retry(mrID, mqRetryNow); err != nil {
		if err == refinery.ErrMRNotFailed {
			return fmt.Errorf("merge request '%s' has not failed (status: %s)", mrID, mr.Status)
		}
		return fmt.Errorf("retrying merge request: %w", err)
	}

	if mqRetryNow {
		fmt.Printf("%s Merge request processed\n", style.Bold.Render("✓"))
	} else {
		fmt.Printf("%s Merge request queued for retry\n", style.Bold.Render("✓"))
		fmt.Printf("  %s\n", style.Dim.Render("Will be processed on next refinery cycle"))
	}

	return nil
}

func runMQList(cmd *cobra.Command, args []string) error {
	rigName := args[0]

	_, r, err := getRefineryManager(rigName)
	if err != nil {
		return err
	}

	// Create beads wrapper for the rig
	b := beads.New(r.Path)

	// Build list options - query for merge-request type
	opts := beads.ListOptions{
		Type: "merge-request",
	}

	// Apply status filter if specified
	if mqListStatus != "" {
		opts.Status = mqListStatus
	} else if !mqListReady {
		// Default to open if not showing ready
		opts.Status = "open"
	}

	var issues []*beads.Issue

	if mqListReady {
		// Use ready query which filters by no blockers
		allReady, err := b.Ready()
		if err != nil {
			return fmt.Errorf("querying ready MRs: %w", err)
		}
		// Filter to only merge-request type
		for _, issue := range allReady {
			if issue.Type == "merge-request" {
				issues = append(issues, issue)
			}
		}
	} else {
		issues, err = b.List(opts)
		if err != nil {
			return fmt.Errorf("querying merge queue: %w", err)
		}
	}

	// Apply additional filters
	var filtered []*beads.Issue
	for _, issue := range issues {
		// Parse MR fields
		fields := beads.ParseMRFields(issue)

		// Filter by worker
		if mqListWorker != "" {
			worker := ""
			if fields != nil {
				worker = fields.Worker
			}
			if !strings.EqualFold(worker, mqListWorker) {
				continue
			}
		}

		// Filter by epic (target branch)
		if mqListEpic != "" {
			target := ""
			if fields != nil {
				target = fields.Target
			}
			expectedTarget := "integration/" + mqListEpic
			if target != expectedTarget {
				continue
			}
		}

		filtered = append(filtered, issue)
	}

	// JSON output
	if mqListJSON {
		return outputJSON(filtered)
	}

	// Human-readable output
	fmt.Printf("%s Merge queue for '%s':\n\n", style.Bold.Render("📋"), rigName)

	if len(filtered) == 0 {
		fmt.Printf("  %s\n", style.Dim.Render("(empty)"))
		return nil
	}

	// Print header
	fmt.Printf("  %-12s %-12s %-8s %-30s %-10s %s\n",
		"ID", "STATUS", "PRIORITY", "BRANCH", "WORKER", "AGE")
	fmt.Printf("  %s\n", strings.Repeat("-", 90))

	// Print each MR
	for _, issue := range filtered {
		fields := beads.ParseMRFields(issue)

		// Determine display status
		displayStatus := issue.Status
		if issue.Status == "open" {
			if len(issue.BlockedBy) > 0 || issue.BlockedByCount > 0 {
				displayStatus = "blocked"
			} else {
				displayStatus = "ready"
			}
		}

		// Format status with styling
		styledStatus := displayStatus
		switch displayStatus {
		case "ready":
			styledStatus = style.Bold.Render("ready")
		case "in_progress":
			styledStatus = style.Bold.Render("in_progress")
		case "blocked":
			styledStatus = style.Dim.Render("blocked")
		case "closed":
			styledStatus = style.Dim.Render("closed")
		}

		// Get MR fields
		branch := ""
		worker := ""
		if fields != nil {
			branch = fields.Branch
			worker = fields.Worker
		}

		// Truncate branch if too long
		if len(branch) > 30 {
			branch = branch[:27] + "..."
		}

		// Format priority
		priority := fmt.Sprintf("P%d", issue.Priority)

		// Calculate age
		age := formatMRAge(issue.CreatedAt)

		// Truncate ID if needed
		displayID := issue.ID
		if len(displayID) > 12 {
			displayID = displayID[:12]
		}

		fmt.Printf("  %-12s %-12s %-8s %-30s %-10s %s\n",
			displayID, styledStatus, priority, branch, worker, style.Dim.Render(age))

		// Show blocking info if blocked
		if displayStatus == "blocked" && len(issue.BlockedBy) > 0 {
			fmt.Printf("  %s\n", style.Dim.Render(fmt.Sprintf("             (waiting on %s)", issue.BlockedBy[0])))
		}
	}

	return nil
}

// formatMRAge formats the age of an MR from its created_at timestamp.
func formatMRAge(createdAt string) string {
	t, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		// Try other formats
		t, err = time.Parse("2006-01-02T15:04:05Z", createdAt)
		if err != nil {
			return "?"
		}
	}

	d := time.Since(t)

	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// outputJSON outputs data as JSON.
func outputJSON(data interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func runMQReject(cmd *cobra.Command, args []string) error {
	rigName := args[0]
	mrIDOrBranch := args[1]

	mgr, _, err := getRefineryManager(rigName)
	if err != nil {
		return err
	}

	result, err := mgr.RejectMR(mrIDOrBranch, mqRejectReason, mqRejectNotify)
	if err != nil {
		return fmt.Errorf("rejecting MR: %w", err)
	}

	fmt.Printf("%s Rejected: %s\n", style.Bold.Render("✗"), result.Branch)
	fmt.Printf("  Worker: %s\n", result.Worker)
	fmt.Printf("  Reason: %s\n", mqRejectReason)

	if result.IssueID != "" {
		fmt.Printf("  Issue:  %s %s\n", result.IssueID, style.Dim.Render("(not closed - work not done)"))
	}

	if mqRejectNotify {
		fmt.Printf("  %s\n", style.Dim.Render("Worker notified via mail"))
	}

	return nil
}

// MRStatusOutput is the JSON output structure for gt mq status.
type MRStatusOutput struct {
	// Core issue fields
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Priority  int    `json:"priority"`
	Type      string `json:"type"`
	Assignee  string `json:"assignee,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	ClosedAt  string `json:"closed_at,omitempty"`

	// MR-specific fields
	Branch      string `json:"branch,omitempty"`
	Target      string `json:"target,omitempty"`
	SourceIssue string `json:"source_issue,omitempty"`
	Worker      string `json:"worker,omitempty"`
	Rig         string `json:"rig,omitempty"`
	MergeCommit string `json:"merge_commit,omitempty"`
	CloseReason string `json:"close_reason,omitempty"`

	// Dependencies
	DependsOn []DependencyInfo `json:"depends_on,omitempty"`
	Blocks    []DependencyInfo `json:"blocks,omitempty"`
}

// DependencyInfo represents a dependency or blocker.
type DependencyInfo struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Priority int    `json:"priority"`
	Type     string `json:"type"`
}

func runMqStatus(cmd *cobra.Command, args []string) error {
	mrID := args[0]

	// Use current working directory for beads operations
	// (beads repos are per-rig, not per-workspace)
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	// Initialize beads client
	bd := beads.New(workDir)

	// Fetch the issue
	issue, err := bd.Show(mrID)
	if err != nil {
		if err == beads.ErrNotFound {
			return fmt.Errorf("merge request '%s' not found", mrID)
		}
		return fmt.Errorf("fetching merge request: %w", err)
	}

	// Parse MR-specific fields from description
	mrFields := beads.ParseMRFields(issue)

	// Build output structure
	output := MRStatusOutput{
		ID:        issue.ID,
		Title:     issue.Title,
		Status:    issue.Status,
		Priority:  issue.Priority,
		Type:      issue.Type,
		Assignee:  issue.Assignee,
		CreatedAt: issue.CreatedAt,
		UpdatedAt: issue.UpdatedAt,
		ClosedAt:  issue.ClosedAt,
	}

	// Add MR fields if present
	if mrFields != nil {
		output.Branch = mrFields.Branch
		output.Target = mrFields.Target
		output.SourceIssue = mrFields.SourceIssue
		output.Worker = mrFields.Worker
		output.Rig = mrFields.Rig
		output.MergeCommit = mrFields.MergeCommit
		output.CloseReason = mrFields.CloseReason
	}

	// Add dependency info from the issue's Dependencies field
	for _, dep := range issue.Dependencies {
		output.DependsOn = append(output.DependsOn, DependencyInfo{
			ID:       dep.ID,
			Title:    dep.Title,
			Status:   dep.Status,
			Priority: dep.Priority,
			Type:     dep.Type,
		})
	}

	// Add blocker info from the issue's Dependents field
	for _, dep := range issue.Dependents {
		output.Blocks = append(output.Blocks, DependencyInfo{
			ID:       dep.ID,
			Title:    dep.Title,
			Status:   dep.Status,
			Priority: dep.Priority,
			Type:     dep.Type,
		})
	}

	// JSON output
	if mqStatusJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	}

	// Human-readable output
	return printMqStatus(issue, mrFields)
}

// printMqStatus prints detailed MR status in human-readable format.
func printMqStatus(issue *beads.Issue, mrFields *beads.MRFields) error {
	// Header
	fmt.Printf("%s %s\n", style.Bold.Render("📋 Merge Request:"), issue.ID)
	fmt.Printf("   %s\n\n", issue.Title)

	// Status section
	fmt.Printf("%s\n", style.Bold.Render("Status"))
	statusDisplay := formatStatus(issue.Status)
	fmt.Printf("   State:    %s\n", statusDisplay)
	fmt.Printf("   Priority: P%d\n", issue.Priority)
	if issue.Type != "" {
		fmt.Printf("   Type:     %s\n", issue.Type)
	}
	if issue.Assignee != "" {
		fmt.Printf("   Assignee: %s\n", issue.Assignee)
	}

	// Timestamps
	fmt.Printf("\n%s\n", style.Bold.Render("Timeline"))
	if issue.CreatedAt != "" {
		fmt.Printf("   Created: %s %s\n", issue.CreatedAt, formatTimeAgo(issue.CreatedAt))
	}
	if issue.UpdatedAt != "" && issue.UpdatedAt != issue.CreatedAt {
		fmt.Printf("   Updated: %s %s\n", issue.UpdatedAt, formatTimeAgo(issue.UpdatedAt))
	}
	if issue.ClosedAt != "" {
		fmt.Printf("   Closed:  %s %s\n", issue.ClosedAt, formatTimeAgo(issue.ClosedAt))
	}

	// MR-specific fields
	if mrFields != nil {
		fmt.Printf("\n%s\n", style.Bold.Render("Merge Details"))
		if mrFields.Branch != "" {
			fmt.Printf("   Branch:       %s\n", mrFields.Branch)
		}
		if mrFields.Target != "" {
			fmt.Printf("   Target:       %s\n", mrFields.Target)
		}
		if mrFields.SourceIssue != "" {
			fmt.Printf("   Source Issue: %s\n", mrFields.SourceIssue)
		}
		if mrFields.Worker != "" {
			fmt.Printf("   Worker:       %s\n", mrFields.Worker)
		}
		if mrFields.Rig != "" {
			fmt.Printf("   Rig:          %s\n", mrFields.Rig)
		}
		if mrFields.MergeCommit != "" {
			fmt.Printf("   Merge Commit: %s\n", mrFields.MergeCommit)
		}
		if mrFields.CloseReason != "" {
			fmt.Printf("   Close Reason: %s\n", mrFields.CloseReason)
		}
	}

	// Dependencies (what this MR is waiting on)
	if len(issue.Dependencies) > 0 {
		fmt.Printf("\n%s\n", style.Bold.Render("Waiting On"))
		for _, dep := range issue.Dependencies {
			statusIcon := getStatusIcon(dep.Status)
			fmt.Printf("   %s %s: %s %s\n",
				statusIcon,
				dep.ID,
				truncateString(dep.Title, 50),
				style.Dim.Render(fmt.Sprintf("[%s]", dep.Status)))
		}
	}

	// Blockers (what's waiting on this MR)
	if len(issue.Dependents) > 0 {
		fmt.Printf("\n%s\n", style.Bold.Render("Blocking"))
		for _, dep := range issue.Dependents {
			statusIcon := getStatusIcon(dep.Status)
			fmt.Printf("   %s %s: %s %s\n",
				statusIcon,
				dep.ID,
				truncateString(dep.Title, 50),
				style.Dim.Render(fmt.Sprintf("[%s]", dep.Status)))
		}
	}

	// Description (if present and not just MR fields)
	desc := getDescriptionWithoutMRFields(issue.Description)
	if desc != "" {
		fmt.Printf("\n%s\n", style.Bold.Render("Notes"))
		// Indent each line
		for _, line := range strings.Split(desc, "\n") {
			fmt.Printf("   %s\n", line)
		}
	}

	return nil
}

// formatStatus formats the status with appropriate styling.
func formatStatus(status string) string {
	switch status {
	case "open":
		return style.Info.Render("● open")
	case "in_progress":
		return style.Bold.Render("▶ in_progress")
	case "closed":
		return style.Dim.Render("✓ closed")
	default:
		return status
	}
}

// getStatusIcon returns an icon for the given status.
func getStatusIcon(status string) string {
	switch status {
	case "open":
		return "○"
	case "in_progress":
		return "▶"
	case "closed":
		return "✓"
	default:
		return "•"
	}
}

// formatTimeAgo formats a timestamp as a relative time string.
func formatTimeAgo(timestamp string) string {
	// Try parsing common formats
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	var t time.Time
	var err error
	for _, format := range formats {
		t, err = time.Parse(format, timestamp)
		if err == nil {
			break
		}
	}
	if err != nil {
		return "" // Can't parse, return empty
	}

	d := time.Since(t)
	if d < 0 {
		return style.Dim.Render("(in the future)")
	}

	var ago string
	if d < time.Minute {
		ago = fmt.Sprintf("%ds ago", int(d.Seconds()))
	} else if d < time.Hour {
		ago = fmt.Sprintf("%dm ago", int(d.Minutes()))
	} else if d < 24*time.Hour {
		ago = fmt.Sprintf("%dh ago", int(d.Hours()))
	} else {
		ago = fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}

	return style.Dim.Render("(" + ago + ")")
}

// truncateString truncates a string to maxLen, adding "..." if truncated.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// getDescriptionWithoutMRFields returns the description with MR field lines removed.
func getDescriptionWithoutMRFields(description string) string {
	if description == "" {
		return ""
	}

	// Known MR field keys (lowercase)
	mrKeys := map[string]bool{
		"branch":       true,
		"target":       true,
		"source_issue": true,
		"source-issue": true,
		"sourceissue":  true,
		"worker":       true,
		"rig":          true,
		"merge_commit": true,
		"merge-commit": true,
		"mergecommit":  true,
		"close_reason": true,
		"close-reason": true,
		"closereason":  true,
		"type":         true,
	}

	var lines []string
	for _, line := range strings.Split(description, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			lines = append(lines, line)
			continue
		}

		// Check if this is an MR field line
		colonIdx := strings.Index(trimmed, ":")
		if colonIdx != -1 {
			key := strings.ToLower(strings.TrimSpace(trimmed[:colonIdx]))
			if mrKeys[key] {
				continue // Skip MR field lines
			}
		}

		lines = append(lines, line)
	}

	// Trim leading/trailing blank lines
	result := strings.Join(lines, "\n")
	result = strings.TrimSpace(result)
	return result
}

// runMqIntegrationCreate creates an integration branch for an epic.
func runMqIntegrationCreate(cmd *cobra.Command, args []string) error {
	epicID := args[0]

	// Find workspace
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return fmt.Errorf("not in a Gas Town workspace: %w", err)
	}

	// Find current rig
	_, r, err := findCurrentRig(townRoot)
	if err != nil {
		return err
	}

	// Initialize beads for the rig
	bd := beads.New(r.Path)

	// 1. Verify epic exists
	epic, err := bd.Show(epicID)
	if err != nil {
		if err == beads.ErrNotFound {
			return fmt.Errorf("epic '%s' not found", epicID)
		}
		return fmt.Errorf("fetching epic: %w", err)
	}

	// Verify it's actually an epic
	if epic.Type != "epic" {
		return fmt.Errorf("'%s' is a %s, not an epic", epicID, epic.Type)
	}

	// Build integration branch name
	branchName := "integration/" + epicID

	// Initialize git for the rig
	g := git.NewGit(r.Path)

	// Check if integration branch already exists locally
	exists, err := g.BranchExists(branchName)
	if err != nil {
		return fmt.Errorf("checking branch existence: %w", err)
	}
	if exists {
		return fmt.Errorf("integration branch '%s' already exists locally", branchName)
	}

	// Check if branch exists on remote
	remoteExists, err := g.RemoteBranchExists("origin", branchName)
	if err != nil {
		// Log warning but continue - remote check isn't critical
		fmt.Printf("  %s\n", style.Dim.Render("(could not check remote, continuing)"))
	}
	if remoteExists {
		return fmt.Errorf("integration branch '%s' already exists on origin", branchName)
	}

	// Ensure we have latest main
	fmt.Printf("Fetching latest from origin...\n")
	if err := g.Fetch("origin"); err != nil {
		return fmt.Errorf("fetching from origin: %w", err)
	}

	// 2. Create branch from origin/main
	fmt.Printf("Creating branch '%s' from main...\n", branchName)
	if err := g.CreateBranchFrom(branchName, "origin/main"); err != nil {
		return fmt.Errorf("creating branch: %w", err)
	}

	// 3. Push to origin
	fmt.Printf("Pushing to origin...\n")
	if err := g.Push("origin", branchName, false); err != nil {
		// Clean up local branch on push failure
		_ = g.DeleteBranch(branchName, true)
		return fmt.Errorf("pushing to origin: %w", err)
	}

	// 4. Store integration branch info in epic metadata
	// Update the epic's description to include the integration branch info
	newDesc := addIntegrationBranchField(epic.Description, branchName)
	if newDesc != epic.Description {
		if err := bd.Update(epicID, beads.UpdateOptions{Description: &newDesc}); err != nil {
			// Non-fatal - branch was created, just metadata update failed
			fmt.Printf("  %s\n", style.Dim.Render("(warning: could not update epic metadata)"))
		}
	}

	// Success output
	fmt.Printf("\n%s Created integration branch\n", style.Bold.Render("✓"))
	fmt.Printf("  Epic:   %s\n", epicID)
	fmt.Printf("  Branch: %s\n", branchName)
	fmt.Printf("  From:   main\n")
	fmt.Printf("\n  Future MRs for this epic's children can target:\n")
	fmt.Printf("    gt mq submit --epic %s\n", epicID)

	return nil
}

// addIntegrationBranchField adds or updates the integration_branch field in a description.
func addIntegrationBranchField(description, branchName string) string {
	fieldLine := "integration_branch: " + branchName

	// If description is empty, just return the field
	if description == "" {
		return fieldLine
	}

	// Check if integration_branch field already exists
	lines := strings.Split(description, "\n")
	var newLines []string
	found := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(trimmed), "integration_branch:") {
			// Replace existing field
			newLines = append(newLines, fieldLine)
			found = true
		} else {
			newLines = append(newLines, line)
		}
	}

	if !found {
		// Add field at the beginning
		newLines = append([]string{fieldLine}, newLines...)
	}

	return strings.Join(newLines, "\n")
}
