// Package agent provides the Agents manager for agent processes.
package agent

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/steveyegge/gastown/internal/session"
	"github.com/steveyegge/gastown/internal/tmux"
)

// AgentID is the logical address of an agent.
// Format: "role" for town-level, "rig/role" for rig-level, "rig/role/name" for named.
// Examples: "mayor", "deacon", "myrig/witness", "myrig/polecat/toast"
type AgentID string

// String returns the address as a string.
func (id AgentID) String() string {
	return string(id)
}

// Parse extracts role, rig, and worker from the AgentID.
// Returns (role, rig, worker) where:
//   - Town-level ("mayor"): role="mayor", rig="", worker=""
//   - Rig-level ("rig/witness"): role="witness", rig="rig", worker=""
//   - Named ("rig/crew/name"): role="crew", rig="rig", worker="name"
func (id AgentID) Parse() (role, rig, worker string) {
	parts := strings.Split(string(id), "/")
	switch len(parts) {
	case 1:
		return parts[0], "", ""
	case 2:
		return parts[1], parts[0], ""
	case 3:
		return parts[1], parts[0], parts[2]
	default:
		return "unknown", "", ""
	}
}

// Role returns the role component of the AgentID.
func (id AgentID) Role() string {
	role, _, _ := id.Parse()
	return role
}

// Rig returns the rig component of the AgentID (empty for town-level agents).
func (id AgentID) Rig() string {
	_, rig, _ := id.Parse()
	return rig
}

// Worker returns the worker/name component of the AgentID (empty for non-named agents).
func (id AgentID) Worker() string {
	_, _, worker := id.Parse()
	return worker
}

// --- Address constructors for convenience ---

// MayorAddress returns the address for the mayor agent.
func MayorAddress() AgentID {
	return "mayor"
}

// DeaconAddress returns the address for the deacon agent.
func DeaconAddress() AgentID {
	return "deacon"
}

// BootAddress returns the address for the boot watchdog agent.
func BootAddress() AgentID {
	return "boot"
}

// WitnessAddress returns the address for a rig's witness.
func WitnessAddress(rig string) AgentID {
	return AgentID(fmt.Sprintf("%s/witness", rig))
}

// RefineryAddress returns the address for a rig's refinery.
func RefineryAddress(rig string) AgentID {
	return AgentID(fmt.Sprintf("%s/refinery", rig))
}

// PolecatAddress returns the address for a polecat.
func PolecatAddress(rig, name string) AgentID {
	return AgentID(fmt.Sprintf("%s/polecat/%s", rig, name))
}

// CrewAddress returns the address for a crew member.
func CrewAddress(rig, name string) AgentID {
	return AgentID(fmt.Sprintf("%s/crew/%s", rig, name))
}

// ErrUnknownRole is returned when the agent role cannot be determined.
var ErrUnknownRole = errors.New("unknown or missing GT_ROLE")

// Self returns the AgentID for the current process based on GT_* environment variables.
// This allows an agent to identify itself without querying tmux.
//
// Required env vars by role:
//   - mayor: GT_ROLE=mayor
//   - deacon: GT_ROLE=deacon
//   - witness: GT_ROLE=witness, GT_RIG
//   - refinery: GT_ROLE=refinery, GT_RIG
//   - crew: GT_ROLE=crew, GT_RIG, GT_CREW
//   - polecat: GT_ROLE=polecat, GT_RIG, GT_POLECAT
func Self() (AgentID, error) {
	role := os.Getenv("GT_ROLE")
	rig := os.Getenv("GT_RIG")

	switch role {
	case "mayor":
		return MayorAddress(), nil
	case "deacon":
		return DeaconAddress(), nil
	case "boot":
		return BootAddress(), nil
	case "witness":
		if rig == "" {
			return "", fmt.Errorf("%w: witness requires GT_RIG", ErrUnknownRole)
		}
		return WitnessAddress(rig), nil
	case "refinery":
		if rig == "" {
			return "", fmt.Errorf("%w: refinery requires GT_RIG", ErrUnknownRole)
		}
		return RefineryAddress(rig), nil
	case "crew":
		name := os.Getenv("GT_CREW")
		if rig == "" || name == "" {
			return "", fmt.Errorf("%w: crew requires GT_RIG and GT_CREW", ErrUnknownRole)
		}
		return CrewAddress(rig, name), nil
	case "polecat":
		name := os.Getenv("GT_POLECAT")
		if rig == "" || name == "" {
			return "", fmt.Errorf("%w: polecat requires GT_RIG and GT_POLECAT", ErrUnknownRole)
		}
		return PolecatAddress(rig, name), nil
	case "":
		return "", ErrUnknownRole
	default:
		return "", fmt.Errorf("%w: %s", ErrUnknownRole, role)
	}
}

