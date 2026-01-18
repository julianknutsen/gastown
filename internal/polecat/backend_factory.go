package polecat

import (
	"path/filepath"

	"github.com/steveyegge/gastown/internal/agent"
	"github.com/steveyegge/gastown/internal/config"
	"github.com/steveyegge/gastown/internal/git"
	"github.com/steveyegge/gastown/internal/rig"
)

// BackendFor returns the appropriate Backend for a rig.
// If the rig has remote configuration, returns RemoteManager.
// Otherwise returns the local Manager.
//
// This allows the spawn code to use a single code path for both
// local and remote polecats.
func BackendFor(agents agent.Agents, r *rig.Rig, g *git.Git) Backend {
	// Check for remote polecat config
	settingsPath := filepath.Join(r.Path, "settings", "config.json")
	settings, err := config.LoadRigSettings(settingsPath)

	if err == nil && settings.Remote != nil {
		return NewRemoteManager(r, RemoteConfig{
			SSHCmd:        settings.Remote.SSHCmd,
			LocalSSH:      settings.Remote.LocalSSH,
			RemoteRigPath: settings.Remote.RemoteRigPath,
		})
	}

	// Default to local manager
	return NewManager(agents, r, g)
}
