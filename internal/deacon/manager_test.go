package deacon_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/steveyegge/gastown/internal/agent"
	"github.com/steveyegge/gastown/internal/deacon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Basic Lifecycle Tests ---

func TestManager_Start_CreatesAgent(t *testing.T) {
	townRoot := t.TempDir()
	setupDeaconDir(t, townRoot)

	agents := agent.NewDouble()
	mgr := deacon.NewManager(townRoot, "claude", agents)

	err := mgr.Start()
	require.NoError(t, err)

	// Verify agent was created with correct name
	agentID := agent.AgentID(deacon.SessionName())
	assert.True(t, agents.Exists(agentID), "agent should exist after Start")
}

func TestManager_Start_WhenAlreadyRunning_ReturnsError(t *testing.T) {
	townRoot := t.TempDir()
	setupDeaconDir(t, townRoot)

	agents := agent.NewDouble()

	// Pre-create agent
	agents.CreateAgent(agent.AgentID(deacon.SessionName()))

	mgr := deacon.NewManager(townRoot, "claude", agents)
	err := mgr.Start()

	assert.ErrorIs(t, err, deacon.ErrAlreadyRunning)
}

// --- Stop Tests ---

func TestManager_Stop_TerminatesAgent(t *testing.T) {
	townRoot := t.TempDir()
	setupDeaconDir(t, townRoot)

	agents := agent.NewDouble()
	mgr := deacon.NewManager(townRoot, "claude", agents)

	_ = mgr.Start()
	err := mgr.Stop()
	require.NoError(t, err)

	assert.False(t, agents.Exists(agent.AgentID(deacon.SessionName())))
}

func TestManager_Stop_WhenNotRunning_ReturnsError(t *testing.T) {
	townRoot := t.TempDir()
	setupDeaconDir(t, townRoot)

	agents := agent.NewDouble()
	mgr := deacon.NewManager(townRoot, "claude", agents)

	err := mgr.Stop()
	assert.ErrorIs(t, err, deacon.ErrNotRunning)
}

// --- IsRunning Tests ---

func TestManager_IsRunning_ReturnsCorrectState(t *testing.T) {
	townRoot := t.TempDir()
	setupDeaconDir(t, townRoot)

	agents := agent.NewDouble()
	mgr := deacon.NewManager(townRoot, "claude", agents)

	running, _ := mgr.IsRunning()
	assert.False(t, running)

	_ = mgr.Start()
	running, _ = mgr.IsRunning()
	assert.True(t, running)

	_ = mgr.Stop()
	running, _ = mgr.IsRunning()
	assert.False(t, running)
}

// --- Status Tests ---

func TestManager_Status_ReturnsInfo_WhenRunning(t *testing.T) {
	townRoot := t.TempDir()
	setupDeaconDir(t, townRoot)

	agents := agent.NewDouble()
	mgr := deacon.NewManager(townRoot, "claude", agents)

	_ = mgr.Start()

	info, err := mgr.Status()
	require.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, deacon.SessionName(), info.Name)
}

func TestManager_Status_ReturnsError_WhenNotRunning(t *testing.T) {
	townRoot := t.TempDir()
	setupDeaconDir(t, townRoot)

	agents := agent.NewDouble()
	mgr := deacon.NewManager(townRoot, "claude", agents)

	_, err := mgr.Status()
	assert.ErrorIs(t, err, deacon.ErrNotRunning)
}

func TestManager_Status_ReturnsError_WhenGetInfoFails(t *testing.T) {
	townRoot := t.TempDir()
	setupDeaconDir(t, townRoot)

	double := agent.NewDouble()
	stub := agent.NewAgentsStub(double)
	mgr := deacon.NewManager(townRoot, "claude", stub)

	_ = mgr.Start()

	// Inject GetInfo failure
	stub.GetInfoErr = errors.New("simulated GetInfo failure")
	_, err := mgr.Status()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "simulated GetInfo failure")
}

// --- Start Parameter Tests ---

func TestManager_Start_UsesCorrectWorkDir(t *testing.T) {
	townRoot := t.TempDir()
	setupDeaconDir(t, townRoot)

	agents := agent.NewDouble()
	mgr := deacon.NewManager(townRoot, "claude", agents)

	err := mgr.Start()
	require.NoError(t, err)

	agentID := agent.AgentID(deacon.SessionName())
	workDir := agents.GetWorkDir(agentID)

	expectedWorkDir := filepath.Join(townRoot, "deacon")
	assert.Equal(t, expectedWorkDir, workDir, "should start in deacon directory")
}

