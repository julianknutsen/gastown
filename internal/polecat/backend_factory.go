package polecat

import (
	"path/filepath"

	"github.com/steveyegge/gastown/internal/agent"
	"github.com/steveyegge/gastown/internal/config"
	"github.com/steveyegge/gastown/internal/git"
	"github.com/steveyegge/gastown/internal/rig"
	"github.com/steveyegge/gastown/internal/runner"
)

// BackendFor returns the appropriate Backend (Manager) for a rig.
// The returned Manager is configured for either local or remote operations
// based on the rig's configuration.
//
// This allows the spawn code to use a single code path for both
// local and remote polecats.
func BackendFor(r *rig.Rig, g *git.Git) Backend {
	// Use agent.Default() for session operations within the Manager.
	// Callers who need remote polecat support should use factory.AgentsFor()
	// directly for session operations instead of going through the Manager.
	agents := agent.Default()

	// Check for remote polecat config
	settingsPath := filepath.Join(r.Path, "settings", "config.json")
	settings, err := config.LoadRigSettings(settingsPath)

	if err == nil && settings.Remote != nil {
		// Remote polecat - create Manager with SSH-based deps
		sshRunner := runner.NewSSH(settings.Remote.SSHCmd)

		remoteRigPath := settings.Remote.RemoteRigPath
		if remoteRigPath == "" {
			remoteRigPath = "~/rigs/" + r.Name
		}

		deps := &ManagerDeps{
			TargetFS:      NewRemoteFilesystem(sshRunner),
			LocalFS:       NewLocalFilesystem(),
			GitOps:        git.NewOps(sshRunner),
			Runner:        sshRunner,
			TargetRigPath: remoteRigPath,
		}
		return NewManagerWithDeps(agents, r, deps)
	}

	// Default to local manager
	localFS := NewLocalFilesystem()
	localRunner := runner.NewLocal()
	deps := &ManagerDeps{
		TargetFS:      localFS,
		LocalFS:       localFS,
		GitOps:        git.NewOps(localRunner),
		Runner:        localRunner,
		TargetRigPath: r.Path,
	}
	return NewManagerWithDeps(agents, r, deps)
}
