package agent_test

import (
	"errors"
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

// --- Zombie Detection Tests (Implementation-specific) ---

func TestImplementation_Start_DetectsAndCleansUpZombie(t *testing.T) {
	procs := session.NewDouble()
	agents := agent.New(procs, nil)

	// Create zombie: session exists but process dead
	sessionID := session.SessionID("test-agent")
	_, _ = procs.Start("test-agent", "/tmp", "dead-command")
	_ = procs.SetRunning(sessionID, false) // Process is dead

	// Start should clean up zombie and create new session
	id, err := agents.Start("test-agent", "/tmp", "new-command")
	require.NoError(t, err)
	assert.NotEmpty(t, id)
	assert.True(t, agents.Exists(id))
}

func TestImplementation_Start_AlreadyRunning_WithLiveProcess(t *testing.T) {
	procs := session.NewDouble()
	agents := agent.New(procs, nil)

	// Pre-create session with live process
	sessionID := session.SessionID("test-agent")
	_, _ = procs.Start("test-agent", "/tmp", "running-command")
	_ = procs.SetRunning(sessionID, true)

	_, err := agents.Start("test-agent", "/tmp", "echo hello")
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

	_, err := agents.Start("test-agent", "/tmp", "echo hello")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session setup")

	// Session should be cleaned up
	exists, _ := procs.Exists(session.SessionID("test-agent"))
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

	id, err := agents.Start("test-agent", "/tmp", "echo hello")
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

	_, err := agents.Start("test-agent", "/tmp", "echo hello")
	require.NoError(t, err)

	cmd := procs.GetCommand(session.SessionID("test-agent"))
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

	_, err := agents.Start("test-agent", "/tmp", "echo hello")
	require.NoError(t, err)

	cmd := procs.GetCommand(session.SessionID("test-agent"))
	aaaIdx := indexOf(cmd, "AAA=first")
	mmmIdx := indexOf(cmd, "MMM=middle")
	zzzIdx := indexOf(cmd, "ZZZ=last")
	assert.True(t, aaaIdx < mmmIdx, "AAA should come before MMM")
	assert.True(t, mmmIdx < zzzIdx, "MMM should come before ZZZ")
}

func TestImplementation_Start_NoEnvVars_CommandUnchanged(t *testing.T) {
	procs := session.NewDouble()
	agents := agent.New(procs, nil)

	_, err := agents.Start("test-agent", "/tmp", "echo hello")
	require.NoError(t, err)

	cmd := procs.GetCommand(session.SessionID("test-agent"))
	assert.Equal(t, "echo hello", cmd)
}

// --- IsReady Tests (Implementation-specific, not in Agents interface) ---

func TestImplementation_IsReady_WhenNoSession_ReturnsFalse(t *testing.T) {
	procs := session.NewDouble()
	cfg := &agent.Config{
		Checker: &agent.PromptChecker{Prefix: ">"},
	}
	agents := agent.New(procs, cfg)

	assert.False(t, agents.IsReady("nonexistent"))
}

func TestImplementation_IsReady_WhenNoChecker_ReturnsFalse(t *testing.T) {
	procs := session.NewDouble()
	agents := agent.New(procs, &agent.Config{}) // No checker

	id, _ := agents.Start("test-agent", "/tmp", "echo hello")
	assert.False(t, agents.IsReady(id))
}

func TestImplementation_IsReady_WhenCheckerMatchesPrompt_ReturnsTrue(t *testing.T) {
	procs := session.NewDouble()
	cfg := &agent.Config{
		Checker: &agent.PromptChecker{Prefix: ">"},
	}
	agents := agent.New(procs, cfg)

	id, _ := agents.Start("test-agent", "/tmp", "echo hello")
	// session.Double initializes buffer with "> " which matches the checker
	assert.True(t, agents.IsReady(id))
}

func TestImplementation_IsReady_WhenCheckerDoesNotMatch_ReturnsFalse(t *testing.T) {
	procs := session.NewDouble()
	cfg := &agent.Config{
		Checker: &agent.PromptChecker{Prefix: "NEVER_MATCHES"},
	}
	agents := agent.New(procs, cfg)

	id, _ := agents.Start("test-agent", "/tmp", "echo hello")
	procs.SetBuffer(session.SessionID(id), []string{"some other output"})
	assert.False(t, agents.IsReady(id))
}

func TestImplementation_IsReady_WhenCaptureErrors_ReturnsFalse(t *testing.T) {
	// This tests the race condition path: session exists but Capture fails
	// (e.g., session killed between Exists check and Capture call)
	procs := session.NewDouble()
	stub := newSessionsStub(procs)

	cfg := &agent.Config{
		Checker: &agent.PromptChecker{Prefix: ">"},
	}
	agents := agent.New(stub, cfg)

	// Start session so Exists returns true
	id, _ := agents.Start("test-agent", "/tmp", "echo hello")

	// Inject Capture error to simulate race condition
	stub.CaptureErr = errors.New("session disappeared")

	assert.False(t, agents.IsReady(id))
}

// --- Sessions() Accessor Test (Implementation-specific) ---

func TestImplementation_Sessions_ReturnsUnderlyingSessions(t *testing.T) {
	procs := session.NewDouble()
	agents := agent.New(procs, nil)

	// Sessions() returns the underlying session.Sessions
	sess := agents.Sessions()
	assert.Equal(t, procs, sess)
}

// --- Timeout Tests (Implementation-specific) ---

func TestImplementation_Timeout_UsesConfigValue(t *testing.T) {
	procs := session.NewDouble()
	cfg := &agent.Config{
		Timeout: 5 * time.Second,
		Checker: &agent.PromptChecker{Prefix: "NEVER"},
	}
	agents := agent.New(procs, cfg)

	id, _ := agents.Start("test-agent", "/tmp", "echo hello")
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

	id, _ := agents.Start("test-agent", "/tmp", "echo hello")
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
	id1, _ := agents.Start("agent1", "/tmp", "echo hello")
	start1 := time.Now()
	_ = agents.Stop(id1, false) // graceful=false
	elapsed1 := time.Since(start1)

	// Graceful stop (includes 100ms sleep after Ctrl-C)
	id2, _ := agents.Start("agent2", "/tmp", "echo hello")
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

	id, _ := agents.Start("test-agent", "/tmp", "echo hello")
	sessionID := agents.SessionID(id)

	err := agents.Stop(id, false) // graceful=false
	require.NoError(t, err)

	// Verify no Ctrl-C was sent
	controls := procs.ControlLog(sessionID)
	assert.Empty(t, controls)
}

// --- Restart Tests (Implementation-specific) ---

func TestImplementation_Restart_StopsAndStarts(t *testing.T) {
	procs := session.NewDouble()
	agents := agent.New(procs, nil)

	id, _ := agents.Start("test-agent", "/tmp", "old-command")
	err := agents.Restart(id, "/new/dir", "new-command")
	require.NoError(t, err)

	assert.True(t, agents.Exists(id))
}

func TestImplementation_Restart_WhenAgentNotExists_StillSucceeds(t *testing.T) {
	procs := session.NewDouble()
	agents := agent.New(procs, nil)

	// Restart on non-existent agent - Stop is idempotent
	err := agents.Restart("nonexistent", "/tmp", "command")
	require.NoError(t, err)
}

// --- Send/Capture Tests (Implementation-specific) ---

func TestImplementation_Send_SendsTextToSession(t *testing.T) {
	procs := session.NewDouble()
	agents := agent.New(procs, nil)

	id, _ := agents.Start("test-agent", "/tmp", "echo hello")
	err := agents.Send(id, "test input")

	assert.NoError(t, err)
}

func TestImplementation_Capture_ReturnsSessionOutput(t *testing.T) {
	procs := session.NewDouble()
	agents := agent.New(procs, nil)

	id, _ := agents.Start("test-agent", "/tmp", "echo hello")
	procs.SetBuffer(session.SessionID(id), []string{"line1", "line2", "line3"})

	output, err := agents.Capture(id, 10)
	require.NoError(t, err)
	assert.Contains(t, output, "line1")
	assert.Contains(t, output, "line2")
	assert.Contains(t, output, "line3")
}

func TestImplementation_SendControl_SendsControlKey(t *testing.T) {
	procs := session.NewDouble()
	agents := agent.New(procs, nil)

	id, _ := agents.Start("test-agent", "/tmp", "echo hello")
	sessionID := agents.SessionID(id)

	err := agents.SendControl(id, "C-c")
	require.NoError(t, err)

	controls := procs.ControlLog(sessionID)
	assert.Contains(t, controls, "C-c")
}

// --- Nudge Tests (Implementation-specific) ---

func TestImplementation_Nudge_SendsMessageToSession(t *testing.T) {
	procs := session.NewDouble()
	agents := agent.New(procs, nil)

	id, _ := agents.Start("test-agent", "/tmp", "echo hello")
	err := agents.Nudge(id, "HEALTH_CHECK: are you alive?")

	assert.NoError(t, err)

	// Verify message was logged in session double
	nudges := procs.NudgeLog(session.SessionID(id))
	assert.Contains(t, nudges, "HEALTH_CHECK: are you alive?")
}

func TestImplementation_Nudge_WhenSessionNotExists_ReturnsError(t *testing.T) {
	procs := session.NewDouble()
	agents := agent.New(procs, nil)

	err := agents.Nudge("nonexistent", "hello")

	assert.Error(t, err)
}

func TestImplementation_Nudge_MultipleCalls_AllRecorded(t *testing.T) {
	procs := session.NewDouble()
	agents := agent.New(procs, nil)

	id, _ := agents.Start("test-agent", "/tmp", "echo hello")
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

	id, err := agents.Start("test-agent", "/tmp", "echo hello")
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
	id, err := agents.Start("test-agent", "/tmp", "echo hello")
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

	id, _ := agents.Start("test-agent", "/tmp", "echo hello")

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

	_, err := agents.Start("test-agent", "/tmp", "echo hello")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "starting session")
}

