package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gofrs/flock"
	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/config"
	"github.com/steveyegge/gastown/internal/events"
	"github.com/steveyegge/gastown/internal/scheduler/capacity"
	"github.com/steveyegge/gastown/internal/style"
)

// maxDispatchFailures is the maximum number of consecutive dispatch failures
// before a sling context is closed as circuit-broken.
const maxDispatchFailures = 3

// dispatchScheduledWork is the main dispatch loop for the capacity scheduler.
// Called by both `gt scheduler run` and the daemon heartbeat.
func dispatchScheduledWork(townRoot, actor string, batchOverride int, dryRun bool) (int, error) {
	// Acquire exclusive lock to prevent concurrent dispatch
	runtimeDir := filepath.Join(townRoot, ".runtime")
	_ = os.MkdirAll(runtimeDir, 0755)
	lockFile := filepath.Join(runtimeDir, "scheduler-dispatch.lock")
	fileLock := flock.New(lockFile)
	locked, err := fileLock.TryLock()
	if err != nil {
		return 0, fmt.Errorf("acquiring dispatch lock: %w", err)
	}
	if !locked {
		return 0, nil
	}
	defer func() { _ = fileLock.Unlock() }()

	// Load scheduler state
	state, err := capacity.LoadState(townRoot)
	if err != nil {
		return 0, fmt.Errorf("loading scheduler state: %w", err)
	}

	if state.Paused {
		if !dryRun {
			fmt.Printf("%s Scheduler is paused (by %s), skipping dispatch\n", style.Dim.Render("⏸"), state.PausedBy)
		}
		return 0, nil
	}

	// Load town settings for scheduler config
	settingsPath := config.TownSettingsPath(townRoot)
	settings, err := config.LoadOrCreateTownSettings(settingsPath)
	if err != nil {
		return 0, fmt.Errorf("loading town settings: %w", err)
	}

	schedulerCfg := settings.Scheduler
	if schedulerCfg == nil {
		schedulerCfg = capacity.DefaultSchedulerConfig()
	}

	// Nothing to dispatch when scheduler is in direct dispatch or disabled mode.
	maxPolecats := schedulerCfg.GetMaxPolecats()
	if maxPolecats <= 0 {
		if !dryRun && !isDaemonDispatch() {
			staleBeads, _ := getReadySlingContexts(townRoot)
			if len(staleBeads) > 0 {
				fmt.Printf("%s %d context bead(s) still open from a previous deferred mode\n",
					style.Warning.Render("⚠"), len(staleBeads))
				fmt.Printf("  Use: gt scheduler clear  (close all sling context beads)\n")
				fmt.Printf("  Or:  gt config set scheduler.max_polecats N  (re-enable deferred dispatch)\n")
			}
		}
		return 0, nil
	}

	// Determine limits
	batchSize := schedulerCfg.GetBatchSize()
	if batchOverride > 0 {
		batchSize = batchOverride
	}
	spawnDelay := schedulerCfg.GetSpawnDelay()

	townBeads := beads.NewWithBeadsDir(townRoot, filepath.Join(townRoot, ".beads"))

	// Clean up invalid/stale contexts before querying for ready beads.
	cleanupStaleContexts(townRoot)

	// Wire up the DispatchCycle
	successfulRigs := make(map[string]bool)
	// Track polecat names from dispatch results, keyed by context bead ID.
	polecatNames := make(map[string]string)
	cycle := &capacity.DispatchCycle{
		AvailableCapacity: func() (int, error) {
			active := countActivePolecats()
			cap := maxPolecats - active
			if cap <= 0 {
				return 0, nil // No free slots — PlanDispatch treats <= 0 as no capacity
			}
			return cap, nil
		},
		QueryPending: func() ([]capacity.PendingBead, error) {
			return getReadySlingContexts(townRoot)
		},
		Execute: func(b capacity.PendingBead) error {
			result, err := dispatchSingleBead(b, townRoot, actor)
			if err != nil {
				return err
			}
			// Track side effects here (Execute runs exactly once, never retried).
			if result != nil && result.PolecatName != "" {
				polecatNames[b.ID] = result.PolecatName
			}
			if b.TargetRig != "" {
				successfulRigs[b.TargetRig] = true
			}
			_ = events.LogFeed(events.TypeSchedulerDispatch, actor,
				events.SchedulerDispatchPayload(b.WorkBeadID, b.TargetRig, polecatNames[b.ID]))
			return nil
		},
		OnSuccess: func(b capacity.PendingBead) error {
			// OnSuccess may be retried — only do the close here, no side effects.
			return townBeads.CloseSlingContext(b.ID, "dispatched")
		},
		OnFailure: func(b capacity.PendingBead, err error) {
			_ = events.LogFeed(events.TypeSchedulerDispatchFailed, actor,
				events.SchedulerDispatchFailedPayload(b.WorkBeadID, b.TargetRig, err.Error()))
			recordDispatchFailure(townBeads, b, err)
		},
		BatchSize:  batchSize,
		SpawnDelay: spawnDelay,
	}

	if dryRun {
		plan, planErr := cycle.Plan()
		if planErr != nil {
			return 0, fmt.Errorf("planning dispatch: %w", planErr)
		}
		printDryRunPlan(plan, maxPolecats, batchSize)
		return 0, nil
	}

	report, err := cycle.Run()
	if err != nil {
		return 0, fmt.Errorf("dispatch cycle failed: %w", err)
	}

	// Wake rig agents for each unique rig that had successful dispatches.
	for rig := range successfulRigs {
		wakeRigAgents(rig)
	}

	// Update runtime state with fresh read to avoid clobbering concurrent pause.
	if report.Dispatched > 0 {
		freshState, err := capacity.LoadState(townRoot)
		if err != nil {
			fmt.Printf("%s Could not reload scheduler state: %v\n", style.Dim.Render("Warning:"), err)
		} else {
			freshState.RecordDispatch(report.Dispatched)
			if err := capacity.SaveState(townRoot, freshState); err != nil {
				fmt.Printf("%s Could not save scheduler state: %v\n", style.Dim.Render("Warning:"), err)
			}
		}
	}

	if report.Dispatched > 0 || report.Failed > 0 {
		fmt.Printf("\n%s Dispatched %d, failed %d (reason: %s)\n",
			style.Bold.Render("✓"), report.Dispatched, report.Failed, report.Reason)
	}

	return report.Dispatched, nil
}

