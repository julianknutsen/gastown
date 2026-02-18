package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/workspace"
)

var (
	queueEpicDryRun bool
	queueEpicForce  bool
)

var queueEpicCmd = &cobra.Command{
	Use:   "epic <epic-id> <rig>",
	Short: "Queue all open children of an epic for deferred dispatch",
	Long: `Queue all open child issues of an epic for capacity-controlled dispatch.

ALL open children are queued, including blocked ones. Blocked beads wait
in the queue and automatically dispatch when their blockers resolve.

Children that are already queued, closed, or assigned are skipped.

Examples:
  gt queue epic gt-epic-123 gastown           # Queue all open children
  gt queue epic gt-epic-123 gastown --dry-run # Preview what would be queued`,
	Args: cobra.ExactArgs(2),
	RunE: runQueueEpic,
}

func init() {
	queueEpicCmd.Flags().BoolVar(&queueEpicDryRun, "dry-run", false, "Show what would be queued without acting")
	queueEpicCmd.Flags().BoolVar(&queueEpicForce, "force", false, "Force enqueue even if bead is hooked/in_progress")

	queueCmd.AddCommand(queueEpicCmd)
}

func runQueueEpic(cmd *cobra.Command, args []string) error {
	epicID := args[0]
	rigName := args[1]

	_, err := workspace.FindFromCwdOrError()
	if err != nil {
		return err
	}

	// Validate rig exists
	if _, isRig := IsRigName(rigName); !isRig {
		return fmt.Errorf("'%s' is not a known rig", rigName)
	}

	// Validate epic exists
	if err := verifyBeadExists(epicID); err != nil {
		return fmt.Errorf("epic '%s' not found", epicID)
	}

	// Get children via dependency query
	children, err := getEpicChildren(epicID)
	if err != nil {
		return fmt.Errorf("listing children of %s: %w", epicID, err)
	}

	if len(children) == 0 {
		fmt.Printf("Epic %s has no child issues.\n", epicID)
		return nil
	}

	// Filter to queueable children
	type queueCandidate struct {
		ID    string
		Title string
	}
	var candidates []queueCandidate
	skippedClosed := 0
	skippedAssigned := 0
	skippedQueued := 0

	for _, c := range children {
		// Skip closed issues
		if c.Status == "closed" || c.Status == "tombstone" {
			skippedClosed++
			continue
		}

		// Skip already assigned unless --force
		if c.Assignee != "" && !queueEpicForce {
			skippedAssigned++
			continue
		}

		// Check if already queued
		if hasQueuedLabel(c.Labels) {
			skippedQueued++
			continue
		}

		candidates = append(candidates, queueCandidate{ID: c.ID, Title: c.Title})
	}

	if len(candidates) == 0 {
		fmt.Printf("No children to queue from epic %s", epicID)
		if skippedClosed > 0 || skippedAssigned > 0 || skippedQueued > 0 {
			fmt.Printf(" (%d closed, %d assigned, %d already queued)", skippedClosed, skippedAssigned, skippedQueued)
		}
		fmt.Println()
		return nil
	}

	if queueEpicDryRun {
		fmt.Printf("%s Would queue %d child(ren) from epic %s → %s:\n",
			style.Bold.Render("📋"), len(candidates), epicID, rigName)
		for _, c := range candidates {
			fmt.Printf("  Would queue: %s (%s)\n", c.ID, c.Title)
		}
		if skippedClosed > 0 || skippedAssigned > 0 || skippedQueued > 0 {
			fmt.Printf("\nSkipped: %d closed, %d assigned, %d already queued\n",
				skippedClosed, skippedAssigned, skippedQueued)
		}
		return nil
	}

	fmt.Printf("%s Queuing %d child(ren) from epic %s → %s...\n",
		style.Bold.Render("📋"), len(candidates), epicID, rigName)

	successCount := 0
	for _, c := range candidates {
		err := enqueueBead(c.ID, rigName, EnqueueOptions{
			Force: queueEpicForce,
		})
		if err != nil {
			fmt.Printf("  %s %s: %v\n", style.Dim.Render("✗"), c.ID, err)
			continue
		}
		successCount++
	}

	fmt.Printf("\n%s Queued %d/%d child(ren) from epic %s\n",
		style.Bold.Render("📊"), successCount, len(candidates), epicID)
	if skippedClosed > 0 || skippedAssigned > 0 || skippedQueued > 0 {
		fmt.Printf("  Skipped: %d closed, %d assigned, %d already queued\n",
			skippedClosed, skippedAssigned, skippedQueued)
	}

	return nil
}

// epicChild holds info about a child issue of an epic.
type epicChild struct {
	ID       string
	Title    string
	Status   string
	Assignee string
	Labels   []string
}

// getEpicChildren returns child issues of an epic via dependency lookup.
func getEpicChildren(epicID string) ([]epicChild, error) {
	// Query children: issues that depend on the epic
	depCmd := exec.Command("bd", "dep", "list", epicID,
		"--direction=down", "--type=depends_on", "--json")
	depCmd.Dir = resolveBeadDir(epicID)
	var stdout bytes.Buffer
	depCmd.Stdout = &stdout

	if err := depCmd.Run(); err != nil {
		// No dependencies is not an error
		return nil, nil
	}

	var deps []struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &deps); err != nil {
		return nil, fmt.Errorf("parsing dependency list: %w", err)
	}

	// Get full details for each child (need labels and assignee)
	children := make([]epicChild, 0, len(deps))
	for _, dep := range deps {
		info, err := getBeadInfo(dep.ID)
		if err != nil {
			// Skip beads we can't look up (cross-rig, deleted, etc.)
			children = append(children, epicChild{
				ID:     dep.ID,
				Title:  dep.Title,
				Status: dep.Status,
			})
			continue
		}
		children = append(children, epicChild{
			ID:       dep.ID,
			Title:    info.Title,
			Status:   info.Status,
			Assignee: info.Assignee,
			Labels:   info.Labels,
		})
	}

	return children, nil
}
