package agent_test

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/steveyegge/gastown/internal/agent"
	"github.com/steveyegge/gastown/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Implementation-Specific Unit Tests
//
// These tests cover behaviors unique to agent.Implementation that cannot be
// tested through the Agents interface alone (zombie detection, callbacks, etc.)
//
// For interface-level tests that run against both Double and Implementation,
// see conformance_test.go.
// =============================================================================

// startCfg is a helper to create a StartConfig for tests.
func startCfg(workDir, command string) agent.StartConfig {
	return agent.StartConfig{WorkDir: workDir, Command: command}
}

// --- Zombie Detection Tests (Implementation-specific) ---

func TestImplementation_Start_DetectsAndCleansUpZombie(t *testing.T) {
	procs := session.NewDouble()
	agents := agent.New(procs, nil)

	// Create zombie: session exists but process dead
	sessionID := session.SessionID("test-agent")
	_, _ = procs.Start("test-agent", "/tmp", "dead-command")
	_ = procs.SetRunning(sessionID, false) // Process is dead

	// Start should clean up zombie and create new session
	id := agent.PolecatAddress("testrig", "test-agent")
	err := agents.StartWithConfig(id, startCfg("/tmp", "new-command"))
	require.NoError(t, err)
	assert.True(t, agents.Exists(id))
}

func TestImplementation_Start_AlreadyRunning_WithLiveProcess(t *testing.T) {
	procs := session.NewDouble()
	agents := agent.New(procs, nil)

	// Create the AgentID first so we use the correct session name
	id := agent.PolecatAddress("testrig", "test-agent")
	sessionID := session.SessionID(id)

	// Pre-create session with live process using the correct logical session ID
	_, _ = procs.Start(string(sessionID), "/tmp", "running-command")
	_ = procs.SetRunning(sessionID, true)

	err := agents.StartWithConfig(id, startCfg("/tmp", "echo hello"))
	assert.ErrorIs(t, err, agent.ErrAlreadyRunning)
}

// --- Callback Tests (Implementation-specific) ---

func TestImplementation_Start_CallbackError_CleansUpSession(t *testing.T) {
	procs := session.NewDouble()

	callbackErr := errors.New("callback failed")
	cfg := &agent.Config{
		OnSessionCreated: func(sess session.Sessions, id session.SessionID) error {
			return callbackErr
		},
	}
	agents := agent.New(procs, cfg)

	id := agent.PolecatAddress("testrig", "test-agent")
	err := agents.StartWithConfig(id, startCfg("/tmp", "echo hello"))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session setup")

	// Session should be cleaned up
	exists, _ := procs.Exists(session.SessionID(id))
	assert.False(t, exists, "session should be cleaned up on callback failure")
}

func TestImplementation_Start_CallbackSuccess_SessionRemains(t *testing.T) {
	procs := session.NewDouble()

	callbackCalled := false
	cfg := &agent.Config{
		OnSessionCreated: func(sess session.Sessions, id session.SessionID) error {
			callbackCalled = true
			return nil
		},
	}
	agents := agent.New(procs, cfg)

	id := agent.PolecatAddress("testrig", "test-agent")
	err := agents.StartWithConfig(id, startCfg("/tmp", "echo hello"))
	require.NoError(t, err)

	assert.True(t, callbackCalled, "callback should be called")
	assert.True(t, agents.Exists(id), "session should exist after successful callback")
}

// --- EnvVars Tests (Implementation-specific) ---

func TestImplementation_Start_PrependsEnvVars(t *testing.T) {
	procs := session.NewDouble()
	cfg := &agent.Config{
		EnvVars: map[string]string{
			"FOO": "bar",
			"BAZ": "qux",
		},
	}
	agents := agent.New(procs, cfg)

	id := agent.PolecatAddress("testrig", "test-agent")
	err := agents.StartWithConfig(id, startCfg("/tmp", "echo hello"))
	require.NoError(t, err)

	cmd := procs.GetCommand(session.SessionID(id))
	assert.Contains(t, cmd, "FOO=bar")
	assert.Contains(t, cmd, "BAZ=qux")
	assert.Contains(t, cmd, "echo hello")
}