// printDryRunPlan displays a dry-run dispatch plan.
func printDryRunPlan(plan capacity.DispatchPlan, maxPolecats, batchSize int) {
	if plan.Reason == "none" {
		fmt.Println("No ready beads scheduled for dispatch")
		return
	}

	activePolecats := countActivePolecats()
	capStr := "unlimited"
	if maxPolecats > 0 {
		cap := maxPolecats - activePolecats
		if cap < 0 {
			cap = 0
		}
		capStr = fmt.Sprintf("%d free of %d", cap, maxPolecats)
	}

	totalReady := len(plan.ToDispatch) + plan.Skipped
	if len(plan.ToDispatch) == 0 {
		fmt.Printf("No capacity: %s, %d ready bead(s) waiting\n", capStr, totalReady)
		return
	}

	fmt.Printf("%s Would dispatch %d bead(s) (capacity: %s, batch: %d, ready: %d, reason: %s)\n",
		style.Bold.Render("📋"), len(plan.ToDispatch), capStr, batchSize, totalReady, plan.Reason)
	for _, b := range plan.ToDispatch {
		fmt.Printf("  Would dispatch: %s → %s\n", b.WorkBeadID, b.TargetRig)
	}
}

// cleanupStaleContexts closes invalid and stale sling context beads.
// Called explicitly before the dispatch cycle to separate cleanup from querying.
func cleanupStaleContexts(townRoot string) {
	townBeads := beads.NewWithBeadsDir(townRoot, filepath.Join(townRoot, ".beads"))

	contexts, err := listAllSlingContexts(townRoot)
	if err != nil {
		return
	}

	for _, ctx := range contexts {
		fields := beads.ParseSlingContextFields(ctx.Description)
		if fields == nil {
			// Invalid context bead — close it
			_ = townBeads.CloseSlingContext(ctx.ID, "invalid-context")
			continue
		}

		// Circuit-broken cleanup: close contexts that hit the failure threshold
		// but whose previous close attempt failed.
		if fields.DispatchFailures >= maxDispatchFailures {
			_ = townBeads.CloseSlingContext(ctx.ID, "circuit-broken")
			continue
		}

		// Stale cleanup: if work bead is hooked/in_progress/closed, close context
		workInfo, err := getBeadInfo(fields.WorkBeadID)
		if err == nil {
			if workInfo.Status == "hooked" || workInfo.Status == "closed" || workInfo.Status == "tombstone" {
				_ = townBeads.CloseSlingContext(ctx.ID, "stale-work-bead")
			}
		}
	}
}

