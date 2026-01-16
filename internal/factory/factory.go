package factory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/steveyegge/gastown/internal/agent"
	"github.com/steveyegge/gastown/internal/config"
	"github.com/steveyegge/gastown/internal/constants"
	"github.com/steveyegge/gastown/internal/crew"
	"github.com/steveyegge/gastown/internal/git"
	"github.com/steveyegge/gastown/internal/polecat"
	"github.com/steveyegge/gastown/internal/refinery"
	"github.com/steveyegge/gastown/internal/rig"
	"github.com/steveyegge/gastown/internal/runtime"
	"github.com/steveyegge/gastown/internal/session"
	"github.com/steveyegge/gastown/internal/tmux"
	"github.com/steveyegge/gastown/internal/witness"
)

// =============================================================================
// Singleton Agent Operations
//
// Agents() returns an Agents interface for interacting with any agent in a town.
// Start() starts a singleton agent with full production setup.
// =============================================================================

// Agents returns an Agents interface for a town.
// Use this for operations like Stop, Nudge, Capture that work with any AgentID.
// The returned interface doesn't have role-specific configuration - use Start()
// for starting agents with proper env vars and settings.
func Agents(townRoot string) agent.Agents {
	t := tmux.NewTmux()
	sess := session.NewTownSessions(t, townRoot)
	return agent.New(sess, agent.Claude())
}

// StartOption configures Start() behavior for specific roles.
type StartOption func(*startConfig)

// startConfig holds optional Start() configuration.
type startConfig struct {
	topic        string            // Crew: startup topic for /resume beacon
	interactive  bool              // Crew: remove --dangerously-skip-permissions
	killExisting bool              // Stop existing session first
	envOverrides map[string]string // Additional env vars (e.g., witness overrides)
}

// WithTopic sets the startup topic for crew agents.
func WithTopic(topic string) StartOption {
	return func(c *startConfig) {
		c.topic = topic
	}
}

// WithInteractive enables interactive mode (no --dangerously-skip-permissions).
func WithInteractive() StartOption {
	return func(c *startConfig) {
		c.interactive = true
	}
}

// WithKillExisting stops any existing session before starting.
func WithKillExisting() StartOption {
	return func(c *startConfig) {
		c.killExisting = true
	}
}

// WithEnvOverrides adds additional environment variables.
func WithEnvOverrides(overrides map[string]string) StartOption {
	return func(c *startConfig) {
		c.envOverrides = overrides
	}
}

// Start starts any agent with full production setup.
// Works for all agent types: singletons, rig-level, and named workers.
//
// Usage:
//
//	factory.Start(townRoot, agent.MayorAddress, aiRuntime)
//	factory.Start(townRoot, agent.WitnessAddress("myrig"), aiRuntime)
//	factory.Start(townRoot, agent.PolecatAddress("myrig", "toast"), aiRuntime)
//	factory.Start(townRoot, agent.CrewAddress("myrig", "joe"), aiRuntime, WithTopic("patrol"))
func Start(townRoot string, id agent.AgentID, aiRuntime string, opts ...StartOption) (agent.AgentID, error) {
	// Apply options
	cfg := &startConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	role, rigName, worker := id.Parse()

	// Compute workDir
	workDir, err := WorkDirForID(townRoot, id)
	if err != nil {
		return "", err
	}

	// Build env vars from parsed components
	envCfg := config.AgentEnvConfig{
		Role:     role,
		TownRoot: townRoot,
	}
	if rigName != "" {
		envCfg.Rig = rigName
		envCfg.BeadsNoDaemon = true // Rig-level agents don't use daemon
	}
	if worker != "" {
		envCfg.AgentName = worker
	}
	envVars := config.AgentEnv(envCfg)

	// Apply env overrides
	for k, v := range cfg.envOverrides {
		envVars[k] = v
	}

	// Ensure runtime settings exist
	runtimeConfig := config.LoadRuntimeConfig(townRoot)
	if err := runtime.EnsureSettingsForRole(workDir, role, runtimeConfig); err != nil {
		return "", fmt.Errorf("ensuring runtime settings: %w", err)
	}

	// Build startup command
	startupCmd := buildCommand(role, aiRuntime, cfg)

	// Create agents with production configuration
	agents := createAgentsForID(townRoot, aiRuntime, envVars)

	// Kill existing session if requested
	if cfg.killExisting {
		_ = agents.Stop(id, true)
	}

	// Build StartConfig with OnCreated callback for theming
	startCfg := agent.StartConfig{
		WorkDir:   workDir,
		Command:   startupCmd,
		EnvVars:   envVars,
		OnCreated: buildSessionConfigurer(id, envVars),
	}
	if err := agents.StartWithConfig(id, startCfg); err != nil {
		return "", err
	}

	// Wait for agent to be ready
	_ = agents.WaitReady(id)

	return id, nil
}

