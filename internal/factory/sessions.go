package factory

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/steveyegge/gastown/internal/claude"
	"github.com/steveyegge/gastown/internal/config"
	"github.com/steveyegge/gastown/internal/constants"
	"github.com/steveyegge/gastown/internal/ids"
	"github.com/steveyegge/gastown/internal/session"
	"github.com/steveyegge/gastown/internal/tmux"
)

// SessionFactory creates Sessions instances based on agent configuration.
// It encapsulates the decision of whether to use local tmux, remote tmux,
// or mirrored sessions based on the agent type and rig configuration.
type SessionFactory struct {
	townRoot string
}

// NewSessionFactory creates a new session factory for the given town.
func NewSessionFactory(townRoot string) *SessionFactory {
	return &SessionFactory{townRoot: townRoot}
}

// SessionInfo contains the Sessions implementation and the local tmux for theming.
type SessionInfo struct {
	// Sessions is the session implementation to use for agent operations.
	Sessions session.Sessions

	// LocalTmux is the local tmux instance for theming.
	// For local agents, this is the same as Sessions.
	// For remote agents, this is the local mirror that can be themed.
	LocalTmux *tmux.Tmux
}

// For returns the appropriate Sessions implementation for an agent.
//
// Routing logic:
//   - Polecat with remote config → MirroredSessions (remote + local mirror)
//   - Everything else → local Tmux
//
// This centralizes the "what kind of sessions?" decision so callers
// don't need to know about remote configuration.
func (f *SessionFactory) For(id ids.AgentID) session.Sessions {
	return f.ForWithInfo(id, "").Sessions
}

// ForWithInfo returns Sessions along with the local tmux for theming.
// If workDir is provided and this is a remote polecat, Claude settings
// are copied to the remote before returning.
func (f *SessionFactory) ForWithInfo(id ids.AgentID, workDir string) SessionInfo {
	role, rigName, _ := id.Parse()

	// Only polecats can be remote
	if role == constants.RolePolecat && rigName != "" {
		if cfg := f.loadRemoteConfig(rigName); cfg != nil {
			// Copy Claude settings to remote for prehooks to work
			if workDir != "" {
				if err := copyClaudeSettingsToRemote(cfg.SSHCmd, workDir, role); err != nil {
					fmt.Printf("Warning: could not copy Claude settings to remote: %v\n", err)
				}
			}

			localTmux := tmux.NewLocalTmux()
			return SessionInfo{
				Sessions: session.NewMirroredSessions(
					tmux.NewRemoteTmuxWithCallback(cfg.SSHCmd, cfg.LocalSSH),
					localTmux,
					cfg.SSHCmd,
				),
				LocalTmux: localTmux,
			}
		}
	}

	// Mayor, deacon, witness, refinery, crew, local polecats → local tmux
	localTmux := tmux.NewLocalTmux()
	return SessionInfo{
		Sessions:  localTmux,
		LocalTmux: localTmux,
	}
}

// copyClaudeSettingsToRemote copies .claude/settings.json to a remote workDir.
func copyClaudeSettingsToRemote(sshCmd, remoteWorkDir, role string) error {
	settingsContent, err := claude.SettingsContentFor(role)
	if err != nil {
		return fmt.Errorf("getting settings content: %w", err)
	}

	// Create .claude directory on remote
	mkdirCmd := fmt.Sprintf("%s 'mkdir -p %s/.claude'", sshCmd, remoteWorkDir)
	if err := exec.Command("sh", "-c", mkdirCmd).Run(); err != nil {
		return fmt.Errorf("creating .claude directory: %w", err)
	}

	// Write settings file to remote via stdin
	writeCmd := fmt.Sprintf("%s 'cat > %s/.claude/settings.json'", sshCmd, remoteWorkDir)
	cmd := exec.Command("sh", "-c", writeCmd)
	cmd.Stdin = strings.NewReader(string(settingsContent))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("writing settings file: %w", err)
	}

	return nil
}

// loadRemoteConfig loads the remote polecat config for a rig if it exists.
// Returns nil if the rig doesn't have remote config.
func (f *SessionFactory) loadRemoteConfig(rigName string) *config.RemotePolecatConfig {
	rigPath := filepath.Join(f.townRoot, rigName)
	settingsPath := filepath.Join(rigPath, "settings", "config.json")
	settings, err := config.LoadRigSettings(settingsPath)
	if err != nil {
		return nil
	}
	return settings.Remote
}