func TestImplementation_Start_EnvVarsSorted(t *testing.T) {
	procs := session.NewDouble()
	cfg := &agent.Config{
		EnvVars: map[string]string{
			"ZZZ": "last",
			"AAA": "first",
			"MMM": "middle",
		},
	}
	agents := agent.New(procs, cfg)

	id := agent.PolecatAddress("testrig", "test-agent")
	err := agents.StartWithConfig(id, startCfg("/tmp", "echo hello"))
	require.NoError(t, err)

	cmd := procs.GetCommand(session.SessionID(id))
	aaaIdx := indexOf(cmd, "AAA=first")
	mmmIdx := indexOf(cmd, "MMM=middle")
	zzzIdx := indexOf(cmd, "ZZZ=last")
	assert.True(t, aaaIdx < mmmIdx, "AAA should come before MMM")
	assert.True(t, mmmIdx < zzzIdx, "MMM should come before ZZZ")
}

func TestImplementation_Start_NoEnvVars_CommandUnchanged(t *testing.T) {
	procs := session.NewDouble()
	agents := agent.New(procs, nil)

	id := agent.PolecatAddress("testrig", "test-agent")
	err := agents.StartWithConfig(id, startCfg("/tmp", "echo hello"))
	require.NoError(t, err)

	cmd := procs.GetCommand(session.SessionID(id))
	assert.Equal(t, "echo hello", cmd)
}

// --- Timeout Tests (Implementation-specific) ---

func TestImplementation_Timeout_UsesConfigValue(t *testing.T) {
	procs := session.NewDouble()
	cfg := &agent.Config{
		Timeout: 5 * time.Second,
		Checker: &agent.PromptChecker{Prefix: "NEVER"},
	}
	agents := agent.New(procs, cfg)

	id := agent.PolecatAddress("testrig", "test-agent")
	_ = agents.StartWithConfig(id, startCfg("/tmp", "echo hello"))
	procs.SetBuffer(session.SessionID(id), []string{"no match"})

	start := time.Now()
	_ = agents.WaitReady(id)
	elapsed := time.Since(start)

	// Should timeout around 5 seconds (with some tolerance)
	assert.True(t, elapsed >= 4*time.Second && elapsed < 7*time.Second,
		"expected ~5s timeout, got %v", elapsed)
}

func TestImplementation_Timeout_DefaultsTo30Seconds(t *testing.T) {
	procs := session.NewDouble()
	cfg := &agent.Config{
		// Timeout: 0 means use default
		Checker: &agent.PromptChecker{Prefix: "NEVER"},
	}
	agents := agent.New(procs, cfg)

	id := agent.PolecatAddress("testrig", "test-agent")
	_ = agents.StartWithConfig(id, startCfg("/tmp", "echo hello"))
	procs.SetBuffer(session.SessionID(id), []string{"no match"})

	// We can't wait 30 seconds in a test, but we can verify it doesn't
	// return immediately when there's no match
	start := time.Now()
	done := make(chan bool)
	go func() {
		_ = agents.WaitReady(id)
		done <- true
	}()

	select {
	case <-done:
		// If it completed, check it took at least some time
		elapsed := time.Since(start)
		assert.True(t, elapsed >= 100*time.Millisecond, "should not return immediately")
	case <-time.After(500 * time.Millisecond):
		// Expected - it's still waiting (would wait 30s)
	}
}

// --- Graceful Stop Tests (Implementation-specific) ---

