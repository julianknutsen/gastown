package cmd

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/steveyegge/gastown/internal/config"
	"github.com/steveyegge/gastown/internal/events"
	"github.com/steveyegge/gastown/internal/style"
)

// dispatchQueuedWork is the main dispatch loop for the work queue.
// Called by both `gt queue run` and the daemon heartbeat (via `gt queue run`).
//
// It checks capacity, queries ready beads, and dispatches up to batchSize beads.
// Returns the number of beads dispatched and any error.
func dispatchQueuedWork(townRoot string, batchOverride, maxPolOverride int, dryRun bool) (int, error) {
	// Load queue state
	queueState, err := LoadQueueState(townRoot)
	if err != nil {
		return 0, fmt.Errorf("loading queue state: %w", err)
	}

	if queueState.Paused {
		if !dryRun {
			fmt.Printf("%s Queue is paused (by %s), skipping dispatch\n", style.Dim.Render("⏸"), queueState.PausedBy)
		}
		return 0, nil
	}

	// Load town settings for queue config
	settingsPath := config.TownSettingsPath(townRoot)
	settings, err := config.LoadOrCreateTownSettings(settingsPath)
	if err != nil {
		return 0, fmt.Errorf("loading town settings: %w", err)
	}

	queueCfg := settings.Queue
	if queueCfg == nil {
		queueCfg = config.DefaultWorkQueueConfig()
	}

	if !queueCfg.Enabled && !dryRun {
		fmt.Printf("%s Queue dispatch is not enabled in town settings\n", style.Dim.Render("○"))
		fmt.Println("  Enable with: gt config set queue.enabled true")
		return 0, nil
	}

	// Determine limits
	maxPolecats := queueCfg.GetMaxPolecats()
	if maxPolOverride > 0 {
		maxPolecats = maxPolOverride
	}
	batchSize := queueCfg.GetBatchSize()
	if batchOverride > 0 {
		batchSize = batchOverride
	}
	spawnDelay := queueCfg.GetSpawnDelay()

	// Count active polecats
	activePolecats := countActivePolecats()

	// Compute available capacity
	capacity := maxPolecats - activePolecats
	if capacity <= 0 {
		if dryRun {
			fmt.Printf("No capacity: %d/%d polecats active\n", activePolecats, maxPolecats)
		}
		return 0, nil
	}

	// Query ready queued beads (unblocked + has gt:queued label)
	readyBeads, err := getReadyQueuedBeads(townRoot)
	if err != nil {
		return 0, fmt.Errorf("querying ready beads: %w", err)
	}

	if len(readyBeads) == 0 {
		if dryRun {
			fmt.Println("No ready beads in queue")
		}
		return 0, nil
	}

	// Dispatch up to the smallest of capacity, batchSize, and readyBeads count
	toDispatch := capacity
	if batchSize < toDispatch {
		toDispatch = batchSize
	}
	if len(readyBeads) < toDispatch {
		toDispatch = len(readyBeads)
	}

	if dryRun {
		fmt.Printf("%s Would dispatch %d bead(s) (capacity: %d/%d, batch: %d, ready: %d)\n",
			style.Bold.Render("📋"), toDispatch, activePolecats, maxPolecats, batchSize, len(readyBeads))
		for i := 0; i < toDispatch; i++ {
			b := readyBeads[i]
			fmt.Printf("  Would dispatch: %s → %s\n", b.ID, b.TargetRig)
		}
		return 0, nil
	}

	fmt.Printf("%s Dispatching %d bead(s) (capacity: %d free of %d, ready: %d)\n",
		style.Bold.Render("▶"), toDispatch, capacity, maxPolecats, len(readyBeads))

	dispatched := 0
	for i := 0; i < toDispatch; i++ {
		b := readyBeads[i]
		fmt.Printf("\n[%d/%d] Dispatching %s → %s...\n", i+1, toDispatch, b.ID, b.TargetRig)

		if err := dispatchSingleBead(b, townRoot); err != nil {
			fmt.Printf("  %s Failed: %v\n", style.Dim.Render("✗"), err)
			continue
		}
		dispatched++

		// Inter-spawn delay to avoid Dolt lock contention
		if i < toDispatch-1 && spawnDelay > 0 {
			time.Sleep(spawnDelay)
		}
	}

	// Update runtime state
	if dispatched > 0 {
		queueState.RecordDispatch(dispatched)
		if err := SaveQueueState(townRoot, queueState); err != nil {
			fmt.Printf("%s Could not save queue state: %v\n", style.Dim.Render("Warning:"), err)
		}
	}

	fmt.Printf("\n%s Dispatched %d/%d bead(s)\n", style.Bold.Render("✓"), dispatched, toDispatch)
	return dispatched, nil
}

