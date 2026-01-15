package agent

import (
	"sync"

	"github.com/steveyegge/gastown/internal/session"
)

// Double is an in-memory test double for the Agents interface.
// It provides a pure drop-in replacement for testing manager logic.
// For error injection, use agentsStub in tests.
type Double struct {
	mu     sync.RWMutex
	agents map[AgentID]*doubleAgent
}

type doubleAgent struct {
	name     string
	workDir  string
	command  string
	nudgeLog []string
}

// NewDouble creates a new in-memory Agents test double.
func NewDouble() *Double {
	return &Double{
		agents: make(map[AgentID]*doubleAgent),
	}
}

// Ensure Double implements Agents
var _ Agents = (*Double)(nil)

// Start creates a new agent.
func (d *Double) Start(name, workDir, command string) (AgentID, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	id := AgentID(name)
	if _, exists := d.agents[id]; exists {
		return "", ErrAlreadyRunning
	}

	d.agents[id] = &doubleAgent{
		name:    name,
		workDir: workDir,
		command: command,
	}

	return id, nil
}

// Stop removes an agent.
func (d *Double) Stop(id AgentID, graceful bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	delete(d.agents, id)
	return nil
}

// Exists checks if an agent exists.
func (d *Double) Exists(id AgentID) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	_, exists := d.agents[id]
	return exists
}

// WaitReady returns nil if the agent exists, ErrNotRunning otherwise.
func (d *Double) WaitReady(id AgentID) error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if _, exists := d.agents[id]; !exists {
		return ErrNotRunning
	}
	return nil
}

// SessionID returns the session ID for an agent.
func (d *Double) SessionID(id AgentID) session.SessionID {
	return session.SessionID(id)
}

// GetInfo returns information about an agent's session.
func (d *Double) GetInfo(id AgentID) (*session.Info, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	agent, exists := d.agents[id]
	if !exists {
		return nil, ErrNotRunning
	}

	return &session.Info{
		Name:    agent.name,
		Created: "2024-01-01T00:00:00Z",
		Windows: 1,
	}, nil
}

// Nudge sends a message to a running agent.
func (d *Double) Nudge(id AgentID, message string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	agent, exists := d.agents[id]
	if !exists {
		return ErrNotRunning
	}

	agent.nudgeLog = append(agent.nudgeLog, message)
	return nil
}

// --- Test helpers (for verification, not error injection) ---

// Clear removes all agents (for test cleanup).
func (d *Double) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.agents = make(map[AgentID]*doubleAgent)
}

// AgentCount returns the number of agents (for test verification).
func (d *Double) AgentCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.agents)
}

// CreateAgent adds an agent without going through Start (for test setup).
func (d *Double) CreateAgent(id AgentID) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.agents[id] = &doubleAgent{name: string(id)}
}

// GetWorkDir returns the working directory passed to Start for an agent.
// Returns empty string if agent doesn't exist.
func (d *Double) GetWorkDir(id AgentID) string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if agent, exists := d.agents[id]; exists {
		return agent.workDir
	}
	return ""
}

// GetCommand returns the command passed to Start for an agent.
// Returns empty string if agent doesn't exist.
func (d *Double) GetCommand(id AgentID) string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if agent, exists := d.agents[id]; exists {
		return agent.command
	}
	return ""
}

// NudgeLog returns the nudge messages sent to an agent (for test verification).
func (d *Double) NudgeLog(id AgentID) []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if agent, exists := d.agents[id]; exists {
		// Return a copy to prevent mutation
		result := make([]string, len(agent.nudgeLog))
		copy(result, agent.nudgeLog)
		return result
	}
	return nil
}