// getReadySlingContexts queries for sling context beads whose work beads are ready.
// This is a pure query — no destructive side effects. Call cleanupStaleContexts()
// before this function to handle invalid/stale contexts.
func getReadySlingContexts(townRoot string) ([]capacity.PendingBead, error) {
	// 1. List all open sling context beads across all dirs
	var allContexts []*beads.Issue
	seen := make(map[string]bool)

	dirs := beadsSearchDirs(townRoot)
	var lastErr error
	failCount := 0

	for _, dir := range dirs {
		dirBeads := beads.NewWithBeadsDir(dir, filepath.Join(dir, ".beads"))
		contexts, err := dirBeads.ListOpenSlingContexts()
		if err != nil {
			failCount++
			lastErr = err
			continue
		}
		for _, ctx := range contexts {
			if !seen[ctx.ID] {
				seen[ctx.ID] = true
				allContexts = append(allContexts, ctx)
			}
		}
	}

	if failCount == len(dirs) && failCount > 0 {
		return nil, fmt.Errorf("all %d bead directories failed (last: %w)", failCount, lastErr)
	}

	if len(allContexts) == 0 {
		return nil, nil
	}

	// 2. Build readyWorkIDs set from bd ready across all dirs
	readyWorkIDs := make(map[string]bool)
	for _, dir := range dirs {
		readyCmd := exec.Command("bd", "ready", "--json", "--limit=0")
		readyCmd.Dir = dir
		readyOut, err := readyCmd.Output()
		if err != nil {
			continue
		}
		var readyBeads []struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(readyOut, &readyBeads); err == nil {
			for _, b := range readyBeads {
				readyWorkIDs[b.ID] = true
			}
		}
	}

	// 3. Build PendingBead list — pure filtering, no mutations.
	// Deduplicate by WorkBeadID: keep the oldest context (first seen) to prevent
	// duplicate dispatches if concurrent scheduleBead calls created multiple contexts.
	seenWork := make(map[string]bool)
	var result []capacity.PendingBead
	for _, ctx := range allContexts {
		fields := beads.ParseSlingContextFields(ctx.Description)
		if fields == nil {
			continue // Skip invalid — cleanupStaleContexts handles these
		}

		// Circuit breaker filter
		if fields.DispatchFailures >= maxDispatchFailures {
			continue
		}

		// Only include if work bead is ready (unblocked)
		if !readyWorkIDs[fields.WorkBeadID] {
			continue
		}

		// Deduplicate: one dispatch per work bead (keep first/oldest context)
		if seenWork[fields.WorkBeadID] {
			continue
		}
		seenWork[fields.WorkBeadID] = true

		result = append(result, capacity.PendingBead{
			ID:          ctx.ID,
			WorkBeadID:  fields.WorkBeadID,
			Title:       ctx.Title,
			TargetRig:   fields.TargetRig,
			Description: ctx.Description,
			Labels:      ctx.Labels,
			Context:     fields,
		})
	}

	return result, nil
}

