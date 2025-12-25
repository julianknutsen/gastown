package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/deacon"
	"github.com/steveyegge/gastown/internal/polecat"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/tmux"
	"github.com/steveyegge/gastown/internal/workspace"
)

// DeaconSessionName is the tmux session name for the Deacon.
const DeaconSessionName = "gt-deacon"

var deaconCmd = &cobra.Command{
	Use:     "deacon",
	Aliases: []string{"dea"},
	GroupID: GroupAgents,
	Short:   "Manage the Deacon session",
	Long: `Manage the Deacon tmux session.

The Deacon is the hierarchical health-check orchestrator for Gas Town.
It monitors the Mayor and Witnesses, handles lifecycle requests, and
keeps the town running. Use the subcommands to start, stop, attach,
and check status.`,
}

var deaconStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Deacon session",
	Long: `Start the Deacon tmux session.

Creates a new detached tmux session for the Deacon and launches Claude.
The session runs in the workspace root directory.`,
	RunE: runDeaconStart,
}

var deaconStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the Deacon session",
	Long: `Stop the Deacon tmux session.

Attempts graceful shutdown first (Ctrl-C), then kills the tmux session.`,
	RunE: runDeaconStop,
}

var deaconAttachCmd = &cobra.Command{
	Use:     "attach",
	Aliases: []string{"at"},
	Short:   "Attach to the Deacon session",
	Long: `Attach to the running Deacon tmux session.

Attaches the current terminal to the Deacon's tmux session.
Detach with Ctrl-B D.`,
	RunE: runDeaconAttach,
}

var deaconStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check Deacon session status",
	Long:  `Check if the Deacon tmux session is currently running.`,
	RunE:  runDeaconStatus,
}

var deaconRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the Deacon session",
	Long: `Restart the Deacon tmux session.

Stops the current session (if running) and starts a fresh one.`,
	RunE: runDeaconRestart,
}

var deaconHeartbeatCmd = &cobra.Command{
	Use:   "heartbeat [action]",
	Short: "Update the Deacon heartbeat",
	Long: `Update the Deacon heartbeat file.

The heartbeat signals to the daemon that the Deacon is alive and working.
Call this at the start of each wake cycle to prevent daemon pokes.

Examples:
  gt deacon heartbeat                    # Touch heartbeat with timestamp
  gt deacon heartbeat "checking mayor"   # Touch with action description`,
	RunE: runDeaconHeartbeat,
}

var deaconTriggerPendingCmd = &cobra.Command{
	Use:   "trigger-pending",
	Short: "Trigger pending polecat spawns (bootstrap mode)",
	Long: `Check inbox for POLECAT_STARTED messages and trigger ready polecats.

⚠️  BOOTSTRAP MODE ONLY - Uses regex detection (ZFC violation acceptable).

This command uses WaitForClaudeReady (regex) to detect when Claude is ready.
This is appropriate for daemon bootstrap when no AI is available.

In steady-state, the Deacon should use AI-based observation instead:
  gt deacon pending     # View pending spawns with captured output
  gt peek <session>     # Observe session output (AI analyzes)
  gt nudge <session>    # Trigger when AI determines ready

This command is typically called by the daemon during cold startup.`,
	RunE: runDeaconTriggerPending,
}

var deaconPendingCmd = &cobra.Command{
	Use:   "pending [session-to-clear]",
	Short: "[DEPRECATED] Use 'gt spawn pending' instead",
	Long: `DEPRECATED: Use 'gt spawn pending' instead.

This command has been moved to 'gt spawn pending' since pending spawn
tracking is not Deacon-specific - anyone might need to observe pending
polecats (Mayor, humans, debugging, etc.).

The functionality is identical:
  gt spawn pending                    # List all pending with output
  gt spawn pending gastown/p-abc123   # Clear specific session from pending`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDeaconPending,
}

var triggerTimeout time.Duration
var pendingLines int

