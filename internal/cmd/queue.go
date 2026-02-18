package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/session"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/workspace"
)

var (
	queueStatusJSON bool
	queueListJSON   bool
	queueClearBead  string
	queueRunBatch   int
	queueRunDryRun  bool
	queueRunMaxPol  int
)

var queueCmd = &cobra.Command{
	Use:     "queue",
	GroupID: GroupWork,
	Short:   "Manage the work queue for capacity-controlled dispatch",
	Long: `Manage the work queue for capacity-controlled polecat dispatch.

The work queue enables "set and forget" dispatch: queue beads, and the
daemon dispatches them as capacity allows. Blocked beads automatically
dispatch when their blockers resolve.

Queue work:
  gt sling gt-abc gastown --queue         # Queue single bead
  gt sling gt-abc gt-def gastown --queue  # Queue batch
  gt convoy queue hq-cv-abc gastown       # Queue convoy's issues
  gt queue epic gt-epic-123 gastown       # Queue epic's children

Manage queue:
  gt queue status                         # Show queue state
  gt queue list                           # List all queued beads
  gt queue run                            # Manual dispatch trigger
  gt queue pause                          # Pause all dispatch
  gt queue resume                         # Resume dispatch
  gt queue clear                          # Remove all beads from queue`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return requireSubcommand(cmd, args)
	},
}

var queueStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show queue state: pending, capacity, active polecats",
	RunE:  runQueueStatus,
}

var queueListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all queued beads with titles, rig, blocked status",
	RunE:  runQueueList,
}

var queuePauseCmd = &cobra.Command{
	Use:   "pause",
	Short: "Pause all queue dispatch (town-wide)",
	RunE:  runQueuePause,
}

var queueResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume queue dispatch",
	RunE:  runQueueResume,
}

var queueClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Remove beads from the queue",
	Long: `Remove beads from the queue by clearing gt:queued labels.

Without --bead, removes ALL beads from the queue.
With --bead, removes only the specified bead.`,
	RunE: runQueueClear,
}

var queueRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Manually trigger queue dispatch",
	Long: `Manually trigger dispatch of queued work.

This dispatches queued beads using the same logic as the daemon heartbeat,
but can be run ad-hoc. Useful for testing or when the daemon is not running.

  gt queue run                  # Dispatch using config defaults
  gt queue run --batch 5        # Dispatch up to 5
  gt queue run --dry-run        # Preview what would dispatch`,
	RunE: runQueueRun,
}

func init() {
	// Status flags
	queueStatusCmd.Flags().BoolVar(&queueStatusJSON, "json", false, "Output as JSON")

	// List flags
	queueListCmd.Flags().BoolVar(&queueListJSON, "json", false, "Output as JSON")

	// Clear flags
	queueClearCmd.Flags().StringVar(&queueClearBead, "bead", "", "Remove specific bead from queue")

	// Run flags
	queueRunCmd.Flags().IntVar(&queueRunBatch, "batch", 0, "Override batch size (0 = use config)")
	queueRunCmd.Flags().BoolVar(&queueRunDryRun, "dry-run", false, "Preview what would dispatch")
	queueRunCmd.Flags().IntVar(&queueRunMaxPol, "max-polecats", 0, "Override max polecats (0 = use config)")

	// Add subcommands
	queueCmd.AddCommand(queueStatusCmd)
	queueCmd.AddCommand(queueListCmd)
	queueCmd.AddCommand(queuePauseCmd)
	queueCmd.AddCommand(queueResumeCmd)
	queueCmd.AddCommand(queueClearCmd)
	queueCmd.AddCommand(queueRunCmd)

	rootCmd.AddCommand(queueCmd)
}

// queuedBeadInfo holds info about a queued bead for display.
type queuedBeadInfo struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	TargetRig string `json:"target_rig"`
	Blocked   bool   `json:"blocked,omitempty"`
}

