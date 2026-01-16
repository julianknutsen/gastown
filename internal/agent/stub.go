package agent

import (
	"github.com/steveyegge/gastown/internal/session"
)

// Note: session import used for GetInfo return type

// =============================================================================
// Test Stubs for Error Injection
//
// These stubs wrap pure doubles and allow injecting errors for testing
// error paths. This keeps the doubles as pure drop-in replacements.
//
// Exported so other packages (e.g., refinery) can use them for testing.
// =============================================================================

// AgentsStub wraps an Agents implementation and allows injecting errors.
// Use this for testing error paths that can't be triggered with Double.
type AgentsStub struct {
	Agents

	// Inject errors for specific operations
	StartErr     error
	StopErr      error
	WaitReadyErr error
	GetInfoErr   error
}

// NewAgentsStub creates a new stub wrapping the given Agents implementation.
func NewAgentsStub(wrapped Agents) *AgentsStub {
	return &AgentsStub{Agents: wrapped}
}

// StartWithConfig creates a new agent with config, or returns StartErr if set.
func (s *AgentsStub) StartWithConfig(id AgentID, cfg StartConfig) error {
	if s.StartErr != nil {
		return s.StartErr
	}
	return s.Agents.StartWithConfig(id, cfg)
}

// Stop terminates an agent, or returns StopErr if set.
func (s *AgentsStub) Stop(id AgentID, graceful bool) error {
	if s.StopErr != nil {
		return s.StopErr
	}
	return s.Agents.Stop(id, graceful)
}

// WaitReady blocks until ready, or returns WaitReadyErr if set.
func (s *AgentsStub) WaitReady(id AgentID) error {
	if s.WaitReadyErr != nil {
		return s.WaitReadyErr
	}
	return s.Agents.WaitReady(id)
}

// Delegate all other methods to the wrapped Agents

// Exists checks if an agent exists.
func (s *AgentsStub) Exists(id AgentID) bool {
	return s.Agents.Exists(id)
}

// GetInfo returns information about an agent's session, or returns GetInfoErr if set.
func (s *AgentsStub) GetInfo(id AgentID) (*session.Info, error) {
	if s.GetInfoErr != nil {
		return nil, s.GetInfoErr
	}
	return s.Agents.GetInfo(id)
}
