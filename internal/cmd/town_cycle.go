package cmd

import (
	"fmt"
	"os/exec"
	"sort"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/session"
)

// townCycleSession is the --session flag for town next/prev commands.
// When run via tmux key binding (run-shell), the session context may not be
// correct, so we pass the session name explicitly via #{session_name} expansion.
var townCycleSession string

// isTownLevelSession checks if the given session name is a town-level session.
// Town-level sessions (Mayor, Deacon) are identified by parsing the session name.
// This handles town ID suffixes correctly.
func isTownLevelSession(sessionName string) bool {
	identity, err := session.ParseSessionName(sessionName)
	if err != nil {
		return false
	}
	return identity.Role == session.RoleMayor || identity.Role == session.RoleDeacon
}

func init() {
	rootCmd.AddCommand(townCmd)
	townCmd.AddCommand(townNextCmd)
	townCmd.AddCommand(townPrevCmd)

	townNextCmd.Flags().StringVar(&townCycleSession, "session", "", "Override current session (used by tmux binding)")
	townPrevCmd.Flags().StringVar(&townCycleSession, "session", "", "Override current session (used by tmux binding)")
}

var townCmd = &cobra.Command{
	Use:   "town",
	Short: "Town-level operations",
	Long:  `Commands for town-level operations including session cycling.`,
}

var townNextCmd = &cobra.Command{
	Use:   "next",
	Short: "Switch to next town session (mayor/deacon)",
	Long: `Switch to the next town-level session in the cycle order.
Town sessions cycle between Mayor and Deacon.

This command is typically invoked via the C-b n keybinding when in a
town-level session (Mayor or Deacon).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cycleTownSession(1, townCycleSession)
	},
}

var townPrevCmd = &cobra.Command{
	Use:   "prev",
	Short: "Switch to previous town session (mayor/deacon)",
	Long: `Switch to the previous town-level session in the cycle order.
Town sessions cycle between Mayor and Deacon.

This command is typically invoked via the C-b p keybinding when in a
town-level session (Mayor or Deacon).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cycleTownSession(-1, townCycleSession)
	},
}

// cycleTownSession switches to the next or previous town-level session.
// direction: 1 for next, -1 for previous
// sessionOverride: if non-empty, use this instead of detecting current session
func cycleTownSession(direction int, sessionOverride string) error {
	var currentSession string
	var err error

	if sessionOverride != "" {
		currentSession = sessionOverride
	} else {
		currentSession, err = getCurrentTmuxSession()
		if err != nil {
			return fmt.Errorf("not in a tmux session: %w", err)
		}
		if currentSession == "" {
			return fmt.Errorf("not in a tmux session")
		}
	}

	// Parse current session to get town ID
	currentIdentity, err := session.ParseSessionName(currentSession)
	if err != nil || (currentIdentity.Role != session.RoleMayor && currentIdentity.Role != session.RoleDeacon) {
		// Not a town session - no cycling, just stay put
		return nil
	}

	// Find running town sessions in the same town
	sessions, err := findRunningTownSessions(currentIdentity.TownID)
	if err != nil {
		return fmt.Errorf("listing sessions: %w", err)
	}

	if len(sessions) == 0 {
		return fmt.Errorf("no town sessions found")
	}

	// Sort for consistent ordering
	sort.Strings(sessions)

	// Find current position
	currentIdx := -1
	for i, s := range sessions {
		if s == currentSession {
			currentIdx = i
			break
		}
	}

	if currentIdx == -1 {
		// Current session not in list (shouldn't happen)
		return fmt.Errorf("current session not found in town session list")
	}

	// Calculate target index (with wrapping)
	targetIdx := (currentIdx + direction + len(sessions)) % len(sessions)

	if targetIdx == currentIdx {
		// Only one session, nothing to switch to
		return nil
	}

	targetSession := sessions[targetIdx]

	// Switch to target session
	cmd := exec.Command("tmux", "switch-client", "-t", targetSession)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("switching to %s: %w", targetSession, err)
	}

	return nil
}

// findRunningTownSessions returns a list of currently running town-level sessions.
// townID filters to sessions in the same town (empty matches any).
func findRunningTownSessions(townID string) ([]string, error) {
	// Get all tmux sessions
	out, err := exec.Command("tmux", "list-sessions", "-F", "#{session_name}").Output()
	if err != nil {
		return nil, fmt.Errorf("listing tmux sessions: %w", err)
	}

	var running []string
	for _, line := range splitLines(string(out)) {
		if line == "" {
			continue
		}
		// Parse session name to check if it's a town-level session
		identity, err := session.ParseSessionName(line)
		if err != nil {
			continue
		}
		// Match: mayor or deacon, same town (or legacy with no town ID)
		if (identity.Role == session.RoleMayor || identity.Role == session.RoleDeacon) &&
			(townID == "" || identity.TownID == "" || identity.TownID == townID) {
			running = append(running, line)
		}
	}

	return running, nil
}
