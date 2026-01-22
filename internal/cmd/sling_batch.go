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
	"github.com/steveyegge/gastown/internal/queue"
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
// Controlled by --spawn-batch-size flag (default 5). Use 1 for sequential execution.
func batchParallelism() int {
	if slingSpawnBatchSize < 1 {
		return 1
	}
	return slingSpawnBatchSize
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

	// --queue mode: add all beads to queue and dispatch
	if slingQueue {
		return runBatchSlingQueue(beadIDs, rigName, townBeadsDir)
	}

	if slingDryRun {
		fmt.Printf("%s Batch slinging %d beads to rig '%s':\n", style.Bold.Render("🎯"), len(beadIDs), rigName)
		fmt.Printf("  Spawn batch size: %d (concurrent spawns)\n", slingSpawnBatchSize)
		numBatches := (len(beadIDs) + slingSpawnBatchSize - 1) / slingSpawnBatchSize
		fmt.Printf("  Will run in %d batch(es)\n", numBatches)
		fmt.Printf("  Beads:\n")
		for _, beadID := range beadIDs {
			fmt.Printf("    - %s\n", beadID)
		}
		if !slingNoConvoy {
			fmt.Printf("Would create batch convoy tracking %d beads\n", len(beadIDs))
		}
		return nil
	}

	fmt.Printf("%s Batch slinging %d beads to rig '%s'...\n", style.Bold.Render("🎯"), len(beadIDs), rigName)

	// Create batch convoy upfront (unless --no-convoy)
	var convoyID string
	if !slingNoConvoy {
		title := fmt.Sprintf("Batch: %d beads to %s", len(beadIDs), rigName)
		var err error
		convoyID, err = createBatchConvoy(beadIDs, title)
		if err != nil {
			fmt.Printf("%s Could not create batch convoy: %v\n", style.Dim.Render("Warning:"), err)
		} else {
			fmt.Printf("%s Created batch convoy 🚚 %s\n", style.Bold.Render("→"), convoyID)
		}
	}

	// Get town root from beads dir (needed for agent ID resolution)
	townRoot := filepath.Dir(townBeadsDir)

	// Dispatch all beads using parallel workers
	err := runBatchSlingParallel(beadIDs, rigName, townRoot, townBeadsDir)

	// Print convoy ID at end for easy tracking
	if convoyID != "" {
		fmt.Printf("\n%s Track progress: bd show %s\n", style.Bold.Render("🚚"), convoyID)
	}

	return err
}

// runBatchSlingParallel handles slinging multiple beads in parallel.
// Uses a worker pool with batchParallelism() workers.
func runBatchSlingParallel(beadIDs []string, rigName, townRoot, townBeadsDir string) error {
	parallelism := batchParallelism()
	fmt.Printf("%s Parallel batch slinging %d beads (parallelism=%d)...\n",
		style.Bold.Render("🚀"), len(beadIDs), parallelism)

	// Pre-allocate polecat names to avoid race condition when spawning in parallel.
	queueItems := make([]queue.QueueItem, len(beadIDs))
	for i, beadID := range beadIDs {
		queueItems[i] = queue.QueueItem{BeadID: beadID, RigName: rigName}
	}
	preAllocatedNames, err := preAllocatePolecatNames(townRoot, queueItems)
	if err != nil {
		return fmt.Errorf("pre-allocating names: %w", err)
	}

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

	// Issue #288: Auto-apply mol-polecat-work for batch sling
	formulaName := "mol-polecat-work"
	formulaCooked := false

	// Start worker pool
	var wg sync.WaitGroup
	var cookMu sync.Mutex // Protects formulaCooked flag
	for w := 0; w < parallelism; w++ {
		wg.Add(1)
		go func() {
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

				// Get pre-allocated name
				polecatName, ok := preAllocatedNames[beadID]
				if !ok {
					result.errMsg = "no pre-allocated name"
					results <- result
					continue
				}

				// Spawn polecat with pre-allocated name
				spawnOpts := SlingSpawnOptions{
					Force:    slingForce,
					Account:  slingAccount,
					Create:   slingCreate,
					HookBead: beadID,
					Agent:    slingAgent,
				}
				spawnInfo, err := SpawnPolecatForSlingWithName(rigName, polecatName, spawnOpts)
				if err != nil {
					result.errMsg = fmt.Sprintf("spawn failed: %v", err)
					results <- result
					continue
				}

				result.polecat = spawnInfo.PolecatName
				targetAgent := spawnInfo.AgentID()
				hookWorkDir := spawnInfo.ClonePath

				// Auto-convoy: check if issue is already tracked
				if !slingNoConvoy {
					existingConvoy := isTrackedByConvoy(beadID)
					if existingConvoy == "" {
						_, _ = createAutoConvoy(beadID, info.Title)
					}
				}

				// Issue #288: Apply mol-polecat-work via formula-on-bead pattern
				// Cook once (lazy with mutex), then instantiate for each bead
				cookMu.Lock()
				if !formulaCooked {
					workDir := beads.ResolveHookDir(townRoot, beadID, hookWorkDir)
					if err := CookFormula(formulaName, workDir); err == nil {
						formulaCooked = true
					}
				}
				cookMu.Unlock()

				beadToHook := beadID
				if formulaCooked {
					formulaResult, err := InstantiateFormulaOnBead(formulaName, beadID, info.Title, hookWorkDir, townRoot, true)
					if err == nil {
						beadToHook = formulaResult.BeadToHook
						_ = storeAttachedMoleculeInBead(beadToHook, formulaResult.WispRootID)
					}
				}

				// Hook the bead (or wisp compound if formula was applied)
				hookCmd := exec.Command("bd", "--no-daemon", "update", beadToHook, "--status=hooked", "--assignee="+targetAgent)
				hookCmd.Dir = beads.ResolveHookDir(townRoot, beadToHook, hookWorkDir)
				hookCmd.Stderr = os.Stderr
				if err := hookCmd.Run(); err != nil {
					result.errMsg = "hook failed"
					results <- result
					continue
				}

				// Log sling event
				actor := detectActor()
				_ = events.LogFeed(events.TypeSling, actor, events.SlingPayload(beadToHook, targetAgent))

				// Update agent bead state
				updateAgentHookBead(targetAgent, beadToHook, hookWorkDir, townBeadsDir)

				// Store args if provided
				if slingArgs != "" {
					_ = storeArgsInBead(beadID, slingArgs)
				}

				// Nudge the polecat
				if spawnInfo.Pane != "" {
					_ = injectStartPrompt(spawnInfo.Pane, beadID, slingSubject, slingArgs)
				}

				result.success = true
				results <- result
			}
		}()
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
	wakeRigAgents(rigName)

	fmt.Printf("\n%s Batch sling complete: %d/%d succeeded\n", style.Bold.Render("📊"), successCount, len(beadIDs))

	return nil
}

