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

// testAgentID creates a test AgentID for double tests.
func testAgentID(name string) agent.AgentID {
	return agent.PolecatAddress("testrig", name)
}

func TestDouble_Clear_RemovesAllAgents(t *testing.T) {
	d := agent.NewDouble()

	// Add some agents
	id1 := testAgentID("agent1")
	id2 := testAgentID("agent2")
	_ = d.Start(id1, "/tmp", "cmd1")
	_ = d.Start(id2, "/tmp", "cmd2")

	d.Clear()

	assert.Equal(t, 0, d.AgentCount())
}

func TestDouble_AgentCount_ReturnsCorrectCount(t *testing.T) {
	d := agent.NewDouble()

	assert.Equal(t, 0, d.AgentCount())

	id1 := testAgentID("agent1")
	_ = d.Start(id1, "/tmp", "cmd1")
	assert.Equal(t, 1, d.AgentCount())

	id2 := testAgentID("agent2")
	_ = d.Start(id2, "/tmp", "cmd2")
	assert.Equal(t, 2, d.AgentCount())

	_ = d.Stop(id1, false)
	assert.Equal(t, 1, d.AgentCount())
}

func TestDouble_CreateAgent_AddsAgentWithoutStart(t *testing.T) {
	d := agent.NewDouble()

	id := testAgentID("pre-created")
	d.CreateAgent(id)

	assert.True(t, d.Exists(id))
	assert.Equal(t, 1, d.AgentCount())
}

func TestDouble_GetWorkDir_ReturnsWorkDir(t *testing.T) {
	d := agent.NewDouble()

	id := testAgentID("test")
	_ = d.Start(id, "/custom/workdir", "cmd")

	assert.Equal(t, "/custom/workdir", d.GetWorkDir(id))
}

func TestDouble_GetWorkDir_ReturnsEmptyForNonexistent(t *testing.T) {
	d := agent.NewDouble()

	id := testAgentID("nonexistent")
	assert.Equal(t, "", d.GetWorkDir(id))
}

func TestDouble_GetCommand_ReturnsCommand(t *testing.T) {
	d := agent.NewDouble()

	id := testAgentID("test")
	_ = d.Start(id, "/tmp", "echo hello world")

	assert.Equal(t, "echo hello world", d.GetCommand(id))
}

func TestDouble_GetCommand_ReturnsEmptyForNonexistent(t *testing.T) {
	d := agent.NewDouble()

	id := testAgentID("nonexistent")
	assert.Equal(t, "", d.GetCommand(id))
}

func TestDouble_Start_ReturnsAlreadyRunning(t *testing.T) {
	d := agent.NewDouble()

	id := testAgentID("test")
	_ = d.Start(id, "/tmp", "cmd")
	err := d.Start(id, "/tmp", "cmd")

	assert.ErrorIs(t, err, agent.ErrAlreadyRunning)
}

func TestDouble_WaitReady_ReturnsNotRunning_WhenAgentDoesNotExist(t *testing.T) {
	d := agent.NewDouble()

	id := testAgentID("nonexistent")
	err := d.WaitReady(id)

	assert.ErrorIs(t, err, agent.ErrNotRunning)
}

func TestDouble_GetInfo_ReturnsNotRunning_WhenAgentDoesNotExist(t *testing.T) {
	d := agent.NewDouble()

	id := testAgentID("nonexistent")
	_, err := d.GetInfo(id)

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

	id := testAgentID("test")
	err := stub.Start(id, "/tmp", "cmd")
	assert.ErrorIs(t, err, expectedErr)
}

func TestAgentsStub_StopErr_CausesStopToFail(t *testing.T) {
	d := agent.NewDouble()
	stub := newAgentsStub(d)
	expectedErr := errors.New("stop failed")

	id := testAgentID("test")
	_ = stub.Start(id, "/tmp", "cmd")
	stub.StopErr = expectedErr

	err := stub.Stop(id, false)
	assert.ErrorIs(t, err, expectedErr)
}

func TestAgentsStub_WaitReadyErr_CausesWaitReadyToFail(t *testing.T) {
	d := agent.NewDouble()
	stub := newAgentsStub(d)
	expectedErr := errors.New("waitready failed")

	id := testAgentID("test")
	_ = stub.Start(id, "/tmp", "cmd")
	stub.WaitReadyErr = expectedErr

	err := stub.WaitReady(id)
	assert.ErrorIs(t, err, expectedErr)
}
