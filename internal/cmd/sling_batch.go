package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/events"
	"github.com/steveyegge/gastown/internal/style"
)

// parseBatchOnTarget parses the --on flag value for batch mode.
// Supports:
//   - Comma-separated: "gt-abc,gt-def,gt-ghi"
//   - File input: "@beads.txt" (one bead ID per line)
//   - Single bead: "gt-abc" (returns slice of 1)
func parseBatchOnTarget(target string) []string {
	// File input: @filename
	if strings.HasPrefix(target, "@") {
		filename := strings.TrimPrefix(target, "@")
		file, err := os.Open(filename)
		if err != nil {
			// Return as single-element slice (will fail validation later)
			return []string{target}
		}
		defer file.Close()

		var beadIDs []string
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			// Skip empty lines and comments
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			beadIDs = append(beadIDs, line)
		}
		return beadIDs
	}

	// Comma-separated or single bead
	parts := strings.Split(target, ",")
	var beadIDs []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			beadIDs = append(beadIDs, p)
		}
	}
	return beadIDs
}

// batchParallelism returns the configured parallelism for batch operations.
// Controlled by --parallel flag (default 5). Use 1 for sequential execution.
func batchParallelism() int {
	if slingParallel < 1 {
		return 1
	}
	return slingParallel
}

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
		if !slingNoConvoy {
			fmt.Printf("Would create batch convoy tracking %d beads\n", len(beadIDs))
		}
		return nil
	}

	fmt.Printf("%s Batch slinging %d beads to rig '%s'...\n", style.Bold.Render("🎯"), len(beadIDs), rigName)

	// Create batch convoy upfront (unless --no-convoy)
	if !slingNoConvoy {
		title := fmt.Sprintf("Batch: %d beads to %s", len(beadIDs), rigName)
		convoyID, err := createBatchConvoy(beadIDs, title)
		if err != nil {
			fmt.Printf("%s Could not create batch convoy: %v\n", style.Dim.Render("Warning:"), err)
		} else {
			fmt.Printf("%s Created batch convoy 🚚 %s\n", style.Bold.Render("→"), convoyID)
		}
	}

	// Get town root from beads dir (needed for agent ID resolution)
	townRoot := filepath.Dir(townBeadsDir)

	// Dispatch all beads (parallel handles both parallel and sequential via parallelism setting)
	return runBatchSlingParallel(beadIDs, rigName, townRoot, townBeadsDir)
}

