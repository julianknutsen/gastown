package factory

import (
	"fmt"
	"os"
	"strings"

	"github.com/steveyegge/gastown/internal/agent"
	"github.com/steveyegge/gastown/internal/boot"
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
	sess := session.NewTownSessions(t, townRoot)
	agents := agent.New(sess, agent.FromPreset(agentName).WithEnvVars(envVars))
	return mayor.NewManager(agents, townRoot, agentName)
}

// BootManager creates a properly configured boot.Boot with Agents.
func BootManager(townRoot, agentName string) *boot.Boot {
	envVars := config.AgentEnv(config.AgentEnvConfig{
		Role:     "boot",
		TownRoot: townRoot,
	})
	sessionCfg := tmux.SessionConfigForRole("boot", "").WithEnvVars(envVars)
	t := tmux.NewTmux().WithSessionConfig(sessionCfg)
	sess := session.NewTownSessions(t, townRoot)
	agents := agent.New(sess, agent.FromPreset(agentName).WithEnvVars(envVars))
	return boot.NewManager(agents, townRoot, agentName)
}

// DeaconManager creates a properly configured deacon.Manager with Agents.
func DeaconManager(townRoot, agentName string) *deacon.Manager {
	envVars := config.AgentEnv(config.AgentEnvConfig{
		Role:     "deacon",
		TownRoot: townRoot,
	})
	sessionCfg := tmux.SessionConfigForRole("deacon", "").WithEnvVars(envVars)
	t := tmux.NewTmux().WithSessionConfig(sessionCfg)
	sess := session.NewTownSessions(t, townRoot)
	agents := agent.New(sess, agent.FromPreset(agentName).WithEnvVars(envVars))
	return deacon.NewManager(agents, townRoot, agentName)
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
	sess := session.NewTownSessions(t, townRoot)
	agents := agent.New(sess, agent.FromPreset(agentName).WithEnvVars(envVars))
	return witness.NewManager(agents, r, agentName)
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
	sess := session.NewTownSessions(t, townRoot)
	agents := agent.New(sess, agent.FromPreset(agentName).WithEnvVars(envVars))
	return refinery.NewManager(agents, r, agentName)
}

// PolecatSessionManager creates a properly configured polecat.SessionManager with Agents.
// Unlike other managers, polecat env vars are set per-instance (based on polecat name),
// so they're baked into the command at Start time rather than at factory time.
// Uses callback pattern for tmux-specific setup (theming, hooks).
func PolecatSessionManager(r *rig.Rig, townRoot, agentName string) *polecat.SessionManager {
	// Forward declaration for closure
	var mgr *polecat.SessionManager

	t := tmux.NewTmux()
	sess := session.NewTownSessions(t, townRoot)

	agentCfg := agent.FromPreset(agentName).WithOnSessionCreated(func(_ session.Sessions, id session.SessionID) error {
		// Apply env vars, theming, and hooks when session is created
		actualID := session.SessionID(session.ToUniqueHumanReadableName(id, townRoot))
		polecatName := mgr.StartingPolecatName()

		// Set environment variables (redundant with command prefix, but provides runtime visibility)
		envVars := config.AgentEnv(config.AgentEnvConfig{
			Role:          "polecat",
			Rig:           r.Name,
			AgentName:     polecatName,
			TownRoot:      townRoot,
			BeadsNoDaemon: true,
		})
		if err := t.SetEnvVars(actualID, envVars); err != nil {
			// Non-fatal, log for debugging
			if os.Getenv("GT_DEBUG_SESSION") != "" {
				fmt.Fprintf(os.Stderr, "[session-debug] SetEnvVars: %v\n", err)
			}
		}

		// Configure session appearance (theme, status bar)
		if err := t.ConfigureGasTownSession(actualID, tmux.AssignTheme(r.Name), r.Name, polecatName, "polecat"); err != nil {
			if os.Getenv("GT_DEBUG_SESSION") != "" {
				fmt.Fprintf(os.Stderr, "[session-debug] ConfigureGasTownSession: %v\n", err)
			}
		}

		// Set pane-died hook for cleanup
		if err := t.SetPaneDiedHook(actualID, fmt.Sprintf("%s/%s", r.Name, polecatName)); err != nil {
			if os.Getenv("GT_DEBUG_SESSION") != "" {
				fmt.Fprintf(os.Stderr, "[session-debug] SetPaneDiedHook: %v\n", err)
			}
		}

		return nil
	})

	agents := agent.New(sess, agentCfg)
	mgr = polecat.NewSessionManager(agents, r, agentName)
	return mgr
}

// CrewManager creates a properly configured crew.Manager with Agents.
// Uses a closure pattern to wire up the OnSessionCreated callback.
func CrewManager(r *rig.Rig, townRoot, agentName string) *crew.Manager {
	g := git.NewGit(r.Path)

	// Forward declaration for closure
	var mgr *crew.Manager

	// Create tmux with TownSessions wrapper and agents with callback
	t := tmux.NewTmux()
	sess := session.NewTownSessions(t, townRoot)
	agentCfg := agent.FromPreset(agentName).WithOnSessionCreated(func(_ session.Sessions, id session.SessionID) error {
		// Apply env vars, theming, and crew bindings when session is created
		// Use captured t directly - no need to unwrap through Sessions()
		actualID := session.SessionID(session.ToUniqueHumanReadableName(id, townRoot))
		crewName := mgr.StartingCrewName()
		rigName := mgr.RigName()

		// Set environment variables
		envVars := config.AgentEnv(config.AgentEnvConfig{
			Role:          "crew",
			Rig:           rigName,
			AgentName:     crewName,
			TownRoot:      townRoot,
			BeadsNoDaemon: true,
		})
		if err := t.SetEnvVars(actualID, envVars); err != nil {
			return fmt.Errorf("setting env vars: %w", err)
		}

		// Configure session appearance (theme, status bar)
		if err := t.ConfigureGasTownSession(actualID, tmux.AssignTheme(rigName), rigName, crewName, "crew"); err != nil {
			return fmt.Errorf("configuring session: %w", err)
		}

		// Set crew cycle key bindings
		if err := t.SetCrewCycleBindings(string(actualID)); err != nil {
			return fmt.Errorf("setting crew bindings: %w", err)
		}

		return nil
	})

	agents := agent.New(sess, agentCfg)
	mgr = crew.NewManager(agents, r, g, agentName)
	return mgr
}

// splitEnvVar splits a "KEY=VALUE" string into key and value.
func splitEnvVar(s string) (key, value string, ok bool) {
	parts := strings.SplitN(s, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}
