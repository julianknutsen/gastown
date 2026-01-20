// Package cmd provides polecat spawning utilities for gt sling.
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/steveyegge/gastown/internal/agent"
	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/config"
	"github.com/steveyegge/gastown/internal/events"
	"github.com/steveyegge/gastown/internal/factory"
	"github.com/steveyegge/gastown/internal/git"
	"github.com/steveyegge/gastown/internal/polecat"
	"github.com/steveyegge/gastown/internal/rig"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/workspace"
)

// SpawnedPolecatInfo contains info about a spawned polecat session.
type SpawnedPolecatInfo struct {
	RigName        string // Rig name (e.g., "gastown")
	PolecatName    string // Polecat name (e.g., "Toast")
	ClonePath      string // Path to polecat's git worktree
	SessionName    string // Tmux session name (e.g., "gt-gastown-p-Toast")
	TownRoot       string // Town root path (needed for deferred session start)
	SessionStarted bool   // Whether session was started (false if DeferSession)
}

// AgentID returns the agent identifier (e.g., "gastown/polecats/Toast")
func (s *SpawnedPolecatInfo) AgentID() string {
	return fmt.Sprintf("%s/polecats/%s", s.RigName, s.PolecatName)
}

// SlingSpawnOptions contains options for spawning a polecat via sling.
type SlingSpawnOptions struct {
	Force        bool   // Force spawn even if polecat has uncommitted work
	Account      string // Claude Code account handle to use
	Create       bool   // Create polecat if it doesn't exist (currently always true for sling)
	HookBead     string // Bead ID to set as hook_bead at spawn time (atomic assignment)
	Agent        string // Agent override for this spawn (e.g., "gemini", "codex", "claude-haiku")
	DeferSession bool   // Don't start session - caller will start after hooking bead
}

