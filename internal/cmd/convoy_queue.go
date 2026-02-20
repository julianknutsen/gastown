package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/workspace"
)

// convoyQueueOpts holds options for convoy queue operations.
type convoyQueueOpts struct {
	Formula     string
	HookRawBead bool
	Force       bool
	DryRun      bool
}

// runConvoyQueueByID queues all open tracked issues of a convoy.
// Called from `gt queue <convoy-id>`.
func runConvoyQueueByID(convoyID string, opts convoyQueueOpts) error {
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return err
	}

	// Validate convoy exists
	if err := verifyBeadExists(convoyID); err != nil {
		return fmt.Errorf("convoy '%s' not found", convoyID)
	}

	// Get tracked issues
	townBeads := filepath.Join(townRoot, ".beads")
	tracked, err := getTrackedIssues(townBeads, convoyID)
	if err != nil {
		return fmt.Errorf("getting tracked issues: %w", err)
	}

	if len(tracked) == 0 {
		fmt.Printf("Convoy %s has no tracked issues.\n", convoyID)
		return nil
	}

	// Filter to queueable issues
	type queueCandidate struct {
		ID      string
		Title   string
		RigName string
	}
	var candidates []queueCandidate
	skippedClosed := 0
	skippedAssigned := 0
	skippedQueued := 0
	skippedNoRig := 0

	for _, t := range tracked {
		// Skip closed issues
		if t.Status == "closed" || t.Status == "tombstone" {
			skippedClosed++
			continue
		}

		// Skip already assigned (hooked/in_progress) unless --force
		if t.Assignee != "" && !opts.Force {
			skippedAssigned++
			continue
		}

		// Check if already queued (need to get labels)
		info, err := getBeadInfo(t.ID)
		if err != nil {
			fmt.Printf("  %s Could not check %s: %v\n", style.Dim.Render("Warning:"), t.ID, err)
			continue
		}
		if hasQueuedLabel(info.Labels) {
			skippedQueued++
			continue
		}

		// Resolve rig from bead prefix
		rigName := resolveRigForBead(townRoot, t.ID)
		if rigName == "" {
			skippedNoRig++
			prefix := beads.ExtractPrefix(t.ID)
			fmt.Printf("  %s %s: cannot resolve rig from prefix %q (town-root or unknown)\n",
				style.Dim.Render("○"), t.ID, prefix)
			continue
		}

		candidates = append(candidates, queueCandidate{ID: t.ID, Title: t.Title, RigName: rigName})
	}

	if len(candidates) == 0 {
		fmt.Printf("No issues to queue from convoy %s", convoyID)
		if skippedClosed > 0 || skippedAssigned > 0 || skippedQueued > 0 || skippedNoRig > 0 {
			fmt.Printf(" (%d closed, %d assigned, %d already queued, %d no rig)",
				skippedClosed, skippedAssigned, skippedQueued, skippedNoRig)
		}
		fmt.Println()
		return nil
	}

	formula := opts.Formula

	if opts.DryRun {
		fmt.Printf("%s Would queue %d issue(s) from convoy %s:\n",
			style.Bold.Render("DRY-RUN"), len(candidates), convoyID)
		if formula != "" {
			fmt.Printf("  Formula: %s\n", formula)
		} else {
			fmt.Printf("  Hook raw beads (no formula)\n")
		}
		for _, c := range candidates {
			fmt.Printf("  Would queue: %s -> %s (%s)\n", c.ID, c.RigName, c.Title)
		}
		if skippedClosed > 0 || skippedAssigned > 0 || skippedQueued > 0 || skippedNoRig > 0 {
			fmt.Printf("\nSkipped: %d closed, %d assigned, %d already queued, %d no rig\n",
				skippedClosed, skippedAssigned, skippedQueued, skippedNoRig)
		}
		return nil
	}

	fmt.Printf("%s Queuing %d issue(s) from convoy %s...\n",
		style.Bold.Render("📋"), len(candidates), convoyID)

	successCount := 0
	for _, c := range candidates {
		err := enqueueBead(c.ID, c.RigName, EnqueueOptions{
			Formula:     formula,
			NoConvoy:    true, // Already tracked by this convoy
			Force:       opts.Force,
			HookRawBead: opts.HookRawBead,
		})
		if err != nil {
			fmt.Printf("  %s %s: %v\n", style.Dim.Render("✗"), c.ID, err)
			continue
		}
		successCount++
	}

	fmt.Printf("\n%s Queued %d/%d issue(s) from convoy %s\n",
		style.Bold.Render("📊"), successCount, len(candidates), convoyID)
	if skippedClosed > 0 || skippedAssigned > 0 || skippedQueued > 0 || skippedNoRig > 0 {
		fmt.Printf("  Skipped: %d closed, %d assigned, %d already queued, %d no rig\n",
			skippedClosed, skippedAssigned, skippedQueued, skippedNoRig)
	}

	if successCount == 0 {
		return fmt.Errorf("all %d enqueue attempts failed for convoy %s", len(candidates), convoyID)
	}
	return nil
}
