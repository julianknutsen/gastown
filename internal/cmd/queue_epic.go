package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/workspace"
)

// epicQueueOpts holds options for epic queue operations.
type epicQueueOpts struct {
	Formula     string
	HookRawBead bool
	Force       bool
	DryRun      bool
}

// runEpicQueueByID queues all open children of an epic.
// Called from `gt queue <epic-id>`.
func runEpicQueueByID(epicID string, opts epicQueueOpts) error {
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return err
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
		ID      string
		Title   string
		RigName string
	}
	var candidates []queueCandidate
	skippedClosed := 0
	skippedAssigned := 0
	skippedQueued := 0
	skippedNoRig := 0

	for _, c := range children {
		// Skip closed issues
		if c.Status == "closed" || c.Status == "tombstone" {
			skippedClosed++
			continue
		}

		// Skip already assigned unless --force
		if c.Assignee != "" && !opts.Force {
			skippedAssigned++
			continue
		}

		// Check if already queued
		if hasQueuedLabel(c.Labels) {
			skippedQueued++
			continue
		}

		// Resolve rig from bead prefix
		rigName := resolveRigForBead(townRoot, c.ID)
		if rigName == "" {
			skippedNoRig++
			prefix := beads.ExtractPrefix(c.ID)
			fmt.Printf("  %s %s: cannot resolve rig from prefix %q (town-root or unknown)\n",
				style.Dim.Render("○"), c.ID, prefix)
			continue
		}

		candidates = append(candidates, queueCandidate{ID: c.ID, Title: c.Title, RigName: rigName})
	}

	if len(candidates) == 0 {
		fmt.Printf("No children to queue from epic %s", epicID)
		if skippedClosed > 0 || skippedAssigned > 0 || skippedQueued > 0 || skippedNoRig > 0 {
			fmt.Printf(" (%d closed, %d assigned, %d already queued, %d no rig)",
				skippedClosed, skippedAssigned, skippedQueued, skippedNoRig)
		}
		fmt.Println()
		return nil
	}

	formula := opts.Formula

	if opts.DryRun {
		fmt.Printf("%s Would queue %d child(ren) from epic %s:\n",
			style.Bold.Render("DRY-RUN"), len(candidates), epicID)
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

	fmt.Printf("%s Queuing %d child(ren) from epic %s...\n",
		style.Bold.Render("📋"), len(candidates), epicID)

	successCount := 0
	for _, c := range candidates {
		err := enqueueBead(c.ID, c.RigName, EnqueueOptions{
			Formula:     formula,
			Force:       opts.Force,
			HookRawBead: opts.HookRawBead,
		})
		if err != nil {
			fmt.Printf("  %s %s: %v\n", style.Dim.Render("✗"), c.ID, err)
			continue
		}
		successCount++
	}

	fmt.Printf("\n%s Queued %d/%d child(ren) from epic %s\n",
		style.Bold.Render("📊"), successCount, len(candidates), epicID)
	if skippedClosed > 0 || skippedAssigned > 0 || skippedQueued > 0 || skippedNoRig > 0 {
		fmt.Printf("  Skipped: %d closed, %d assigned, %d already queued, %d no rig\n",
			skippedClosed, skippedAssigned, skippedQueued, skippedNoRig)
	}

	if successCount == 0 {
		return fmt.Errorf("all %d enqueue attempts failed for epic %s", len(candidates), epicID)
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

	var stderr bytes.Buffer
	depCmd.Stderr = &stderr
	if err := depCmd.Run(); err != nil {
		// bd dep list exits non-zero for both "no deps" and real errors.
		// Distinguish by checking if stdout is empty (no deps) vs stderr has content (real error).
		if stdout.Len() == 0 && stderr.Len() == 0 {
			return nil, nil // No dependencies
		}
		return nil, fmt.Errorf("bd dep list %s: %w (stderr: %s)", epicID, err, strings.TrimSpace(stderr.String()))
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