// SpawnPolecatForSling creates a fresh polecat and optionally starts its session.
// This is used by gt sling when the target is a rig name.
// The caller (sling) handles hook attachment and nudging.
//
// Uses polecat.BackendFor() to automatically select local or remote backend
// based on rig configuration.
func SpawnPolecatForSling(rigName string, opts SlingSpawnOptions) (*SpawnedPolecatInfo, error) {
	// Find workspace
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return nil, fmt.Errorf("not in a Gas Town workspace: %w", err)
	}

	// Load rig config
	rigsConfigPath := filepath.Join(townRoot, "mayor", "rigs.json")
	rigsConfig, err := config.LoadRigsConfig(rigsConfigPath)
	if err != nil {
		rigsConfig = &config.RigsConfig{Rigs: make(map[string]config.RigEntry)}
	}

	g := git.NewGit(townRoot)
	rigMgr := rig.NewManager(townRoot, rigsConfig, g)
	r, err := rigMgr.GetRig(rigName)
	if err != nil {
		return nil, fmt.Errorf("rig '%s' not found", rigName)
	}

	// Get backend (local or remote based on rig config)
	polecatGit := git.NewGit(r.Path)
	backend := polecat.BackendFor(r, polecatGit)

	// Allocate a new polecat name
	polecatName, err := backend.AllocateName()
	if err != nil {
		return nil, fmt.Errorf("allocating polecat name: %w", err)
	}
	fmt.Printf("Allocated polecat: %s\n", polecatName)

	// Build add options with hook_bead set atomically at spawn time
	addOpts := polecat.AddOptions{
		HookBead: opts.HookBead,
	}

	// Check if polecat already exists (shouldn't happen - indicates stale state needing repair)
	if backend.Exists(polecatName) {
		// Stale state: polecat exists despite fresh name allocation - remove and recreate
		// Use nuclear=true to bypass all safety checks (stale polecats shouldn't block new work)
		fmt.Printf("Repairing stale polecat %s with fresh worktree...\n", polecatName)
		if err := backend.RemoveWithOptions(polecatName, true, true); err != nil {
			return nil, fmt.Errorf("removing stale polecat: %w", err)
		}
	}

	// Create new polecat
	fmt.Printf("Creating polecat %s...\n", polecatName)
	polecatObj, err := backend.AddWithOptions(polecatName, addOpts)
	if err != nil {
		// Handle race condition: another process may have created the polecat
		// between our Exists() check and AddWithOptions() call
		if err == polecat.ErrPolecatExists {
			fmt.Printf("Repairing polecat %s (concurrent creation detected)...\n", polecatName)
			// Remove and recreate - use nuclear=true to bypass all safety checks
			if removeErr := backend.RemoveWithOptions(polecatName, true, true); removeErr != nil {
				return nil, fmt.Errorf("removing polecat for repair: %w", removeErr)
			}
			polecatObj, err = backend.AddWithOptions(polecatName, addOpts)
			if err != nil {
				return nil, fmt.Errorf("recreating polecat: %w", err)
			}
		} else {
			return nil, fmt.Errorf("creating polecat: %w", err)
		}
	}

	// For remote polecats, create a local mirror directory for lock files and state
	// The actual worktree is on the remote, but gt prime needs a local .runtime/ dir
	localPolecatDir := filepath.Join(r.Path, "polecats", polecatName)
	if err := os.MkdirAll(filepath.Join(localPolecatDir, ".runtime"), 0755); err != nil {
		fmt.Printf("Warning: could not create local mirror directory: %v\n", err)
	}

	sessionName := backend.SessionName(polecatName)
	sessionStarted := false

	// Start the session unless caller wants to defer (hook-before-spawn pattern)
	// Always use factory.Start which goes through SessionFactory for proper
	// local vs remote session routing (MirroredSessions for remote polecats)
	if !opts.DeferSession {
		id := agent.PolecatAddress(rigName, polecatName)
		factoryAgents := factory.AgentsFor(townRoot, id)
		if !factoryAgents.Exists(id) {
			fmt.Printf("Starting session for %s/%s...\n", rigName, polecatName)
			var startOpts []factory.StartOption
			if opts.Agent != "" {
				startOpts = append(startOpts, factory.WithAgent(opts.Agent))
			}
			if _, err := factory.Start(townRoot, id, startOpts...); err != nil {
				return nil, fmt.Errorf("starting session: %w", err)
			}
		}
		sessionStarted = true
	}

	fmt.Printf("%s Polecat %s spawned\n", style.Bold.Render("✓"), polecatName)

	// Log spawn event to activity feed
	_ = events.LogFeed(events.TypeSpawn, "gt", events.SpawnPayload(rigName, polecatName))

	return &SpawnedPolecatInfo{
		RigName:        rigName,
		PolecatName:    polecatName,
		ClonePath:      polecatObj.ClonePath,
		SessionName:    sessionName,
		TownRoot:       townRoot,
		SessionStarted: sessionStarted,
	}, nil
}

// IsRigName checks if a target string is a rig name (not a role or path).
// Returns the rig name and true if it's a valid rig.
func IsRigName(target string) (string, bool) {
	// If it contains a slash, it's a path format (rig/role or rig/crew/name)
	if strings.Contains(target, "/") {
		return "", false
	}

	// Check known non-rig role names
	switch strings.ToLower(target) {
	case "mayor", "may", "deacon", "dea", "crew", "witness", "wit", "refinery", "ref":
		return "", false
	}

	// Try to load as a rig
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return "", false
	}

	rigsConfigPath := filepath.Join(townRoot, "mayor", "rigs.json")
	rigsConfig, err := config.LoadRigsConfig(rigsConfigPath)
	if err != nil {
		return "", false
	}

	g := git.NewGit(townRoot)
	rigMgr := rig.NewManager(townRoot, rigsConfig, g)
	_, err = rigMgr.GetRig(target)
	if err != nil {
		return "", false
	}

	return target, true
}

// SpawnAndHookOptions contains options for the complete spawn+hook workflow.
type SpawnAndHookOptions struct {
	// Spawn options
	Force   bool   // Force spawn even if polecat has uncommitted work
	Account string // Claude Code account handle
	Create  bool   // Create polecat if it doesn't exist
	Agent   string // Agent override (e.g., "gemini", "codex")

	// Hook options
	Subject string // Subject for start prompt
	Args    string // Args for start prompt

	// Event logging
	LogEvent bool // Whether to log sling event to feed
}

