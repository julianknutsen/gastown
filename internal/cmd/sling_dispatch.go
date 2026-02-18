package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/events"
	"github.com/steveyegge/gastown/internal/style"
)

// SlingBeadOptions holds parameters for dispatching a single bead to a polecat.
// Used by both batch sling and queue dispatch.
type SlingBeadOptions struct {
	Force    bool
	Account  string
	Create   bool
	Agent    string
	NoConvoy bool
	Args     string
	Subject  string
	TownRoot string
	BeadsDir string // townBeadsDir
}

// slingBeadToPolecat performs the complete sling for a single bead:
// spawns a polecat, hooks the bead, creates auto-convoy, attaches work
// molecule, stores args, and nudges the session.
//
// Returns the spawn info on success. Callers handle result tracking.
// Used by: runBatchSling(), queue dispatch.
func slingBeadToPolecat(beadID, rigName string, opts SlingBeadOptions) (*SpawnedPolecatInfo, error) {
	// Check bead status
	info, err := getBeadInfo(beadID)
	if err != nil {
		return nil, fmt.Errorf("could not get bead info: %w", err)
	}

	if info.Status == "pinned" && !opts.Force {
		return nil, fmt.Errorf("already pinned (use --force to re-sling)")
	}

	// Spawn a fresh polecat
	spawnOpts := SlingSpawnOptions{
		Force:    opts.Force,
		Account:  opts.Account,
		Create:   opts.Create,
		HookBead: beadID,
		Agent:    opts.Agent,
	}
	spawnInfo, err := SpawnPolecatForSling(rigName, spawnOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to spawn polecat: %w", err)
	}

	targetAgent := spawnInfo.AgentID()
	hookWorkDir := spawnInfo.ClonePath

	// Auto-convoy: check if issue is already tracked
	if !opts.NoConvoy {
		existingConvoy := isTrackedByConvoy(beadID)
		if existingConvoy == "" {
			convoyID, err := createAutoConvoy(beadID, info.Title)
			if err != nil {
				fmt.Printf("  %s Could not create auto-convoy: %v\n", style.Dim.Render("Warning:"), err)
			} else {
				fmt.Printf("  %s Created convoy 🚚 %s\n", style.Bold.Render("→"), convoyID)
			}
		} else {
			fmt.Printf("  %s Already tracked by convoy %s\n", style.Dim.Render("○"), existingConvoy)
		}
	}

	// Hook the bead
	hookCmd := exec.Command("bd", "--no-daemon", "update", beadID, "--status=hooked", "--assignee="+targetAgent)
	hookCmd.Dir = beads.ResolveHookDir(opts.TownRoot, beadID, hookWorkDir)
	hookCmd.Stderr = os.Stderr
	if err := hookCmd.Run(); err != nil {
		return spawnInfo, fmt.Errorf("failed to hook bead: %w", err)
	}

	fmt.Printf("  %s Work attached to %s\n", style.Bold.Render("✓"), spawnInfo.PolecatName)

	// Log sling event
	actor := detectActor()
	_ = events.LogFeed(events.TypeSling, actor, events.SlingPayload(beadID, targetAgent))

	// Update agent bead state
	updateAgentHookBead(targetAgent, beadID, hookWorkDir, opts.BeadsDir)

	// Auto-attach mol-polecat-work molecule to polecat agent bead
	if err := attachPolecatWorkMolecule(targetAgent, hookWorkDir, opts.TownRoot); err != nil {
		fmt.Printf("  %s Could not attach work molecule: %v\n", style.Dim.Render("Warning:"), err)
	}

	// Store args if provided
	if opts.Args != "" {
		if err := storeArgsInBead(beadID, opts.Args); err != nil {
			fmt.Printf("  %s Could not store args: %v\n", style.Dim.Render("Warning:"), err)
		}
	}

	// Nudge the polecat
	if spawnInfo.Pane != "" {
		if err := injectStartPrompt(spawnInfo.Pane, beadID, opts.Subject, opts.Args); err != nil {
			fmt.Printf("  %s Could not nudge (agent will discover via gt prime)\n", style.Dim.Render("○"))
		} else {
			fmt.Printf("  %s Start prompt sent\n", style.Bold.Render("▶"))
		}
	}

	return spawnInfo, nil
}
