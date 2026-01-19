package polecat

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/steveyegge/gastown/internal/agent"
	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/claude"
	"github.com/steveyegge/gastown/internal/config"
	"github.com/steveyegge/gastown/internal/rig"
	"github.com/steveyegge/gastown/internal/runner"
	"github.com/steveyegge/gastown/internal/tmux"
	"github.com/steveyegge/gastown/internal/workspace"
)

// RemoteConfig holds configuration for remote polecat operations.
type RemoteConfig struct {
	// SSHCmd is the SSH command to reach the remote (e.g., "ssh user@host").
	SSHCmd string

	// LocalSSH is the SSH command the remote uses to call back to local.
	LocalSSH string

	// RemoteRigPath is where the rig lives on the remote.
	// If empty, uses ~/rigs/<rigname>.
	RemoteRigPath string
}

// RemoteManager handles polecat lifecycle on a remote machine.
// Worktrees are created on the remote via SSH.
// Agent beads are still managed locally (beads database is always local).
type RemoteManager struct {
	rig      *rig.Rig
	namePool *NamePool
	agents   agent.Agents
	beads    *beads.Beads
	config   RemoteConfig
	fs       Filesystem        // SSH filesystem for remote operations
	localFS  Filesystem        // Local filesystem for reading source files
	gitOps   *runner.GitOps    // Git operations via SSH
}

// NewRemoteManager creates a new remote polecat manager.
func NewRemoteManager(r *rig.Rig, cfg RemoteConfig) *RemoteManager {
	sshRunner := runner.NewSSH(cfg.SSHCmd)
	return NewRemoteManagerWithFilesystem(r, cfg, NewRemoteFilesystem(sshRunner), NewLocalFilesystem(), runner.NewGitOps(sshRunner))
}

// NewRemoteManagerWithFilesystem creates a new remote polecat manager with custom filesystems.
// This is useful for testing.
func NewRemoteManagerWithFilesystem(r *rig.Rig, cfg RemoteConfig, remoteFS, localFS Filesystem, gitOps *runner.GitOps) *RemoteManager {
	// Use the resolved beads directory to find where bd commands should run.
	resolvedBeads := beads.ResolveBeadsDir(r.Path)
	beadsPath := filepath.Dir(resolvedBeads)

	// Load namepool settings
	settingsPath := filepath.Join(r.Path, "settings", "config.json")
	var pool *NamePool

	settings, err := config.LoadRigSettings(settingsPath)
	if err == nil && settings.Namepool != nil {
		pool = NewNamePoolWithConfig(
			r.Path,
			r.Name,
			settings.Namepool.Style,
			settings.Namepool.Names,
			settings.Namepool.MaxBeforeNumbering,
		)
	} else {
		pool = NewNamePool(r.Path, r.Name)
	}
	_ = pool.Load()

	// Set default remote rig path
	if cfg.RemoteRigPath == "" {
		cfg.RemoteRigPath = fmt.Sprintf("~/rigs/%s", r.Name)
	}

	// Create agents with RemoteTmux for session operations
	sess := tmux.NewRemoteTmuxWithCallback(cfg.SSHCmd, cfg.LocalSSH)
	preset := agent.Claude() // Will be overridden in Start()

	return &RemoteManager{
		rig:      r,
		namePool: pool,
		agents:   agent.New(sess, preset),
		fs:       remoteFS,
		localFS:  localFS,
		gitOps:   gitOps,
		beads:    beads.NewWithBeadsDir(beadsPath, resolvedBeads),
		config:   cfg,
	}
}

// RigName returns the rig name for this manager.
func (m *RemoteManager) RigName() string {
	return m.rig.Name
}

// Exists checks if a polecat exists on the remote.
func (m *RemoteManager) Exists(name string) bool {
	return m.fs.IsDir(m.remotePolecatDir(name))
}

// Get returns polecat info.
// Since worktrees are remote, we return basic info based on what we know locally.
func (m *RemoteManager) Get(name string) (*Polecat, error) {
	if !m.Exists(name) {
		return nil, ErrPolecatNotFound
	}

	// Get branch name from remote
	remotePath := m.remoteClonePath(name)
	branchName, err := m.gitOps.CurrentBranch(remotePath)
	if err != nil || branchName == "" {
		branchName = fmt.Sprintf("polecat/%s", name)
	}

	return &Polecat{
		Name:      name,
		Rig:       m.rig.Name,
		State:     StateWorking,
		ClonePath: remotePath, // This is the remote path
		Branch:    branchName,
	}, nil
}