// SpawnAndHookResult contains the result of a spawn+hook operation.
type SpawnAndHookResult struct {
	TargetAgent string // Agent ID (e.g., "gastown/polecats/Toast")
	PolecatName string // Polecat name
	HookWorkDir string // Path to polecat's worktree
}

// SpawnAndHookBead is the common function for spawning a polecat and hooking a bead.
// This handles the complete workflow using hook-before-spawn pattern:
// 1. Create polecat (without starting session)
// 2. Hook bead via bd update (so polecat finds work ready)
// 3. Start session (polecat wakes up with work already hooked)
// 4. Update state, attach molecule, nudge
// Used by both queue dispatch and batch sling.
func SpawnAndHookBead(townRoot, rigName, beadID string, opts SpawnAndHookOptions) (*SpawnAndHookResult, error) {
	// Step 1: Spawn polecat with deferred session start
	spawnOpts := SlingSpawnOptions{
		Force:        opts.Force,
		Account:      opts.Account,
		Create:       opts.Create,
		HookBead:     beadID,
		Agent:        opts.Agent,
		DeferSession: true, // Don't start session yet - hook first
	}
	spawnInfo, err := SpawnPolecatForSling(rigName, spawnOpts)
	if err != nil {
		return nil, fmt.Errorf("spawning polecat: %w", err)
	}

	targetAgent := spawnInfo.AgentID()
	hookWorkDir := spawnInfo.ClonePath
	townBeadsDir := filepath.Join(townRoot, ".beads")

	// Step 2: Hook the bead BEFORE starting session (avoids race condition)
	hookCmd := exec.Command("bd", "--no-daemon", "update", beadID, "--status=hooked", "--assignee="+targetAgent)
	hookCmd.Dir = resolveHookDir(townRoot, beadID, hookWorkDir)
	if err := hookCmd.Run(); err != nil {
		return nil, fmt.Errorf("hooking bead: %w", err)
	}

	// Step 3: Now start the session (polecat will find its bead already hooked)
	id := agent.PolecatAddress(rigName, spawnInfo.PolecatName)
	factoryAgents := factory.AgentsFor(townRoot, id)
	if !factoryAgents.Exists(id) {
		fmt.Printf("Starting session for %s/%s...\n", rigName, spawnInfo.PolecatName)
		var startOpts []factory.StartOption
		if opts.Agent != "" {
			startOpts = append(startOpts, factory.WithAgent(opts.Agent))
		}
		if _, err := factory.Start(townRoot, id, startOpts...); err != nil {
			return nil, fmt.Errorf("starting session: %w", err)
		}
	}

	// Log sling event if requested
	if opts.LogEvent {
		actor := detectActor()
		_ = events.LogFeed(events.TypeSling, actor, events.SlingPayload(beadID, targetAgent))
	}

	// Update agent bead state
	updateAgentHookBead(targetAgent, beadID, hookWorkDir, townBeadsDir)

	// Auto-attach work molecule
	_ = attachPolecatWorkMolecule(targetAgent, hookWorkDir, townRoot)

	// Nudge the polecat
	agentID, err := addressToAgentID(targetAgent)
	if err == nil {
		_ = ensureAgentReady(townRoot, agentID)
		_ = injectStartPrompt(townRoot, agentID, beadID, opts.Subject, opts.Args)
	}

	return &SpawnAndHookResult{
		TargetAgent: targetAgent,
		PolecatName: spawnInfo.PolecatName,
		HookWorkDir: hookWorkDir,
	}, nil
}

// resolveHookDir determines the directory for running bd update on a bead.
// This is a local wrapper around beads.ResolveHookDir to avoid import cycles.
func resolveHookDir(townRoot, beadID, hookWorkDir string) string {
	// Use beads package function via import
	return beads.ResolveHookDir(townRoot, beadID, hookWorkDir)
}
