package cmd

import (
	"fmt"
	"path/filepath"

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
// spawns a polecat, hooks the bead, creates auto-convoy, stores args,
// and nudges the session.
//
// Returns the spawn info on success. Callers handle result tracking.
// Used by: runBatchSling() (after refactor), queue dispatch.
func slingBeadToPolecat(beadID, rigName string, opts SlingBeadOptions) (*SpawnedPolecatInfo, error) {
	townRoot := opts.TownRoot
	if townRoot == "" {
		var err error
		townRoot, err = findTownRoot()
		if err != nil {
			return nil, err
		}
	}

	// Check bead status
	info, err := getBeadInfo(beadID)
	if err != nil {
		return nil, fmt.Errorf("could not get bead info: %w", err)
	}

	if (info.Status == "pinned" || info.Status == "hooked") && !opts.Force {
		return nil, fmt.Errorf("already %s (use --force to re-sling)", info.Status)
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
			convoyID, err := createAutoConvoy(beadID, info.Title, false, "")
			if err != nil {
				fmt.Printf("  %s Could not create auto-convoy: %v\n", style.Dim.Render("Warning:"), err)
			} else {
				fmt.Printf("  %s Created convoy %s\n", style.Bold.Render("→"), convoyID)
			}
		} else {
			fmt.Printf("  %s Already tracked by convoy %s\n", style.Dim.Render("○"), existingConvoy)
		}
	}

	// Hook the bead with retry
	hookDir := beads.ResolveHookDir(townRoot, beadID, hookWorkDir)
	if err := hookBeadWithRetry(beadID, targetAgent, hookDir); err != nil {
		return spawnInfo, fmt.Errorf("failed to hook bead: %w", err)
	}

	fmt.Printf("  %s Work attached to %s\n", style.Bold.Render("✓"), spawnInfo.PolecatName)

	// Log sling event
	actor := detectActor()
	_ = events.LogFeed(events.TypeSling, actor, events.SlingPayload(beadID, targetAgent))

	// Update agent bead state
	beadsDir := opts.BeadsDir
	if beadsDir == "" {
		beadsDir = filepath.Join(townRoot, ".beads")
	}
	updateAgentHookBead(targetAgent, beadID, hookWorkDir, beadsDir)

	// Store fields in bead (dispatcher, args)
	fieldUpdates := beadFieldUpdates{
		Dispatcher: actor,
		Args:       opts.Args,
	}
	if err := storeFieldsInBead(beadID, fieldUpdates); err != nil {
		fmt.Printf("  %s Could not store fields in bead: %v\n", style.Dim.Render("Warning:"), err)
	}

	// Create Dolt branch AFTER all sling writes are complete
	if spawnInfo.DoltBranch != "" {
		if err := spawnInfo.CreateDoltBranch(); err != nil {
			fmt.Printf("  %s Could not create Dolt branch: %v\n", style.Dim.Render("Warning:"), err)
		}
	}

	// Start polecat session
	pane, err := spawnInfo.StartSession()
	if err != nil {
		fmt.Printf("  %s Could not start session: %v\n", style.Dim.Render("Warning:"), err)
	} else {
		_ = pane
		fmt.Printf("  %s Session started for %s\n", style.Bold.Render("▶"), spawnInfo.PolecatName)
	}

	return spawnInfo, nil
}

// findTownRoot is defined in hook.go