// buildCommand constructs the startup command for an agent.
func buildCommand(role, aiRuntime string, cfg *startConfig) string {
	beacon := ""
	if cfg.topic != "" {
		beacon = cfg.topic
	}
	cmd := config.BuildAgentCommand(aiRuntime, beacon)

	// For interactive/refresh mode, remove --dangerously-skip-permissions
	if cfg.interactive {
		cmd = strings.Replace(cmd, " --dangerously-skip-permissions", "", 1)
	}

	return cmd
}

// buildSessionConfigurer returns OnCreated callback for tmux setup (theming, hooks).
// ALL agents need this - town-level and rig-level alike.
func buildSessionConfigurer(id agent.AgentID, envVars map[string]string) agent.OnSessionCreated {
	role, rigName, worker := id.Parse()

	return func(_ session.Sessions, sessID session.SessionID) error {
		t := tmux.NewTmux()

		// Set env vars
		if err := t.SetEnvVars(sessID, envVars); err != nil {
			return fmt.Errorf("setting env vars: %w", err)
		}

		// Compute theming parameters:
		// - Town-level (mayor, deacon): rig="", worker=role (capitalized), role=role
		// - Rig singletons (witness, refinery): rig=rigName, worker=role, role=role
		// - Named workers (polecat, crew): rig=rigName, worker=name, role=role
		var themeRig, themeWorker string
		var theme tmux.Theme
		if rigName == "" {
			// Town-level: no rig, worker is the role name
			themeRig = ""
			themeWorker = strings.Title(role)
			theme = tmux.DefaultTheme()
		} else {
			themeRig = rigName
			if worker != "" {
				themeWorker = worker // Named worker
			} else {
				themeWorker = role // Rig singleton
			}
			theme = tmux.AssignTheme(rigName)
		}

		// Apply theming
		if err := t.ConfigureGasTownSession(sessID, theme, themeRig, themeWorker, role); err != nil {
			return fmt.Errorf("configuring session: %w", err)
		}

		// Role-specific hooks (only for rig-level named agents)
		switch role {
		case constants.RolePolecat:
			if err := t.SetPaneDiedHook(sessID, fmt.Sprintf("%s/%s", rigName, worker)); err != nil {
				return fmt.Errorf("setting pane died hook: %w", err)
			}
		case constants.RoleCrew:
			if err := t.SetCrewCycleBindings(string(sessID)); err != nil {
				return fmt.Errorf("setting crew bindings: %w", err)
			}
		}

		return nil
	}
}

// createAgentsForID creates an Agents implementation with production configuration.
func createAgentsForID(townRoot, aiRuntime string, envVars map[string]string) agent.Agents {
	t := tmux.NewTmux()
	sess := session.NewTownSessions(t, townRoot)
	return agent.New(sess, agent.FromPreset(aiRuntime).WithEnvVars(envVars))
}

// WorkDirForRole computes the working directory for a singleton agent role.
func WorkDirForRole(townRoot, role string) (string, error) {
	switch role {
	case constants.RoleMayor:
		return filepath.Join(townRoot, constants.RoleMayor), nil
	case constants.RoleDeacon:
		return filepath.Join(townRoot, constants.RoleDeacon), nil
	case constants.RoleBoot:
		return filepath.Join(townRoot, "deacon", "dogs", constants.RoleBoot), nil
	default:
		return "", fmt.Errorf("unknown singleton role: %s", role)
	}
}

