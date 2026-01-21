package witness_test

import (
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
//
// Note: Start/Stop operations are handled by factory.Start()/factory.Agents().Stop()
// The Manager handles status queries (ZFC-compliant: tmux is source of truth).
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
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(`{}`), 0644))

	return &rig.Rig{
		Name:     "testrig",
		Path:     rigPath,
		Polecats: []string{"p1", "p2"},
	}, rigPath
}

// --- ZFC-Compliant Status() Tests ---
// Status() returns *session.Info from tmux (source of truth)

func TestManager_Status_WhenNotRunning_ReturnsNil(t *testing.T) {
	r, _ := setupTestRig(t)
	agents := agent.NewDouble()
	mgr := witness.NewManager(agents, r)

	// Agent doesn't exist
	status, err := mgr.Status()
	require.NoError(t, err)
	assert.Nil(t, status, "Status should be nil when not running")
}

func TestManager_Status_WhenRunning_ReturnsSessionInfo(t *testing.T) {
	r, _ := setupTestRig(t)
	agents := agent.NewDouble()
	mgr := witness.NewManager(agents, r)

	// Create running agent
	agentID := agent.WitnessAddress(r.Name)
	agents.CreateAgent(agentID)

	status, err := mgr.Status()
	require.NoError(t, err)
	assert.NotNil(t, status, "Status should return session info when running")
}

// --- IsRunning() Tests ---

func TestManager_IsRunning_WhenAgentExists_ReturnsTrue(t *testing.T) {
	r, _ := setupTestRig(t)
	agents := agent.NewDouble()
	mgr := witness.NewManager(agents, r)

	agentID := agent.WitnessAddress(r.Name)
	agents.CreateAgent(agentID)

	assert.True(t, mgr.IsRunning())
}

func TestManager_IsRunning_WhenAgentNotExists_ReturnsFalse(t *testing.T) {
	r, _ := setupTestRig(t)
	agents := agent.NewDouble()
	mgr := witness.NewManager(agents, r)

	assert.False(t, mgr.IsRunning())
}

// --- SessionName() Tests ---

func TestManager_SessionName_Format(t *testing.T) {
	r, _ := setupTestRig(t)
	agents := agent.NewDouble()
	mgr := witness.NewManager(agents, r)

	assert.Equal(t, "gt-testrig-witness", mgr.SessionName())
}

// --- Address() Tests ---

func TestManager_Address_ReturnsCorrectAgentID(t *testing.T) {
	r, _ := setupTestRig(t)
	agents := agent.NewDouble()
	mgr := witness.NewManager(agents, r)

	expected := agent.WitnessAddress(r.Name)
	assert.Equal(t, expected, mgr.Address())
}

// NOTE: Tests for LoadState/SaveState have been removed.
// These methods no longer exist - ZFC-compliant: no state files.
// NOTE: Tests for Status() returning Witness state have been removed.
// Status() now returns *session.Info (tmux is source of truth).