func TestImplementation_Stop_Graceful_TakesLongerThanNonGraceful(t *testing.T) {
	// Graceful stop sends Ctrl-C and waits before killing
	// We verify this by timing the operation

	procs := session.NewDouble()
	agents := agent.New(procs, nil)

	// Non-graceful stop
	id1 := agent.PolecatAddress("testrig", "agent1")
	_ = agents.StartWithConfig(id1, startCfg("/tmp", "echo hello"))
	start1 := time.Now()
	_ = agents.Stop(id1, false) // graceful=false
	elapsed1 := time.Since(start1)

	// Graceful stop (includes 100ms sleep after Ctrl-C)
	id2 := agent.PolecatAddress("testrig", "agent2")
	_ = agents.StartWithConfig(id2, startCfg("/tmp", "echo hello"))
	start2 := time.Now()
	_ = agents.Stop(id2, true) // graceful=true
	elapsed2 := time.Since(start2)

	// Graceful should take noticeably longer due to the sleep
	assert.True(t, elapsed2 > elapsed1, "graceful stop should take longer")
	assert.True(t, elapsed2 >= 100*time.Millisecond, "graceful stop should wait at least 100ms")
}

func TestImplementation_Stop_NotGraceful_SkipsCtrlC(t *testing.T) {
	procs := session.NewDouble()
	agents := agent.New(procs, nil)

	id := agent.PolecatAddress("testrig", "test-agent")
	_ = agents.StartWithConfig(id, startCfg("/tmp", "echo hello"))
	sessionID := session.SessionID(id)

	err := agents.Stop(id, false) // graceful=false
	require.NoError(t, err)

	// Verify no Ctrl-C was sent
	controls := procs.ControlLog(sessionID)
	assert.Empty(t, controls)
}

// --- Nudge Tests (Implementation-specific) ---

func TestImplementation_Nudge_SendsMessageToSession(t *testing.T) {
	procs := session.NewDouble()
	agents := agent.New(procs, nil)

	id := agent.PolecatAddress("testrig", "test-agent")
	_ = agents.StartWithConfig(id, startCfg("/tmp", "echo hello"))
	err := agents.Nudge(id, "HEALTH_CHECK: are you alive?")

	assert.NoError(t, err)

	// Verify message was logged in session double
	nudges := procs.NudgeLog(session.SessionID(id))
	assert.Contains(t, nudges, "HEALTH_CHECK: are you alive?")
}

func TestImplementation_Nudge_WhenSessionNotExists_ReturnsError(t *testing.T) {
	procs := session.NewDouble()
	agents := agent.New(procs, nil)

	id := agent.PolecatAddress("testrig", "nonexistent")
	err := agents.Nudge(id, "hello")

	assert.Error(t, err)
}

func TestImplementation_Nudge_MultipleCalls_AllRecorded(t *testing.T) {
	procs := session.NewDouble()
	agents := agent.New(procs, nil)

	id := agent.PolecatAddress("testrig", "test-agent")
	_ = agents.StartWithConfig(id, startCfg("/tmp", "echo hello"))
	_ = agents.Nudge(id, "message 1")
	_ = agents.Nudge(id, "message 2")
	_ = agents.Nudge(id, "message 3")

	nudges := procs.NudgeLog(session.SessionID(id))
	assert.Len(t, nudges, 3)
	assert.Equal(t, "message 1", nudges[0])
	assert.Equal(t, "message 2", nudges[1])
	assert.Equal(t, "message 3", nudges[2])
}

// --- StartupHook Tests (Implementation-specific) ---

func TestImplementation_StartupHook_CalledOnStart(t *testing.T) {
	procs := session.NewDouble()

	hookCalled := false
	var hookedSessionID session.SessionID
	cfg := &agent.Config{
		StartupHook: func(sess session.Sessions, id session.SessionID) error {
			hookCalled = true
			hookedSessionID = id
			return nil
		},
	}
	agents := agent.New(procs, cfg)

	id := agent.PolecatAddress("testrig", "test-agent")
	err := agents.StartWithConfig(id, startCfg("/tmp", "echo hello"))
	require.NoError(t, err)

	// Give the goroutine time to execute
	time.Sleep(100 * time.Millisecond)

	assert.True(t, hookCalled, "startup hook should be called")
	assert.Equal(t, session.SessionID(id), hookedSessionID)
}