// WorkDirForID computes the working directory for any agent based on its ID.
// This is the generalized version of WorkDirForRole that works with all agent types.
//
// For rig-level agents (witness, refinery, polecat, crew), this performs
// filesystem checks to handle legacy vs new directory structures.
func WorkDirForID(townRoot string, id agent.AgentID) (string, error) {
	role, rigName, worker := id.Parse()

	switch role {
	// Town-level singletons
	case constants.RoleMayor:
		return filepath.Join(townRoot, constants.RoleMayor), nil
	case constants.RoleDeacon:
		return filepath.Join(townRoot, constants.RoleDeacon), nil
	case constants.RoleBoot:
		return filepath.Join(townRoot, "deacon", "dogs", constants.RoleBoot), nil

	// Rig-level singletons
	case constants.RoleWitness:
		if rigName == "" {
			return "", fmt.Errorf("witness requires rig name in ID")
		}
		return witnessWorkDir(townRoot, rigName), nil
	case constants.RoleRefinery:
		if rigName == "" {
			return "", fmt.Errorf("refinery requires rig name in ID")
		}
		return refineryWorkDir(townRoot, rigName), nil

	// Named agents
	case constants.RolePolecat:
		if rigName == "" || worker == "" {
			return "", fmt.Errorf("polecat requires rig and worker name in ID")
		}
		return polecatWorkDir(townRoot, rigName, worker), nil
	case constants.RoleCrew:
		if rigName == "" || worker == "" {
			return "", fmt.Errorf("crew requires rig and worker name in ID")
		}
		return crewWorkDir(townRoot, rigName, worker), nil

	default:
		return "", fmt.Errorf("unknown role in AgentID: %s", role)
	}
}

// witnessWorkDir returns the working directory for a witness.
// Prefers witness/rig/, falls back to witness/, then rig root.
func witnessWorkDir(townRoot, rigName string) string {
	rigPath := filepath.Join(townRoot, rigName)

	witnessRigDir := filepath.Join(rigPath, "witness", "rig")
	if _, err := os.Stat(witnessRigDir); err == nil {
		return witnessRigDir
	}

	witnessDir := filepath.Join(rigPath, "witness")
	if _, err := os.Stat(witnessDir); err == nil {
		return witnessDir
	}

	return rigPath
}

// refineryWorkDir returns the working directory for a refinery.
// Prefers refinery/rig/, falls back to mayor/rig (legacy).
func refineryWorkDir(townRoot, rigName string) string {
	rigPath := filepath.Join(townRoot, rigName)

	refineryRigDir := filepath.Join(rigPath, "refinery", "rig")
	if _, err := os.Stat(refineryRigDir); err == nil {
		return refineryRigDir
	}

	// Fall back to mayor/rig (legacy architecture)
	return filepath.Join(rigPath, "mayor", "rig")
}

// polecatWorkDir returns the working directory for a polecat.
// New structure: polecats/<name>/<rigname>/ - falls back to old: polecats/<name>/
func polecatWorkDir(townRoot, rigName, polecatName string) string {
	rigPath := filepath.Join(townRoot, rigName)

	// New structure: polecats/<name>/<rigname>/
	newPath := filepath.Join(rigPath, "polecats", polecatName, rigName)
	if info, err := os.Stat(newPath); err == nil && info.IsDir() {
		return newPath
	}

	// Old structure: polecats/<name>/
	return filepath.Join(rigPath, "polecats", polecatName)
}

// crewWorkDir returns the working directory for a crew member.
func crewWorkDir(townRoot, rigName, crewName string) string {
	rigPath := filepath.Join(townRoot, rigName)
	return filepath.Join(rigPath, "crew", crewName)
}


// =============================================================================
// Factory with Dependency Injection (for rig-level agents)
//
// The Factory struct is used for rig-level agents that need tmux callbacks
// for theming and hooks. Singleton agents use Create() instead.
// =============================================================================

// SessionConfigurer configures a tmux session after creation.
// Created as a closure capturing the tmux instance at factory creation time.
type SessionConfigurer func(rigName, workerName, role, sessionName string) error

// Factory creates properly configured agent managers.
// It holds shared dependencies and configuration.
type Factory struct {
	sess             session.Sessions  // Core session operations
	configureSession SessionConfigurer // Closure capturing tmux instance (nil in tests)
	townRoot         string
}

