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

// epicScheduleOpts holds options for epic schedule operations.
type epicScheduleOpts struct {
	Formula     string
	HookRawBead bool
	Force       bool
	DryRun      bool
}

// runEpicScheduleByID schedules all open children of an epic.
func runEpicScheduleByID(epicID string, opts epicScheduleOpts) error {
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return err
	}

	if err := verifyBeadExists(epicID); err != nil {
		return fmt.Errorf("epic '%s' not found", epicID)
	}

	children, err := getEpicChildren(epicID)
	if err != nil {
		return fmt.Errorf("listing children of %s: %w", epicID, err)
	}

	if len(children) == 0 {
		fmt.Printf("Epic %s has no child issues.\n", epicID)
		return nil
	}

	type scheduleCandidate struct {
		ID      string
		Title   string
		RigName string
	}
	var candidates []scheduleCandidate
	skippedClosed := 0
	skippedAssigned := 0
	skippedScheduled := 0
	skippedNoRig := 0

	for _, c := range children {
		if c.Status == "closed" || c.Status == "tombstone" {
			skippedClosed++
			continue
		}

		if c.Assignee != "" && !opts.Force {
			skippedAssigned++
			continue
		}

		if hasScheduledLabel(c.Labels) {
			skippedScheduled++
			continue
		}

		rigName := resolveRigForBead(townRoot, c.ID)
		if rigName == "" {
			skippedNoRig++
			prefix := beads.ExtractPrefix(c.ID)
			fmt.Printf("  %s %s: cannot resolve rig from prefix %q (town-root or unknown)\n",
				style.Dim.Render("○"), c.ID, prefix)
			continue
		}

		candidates = append(candidates, scheduleCandidate{ID: c.ID, Title: c.Title, RigName: rigName})
	}

	if len(candidates) == 0 {
		fmt.Printf("No children to schedule from epic %s", epicID)
		if skippedClosed > 0 || skippedAssigned > 0 || skippedScheduled > 0 || skippedNoRig > 0 {
			fmt.Printf(" (%d closed, %d assigned, %d already scheduled, %d no rig)",
				skippedClosed, skippedAssigned, skippedScheduled, skippedNoRig)
		}
		fmt.Println()
		return nil
	}

	formula := opts.Formula

	if opts.DryRun {
		fmt.Printf("%s Would schedule %d child(ren) from epic %s:\n",
			style.Bold.Render("DRY-RUN"), len(candidates), epicID)
		if formula != "" {
			fmt.Printf("  Formula: %s\n", formula)
		} else {
			fmt.Printf("  Hook raw beads (no formula)\n")
		}
		for _, c := range candidates {
			fmt.Printf("  Would schedule: %s -> %s (%s)\n", c.ID, c.RigName, c.Title)
		}
		if skippedClosed > 0 || skippedAssigned > 0 || skippedScheduled > 0 || skippedNoRig > 0 {
			fmt.Printf("\nSkipped: %d closed, %d assigned, %d already scheduled, %d no rig\n",
				skippedClosed, skippedAssigned, skippedScheduled, skippedNoRig)
		}
		return nil
	}

	fmt.Printf("%s Scheduling %d child(ren) from epic %s...\n",
		style.Bold.Render("📋"), len(candidates), epicID)

	successCount := 0
	for _, c := range candidates {
		err := scheduleBead(c.ID, c.RigName, ScheduleOptions{
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

	fmt.Printf("\n%s Scheduled %d/%d child(ren) from epic %s\n",
		style.Bold.Render("📊"), successCount, len(candidates), epicID)
	if skippedClosed > 0 || skippedAssigned > 0 || skippedScheduled > 0 || skippedNoRig > 0 {
		fmt.Printf("  Skipped: %d closed, %d assigned, %d already scheduled, %d no rig\n",
			skippedClosed, skippedAssigned, skippedScheduled, skippedNoRig)
	}

	if successCount == 0 {
		return fmt.Errorf("all %d schedule attempts failed for epic %s", len(candidates), epicID)
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
	depCmd := exec.Command("bd", "dep", "list", epicID,
		"--direction=down", "--type=depends_on", "--json")
	depCmd.Dir = resolveBeadDir(epicID)
	var stdout bytes.Buffer
	depCmd.Stdout = &stdout

	var stderr bytes.Buffer
	depCmd.Stderr = &stderr
	if err := depCmd.Run(); err != nil {
		if stdout.Len() == 0 && stderr.Len() == 0 {
			return nil, nil
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

	children := make([]epicChild, 0, len(deps))
	for _, dep := range deps {
		info, err := getBeadInfo(dep.ID)
		if err != nil {
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
