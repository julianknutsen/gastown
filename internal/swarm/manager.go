package swarm

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/rig"
)

// Common errors
var (
	ErrSwarmNotFound  = errors.New("swarm not found")
	ErrSwarmExists    = errors.New("swarm already exists")
	ErrInvalidState   = errors.New("invalid state transition")
	ErrNoReadyTasks   = errors.New("no ready tasks")
	ErrBeadsNotFound  = errors.New("beads not available")
)

// Manager handles swarm lifecycle operations.
// Manager is stateless - all swarm state is discovered from beads.
type Manager struct {
	rig       *rig.Rig
	beadsDir  string // Path for beads operations (git-synced)
	gitDir    string // Path for git operations (rig root)
}

// NewManager creates a new swarm manager for a rig.
func NewManager(r *rig.Rig) *Manager {
	return &Manager{
		rig:      r,
		beadsDir: r.BeadsPath(), // Use BeadsPath() for git-synced beads operations
		gitDir:   r.Path,        // Use rig root for git operations
	}
}

// LoadSwarm loads swarm state from beads by querying the epic.
// This is the canonical way to get swarm state - no in-memory caching.
func (m *Manager) LoadSwarm(epicID string) (*Swarm, error) {
	// BeadsOps Migration: cmd.Dir=m.beadsDir (REQUIRED - rig beads location), BEADS_DIR N/A
	b := beads.New(m.beadsDir)
	epic, err := b.Show(epicID)
	if err != nil {
		return nil, fmt.Errorf("bd show: %w", err)
	}

	// Verify it's a swarm molecule - check for mol_type in Type field or labels
	// Note: beads.Issue doesn't have a direct MolType field, so we check type
	// In practice, swarm epics are type=molecule with labels indicating swarm
	if epic.Type != "molecule" {
		return nil, fmt.Errorf("epic %s is not a swarm (type=%s)", epicID, epic.Type)
	}

	// Get current git commit as base
	baseCommit, _ := m.getGitHead()
	if baseCommit == "" {
		baseCommit = "unknown"
	}

	// Map status to swarm state
	state := SwarmActive
	if epic.Status == "closed" {
		state = SwarmLanded
	}

	swarm := &Swarm{
		ID:           epicID,
		RigName:      m.rig.Name,
		EpicID:       epicID,
		BaseCommit:   baseCommit,
		Integration:  fmt.Sprintf("swarm/%s", epicID),
		TargetBranch: m.rig.DefaultBranch(),
		State:        state,
		Workers:      []string{}, // Discovered from active tasks
		Tasks:        []SwarmTask{},
	}

	// Load tasks from beads (children of the epic)
	tasks, err := m.loadTasksFromBeads(epicID)
	if err == nil {
		swarm.Tasks = tasks
		// Discover workers from assigned tasks
		for _, task := range tasks {
			if task.Assignee != "" {
				swarm.Workers = appendUnique(swarm.Workers, task.Assignee)
			}
		}
	}

	return swarm, nil
}

// appendUnique appends s to slice if not already present.
func appendUnique(slice []string, s string) []string {
	for _, v := range slice {
		if v == s {
			return slice
		}
	}
	return append(slice, s)
}

// GetSwarm loads a swarm from beads. Alias for LoadSwarm for compatibility.
func (m *Manager) GetSwarm(id string) (*Swarm, error) {
	return m.LoadSwarm(id)
}

// GetReadyTasks returns tasks ready to be assigned by querying beads.
func (m *Manager) GetReadyTasks(swarmID string) ([]SwarmTask, error) {
	// BeadsOps Migration: cmd.Dir=m.beadsDir (REQUIRED - rig beads location), BEADS_DIR N/A
	b := beads.New(m.beadsDir)
	status, err := b.SwarmStatus(swarmID)
	if err != nil {
		return nil, ErrSwarmNotFound
	}

	if len(status.ReadyTasks) == 0 {
		return nil, ErrNoReadyTasks
	}

	tasks := make([]SwarmTask, len(status.ReadyTasks))
	for i, r := range status.ReadyTasks {
		tasks[i] = SwarmTask{
			IssueID: r.ID,
			Title:   r.Title,
			State:   TaskPending,
		}
	}
	return tasks, nil
}

// IsComplete checks if all tasks are closed by querying beads.
func (m *Manager) IsComplete(swarmID string) (bool, error) {
	// BeadsOps Migration: cmd.Dir=m.beadsDir (REQUIRED - rig beads location), BEADS_DIR N/A
	b := beads.New(m.beadsDir)
	status, err := b.SwarmStatus(swarmID)
	if err != nil {
		return false, ErrSwarmNotFound
	}

	// Complete if nothing is in progress or blocked, and all tasks are completed.
	// This implies Ready == 0 as well since TotalTasks = Ready + InProgress + Blocked + Completed
	return status.InProgress == 0 && status.Blocked == 0 && status.Completed == status.TotalTasks, nil
}

// isValidTransition checks if a state transition is allowed.
func isValidTransition(from, to SwarmState) bool {
	transitions := map[SwarmState][]SwarmState{
		SwarmCreated:  {SwarmActive, SwarmCanceled},
		SwarmActive:   {SwarmMerging, SwarmFailed, SwarmCanceled},
		SwarmMerging:  {SwarmLanded, SwarmFailed, SwarmCanceled},
		SwarmLanded:   {}, // Terminal
		SwarmFailed:   {}, // Terminal
		SwarmCanceled: {}, // Terminal
	}

	allowed, ok := transitions[from]
	if !ok {
		return false
	}

	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// loadTasksFromBeads loads child issues from beads CLI.
func (m *Manager) loadTasksFromBeads(epicID string) ([]SwarmTask, error) {
	// BeadsOps Migration: cmd.Dir=m.beadsDir (REQUIRED - rig beads location), BEADS_DIR N/A
	b := beads.New(m.beadsDir)
	epic, err := b.Show(epicID)
	if err != nil {
		return nil, fmt.Errorf("bd show: %w", err)
	}

	// Extract dependents as tasks (issues that depend on/are blocked by this epic)
	// Accept both "parent-child" and "blocks" relationships
	var tasks []SwarmTask
	for _, dep := range epic.Dependents {
		if dep.DependencyType != "parent-child" && dep.DependencyType != "blocks" {
			continue
		}

		state := TaskPending
		switch dep.Status {
		case "in_progress", "hooked":
			state = TaskInProgress
		case "closed":
			state = TaskMerged
		}

		tasks = append(tasks, SwarmTask{
			IssueID:  dep.ID,
			Title:    dep.Title,
			State:    state,
			Assignee: dep.Assignee,
		})
	}

	return tasks, nil
}

// getGitHead returns the current HEAD commit.
func (m *Manager) getGitHead() (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = m.gitDir

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return "", err
	}

	return strings.TrimSpace(stdout.String()), nil
}
