package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/steveyegge/gastown/internal/config"
	"github.com/steveyegge/gastown/internal/events"
	"github.com/steveyegge/gastown/internal/style"
)

// maxDispatchFailures is the maximum number of consecutive dispatch failures
// before a bead is marked as gt:dispatch-failed and removed from the queue.
// Prevents permanently-failing beads from causing infinite retry loops.
const maxDispatchFailures = 3

// dispatchQueuedWork is the main dispatch loop for the work queue.
// Called by both `gt queue run` and the daemon heartbeat (via `gt queue run`).
//
// It checks capacity, queries ready beads, and dispatches up to batchSize beads.
// Returns the number of beads dispatched and any error.
func dispatchQueuedWork(townRoot, actor string, batchOverride, maxPolOverride int, dryRun bool) (int, error) {
	// Acquire exclusive lock to prevent concurrent dispatch from overlapping
	// daemon heartbeats. Without this, two `gt queue run` processes could race
	// on `bd ready --label gt:queued` and double-dispatch the same bead.
	runtimeDir := filepath.Join(townRoot, ".runtime")
	_ = os.MkdirAll(runtimeDir, 0755)
	lockFile := filepath.Join(runtimeDir, "queue-dispatch.lock")
	fileLock := flock.New(lockFile)
	locked, err := fileLock.TryLock()
	if err != nil {
		return 0, fmt.Errorf("acquiring dispatch lock: %w", err)
	}
	if !locked {
		// Another dispatch is already running — skip silently
		return 0, nil
	}
	defer func() { _ = fileLock.Unlock() }()

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
		// queue.enabled gates daemon auto-dispatch only. Manual `gt queue run`
		// always works so operators can dispatch without enabling the daemon loop.
		if isDaemonDispatch() {
			return 0, nil
		}
		fmt.Printf("%s Queue auto-dispatch is not enabled (manual dispatch proceeding)\n", style.Dim.Render("○"))
		fmt.Println("  Enable daemon dispatch with: gt config set queue.enabled true")
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

	// Compute available capacity (0 = unlimited)
	capacity := 0
	if maxPolecats > 0 {
		capacity = maxPolecats - activePolecats
		if capacity <= 0 {
			if dryRun {
				fmt.Printf("No capacity: %d/%d polecats active\n", activePolecats, maxPolecats)
			}
			return 0, nil
		}
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

	// Dispatch up to the smallest of capacity, batchSize, and readyBeads count.
	// When capacity is 0 (unlimited), only batchSize and readyBeads constrain dispatch.
	toDispatch := batchSize
	if capacity > 0 && capacity < toDispatch {
		toDispatch = capacity
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
	successfulRigs := make(map[string]bool)
	for i := 0; i < toDispatch; i++ {
		b := readyBeads[i]
		fmt.Printf("\n[%d/%d] Dispatching %s → %s...\n", i+1, toDispatch, b.ID, b.TargetRig)

		if err := dispatchSingleBead(b, townRoot, actor); err != nil {
			fmt.Printf("  %s Failed: %v\n", style.Dim.Render("✗"), err)
			continue
		}
		dispatched++
		if b.TargetRig != "" {
			successfulRigs[b.TargetRig] = true
		}

		// Inter-spawn delay to avoid Dolt lock contention
		if i < toDispatch-1 && spawnDelay > 0 {
			time.Sleep(spawnDelay)
		}
	}

	// Wake rig agents for each unique rig that had successful dispatches.
	// Dispatch runs with NoBoot=true to avoid lock contention, but polecats
	// need the witness awake to monitor them. Mirrors sling_batch.go post-loop.
	for rig := range successfulRigs {
		wakeRigAgents(rig)
	}

	// Update runtime state with fresh read to avoid clobbering concurrent pause.
	// Between our initial load and now, a user may have run `gt queue pause`.
	// Re-reading ensures we preserve the current pause state.
	if dispatched > 0 {
		freshState, err := LoadQueueState(townRoot)
		if err != nil {
			fmt.Printf("%s Could not reload queue state: %v\n", style.Dim.Render("Warning:"), err)
		} else {
			freshState.RecordDispatch(dispatched)
			if err := SaveQueueState(townRoot, freshState); err != nil {
				fmt.Printf("%s Could not save queue state: %v\n", style.Dim.Render("Warning:"), err)
			}
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
// Scans all rig directories since bd ready is CWD-scoped.
// Returns an error if ALL directories fail (bd unreachable), distinguishing
// from a legitimately empty queue.
func getReadyQueuedBeads(townRoot string) ([]readyQueuedBead, error) {
	var result []readyQueuedBead
	seen := make(map[string]bool)

	dirs := beadsSearchDirs(townRoot)
	var lastErr error
	failCount := 0

	for _, dir := range dirs {
		beads, err := getReadyQueuedBeadsFrom(dir)
		if err != nil {
			failCount++
			lastErr = err
			fmt.Printf("%s bd ready failed in %s: %v\n", style.Dim.Render("Warning:"), dir, err)
			continue
		}
		for _, b := range beads {
			if !seen[b.ID] {
				seen[b.ID] = true
				result = append(result, b)
			}
		}
	}

	// If every directory failed, bd is likely unreachable — surface the error
	if failCount == len(dirs) && failCount > 0 {
		return nil, fmt.Errorf("all %d bead directories failed (last: %w)", failCount, lastErr)
	}
	return result, nil
}

// getReadyQueuedBeadsFrom queries a single directory for ready queued beads.
func getReadyQueuedBeadsFrom(dir string) ([]readyQueuedBead, error) {
	cmd := exec.Command("bd", "ready", "--label", LabelQueued, "--json", "--limit=0")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("bd ready failed in %s: %w", dir, err)
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
		targetRig := ""
		meta := ParseQueueMetadata(r.Description)
		if meta != nil {
			targetRig = meta.TargetRig
			// Circuit breaker: skip beads that have exceeded max dispatch failures
			if meta.DispatchFailures >= maxDispatchFailures {
				continue
			}
		}
		result = append(result, readyQueuedBead{
			ID:          r.ID,
			Title:       r.Title,
			TargetRig:   targetRig,
			Description: r.Description,
			Labels:      r.Labels,
		})
	}
	return result, nil
}

// dispatchSingleBead dispatches one queued bead via executeSling.
// Reconstructs full SlingParams from queue metadata stored at enqueue time.
//
// The gt:queued label is intentionally NOT removed. Once executeSling hooks
// the bead, its status changes to "hooked" and bd ready won't return it —
// the flock prevents concurrent dispatch. Keeping the label provides an audit
// trail showing the bead was queue-dispatched. The label is only removed
// explicitly via `gt queue clear`.
func dispatchSingleBead(b readyQueuedBead, townRoot, actor string) error {
	// Parse queue metadata from description
	meta := ParseQueueMetadata(b.Description)

	// Validate metadata exists — beads without queue metadata (e.g., manually
	// labeled gt:queued) cannot be dispatched. Quarantine immediately rather
	// than wasting circuit breaker retries on guaranteed failures.
	if meta == nil || meta.TargetRig == "" {
		quarantineErr := fmt.Errorf("missing queue metadata or target_rig")
		failCmd := exec.Command("bd", "update", b.ID, "--add-label=gt:dispatch-failed")
		failCmd.Dir = resolveBeadDir(b.ID)
		_ = failCmd.Run() // best effort
		return quarantineErr
	}

	// Resolve rig name: prefer pre-parsed value, fall back to metadata
	rigName := b.TargetRig
	if rigName == "" {
		rigName = meta.TargetRig
	}

	// Reconstruct SlingParams from queue metadata
	params := SlingParams{
		BeadID:           b.ID,
		RigName:          rigName,
		FormulaFailFatal: true,  // Queue: rollback + requeue on failure
		Force:            true,  // Always force at dispatch (validated at enqueue)
		NoConvoy:         true,  // Convoy already created at enqueue
		NoBoot:           true,  // Avoid lock contention in daemon
		TownRoot:         townRoot,
		BeadsDir:         filepath.Join(townRoot, ".beads"),
	}
	if meta != nil {
		params.FormulaName = meta.Formula
		params.Args = meta.Args
		if meta.Vars != "" {
			params.Vars = splitVars(meta.Vars)
		}
		params.Merge = meta.Merge
		params.BaseBranch = meta.BaseBranch
		params.NoMerge = meta.NoMerge
		params.Account = meta.Account
		params.Agent = meta.Agent
		params.HookRawBead = meta.HookRawBead
	}

	// Dispatch via unified executeSling
	result, err := executeSling(params)
	if err != nil {
		_ = events.LogFeed(events.TypeQueueDispatchFailed, actor,
			events.QueueDispatchFailedPayload(b.ID, rigName, err.Error()))
		// Record failure in queue metadata for circuit breaker
		recordDispatchFailure(b, err)
		return fmt.Errorf("sling failed: %w", err)
	}

	// Strip queue metadata from description after successful dispatch.
	// The gt:queued label stays for audit, but metadata is no longer needed.
	// Uses resolveBeadDir for CWD so rig-scoped beads are found correctly.
	cleanDesc := StripQueueMetadata(b.Description)
	if cleanDesc != b.Description {
		descCmd := exec.Command("bd", "update", b.ID, "--description="+cleanDesc)
		descCmd.Dir = resolveBeadDir(b.ID)
		_ = descCmd.Run() // best effort — bead is already dispatched
	}

	// Log dispatch event
	polecatName := ""
	if result != nil && result.SpawnInfo != nil {
		polecatName = result.SpawnInfo.PolecatName
	}
	_ = events.LogFeed(events.TypeQueueDispatch, actor,
		events.QueueDispatchPayload(b.ID, rigName, polecatName))

	return nil
}

// isDaemonDispatch returns true when dispatch is triggered by the daemon heartbeat.
// The daemon sets GT_DAEMON=1 in the subprocess environment to distinguish
// automatic dispatch from manual `gt queue run`.
func isDaemonDispatch() bool {
	return os.Getenv("GT_DAEMON") == "1"
}

// recordDispatchFailure increments the dispatch failure counter in the bead's
// queue metadata. When the counter reaches maxDispatchFailures, adds the
// gt:dispatch-failed label so the bead is surfaced in queue status.
// Best-effort: the bead already failed, so metadata update failure is logged.
func recordDispatchFailure(b readyQueuedBead, dispatchErr error) {
	meta := ParseQueueMetadata(b.Description)
	if meta == nil {
		meta = &QueueMetadata{}
	}
	meta.DispatchFailures++
	meta.LastFailure = dispatchErr.Error()

	// Update description with incremented failure count
	baseDesc := StripQueueMetadata(b.Description)
	metaBlock := FormatQueueMetadata(meta)
	newDesc := baseDesc
	if newDesc != "" {
		newDesc += "\n"
	}
	newDesc += metaBlock

	beadDir := resolveBeadDir(b.ID)
	descCmd := exec.Command("bd", "update", b.ID, "--description="+newDesc)
	descCmd.Dir = beadDir
	_ = descCmd.Run() // best effort

	if meta.DispatchFailures >= maxDispatchFailures {
		// Mark as permanently failed — visible in queue status
		failCmd := exec.Command("bd", "update", b.ID, "--add-label=gt:dispatch-failed")
		failCmd.Dir = beadDir
		_ = failCmd.Run() // best effort
		fmt.Printf("  %s Bead %s failed %d times, marked gt:dispatch-failed\n",
			style.Warning.Render("⚠"), b.ID, meta.DispatchFailures)
	}
}

// splitVars splits a newline-separated vars string into individual key=value pairs.
func splitVars(vars string) []string {
	if vars == "" {
		return nil
	}
	var result []string
	for _, line := range strings.Split(vars, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}
