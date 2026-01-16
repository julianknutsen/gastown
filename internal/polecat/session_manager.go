// Package polecat provides polecat workspace and session management.
package polecat

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/steveyegge/gastown/internal/agent"
	"github.com/steveyegge/gastown/internal/config"
	"github.com/steveyegge/gastown/internal/rig"
	"github.com/steveyegge/gastown/internal/runtime"
)

// Session errors
var (
	ErrSessionRunning  = agent.ErrAlreadyRunning
	ErrSessionNotFound = errors.New("session not found")
)

// SessionManager handles polecat session lifecycle.
// Uses agent.Agents for all operations (same pattern as mayor/deacon/witness/refinery).
type SessionManager struct {
	agents    agent.Agents
	rig       *rig.Rig
	agentName string

	// startingPolecatName holds the polecat name for the current Start() call.
	// Used by factory callbacks to access the polecat name during session creation.
	startingPolecatName string
}

// NewSessionManager creates a new polecat session manager for a rig.
// agents is the Agents instance for all agent operations.
// agentName is the resolved agent to use (from config.ResolveRoleAgentName or command line).
func NewSessionManager(agents agent.Agents, r *rig.Rig, agentName string) *SessionManager {
	return &SessionManager{
		agents:    agents,
		rig:       r,
		agentName: agentName,
	}
}

// StartingPolecatName returns the name of the polecat currently being started.
// This is only valid during a Start() call and is used by factory callbacks.
func (m *SessionManager) StartingPolecatName() string {
	return m.startingPolecatName
}

// RigName returns the rig name for this manager.
func (m *SessionManager) RigName() string {
	return m.rig.Name
}

// envConfig returns the environment configuration for the given polecat.
func (m *SessionManager) envConfig(polecat string) config.AgentEnvConfig {
	return config.AgentEnvConfig{
		Role:          "polecat",
		Rig:           m.rig.Name,
		AgentName:     polecat,
		TownRoot:      filepath.Dir(m.rig.Path),
		BeadsNoDaemon: true,
	}
}

// SessionInfo contains information about a running polecat session.
type SessionInfo struct {
	// Polecat is the polecat name.
	Polecat string `json:"polecat"`

	// SessionID is the tmux session identifier.
	SessionID string `json:"session_id"`

	// Running indicates if the session is currently active.
	Running bool `json:"running"`

	// RigName is the rig this session belongs to.
	RigName string `json:"rig_name"`

	// Attached indicates if someone is attached to the session.
	Attached bool `json:"attached,omitempty"`

	// Created is when the session was created.
	Created time.Time `json:"created,omitempty"`

	// Windows is the number of tmux windows.
	Windows int `json:"windows,omitempty"`

	// LastActivity is when the session last had activity.
	LastActivity time.Time `json:"last_activity,omitempty"`
}

// SessionName generates the tmux session name for a polecat.
func (m *SessionManager) SessionName(polecat string) string {
	return fmt.Sprintf("gt-%s-%s", m.rig.Name, polecat)
}

// agentID returns the AgentID for a polecat's session.
func (m *SessionManager) agentID(polecat string) agent.AgentID {
	return agent.PolecatAddress(m.rig.Name, polecat)
}

// polecatDir returns the parent directory for a polecat.
// This is polecats/<name>/ - the polecat's home directory.
func (m *SessionManager) polecatDir(polecat string) string {
	return filepath.Join(m.rig.Path, "polecats", polecat)
}

