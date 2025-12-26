package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/tmux"
	"github.com/steveyegge/gastown/internal/wisp"
)

var handoffCmd = &cobra.Command{
	Use:     "handoff [bead-or-role]",
	GroupID: GroupWork,
	Short:   "Hand off to a fresh session, work continues from hook",
	Long: `End watch. Hand off to a fresh agent session.

This is the canonical way to end any agent session. It handles all roles:

  - Mayor, Crew, Witness, Refinery, Deacon: Respawns with fresh Claude instance
  - Polecats: Calls 'gt done --exit DEFERRED' (Witness handles lifecycle)

When run without arguments, hands off the current session.
When given a bead ID (gt-xxx, hq-xxx), hooks that work first, then restarts.
When given a role name, hands off that role's session (and switches to it).

Examples:
  gt handoff                          # Hand off current session
  gt handoff gt-abc                   # Hook bead, then restart
  gt handoff gt-abc -s "Fix it"       # Hook with context, then restart
  gt handoff -s "Context" -m "Notes"  # Hand off with custom message
  gt handoff crew                     # Hand off crew session
  gt handoff mayor                    # Hand off mayor session

Any molecule on the hook will be auto-continued by the new session.
The SessionStart hook runs 'gt prime' to restore context.`,
	RunE: runHandoff,
}

var (
	handoffWatch   bool
	handoffDryRun  bool
	handoffSubject string
	handoffMessage string
)

func init() {
	handoffCmd.Flags().BoolVarP(&handoffWatch, "watch", "w", true, "Switch to new session (for remote handoff)")
	handoffCmd.Flags().BoolVarP(&handoffDryRun, "dry-run", "n", false, "Show what would be done without executing")
	handoffCmd.Flags().StringVarP(&handoffSubject, "subject", "s", "", "Subject for handoff mail (optional)")
	handoffCmd.Flags().StringVarP(&handoffMessage, "message", "m", "", "Message body for handoff mail (optional)")
	rootCmd.AddCommand(handoffCmd)
}

func runHandoff(cmd *cobra.Command, args []string) error {
	// Check if we're a polecat - polecats use gt done instead
	// GT_POLECAT is set by the session manager when starting polecat sessions
	if polecatName := os.Getenv("GT_POLECAT"); polecatName != "" {
		fmt.Printf("%s Polecat detected (%s) - using gt done for handoff\n",
			style.Bold.Render("🐾"), polecatName)
		// Polecats don't respawn themselves - Witness handles lifecycle
		// Call gt done with DEFERRED exit type to preserve work state
		doneCmd := exec.Command("gt", "done", "--exit", "DEFERRED")
		doneCmd.Stdout = os.Stdout
		doneCmd.Stderr = os.Stderr
		return doneCmd.Run()
	}

	t := tmux.NewTmux()

	// Verify we're in tmux
	if !tmux.IsInsideTmux() {
		return fmt.Errorf("not running in tmux - cannot hand off")
	}

	pane := os.Getenv("TMUX_PANE")
	if pane == "" {
		return fmt.Errorf("TMUX_PANE not set - cannot hand off")
	}

	// Get current session name
	currentSession, err := getCurrentTmuxSession()
	if err != nil {
		return fmt.Errorf("getting session name: %w", err)
	}

	// Determine target session and check for bead hook
	targetSession := currentSession
	if len(args) > 0 {
		arg := args[0]

		// Check if arg is a bead ID (gt-xxx, hq-xxx, bd-xxx, etc.)
		if looksLikeBeadID(arg) {
			// Hook the bead first
			if err := hookBeadForHandoff(arg); err != nil {
				return fmt.Errorf("hooking bead: %w", err)
			}
			// Update subject if not set
			if handoffSubject == "" {
				handoffSubject = fmt.Sprintf("🪝 HOOKED: %s", arg)
			}
		} else {
			// User specified a role to hand off
			targetSession, err = resolveRoleToSession(arg)
			if err != nil {
				return fmt.Errorf("resolving role: %w", err)
			}
		}
	}

	// Build the restart command
	restartCmd, err := buildRestartCommand(targetSession)
	if err != nil {
		return err
	}

	// If handing off a different session, we need to find its pane and respawn there
	if targetSession != currentSession {
		return handoffRemoteSession(t, targetSession, restartCmd)
	}

	// Handing off ourselves - print feedback then respawn
	fmt.Printf("%s Handing off %s...\n", style.Bold.Render("🤝"), currentSession)

	// Dry run mode - show what would happen (BEFORE any side effects)
	if handoffDryRun {
		if handoffSubject != "" || handoffMessage != "" {
			fmt.Printf("Would send handoff mail: subject=%q\n", handoffSubject)
		}
		fmt.Printf("Would execute: tmux clear-history -t %s\n", pane)
		fmt.Printf("Would execute: tmux respawn-pane -k -t %s %s\n", pane, restartCmd)
		return nil
	}

	// If subject/message provided, send handoff mail to self first
	if handoffSubject != "" || handoffMessage != "" {
		if err := sendHandoffMail(handoffSubject, handoffMessage); err != nil {
			fmt.Printf("%s Warning: could not send handoff mail: %v\n", style.Dim.Render("⚠"), err)
			// Continue anyway - the respawn is more important
		} else {
			fmt.Printf("%s Sent handoff mail\n", style.Bold.Render("📬"))
		}
	}

	// Clear scrollback history before respawn (resets copy-mode from [0/N] to [0/0])
	if err := t.ClearHistory(pane); err != nil {
		// Non-fatal - continue with respawn even if clear fails
		fmt.Printf("%s Warning: could not clear history: %v\n", style.Dim.Render("⚠"), err)
	}

	// Use exec to respawn the pane - this kills us and restarts
	return t.RespawnPane(pane, restartCmd)
}

