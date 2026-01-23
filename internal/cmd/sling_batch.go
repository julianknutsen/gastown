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
	// Cook formula once before starting workers
	formulaName := "mol-polecat-work"
	if err := CookFormula(formulaName, townRoot); err != nil {
		fmt.Printf("%s Warning: could not cook %s: %v\n", style.Dim.Render("○"), formulaName, err)
	}

	// Start worker pool
	var wg sync.WaitGroup
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

				// Auto-convoy: check if issue is already tracked
				if !slingNoConvoy {
					existingConvoy := isTrackedByConvoy(beadID)
					if existingConvoy == "" {
						_, _ = createAutoConvoy(beadID, info.Title)
					}
				}

				// Apply formula to get beadToHook and wispID
				beadToHook := beadID
				formulaResult, fErr := InstantiateFormulaOnBead(formulaName, beadID, info.Title, "", townRoot, true)
				if fErr == nil {
					beadToHook = formulaResult.BeadToHook
					// Store attached_molecule in base bead
					_ = storeAttachedMoleculeInBead(beadToHook, formulaResult.WispRootID)
				}

				// Use unified spawn helper (WakeRig=false, done once at end of batch)
				spawnResult, err := PrepareAndSpawnPolecatWithName(PrepareAndSpawnParams{
					TownRoot: townRoot,
					RigName:  rigName,
					BeadID:   beadToHook,
					Force:    slingForce,
					Account:  slingAccount,
					Create:   slingCreate,
					Agent:    slingAgent,
					Subject:  slingSubject,
					Args:     slingArgs,
					WakeRig:  false, // Wake once at end of batch
				}, polecatName)
				if err != nil {
					result.errMsg = fmt.Sprintf("spawn failed: %v", err)
					results <- result
					continue
				}

				result.polecat = spawnResult.PolecatName
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
		fmt.Printf("  Would apply mol-polecat-work formula to each bead\n")
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

	// Issue #288: Apply mol-polecat-work formula to each bead before queueing
	// This ensures base beads have attached_molecule set when dispatched
	formulaName := "mol-polecat-work"
	formulaCooked := false

	fmt.Printf("  Applying %s formula to beads...\n", formulaName)

	var beadsToQueue []string
	for _, beadID := range beadIDs {
		// Get bead info for formula variables
		info, err := getBeadInfo(beadID)
		if err != nil {
			fmt.Printf("  %s %s: %v\n", style.Dim.Render("✗"), beadID, err)
			continue
		}

		// Cook formula once (lazy)
		if !formulaCooked {
			if err := CookFormula(formulaName, townRoot); err != nil {
				fmt.Printf("  %s Could not cook formula: %v (will hook raw beads)\n", style.Dim.Render("Warning:"), err)
			} else {
				formulaCooked = true
			}
		}

		if formulaCooked {
			// Apply formula - hooks base bead with attached_molecule pointing to wisp
			result, err := InstantiateFormulaOnBead(formulaName, beadID, info.Title, "", townRoot, true)
			if err != nil {
				fmt.Printf("  %s %s: formula failed: %v\n", style.Dim.Render("✗"), beadID, err)
				continue
			}

			// Store attached molecule in the BASE bead (lifecycle fix)
			_ = storeAttachedMoleculeInBead(result.BeadToHook, result.WispRootID)

			beadsToQueue = append(beadsToQueue, result.BeadToHook)
			fmt.Printf("  %s %s (attached: %s)\n", style.Bold.Render("✓"), beadID, result.WispRootID)
		} else {
			// Fallback: queue raw bead if formula failed
			beadsToQueue = append(beadsToQueue, beadID)
			fmt.Printf("  %s %s (raw)\n", style.Bold.Render("✓"), beadID)
		}
	}

	if len(beadsToQueue) == 0 {
		return fmt.Errorf("no beads prepared for queueing")
	}

	// Create queue with town-wide ops
	ops := beads.NewRealBeadsOps(townRoot)
	q := queue.New(ops)

	// Add all beads to queue
	for _, beadID := range beadsToQueue {
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

	// Create spawner that uses pre-allocated names and unified spawn workflow
	var mu sync.Mutex
	var successCount int
	spawner := &queue.RealSpawner{
		SpawnInFunc: func(spawnRigName, bid string) error {
			polecatName, ok := preAllocatedNames[bid]
			if !ok {
				return fmt.Errorf("no pre-allocated name for bead %s", bid)
			}

			// Use unified spawn helper (bead already prepared, WakeRig=false for batch)
			result, err := PrepareAndSpawnPolecatWithName(PrepareAndSpawnParams{
				TownRoot: townRoot,
				RigName:  spawnRigName,
				BeadID:   bid, // Already prepared with formula
				Force:    slingForce,
				Account:  slingAccount,
				Create:   true,
				Agent:    slingAgent,
				Subject:  slingSubject,
				Args:     slingArgs,
				WakeRig:  false, // Wake once at end of batch
			}, polecatName)
			if err != nil {
				return err
			}

			mu.Lock()
			successCount++
			fmt.Printf("  %s Dispatched: %s → %s\n", style.Bold.Render("✓"), bid, result.PolecatName)
			mu.Unlock()

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
		if slingQueue {
			// Queue mode dry-run
			fmt.Printf("%s Batch slinging formula %s onto %d beads to rig '%s' (--queue mode):\n",
				style.Bold.Render("🎯"), formulaName, len(beadIDs), rigName)
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
					fmt.Printf("    - %s: create wisp, bond, spawn polecat\n", beadIDs[i])
				}
			}
			if queueCount > 0 {
				fmt.Printf("  Would remain queued until slots free (%d):\n", queueCount)
				for i := spawnCount; i < len(beadIDs); i++ {
					fmt.Printf("    - %s: create wisp, bond, queue\n", beadIDs[i])
				}
			}
		} else {
			// Non-queue mode dry-run
			fmt.Printf("%s Batch slinging formula %s onto %d beads to rig '%s':\n",
				style.Bold.Render("🎯"), formulaName, len(beadIDs), rigName)
			fmt.Printf("  Spawn batch size: %d (concurrent spawns)\n", slingSpawnBatchSize)
			numBatches := (len(beadIDs) + slingSpawnBatchSize - 1) / slingSpawnBatchSize
			fmt.Printf("  Will run in %d batch(es)\n", numBatches)
			fmt.Printf("  Beads:\n")
			for _, beadID := range beadIDs {
				fmt.Printf("    - %s: create wisp, bond, spawn polecat\n", beadID)
			}
		}
		if !slingNoConvoy {
			fmt.Printf("Would create batch convoy tracking %d beads\n", len(beadIDs))
		}
		return nil
	}

	fmt.Printf("%s Batch slinging formula %s onto %d beads to rig '%s'...\n",
		style.Bold.Render("🎯"), formulaName, len(beadIDs), rigName)

	// Create batch convoy upfront (unless --no-convoy)
	var convoyID string
	if !slingNoConvoy {
		title := fmt.Sprintf("Batch: %s on %d beads", formulaName, len(beadIDs))
		var err error
		convoyID, err = createBatchConvoy(beadIDs, title)
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

	// --queue mode: create wisps+bonds, queue compound beads, then dispatch
	if slingQueue {
		return runBatchSlingFormulaOnQueue(formulaName, beadIDs, rigName, townRoot)
	}

	// Dispatch all beads (parallel handles both parallel and sequential via parallelism setting)
	err := runBatchSlingFormulaOnParallel(formulaName, beadIDs, rigName, townRoot, townBeadsDir)

	// Print convoy ID at end for easy tracking
	if convoyID != "" {
		fmt.Printf("\n%s Track progress: bd show %s\n", style.Bold.Render("🚚"), convoyID)
	}

	return err
}