func TestManager_Start_UsesCorrectCommand(t *testing.T) {
	townRoot := t.TempDir()
	setupDeaconDir(t, townRoot)

	agents := agent.NewDouble()
	mgr := deacon.NewManager(townRoot, "claude", agents)

	err := mgr.Start()
	require.NoError(t, err)

	agentID := agent.AgentID(deacon.SessionName())
	command := agents.GetCommand(agentID)

	assert.Contains(t, command, "claude", "command should include agent name")
}

// --- Settings Tests ---

func TestManager_Start_CreatesSettings(t *testing.T) {
	// S4: Start should create settings if they don't exist
	townRoot := t.TempDir()
	deaconDir := filepath.Join(townRoot, "deacon")
	require.NoError(t, os.MkdirAll(deaconDir, 0755))
	// Note: NOT creating .claude/settings.local.json

	agents := agent.NewDouble()
	mgr := deacon.NewManager(townRoot, "claude", agents)

	err := mgr.Start()
	require.NoError(t, err)

	// Verify settings were created
	settingsPath := filepath.Join(deaconDir, ".claude", "settings.local.json")
	_, err = os.Stat(settingsPath)
	assert.NoError(t, err, "settings file should be created by Start")
}

func TestManager_Start_SettingsFailure_NoAgentCreated(t *testing.T) {
	// S5: If settings can't be created, Start should fail and no agent should exist
	townRoot := t.TempDir()
	deaconDir := filepath.Join(townRoot, "deacon")
	require.NoError(t, os.MkdirAll(deaconDir, 0755))

	// Make deacon dir read-only so settings can't be created
	require.NoError(t, os.Chmod(deaconDir, 0555))
	t.Cleanup(func() {
		os.Chmod(deaconDir, 0755) // Restore for cleanup
	})

	agents := agent.NewDouble()
	mgr := deacon.NewManager(townRoot, "claude", agents)

	err := mgr.Start()
	assert.Error(t, err, "should fail when settings can't be created")

	// Agent should NOT have been created
	assert.Equal(t, 0, agents.AgentCount(), "no agent should be created on settings failure")
}

// --- Failure Injection Tests ---

func TestManager_Start_AgentStartFailure(t *testing.T) {
	townRoot := t.TempDir()
	setupDeaconDir(t, townRoot)

	double := agent.NewDouble()
	stub := agent.NewAgentsStub(double)
	stub.StartErr = errors.New("simulated start failure")

	mgr := deacon.NewManager(townRoot, "claude", stub)
	err := mgr.Start()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "simulated start failure")
}

func TestManager_Stop_AgentStopFailure(t *testing.T) {
	townRoot := t.TempDir()
	setupDeaconDir(t, townRoot)

	double := agent.NewDouble()
	stub := agent.NewAgentsStub(double)
	mgr := deacon.NewManager(townRoot, "claude", stub)

	_ = mgr.Start()

	// Inject stop failure
	stub.StopErr = errors.New("simulated stop failure")
	err := mgr.Stop()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "simulated stop failure")
}

func TestManager_Start_WaitReadyFailure(t *testing.T) {
	// Currently WaitReady errors are ignored in deacon.Start()
	// This test documents that behavior
	townRoot := t.TempDir()
	setupDeaconDir(t, townRoot)

	double := agent.NewDouble()
	stub := agent.NewAgentsStub(double)
	stub.WaitReadyErr = errors.New("wait ready timeout")

	mgr := deacon.NewManager(townRoot, "claude", stub)
	err := mgr.Start()

	// Currently Start() ignores WaitReady errors - this may be intentional
	// The agent is created even if WaitReady fails
	assert.NoError(t, err, "Start currently ignores WaitReady failures")
	assert.True(t, double.Exists(agent.AgentID(deacon.SessionName())), "agent should exist despite WaitReady failure")
}

// --- Helpers ---

func setupDeaconDir(t *testing.T, townRoot string) {
	t.Helper()
	deaconDir := filepath.Join(townRoot, "deacon")
	require.NoError(t, os.MkdirAll(deaconDir, 0755))

	claudeDir := filepath.Join(deaconDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	settingsFile := filepath.Join(claudeDir, "settings.local.json")
	require.NoError(t, os.WriteFile(settingsFile, []byte(`{}`), 0644))
}