func init() {
	deaconCmd.AddCommand(deaconStartCmd)
	deaconCmd.AddCommand(deaconStopCmd)
	deaconCmd.AddCommand(deaconAttachCmd)
	deaconCmd.AddCommand(deaconStatusCmd)
	deaconCmd.AddCommand(deaconRestartCmd)
	deaconCmd.AddCommand(deaconHeartbeatCmd)
	deaconCmd.AddCommand(deaconTriggerPendingCmd)
	deaconCmd.AddCommand(deaconPendingCmd)

	// Flags for trigger-pending
	deaconTriggerPendingCmd.Flags().DurationVar(&triggerTimeout, "timeout", 2*time.Second,
		"Timeout for checking if Claude is ready")

	// Flags for pending
	deaconPendingCmd.Flags().IntVarP(&pendingLines, "lines", "n", 15,
		"Number of terminal lines to capture per session")

	rootCmd.AddCommand(deaconCmd)
}

func runDeaconStart(cmd *cobra.Command, args []string) error {
	t := tmux.NewTmux()

	// Check if session already exists
	running, err := t.HasSession(DeaconSessionName)
	if err != nil {
		return fmt.Errorf("checking session: %w", err)
	}
	if running {
		return fmt.Errorf("Deacon session already running. Attach with: gt deacon attach")
	}

	if err := startDeaconSession(t); err != nil {
		return err
	}

	fmt.Printf("%s Deacon session started. Attach with: %s\n",
		style.Bold.Render("✓"),
		style.Dim.Render("gt deacon attach"))

	return nil
}

// startDeaconSession creates and initializes the Deacon tmux session.
func startDeaconSession(t *tmux.Tmux) error {
	// Find workspace root
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return fmt.Errorf("not in a Gas Town workspace: %w", err)
	}

	// Deacon runs from its own directory (for correct role detection by gt prime)
	deaconDir := filepath.Join(townRoot, "deacon")

	// Ensure deacon directory exists
	if err := os.MkdirAll(deaconDir, 0755); err != nil {
		return fmt.Errorf("creating deacon directory: %w", err)
	}

	// Create session in deacon directory
	fmt.Println("Starting Deacon session...")
	if err := t.NewSession(DeaconSessionName, deaconDir); err != nil {
		return fmt.Errorf("creating session: %w", err)
	}

	// Set environment
	_ = t.SetEnvironment(DeaconSessionName, "GT_ROLE", "deacon")
	_ = t.SetEnvironment(DeaconSessionName, "BD_ACTOR", "deacon")

	// Apply Deacon theme
	theme := tmux.DeaconTheme()
	_ = t.ConfigureGasTownSession(DeaconSessionName, theme, "", "Deacon", "health-check")

	// Launch Claude directly (no shell respawn loop)
	// Restarts are handled by daemon via ensureDeaconRunning on each heartbeat
	// The startup hook handles context loading automatically
	// Export GT_ROLE and BD_ACTOR in the command since tmux SetEnvironment only affects new panes
	if err := t.SendKeys(DeaconSessionName, "export GT_ROLE=deacon BD_ACTOR=deacon && claude --dangerously-skip-permissions"); err != nil {
		return fmt.Errorf("sending command: %w", err)
	}

	return nil
}

func runDeaconStop(cmd *cobra.Command, args []string) error {
	t := tmux.NewTmux()

	// Check if session exists
	running, err := t.HasSession(DeaconSessionName)
	if err != nil {
		return fmt.Errorf("checking session: %w", err)
	}
	if !running {
		return errors.New("Deacon session is not running")
	}

	fmt.Println("Stopping Deacon session...")

	// Try graceful shutdown first
	_ = t.SendKeysRaw(DeaconSessionName, "C-c")
	time.Sleep(100 * time.Millisecond)

	// Kill the session
	if err := t.KillSession(DeaconSessionName); err != nil {
		return fmt.Errorf("killing session: %w", err)
	}

	fmt.Printf("%s Deacon session stopped.\n", style.Bold.Render("✓"))
	return nil
}

func runDeaconAttach(cmd *cobra.Command, args []string) error {
	t := tmux.NewTmux()

	// Check if session exists
	running, err := t.HasSession(DeaconSessionName)
	if err != nil {
		return fmt.Errorf("checking session: %w", err)
	}
	if !running {
		// Auto-start if not running
		fmt.Println("Deacon session not running, starting...")
		if err := startDeaconSession(t); err != nil {
			return err
		}
	}
	// Session uses a respawn loop, so Claude restarts automatically if it exits

	// Use shared attach helper (smart: links if inside tmux, attaches if outside)
	return attachToTmuxSession(DeaconSessionName)
}

