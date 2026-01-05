// Package constants defines shared constant values used throughout Gas Town.
// Centralizing these magic strings improves maintainability and consistency.
package constants

import (
	"path/filepath"
	"time"
)

// Timing constants for session management and tmux operations.
const (
	// ShutdownNotifyDelay is the pause after sending shutdown notification.
	ShutdownNotifyDelay = 500 * time.Millisecond

	// ClaudeStartTimeout is how long to wait for Claude to start in a session.
	// Increased to 60s because Claude can take 30s+ on slower machines.
	ClaudeStartTimeout = 60 * time.Second

	// ShellReadyTimeout is how long to wait for shell prompt after command.
	ShellReadyTimeout = 5 * time.Second

	// DefaultDebounceMs is the default debounce for SendKeys operations.
	// 500ms is required for Claude Code to reliably process paste before Enter.
	// See NudgeSession comment: "Wait 500ms for paste to complete (tested, required)"
	DefaultDebounceMs = 500

	// DefaultDisplayMs is the default duration for tmux display-message.
	DefaultDisplayMs = 5000

	// PollInterval is the default polling interval for wait loops.
	PollInterval = 100 * time.Millisecond
)

// Directory names within a Gas Town workspace.
const (
	// DirMayor is the directory containing mayor configuration and state.
	DirMayor = "mayor"

	// DirPolecats is the directory containing polecat worktrees.
	DirPolecats = "polecats"

	// DirCrew is the directory containing crew workspaces.
	DirCrew = "crew"

	// DirRefinery is the directory containing the refinery clone.
	DirRefinery = "refinery"

	// DirWitness is the directory containing witness state.
	DirWitness = "witness"

	// DirRig is the subdirectory containing the actual git clone.
	DirRig = "rig"

	// DirBeads is the beads database directory.
	DirBeads = ".beads"

	// DirRuntime is the runtime state directory (gitignored).
	DirRuntime = ".runtime"

	// DirSettings is the rig settings directory (git-tracked).
	DirSettings = "settings"
)

// File names for configuration and state.
const (
	// FileRigsJSON is the rig registry file in mayor/.
	FileRigsJSON = "rigs.json"

	// FileTownJSON is the town configuration file in mayor/.
	FileTownJSON = "town.json"

	// FileConfigJSON is the general config file.
	FileConfigJSON = "config.json"

	// FileAccountsJSON is the accounts configuration file in mayor/.
	FileAccountsJSON = "accounts.json"
)

// Git branch names.
const (
	// BranchMain is the default main branch name.
	BranchMain = "main"

	// BranchBeadsSync is the branch used for beads synchronization.
	BranchBeadsSync = "beads-sync"

	// BranchPolecatPrefix is the prefix for polecat work branches.
	BranchPolecatPrefix = "polecat/"

	// BranchIntegrationPrefix is the prefix for integration branches.
	BranchIntegrationPrefix = "integration/"
)

// Tmux session names.
// Mayor and Deacon use simple session names: gt-mayor, gt-deacon (one per machine).
// Use session.MayorSessionName() and session.DeaconSessionName().
const (
	// SessionPrefix is the prefix for all Gas Town tmux sessions.
	SessionPrefix = "gt-"
)

// Agent role names.
const (
	// RoleMayor is the mayor agent role.
	RoleMayor = "mayor"

	// RoleWitness is the witness agent role.
	RoleWitness = "witness"

	// RoleRefinery is the refinery agent role.
	RoleRefinery = "refinery"

	// RolePolecat is the polecat agent role.
	RolePolecat = "polecat"

	// RoleCrew is the crew agent role.
	RoleCrew = "crew"

	// RoleDeacon is the deacon agent role.
	RoleDeacon = "deacon"
)

// Role emojis - centralized for easy customization.
// These match the Gas Town visual identity (see ~/Desktop/Gas Town/ prompts).
const (
	// EmojiMayor is the mayor emoji (fox conductor).
	EmojiMayor = "🎩"

	// EmojiDeacon is the deacon emoji (wolf in the engine room).
	EmojiDeacon = "🐺"

	// EmojiWitness is the witness emoji (watchful owl).
	EmojiWitness = "🦉"

	// EmojiRefinery is the refinery emoji (industrial).
	EmojiRefinery = "🏭"

	// EmojiCrew is the crew emoji (established worker).
	EmojiCrew = "👷"

	// EmojiPolecat is the polecat emoji (transient worker).
	EmojiPolecat = "😺"
)

// RoleEmoji returns the emoji for a given role name.
func RoleEmoji(role string) string {
	switch role {
	case RoleMayor:
		return EmojiMayor
	case RoleDeacon:
		return EmojiDeacon
	case RoleWitness:
		return EmojiWitness
	case RoleRefinery:
		return EmojiRefinery
	case RoleCrew:
		return EmojiCrew
	case RolePolecat:
		return EmojiPolecat
	default:
		return "❓"
	}
}