// New creates a production Factory with real tmux and sessions.
func New(townRoot string) *Factory {
	t := tmux.NewTmux()

	// Create closure capturing t - no nil checks needed, t is always valid here
	configureSession := func(rigName, workerName, role, sessionName string) error {
		actualID := session.SessionID(sessionName)

		envVars := config.AgentEnv(config.AgentEnvConfig{
			Role:          role,
			Rig:           rigName,
			AgentName:     workerName,
			TownRoot:      townRoot,
			BeadsNoDaemon: true,
		})
		if err := t.SetEnvVars(actualID, envVars); err != nil {
			return fmt.Errorf("setting env vars: %w", err)
		}

		if err := t.ConfigureGasTownSession(actualID, tmux.AssignTheme(rigName), rigName, workerName, role); err != nil {
			return fmt.Errorf("configuring session: %w", err)
		}

		// Role-specific hooks
		switch role {
		case constants.RolePolecat:
			if err := t.SetPaneDiedHook(actualID, fmt.Sprintf("%s/%s", rigName, workerName)); err != nil {
				return fmt.Errorf("setting pane died hook: %w", err)
			}
		case constants.RoleCrew:
			if err := t.SetCrewCycleBindings(string(actualID)); err != nil {
				return fmt.Errorf("setting crew bindings: %w", err)
			}
		}

		return nil
	}

	return &Factory{
		sess:             session.NewTownSessions(t, townRoot),
		configureSession: configureSession,
		townRoot:         townRoot,
	}
}

// NewWithSessions creates a Factory with an injected Sessions for testing.
// No tmux configuration happens - configureSession is nil.
func NewWithSessions(townRoot string, sess session.Sessions) *Factory {
	return &Factory{
		sess:             sess,
		configureSession: nil,
		townRoot:         townRoot,
	}
}

// =============================================================================
// Rig-Level Agent Creation
// =============================================================================

// WitnessManager creates a properly configured witness.Manager.
func (f *Factory) WitnessManager(r *rig.Rig, aiRuntime string, envOverrides ...string) *witness.Manager {
	overrides := parseEnvOverrides(envOverrides)
	agents := f.agentsForRole(constants.RoleWitness, r.Name, aiRuntime, overrides)
	return witness.NewManager(agents, r, aiRuntime)
}

// RefineryManager creates a properly configured refinery.Manager.
func (f *Factory) RefineryManager(r *rig.Rig, aiRuntime string) *refinery.Manager {
	agents := f.agentsForRole(constants.RoleRefinery, r.Name, aiRuntime, nil)
	return refinery.NewManager(agents, r, aiRuntime)
}

// PolecatSessionManager creates a properly configured polecat.SessionManager.
func (f *Factory) PolecatSessionManager(r *rig.Rig, aiRuntime string) *polecat.SessionManager {
	agents := agent.New(f.sess, agent.FromPreset(aiRuntime))
	return polecat.NewSessionManager(agents, r, aiRuntime, f.townRoot)
}

// CrewManager creates a properly configured crew.Manager.
// Note: Lifecycle operations (Start) should use factory.Start().
func (f *Factory) CrewManager(r *rig.Rig, aiRuntime string) *crew.Manager {
	g := git.NewGit(r.Path)
	agents := agent.New(f.sess, agent.FromPreset(aiRuntime))
	return crew.NewManager(agents, r, g, aiRuntime, f.townRoot)
}

// =============================================================================
// Factory Helpers
// =============================================================================

// agentsForRole creates a configured agent.Agents for a given role.
func (f *Factory) agentsForRole(role, rigName, aiRuntime string, envOverrides map[string]string) agent.Agents {
	envVars := config.AgentEnv(config.AgentEnvConfig{
		Role:     role,
		Rig:      rigName,
		TownRoot: f.townRoot,
	})
	for k, v := range envOverrides {
		envVars[k] = v
	}

	return agent.New(f.sess, agent.FromPreset(aiRuntime).WithEnvVars(envVars))
}

// =============================================================================
// Utility Functions
// =============================================================================

// parseEnvOverrides converts a list of KEY=VALUE strings to a map.
func parseEnvOverrides(overrides []string) map[string]string {
	result := make(map[string]string)
	for _, override := range overrides {
		parts := strings.SplitN(override, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}