// runBatchSlingFormulaOnParallel handles formula-on-bead slinging in parallel.
// Uses a worker pool with batchParallelism() workers.
func runBatchSlingFormulaOnParallel(formulaName string, beadIDs []string, rigName, townRoot, townBeadsDir string) error {
	parallelism := batchParallelism()
	fmt.Printf("%s Parallel batch formula slinging %d beads (parallelism=%d)...\n",
		style.Bold.Render("🚀"), len(beadIDs), parallelism)

	// Pre-allocate polecat names to avoid race condition when spawning in parallel.
	// We use the original bead IDs as keys since we need one name per bead.
	queueItems := make([]queue.QueueItem, len(beadIDs))
	for i, beadID := range beadIDs {
		queueItems[i] = queue.QueueItem{BeadID: beadID, RigName: rigName}
	}
	preAllocatedNames, err := preAllocatePolecatNames(townRoot, queueItems)
	if err != nil {
		return fmt.Errorf("pre-allocating names: %w", err)
	}

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
		go func() {
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

				// Get pre-allocated name
				polecatName, ok := preAllocatedNames[beadID]
				if !ok {
					result.errMsg = "no pre-allocated name"
					results <- result
					continue
				}

				// Apply formula to bead using InstantiateFormulaOnBead
				// This returns BeadToHook (base bead) and WispRootID (attached molecule)
				formulaResult, err := InstantiateFormulaOnBead(formulaName, beadID, info.Title, "", townRoot, true)
				if err != nil {
					result.errMsg = fmt.Sprintf("formula failed: %v", err)
					results <- result
					continue
				}

				beadToHook := formulaResult.BeadToHook
				result.compoundID = formulaResult.WispRootID

				// Store attached molecule in base bead
				_ = storeAttachedMoleculeInBead(beadToHook, formulaResult.WispRootID)

				// Use unified spawn helper (WakeRig=false, done once at end of batch)
				spawnResult, err := PrepareAndSpawnPolecatWithName(PrepareAndSpawnParams{
					TownRoot: townRoot,
					RigName:  rigName,
					BeadID:   beadToHook,
					Force:    slingForce,
					Account:  slingAccount,
					Create:   slingCreate,
					Agent:    slingAgent,
					Subject:  slingSubject,
					Args:     slingArgs,
					WakeRig:  false, // Wake once at end of batch
				}, polecatName)
				if err != nil {
					result.errMsg = fmt.Sprintf("spawn failed: %v", err)
					results <- result
					continue
				}

				result.polecat = spawnResult.PolecatName
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
			fmt.Printf("  %s [%d] %s → %s (compound: %s)\n",
				style.Bold.Render("✓"), r.index+1, r.beadID, r.polecat, r.compoundID)
		} else {
			fmt.Printf("  %s [%d] %s: %s\n", style.Dim.Render("✗"), r.index+1, r.beadID, r.errMsg)
		}
	}

	// Wake witness and refinery once at the end
	wakeRigAgents(rigName)

	fmt.Printf("\n%s Batch formula sling complete: %d/%d succeeded\n", style.Bold.Render("📊"), successCount, len(beadIDs))

	return nil
}