// ErrAlreadyRunning is returned when trying to start an already running agent.
var ErrAlreadyRunning = errors.New("agent already running")

// ErrNotRunning is returned when trying to operate on a non-running agent.
var ErrNotRunning = errors.New("agent not running")

// Agents is the interface for managing agent processes.
// Implementations handle lifecycle, readiness detection, and session management.
type Agents interface {
	// Start launches an agent process in a new session.
	Start(id AgentID, workDir, command string) error

	// Stop terminates an agent process.
	Stop(id AgentID, graceful bool) error

	// Respawn atomically kills the agent process and starts a new one.
	// Clears scrollback history before respawning for a clean start.
	// This is used for handoff - an agent can respawn itself or another agent.
	// Unlike Stop+Start, this is atomic and works for self-handoff.
	// The original start command (including env vars and beacon) is reused.
	Respawn(id AgentID) error

	// Exists checks if an agent is running (session exists AND process is alive).
	// Returns false for zombie sessions (tmux exists but agent process died).
	Exists(id AgentID) bool

	// WaitReady blocks until the agent is ready for input or times out.
	WaitReady(id AgentID) error

	// GetInfo returns information about an agent's session.
	GetInfo(id AgentID) (*session.Info, error)

	// Nudge sends a message to a running agent reliably.
	// Uses robust delivery (handles vim mode, retries) for inter-agent communication.
	Nudge(id AgentID, message string) error

	// Capture returns the recent output from an agent's session.
	Capture(id AgentID, lines int) (string, error)

	// CaptureAll returns the entire scrollback history from an agent's session.
	CaptureAll(id AgentID) (string, error)

	// List returns all agent addresses.
	List() ([]AgentID, error)

	// Attach attaches to a running agent's session (exec into terminal).
	Attach(id AgentID) error
}

// Implementation is the concrete implementation of the Agents interface.
// It provides runtime-aware lifecycle management.
type Implementation struct {
	sess   session.Sessions
	config *Config
}

// Ensure Implementation implements Agents
var _ Agents = (*Implementation)(nil)

// New creates a new Agents implementation.
// The Sessions handles address-to-session mapping (typically TownSessions with town-specific hashing).
func New(sess session.Sessions, config *Config) *Implementation {
	if config == nil {
		config = Claude()
	}
	return &Implementation{
		sess:   sess,
		config: config,
	}
}

// timeout returns the effective timeout for readiness detection.
func (a *Implementation) timeout() time.Duration {
	if a.config.Timeout > 0 {
		return a.config.Timeout
	}
	return 30 * time.Second // Default fallback
}

// prependEnvVars prepends environment variables to a command.
// Returns a command like "VAR1=val1 VAR2=val2 original-command".
// Precondition: envVars is non-empty (caller checks before calling).
func prependEnvVars(envVars map[string]string, command string) string {
	// Sort keys for deterministic output
	keys := make([]string, 0, len(envVars))
	for k := range envVars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, envVars[k]))
	}

	return strings.Join(parts, " ") + " " + command
}

