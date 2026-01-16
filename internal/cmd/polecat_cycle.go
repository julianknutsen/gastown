package cmd

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/steveyegge/gastown/internal/session"
)

// cyclePolecatSession switches to the next or previous polecat session in the same rig.
// direction: 1 for next, -1 for previous
// sessionOverride: if non-empty, use this instead of detecting current session
func cyclePolecatSession(direction int, sessionOverride string) error {
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

	// Parse session to get rig and town ID
	currentIdentity, err := session.ParseSessionName(currentSession)
	if err != nil || currentIdentity.Role != session.RolePolecat {
		// Not a polecat session - no cycling
		return nil
	}

	// Find all polecat sessions for this rig in the same town
	sessions, err := findRigPolecatSessions(currentIdentity.Rig, currentIdentity.TownID)
	if err != nil {
		return fmt.Errorf("listing sessions: %w", err)
	}

	if len(sessions) == 0 {
		return nil // No polecat sessions
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
		return nil
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

// findRigPolecatSessions returns all polecat sessions for a given rig.
// townID filters to sessions in the same town (empty matches any).
func findRigPolecatSessions(rigName, townID string) ([]string, error) { //nolint:unparam // error return kept for future use
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}")
	out, err := cmd.Output()
	if err != nil {
		// No tmux server or no sessions
		return nil, nil
	}

	var sessions []string

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		// Parse session name to check role, rig, and town
		identity, err := session.ParseSessionName(line)
		if err != nil {
			continue
		}
		// Match: polecat role, same rig, same town (or legacy with no town ID)
		if identity.Role == session.RolePolecat &&
			identity.Rig == rigName &&
			(townID == "" || identity.TownID == "" || identity.TownID == townID) {
			sessions = append(sessions, line)
		}
	}

	return sessions, nil
}