// getCurrentTmuxSession returns the current tmux session name.
func getCurrentTmuxSession() (string, error) {
	out, err := exec.Command("tmux", "display-message", "-p", "#{session_name}").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// resolveRoleToSession converts a role name or path to a tmux session name.
// Accepts:
//   - Role shortcuts: "crew", "witness", "refinery", "mayor", "deacon"
//   - Full paths: "<rig>/crew/<name>", "<rig>/witness", "<rig>/refinery"
//   - Direct session names (passed through)
//
// For role shortcuts that need context (crew, witness, refinery), it auto-detects from environment.
func resolveRoleToSession(role string) (string, error) {
	// First, check if it's a path format (contains /)
	if strings.Contains(role, "/") {
		return resolvePathToSession(role)
	}

	switch strings.ToLower(role) {
	case "mayor", "may":
		return "gt-mayor", nil

	case "deacon", "dea":
		return "gt-deacon", nil

	case "crew":
		// Try to get rig and crew name from environment or cwd
		rig := os.Getenv("GT_RIG")
		crewName := os.Getenv("GT_CREW")
		if rig == "" || crewName == "" {
			// Try to detect from cwd
			detected, err := detectCrewFromCwd()
			if err == nil {
				rig = detected.rigName
				crewName = detected.crewName
			}
		}
		if rig == "" || crewName == "" {
			return "", fmt.Errorf("cannot determine crew identity - run from crew directory or specify GT_RIG/GT_CREW")
		}
		return fmt.Sprintf("gt-%s-crew-%s", rig, crewName), nil

	case "witness", "wit":
		rig := os.Getenv("GT_RIG")
		if rig == "" {
			return "", fmt.Errorf("cannot determine rig - set GT_RIG or run from rig context")
		}
		return fmt.Sprintf("gt-%s-witness", rig), nil

	case "refinery", "ref":
		rig := os.Getenv("GT_RIG")
		if rig == "" {
			return "", fmt.Errorf("cannot determine rig - set GT_RIG or run from rig context")
		}
		return fmt.Sprintf("gt-%s-refinery", rig), nil

	default:
		// Assume it's a direct session name (e.g., gt-gastown-crew-max)
		return role, nil
	}
}

// resolvePathToSession converts a path like "<rig>/crew/<name>" to a session name.
// Supported formats:
//   - <rig>/crew/<name> -> gt-<rig>-crew-<name>
//   - <rig>/witness -> gt-<rig>-witness
//   - <rig>/refinery -> gt-<rig>-refinery
func resolvePathToSession(path string) (string, error) {
	parts := strings.Split(path, "/")

	// Handle <rig>/crew/<name> format
	if len(parts) == 3 && parts[1] == "crew" {
		rig := parts[0]
		name := parts[2]
		return fmt.Sprintf("gt-%s-crew-%s", rig, name), nil
	}

	// Handle <rig>/<role> format (witness, refinery)
	if len(parts) == 2 {
		rig := parts[0]
		role := strings.ToLower(parts[1])
		switch role {
		case "witness":
			return fmt.Sprintf("gt-%s-witness", rig), nil
		case "refinery":
			return fmt.Sprintf("gt-%s-refinery", rig), nil
		case "crew":
			// Just "<rig>/crew" without a name - need more info
			return "", fmt.Errorf("crew path requires name: %s/crew/<name>", rig)
		}
	}

	return "", fmt.Errorf("cannot parse path '%s' - expected <rig>/crew/<name>, <rig>/witness, or <rig>/refinery", path)
}

// buildRestartCommand creates the command to run when respawning a session's pane.
// This needs to be the actual command to execute (e.g., claude), not a session attach command.
// The command includes a cd to the correct working directory for the role.
func buildRestartCommand(sessionName string) (string, error) {
	// Detect town root from current directory
	townRoot := detectTownRootFromCwd()
	if townRoot == "" {
		return "", fmt.Errorf("cannot detect town root - run from within a Gas Town workspace")
	}

	// Determine the working directory for this session type
	workDir, err := sessionWorkDir(sessionName, townRoot)
	if err != nil {
		return "", err
	}

	// Determine GT_ROLE and BD_ACTOR values for this session
	gtRole := sessionToGTRole(sessionName)

	// For respawn-pane, we:
	// 1. cd to the right directory (role's canonical home)
	// 2. export GT_ROLE and BD_ACTOR so role detection works correctly
	// 3. run claude
	// The SessionStart hook will run gt prime.
	// Use exec to ensure clean process replacement.
	if gtRole != "" {
		return fmt.Sprintf("cd %s && export GT_ROLE=%s BD_ACTOR=%s && exec claude --dangerously-skip-permissions", workDir, gtRole, gtRole), nil
	}
	return fmt.Sprintf("cd %s && exec claude --dangerously-skip-permissions", workDir), nil
}

// sessionWorkDir returns the correct working directory for a session.
// This is the canonical home for each role type.
func sessionWorkDir(sessionName, townRoot string) (string, error) {
	switch {
	case sessionName == "gt-mayor":
		return townRoot, nil

	case sessionName == "gt-deacon":
		return townRoot + "/deacon", nil

	case strings.Contains(sessionName, "-crew-"):
		// gt-<rig>-crew-<name> -> <townRoot>/<rig>/crew/<name>
		parts := strings.Split(sessionName, "-")
		if len(parts) < 4 {
			return "", fmt.Errorf("invalid crew session name: %s", sessionName)
		}
		// Find the index of "crew" to split rig name (may contain dashes)
		for i, p := range parts {
			if p == "crew" && i > 1 && i < len(parts)-1 {
				rig := strings.Join(parts[1:i], "-")
				name := strings.Join(parts[i+1:], "-")
				return fmt.Sprintf("%s/%s/crew/%s", townRoot, rig, name), nil
			}
		}
		return "", fmt.Errorf("cannot parse crew session name: %s", sessionName)

	case strings.HasSuffix(sessionName, "-witness"):
		// gt-<rig>-witness -> <townRoot>/<rig>/witness/rig
		rig := strings.TrimPrefix(sessionName, "gt-")
		rig = strings.TrimSuffix(rig, "-witness")
		return fmt.Sprintf("%s/%s/witness/rig", townRoot, rig), nil

	case strings.HasSuffix(sessionName, "-refinery"):
		// gt-<rig>-refinery -> <townRoot>/<rig>/refinery/rig
		rig := strings.TrimPrefix(sessionName, "gt-")
		rig = strings.TrimSuffix(rig, "-refinery")
		return fmt.Sprintf("%s/%s/refinery/rig", townRoot, rig), nil

	default:
		return "", fmt.Errorf("unknown session type: %s (try specifying role explicitly)", sessionName)
	}
}

// sessionToGTRole converts a session name to a GT_ROLE value.
func sessionToGTRole(sessionName string) string {
	switch {
	case sessionName == "gt-mayor":
		return "mayor"
	case sessionName == "gt-deacon":
		return "deacon"
	case strings.Contains(sessionName, "-crew-"):
		// gt-<rig>-crew-<name> -> <rig>/crew/<name>
		parts := strings.Split(sessionName, "-")
		for i, p := range parts {
			if p == "crew" && i > 1 && i < len(parts)-1 {
				rig := strings.Join(parts[1:i], "-")
				name := strings.Join(parts[i+1:], "-")
				return fmt.Sprintf("%s/crew/%s", rig, name)
			}
		}
		return ""
	case strings.HasSuffix(sessionName, "-witness"):
		rig := strings.TrimPrefix(sessionName, "gt-")
		rig = strings.TrimSuffix(rig, "-witness")
		return fmt.Sprintf("%s/witness", rig)
	case strings.HasSuffix(sessionName, "-refinery"):
		rig := strings.TrimPrefix(sessionName, "gt-")
		rig = strings.TrimSuffix(rig, "-refinery")
		return fmt.Sprintf("%s/refinery", rig)
	default:
		return ""
	}
}

// detectTownRootFromCwd walks up from the current directory to find the town root.
func detectTownRootFromCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	dir := cwd
	for {
		// Check for primary marker (mayor/town.json)
		markerPath := filepath.Join(dir, "mayor", "town.json")
		if _, err := os.Stat(markerPath); err == nil {
			return dir
		}

		// Move up
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// handoffRemoteSession respawns a different session and optionally switches to it.
func handoffRemoteSession(t *tmux.Tmux, targetSession, restartCmd string) error {
	// Check if target session exists
	exists, err := t.HasSession(targetSession)
	if err != nil {
		return fmt.Errorf("checking session: %w", err)
	}
	if !exists {
		return fmt.Errorf("session '%s' not found - is the agent running?", targetSession)
	}

	// Get the pane ID for the target session
	targetPane, err := getSessionPane(targetSession)
	if err != nil {
		return fmt.Errorf("getting target pane: %w", err)
	}

	fmt.Printf("%s Handing off %s...\n", style.Bold.Render("🤝"), targetSession)

	// Dry run mode
	if handoffDryRun {
		fmt.Printf("Would execute: tmux clear-history -t %s\n", targetPane)
		fmt.Printf("Would execute: tmux respawn-pane -k -t %s %s\n", targetPane, restartCmd)
		if handoffWatch {
			fmt.Printf("Would execute: tmux switch-client -t %s\n", targetSession)
		}
		return nil
	}

	// Clear scrollback history before respawn (resets copy-mode from [0/N] to [0/0])
	if err := t.ClearHistory(targetPane); err != nil {
		// Non-fatal - continue with respawn even if clear fails
		fmt.Printf("%s Warning: could not clear history: %v\n", style.Dim.Render("⚠"), err)
	}

	// Respawn the remote session's pane
	if err := t.RespawnPane(targetPane, restartCmd); err != nil {
		return fmt.Errorf("respawning pane: %w", err)
	}

	// If --watch, switch to that session
	if handoffWatch {
		fmt.Printf("Switching to %s...\n", targetSession)
		// Use tmux switch-client to move our view to the target session
		if err := exec.Command("tmux", "switch-client", "-t", targetSession).Run(); err != nil {
			// Non-fatal - they can manually switch
			fmt.Printf("Note: Could not auto-switch (use: tmux switch-client -t %s)\n", targetSession)
		}
	}

	return nil
}

// getSessionPane returns the pane identifier for a session's main pane.
func getSessionPane(sessionName string) (string, error) {
	// Get the pane ID for the first pane in the session
	out, err := exec.Command("tmux", "list-panes", "-t", sessionName, "-F", "#{pane_id}").Output()
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return "", fmt.Errorf("no panes found in session")
	}
	return lines[0], nil
}

// sendHandoffMail sends a handoff mail to self using gt mail send.
func sendHandoffMail(subject, message string) error {
	// Build subject with handoff prefix if not already present
	if subject == "" {
		subject = "🤝 HANDOFF: Session cycling"
	} else if !strings.Contains(subject, "HANDOFF") {
		subject = "🤝 HANDOFF: " + subject
	}

	// Default message if not provided
	if message == "" {
		message = "Context cycling. Check bd ready for pending work."
	}

	// Use gt mail send to self (--self flag sends to current agent identity)
	cmd := exec.Command("gt", "mail", "send", "--self", "-s", subject, "-m", message)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// looksLikeBeadID checks if a string looks like a bead ID.
// Bead IDs have format: prefix-xxxx where prefix is 2+ letters and xxxx is alphanumeric.
func looksLikeBeadID(s string) bool {
	// Common bead prefixes
	prefixes := []string{"gt-", "hq-", "bd-", "beads-"}
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// hookBeadForHandoff attaches a bead to the current agent's hook.
func hookBeadForHandoff(beadID string) error {
	// Verify the bead exists first
	verifyCmd := exec.Command("bd", "show", beadID, "--json")
	if err := verifyCmd.Run(); err != nil {
		return fmt.Errorf("bead '%s' not found", beadID)
	}

	// Determine agent identity and clone root
	agentID, _, cloneRoot, err := resolveSelfTarget()
	if err != nil {
		return fmt.Errorf("detecting agent identity: %w", err)
	}

	// Create the slung work wisp
	sw := wisp.NewSlungWork(beadID, agentID)
	sw.Subject = handoffSubject
	sw.Context = handoffMessage

	fmt.Printf("%s Hooking %s...\n", style.Bold.Render("🪝"), beadID)

	if handoffDryRun {
		fmt.Printf("Would create wisp: %s\n", wisp.HookPath(cloneRoot, agentID))
		return nil
	}

	// Write the wisp to the hook
	if err := wisp.WriteSlungWork(cloneRoot, agentID, sw); err != nil {
		return fmt.Errorf("writing wisp: %w", err)
	}

	fmt.Printf("%s Work attached to hook\n", style.Bold.Render("✓"))
	return nil
}
