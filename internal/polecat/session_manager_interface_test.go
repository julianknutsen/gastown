package polecat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/steveyegge/gastown/internal/agent"
	"github.com/steveyegge/gastown/internal/rig"
	"github.com/steveyegge/gastown/internal/session"
)

// These tests use agent.Double and session.Double (same pattern as other managers).

func TestStart_CreatesSessionWithCorrectCommand(t *testing.T) {
	// Setup: create temp dir with polecat structure
	root := t.TempDir()
	polecatDir := filepath.Join(root, "polecats", "Toast", "testrig")
	if err := os.MkdirAll(polecatDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create .git to make it look like a worktree
	if err := os.WriteFile(filepath.Join(polecatDir, ".git"), []byte("gitdir: ../../../.git/worktrees/Toast"), 0644); err != nil {
		t.Fatal(err)
	}

	agents := agent.NewDouble()
	sess := session.NewDouble()
	r := &rig.Rig{Name: "testrig", Path: root}
	m := NewSessionManager(agents, sess, r, "")

	// Act: start the polecat
	err := m.Start("Toast")

	// Assert: no error
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify agent was created
	if agents.AgentCount() != 1 {
		t.Fatalf("expected 1 agent, got %d", agents.AgentCount())
	}

	// Verify agent ID
	agentID := agent.AgentID("gt-testrig-Toast")
	if !agents.Exists(agentID) {
		t.Error("agent gt-testrig-Toast should exist")
	}

	// Verify command contains required env vars
	command := agents.GetCommand(agentID)
	requiredEnvVars := []string{"GT_ROLE=polecat", "GT_RIG=testrig", "GT_POLECAT=Toast"}
	for _, env := range requiredEnvVars {
		if !strings.Contains(command, env) {
			t.Errorf("command missing %q: %s", env, command)
		}
	}
}

func TestStart_RejectsAlreadyRunningSession(t *testing.T) {
	root := t.TempDir()
	polecatDir := filepath.Join(root, "polecats", "Toast", "testrig")
	os.MkdirAll(polecatDir, 0755)
	os.WriteFile(filepath.Join(polecatDir, ".git"), []byte("gitdir: test"), 0644)

	agents := agent.NewDouble()
	sess := session.NewDouble()
	// Pre-create agent
	agents.CreateAgent(agent.AgentID("gt-testrig-Toast"))

	r := &rig.Rig{Name: "testrig", Path: root}
	m := NewSessionManager(agents, sess, r, "")

	err := m.Start("Toast")

	if err == nil {
		t.Fatal("expected error for already running session")
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("error = %q, want 'already running'", err)
	}
}

func TestStop_TerminatesSession(t *testing.T) {
	root := t.TempDir()

	agents := agent.NewDouble()
	sess := session.NewDouble()
	agents.CreateAgent(agent.AgentID("gt-testrig-Toast"))

	r := &rig.Rig{Name: "testrig", Path: root}
	m := NewSessionManager(agents, sess, r, "")

	// Stop the agent
	err := m.Stop("Toast", false)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify agent was stopped
	if agents.Exists(agent.AgentID("gt-testrig-Toast")) {
		t.Error("agent should be stopped")
	}
}

func TestStop_ForceSkipsGracefulShutdown(t *testing.T) {
	root := t.TempDir()

	agents := agent.NewDouble()
	sess := session.NewDouble()
	agents.CreateAgent(agent.AgentID("gt-testrig-Toast"))

	r := &rig.Rig{Name: "testrig", Path: root}
	m := NewSessionManager(agents, sess, r, "")

	// Force stop
	err := m.Stop("Toast", true)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify agent was stopped
	if agents.Exists(agent.AgentID("gt-testrig-Toast")) {
		t.Error("agent should be stopped")
	}
}

func TestIsRunning_ReturnsTrue(t *testing.T) {
	agents := agent.NewDouble()
	sess := session.NewDouble()
	agents.CreateAgent(agent.AgentID("gt-testrig-Toast"))

	r := &rig.Rig{Name: "testrig", Path: "/tmp"}
	m := NewSessionManager(agents, sess, r, "")

	running, err := m.IsRunning("Toast")
	if err != nil {
		t.Fatalf("IsRunning error: %v", err)
	}
	if !running {
		t.Error("expected IsRunning = true")
	}
}

func TestIsRunning_ReturnsFalse(t *testing.T) {
	agents := agent.NewDouble()
	sess := session.NewDouble()
	// Don't create the agent

	r := &rig.Rig{Name: "testrig", Path: "/tmp"}
	m := NewSessionManager(agents, sess, r, "")

	running, err := m.IsRunning("Toast")
	if err != nil {
		t.Fatalf("IsRunning error: %v", err)
	}
	if running {
		t.Error("expected IsRunning = false")
	}
}

func TestCapture_ReturnsOutput(t *testing.T) {
	agents := agent.NewDouble()
	sess := session.NewDouble()
	sess.Start("gt-testrig-Toast", "/tmp", "")

	// Set buffer content for capture
	sessionID := session.SessionID("gt-testrig-Toast")
	sess.SetBuffer(sessionID, []string{"line1", "line2", "line3"})

	r := &rig.Rig{Name: "testrig", Path: "/tmp"}
	m := NewSessionManager(agents, sess, r, "")

	output, err := m.Capture("Toast", 50)
	if err != nil {
		t.Fatalf("Capture error: %v", err)
	}
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestInject_SendsMessage(t *testing.T) {
	agents := agent.NewDouble()
	sess := session.NewDouble()
	sess.Start("gt-testrig-Toast", "/tmp", "")

	r := &rig.Rig{Name: "testrig", Path: "/tmp"}
	m := NewSessionManager(agents, sess, r, "")

	message := "Hello, polecat!"
	err := m.Inject("Toast", message)
	if err != nil {
		t.Fatalf("Inject error: %v", err)
	}
	// Injection succeeded - with session.Double this uses Send() fallback
}

func TestList_FiltersByRigPrefix(t *testing.T) {
	agents := agent.NewDouble()
	sess := session.NewDouble()
	// Add sessions for different rigs
	sess.Start("gt-testrig-Toast", "/tmp", "")
	sess.Start("gt-testrig-Furiosa", "/tmp", "")
	sess.Start("gt-otherrig-Max", "/tmp", "")      // Different rig
	sess.Start("hq-mayor", "/tmp", "")             // Not a polecat
	sess.Start("random-session", "/tmp", "")       // Not gt- prefix

	r := &rig.Rig{Name: "testrig", Path: "/tmp"}
	m := NewSessionManager(agents, sess, r, "")

	infos, err := m.List()
	if err != nil {
		t.Fatalf("List error: %v", err)
	}

	// Should only return testrig polecats
	if len(infos) != 2 {
		t.Fatalf("expected 2 sessions for testrig, got %d", len(infos))
	}

	names := make(map[string]bool)
	for _, info := range infos {
		names[info.Polecat] = true
	}

	if !names["Toast"] {
		t.Error("expected Toast in list")
	}
	if !names["Furiosa"] {
		t.Error("expected Furiosa in list")
	}
	if names["Max"] {
		t.Error("Max should not be in list (different rig)")
	}
}

func TestStatus_PopulatesSessionInfo(t *testing.T) {
	agents := agent.NewDouble()
	sess := session.NewDouble()
	sess.Start("gt-testrig-Toast", "/tmp", "")

	r := &rig.Rig{Name: "testrig", Path: "/tmp"}
	m := NewSessionManager(agents, sess, r, "")

	info, err := m.Status("Toast")
	if err != nil {
		t.Fatalf("Status error: %v", err)
	}

	if !info.Running {
		t.Error("expected Running = true")
	}
	if info.Polecat != "Toast" {
		t.Errorf("Polecat = %q, want Toast", info.Polecat)
	}
	if info.SessionID != "gt-testrig-Toast" {
		t.Errorf("SessionID = %q, want gt-testrig-Toast", info.SessionID)
	}
	if info.RigName != "testrig" {
		t.Errorf("RigName = %q, want testrig", info.RigName)
	}
}

func TestStopAll_StopsAllSessions(t *testing.T) {
	agents := agent.NewDouble()
	sess := session.NewDouble()
	sess.Start("gt-testrig-Toast", "/tmp", "")
	sess.Start("gt-testrig-Furiosa", "/tmp", "")
	sess.Start("gt-testrig-Max", "/tmp", "")
	agents.CreateAgent(agent.AgentID("gt-testrig-Toast"))
	agents.CreateAgent(agent.AgentID("gt-testrig-Furiosa"))
	agents.CreateAgent(agent.AgentID("gt-testrig-Max"))

	r := &rig.Rig{Name: "testrig", Path: "/tmp"}
	m := NewSessionManager(agents, sess, r, "")

	err := m.StopAll(false)
	if err != nil {
		t.Fatalf("StopAll error: %v", err)
	}

	// Verify all agents were stopped
	if agents.AgentCount() != 0 {
		t.Errorf("expected 0 agents after StopAll, got %d", agents.AgentCount())
	}
}

func TestStopAll_EmptyList_Succeeds(t *testing.T) {
	agents := agent.NewDouble()
	sess := session.NewDouble()
	// No sessions added

	r := &rig.Rig{Name: "testrig", Path: "/tmp"}
	m := NewSessionManager(agents, sess, r, "")

	err := m.StopAll(false)
	if err != nil {
		t.Errorf("StopAll on empty list should succeed, got: %v", err)
	}
}

func TestStopAll_Force_SkipsGracefulShutdown(t *testing.T) {
	agents := agent.NewDouble()
	sess := session.NewDouble()
	sess.Start("gt-testrig-Toast", "/tmp", "")
	agents.CreateAgent(agent.AgentID("gt-testrig-Toast"))

	r := &rig.Rig{Name: "testrig", Path: "/tmp"}
	m := NewSessionManager(agents, sess, r, "")

	err := m.StopAll(true) // force=true
	if err != nil {
		t.Fatalf("StopAll error: %v", err)
	}

	// Verify agent was stopped
	if agents.AgentCount() != 0 {
		t.Error("expected all agents to be stopped")
	}
}
