package agent_test

import (
	"errors"
	"testing"

	"github.com/steveyegge/gastown/internal/agent"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Double Test Helper Tests
//
// These tests verify the test helper methods on agent.Double work correctly.
// =============================================================================

func TestDouble_Clear_RemovesAllAgents(t *testing.T) {
	d := agent.NewDouble()

	// Add some agents
	_, _ = d.Start("agent1", "/tmp", "cmd1")
	_, _ = d.Start("agent2", "/tmp", "cmd2")

	d.Clear()

	assert.Equal(t, 0, d.AgentCount())
}

func TestDouble_AgentCount_ReturnsCorrectCount(t *testing.T) {
	d := agent.NewDouble()

	assert.Equal(t, 0, d.AgentCount())

	_, _ = d.Start("agent1", "/tmp", "cmd1")
	assert.Equal(t, 1, d.AgentCount())

	_, _ = d.Start("agent2", "/tmp", "cmd2")
	assert.Equal(t, 2, d.AgentCount())

	_ = d.Stop("agent1", false)
	assert.Equal(t, 1, d.AgentCount())
}

func TestDouble_CreateAgent_AddsAgentWithoutStart(t *testing.T) {
	d := agent.NewDouble()

	d.CreateAgent("pre-created")

	assert.True(t, d.Exists("pre-created"))
	assert.Equal(t, 1, d.AgentCount())
}

func TestDouble_GetWorkDir_ReturnsWorkDir(t *testing.T) {
	d := agent.NewDouble()

	_, _ = d.Start("test", "/custom/workdir", "cmd")

	assert.Equal(t, "/custom/workdir", d.GetWorkDir("test"))
}

func TestDouble_GetWorkDir_ReturnsEmptyForNonexistent(t *testing.T) {
	d := agent.NewDouble()

	assert.Equal(t, "", d.GetWorkDir("nonexistent"))
}

func TestDouble_GetCommand_ReturnsCommand(t *testing.T) {
	d := agent.NewDouble()

	_, _ = d.Start("test", "/tmp", "echo hello world")

	assert.Equal(t, "echo hello world", d.GetCommand("test"))
}

func TestDouble_GetCommand_ReturnsEmptyForNonexistent(t *testing.T) {
	d := agent.NewDouble()

	assert.Equal(t, "", d.GetCommand("nonexistent"))
}

func TestDouble_Start_ReturnsAlreadyRunning(t *testing.T) {
	d := agent.NewDouble()

	_, _ = d.Start("test", "/tmp", "cmd")
	_, err := d.Start("test", "/tmp", "cmd")

	assert.ErrorIs(t, err, agent.ErrAlreadyRunning)
}

func TestDouble_WaitReady_ReturnsNotRunning_WhenAgentDoesNotExist(t *testing.T) {
	d := agent.NewDouble()

	err := d.WaitReady("nonexistent")

	assert.ErrorIs(t, err, agent.ErrNotRunning)
}

func TestDouble_GetInfo_ReturnsNotRunning_WhenAgentDoesNotExist(t *testing.T) {
	d := agent.NewDouble()

	_, err := d.GetInfo("nonexistent")

	assert.ErrorIs(t, err, agent.ErrNotRunning)
}

// =============================================================================
// agentsStub Tests - Error Injection
// =============================================================================

func TestAgentsStub_StartErr_CausesStartToFail(t *testing.T) {
	d := agent.NewDouble()
	stub := newAgentsStub(d)
	expectedErr := errors.New("start failed")

	stub.StartErr = expectedErr

	_, err := stub.Start("test", "/tmp", "cmd")
	assert.ErrorIs(t, err, expectedErr)
}

func TestAgentsStub_StopErr_CausesStopToFail(t *testing.T) {
	d := agent.NewDouble()
	stub := newAgentsStub(d)
	expectedErr := errors.New("stop failed")

	id, _ := stub.Start("test", "/tmp", "cmd")
	stub.StopErr = expectedErr

	err := stub.Stop(id, false)
	assert.ErrorIs(t, err, expectedErr)
}

func TestAgentsStub_WaitReadyErr_CausesWaitReadyToFail(t *testing.T) {
	d := agent.NewDouble()
	stub := newAgentsStub(d)
	expectedErr := errors.New("waitready failed")

	id, _ := stub.Start("test", "/tmp", "cmd")
	stub.WaitReadyErr = expectedErr

	err := stub.WaitReady(id)
	assert.ErrorIs(t, err, expectedErr)
}