// runBatchSlingFormulaOnQueue handles formula-on-bead slinging using the queue workflow.
// Creates wisps+bonds for all beads, queues the compound beads, then dispatches.
func runBatchSlingFormulaOnQueue(formulaName string, beadIDs []string, rigName, townRoot string) error {
	fmt.Printf("%s Creating wisp compounds for %d beads...\n", style.Bold.Render("🔧"), len(beadIDs))

	// Step 1: Create wisps and bonds for all beads, storing attached_molecule in base beads
	var baseBeadIDs []string
	for _, beadID := range beadIDs {
		// Get bead info for wisp variables
		info, err := getBeadInfo(beadID)
		if err != nil {
			fmt.Printf("  %s %s: %v\n", style.Dim.Render("✗"), beadID, err)
			continue
		}

		// Apply formula to bead using InstantiateFormulaOnBead
		// This returns BeadToHook (base bead) and WispRootID (attached molecule)
		formulaResult, err := InstantiateFormulaOnBead(formulaName, beadID, info.Title, "", townRoot, true)
		if err != nil {
			fmt.Printf("  %s %s: formula failed: %v\n", style.Dim.Render("✗"), beadID, err)
			continue
		}

		// Store attached molecule in the BASE bead (lifecycle fix)
		_ = storeAttachedMoleculeInBead(formulaResult.BeadToHook, formulaResult.WispRootID)

		baseBeadIDs = append(baseBeadIDs, formulaResult.BeadToHook)
		fmt.Printf("  %s %s (attached: %s)\n", style.Bold.Render("✓"), beadID, formulaResult.WispRootID)
	}

	if len(baseBeadIDs) == 0 {
		return fmt.Errorf("no beads prepared")
	}

	fmt.Printf("%s Queueing %d base beads...\n", style.Bold.Render("📋"), len(baseBeadIDs))

	// Step 2: Queue all BASE beads (not compounds - lifecycle fix)
	ops := beads.NewRealBeadsOps(townRoot)
	q := queue.New(ops)

	for _, baseBeadID := range baseBeadIDs {
		if err := q.Add(baseBeadID); err != nil {
			fmt.Printf("  %s %s: queue add failed: %v\n", style.Dim.Render("✗"), baseBeadID, err)
			continue
		}
		fmt.Printf("  %s Queued: %s\n", style.Bold.Render("✓"), baseBeadID)
	}

	// Step 3: Pre-allocate polecat names
	items, err := q.Load()
	if err != nil {
		return fmt.Errorf("loading queue: %w", err)
	}

	preAllocatedNames, err := preAllocatePolecatNames(townRoot, items)
	if err != nil {
		return fmt.Errorf("pre-allocating names: %w", err)
	}

	// Step 4: Create spawner that uses pre-allocated names and unified spawn workflow
	var mu sync.Mutex
	var successCount int
	spawner := &queue.RealSpawner{
		SpawnInFunc: func(spawnRigName, bid string) error {
			polecatName, ok := preAllocatedNames[bid]
			if !ok {
				return fmt.Errorf("no pre-allocated name for bead %s", bid)
			}

			// Use unified spawn helper (bead already prepared, WakeRig=false for batch)
			result, err := PrepareAndSpawnPolecatWithName(PrepareAndSpawnParams{
				TownRoot: townRoot,
				RigName:  spawnRigName,
				BeadID:   bid, // Already prepared with formula
				Force:    slingForce,
				Account:  slingAccount,
				Create:   true,
				Agent:    slingAgent,
				Subject:  slingSubject,
				Args:     slingArgs,
				WakeRig:  false, // Wake once at end of batch
			}, polecatName)
			if err != nil {
				return err
			}

			mu.Lock()
			successCount++
			fmt.Printf("  %s Dispatched: %s → %s\n", style.Bold.Render("✓"), bid, result.PolecatName)
			mu.Unlock()

			return nil
		},
	}

	// Step 5: Calculate dispatch limit based on capacity
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

	fmt.Printf("\n%s Batch formula queue dispatch complete: %d/%d succeeded\n", style.Bold.Render("📊"), successCount, len(baseBeadIDs))

	return nil
}