// AllocateName allocates a name from the name pool.
func (m *RemoteManager) AllocateName() (string, error) {
	m.reconcilePool()

	name, err := m.namePool.Allocate()
	if err != nil {
		return "", err
	}

	if err := m.namePool.Save(); err != nil {
		return "", fmt.Errorf("saving pool state: %w", err)
	}

	return name, nil
}

// reconcilePool syncs the name pool with remote state.
func (m *RemoteManager) reconcilePool() {
	// List remote polecats directories
	polecatsDir := fmt.Sprintf("%s/polecats", m.config.RemoteRigPath)
	entries, err := m.fs.ReadDir(polecatsDir)
	if err != nil {
		return
	}

	var namesWithDirs []string
	for _, entry := range entries {
		name := entry.Name()
		if name != "" && !strings.HasPrefix(name, ".") && entry.IsDir() {
			namesWithDirs = append(namesWithDirs, name)
		}
	}

	m.namePool.Reconcile(namesWithDirs)
}

// AddWithOptions creates a new polecat on the remote machine.
func (m *RemoteManager) AddWithOptions(name string, opts AddOptions) (*Polecat, error) {
	if m.Exists(name) {
		return nil, ErrPolecatExists
	}

	remotePolecatDir := m.remotePolecatDir(name)
	remoteClonePath := m.remoteClonePath(name)

	// Generate branch name
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 36)
	var branchName string
	if opts.HookBead != "" {
		branchName = fmt.Sprintf("polecat/%s/%s@%s", name, opts.HookBead, timestamp)
	} else {
		branchName = fmt.Sprintf("polecat/%s-%s", name, timestamp)
	}

	// Determine start point (default branch)
	defaultBranch := "main"
	if rigCfg, err := rig.LoadRigConfig(m.rig.Path); err == nil && rigCfg.DefaultBranch != "" {
		defaultBranch = rigCfg.DefaultBranch
	}
	startPoint := fmt.Sprintf("origin/%s", defaultBranch)

	// Create polecat directory on remote
	if err := m.fs.MkdirAll(remotePolecatDir, 0755); err != nil {
		return nil, fmt.Errorf("creating remote polecat dir: %w", err)
	}

	// The remote must have .repo.git already set up
	remoteRepoPath := fmt.Sprintf("%s/.repo.git", m.config.RemoteRigPath)

	// Fetch latest from origin (non-fatal)
	_ = m.gitOps.Fetch(remoteRepoPath, "origin")

	// Create worktree
	if err := m.gitOps.WorktreeAdd(remoteRepoPath, remoteClonePath, branchName, startPoint); err != nil {
		return nil, fmt.Errorf("creating remote worktree: %w", err)
	}

	// Copy AGENTS.md if it exists locally (from mayor/rig)
	localAgentsMD := filepath.Join(m.rig.Path, "mayor", "rig", "AGENTS.md")
	if err := m.copyFileToRemote(localAgentsMD, filepath.Join(remoteClonePath, "AGENTS.md")); err != nil {
		fmt.Printf("Warning: could not copy AGENTS.md to remote: %v\n", err)
	}

	// Copy .claude/settings.json for prehooks
	if err := m.copyClaudeSettings(remoteClonePath); err != nil {
		fmt.Printf("Warning: could not copy Claude settings to remote: %v\n", err)
	}

	// Copy overlay files from .runtime/overlay/
	overlayDir := filepath.Join(m.rig.Path, ".runtime", "overlay")
	if err := m.copyDirToRemote(overlayDir, remoteClonePath); err != nil {
		fmt.Printf("Warning: could not copy overlay files to remote: %v\n", err)
	}

	// Set up beads redirect on remote (points to local beads via SSH callback)
	if err := m.setupRemoteBeadsRedirect(remoteClonePath); err != nil {
		fmt.Printf("Warning: could not set up beads redirect on remote: %v\n", err)
	}

	// Create agent bead locally (beads database is always local)
	agentID := m.agentBeadID(name)
	_, err := m.beads.CreateOrReopenAgentBead(agentID, agentID, &beads.AgentFields{
		RoleType:   "polecat",
		Rig:        m.rig.Name,
		AgentState: "spawning",
		RoleBead:   beads.RoleBeadIDTown("polecat"),
		HookBead:   opts.HookBead,
	})
	if err != nil {
		fmt.Printf("Warning: could not create agent bead: %v\n", err)
	}

	now := time.Now()
	return &Polecat{
		Name:      name,
		Rig:       m.rig.Name,
		State:     StateWorking,
		ClonePath: remoteClonePath,
		Branch:    branchName,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// Remove deletes a polecat from the remote machine.
func (m *RemoteManager) Remove(name string, force bool) error {
	if !m.Exists(name) {
		return ErrPolecatNotFound
	}

	remoteClonePath := m.remoteClonePath(name)
	remotePolecatDir := m.remotePolecatDir(name)
	remoteRepoPath := fmt.Sprintf("%s/.repo.git", m.config.RemoteRigPath)

	// Try to remove as a worktree first
	if err := m.gitOps.WorktreeRemove(remoteRepoPath, remoteClonePath, force); err != nil {
		// Fall back to direct removal
		if err := m.fs.RemoveAll(remoteClonePath); err != nil {
			return fmt.Errorf("removing remote polecat: %w", err)
		}
	}

	// Also remove the parent polecat directory
	_ = m.fs.RemoveAll(remotePolecatDir)

	// Prune stale worktree entries
	_ = m.gitOps.WorktreePrune(remoteRepoPath)

	// Release name back to pool
	m.namePool.Release(name)
	_ = m.namePool.Save()

	// Close agent bead locally
	agentID := m.agentBeadID(name)
	if err := m.beads.CloseAndClearAgentBead(agentID, "polecat removed"); err != nil {
		// Non-fatal - may not exist
	}

	return nil
}

// Start starts the polecat's session on the remote machine.
func (m *RemoteManager) Start(name string) error {
	if !m.Exists(name) {
		return ErrPolecatNotFound
	}

	id := m.agentID(name)
	if m.agents.Exists(id) {
		return agent.ErrAlreadyRunning
	}

	aiRuntime := m.resolveAIRuntimeName()
	startCfg := agent.StartConfig{
		WorkDir: m.remoteClonePath(name),
		Command: config.BuildAgentCommand(aiRuntime, ""),
	}

	if err := m.agents.StartWithConfig(id, startCfg); err != nil {
		return fmt.Errorf("starting remote session: %w", err)
	}

	return m.agents.WaitReady(id)
}

// SessionName returns the tmux session name for a polecat.
func (m *RemoteManager) SessionName(name string) string {
	return fmt.Sprintf("gt-%s-%s", m.rig.Name, name)
}

// agentID returns the AgentID for a polecat.
func (m *RemoteManager) agentID(name string) agent.AgentID {
	return agent.PolecatAddress(m.rig.Name, name)
}

// agentBeadID returns the agent bead ID for a polecat.
func (m *RemoteManager) agentBeadID(name string) string {
	townRoot, err := workspace.Find(m.rig.Path)
	if err != nil || townRoot == "" {
		return beads.PolecatBeadID(m.rig.Name, name)
	}
	prefix := beads.GetPrefixForRig(townRoot, m.rig.Name)
	return beads.PolecatBeadIDWithPrefix(prefix, m.rig.Name, name)
}

// resolveAIRuntimeName returns the configured AI runtime for polecats in this rig.
func (m *RemoteManager) resolveAIRuntimeName() string {
	townRoot, err := workspace.Find(m.rig.Path)
	if err != nil || townRoot == "" {
		return "claude"
	}
	name, _ := config.ResolveRoleAgentName("polecat", townRoot, m.rig.Path)
	if name == "" {
		return "claude"
	}
	return name
}

// remotePolecatDir returns the polecat home directory path on the remote.
func (m *RemoteManager) remotePolecatDir(name string) string {
	return fmt.Sprintf("%s/polecats/%s", m.config.RemoteRigPath, name)
}

// remoteClonePath returns the worktree path on the remote.
func (m *RemoteManager) remoteClonePath(name string) string {
	return fmt.Sprintf("%s/polecats/%s/%s", m.config.RemoteRigPath, name, m.rig.Name)
}

// copyFileToRemote copies a local file to the remote machine.
func (m *RemoteManager) copyFileToRemote(localPath, remotePath string) error {
	content, err := m.localFS.ReadFile(localPath)
	if err != nil {
		return err
	}
	return m.fs.WriteFile(remotePath, content, 0644)
}

// copyDirToRemote copies files from a local directory to the remote.
// This is a simplified version that copies individual files.
func (m *RemoteManager) copyDirToRemote(localDir, remoteDir string) error {
	if !m.localFS.Exists(localDir) {
		return nil // Source doesn't exist, nothing to copy
	}

	entries, err := m.localFS.ReadDir(localDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		localPath := filepath.Join(localDir, entry.Name())
		remotePath := fmt.Sprintf("%s/%s", remoteDir, entry.Name())

		if entry.IsDir() {
			// Recursively copy directory
			if err := m.copyDirToRemote(localPath, remotePath); err != nil {
				return err
			}
		} else {
			// Copy file
			if err := m.copyFileToRemote(localPath, remotePath); err != nil {
				return err
			}
		}
	}
	return nil
}

// copyClaudeSettings copies .claude/settings.json to the remote.
func (m *RemoteManager) copyClaudeSettings(remoteClonePath string) error {
	settingsContent, err := claude.SettingsContentFor("polecat")
	if err != nil {
		return err
	}

	// Create .claude directory
	claudeDir := fmt.Sprintf("%s/.claude", remoteClonePath)
	if err := m.fs.MkdirAll(claudeDir, 0755); err != nil {
		return err
	}

	// Write settings file
	settingsPath := fmt.Sprintf("%s/.claude/settings.json", remoteClonePath)
	return m.fs.WriteFile(settingsPath, settingsContent, 0644)
}

// setupRemoteBeadsRedirect sets up beads redirect on the remote.
// The redirect points beads commands to use the gt-wrapper which SSHes back to local.
func (m *RemoteManager) setupRemoteBeadsRedirect(remoteClonePath string) error {
	// Create .beads directory
	beadsDir := fmt.Sprintf("%s/.beads", remoteClonePath)
	if err := m.fs.MkdirAll(beadsDir, 0755); err != nil {
		return err
	}

	// Create redirect file that tells bd-wrapper to SSH back
	redirectPath := fmt.Sprintf("%s/.beads/redirect", remoteClonePath)
	return m.fs.WriteFile(redirectPath, []byte("remote\n"), 0644)
}

// List returns all polecats in this rig.
func (m *RemoteManager) List() ([]*Polecat, error) {
	polecatsDir := fmt.Sprintf("%s/polecats", m.config.RemoteRigPath)
	entries, err := m.fs.ReadDir(polecatsDir)
	if err != nil {
		// Directory may not exist yet
		return nil, nil
	}

	var polecats []*Polecat
	for _, entry := range entries {
		name := entry.Name()
		if name == "" || strings.HasPrefix(name, ".") || !entry.IsDir() {
			continue
		}

		p, err := m.Get(name)
		if err != nil {
			continue // Skip polecats we can't read
		}
		polecats = append(polecats, p)
	}
	return polecats, nil
}

// RemoveWithOptions removes a polecat with control over safety checks.
// force: bypass uncommitted changes check (but not stashes/unpushed)
// nuclear: bypass ALL safety checks including stashes and unpushed commits
func (m *RemoteManager) RemoveWithOptions(name string, force, nuclear bool) error {
	if !m.Exists(name) {
		return ErrPolecatNotFound
	}

	remoteClonePath := m.remoteClonePath(name)
	remotePolecatDir := m.remotePolecatDir(name)
	remoteRepoPath := fmt.Sprintf("%s/.repo.git", m.config.RemoteRigPath)

	// Check for uncommitted work unless bypassed
	if !nuclear {
		state, err := m.GitState(name)
		if err == nil && !state.Clean {
			if force {
				// Force mode: allow uncommitted changes but still block on stashes/unpushed
				if state.StashCount > 0 || state.UnpushedCommits > 0 {
					return ErrHasUncommittedWork
				}
			} else {
				return ErrHasUncommittedWork
			}
		}
	}

	// Try to remove as a worktree first
	if err := m.gitOps.WorktreeRemove(remoteRepoPath, remoteClonePath, force); err != nil {
		// Fall back to direct removal
		if err := m.fs.RemoveAll(remoteClonePath); err != nil {
			return fmt.Errorf("removing remote polecat: %w", err)
		}
	}

	// Also remove the parent polecat directory
	_ = m.fs.RemoveAll(remotePolecatDir)

	// Prune stale worktree entries
	_ = m.gitOps.WorktreePrune(remoteRepoPath)

	// Release name back to pool
	m.namePool.Release(name)
	_ = m.namePool.Save()

	// Close agent bead locally (beads are always local)
	agentID := m.agentBeadID(name)
	if err := m.beads.CloseAndClearAgentBead(agentID, "polecat removed"); err != nil {
		// Non-fatal - may not exist
	}

	return nil
}

// GitState returns the git state of a polecat's worktree.
// Used for pre-kill verification to ensure no work is lost.
func (m *RemoteManager) GitState(name string) (*GitState, error) {
	if !m.Exists(name) {
		return nil, ErrPolecatNotFound
	}

	clonePath := m.remoteClonePath(name)

	state := &GitState{
		Clean:            true,
		UncommittedFiles: []string{},
	}

	// Check for uncommitted changes using GitOps.Status
	files, err := m.gitOps.Status(clonePath)
	if err == nil && len(files) > 0 {
		state.UncommittedFiles = files
		state.Clean = false
	}

	// Check for unpushed commits
	unpushed, err := m.gitOps.UnpushedCommitCount(clonePath)
	if err == nil && unpushed > 0 {
		// Check if there's any actual content difference (handle squash merges)
		hasDiff, _ := m.gitOps.HasContentDiffFromRef(clonePath, "origin/main")
		if hasDiff {
			state.UnpushedCommits = unpushed
			state.Clean = false
		}
	}

	// Check for stashes
	stashCount, _ := m.gitOps.StashCount(clonePath)
	if stashCount > 0 {
		state.StashCount = stashCount
		state.Clean = false
	}

	return state, nil
}

// Sync runs bd sync in the polecat's worktree.
// fromMain: only pull changes, don't push
func (m *RemoteManager) Sync(name string, fromMain bool) error {
	if !m.Exists(name) {
		return ErrPolecatNotFound
	}

	clonePath := m.remoteClonePath(name)

	// Build sync command - bd commands are run via bd-wrapper on remote
	// which SSHes back to local for beads operations
	args := []string{"sync"}
	if fromMain {
		args = append(args, "--from-main")
	}

	// Use the SSH runner to execute bd on remote
	sshRunner := runner.NewSSH(m.config.SSHCmd)
	return sshRunner.Run(clonePath, "bd", args...)
}

// CleanupStaleBranches removes orphaned polecat branches.
// Returns the number of branches deleted.
func (m *RemoteManager) CleanupStaleBranches() (int, error) {
	remoteRepoPath := fmt.Sprintf("%s/.repo.git", m.config.RemoteRigPath)

	// List all polecat branches
	branches, err := m.gitOps.ListBranches(remoteRepoPath, "polecat/*")
	if err != nil {
		return 0, fmt.Errorf("listing branches: %w", err)
	}

	if len(branches) == 0 {
		return 0, nil
	}

	// Get current polecats
	polecats, err := m.List()
	if err != nil {
		return 0, fmt.Errorf("listing polecats: %w", err)
	}

	// Build map of current branches
	currentBranches := make(map[string]bool)
	for _, p := range polecats {
		currentBranches[p.Branch] = true
	}

	// Delete orphaned branches
	deleted := 0
	for _, branch := range branches {
		if !currentBranches[branch] {
			if err := m.gitOps.DeleteBranch(remoteRepoPath, branch, true); err == nil {
				deleted++
			}
		}
	}

	return deleted, nil
}

// DetectStalePolecats identifies polecats that may need cleanup.
// threshold: number of commits behind main to consider stale
func (m *RemoteManager) DetectStalePolecats(threshold int) ([]*StalenessInfo, error) {
	polecats, err := m.List()
	if err != nil {
		return nil, fmt.Errorf("listing polecats: %w", err)
	}

	if len(polecats) == 0 {
		return nil, nil
	}

	// Get default branch from rig config
	defaultBranch := "main"
	if rigCfg, err := rig.LoadRigConfig(m.rig.Path); err == nil && rigCfg.DefaultBranch != "" {
		defaultBranch = rigCfg.DefaultBranch
	}

	var results []*StalenessInfo
	for _, p := range polecats {
		info := &StalenessInfo{
			Name: p.Name,
		}

		// Check for active tmux session via agents interface
		agentID := m.agentID(p.Name)
		info.HasActiveSession = m.agents.Exists(agentID)

		// Check how far behind main
		clonePath := m.remoteClonePath(p.Name)
		remoteBranch := "origin/" + defaultBranch
		info.CommitsBehind, _ = m.gitOps.CountCommitsBehind(clonePath, remoteBranch)

		// Check for uncommitted work
		gitState, err := m.GitState(p.Name)
		if err == nil && !gitState.Clean {
			info.HasUncommittedWork = true
		}

		// Check agent bead state (beads are local)
		agentBeadID := m.agentBeadID(p.Name)
		_, fields, err := m.beads.GetAgentBead(agentBeadID)
		if err == nil && fields != nil {
			info.AgentState = fields.AgentState
		}

		// Determine staleness using same logic as Manager
		info.IsStale, info.Reason = assessStaleness(info, threshold)
		results = append(results, info)
	}

	return results, nil
}
