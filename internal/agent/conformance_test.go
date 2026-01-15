package agent_test

import (
	"testing"

	"github.com/steveyegge/gastown/internal/agent"
	"github.com/steveyegge/gastown/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Conformance Tests for Agents Interface
//
// These tests verify that both implementations behave identically:
// 1. agent.Double (test double)
// 2. agent.Implementation backed by session.Double
//
// Run these tests to ensure the test double is a faithful stand-in for the
// real implementation.
// =============================================================================

// agentsFactory creates an Agents implementation for testing.
type agentsFactory func() agent.Agents

// testCases returns the factories for both implementations.
func testCases() map[string]agentsFactory {
	return map[string]agentsFactory{
		"Double": func() agent.Agents {
			return agent.NewDouble()
		},
		"Implementation": func() agent.Agents {
			return agent.New(session.NewDouble(), nil)
		},
	}
}

// --- Start/Exists Conformance ---

func TestConformance_Start_CreatesAgent(t *testing.T) {
	for name, factory := range testCases() {
		t.Run(name, func(t *testing.T) {
			agents := factory()

			id, err := agents.Start("test-agent", "/tmp", "echo hello")
			require.NoError(t, err)
			assert.NotEmpty(t, id)
			assert.True(t, agents.Exists(id))
		})
	}
}

func TestConformance_Start_AlreadyRunning_ReturnsError(t *testing.T) {
	for name, factory := range testCases() {
		t.Run(name, func(t *testing.T) {
			agents := factory()

			// Start first agent
			id, err := agents.Start("test-agent", "/tmp", "echo hello")
			require.NoError(t, err)

			// Try to start again with same name
			_, err = agents.Start("test-agent", "/tmp", "echo hello")
			assert.ErrorIs(t, err, agent.ErrAlreadyRunning)

			// Original agent should still exist
			assert.True(t, agents.Exists(id))
		})
	}
}

func TestConformance_Exists_ReturnsFalse_WhenNoAgent(t *testing.T) {
	for name, factory := range testCases() {
		t.Run(name, func(t *testing.T) {
			agents := factory()

			assert.False(t, agents.Exists("nonexistent"))
		})
	}
}

// --- Stop Conformance ---

func TestConformance_Stop_TerminatesAgent(t *testing.T) {
	for name, factory := range testCases() {
		t.Run(name, func(t *testing.T) {
			agents := factory()

			id, _ := agents.Start("test-agent", "/tmp", "echo hello")
			err := agents.Stop(id, false)
			require.NoError(t, err)

			assert.False(t, agents.Exists(id))
		})
	}
}

func TestConformance_Stop_Idempotent(t *testing.T) {
	for name, factory := range testCases() {
		t.Run(name, func(t *testing.T) {
			agents := factory()

			// Stop non-existent agent should not error
			err := agents.Stop("nonexistent", false)
			assert.NoError(t, err)
		})
	}
}

func TestConformance_Stop_Graceful(t *testing.T) {
	for name, factory := range testCases() {
		t.Run(name, func(t *testing.T) {
			agents := factory()

			id, _ := agents.Start("test-agent", "/tmp", "echo hello")
			err := agents.Stop(id, true) // graceful=true
			require.NoError(t, err)

			assert.False(t, agents.Exists(id))
		})
	}
}

// --- SessionID Conformance ---

func TestConformance_SessionID_ReturnsCorrectID(t *testing.T) {
	for name, factory := range testCases() {
		t.Run(name, func(t *testing.T) {
			agents := factory()

			id, _ := agents.Start("test-agent", "/tmp", "echo hello")
			sessionID := agents.SessionID(id)

			assert.Equal(t, session.SessionID("test-agent"), sessionID)
		})
	}
}

// --- WaitReady Conformance ---

func TestConformance_WaitReady_NotRunning_ReturnsError(t *testing.T) {
	for name, factory := range testCases() {
		t.Run(name, func(t *testing.T) {
			agents := factory()

			err := agents.WaitReady("nonexistent")
			assert.ErrorIs(t, err, agent.ErrNotRunning)
		})
	}
}

func TestConformance_WaitReady_WhenRunning_ReturnsNil(t *testing.T) {
	for name, factory := range testCases() {
		t.Run(name, func(t *testing.T) {
			agents := factory()

			id, _ := agents.Start("test-agent", "/tmp", "echo hello")
			err := agents.WaitReady(id)
			assert.NoError(t, err)
		})
	}
}

// --- GetInfo Conformance ---

func TestConformance_GetInfo_ReturnsInfo(t *testing.T) {
	for name, factory := range testCases() {
		t.Run(name, func(t *testing.T) {
			agents := factory()

			id, _ := agents.Start("test-agent", "/tmp", "echo hello")
			info, err := agents.GetInfo(id)

			require.NoError(t, err)
			assert.NotNil(t, info)
			assert.Equal(t, "test-agent", info.Name)
		})
	}
}

func TestConformance_GetInfo_NotRunning_ReturnsError(t *testing.T) {
	for name, factory := range testCases() {
		t.Run(name, func(t *testing.T) {
			agents := factory()

			_, err := agents.GetInfo("nonexistent")
			assert.Error(t, err)
		})
	}
}

// --- Full Lifecycle Conformance ---

func TestConformance_FullLifecycle(t *testing.T) {
	for name, factory := range testCases() {
		t.Run(name, func(t *testing.T) {
			agents := factory()

			// Start
			id, err := agents.Start("lifecycle-test", "/tmp", "echo hello")
			require.NoError(t, err)
			assert.True(t, agents.Exists(id))

			// Get info
			info, err := agents.GetInfo(id)
			require.NoError(t, err)
			assert.Equal(t, "lifecycle-test", info.Name)

			// Stop
			err = agents.Stop(id, true)
			require.NoError(t, err)
			assert.False(t, agents.Exists(id))

			// Start again (should work after stop)
			id, err = agents.Start("lifecycle-test", "/tmp", "echo hello again")
			require.NoError(t, err)
			assert.True(t, agents.Exists(id))
		})
	}
}

// --- Multiple Agents Conformance ---

func TestConformance_MultipleAgents(t *testing.T) {
	for name, factory := range testCases() {
		t.Run(name, func(t *testing.T) {
			agents := factory()

			// Start multiple agents
			id1, err := agents.Start("agent-1", "/tmp", "echo 1")
			require.NoError(t, err)
			id2, err := agents.Start("agent-2", "/tmp", "echo 2")
			require.NoError(t, err)
			id3, err := agents.Start("agent-3", "/tmp", "echo 3")
			require.NoError(t, err)

			// All should exist
			assert.True(t, agents.Exists(id1))
			assert.True(t, agents.Exists(id2))
			assert.True(t, agents.Exists(id3))

			// Stop one
			_ = agents.Stop(id2, false)

			// Only stopped one should be gone
			assert.True(t, agents.Exists(id1))
			assert.False(t, agents.Exists(id2))
			assert.True(t, agents.Exists(id3))
		})
	}
}
