package witness_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/steveyegge/gastown/internal/agent"
	"github.com/steveyegge/gastown/internal/rig"
	"github.com/steveyegge/gastown/internal/witness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Witness Manager Unit Tests
// Using agent.Double for testable abstraction
// =============================================================================

func setupTestRig(t *testing.T) (*rig.Rig, string) {
	t.Helper()
	tmpDir := t.TempDir()
	rigPath := filepath.Join(tmpDir, "testrig")

	// Create required directories
	require.NoError(t, os.MkdirAll(filepath.Join(rigPath, ".runtime"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(rigPath, "witness"), 0755))

	// Create minimal Claude settings
	claudeDir := filepath.Join(rigPath, "witness", ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.local.json"), []byte(`{}`), 0644))

	return &rig.Rig{
		Name:     "testrig",
		Path:     rigPath,
		Polecats: []string{"p1", "p2"},
	}, rigPath
}

// --- Start() Tests ---

func TestManager_Start_CreatesAgent(t *testing.T) {
	r, _ := setupTestRig(t)
	agents := agent.NewDouble()
	mgr := witness.NewManager(agents, r, "claude")

	err := mgr.Start()
	require.NoError(t, err)

	// Verify agent exists with correct name
	agentID := agent.WitnessAddress(r.Name)
	assert.True(t, agents.Exists(agentID), "agent should exist after Start")
}

func TestManager_Start_WhenAlreadyRunning_ReturnsError(t *testing.T) {
	r, _ := setupTestRig(t)
	agents := agent.NewDouble()
	mgr := witness.NewManager(agents, r, "claude")

	// Pre-create agent using agent.WitnessAddress(r.Name)
	agents.CreateAgent(agent.WitnessAddress(r.Name))

	err := mgr.Start()

	assert.ErrorIs(t, err, witness.ErrAlreadyRunning)
}

func TestManager_Start_UpdatesStateToRunning(t *testing.T) {
	r, rigPath := setupTestRig(t)
	agents := agent.NewDouble()
	mgr := witness.NewManager(agents, r, "claude")

	err := mgr.Start()
	require.NoError(t, err)

	// Verify state file
	stateFile := filepath.Join(rigPath, ".runtime", "witness.json")
	data, err := os.ReadFile(stateFile)
	require.NoError(t, err, "state file should exist")

	var state witness.Witness
	require.NoError(t, json.Unmarshal(data, &state))
	assert.Equal(t, witness.StateRunning, state.State)
	assert.NotNil(t, state.StartedAt)
}

// --- Stop() Tests ---

func TestManager_Stop_TerminatesAgent(t *testing.T) {
	r, _ := setupTestRig(t)
	agents := agent.NewDouble()
	mgr := witness.NewManager(agents, r, "claude")

	_ = mgr.Start()
	err := mgr.Stop()
	require.NoError(t, err)

	agentID := agent.WitnessAddress(r.Name)
	assert.False(t, agents.Exists(agentID), "agent should be gone after Stop")
}

func TestManager_Stop_WhenNotRunning_ReturnsError(t *testing.T) {
	r, _ := setupTestRig(t)
	agents := agent.NewDouble()
	mgr := witness.NewManager(agents, r, "claude")

	err := mgr.Stop()
	assert.ErrorIs(t, err, witness.ErrNotRunning)
}

func TestManager_Stop_UpdatesStateToStopped(t *testing.T) {
	r, rigPath := setupTestRig(t)
	agents := agent.NewDouble()
	mgr := witness.NewManager(agents, r, "claude")

	_ = mgr.Start()
	err := mgr.Stop()
	require.NoError(t, err)

	// Verify state file
	stateFile := filepath.Join(rigPath, ".runtime", "witness.json")
	data, err := os.ReadFile(stateFile)
	require.NoError(t, err)

	var state witness.Witness
	require.NoError(t, json.Unmarshal(data, &state))
	assert.Equal(t, witness.StateStopped, state.State)
}

func TestManager_Stop_StateRunningButNoAgent_Succeeds(t *testing.T) {
	r, rigPath := setupTestRig(t)
	agents := agent.NewDouble()
	mgr := witness.NewManager(agents, r, "claude")

	// Write stale state (says running but no agent)
	stateFile := filepath.Join(rigPath, ".runtime", "witness.json")
	staleState := witness.Witness{
		RigName: "testrig",
		State:   witness.StateRunning,
	}
	data, _ := json.Marshal(staleState)
	require.NoError(t, os.WriteFile(stateFile, data, 0644))

	// Stop should succeed and update state
	err := mgr.Stop()
	require.NoError(t, err)

	// Verify state updated
	data, _ = os.ReadFile(stateFile)
	var state witness.Witness
	_ = json.Unmarshal(data, &state)
	assert.Equal(t, witness.StateStopped, state.State)
}

func TestManager_Stop_StateStoppedButAgentExists_Succeeds(t *testing.T) {
	r, rigPath := setupTestRig(t)
	agents := agent.NewDouble()
	mgr := witness.NewManager(agents, r, "claude")

	// Create agent manually (simulating stale agent)
	agentID := agent.WitnessAddress(r.Name)
	agents.CreateAgent(agentID)

	// Write state that says stopped
	stateFile := filepath.Join(rigPath, ".runtime", "witness.json")
	staleState := witness.Witness{
		RigName: "testrig",
		State:   witness.StateStopped,
	}
	data, _ := json.Marshal(staleState)
	require.NoError(t, os.WriteFile(stateFile, data, 0644))

	// Stop should succeed and kill the agent
	err := mgr.Stop()
	require.NoError(t, err)

	// Agent should be gone
	assert.False(t, agents.Exists(agentID))
}

// --- Status() Tests ---

func TestManager_Status_ReturnsState(t *testing.T) {
	r, _ := setupTestRig(t)
	agents := agent.NewDouble()
	mgr := witness.NewManager(agents, r, "claude")

	_ = mgr.Start()

	status, err := mgr.Status()
	require.NoError(t, err)
	assert.Equal(t, witness.StateRunning, status.State)
	assert.Equal(t, []string{"p1", "p2"}, status.MonitoredPolecats)
}

func TestManager_Status_WhenAgentCrashed_DetectsMismatch(t *testing.T) {
	// Scenario: Agent starts successfully, then crashes (killed externally).
	// Status() should detect that state=running but agent doesn't exist.
	r, _ := setupTestRig(t)
	agents := agent.NewDouble()
	mgr := witness.NewManager(agents, r, "claude")

	// Start the witness
	require.NoError(t, mgr.Start())

	// Simulate crash: kill agent directly without going through manager.Stop()
	agentID := agent.WitnessAddress(r.Name)
	_ = agents.Stop(agentID, false) // Direct kill, bypasses manager state update

	// Agent is dead
	assert.False(t, agents.Exists(agentID), "agent should be dead after crash")

	// Status() detects the mismatch and reports stopped
	status, err := mgr.Status()
	require.NoError(t, err)
	assert.Equal(t, witness.StateStopped, status.State, "should detect crashed agent")
}

// --- SessionName() Tests ---

func TestManager_SessionName_Format(t *testing.T) {
	r, _ := setupTestRig(t)
	agents := agent.NewDouble()
	mgr := witness.NewManager(agents, r, "claude")

	assert.Equal(t, "gt-testrig-witness", mgr.SessionName())
}

// --- Lifecycle Integration ---

func TestManager_FullLifecycle(t *testing.T) {
	r, _ := setupTestRig(t)
	agents := agent.NewDouble()
	mgr := witness.NewManager(agents, r, "claude")

	// Start
	require.NoError(t, mgr.Start())

	// Status shows running
	status, _ := mgr.Status()
	assert.Equal(t, witness.StateRunning, status.State)

	// Stop
	require.NoError(t, mgr.Stop())

	// Status shows stopped
	status, _ = mgr.Status()
	assert.Equal(t, witness.StateStopped, status.State)

	// Can start again
	require.NoError(t, mgr.Start())
}