// runBatchSlingQueue handles batch slinging using the queue workflow.
// All beads are added to the queue first, then dispatched.
func runBatchSlingQueue(beadIDs []string, rigName string, townBeadsDir string) error {
	townRoot := filepath.Dir(townBeadsDir)

	if slingDryRun {
		fmt.Printf("%s Batch queueing %d beads (--queue mode):\n", style.Bold.Render("🎯"), len(beadIDs))
		fmt.Printf("  Spawn batch size: %d (concurrent spawns)\n", slingSpawnBatchSize)

		// In queue mode, max polecats limits how many run at once
		spawnCount := len(beadIDs)
		queueCount := 0
		if slingQueueMaxPolecats > 0 {
			if slingQueueMaxPolecats < len(beadIDs) {
				spawnCount = slingQueueMaxPolecats
				queueCount = len(beadIDs) - slingQueueMaxPolecats
			}
			fmt.Printf("  Max polecats: %d\n", slingQueueMaxPolecats)
		} else {
			fmt.Printf("  Max polecats: unlimited\n")
		}

		if spawnCount > 0 {
			fmt.Printf("  Would spawn immediately (%d):\n", spawnCount)
			for i := 0; i < spawnCount && i < len(beadIDs); i++ {
				fmt.Printf("    - %s\n", beadIDs[i])
			}
		}
		if queueCount > 0 {
			fmt.Printf("  Would remain queued until slots free (%d):\n", queueCount)
			for i := spawnCount; i < len(beadIDs); i++ {
				fmt.Printf("    - %s\n", beadIDs[i])
			}
		}
		return nil
	}

	fmt.Printf("%s Batch queueing %d beads...\n", style.Bold.Render("🎯"), len(beadIDs))

	// Create queue with town-wide ops
	ops := beads.NewRealBeadsOps(townRoot)
	q := queue.New(ops)

	// Add all beads to queue
	for _, beadID := range beadIDs {
		if err := q.Add(beadID); err != nil {
			return fmt.Errorf("adding bead %s to queue: %w", beadID, err)
		}
		fmt.Printf("  %s Queued: %s\n", style.Bold.Render("✓"), beadID)
	}

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

	// Pre-allocate polecat names
	items, err := q.Load()
	if err != nil {
		return fmt.Errorf("loading queue: %w", err)
	}

	preAllocatedNames, err := preAllocatePolecatNames(townRoot, items)
	if err != nil {
		return fmt.Errorf("pre-allocating names: %w", err)
	}

	// Create spawner that uses pre-allocated names
	var mu sync.Mutex
	var successCount int
	spawner := &queue.RealSpawner{
		SpawnInFunc: func(spawnRigName, bid string) error {
			polecatName, ok := preAllocatedNames[bid]
			if !ok {
				return fmt.Errorf("no pre-allocated name for bead %s", bid)
			}

			spawnOpts := SlingSpawnOptions{
				Force:    slingForce,
				Account:  slingAccount,
				HookBead: bid,
				Agent:    slingAgent,
				Create:   true,
			}
			info, err := SpawnPolecatForSlingWithName(spawnRigName, polecatName, spawnOpts)
			if err != nil {
				return err
			}

			mu.Lock()
			successCount++
			fmt.Printf("  %s Dispatched: %s → %s\n", style.Bold.Render("✓"), bid, info.PolecatName)
			mu.Unlock()

			wakeRigAgents(spawnRigName)
			return nil
		},
	}

	// Calculate dispatch limit based on capacity
	limit := 0 // 0 means unlimited
	if slingQueueMaxPolecats > 0 {
		running := countRunningPolecats(townRoot)
		slots := slingQueueMaxPolecats - running
		if slots <= 0 {
			fmt.Printf("At capacity: %d polecats running (max=%d)\n", running, slingQueueMaxPolecats)
			return nil
		}
		limit = slots
		fmt.Printf("Capacity: %d/%d polecats running, %d slots available\n", running, slingQueueMaxPolecats, slots)
	}

	dispatcher := queue.NewDispatcher(q, spawner).
		WithParallelism(slingSpawnBatchSize).
		WithLimit(limit)

	fmt.Printf("%s Dispatching %d queued beads...\n", style.Bold.Render("🚀"), q.Len())
	if _, err := dispatcher.Dispatch(); err != nil {
		fmt.Printf("%s Some dispatches failed: %v\n", style.Dim.Render("Warning:"), err)
	}

	// Wake witness and refinery once at the end
	wakeRigAgents(rigName)

	fmt.Printf("\n%s Batch queue dispatch complete: %d/%d succeeded\n", style.Bold.Render("📊"), successCount, len(beadIDs))

	return nil
}
