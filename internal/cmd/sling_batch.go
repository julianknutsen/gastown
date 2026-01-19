package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/events"
	"github.com/steveyegge/gastown/internal/style"
)

// batchParallelism controls how many polecats are spawned concurrently.
// Set to 0 for sequential execution (old behavior).
// Conservative for demos: 5. Increase after confirming API rate limits.
const batchParallelism = 5

// runBatchSling handles slinging multiple beads to a rig.
// Each bead gets its own freshly spawned polecat.
func runBatchSling(beadIDs []string, rigName string, townBeadsDir string) error {
	// Validate all beads exist before spawning any polecats
	for _, beadID := range beadIDs {
		if err := verifyBeadExists(beadID); err != nil {
			return fmt.Errorf("bead '%s' not found", beadID)
		}
	}

	if slingDryRun {
		fmt.Printf("%s Batch slinging %d beads to rig '%s':\n", style.Bold.Render("🎯"), len(beadIDs), rigName)
		for _, beadID := range beadIDs {
			fmt.Printf("  Would spawn polecat for: %s\n", beadID)
		}
		return nil
	}

	fmt.Printf("%s Batch slinging %d beads to rig '%s'...\n", style.Bold.Render("🎯"), len(beadIDs), rigName)

	// Track results for summary
	type slingResult struct {
		beadID  string
		polecat string
		success bool
		errMsg  string
	}

	// Get town root from beads dir (needed for agent ID resolution)
	townRoot := filepath.Dir(townBeadsDir)

	// Use parallel execution if batchParallelism > 0
	if batchParallelism > 0 {
		return runBatchSlingParallel(beadIDs, rigName, townRoot, townBeadsDir)
	}

	// Sequential fallback
	results := make([]slingResult, 0, len(beadIDs))

	// Spawn a polecat for each bead and sling it
	for i, beadID := range beadIDs {
		fmt.Printf("\n[%d/%d] Slinging %s...\n", i+1, len(beadIDs), beadID)

		// Check bead status
		info, err := getBeadInfo(beadID)
		if err != nil {
			results = append(results, slingResult{beadID: beadID, success: false, errMsg: err.Error()})
			fmt.Printf("  %s Could not get bead info: %v\n", style.Dim.Render("✗"), err)
			continue
		}

		if info.Status == "pinned" && !slingForce {
			results = append(results, slingResult{beadID: beadID, success: false, errMsg: "already pinned"})
			fmt.Printf("  %s Already pinned (use --force to re-sling)\n", style.Dim.Render("✗"))
			continue
		}

		// Spawn a fresh polecat
		spawnOpts := SlingSpawnOptions{
			Force:    slingForce,
			Account:  slingAccount,
			Create:   slingCreate,
			HookBead: beadID, // Set atomically at spawn time
			Agent:    slingAgent,
		}
		spawnInfo, err := SpawnPolecatForSling(rigName, spawnOpts)
		if err != nil {
			results = append(results, slingResult{beadID: beadID, success: false, errMsg: err.Error()})
			fmt.Printf("  %s Failed to spawn polecat: %v\n", style.Dim.Render("✗"), err)
			continue
		}

		targetAgent := spawnInfo.AgentID()
		hookWorkDir := spawnInfo.ClonePath

		// Auto-convoy: check if issue is already tracked
		if !slingNoConvoy {
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

		// Hook the bead. See: https://github.com/steveyegge/gastown/issues/148
		hookCmd := exec.Command("bd", "--no-daemon", "update", beadID, "--status=hooked", "--assignee="+targetAgent)
		hookCmd.Dir = beads.ResolveHookDir(townRoot, beadID, hookWorkDir)
		hookCmd.Stderr = os.Stderr
		if err := hookCmd.Run(); err != nil {
			results = append(results, slingResult{beadID: beadID, polecat: spawnInfo.PolecatName, success: false, errMsg: "hook failed"})
			fmt.Printf("  %s Failed to hook bead: %v\n", style.Dim.Render("✗"), err)
			continue
		}

		fmt.Printf("  %s Work attached to %s\n", style.Bold.Render("✓"), spawnInfo.PolecatName)

		// Log sling event
		actor := detectActor()
		_ = events.LogFeed(events.TypeSling, actor, events.SlingPayload(beadID, targetAgent))

		// Update agent bead state
		updateAgentHookBead(targetAgent, beadID, hookWorkDir, townBeadsDir)

		// Auto-attach mol-polecat-work molecule to polecat agent bead
		if err := attachPolecatWorkMolecule(targetAgent, hookWorkDir, townRoot); err != nil {
			fmt.Printf("  %s Could not attach work molecule: %v\n", style.Dim.Render("Warning:"), err)
		}

		// Store args if provided
		if slingArgs != "" {
			if err := storeArgsInBead(beadID, slingArgs); err != nil {
				fmt.Printf("  %s Could not store args: %v\n", style.Dim.Render("Warning:"), err)
			}
		}

		// Nudge the polecat using logical AgentID (translation to tmux happens in session layer)
		agentID, err := addressToAgentID(targetAgent)
		if err != nil {
			fmt.Printf("  %s Could not resolve agent address: %v\n", style.Dim.Render("○"), err)
		} else {
			// Ensure agent is ready before nudging
			if err := ensureAgentReady(townRoot, agentID); err != nil {
				fmt.Printf("  %s Could not verify agent ready: %v\n", style.Dim.Render("○"), err)
			}
			if err := injectStartPrompt(townRoot, agentID, beadID, slingSubject, slingArgs); err != nil {
				fmt.Printf("  %s Could not nudge (agent will discover via gt prime)\n", style.Dim.Render("○"))
			} else {
				fmt.Printf("  %s Start prompt sent\n", style.Bold.Render("▶"))
			}
		}

		results = append(results, slingResult{beadID: beadID, polecat: spawnInfo.PolecatName, success: true})
	}

	// Wake witness and refinery once at the end
	wakeRigAgents(townRoot, rigName)

	// Print summary
	successCount := 0
	for _, r := range results {
		if r.success {
			successCount++
		}
	}

	fmt.Printf("\n%s Batch sling complete: %d/%d succeeded\n", style.Bold.Render("📊"), successCount, len(beadIDs))
	if successCount < len(beadIDs) {
		for _, r := range results {
			if !r.success {
				fmt.Printf("  %s %s: %s\n", style.Dim.Render("✗"), r.beadID, r.errMsg)
			}
		}
	}

	return nil
}

// runBatchSlingParallel handles slinging multiple beads in parallel.
// Uses a worker pool with batchParallelism workers.
func runBatchSlingParallel(beadIDs []string, rigName, townRoot, townBeadsDir string) error {
	fmt.Printf("%s Parallel batch slinging %d beads (parallelism=%d)...\n",
		style.Bold.Render("🚀"), len(beadIDs), batchParallelism)

	type slingResult struct {
		index   int
		beadID  string
		polecat string
		success bool
		errMsg  string
	}

	// Channel for work items and results
	jobs := make(chan int, len(beadIDs))
	results := make(chan slingResult, len(beadIDs))

	// Start worker pool
	var wg sync.WaitGroup
	for w := 0; w < batchParallelism; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for idx := range jobs {
				beadID := beadIDs[idx]
				result := slingResult{index: idx, beadID: beadID}

				// Check bead status
				info, err := getBeadInfo(beadID)
				if err != nil {
					result.errMsg = err.Error()
					results <- result
					continue
				}

				if info.Status == "pinned" && !slingForce {
					result.errMsg = "already pinned"
					results <- result
					continue
				}

				// Spawn a fresh polecat
				spawnOpts := SlingSpawnOptions{
					Force:    slingForce,
					Account:  slingAccount,
					Create:   slingCreate,
					HookBead: beadID,
					Agent:    slingAgent,
				}
				spawnInfo, err := SpawnPolecatForSling(rigName, spawnOpts)
				if err != nil {
					result.errMsg = fmt.Sprintf("spawn failed: %v", err)
					results <- result
					continue
				}

				result.polecat = spawnInfo.PolecatName
				targetAgent := spawnInfo.AgentID()
				hookWorkDir := spawnInfo.ClonePath

				// Auto-convoy (skip for parallel - too noisy)
				// Hook the bead
				hookCmd := exec.Command("bd", "--no-daemon", "update", beadID, "--status=hooked", "--assignee="+targetAgent)
				hookCmd.Dir = beads.ResolveHookDir(townRoot, beadID, hookWorkDir)
				if err := hookCmd.Run(); err != nil {
					result.errMsg = "hook failed"
					results <- result
					continue
				}

				// Log sling event
				actor := detectActor()
				_ = events.LogFeed(events.TypeSling, actor, events.SlingPayload(beadID, targetAgent))

				// Update agent bead state
				updateAgentHookBead(targetAgent, beadID, hookWorkDir, townBeadsDir)

				// Auto-attach work molecule
				_ = attachPolecatWorkMolecule(targetAgent, hookWorkDir, townRoot)

				// Nudge the polecat
				agentID, err := addressToAgentID(targetAgent)
				if err == nil {
					_ = ensureAgentReady(townRoot, agentID)
					_ = injectStartPrompt(townRoot, agentID, beadID, slingSubject, slingArgs)
				}

				result.success = true
				results <- result
			}
		}(w)
	}

	// Send jobs to workers
	for i := range beadIDs {
		jobs <- i
	}
	close(jobs)

	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	allResults := make([]slingResult, len(beadIDs))
	successCount := 0
	for r := range results {
		allResults[r.index] = r
		if r.success {
			successCount++
			fmt.Printf("  %s [%d] %s → %s\n", style.Bold.Render("✓"), r.index+1, r.beadID, r.polecat)
		} else {
			fmt.Printf("  %s [%d] %s: %s\n", style.Dim.Render("✗"), r.index+1, r.beadID, r.errMsg)
		}
	}

	// Wake witness and refinery once at the end
	wakeRigAgents(townRoot, rigName)

	fmt.Printf("\n%s Batch sling complete: %d/%d succeeded\n", style.Bold.Render("📊"), successCount, len(beadIDs))

	return nil
}