func runQueueStatus(cmd *cobra.Command, args []string) error {
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return err
	}

	// Load queue config
	queueState, err := LoadQueueState(townRoot)
	if err != nil {
		return fmt.Errorf("loading queue state: %w", err)
	}

	// Query queued beads
	queued, err := listQueuedBeads(townRoot)
	if err != nil {
		return fmt.Errorf("listing queued beads: %w", err)
	}

	// Count active polecats (simplified: count tmux sessions matching polecat pattern)
	activePolecats := countActivePolecats()

	if queueStatusJSON {
		out := struct {
			Paused         bool             `json:"paused"`
			PausedBy       string           `json:"paused_by,omitempty"`
			QueuedTotal    int              `json:"queued_total"`
			QueuedReady    int              `json:"queued_ready"`
			ActivePolecats int              `json:"active_polecats"`
			LastDispatchAt string           `json:"last_dispatch_at,omitempty"`
			Beads          []queuedBeadInfo `json:"beads"`
		}{
			Paused:         queueState.Paused,
			PausedBy:       queueState.PausedBy,
			QueuedTotal:    len(queued),
			ActivePolecats: activePolecats,
			LastDispatchAt: queueState.LastDispatchAt,
			Beads:          queued,
		}
		// Count ready (not blocked)
		for _, b := range queued {
			if !b.Blocked {
				out.QueuedReady++
			}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	// Human-readable output
	readyCount := 0
	for _, b := range queued {
		if !b.Blocked {
			readyCount++
		}
	}

	fmt.Printf("%s\n\n", style.Bold.Render("Work Queue Status"))
	if queueState.Paused {
		fmt.Printf("  State:    %s (by %s)\n", style.Warning.Render("PAUSED"), queueState.PausedBy)
	} else {
		fmt.Printf("  State:    active\n")
	}
	fmt.Printf("  Queued:   %d total, %d ready\n", len(queued), readyCount)
	fmt.Printf("  Active:   %d polecats\n", activePolecats)
	if queueState.LastDispatchAt != "" {
		fmt.Printf("  Last dispatch: %s (%d beads)\n", queueState.LastDispatchAt, queueState.LastDispatchCount)
	}

	return nil
}

func runQueueList(cmd *cobra.Command, args []string) error {
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return err
	}

	queued, err := listQueuedBeads(townRoot)
	if err != nil {
		return fmt.Errorf("listing queued beads: %w", err)
	}

	if queueListJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(queued)
	}

	if len(queued) == 0 {
		fmt.Println("Queue is empty.")
		fmt.Println("Queue work with: gt sling <bead> <rig> --queue")
		return nil
	}

	// Group by target rig
	byRig := make(map[string][]queuedBeadInfo)
	for _, b := range queued {
		byRig[b.TargetRig] = append(byRig[b.TargetRig], b)
	}

	fmt.Printf("%s (%d beads)\n\n", style.Bold.Render("Queued Work"), len(queued))
	for rig, beads := range byRig {
		fmt.Printf("  %s (%d):\n", style.Bold.Render(rig), len(beads))
		for _, b := range beads {
			indicator := "○"
			if b.Blocked {
				indicator = "⏸"
			}
			fmt.Printf("    %s %s: %s\n", indicator, b.ID, b.Title)
		}
		fmt.Println()
	}

	return nil
}

func runQueuePause(cmd *cobra.Command, args []string) error {
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return err
	}

	state, err := LoadQueueState(townRoot)
	if err != nil {
		return fmt.Errorf("loading queue state: %w", err)
	}

	if state.Paused {
		fmt.Printf("%s Queue is already paused (by %s)\n", style.Dim.Render("○"), state.PausedBy)
		return nil
	}

	actor := detectActor()
	state.SetPaused(actor)
	if err := SaveQueueState(townRoot, state); err != nil {
		return fmt.Errorf("saving queue state: %w", err)
	}

	fmt.Printf("%s Queue paused\n", style.Bold.Render("⏸"))
	return nil
}

func runQueueResume(cmd *cobra.Command, args []string) error {
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return err
	}

	state, err := LoadQueueState(townRoot)
	if err != nil {
		return fmt.Errorf("loading queue state: %w", err)
	}

	if !state.Paused {
		fmt.Printf("%s Queue is not paused\n", style.Dim.Render("○"))
		return nil
	}

	state.SetResumed()
	if err := SaveQueueState(townRoot, state); err != nil {
		return fmt.Errorf("saving queue state: %w", err)
	}

	fmt.Printf("%s Queue resumed\n", style.Bold.Render("▶"))
	return nil
}

func runQueueClear(cmd *cobra.Command, args []string) error {
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return err
	}

	if queueClearBead != "" {
		// Clear specific bead
		if err := dequeueBeadLabels(queueClearBead); err != nil {
			return fmt.Errorf("clearing bead %s from queue: %w", queueClearBead, err)
		}
		fmt.Printf("%s Removed %s from queue\n", style.Bold.Render("✓"), queueClearBead)
		return nil
	}

	// Clear all queued beads
	queued, err := listQueuedBeads(townRoot)
	if err != nil {
		return fmt.Errorf("listing queued beads: %w", err)
	}

	if len(queued) == 0 {
		fmt.Println("Queue is already empty.")
		return nil
	}

	cleared := 0
	for _, b := range queued {
		if err := dequeueBeadLabels(b.ID); err != nil {
			fmt.Printf("  %s Could not clear %s: %v\n", style.Dim.Render("Warning:"), b.ID, err)
			continue
		}
		cleared++
	}

	fmt.Printf("%s Cleared %d bead(s) from queue\n", style.Bold.Render("✓"), cleared)
	return nil
}