func TestImplementation_Start_WhenZombieCleanupFails_ReturnsError(t *testing.T) {
	procs := session.NewDouble()

	// Create a zombie session
	sessionID := session.SessionID("test-agent")
	_, _ = procs.Start("test-agent", "/tmp", "dead-command")
	_ = procs.SetRunning(sessionID, false) // Process is dead

	// Now wrap with stub that fails on Stop (zombie cleanup)
	stub := newSessionsStub(procs)
	stub.StopErr = errors.New("stop failed")

	agents := agent.New(stub, nil)

	_, err := agents.Start("test-agent", "/tmp", "new-command")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "killing zombie")
}

func TestImplementation_Stop_WhenSessionStopFails_ReturnsError(t *testing.T) {
	procs := session.NewDouble()
	stub := newSessionsStub(procs)

	agents := agent.New(stub, nil)

	id, _ := agents.Start("test-agent", "/tmp", "echo hello")

	// Now inject Stop error
	stub.StopErr = errors.New("stop failed")

	err := agents.Stop(id, false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stopping session")
}

func TestImplementation_Restart_WhenStopFails_ReturnsError(t *testing.T) {
	procs := session.NewDouble()
	stub := newSessionsStub(procs)

	agents := agent.New(stub, nil)

	id, _ := agents.Start("test-agent", "/tmp", "old-command")

	// Inject Stop error
	stub.StopErr = errors.New("stop failed")

	err := agents.Restart(id, "/tmp", "new-command")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stopping for restart")
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