// runBatchSlingParallel handles slinging multiple beads in parallel.
// Uses a worker pool with batchParallelism() workers.
func runBatchSlingParallel(beadIDs []string, rigName, townRoot, townBeadsDir string) error {
	parallelism := batchParallelism()
	fmt.Printf("%s Parallel batch slinging %d beads (parallelism=%d)...\n",
		style.Bold.Render("🚀"), len(beadIDs), parallelism)

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
	for w := 0; w < parallelism; w++ {
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

// runBatchSlingFormulaOn handles slinging a formula onto multiple beads.
// Each bead gets its own wisp instance bonded to it, then a fresh polecat.
// The formula is pre-cooked once and reused across all beads.
func runBatchSlingFormulaOn(formulaName string, beadIDs []string, rigName string, townBeadsDir string) error {
	// Validate all beads exist before cooking formula
	for _, beadID := range beadIDs {
		if err := verifyBeadExists(beadID); err != nil {
			return fmt.Errorf("bead '%s' not found", beadID)
		}
	}

	// Validate formula exists
	if err := verifyFormulaExists(formulaName); err != nil {
		return err
	}

	if slingDryRun {
		fmt.Printf("%s Batch slinging formula %s onto %d beads to rig '%s':\n",
			style.Bold.Render("🎯"), formulaName, len(beadIDs), rigName)
		for _, beadID := range beadIDs {
			fmt.Printf("  Would create wisp, bond to %s, spawn polecat\n", beadID)
		}
		if !slingNoConvoy {
			fmt.Printf("Would create batch convoy tracking %d beads\n", len(beadIDs))
		}
		return nil
	}

	fmt.Printf("%s Batch slinging formula %s onto %d beads to rig '%s'...\n",
		style.Bold.Render("🎯"), formulaName, len(beadIDs), rigName)

	// Create batch convoy upfront (unless --no-convoy)
	if !slingNoConvoy {
		title := fmt.Sprintf("Batch: %s on %d beads", formulaName, len(beadIDs))
		convoyID, err := createBatchConvoy(beadIDs, title)
		if err != nil {
			fmt.Printf("%s Could not create batch convoy: %v\n", style.Dim.Render("Warning:"), err)
		} else {
			fmt.Printf("%s Created batch convoy 🚚 %s\n", style.Bold.Render("→"), convoyID)
		}
	}

	// Get town root from beads dir
	townRoot := filepath.Dir(townBeadsDir)

	// Step 1: Pre-cook the formula once (shared across all beads)
	fmt.Printf("  Pre-cooking formula %s...\n", formulaName)
	cookCmd := exec.Command("bd", "--no-daemon", "cook", formulaName)
	cookCmd.Stderr = os.Stderr
	if err := cookCmd.Run(); err != nil {
		return fmt.Errorf("cooking formula %s: %w", formulaName, err)
	}
	fmt.Printf("%s Formula pre-cooked\n", style.Bold.Render("✓"))

	// Dispatch all beads (parallel handles both parallel and sequential via parallelism setting)
	return runBatchSlingFormulaOnParallel(formulaName, beadIDs, rigName, townRoot, townBeadsDir)
}

// runBatchSlingFormulaOnParallel handles formula-on-bead slinging in parallel.
// Uses a worker pool with batchParallelism() workers.
func runBatchSlingFormulaOnParallel(formulaName string, beadIDs []string, rigName, townRoot, townBeadsDir string) error {
	parallelism := batchParallelism()
	fmt.Printf("%s Parallel batch formula slinging %d beads (parallelism=%d)...\n",
		style.Bold.Render("🚀"), len(beadIDs), parallelism)

	type slingResult struct {
		index      int
		beadID     string
		compoundID string
		polecat    string
		success    bool
		errMsg     string
	}

	// Channels for work items and results
	jobs := make(chan int, len(beadIDs))
	results := make(chan slingResult, len(beadIDs))

	// Start worker pool
	var wg sync.WaitGroup
	for w := 0; w < parallelism; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for idx := range jobs {
				beadID := beadIDs[idx]
				result := slingResult{index: idx, beadID: beadID}

				// Get bead info for wisp variables
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

				// Route bd mutations to the correct beads context
				formulaWorkDir := beads.ResolveHookDir(townRoot, beadID, "")

				// Create wisp with feature and issue variables
				featureVar := fmt.Sprintf("feature=%s", info.Title)
				issueVar := fmt.Sprintf("issue=%s", beadID)
				wispArgs := []string{"--no-daemon", "mol", "wisp", formulaName, "--var", featureVar, "--var", issueVar, "--json"}
				wispCmd := exec.Command("bd", wispArgs...)
				wispCmd.Dir = formulaWorkDir
				wispCmd.Env = append(os.Environ(), "GT_ROOT="+townRoot)
				wispOut, err := wispCmd.Output()
				if err != nil {
					result.errMsg = fmt.Sprintf("wisp creation failed: %v", err)
					results <- result
					continue
				}

				wispRootID, err := parseWispIDFromJSON(wispOut)
				if err != nil {
					result.errMsg = fmt.Sprintf("wisp parse failed: %v", err)
					results <- result
					continue
				}

				// Bond wisp to original bead
				bondArgs := []string{"--no-daemon", "mol", "bond", wispRootID, beadID, "--json"}
				bondCmd := exec.Command("bd", bondArgs...)
				bondCmd.Dir = formulaWorkDir
				if err := bondCmd.Run(); err != nil {
					result.errMsg = fmt.Sprintf("bond failed: %v", err)
					results <- result
					continue
				}

				// Record attached molecule
				_ = storeAttachedMoleculeInBead(wispRootID, wispRootID)

				result.compoundID = wispRootID

				// Spawn polecat
				spawnOpts := SlingSpawnOptions{
					Force:    slingForce,
					Account:  slingAccount,
					Create:   slingCreate,
					HookBead: wispRootID,
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

				// Hook the compound bead
				hookCmd := exec.Command("bd", "--no-daemon", "update", wispRootID, "--status=hooked", "--assignee="+targetAgent)
				hookCmd.Dir = beads.ResolveHookDir(townRoot, wispRootID, hookWorkDir)
				if err := hookCmd.Run(); err != nil {
					result.errMsg = "hook failed"
					results <- result
					continue
				}

				// Log sling event
				actor := detectActor()
				_ = events.LogFeed(events.TypeSling, actor, events.SlingPayload(wispRootID, targetAgent))

				// Update agent bead state
				updateAgentHookBead(targetAgent, wispRootID, hookWorkDir, townBeadsDir)

				// Auto-attach work molecule
				_ = attachPolecatWorkMolecule(targetAgent, hookWorkDir, townRoot)

				// Nudge the polecat
				agentID, err := addressToAgentID(targetAgent)
				if err == nil {
					_ = ensureAgentReady(townRoot, agentID)
					_ = injectStartPrompt(townRoot, agentID, wispRootID, slingSubject, slingArgs)
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
			fmt.Printf("  %s [%d] %s → %s (compound: %s)\n",
				style.Bold.Render("✓"), r.index+1, r.beadID, r.polecat, r.compoundID)
		} else {
			fmt.Printf("  %s [%d] %s: %s\n", style.Dim.Render("✗"), r.index+1, r.beadID, r.errMsg)
		}
	}

	// Wake witness and refinery once at the end
	wakeRigAgents(townRoot, rigName)

	fmt.Printf("\n%s Batch formula sling complete: %d/%d succeeded\n", style.Bold.Render("📊"), successCount, len(beadIDs))

	return nil
}
