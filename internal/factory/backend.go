package factory

import (
	"path/filepath"

	"github.com/steveyegge/gastown/internal/config"
	"github.com/steveyegge/gastown/internal/constants"
	"github.com/steveyegge/gastown/internal/git"
	"github.com/steveyegge/gastown/internal/ids"
	"github.com/steveyegge/gastown/internal/polecat"
	"github.com/steveyegge/gastown/internal/runner"
)

// ManagerDepsFor returns the appropriate ManagerDeps for a polecat agent.
//
// Routing logic:
//   - Polecat with remote config → SSH filesystem + GitOps
//   - Everything else → local filesystem + GitOps
//
// This determines whether polecat operations are local or remote.
func ManagerDepsFor(townRoot string, id ids.AgentID) *polecat.ManagerDeps {
	role, rigName, _ := id.Parse()

	// Only polecats can be remote
	if role == constants.RolePolecat && rigName != "" {
		if cfg := loadRemotePolecatConfig(townRoot, rigName); cfg != nil {
			sshRunner := runner.NewSSH(cfg.SSHCmd)

			// Default remote rig path if not specified
			remoteRigPath := cfg.RemoteRigPath
			if remoteRigPath == "" {
				remoteRigPath = "~/rigs/" + rigName
			}

			return &polecat.ManagerDeps{
				TargetFS:      polecat.NewRemoteFilesystem(sshRunner),
				LocalFS:       polecat.NewLocalFilesystem(),
				GitOps:        git.NewOps(sshRunner),
				TargetRigPath: remoteRigPath,
			}
		}
	}

	// Local deps
	rigPath := filepath.Join(townRoot, rigName)
	return &polecat.ManagerDeps{
		TargetFS:      polecat.NewLocalFilesystem(),
		LocalFS:       polecat.NewLocalFilesystem(),
		GitOps:        git.NewOps(runner.NewLocal()),
		TargetRigPath: rigPath,
	}
}

// loadRemotePolecatConfig loads the remote polecat config for a rig if it exists.
// Returns nil if the rig doesn't have remote config.
func loadRemotePolecatConfig(townRoot, rigName string) *config.RemotePolecatConfig {
	rigPath := filepath.Join(townRoot, rigName)
	settingsPath := filepath.Join(rigPath, "settings", "config.json")
	settings, err := config.LoadRigSettings(settingsPath)
	if err != nil {
		return nil
	}
	return settings.Remote
}
