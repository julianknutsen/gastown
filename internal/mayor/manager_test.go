package mayor_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/steveyegge/gastown/internal/agent"
	"github.com/steveyegge/gastown/internal/mayor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// TESTS NOW POSSIBLE WITH INJECTABLE AGENTS
//
// Before: Managers depended on real tmux, couldn't test without subprocess overhead
// After: Full unit testing of manager logic with controllable test doubles
// =============================================================================

// --- Basic Lifecycle Tests ---

func TestManager_Start_CreatesAgent(t *testing.T) {
	// WHAT'S NEW: We can verify agent creation without tmux
	townRoot := t.TempDir()
	setupMayorDir(t, townRoot)

	agents := agent.NewDouble()
	mgr := mayor.NewManager(agents, townRoot, "claude")

	err := mgr.Start()
	require.NoError(t, err)

	// Verify agent was created with correct ID
	assert.True(t, agents.Exists(agent.MayorAddress()), "agent should exist after Start")
}

func TestManager_Start_WhenAlreadyRunning_ReturnsError(t *testing.T) {
	// WHAT'S NEW: We can test the "already running" path
	townRoot := t.TempDir()
	setupMayorDir(t, townRoot)

	agents := agent.NewDouble()
	mgr := mayor.NewManager(agents, townRoot, "claude")

	// Pre-create agent using agent.MayorAddress()
	agents.CreateAgent(agent.MayorAddress())

	err := mgr.Start()

	assert.ErrorIs(t, err, mayor.ErrAlreadyRunning)
}

// --- Stop Tests ---

func TestManager_Stop_TerminatesAgent(t *testing.T) {
	townRoot := t.TempDir()
	setupMayorDir(t, townRoot)

	agents := agent.NewDouble()
	mgr := mayor.NewManager(agents, townRoot, "claude")

	// Start then stop
	_ = mgr.Start()
	err := mgr.Stop()
	require.NoError(t, err)

	// Agent should be gone
	assert.False(t, agents.Exists(agent.MayorAddress()))
}

func TestManager_Stop_WhenNotRunning_ReturnsError(t *testing.T) {
	townRoot := t.TempDir()
	setupMayorDir(t, townRoot)

	agents := agent.NewDouble()
	mgr := mayor.NewManager(agents, townRoot, "claude")

	// Stop without starting
	err := mgr.Stop()
	assert.ErrorIs(t, err, mayor.ErrNotRunning)
}

// --- IsRunning Tests ---

func TestManager_IsRunning_ReturnsCorrectState(t *testing.T) {
	townRoot := t.TempDir()
	setupMayorDir(t, townRoot)

	agents := agent.NewDouble()
	mgr := mayor.NewManager(agents, townRoot, "claude")

	// Not running initially
	running, _ := mgr.IsRunning()
	assert.False(t, running)

	// Running after start
	_ = mgr.Start()
	running, _ = mgr.IsRunning()
	assert.True(t, running)

	// Not running after stop
	_ = mgr.Stop()
	running, _ = mgr.IsRunning()
	assert.False(t, running)
}

// --- Status Tests ---

func TestManager_Status_ReturnsRunning_WhenRunning(t *testing.T) {
	townRoot := t.TempDir()
	setupMayorDir(t, townRoot)

	agents := agent.NewDouble()
	mgr := mayor.NewManager(agents, townRoot, "claude")

	_ = mgr.Start()

	status, err := mgr.Status()
	require.NoError(t, err)
	assert.Equal(t, mayor.StateRunning, status.State)
	assert.Equal(t, mayor.SessionName(), status.Name)
}

func TestManager_Status_ReturnsStopped_WhenNotRunning(t *testing.T) {
	townRoot := t.TempDir()
	setupMayorDir(t, townRoot)

	agents := agent.NewDouble()
	mgr := mayor.NewManager(agents, townRoot, "claude")

	status, err := mgr.Status()
	require.NoError(t, err)
	assert.Equal(t, mayor.StateStopped, status.State)
}

// --- Start Parameter Tests ---

func TestManager_Start_UsesCorrectWorkDir(t *testing.T) {
	townRoot := t.TempDir()
	setupMayorDir(t, townRoot)

	agents := agent.NewDouble()
	mgr := mayor.NewManager(agents, townRoot, "claude")

	err := mgr.Start()
	require.NoError(t, err)

	workDir := agents.GetWorkDir(agent.MayorAddress())

	expectedWorkDir := filepath.Join(townRoot, "mayor")
	assert.Equal(t, expectedWorkDir, workDir, "should start in mayor directory")
}

func TestManager_Start_UsesCorrectCommand(t *testing.T) {
	townRoot := t.TempDir()
	setupMayorDir(t, townRoot)

	agents := agent.NewDouble()
	mgr := mayor.NewManager(agents, townRoot, "claude")

	err := mgr.Start()
	require.NoError(t, err)

	command := agents.GetCommand(agent.MayorAddress())

	assert.Contains(t, command, "claude", "command should include agent name")
}

