// Package session provides polecat session lifecycle management.
package session

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ErrSessionNotFound is returned when a session cannot be found.
var ErrSessionNotFound = errors.New("session not found")

// Prefix is the common prefix for rig-level Gas Town tmux sessions.
const Prefix = "gt-"

// HQPrefix is the prefix for town-level services (Mayor, Deacon).
const HQPrefix = "hq-"

// TownIDLength is the number of characters in a town ID suffix.
const TownIDLength = 6

// townIDPattern matches a valid town ID suffix (hyphen + 6 hex chars at end).
var townIDPattern = regexp.MustCompile(`-([0-9a-f]{6})$`)

// TownID generates a 6-character hex identifier from the town root path.
// This provides collision resistance for multi-town setups on the same machine.
// Returns empty string if townRoot is empty (backwards compatibility mode).
func TownID(townRoot string) string {
	if townRoot == "" {
		return ""
	}
	h := sha256.Sum256([]byte(townRoot))
	return hex.EncodeToString(h[:])[:TownIDLength]
}

// townSuffix returns the town ID suffix (e.g., "-x7k2f9") or empty string.
func townSuffix(townRoot string) string {
	id := TownID(townRoot)
	if id == "" {
		return ""
	}
	return "-" + id
}

// MayorSessionName returns the session name for the Mayor agent.
// Use with TownSessions which handles town suffix automatically.
func MayorSessionName() string {
	return HQPrefix + "mayor"
}

// DeaconSessionName returns the session name for the Deacon agent.
// Use with TownSessions which handles town suffix automatically.
func DeaconSessionName() string {
	return HQPrefix + "deacon"
}

// BootSessionName is the session name for the Boot watchdog.
// Boot uses the "gt-" prefix since it's a system service, not HQ.
const BootSessionName = Prefix + "boot"

// WitnessSessionName returns the session name for a rig's Witness agent.
// Use with TownSessions which handles town suffix automatically.
func WitnessSessionName(rig string) string {
	return fmt.Sprintf("%s%s-witness", Prefix, rig)
}

// RefinerySessionName returns the session name for a rig's Refinery agent.
// Use with TownSessions which handles town suffix automatically.
func RefinerySessionName(rig string) string {
	return fmt.Sprintf("%s%s-refinery", Prefix, rig)
}

// CrewSessionName returns the session name for a crew worker in a rig.
// Use with TownSessions which handles town suffix automatically.
func CrewSessionName(rig, name string) string {
	return fmt.Sprintf("%s%s-crew-%s", Prefix, rig, name)
}

// PolecatSessionName returns the session name for a polecat in a rig.
// Use with TownSessions which handles town suffix automatically.
func PolecatSessionName(rig, name string) string {
	return fmt.Sprintf("%s%s-%s", Prefix, rig, name)
}

// --- Session name parsing utilities ---

// ExtractTownID extracts the town ID suffix from a session name.
// Returns empty string if no town ID is present (legacy session).
func ExtractTownID(sessionName string) string {
	matches := townIDPattern.FindStringSubmatch(sessionName)
	if len(matches) == 2 {
		return matches[1]
	}
	return ""
}

// StripTownID removes the town ID suffix from a session name.
// Returns the original name if no town ID is present.
func StripTownID(sessionName string) string {
	return townIDPattern.ReplaceAllString(sessionName, "")
}

// MatchesTown checks if a session name matches a given town root.
// Returns true if:
// - The session has a town ID suffix matching the given town root, OR
// - The session has no town ID suffix (legacy session, backwards compatible)
func MatchesTown(sessionName, townRoot string) bool {
	sessionTownID := ExtractTownID(sessionName)
	if sessionTownID == "" {
		// Legacy session without town ID - assume it matches (single-town mode)
		return true
	}
	expectedTownID := TownID(townRoot)
	return sessionTownID == expectedTownID
}

// FilterSessionsByTown filters a list of session names to only those matching the given town.
// If townRoot is empty, all sessions are returned (backwards compatibility).
// Sessions with a different town ID suffix are excluded.
// Legacy sessions (no suffix) are included for backwards compatibility.
func FilterSessionsByTown(sessions []string, townRoot string) []string {
	if townRoot == "" {
		// No filtering when town root is not specified
		return sessions
	}

	var filtered []string
	for _, s := range sessions {
		if MatchesTown(s, townRoot) {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

// PropulsionNudge generates the GUPP (Gas Town Universal Propulsion Principle) nudge.
// This is sent after the beacon to trigger autonomous work execution.
// The agent receives this as user input, triggering the propulsion principle:
// "If work is on your hook, YOU RUN IT."
func PropulsionNudge() string {
	return "Run `gt hook` to check your hook and begin work."
}

// PropulsionNudgeForRole generates a role-specific GUPP nudge.
// Different roles have different startup flows:
// - polecat/crew: Check hook for slung work
// - witness/refinery: Start patrol cycle
// - deacon: Start heartbeat patrol
// - mayor: Check mail for coordination work
//
// The workDir parameter is used to locate .runtime/session_id for including
// session ID in the message (for Claude Code /resume picker discovery).
func PropulsionNudgeForRole(role, workDir string) string {
	var msg string
	switch role {
	case "polecat", "crew":
		msg = PropulsionNudge()
	case "witness":
		msg = "Run `gt prime` to check patrol status and begin work."
	case "refinery":
		msg = "Run `gt prime` to check MQ status and begin patrol."
	case "deacon":
		msg = "Run `gt prime` to check patrol status and begin heartbeat cycle."
	case "mayor":
		msg = "Run `gt prime` to check mail and begin coordination."
	default:
		msg = PropulsionNudge()
	}

	// Append session ID if available (for /resume picker visibility)
	if sessionID := readSessionID(workDir); sessionID != "" {
		msg = fmt.Sprintf("%s [session:%s]", msg, sessionID)
	}
	return msg
}

// readSessionID reads the session ID from .runtime/session_id if it exists.
// Returns empty string if the file doesn't exist or can't be read.
func readSessionID(workDir string) string {
	if workDir == "" {
		return ""
	}
	sessionPath := filepath.Join(workDir, ".runtime", "session_id")
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