func runDeaconStatus(cmd *cobra.Command, args []string) error {
	t := tmux.NewTmux()

	running, err := t.HasSession(DeaconSessionName)
	if err != nil {
		return fmt.Errorf("checking session: %w", err)
	}

	if running {
		// Get session info for more details
		info, err := t.GetSessionInfo(DeaconSessionName)
		if err == nil {
			status := "detached"
			if info.Attached {
				status = "attached"
			}
			fmt.Printf("%s Deacon session is %s\n",
				style.Bold.Render("●"),
				style.Bold.Render("running"))
			fmt.Printf("  Status: %s\n", status)
			fmt.Printf("  Created: %s\n", info.Created)
			fmt.Printf("\nAttach with: %s\n", style.Dim.Render("gt deacon attach"))
		} else {
			fmt.Printf("%s Deacon session is %s\n",
				style.Bold.Render("●"),
				style.Bold.Render("running"))
		}
	} else {
		fmt.Printf("%s Deacon session is %s\n",
			style.Dim.Render("○"),
			"not running")
		fmt.Printf("\nStart with: %s\n", style.Dim.Render("gt deacon start"))
	}

	return nil
}

func runDeaconRestart(cmd *cobra.Command, args []string) error {
	t := tmux.NewTmux()

	running, err := t.HasSession(DeaconSessionName)
	if err != nil {
		return fmt.Errorf("checking session: %w", err)
	}

	fmt.Println("Restarting Deacon...")

	if running {
		// Kill existing session
		if err := t.KillSession(DeaconSessionName); err != nil {
			fmt.Printf("%s Warning: failed to kill session: %v\n", style.Dim.Render("⚠"), err)
		}
	}

	// Start fresh
	if err := runDeaconStart(cmd, args); err != nil {
		return err
	}

	fmt.Printf("%s Deacon restarted\n", style.Bold.Render("✓"))
	fmt.Printf("  %s\n", style.Dim.Render("Use 'gt deacon attach' to connect"))
	return nil
}

func runDeaconHeartbeat(cmd *cobra.Command, args []string) error {
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return fmt.Errorf("not in a Gas Town workspace: %w", err)
	}

	action := ""
	if len(args) > 0 {
		action = strings.Join(args, " ")
	}

	if action != "" {
		if err := deacon.TouchWithAction(townRoot, action, 0, 0); err != nil {
			return fmt.Errorf("updating heartbeat: %w", err)
		}
		fmt.Printf("%s Heartbeat updated: %s\n", style.Bold.Render("✓"), action)
	} else {
		if err := deacon.Touch(townRoot); err != nil {
			return fmt.Errorf("updating heartbeat: %w", err)
		}
		fmt.Printf("%s Heartbeat updated\n", style.Bold.Render("✓"))
	}

	return nil
}

func runDeaconTriggerPending(cmd *cobra.Command, args []string) error {
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return fmt.Errorf("not in a Gas Town workspace: %w", err)
	}

	// Step 1: Check inbox for new POLECAT_STARTED messages
	pending, err := polecat.CheckInboxForSpawns(townRoot)
	if err != nil {
		return fmt.Errorf("checking inbox: %w", err)
	}

	if len(pending) == 0 {
		fmt.Printf("%s No pending spawns\n", style.Dim.Render("○"))
		return nil
	}

	fmt.Printf("%s Found %d pending spawn(s)\n", style.Bold.Render("●"), len(pending))

	// Step 2: Try to trigger each pending spawn
	results, err := polecat.TriggerPendingSpawns(townRoot, triggerTimeout)
	if err != nil {
		return fmt.Errorf("triggering: %w", err)
	}

	// Report results
	triggered := 0
	for _, r := range results {
		if r.Triggered {
			triggered++
			fmt.Printf("  %s Triggered %s/%s\n",
				style.Bold.Render("✓"),
				r.Spawn.Rig, r.Spawn.Polecat)
		} else if r.Error != nil {
			fmt.Printf("  %s %s/%s: %v\n",
				style.Dim.Render("⚠"),
				r.Spawn.Rig, r.Spawn.Polecat, r.Error)
		}
	}

	// Step 3: Prune stale pending spawns (older than 5 minutes)
	pruned, _ := polecat.PruneStalePending(townRoot, 5*time.Minute)
	if pruned > 0 {
		fmt.Printf("  %s Pruned %d stale spawn(s)\n", style.Dim.Render("○"), pruned)
	}

	// Summary
	remaining := len(pending) - triggered
	if remaining > 0 {
		fmt.Printf("%s %d spawn(s) still waiting for Claude\n",
			style.Dim.Render("○"), remaining)
	}

	return nil
}