// Start launches an agent process in a new session.
// Handles zombie detection: if a session exists but the agent process is dead, it cleans up first.
// If EnvVars are configured, they are prepended to the command.
func (a *Implementation) Start(id AgentID, workDir, command string) error {
	sessionID := session.SessionID(id)

	// Check for existing session and handle zombie detection
	exists, _ := a.sess.Exists(sessionID)
	if exists {
		// Session exists - check if agent is actually running (healthy vs zombie)
		if a.sess.IsRunning(sessionID, a.config.ProcessNames...) {
			return ErrAlreadyRunning
		}
		// Zombie - session alive but agent dead. Kill and recreate.
		if err := a.sess.Stop(sessionID); err != nil {
			return fmt.Errorf("killing zombie session: %w", err)
		}
	}

	// Prepend env vars to command if configured
	if len(a.config.EnvVars) > 0 {
		command = prependEnvVars(a.config.EnvVars, command)
	}

	// Create the session
	if _, err := a.sess.Start(string(id), workDir, command); err != nil {
		return fmt.Errorf("starting session: %w", err)
	}

	// Run post-creation callback (theming, env vars, etc.)
	if a.config.OnSessionCreated != nil {
		if err := a.config.OnSessionCreated(a.sess, sessionID); err != nil {
			// Cleanup session on callback failure
			_ = a.sess.Stop(sessionID)
			return fmt.Errorf("session setup: %w", err)
		}
	}

	// Wait for agent to be ready (non-blocking)
	go a.doWaitForReady(id)

	return nil
}

// doWaitForReady implements the readiness wait logic.
func (a *Implementation) doWaitForReady(id AgentID) error {
	sessionID := session.SessionID(id)

	// Run startup hook if defined (e.g., dismiss dialogs)
	if a.config.StartupHook != nil {
		_ = a.config.StartupHook(a.sess, sessionID) // Non-fatal
	}

	// Use checker if available
	if a.config.Checker != nil {
		return a.waitForReady(sessionID, a.timeout(), a.config.Checker)
	}

	// Fall back to startup delay
	if a.config.StartupDelay > 0 {
		time.Sleep(a.config.StartupDelay)
	}

	return nil
}

// waitForReady polls until the agent is ready or times out.
func (a *Implementation) waitForReady(id session.SessionID, timeout time.Duration, checker ReadinessChecker) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		output, err := a.sess.Capture(id, 50)
		if err == nil && checker.IsReady(output) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for agent ready")
}

// WaitReady blocks until the agent is ready for input or times out.
func (a *Implementation) WaitReady(id AgentID) error {
	sessionID := session.SessionID(id)
	exists, _ := a.sess.Exists(sessionID)
	if !exists {
		return ErrNotRunning
	}
	return a.doWaitForReady(id)
}

// Stop terminates an agent process.
func (a *Implementation) Stop(id AgentID, graceful bool) error {
	sessionID := session.SessionID(id)
	exists, _ := a.sess.Exists(sessionID)
	if !exists {
		return nil // Idempotent - nothing to stop
	}

	if graceful {
		_ = a.sess.SendControl(sessionID, "C-c")
		time.Sleep(100 * time.Millisecond)
	}

	if err := a.sess.Stop(sessionID); err != nil {
		return fmt.Errorf("stopping session: %w", err)
	}

	return nil
}