// readyQueuedBead holds info about a queued bead ready for dispatch.
type readyQueuedBead struct {
	ID          string
	Title       string
	TargetRig   string
	Description string
	Labels      []string
}

// getReadyQueuedBeads queries for beads that are both queued and unblocked.
// Uses `bd ready --label gt:queued --json` which returns only unblocked beads.
func getReadyQueuedBeads(townRoot string) ([]readyQueuedBead, error) {
	cmd := exec.Command("bd", "ready", "--label", LabelQueued, "--json", "-n", "100")
	cmd.Dir = townRoot
	out, err := cmd.Output()
	if err != nil {
		// If no beads match, bd ready may exit non-zero
		return nil, nil
	}

	var raw []struct {
		ID          string   `json:"id"`
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Labels      []string `json:"labels"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing ready beads: %w", err)
	}

	result := make([]readyQueuedBead, 0, len(raw))
	for _, r := range raw {
		result = append(result, readyQueuedBead{
			ID:          r.ID,
			Title:       r.Title,
			TargetRig:   getQueueRig(r.Labels),
			Description: r.Description,
			Labels:      r.Labels,
		})
	}
	return result, nil
}

// dispatchSingleBead dequeues and dispatches one bead via slingBeadToPolecat.
// On failure after label removal, it re-adds the labels (put back in queue).
func dispatchSingleBead(b readyQueuedBead, townRoot string) error {
	// Parse queue metadata from description
	meta := ParseQueueMetadata(b.Description)

	// Remove queue labels (claim the bead)
	if err := dequeueBeadLabels(b.ID, townRoot); err != nil {
		return fmt.Errorf("removing queue labels: %w", err)
	}

	// Strip queue metadata from description
	cleanDesc := StripQueueMetadata(b.Description)
	if cleanDesc != b.Description {
		descCmd := exec.Command("bd", "update", b.ID, "--description="+cleanDesc)
		descCmd.Dir = townRoot
		_ = descCmd.Run() // best effort
	}

	// Build SlingParams from queue metadata
	params := SlingParams{
		BeadID:           b.ID,
		RigName:          b.TargetRig,
		FormulaFailFatal: true,  // Queue: rollback + requeue on failure
		Force:            true,  // Always force at dispatch (validated at enqueue)
		NoConvoy:         true,  // Convoy created at enqueue
		NoBoot:           true,  // Avoid lock contention in daemon
		TownRoot:         townRoot,
		BeadsDir:         townRoot + "/.beads",
	}
	if meta != nil {
		params.Args = meta.Args
	}

	// Dispatch via unified executeSling
	result, err := executeSling(params)
	if err != nil {
		// Re-queue on failure: re-add labels
		requeueBead(b.ID, b.TargetRig, townRoot)
		return fmt.Errorf("sling failed: %w", err)
	}

	// Log dispatch event
	polecatName := ""
	if result != nil && result.SpawnInfo != nil {
		polecatName = result.SpawnInfo.PolecatName
	}
	_ = events.LogFeed(events.TypeQueueDispatch, "daemon",
		events.QueueDispatchPayload(b.ID, b.TargetRig, polecatName))

	return nil
}

// requeueBead re-adds queue labels to a bead after a dispatch failure.
func requeueBead(beadID, rigName, townRoot string) {
	rigLabel := LabelQueueRigPrefix + rigName
	cmd := exec.Command("bd", "update", beadID,
		"--add-label="+LabelQueued,
		"--add-label="+rigLabel)
	cmd.Dir = townRoot
	if err := cmd.Run(); err != nil {
		fmt.Printf("  %s Could not re-queue %s: %v\n", style.Dim.Render("Warning:"), beadID, err)
	}
}