// runDeaconPending shows pending spawns with captured output for AI observation.
// This is the ZFC-compliant way for Deacon to observe polecats.
func runDeaconPending(cmd *cobra.Command, args []string) error {
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return fmt.Errorf("not in a Gas Town workspace: %w", err)
	}

	// If session argument provided, clear it from pending
	if len(args) == 1 {
		return clearPendingSession(townRoot, args[0])
	}

	// Step 1: Check inbox for new POLECAT_STARTED messages
	pending, err := polecat.CheckInboxForSpawns(townRoot)
	if err != nil {
		return fmt.Errorf("checking inbox: %w", err)
	}

	if len(pending) == 0 {
		fmt.Printf("%s No pending spawns\n", style.Dim.Render("○"))
		return nil
	}

	t := tmux.NewTmux()

	fmt.Printf("%s Pending spawns (%d):\n\n", style.Bold.Render("●"), len(pending))

	for i, ps := range pending {
		// Check if session still exists
		running, err := t.HasSession(ps.Session)
		if err != nil {
			fmt.Printf("Session: %s\n", ps.Session)
			fmt.Printf("  Status: error checking session: %v\n\n", err)
			continue
		}

		if !running {
			fmt.Printf("Session: %s\n", ps.Session)
			fmt.Printf("  Status: session no longer exists\n\n")
			continue
		}

		// Capture terminal output for AI analysis
		output, err := t.CapturePane(ps.Session, pendingLines)
		if err != nil {
			fmt.Printf("Session: %s\n", ps.Session)
			fmt.Printf("  Status: error capturing output: %v\n\n", err)
			continue
		}

		// Print session info
		fmt.Printf("Session: %s\n", ps.Session)
		fmt.Printf("  Rig: %s\n", ps.Rig)
		fmt.Printf("  Polecat: %s\n", ps.Polecat)
		if ps.Issue != "" {
			fmt.Printf("  Issue: %s\n", ps.Issue)
		}
		fmt.Printf("  Spawned: %s ago\n", time.Since(ps.SpawnedAt).Round(time.Second))
		fmt.Printf("  Terminal output (last %d lines):\n", pendingLines)
		fmt.Println(strings.Repeat("─", 50))
		fmt.Println(output)
		fmt.Println(strings.Repeat("─", 50))

		if i < len(pending)-1 {
			fmt.Println()
		}
	}

	fmt.Println()
	fmt.Printf("%s To trigger a ready polecat:\n", style.Dim.Render("→"))
	fmt.Printf("  gt nudge <session> \"Begin.\"\n")

	return nil
}

// clearPendingSession removes a session from the pending list.
func clearPendingSession(townRoot, session string) error {
	pending, err := polecat.LoadPending(townRoot)
	if err != nil {
		return fmt.Errorf("loading pending: %w", err)
	}

	var remaining []*polecat.PendingSpawn
	found := false
	for _, ps := range pending {
		if ps.Session == session {
			found = true
			continue
		}
		remaining = append(remaining, ps)
	}

	if !found {
		return fmt.Errorf("session %s not found in pending list", session)
	}

	if err := polecat.SavePending(townRoot, remaining); err != nil {
		return fmt.Errorf("saving pending: %w", err)
	}

	fmt.Printf("%s Cleared %s from pending list\n", style.Bold.Render("✓"), session)
	return nil
}
