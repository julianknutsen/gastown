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
	"github.com/steveyegge/gastown/internal/constants"
	"github.com/steveyegge/gastown/internal/rig"
	"github.com/steveyegge/gastown/internal/runtime"
	"github.com/steveyegge/gastown/internal/session"
	"github.com/steveyegge/gastown/internal/tmux"
)

// debugSession logs non-fatal errors during session startup when GT_DEBUG_SESSION=1.
func debugSession(context string, err error) {
	if os.Getenv("GT_DEBUG_SESSION") != "" && err != nil {
		fmt.Fprintf(os.Stderr, "[session-debug] %s: %v\n", context, err)
	}
}

// Session errors
var (
	ErrSessionRunning  = agent.ErrAlreadyRunning
	ErrSessionNotFound = errors.New("session not found")
)

// SessionManager handles polecat session lifecycle.
// Uses agent.Agents for lifecycle operations (same pattern as mayor/deacon/witness/refinery).
type SessionManager struct {
	agents    agent.Agents
	sess      session.Sessions // For direct session access (capture, inject, etc.)
	rig       *rig.Rig
	agentName string
}

// NewSessionManager creates a new polecat session manager for a rig.
// agents is the Agents instance for lifecycle management.
// sess is the underlying Sessions for direct access (capture, inject, attach).
// agentName is the resolved agent to use (from config.ResolveRoleAgentName or command line).
func NewSessionManager(agents agent.Agents, sess session.Sessions, r *rig.Rig, agentName string) *SessionManager {
	return &SessionManager{
		agents:    agents,
		sess:      sess,
		rig:       r,
		agentName: agentName,
	}
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

// configureTmuxSession applies tmux-specific configuration after session creation.
// This is a no-op if sess is not a *tmux.Tmux.
func (m *SessionManager) configureTmuxSession(polecat string, sessionID session.SessionID) {
	t, ok := m.sess.(*tmux.Tmux)
	if !ok {
		return
	}

	// Set environment variables (redundant with command prefix, but provides runtime visibility)
	if err := t.SetEnvVars(sessionID, config.AgentEnv(m.envConfig(polecat))); err != nil {
		debugSession("SetEnvVars", err)
	}

	// Configure session appearance (theme, status bar)
	if err := t.ConfigureGasTownSession(sessionID, tmux.AssignTheme(m.rig.Name), m.rig.Name, polecat, "polecat"); err != nil {
		debugSession("ConfigureGasTownSession", err)
	}

	// Set pane-died hook for cleanup
	if err := t.SetPaneDiedHook(sessionID, fmt.Sprintf("%s/%s", m.rig.Name, polecat)); err != nil {
		debugSession("SetPaneDiedHook", err)
	}
}

// waitForReady performs tmux-specific startup waiting.
// This is a no-op if sess is not a *tmux.Tmux.
func (m *SessionManager) waitForReady(sessionName string) {
	t, ok := m.sess.(*tmux.Tmux)
	if !ok {
		return
	}

	// Wait for Claude to start and handle dialogs
	debugSession("WaitForCommand", t.WaitForCommand(sessionName, constants.SupportedShells, constants.ClaudeStartTimeout))

	// Accept bypass permissions warning dialog if it appears
	debugSession("AcceptBypassPermissionsWarning", t.AcceptBypassPermissionsWarning(sessionName))

	// Wait for runtime to be fully ready at the prompt
	runtimeConfig := config.LoadRuntimeConfig(m.rig.Path)
	runtime.SleepForReadyDelay(runtimeConfig)
	_ = runtime.RunStartupFallback(t, sessionName, "polecat", runtimeConfig)
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

	// Create session via agent.Agents (handles zombie detection)
	_, err := m.agents.Start(sessionName, workDir, command)
	if err != nil {
		if errors.Is(err, agent.ErrAlreadyRunning) {
			return fmt.Errorf("%w: %s", ErrSessionRunning, sessionName)
		}
		return fmt.Errorf("creating session: %w", err)
	}

	// Apply tmux-specific configuration (themes, hooks) - no-op for non-tmux
	sessionID := session.SessionID(sessionName)
	m.configureTmuxSession(polecat, sessionID)

	// Wait for ready (tmux-specific startup) - no-op for non-tmux
	m.waitForReady(sessionName)

	return nil
}

// Stop terminates a polecat session.
func (m *SessionManager) Stop(polecat string, force bool) error {
	sessionName := m.SessionName(polecat)
	agentID := agent.AgentID(sessionName)

	if !m.agents.Exists(agentID) {
		return ErrSessionNotFound
	}

	// Sync beads before shutdown (non-fatal)
	if !force {
		polecatDir := m.polecatDir(polecat)
		if err := m.syncBeads(polecatDir); err != nil {
			fmt.Printf("Warning: beads sync failed: %v\n", err)
		}
	}

	// Stop via agent.Agents (graceful sends Ctrl-C first)
	if err := m.agents.Stop(agentID, !force); err != nil {
		return fmt.Errorf("stopping session: %w", err)
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
	agentID := agent.AgentID(m.SessionName(polecat))
	return m.agents.Exists(agentID), nil
}

// Status returns detailed status for a polecat session.
func (m *SessionManager) Status(polecat string) (*SessionInfo, error) {
	sessionName := m.SessionName(polecat)
	sessionID := session.SessionID(sessionName)

	running, err := m.sess.Exists(sessionID)
	if err != nil {
		return nil, fmt.Errorf("checking session: %w", err)
	}

	info := &SessionInfo{
		Polecat:   polecat,
		SessionID: sessionName,
		Running:   running,
		RigName:   m.rig.Name,
	}

	if !running {
		return info, nil
	}

	tmuxInfo, err := m.sess.GetInfo(sessionID)
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
	sessions, err := m.sess.List()
	if err != nil {
		return nil, err
	}

	prefix := fmt.Sprintf("gt-%s-", m.rig.Name)
	var infos []SessionInfo

	for _, sessionID := range sessions {
		sessionName := string(sessionID)
		if !strings.HasPrefix(sessionName, prefix) {
			continue
		}

		polecat := strings.TrimPrefix(sessionName, prefix)
		infos = append(infos, SessionInfo{
			Polecat:   polecat,
			SessionID: sessionName,
			Running:   true,
			RigName:   m.rig.Name,
		})
	}

	return infos, nil
}

// Attach attaches to a polecat session.
func (m *SessionManager) Attach(polecat string) error {
	sessionName := m.SessionName(polecat)
	sessionID := session.SessionID(sessionName)

	running, err := m.sess.Exists(sessionID)
	if err != nil {
		return fmt.Errorf("checking session: %w", err)
	}
	if !running {
		return ErrSessionNotFound
	}

	// AttachSession is tmux-specific
	if t, ok := m.sess.(*tmux.Tmux); ok {
		return t.AttachSession(sessionName)
	}
	return fmt.Errorf("attach not supported for this session type")
}

// Capture returns the recent output from a polecat session.
func (m *SessionManager) Capture(polecat string, lines int) (string, error) {
	sessionID := session.SessionID(m.SessionName(polecat))

	running, err := m.sess.Exists(sessionID)
	if err != nil {
		return "", fmt.Errorf("checking session: %w", err)
	}
	if !running {
		return "", ErrSessionNotFound
	}

	return m.sess.Capture(sessionID, lines)
}

// CaptureSession returns the recent output from a session by raw session ID.
func (m *SessionManager) CaptureSession(sessionName string, lines int) (string, error) {
	sessionID := session.SessionID(sessionName)
	running, err := m.sess.Exists(sessionID)
	if err != nil {
		return "", fmt.Errorf("checking session: %w", err)
	}
	if !running {
		return "", ErrSessionNotFound
	}

	return m.sess.Capture(sessionID, lines)
}

// Inject sends a message to a polecat session.
func (m *SessionManager) Inject(polecat, message string) error {
	sessionName := m.SessionName(polecat)
	sessionID := session.SessionID(sessionName)

	running, err := m.sess.Exists(sessionID)
	if err != nil {
		return fmt.Errorf("checking session: %w", err)
	}
	if !running {
		return ErrSessionNotFound
	}

	debounceMs := 200 + (len(message)/1024)*100
	if debounceMs > 1500 {
		debounceMs = 1500
	}

	// SendKeysDebounced is tmux-specific
	if t, ok := m.sess.(*tmux.Tmux); ok {
		return t.SendKeysDebounced(sessionName, message, debounceMs)
	}
	// Fallback to regular Send for non-tmux sessions
	return m.sess.Send(sessionID, message)
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