func runQueueRun(cmd *cobra.Command, args []string) error {
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return err
	}

	_, err = dispatchQueuedWork(townRoot, detectActor(), queueRunBatch, queueRunMaxPol, queueRunDryRun)
	return err
}

// listQueuedBeads returns all beads with the gt:queued label across all rig DBs.
// bd list is CWD-scoped, so we scan all rig directories to find queued beads.
// Populates Blocked by reconciling bd list (all queued) vs bd ready (unblocked).
// Returns an error if ALL directories fail (bd unreachable).
func listQueuedBeads(townRoot string) ([]queuedBeadInfo, error) {
	var result []queuedBeadInfo
	seen := make(map[string]bool)

	dirs := beadsSearchDirs(townRoot)

	// Collect ready (unblocked) bead IDs for blocked-status reconciliation.
	// Also parses descriptions to skip circuit-broken beads consistently
	// with dispatch filtering.
	readyIDs := make(map[string]bool)
	for _, dir := range dirs {
		readyCmd := exec.Command("bd", "ready", "--label", LabelQueued, "--json", "--limit=0")
		readyCmd.Dir = dir
		readyOut, err := readyCmd.Output()
		if err != nil {
			continue
		}
		var readyBeads []struct {
			ID          string `json:"id"`
			Description string `json:"description"`
		}
		if err := json.Unmarshal(readyOut, &readyBeads); err == nil {
			for _, b := range readyBeads {
				// Skip circuit-broken beads from ready set
				if meta := ParseQueueMetadata(b.Description); meta != nil && meta.DispatchFailures >= maxDispatchFailures {
					continue
				}
				readyIDs[b.ID] = true
			}
		}
	}

	var lastErr error
	failCount := 0
	for _, dir := range dirs {
		beads, err := listQueuedBeadsFrom(dir)
		if err != nil {
			failCount++
			lastErr = err
			continue
		}
		for _, b := range beads {
			// Skip already-dispatched beads (hooked/closed). The gt:queued
			// label stays as audit trail, but queue list shows only pending.
			if b.Status == "hooked" || b.Status == "closed" {
				continue
			}
			if !seen[b.ID] {
				seen[b.ID] = true
				b.Blocked = !readyIDs[b.ID]
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

// listQueuedBeadsFrom queries a single directory for beads with gt:queued label.
func listQueuedBeadsFrom(dir string) ([]queuedBeadInfo, error) {
	listCmd := exec.Command("bd", "list", "--label="+LabelQueued, "--json", "--limit=0")
	listCmd.Dir = dir
	var stdout strings.Builder
	listCmd.Stdout = &stdout

	if err := listCmd.Run(); err != nil {
		return nil, err
	}

	var raw []struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Status      string `json:"status"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(stdout.String()), &raw); err != nil {
		return nil, fmt.Errorf("parsing queued beads: %w", err)
	}

	result := make([]queuedBeadInfo, 0, len(raw))
	for _, r := range raw {
		targetRig := ""
		meta := ParseQueueMetadata(r.Description)
		if meta != nil {
			targetRig = meta.TargetRig
			// Skip circuit-broken beads — they are permanently failed and
			// should not appear as pending queue items.
			if meta.DispatchFailures >= maxDispatchFailures {
				continue
			}
		}
		result = append(result, queuedBeadInfo{
			ID:        r.ID,
			Title:     r.Title,
			Status:    r.Status,
			TargetRig: targetRig,
		})
	}
	return result, nil
}

// beadsSearchDirs returns directories to scan for queued beads:
// the town root plus any rig directories that have a .beads/ subdirectory.
func beadsSearchDirs(townRoot string) []string {
	dirs := []string{townRoot}
	entries, err := os.ReadDir(townRoot)
	if err != nil {
		return dirs
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") || e.Name() == "mayor" || e.Name() == "settings" {
			continue
		}
		rigDir := filepath.Join(townRoot, e.Name())
		beadsDir := filepath.Join(rigDir, ".beads")
		if _, err := os.Stat(beadsDir); err == nil {
			dirs = append(dirs, rigDir)
		}
	}
	return dirs
}

// countActivePolecats counts all running polecats across all rigs in the town.
// Uses session.ParseSessionName for canonical role detection rather than string
// heuristics, ensuring correct counting regardless of rig prefix or polecat name.
func countActivePolecats() int {
	listCmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}")
	out, err := listCmd.Output()
	if err != nil {
		return 0
	}

	count := 0
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		identity, err := session.ParseSessionName(line)
		if err != nil {
			continue // Not a gt-managed session
		}
		if identity.Role == session.RolePolecat {
			count++
		}
	}
	return count
}