// SupportedShells lists shell binaries that Gas Town can detect and work with.
// Used to identify if a tmux pane is at a shell prompt vs running a command.
var SupportedShells = []string{"bash", "zsh", "sh", "fish", "tcsh", "ksh"}

// Path helpers construct common paths.
// All helpers panic on empty inputs to catch programming errors early.

// RigPath returns the absolute path to a rig given the town root and rig name.
func RigPath(townRoot, rigName string) string {
	if townRoot == "" {
		panic("RigPath: townRoot is required")
	}
	if rigName == "" {
		panic("RigPath: rigName is required")
	}
	return filepath.Join(townRoot, rigName)
}

// TownRootFromRig returns the town root given a rig path.
// Rigs are always direct children of the town root.
func TownRootFromRig(rigPath string) string {
	if rigPath == "" {
		panic("TownRootFromRig: rigPath is required")
	}
	return filepath.Dir(rigPath)
}

// AgentClonePath returns the path to an agent's worktree within a rig.
// agentDir should be DirPolecats or DirCrew.
func AgentClonePath(rigPath, agentDir, agentName string) string {
	if rigPath == "" {
		panic("AgentClonePath: rigPath is required")
	}
	if agentDir == "" {
		panic("AgentClonePath: agentDir is required")
	}
	if agentName == "" {
		panic("AgentClonePath: agentName is required")
	}
	return filepath.Join(rigPath, agentDir, agentName)
}

// RigPathFromAgentClone returns the rig path from an agent clone path.
// Agent clones (polecat or crew) are at rigPath/{polecats,crew}/name,
// so rig is two dirs up.
func RigPathFromAgentClone(agentClonePath string) string {
	if agentClonePath == "" {
		panic("RigPathFromAgentClone: agentClonePath is required")
	}
	return filepath.Dir(filepath.Dir(agentClonePath))
}

// MayorRigsPath returns the path to rigs.json within a town root.
func MayorRigsPath(townRoot string) string {
	if townRoot == "" {
		panic("MayorRigsPath: townRoot is required")
	}
	return filepath.Join(townRoot, DirMayor, FileRigsJSON)
}

// MayorTownPath returns the path to town.json within a town root.
func MayorTownPath(townRoot string) string {
	if townRoot == "" {
		panic("MayorTownPath: townRoot is required")
	}
	return filepath.Join(townRoot, DirMayor, FileTownJSON)
}

// RigMayorPath returns the path to mayor/rig within a rig.
func RigMayorPath(rigPath string) string {
	if rigPath == "" {
		panic("RigMayorPath: rigPath is required")
	}
	return filepath.Join(rigPath, DirMayor, DirRig)
}

// RigBeadsPath returns the path to mayor/rig/.beads within a rig.
func RigBeadsPath(rigPath string) string {
	if rigPath == "" {
		panic("RigBeadsPath: rigPath is required")
	}
	return filepath.Join(rigPath, DirMayor, DirRig, DirBeads)
}

// RigPolecatsPath returns the path to polecats/ within a rig.
func RigPolecatsPath(rigPath string) string {
	if rigPath == "" {
		panic("RigPolecatsPath: rigPath is required")
	}
	return filepath.Join(rigPath, DirPolecats)
}

// RigCrewPath returns the path to crew/ within a rig.
func RigCrewPath(rigPath string) string {
	if rigPath == "" {
		panic("RigCrewPath: rigPath is required")
	}
	return filepath.Join(rigPath, DirCrew)
}

// MayorConfigPath returns the path to mayor/config.json within a town root.
func MayorConfigPath(townRoot string) string {
	if townRoot == "" {
		panic("MayorConfigPath: townRoot is required")
	}
	return filepath.Join(townRoot, DirMayor, FileConfigJSON)
}

// TownRuntimePath returns the path to .runtime/ at the town root.
func TownRuntimePath(townRoot string) string {
	if townRoot == "" {
		panic("TownRuntimePath: townRoot is required")
	}
	return filepath.Join(townRoot, DirRuntime)
}

// RigRuntimePath returns the path to .runtime/ within a rig.
func RigRuntimePath(rigPath string) string {
	if rigPath == "" {
		panic("RigRuntimePath: rigPath is required")
	}
	return filepath.Join(rigPath, DirRuntime)
}

// RigSettingsPath returns the path to settings/ within a rig.
func RigSettingsPath(rigPath string) string {
	if rigPath == "" {
		panic("RigSettingsPath: rigPath is required")
	}
	return filepath.Join(rigPath, DirSettings)
}

// MayorAccountsPath returns the path to mayor/accounts.json within a town root.
func MayorAccountsPath(townRoot string) string {
	if townRoot == "" {
		panic("MayorAccountsPath: townRoot is required")
	}
	return filepath.Join(townRoot, DirMayor, FileAccountsJSON)
}