func TestImplementation_StartupHook_ErrorIsNonFatal(t *testing.T) {
	procs := session.NewDouble()

	hookErr := errors.New("hook failed")
	cfg := &agent.Config{
		StartupHook: func(sess session.Sessions, id session.SessionID) error {
			return hookErr
		},
	}
	agents := agent.New(procs, cfg)

	// Start should still succeed even if startup hook fails
	id := agent.PolecatAddress("testrig", "test-agent")
	err := agents.StartWithConfig(id, startCfg("/tmp", "echo hello"))
	require.NoError(t, err)
	assert.True(t, agents.Exists(id))
}

// --- doWaitForReady Tests (Implementation-specific) ---

func TestImplementation_WaitReady_WithStartupDelay(t *testing.T) {
	procs := session.NewDouble()
	cfg := &agent.Config{
		StartupDelay: 100 * time.Millisecond,
		// No Checker - falls back to StartupDelay
	}
	agents := agent.New(procs, cfg)

	id := agent.PolecatAddress("testrig", "test-agent")
	_ = agents.StartWithConfig(id, startCfg("/tmp", "echo hello"))

	start := time.Now()
	err := agents.WaitReady(id)
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.True(t, elapsed >= 100*time.Millisecond, "should wait at least StartupDelay")
}

// --- Error Path Tests (using sessionsStub) ---

func TestImplementation_Start_WhenSessionStartFails_ReturnsError(t *testing.T) {
	procs := session.NewDouble()
	stub := newSessionsStub(procs)
	stub.StartErr = errors.New("session start failed")

	agents := agent.New(stub, nil)

	id := agent.PolecatAddress("testrig", "test-agent")
	err := agents.StartWithConfig(id, startCfg("/tmp", "echo hello"))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "starting session")
}

func TestImplementation_Start_WhenZombieCleanupFails_ReturnsError(t *testing.T) {
	procs := session.NewDouble()

	// Create a zombie session using the proper session ID
	id := agent.PolecatAddress("testrig", "test-agent")
	sessionID := session.SessionID(id)
	_, _ = procs.Start(string(sessionID), "/tmp", "dead-command")
	_ = procs.SetRunning(sessionID, false) // Process is dead

	// Now wrap with stub that fails on Stop (zombie cleanup)
	stub := newSessionsStub(procs)
	stub.StopErr = errors.New("stop failed")

	agents := agent.New(stub, nil)

	err := agents.StartWithConfig(id, startCfg("/tmp", "new-command"))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "killing zombie")
}

