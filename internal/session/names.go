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

// --- SessionID to unique human-readable name translation ---
//
// SessionID is a logical identifier for a session (e.g., "mayor", "gastown/witness").
// The TownSessions layer translates this to unique, human-readable names (e.g., "hq-mayor-abc123").
// This keeps the logical namespace clean while ensuring no collisions across towns.
//
// The translation is bidirectional:
// - ToUniqueHumanReadableName: logical ID + townRoot → unique name (no collisions)
// - FromUniqueHumanReadableName: unique name + townRoot → logical ID + ownership check

// ToUniqueHumanReadableName translates a logical SessionID to a unique, human-readable name.
// Includes the town suffix for collision prevention in multi-town setups.
//
// Logical IDs use path format: "mayor", "deacon", "<rig>/witness", "<rig>/polecats/<name>", etc.
// Output names use the gt-/hq- prefix format with town suffix: "hq-mayor-abc123", "gt-gastown-witness-abc123".
//
// If townRoot is empty, no suffix is added (legacy/single-town mode).
func ToUniqueHumanReadableName(id SessionID, townRoot string) string {
	s := string(id)
	if s == "" {
		return ""
	}

	var baseName string

	// Town-level agents use hq- prefix
	if s == "mayor" {
		baseName = HQPrefix + "mayor"
	} else if s == "deacon" {
		baseName = HQPrefix + "deacon"
	} else if s == "boot" {
		// Boot is a special case - system service
		baseName = Prefix + "boot"
	} else {
		// Rig-level agents: <rig>/<role>[/<name>]
		parts := strings.Split(s, "/")
		if len(parts) < 2 {
			// Not a recognized format, return as-is (might be legacy)
			baseName = s
		} else {
			rig := parts[0]
			role := parts[1]

			switch role {
			case "witness":
				baseName = fmt.Sprintf("%s%s-witness", Prefix, rig)
			case "refinery":
				baseName = fmt.Sprintf("%s%s-refinery", Prefix, rig)
			case "polecats":
				if len(parts) < 3 {
					baseName = s // Invalid, return as-is
				} else {
					name := parts[2]
					baseName = fmt.Sprintf("%s%s-%s", Prefix, rig, name)
				}
			case "crew":
				if len(parts) < 3 {
					baseName = s // Invalid, return as-is
				} else {
					name := parts[2]
					baseName = fmt.Sprintf("%s%s-crew-%s", Prefix, rig, name)
				}
			default:
				// Unknown role, return as-is
				baseName = s
			}
		}
	}

	// Append town suffix for collision prevention
	return baseName + townSuffix(townRoot)
}

// FromUniqueHumanReadableName translates a unique name back to a logical SessionID.
// Also checks ownership - returns false if the session belongs to a different town.
//
// Parameters:
//   - uniqueName: the unique session name (e.g., "hq-mayor-abc123")
//   - townRoot: the caller's town root path (used to verify ownership)
//
// Returns:
//   - SessionID: the logical ID (e.g., "mayor"), empty if not owned
//   - bool: true if this session belongs to the caller's town (or is legacy)
//
// Legacy sessions (no town suffix) are considered owned for backwards compatibility.
func FromUniqueHumanReadableName(uniqueName string, townRoot string) (SessionID, bool) {
	// Check ownership first
	sessionTownID := ExtractTownID(uniqueName)
	expectedTownID := TownID(townRoot)

	if sessionTownID != "" && expectedTownID != "" && sessionTownID != expectedTownID {
		// Different town - not ours
		return "", false
	}

	// Legacy session (no suffix) or ours - proceed with translation
	baseName := StripTownID(uniqueName)

	// Town-level agents (HQ prefix)
	if baseName == HQPrefix+"mayor" || baseName == "gt-mayor" {
		return SessionID("mayor"), true
	}
	if baseName == HQPrefix+"deacon" || baseName == "gt-deacon" {
		return SessionID("deacon"), true
	}
	if baseName == Prefix+"boot" {
		return SessionID("boot"), true
	}

	// Must start with gt- for rig-level agents
	if !strings.HasPrefix(baseName, Prefix) {
		// Unknown format - return as-is but mark as owned (could be legacy)
		return SessionID(uniqueName), true
	}

	rest := strings.TrimPrefix(baseName, Prefix)
	parts := strings.Split(rest, "-")
	if len(parts) < 2 {
		return SessionID(uniqueName), true // Unknown format
	}

	rig := parts[0]

	// Check for known suffixes
	if parts[len(parts)-1] == "witness" && len(parts) == 2 {
		return SessionID(fmt.Sprintf("%s/witness", rig)), true
	}
	if parts[len(parts)-1] == "refinery" && len(parts) == 2 {
		return SessionID(fmt.Sprintf("%s/refinery", rig)), true
	}

	// Check for crew: gt-<rig>-crew-<name>
	if len(parts) >= 3 && parts[1] == "crew" {
		name := strings.Join(parts[2:], "-")
		return SessionID(fmt.Sprintf("%s/crew/%s", rig, name)), true
	}

	// Default: polecat gt-<rig>-<name>
	name := strings.Join(parts[1:], "-")
	return SessionID(fmt.Sprintf("%s/polecats/%s", rig, name)), true
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