// Respawn atomically kills the agent process and starts a new one.
// Clears scrollback history before respawning for a clean start.
// This is used for handoff - an agent can respawn itself or another agent.
//
// The original command (including all env vars and beacon) is reused.
// Work discovery happens through hooks, not through the beacon.
//
// For self-handoff: sess.Respawn() terminates the calling process, so nothing
// after that call executes. The new process starts fresh.
//
// For remote handoff: the caller survives, so we launch the readiness wait.
func (a *Implementation) Respawn(id AgentID) error {
	sessionID := session.SessionID(id)
	exists, _ := a.sess.Exists(sessionID)
	if !exists {
		return ErrNotRunning
	}

	// Get the original start command (includes all env vars and beacon)
	command, err := a.sess.GetStartCommand(sessionID)
	if err != nil {
		return fmt.Errorf("getting start command: %w", err)
	}

	// For self-handoff, this call terminates the current process.
	// For remote handoff, execution continues below.
	if err := a.sess.Respawn(sessionID, command); err != nil {
		return fmt.Errorf("respawning session: %w", err)
	}

	// Only reached for remote handoff (respawning a different agent)
	go a.doWaitForReady(id)

	return nil
}

// Exists checks if an agent is running (session exists AND process is alive).
// Returns false for zombie sessions (tmux exists but agent process died).
// If ProcessNames is not configured, falls back to session existence only.
func (a *Implementation) Exists(id AgentID) bool {
	sessionID := session.SessionID(id)
	exists, _ := a.sess.Exists(sessionID)
	if !exists {
		return false
	}
	// If no process names configured, can't check - assume session exists = agent exists
	if len(a.config.ProcessNames) == 0 {
		return true
	}
	// Check if agent process is actually running within the session
	return a.sess.IsRunning(sessionID, a.config.ProcessNames...)
}

// GetInfo returns information about an agent's session.
func (a *Implementation) GetInfo(id AgentID) (*session.Info, error) {
	return a.sess.GetInfo(session.SessionID(id))
}

// Nudge sends a message to a running agent reliably.
func (a *Implementation) Nudge(id AgentID, message string) error {
	return a.sess.Nudge(session.SessionID(id), message)
}

// Capture returns the recent output from an agent's session.
func (a *Implementation) Capture(id AgentID, lines int) (string, error) {
	return a.sess.Capture(session.SessionID(id), lines)
}

// CaptureAll returns the entire scrollback history from an agent's session.
func (a *Implementation) CaptureAll(id AgentID) (string, error) {
	return a.sess.CaptureAll(session.SessionID(id))
}

// List returns all agent addresses.
func (a *Implementation) List() ([]AgentID, error) {
	sessionIDs, err := a.sess.List()
	if err != nil {
		return nil, err
	}
	ids := make([]AgentID, 0, len(sessionIDs))
	for _, sid := range sessionIDs {
		// If no process names configured, include all sessions
		// Otherwise only include if process is running (not zombie)
		if len(a.config.ProcessNames) == 0 || a.sess.IsRunning(sid, a.config.ProcessNames...) {
			ids = append(ids, AgentID(sid))
		}
	}
	return ids, nil
}

// Attach attaches to a running agent's session.
// Smart context detection:
//   - Inside tmux → switch-client (no-op if already in target)
//   - Outside tmux → blocking attach
func (a *Implementation) Attach(id AgentID) error {
	sessionID := session.SessionID(id)

	// Try to switch (works if inside tmux, no-op if already in target)
	if err := a.sess.SwitchTo(sessionID); err == nil {
		return nil
	}

	// Not inside tmux - do a blocking attach
	return a.sess.Attach(sessionID)
}

// =============================================================================
// Factory Functions
// =============================================================================

// ForTown creates an Agents interface for the given town root path.
func ForTown(townRoot string) Agents {
	t := tmux.NewTmux()
	sess := session.NewTownSessions(t, townRoot)
	return New(sess, nil)
}

// ForTownPath is an alias for ForTown.
func ForTownPath(townRoot string) Agents {
	return ForTown(townRoot)
}

// ForTownWithConfig creates an Agents interface with the specified config.
// Use Claude() for Claude-specific behavior (zombie filtering, readiness).
func ForTownWithConfig(townRoot string, cfg *Config) Agents {
	t := tmux.NewTmux()
	sess := session.NewTownSessions(t, townRoot)
	return New(sess, cfg)
}