// --- Settings Tests ---

func TestManager_Start_CreatesSettings(t *testing.T) {
	// S4: Start should create settings if they don't exist
	townRoot := t.TempDir()
	mayorDir := filepath.Join(townRoot, "mayor")
	require.NoError(t, os.MkdirAll(mayorDir, 0755))
	// Note: NOT creating .claude/settings.local.json

	agents := agent.NewDouble()
	mgr := mayor.NewManager(agents, townRoot, "claude")

	err := mgr.Start()
	require.NoError(t, err)

	// Verify settings were created
	settingsPath := filepath.Join(mayorDir, ".claude", "settings.local.json")
	_, err = os.Stat(settingsPath)
	assert.NoError(t, err, "settings file should be created by Start")
}

func TestManager_Start_SettingsFailure_NoAgentCreated(t *testing.T) {
	// S5: If settings can't be created, Start should fail and no agent should exist
	townRoot := t.TempDir()
	mayorDir := filepath.Join(townRoot, "mayor")
	require.NoError(t, os.MkdirAll(mayorDir, 0755))

	// Make mayor dir read-only so settings can't be created
	require.NoError(t, os.Chmod(mayorDir, 0555))
	t.Cleanup(func() {
		os.Chmod(mayorDir, 0755) // Restore for cleanup
	})

	agents := agent.NewDouble()
	mgr := mayor.NewManager(agents, townRoot, "claude")

	err := mgr.Start()
	assert.Error(t, err, "should fail when settings can't be created")

	// Agent should NOT have been created
	assert.Equal(t, 0, agents.AgentCount(), "no agent should be created on settings failure")
}

// --- Failure Injection Tests ---

func TestManager_Start_AgentStartFailure(t *testing.T) {
	townRoot := t.TempDir()
	setupMayorDir(t, townRoot)

	double := agent.NewDouble()
	stub := agent.NewAgentsStub(double)
	stub.StartErr = errors.New("simulated start failure")

	mgr := mayor.NewManager(stub, townRoot, "claude")
	err := mgr.Start()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "simulated start failure")
}

func TestManager_Stop_AgentStopFailure(t *testing.T) {
	townRoot := t.TempDir()
	setupMayorDir(t, townRoot)

	double := agent.NewDouble()
	stub := agent.NewAgentsStub(double)
	mgr := mayor.NewManager(stub, townRoot, "claude")

	_ = mgr.Start()

	// Inject stop failure
	stub.StopErr = errors.New("simulated stop failure")
	err := mgr.Stop()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "simulated stop failure")
}

func TestManager_Start_WaitReadyFailure(t *testing.T) {
	// Currently WaitReady errors are ignored in mayor.Start()
	// This test documents that behavior
	townRoot := t.TempDir()
	setupMayorDir(t, townRoot)

	double := agent.NewDouble()
	stub := agent.NewAgentsStub(double)
	stub.WaitReadyErr = errors.New("wait ready timeout")

	mgr := mayor.NewManager(stub, townRoot, "claude")
	err := mgr.Start()

	// Currently Start() ignores WaitReady errors - this may be intentional
	// The agent is created even if WaitReady fails
	assert.NoError(t, err, "Start currently ignores WaitReady failures")
	assert.True(t, double.Exists(agent.MayorAddress()), "agent should exist despite WaitReady failure")
}

// --- Helpers ---

func setupMayorDir(t *testing.T, townRoot string) {
	t.Helper()
	// Create required directory structure
	mayorDir := filepath.Join(townRoot, "mayor")
	require.NoError(t, os.MkdirAll(mayorDir, 0755))

	// Create minimal Claude settings to avoid EnsureSettingsForRole failing
	claudeDir := filepath.Join(mayorDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	settingsFile := filepath.Join(claudeDir, "settings.local.json")
	require.NoError(t, os.WriteFile(settingsFile, []byte(`{}`), 0644))
}

// =============================================================================
// SUMMARY: What we can test now with agent.Double and agent.AgentsStub
// =============================================================================
//
// 1. LIFECYCLE CORRECTNESS
//    - Start creates agent, Stop destroys it
//    - IsRunning reflects actual state
//    - No subprocess overhead, fast tests
//
// 2. ERROR PATHS
//    - Stop when not running → ErrNotRunning
//    - Start when already running → ErrAlreadyRunning
//    - All paths testable without side effects
//
// 3. FAILURE INJECTION (using agent.AgentsStub)
//    - stub.StartErr = err → test Start failures
//    - stub.StopErr = err → test Stop failures
//    - stub.WaitReadyErr = err → test readiness failures
//
// Note: Zombie detection is handled by the Agents layer (agent.Implementation),
// not the manager. See agent/agent_test.go for those tests.
// =============================================================================
