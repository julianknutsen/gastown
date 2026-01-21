package refinery

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/steveyegge/gastown/internal/agent"
	"github.com/steveyegge/gastown/internal/rig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestManager(t *testing.T) (*Manager, *agent.Double, string) {
	t.Helper()

	// Create temp directory structure
	tmpDir := t.TempDir()
	rigPath := filepath.Join(tmpDir, "testrig")
	if err := os.MkdirAll(filepath.Join(rigPath, ".runtime"), 0755); err != nil {
		t.Fatalf("mkdir .runtime: %v", err)
	}

	r := &rig.Rig{
		Name: "testrig",
		Path: rigPath,
	}

	agents := agent.NewDouble()
	return NewManager(agents, r), agents, rigPath
}

// =============================================================================
// ZFC-Compliant Tests: Deprecated Methods
// RegisterMR and Retry are deprecated in favor of beads as source of truth.
// =============================================================================

func TestManager_RegisterMR_ReturnsDeprecatedError(t *testing.T) {
	mgr, _, _ := setupTestManager(t)

	mr := &MergeRequest{
		ID:     "gt-mr-new",
		Branch: "polecat/Cheedo/gt-123",
		Worker: "Cheedo",
		Status: MROpen,
	}

	err := mgr.RegisterMR(mr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "deprecated")
}

func TestManager_Retry_ReturnsNilAndPrintsDeprecation(t *testing.T) {
	mgr, _, _ := setupTestManager(t)

	var buf bytes.Buffer
	mgr.SetOutput(&buf)

	// Retry is deprecated - always returns nil and prints message
	err := mgr.Retry("any-id", false)
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "deprecated")
}

// =============================================================================
// ZFC-Compliant Status Tests
// Status() returns *session.Info from tmux (source of truth)
// =============================================================================

func setupTestManagerForStatus(t *testing.T) (*Manager, *agent.Double, string) {
	t.Helper()
	tmpDir := t.TempDir()
	rigPath := filepath.Join(tmpDir, "testrig")

	// Create required directories
	require.NoError(t, os.MkdirAll(filepath.Join(rigPath, ".runtime"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(rigPath, "refinery", "rig"), 0755))

	r := &rig.Rig{
		Name: "testrig",
		Path: rigPath,
	}

	agents := agent.NewDouble()
	return NewManager(agents, r), agents, rigPath
}

func TestManager_Status_WhenNotRunning_ReturnsNil(t *testing.T) {
	mgr, _, _ := setupTestManagerForStatus(t)

	// Agent doesn't exist
	status, err := mgr.Status()
	require.NoError(t, err)
	assert.Nil(t, status, "Status should be nil when not running")
}

func TestManager_Status_WhenRunning_ReturnsSessionInfo(t *testing.T) {
	mgr, agents, _ := setupTestManagerForStatus(t)

	// Create running agent
	agentID := mgr.Address()
	agents.CreateAgent(agentID)

	status, err := mgr.Status()
	require.NoError(t, err)
	assert.NotNil(t, status, "Status should return session info when running")
}

// --- IsRunning() Tests ---

func TestManager_IsRunning_WhenAgentExists_ReturnsTrue(t *testing.T) {
	mgr, agents, _ := setupTestManagerForStatus(t)

	agentID := mgr.Address()
	agents.CreateAgent(agentID)

	assert.True(t, mgr.IsRunning())
}

func TestManager_IsRunning_WhenAgentNotExists_ReturnsFalse(t *testing.T) {
	mgr, _, _ := setupTestManagerForStatus(t)

	assert.False(t, mgr.IsRunning())
}

// --- SessionName() Tests ---

func TestManager_SessionName_Format(t *testing.T) {
	mgr, _, _ := setupTestManagerForStatus(t)
	assert.Equal(t, "gt-testrig-refinery", mgr.SessionName())
}

// --- Address() Tests ---

func TestManager_Address_ReturnsCorrectAgentID(t *testing.T) {
	mgr, _, _ := setupTestManagerForStatus(t)
	expected := agent.RefineryAddress("testrig")
	assert.Equal(t, expected, mgr.Address())
}

// NOTE: Tests for LoadState/SaveState have been removed.
// These methods no longer exist - ZFC-compliant: no state files.
// NOTE: Tests for Status() returning Refinery state have been removed.
// Status() now returns *session.Info (tmux is source of truth).