// dispatchSingleBead dispatches one scheduled bead via executeSling.
// Context fields are already parsed (from PendingBead.Context).
// Returns the SlingResult (including PolecatName) on success.
func dispatchSingleBead(b capacity.PendingBead, townRoot, actor string) (*SlingResult, error) {
	if b.Context == nil {
		return nil, fmt.Errorf("missing sling context for %s", b.ID)
	}

	dp := capacity.ReconstructFromContext(b.Context)
	params := SlingParams{
		BeadID:           dp.BeadID,
		RigName:          dp.RigName,
		FormulaName:      dp.FormulaName,
		Args:             dp.Args,
		Vars:             dp.Vars,
		Merge:            dp.Merge,
		BaseBranch:       dp.BaseBranch,
		NoMerge:          dp.NoMerge,
		Account:          dp.Account,
		Agent:            dp.Agent,
		HookRawBead:      dp.HookRawBead,
		Mode:             dp.Mode,
		FormulaFailFatal: true,
		CallerContext:    "scheduler-dispatch",
		NoConvoy:         true,
		NoBoot:           true,
		TownRoot:         townRoot,
		BeadsDir:         filepath.Join(townRoot, ".beads"),
	}

	fmt.Printf("  Dispatching %s → %s...\n", b.WorkBeadID, b.TargetRig)
	result, err := executeSling(params)
	if err != nil {
		return nil, fmt.Errorf("sling failed: %w", err)
	}

	return result, nil
}

// isDaemonDispatch returns true when dispatch is triggered by the daemon heartbeat.
func isDaemonDispatch() bool {
	return os.Getenv("GT_DAEMON") == "1"
}

// recordDispatchFailure increments the dispatch failure counter on the sling context bead.
func recordDispatchFailure(townBeads *beads.Beads, b capacity.PendingBead, dispatchErr error) {
	if b.Context == nil {
		return
	}

	b.Context.DispatchFailures++
	b.Context.LastFailure = dispatchErr.Error()

	if err := townBeads.UpdateSlingContextFields(b.ID, b.Context); err != nil {
		fmt.Printf("  %s Failed to record dispatch failure for %s: %v\n",
			style.Warning.Render("⚠"), b.ID, err)
	}

	if b.Context.DispatchFailures >= maxDispatchFailures {
		if err := townBeads.CloseSlingContext(b.ID, "circuit-broken"); err != nil {
			fmt.Printf("  %s Failed to close circuit-broken context %s: %v\n",
				style.Warning.Render("⚠"), b.ID, err)
		}
		fmt.Printf("  %s Context %s (work: %s) failed %d times, circuit-broken\n",
			style.Warning.Render("⚠"), b.ID, b.WorkBeadID, b.Context.DispatchFailures)
	}
}

// listAllSlingContexts returns all open sling context beads across all dirs.
// Used by scheduler list/status. Does NOT filter by readiness or circuit breaker.
func listAllSlingContexts(townRoot string) ([]*beads.Issue, error) {
	var result []*beads.Issue
	seen := make(map[string]bool)

	dirs := beadsSearchDirs(townRoot)
	var lastErr error
	failCount := 0

	for _, dir := range dirs {
		dirBeads := beads.NewWithBeadsDir(dir, filepath.Join(dir, ".beads"))
		contexts, err := dirBeads.ListOpenSlingContexts()
		if err != nil {
			failCount++
			lastErr = err
			continue
		}
		for _, ctx := range contexts {
			if !seen[ctx.ID] {
				seen[ctx.ID] = true
				result = append(result, ctx)
			}
		}
	}

	if failCount == len(dirs) && failCount > 0 {
		return nil, fmt.Errorf("all %d bead directories failed (last: %w)", failCount, lastErr)
	}

	return result, nil
}

// listReadyWorkBeadIDs returns a set of work bead IDs that are unblocked.
func listReadyWorkBeadIDs(townRoot string) map[string]bool {
	readyIDs := make(map[string]bool)
	for _, dir := range beadsSearchDirs(townRoot) {
		readyCmd := exec.Command("bd", "ready", "--json", "--limit=0")
		readyCmd.Dir = dir
		readyOut, err := readyCmd.Output()
		if err != nil {
			continue
		}
		var readyBeads []struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(readyOut, &readyBeads); err == nil {
			for _, b := range readyBeads {
				readyIDs[b.ID] = true
			}
		}
	}
	return readyIDs
}

