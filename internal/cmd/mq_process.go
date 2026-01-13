package cmd

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/refinery"
	"github.com/steveyegge/gastown/internal/style"
)

// MQ process command flags
var (
	mqProcessDryRun bool
	mqProcessJSON   bool
)

var mqProcessCmd = &cobra.Command{
	Use:   "process <rig> [mr-id]",
	Short: "Process the next merge request (or a specific one)",
	Long: `Process merge requests from the queue.

If no MR ID is provided, processes the highest-priority ready MR.
If an MR ID is provided, processes that specific MR.

This command performs the full merge workflow:
  1. Claims the MR (prevents concurrent processing)
  2. Fetches and checks out the source branch
  3. Merges into the target branch (typically main)
  4. Runs tests if configured
  5. Pushes the merged result
  6. Closes the MR bead and source issue
  7. Deletes the merged branch if configured

Use --dry-run to preview what would be processed without making changes.

Examples:
  gt mq process gastown                    # Process next ready MR
  gt mq process gastown gt-mr-abc123       # Process specific MR
  gt mq process gastown --dry-run          # Preview next MR without processing`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runMqProcess,
}

func init() {
	mqProcessCmd.Flags().BoolVar(&mqProcessDryRun, "dry-run", false, "Preview what would be processed without making changes")
	mqProcessCmd.Flags().BoolVar(&mqProcessJSON, "json", false, "Output result as JSON")

	mqCmd.AddCommand(mqProcessCmd)
}

func runMqProcess(cmd *cobra.Command, args []string) error {
	rigName := args[0]
	var mrID string
	if len(args) > 1 {
		mrID = args[1]
	}

	_, r, _, err := getRefineryManager(rigName)
	if err != nil {
		return err
	}

	// Create engineer for the rig
	eng := refinery.NewEngineer(r)
	if err := eng.LoadConfig(); err != nil {
		return fmt.Errorf("loading merge queue config: %w", err)
	}

	// Create beads wrapper for the rig
	b := beads.New(r.BeadsPath())

	var mr *refinery.MRInfo

	if mrID != "" {
		// Process specific MR
		mr, err = getMRByID(b, mrID)
		if err != nil {
			return err
		}
	} else {
		// Get the next ready MR
		mr, err = getNextReadyMR(b, r.DefaultBranch())
		if err != nil {
			return err
		}
	}

	if mr == nil {
		if mqProcessJSON {
			fmt.Println(`{"status": "empty", "message": "No ready merge requests in queue"}`)
		} else {
			fmt.Printf("%s No ready merge requests in queue\n", style.Dim.Render("ℹ"))
		}
		return nil
	}

	// Display what we're processing
	if !mqProcessJSON {
		fmt.Printf("%s Processing merge request:\n\n", style.Bold.Render("🔄"))
		fmt.Printf("  ID:       %s\n", mr.ID)
		fmt.Printf("  Branch:   %s\n", mr.Branch)
		fmt.Printf("  Target:   %s\n", mr.Target)
		fmt.Printf("  Worker:   %s\n", mr.Worker)
		fmt.Printf("  Priority: P%d\n", mr.Priority)
		if mr.SourceIssue != "" {
			fmt.Printf("  Issue:    %s\n", mr.SourceIssue)
		}
		fmt.Println()
	}

	if mqProcessDryRun {
		if mqProcessJSON {
			return outputJSON(map[string]interface{}{
				"status":  "dry_run",
				"mr_id":   mr.ID,
				"branch":  mr.Branch,
				"target":  mr.Target,
				"worker":  mr.Worker,
				"message": "Would process this MR (dry-run mode)",
			})
		}
		fmt.Printf("%s Dry run - would process %s\n", style.Dim.Render("ℹ"), mr.ID)
		return nil
	}

	// Claim the MR
	workerID := fmt.Sprintf("%s/refinery-cli", rigName)
	if err := eng.ClaimMR(mr.ID, workerID); err != nil {
		return fmt.Errorf("claiming MR: %w", err)
	}

	// Process the MR
	ctx := context.Background()
	result := eng.ProcessMRInfo(ctx, mr)

	// Handle result
	if result.Success {
		eng.HandleMRInfoSuccess(mr, result)

		if mqProcessJSON {
			return outputJSON(map[string]interface{}{
				"status":       "success",
				"mr_id":        mr.ID,
				"merge_commit": result.MergeCommit,
				"message":      fmt.Sprintf("Successfully merged %s", mr.Branch),
			})
		}

		fmt.Printf("\n%s Successfully merged!\n", style.Bold.Render("✓"))
		fmt.Printf("  Commit: %s\n", result.MergeCommit)
	} else {
		eng.HandleMRInfoFailure(mr, result)

		// Release the claim since we failed
		_ = eng.ReleaseMR(mr.ID)

		if mqProcessJSON {
			return outputJSON(map[string]interface{}{
				"status":       "failed",
				"mr_id":        mr.ID,
				"error":        result.Error,
				"conflict":     result.Conflict,
				"tests_failed": result.TestsFailed,
			})
		}

		errType := "Error"
		if result.Conflict {
			errType = "Conflict"
		} else if result.TestsFailed {
			errType = "Tests failed"
		}

		fmt.Printf("\n%s %s: %s\n", style.Bold.Render("✗"), errType, result.Error)
		return fmt.Errorf("merge failed: %s", result.Error)
	}

	return nil
}

