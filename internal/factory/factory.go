package factory

import (
	"strings"

	"github.com/steveyegge/gastown/internal/agent"
	"github.com/steveyegge/gastown/internal/config"
	"github.com/steveyegge/gastown/internal/crew"
	"github.com/steveyegge/gastown/internal/deacon"
	"github.com/steveyegge/gastown/internal/git"
	"github.com/steveyegge/gastown/internal/mayor"
	"github.com/steveyegge/gastown/internal/polecat"
	"github.com/steveyegge/gastown/internal/refinery"
	"github.com/steveyegge/gastown/internal/rig"
	"github.com/steveyegge/gastown/internal/session"
	"github.com/steveyegge/gastown/internal/tmux"
	"github.com/steveyegge/gastown/internal/witness"
)

// =============================================================================
// Agent Manager Factories
//
// These functions create properly configured managers with Agents, centralizing
// the pattern of env vars + session config + Agents creation.
// =============================================================================

// MayorManager creates a properly configured mayor.Manager with Agents.
func MayorManager(townRoot, agentName string) *mayor.Manager {
	envVars := config.AgentEnv(config.AgentEnvConfig{
		Role:     "mayor",
		TownRoot: townRoot,
	})
	sessionCfg := tmux.SessionConfigForRole("mayor", "").WithEnvVars(envVars)
	t := tmux.NewTmux().WithSessionConfig(sessionCfg)
	agents := agent.New(t, agent.FromPreset(agentName).WithEnvVars(envVars))
	return mayor.NewManager(townRoot, agentName, agents)
}

// DeaconManager creates a properly configured deacon.Manager with Agents.
func DeaconManager(townRoot, agentName string) *deacon.Manager {
	envVars := config.AgentEnv(config.AgentEnvConfig{
		Role:     "deacon",
		TownRoot: townRoot,
	})
	sessionCfg := tmux.SessionConfigForRole("deacon", "").WithEnvVars(envVars)
	t := tmux.NewTmux().WithSessionConfig(sessionCfg)
	agents := agent.New(t, agent.FromPreset(agentName).WithEnvVars(envVars))
	return deacon.NewManager(townRoot, agentName, agents)
}

// WitnessManager creates a properly configured witness.Manager with Agents.
// envOverrides is an optional list of KEY=VALUE strings to merge with base env vars.
func WitnessManager(r *rig.Rig, townRoot, agentName string, envOverrides ...string) *witness.Manager {
	envVars := config.AgentEnv(config.AgentEnvConfig{
		Role:     "witness",
		Rig:      r.Name,
		TownRoot: townRoot,
	})
	// Merge env overrides
	for _, override := range envOverrides {
		if key, value, ok := splitEnvVar(override); ok {
			envVars[key] = value
		}
	}
	sessionCfg := tmux.SessionConfigForRole("witness", r.Name).WithEnvVars(envVars)
	t := tmux.NewTmux().WithSessionConfig(sessionCfg)
	agents := agent.New(t, agent.FromPreset(agentName).WithEnvVars(envVars))
	return witness.NewManager(r, agents, agentName)
}

// RefineryManager creates a properly configured refinery.Manager with Agents.
func RefineryManager(r *rig.Rig, townRoot, agentName string) *refinery.Manager {
	envVars := config.AgentEnv(config.AgentEnvConfig{
		Role:     "refinery",
		Rig:      r.Name,
		TownRoot: townRoot,
	})
	sessionCfg := tmux.SessionConfigForRole("refinery", r.Name).WithEnvVars(envVars)
	t := tmux.NewTmux().WithSessionConfig(sessionCfg)
	agents := agent.New(t, agent.FromPreset(agentName).WithEnvVars(envVars))
	return refinery.NewManager(r, agents, agentName)
}

// PolecatSessionManager creates a properly configured polecat.SessionManager with Agents.
// Unlike other managers, polecat env vars are set per-instance (based on polecat name),
// so they're baked into the command at Start time rather than at factory time.
func PolecatSessionManager(r *rig.Rig, agentName string) *polecat.SessionManager {
	t := tmux.NewTmux()
	agents := agent.New(t, agent.FromPreset(agentName))
	return polecat.NewSessionManager(agents, t, r, agentName)
}

// CrewManager creates a properly configured crew.Manager with Agents.
// Uses a closure pattern to wire up the OnSessionCreated callback.
func CrewManager(r *rig.Rig, townRoot, agentName string) *crew.Manager {
	g := git.NewGit(r.Path)

	// Forward declaration for closure
	var mgr *crew.Manager

	// Create tmux and agents with callback that forwards to manager
	t := tmux.NewTmux()
	agentCfg := agent.FromPreset(agentName).WithOnSessionCreated(func(sess session.Sessions, id session.SessionID) error {
		return mgr.OnSessionCreated(sess, id)
	})
	agents := agent.New(t, agentCfg)

	mgr = crew.NewManager(r, g, agents, agentName)
	return mgr
}

// splitEnvVar splits a KEY=VALUE string into key and value.
func splitEnvVar(s string) (key, value string, ok bool) {
	idx := strings.Index(s, "=")
	if idx < 0 {
		return "", "", false
	}
	return s[:idx], s[idx+1:], true
}

// =============================================================================
// Generic Factories
//
// These functions provide generic interfaces without role-specific configuration.
// Use them when operating on existing sessions without knowing their role.
// =============================================================================

// GenericAgents returns an Agents interface for interacting with existing sessions.
// Use this for health checks, force-kills, and other operations on arbitrary agents
// where you don't need role-specific configuration (theming, env vars, etc.).
func GenericAgents() agent.Agents {
	t := tmux.NewTmux()
	return agent.New(t, agent.Claude())
}