// clonePath returns the path where the git worktree lives.
// New structure: polecats/<name>/<rigname>/ - gives LLMs recognizable repo context.
// Falls back to old structure: polecats/<name>/ for backward compatibility.
func (m *SessionManager) clonePath(polecat string) string {
	// New structure: polecats/<name>/<rigname>/
	newPath := filepath.Join(m.rig.Path, "polecats", polecat, m.rig.Name)
	if info, err := os.Stat(newPath); err == nil && info.IsDir() {
		return newPath
	}

	// Old structure: polecats/<name>/ (backward compat)
	oldPath := filepath.Join(m.rig.Path, "polecats", polecat)
	if info, err := os.Stat(oldPath); err == nil && info.IsDir() {
		// Check if this is actually a git worktree (has .git file or dir)
		gitPath := filepath.Join(oldPath, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			return oldPath
		}
	}

	// Default to new structure for new polecats
	return newPath
}

// hasPolecat checks if the polecat exists in this rig.
func (m *SessionManager) hasPolecat(polecat string) bool {
	polecatPath := m.polecatDir(polecat)
	info, err := os.Stat(polecatPath)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// Start creates and starts a new session for a polecat.
func (m *SessionManager) Start(polecat string) error {
	if !m.hasPolecat(polecat) {
		return fmt.Errorf("%w: %s", ErrPolecatNotFound, polecat)
	}

	sessionName := m.SessionName(polecat)
	workDir := m.clonePath(polecat)

	runtimeConfig := config.LoadRuntimeConfig(m.rig.Path)

	// Ensure runtime settings exist INSIDE the worktree so Claude Code can find them.
	// Claude Code does NOT traverse parent directories for settings.json, only for CLAUDE.md.
	// See: https://github.com/anthropics/claude-code/issues/12962
	if err := runtime.EnsureSettingsForRole(workDir, "polecat", runtimeConfig); err != nil {
		return fmt.Errorf("ensuring runtime settings: %w", err)
	}

	// Build startup command with env vars baked in
	command := config.PrependEnv(config.BuildAgentCommand(m.agentName, ""), config.AgentEnv(m.envConfig(polecat)))

	// Store polecat name for factory callback (OnSessionCreated)
	m.startingPolecatName = polecat

	// Create session via agent.Agents (handles zombie detection, theming via callback)
	if err := m.agents.Start(m.agentID(polecat), workDir, command); err != nil {
		if errors.Is(err, agent.ErrAlreadyRunning) {
			return fmt.Errorf("%w: %s", ErrSessionRunning, sessionName)
		}
		return fmt.Errorf("creating session: %w", err)
	}

	return nil
}

// Stop terminates a polecat session.
// Returns ErrSessionNotFound if the agent was not running (for user messaging).
// Always cleans up zombie sessions (tmux exists but process dead).
func (m *SessionManager) Stop(polecat string, force bool) error {
	id := m.agentID(polecat)
	wasRunning := m.agents.Exists(id)

	// Sync beads before shutdown (non-fatal) - only if actually running
	if wasRunning && !force {
		polecatDir := m.polecatDir(polecat)
		if err := m.syncBeads(polecatDir); err != nil {
			fmt.Printf("Warning: beads sync failed: %v\n", err)
		}
	}

	// Always call Stop to clean up zombies (tmux session without process)
	if err := m.agents.Stop(id, !force); err != nil {
		return fmt.Errorf("stopping session: %w", err)
	}

	if !wasRunning {
		return ErrSessionNotFound
	}
	return nil
}

// syncBeads runs bd sync in the given directory.
func (m *SessionManager) syncBeads(workDir string) error {
	cmd := exec.Command("bd", "sync")
	cmd.Dir = workDir
	return cmd.Run()
}

// IsRunning checks if a polecat session is active.
func (m *SessionManager) IsRunning(polecat string) (bool, error) {
	return m.agents.Exists(m.agentID(polecat)), nil
}

// Status returns detailed status for a polecat session.
func (m *SessionManager) Status(polecat string) (*SessionInfo, error) {
	sessionName := m.SessionName(polecat)
	id := m.agentID(polecat)

	running := m.agents.Exists(id)

	info := &SessionInfo{
		Polecat:   polecat,
		SessionID: sessionName,
		Running:   running,
		RigName:   m.rig.Name,
	}

	if !running {
		return info, nil
	}

	tmuxInfo, err := m.agents.GetInfo(id)
	if err != nil {
		return info, nil
	}

	info.Attached = tmuxInfo.Attached
	info.Windows = tmuxInfo.Windows

	if tmuxInfo.Created != "" {
		formats := []string{
			"Mon Jan 2 15:04:05 2006",
			"Mon Jan _2 15:04:05 2006",
			time.ANSIC,
			time.UnixDate,
		}
		for _, format := range formats {
			if t, err := time.Parse(format, tmuxInfo.Created); err == nil {
				info.Created = t
				break
			}
		}
	}

	if tmuxInfo.Activity != "" {
		var activityUnix int64
		if _, err := fmt.Sscanf(tmuxInfo.Activity, "%d", &activityUnix); err == nil && activityUnix > 0 {
			info.LastActivity = time.Unix(activityUnix, 0)
		}
	}

	return info, nil
}

// List returns information about all polecat sessions for this rig.
func (m *SessionManager) List() ([]SessionInfo, error) {
	agentIDs, err := m.agents.List()
	if err != nil {
		return nil, err
	}

	// Filter to polecats for this rig: "rigname/polecat/name"
	prefix := m.rig.Name + "/polecat/"
	var infos []SessionInfo

	for _, id := range agentIDs {
		idStr := string(id)
		if !strings.HasPrefix(idStr, prefix) {
			continue
		}

		polecat := strings.TrimPrefix(idStr, prefix)
		infos = append(infos, SessionInfo{
			Polecat:   polecat,
			SessionID: m.SessionName(polecat),
			Running:   true,
			RigName:   m.rig.Name,
		})
	}

	return infos, nil
}

// Attach attaches to a polecat session.
func (m *SessionManager) Attach(polecat string) error {
	id := m.agentID(polecat)
	if !m.agents.Exists(id) {
		return ErrSessionNotFound
	}
	return m.agents.Attach(id)
}

// Capture returns the recent output from a polecat session.
func (m *SessionManager) Capture(polecat string, lines int) (string, error) {
	id := m.agentID(polecat)

	if !m.agents.Exists(id) {
		return "", ErrSessionNotFound
	}

	return m.agents.Capture(id, lines)
}

// CaptureSession returns the recent output from a session by raw session ID.
// Deprecated: This method uses raw session names. Prefer Capture(polecat, lines).
func (m *SessionManager) CaptureSession(sessionName string, lines int) (string, error) {
	// Convert session name back to AgentID
	// Session names are "gt-rigname-polecatname", we need "rigname/polecat/polecatname"
	prefix := fmt.Sprintf("gt-%s-", m.rig.Name)
	if !strings.HasPrefix(sessionName, prefix) {
		return "", ErrSessionNotFound
	}
	polecat := strings.TrimPrefix(sessionName, prefix)
	return m.Capture(polecat, lines)
}

// Inject sends a message to a polecat session.
func (m *SessionManager) Inject(polecat, message string) error {
	id := m.agentID(polecat)

	if !m.agents.Exists(id) {
		return ErrSessionNotFound
	}

	return m.agents.Nudge(id, message)
}

// StopAll terminates all polecat sessions for this rig.
func (m *SessionManager) StopAll(force bool) error {
	infos, err := m.List()
	if err != nil {
		return err
	}

	var lastErr error
	for _, info := range infos {
		if err := m.Stop(info.Polecat, force); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// hookIssue pins an issue to a polecat's hook using bd update.
func (m *SessionManager) hookIssue(issueID, agentID, workDir string) error {
	cmd := exec.Command("bd", "update", issueID, "--status=hooked", "--assignee="+agentID) //nolint:gosec
	cmd.Dir = workDir
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bd update failed: %w", err)
	}
	fmt.Printf("✓ Hooked issue %s to %s\n", issueID, agentID)
	return nil
}