// getMRByID fetches a specific MR by ID from beads.
func getMRByID(b *beads.Beads, mrID string) (*refinery.MRInfo, error) {
	issue, err := b.Show(mrID)
	if err != nil {
		return nil, fmt.Errorf("MR %s not found: %w", mrID, err)
	}

	fields := beads.ParseMRFields(issue)
	if fields == nil {
		return nil, fmt.Errorf("MR %s has no merge request fields", mrID)
	}

	// Parse times
	var convoyCreatedAt *time.Time
	if fields.ConvoyCreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, fields.ConvoyCreatedAt); err == nil {
			convoyCreatedAt = &t
		}
	}

	var createdAt time.Time
	if issue.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, issue.CreatedAt); err == nil {
			createdAt = t
		}
	}

	return &refinery.MRInfo{
		ID:              issue.ID,
		Branch:          fields.Branch,
		Target:          fields.Target,
		SourceIssue:     fields.SourceIssue,
		Worker:          fields.Worker,
		Rig:             fields.Rig,
		Title:           issue.Title,
		Priority:        issue.Priority,
		AgentBead:       fields.AgentBead,
		RetryCount:      fields.RetryCount,
		ConvoyID:        fields.ConvoyID,
		ConvoyCreatedAt: convoyCreatedAt,
		CreatedAt:       createdAt,
	}, nil
}

// getNextReadyMR finds the highest-priority ready MR from the queue.
func getNextReadyMR(b *beads.Beads, defaultBranch string) (*refinery.MRInfo, error) {
	// Query for open merge-requests
	opts := beads.ListOptions{
		Type:     "merge-request",
		Status:   "open",
		Priority: -1, // No priority filter
	}

	issues, err := b.List(opts)
	if err != nil {
		return nil, fmt.Errorf("querying merge queue: %w", err)
	}

	// Filter to ready MRs (no blockers, no assignee)
	var ready []*beads.Issue
	for _, issue := range issues {
		// Skip blocked issues
		if len(issue.BlockedBy) > 0 || issue.BlockedByCount > 0 {
			continue
		}
		// Skip already claimed issues
		if issue.Assignee != "" {
			continue
		}
		ready = append(ready, issue)
	}

	if len(ready) == 0 {
		return nil, nil
	}

	// Sort by priority score (highest first)
	now := time.Now()
	sort.Slice(ready, func(i, j int) bool {
		scoreI := calculateMRScore(ready[i], beads.ParseMRFields(ready[i]), now)
		scoreJ := calculateMRScore(ready[j], beads.ParseMRFields(ready[j]), now)
		return scoreI > scoreJ
	})

	// Convert the top issue to MRInfo
	issue := ready[0]
	fields := beads.ParseMRFields(issue)
	if fields == nil {
		return nil, fmt.Errorf("top MR %s has no merge request fields", issue.ID)
	}

	// Parse times
	var convoyCreatedAt *time.Time
	if fields.ConvoyCreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, fields.ConvoyCreatedAt); err == nil {
			convoyCreatedAt = &t
		}
	}

	var createdAt time.Time
	if issue.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, issue.CreatedAt); err == nil {
			createdAt = t
		}
	}

	// Default target to the rig's default branch if not specified
	target := fields.Target
	if target == "" {
		target = defaultBranch
	}

	return &refinery.MRInfo{
		ID:              issue.ID,
		Branch:          fields.Branch,
		Target:          target,
		SourceIssue:     fields.SourceIssue,
		Worker:          fields.Worker,
		Rig:             fields.Rig,
		Title:           issue.Title,
		Priority:        issue.Priority,
		AgentBead:       fields.AgentBead,
		RetryCount:      fields.RetryCount,
		ConvoyID:        fields.ConvoyID,
		ConvoyCreatedAt: convoyCreatedAt,
		CreatedAt:       createdAt,
	}, nil
}
