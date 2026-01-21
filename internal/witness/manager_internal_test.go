package witness

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/steveyegge/gastown/internal/agent"
	"github.com/steveyegge/gastown/internal/rig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestManagerInternal(t *testing.T) (*Manager, string) {
	t.Helper()
	tmpDir := t.TempDir()
	rigPath := filepath.Join(tmpDir, "testrig")

	require.NoError(t, os.MkdirAll(filepath.Join(rigPath, ".runtime"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(rigPath, "witness"), 0755))

	// Create minimal Claude settings
	claudeDir := filepath.Join(rigPath, "witness", ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(`{}`), 0644))

	r := &rig.Rig{
		Name:     "testrig",
		Path:     rigPath,
		Polecats: []string{"p1", "p2"},
	}

	agents := agent.NewDouble()
	return NewManager(agents, r), rigPath
}

// =============================================================================
// witnessDir Tests
// =============================================================================

func TestManager_witnessDir_PrefersWitnessRig(t *testing.T) {
	tmpDir := t.TempDir()
	rigPath := filepath.Join(tmpDir, "testrig")

	// Create witness/rig directory
	witnessRigDir := filepath.Join(rigPath, "witness", "rig")
	require.NoError(t, os.MkdirAll(witnessRigDir, 0755))

	r := &rig.Rig{Name: "testrig", Path: rigPath}
	mgr := NewManager(agent.NewDouble(), r)

	assert.Equal(t, witnessRigDir, mgr.witnessDir())
}

func TestManager_witnessDir_FallsBackToWitness(t *testing.T) {
	tmpDir := t.TempDir()
	rigPath := filepath.Join(tmpDir, "testrig")

	// Create only witness directory (not witness/rig)
	witnessDir := filepath.Join(rigPath, "witness")
	require.NoError(t, os.MkdirAll(witnessDir, 0755))

	r := &rig.Rig{Name: "testrig", Path: rigPath}
	mgr := NewManager(agent.NewDouble(), r)

	assert.Equal(t, witnessDir, mgr.witnessDir())
}

func TestManager_witnessDir_FallsBackToRigPath(t *testing.T) {
	tmpDir := t.TempDir()
	rigPath := filepath.Join(tmpDir, "testrig")

	// Create only the rig directory (no witness subdirs)
	require.NoError(t, os.MkdirAll(rigPath, 0755))

	r := &rig.Rig{Name: "testrig", Path: rigPath}
	mgr := NewManager(agent.NewDouble(), r)

	assert.Equal(t, rigPath, mgr.witnessDir())
}

// =============================================================================
// ZFC-Compliant Status Tests
// Status() returns *session.Info from tmux (source of truth)
// =============================================================================

func TestManager_Status_WhenNotRunning_ReturnsNil(t *testing.T) {
	mgr, _ := setupTestManagerInternal(t)

	// Agent doesn't exist
	status, err := mgr.Status()
	require.NoError(t, err)
	assert.Nil(t, status, "Status should be nil when not running")
}

func TestManager_Status_WhenRunning_ReturnsSessionInfo(t *testing.T) {
	tmpDir := t.TempDir()
	rigPath := filepath.Join(tmpDir, "testrig")
	require.NoError(t, os.MkdirAll(rigPath, 0755))

	r := &rig.Rig{Name: "testrig", Path: rigPath, Polecats: []string{}}
	agents := agent.NewDouble()
	mgr := NewManager(agents, r)

	// Create running agent
	agentID := mgr.Address()
	agents.CreateAgent(agentID)

	status, err := mgr.Status()
	require.NoError(t, err)
	assert.NotNil(t, status, "Status should return session info when running")
}

// NOTE: Tests for LoadState/SaveState have been removed.
// These methods no longer exist - ZFC-compliant: no state files.
// NOTE: Tests for Status() returning Witness state have been removed.
// Status() now returns *session.Info (tmux is source of truth).