func TestImplementation_Stop_WhenSessionStopFails_ReturnsError(t *testing.T) {
	procs := session.NewDouble()
	stub := newSessionsStub(procs)

	agents := agent.New(stub, nil)

	id := agent.PolecatAddress("testrig", "test-agent")
	_ = agents.StartWithConfig(id, startCfg("/tmp", "echo hello"))

	// Now inject Stop error
	stub.StopErr = errors.New("stop failed")

	err := agents.Stop(id, false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stopping session")
}

// Helper function
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// --- Self() Tests ---

func TestSelf_Mayor(t *testing.T) {
	// Save and restore env
	orig := os.Getenv("GT_ROLE")
	defer os.Setenv("GT_ROLE", orig)

	os.Setenv("GT_ROLE", "mayor")
	id, err := agent.Self()

	assert.NoError(t, err)
	assert.Equal(t, agent.MayorAddress, id)
}

func TestSelf_Deacon(t *testing.T) {
	orig := os.Getenv("GT_ROLE")
	defer os.Setenv("GT_ROLE", orig)

	os.Setenv("GT_ROLE", "deacon")
	id, err := agent.Self()

	assert.NoError(t, err)
	assert.Equal(t, agent.DeaconAddress, id)
}

func TestSelf_Witness(t *testing.T) {
	origRole := os.Getenv("GT_ROLE")
	origRig := os.Getenv("GT_RIG")
	defer func() {
		os.Setenv("GT_ROLE", origRole)
		os.Setenv("GT_RIG", origRig)
	}()

	os.Setenv("GT_ROLE", "witness")
	os.Setenv("GT_RIG", "myrig")
	id, err := agent.Self()

	assert.NoError(t, err)
	assert.Equal(t, agent.WitnessAddress("myrig"), id)
}

func TestSelf_Refinery(t *testing.T) {
	origRole := os.Getenv("GT_ROLE")
	origRig := os.Getenv("GT_RIG")
	defer func() {
		os.Setenv("GT_ROLE", origRole)
		os.Setenv("GT_RIG", origRig)
	}()

	os.Setenv("GT_ROLE", "refinery")
	os.Setenv("GT_RIG", "myrig")
	id, err := agent.Self()

	assert.NoError(t, err)
	assert.Equal(t, agent.RefineryAddress("myrig"), id)
}

func TestSelf_Crew(t *testing.T) {
	origRole := os.Getenv("GT_ROLE")
	origRig := os.Getenv("GT_RIG")
	origCrew := os.Getenv("GT_CREW")
	defer func() {
		os.Setenv("GT_ROLE", origRole)
		os.Setenv("GT_RIG", origRig)
		os.Setenv("GT_CREW", origCrew)
	}()

	os.Setenv("GT_ROLE", "crew")
	os.Setenv("GT_RIG", "myrig")
	os.Setenv("GT_CREW", "max")
	id, err := agent.Self()

	assert.NoError(t, err)
	assert.Equal(t, agent.CrewAddress("myrig", "max"), id)
}

func TestSelf_Polecat(t *testing.T) {
	origRole := os.Getenv("GT_ROLE")
	origRig := os.Getenv("GT_RIG")
	origPolecat := os.Getenv("GT_POLECAT")
	defer func() {
		os.Setenv("GT_ROLE", origRole)
		os.Setenv("GT_RIG", origRig)
		os.Setenv("GT_POLECAT", origPolecat)
	}()

	os.Setenv("GT_ROLE", "polecat")
	os.Setenv("GT_RIG", "myrig")
	os.Setenv("GT_POLECAT", "Toast")
	id, err := agent.Self()

	assert.NoError(t, err)
	assert.Equal(t, agent.PolecatAddress("myrig", "Toast"), id)
}

func TestSelf_MissingRole_ReturnsError(t *testing.T) {
	origRole := os.Getenv("GT_ROLE")
	defer os.Setenv("GT_ROLE", origRole)

	os.Setenv("GT_ROLE", "")
	_, err := agent.Self()

	assert.Error(t, err)
	assert.ErrorIs(t, err, agent.ErrUnknownRole)
}

func TestSelf_UnknownRole_ReturnsError(t *testing.T) {
	origRole := os.Getenv("GT_ROLE")
	defer os.Setenv("GT_ROLE", origRole)

	os.Setenv("GT_ROLE", "bogus")
	_, err := agent.Self()

	assert.Error(t, err)
	assert.ErrorIs(t, err, agent.ErrUnknownRole)
}

func TestSelf_WitnessMissingRig_ReturnsError(t *testing.T) {
	origRole := os.Getenv("GT_ROLE")
	origRig := os.Getenv("GT_RIG")
	defer func() {
		os.Setenv("GT_ROLE", origRole)
		os.Setenv("GT_RIG", origRig)
	}()

	os.Setenv("GT_ROLE", "witness")
	os.Setenv("GT_RIG", "")
	_, err := agent.Self()

	assert.Error(t, err)
	assert.ErrorIs(t, err, agent.ErrUnknownRole)
}

func TestSelf_CrewMissingName_ReturnsError(t *testing.T) {
	origRole := os.Getenv("GT_ROLE")
	origRig := os.Getenv("GT_RIG")
	origCrew := os.Getenv("GT_CREW")
	defer func() {
		os.Setenv("GT_ROLE", origRole)
		os.Setenv("GT_RIG", origRig)
		os.Setenv("GT_CREW", origCrew)
	}()

	os.Setenv("GT_ROLE", "crew")
	os.Setenv("GT_RIG", "myrig")
	os.Setenv("GT_CREW", "")
	_, err := agent.Self()

	assert.Error(t, err)
	assert.ErrorIs(t, err, agent.ErrUnknownRole)
}
