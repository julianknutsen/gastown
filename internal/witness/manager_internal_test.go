package witness

import (
	"encoding/json"
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
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.local.json"), []byte(`{}`), 0644))

	r := &rig.Rig{
		Name:     "testrig",
		Path:     rigPath,
		Polecats: []string{"p1", "p2"},
	}

	agents := agent.NewDouble()
	return NewManager(r, agents, "claude"), rigPath
}

// =============================================================================
// stateFile Tests
// =============================================================================

func TestManager_stateFile(t *testing.T) {
	mgr, rigPath := setupTestManagerInternal(t)
	expected := filepath.Join(rigPath, ".runtime", "witness.json")
	assert.Equal(t, expected, mgr.stateFile())
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
	mgr := NewManager(r, agent.NewDouble(), "claude")

	assert.Equal(t, witnessRigDir, mgr.witnessDir())
}

func TestManager_witnessDir_FallsBackToWitness(t *testing.T) {
	tmpDir := t.TempDir()
	rigPath := filepath.Join(tmpDir, "testrig")

	// Create only witness directory (not witness/rig)
	witnessDir := filepath.Join(rigPath, "witness")
	require.NoError(t, os.MkdirAll(witnessDir, 0755))

	r := &rig.Rig{Name: "testrig", Path: rigPath}
	mgr := NewManager(r, agent.NewDouble(), "claude")

	assert.Equal(t, witnessDir, mgr.witnessDir())
}

func TestManager_witnessDir_FallsBackToRigPath(t *testing.T) {
	tmpDir := t.TempDir()
	rigPath := filepath.Join(tmpDir, "testrig")

	// Create only the rig directory (no witness subdirs)
	require.NoError(t, os.MkdirAll(rigPath, 0755))

	r := &rig.Rig{Name: "testrig", Path: rigPath}
	mgr := NewManager(r, agent.NewDouble(), "claude")

	assert.Equal(t, rigPath, mgr.witnessDir())
}

// =============================================================================
// townRoot Tests
// =============================================================================

func TestManager_townRoot_ReturnsRigPathWhenNoWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	rigPath := filepath.Join(tmpDir, "testrig")
	require.NoError(t, os.MkdirAll(rigPath, 0755))

	r := &rig.Rig{Name: "testrig", Path: rigPath}
	mgr := NewManager(r, agent.NewDouble(), "claude")

	// When no workspace is found, returns rig path
	assert.Equal(t, rigPath, mgr.townRoot())
}

func TestManager_townRoot_ReturnsWorkspaceWhenFound(t *testing.T) {
	tmpDir := t.TempDir()
	townPath := filepath.Join(tmpDir, "town")
	rigPath := filepath.Join(townPath, "rigs", "testrig")

	// Create workspace marker (mayor/town.json)
	require.NoError(t, os.MkdirAll(rigPath, 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(townPath, "mayor"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(townPath, "mayor", "town.json"), []byte(`{"name": "mytown"}`), 0644))

	r := &rig.Rig{Name: "testrig", Path: rigPath}
	mgr := NewManager(r, agent.NewDouble(), "claude")

	assert.Equal(t, townPath, mgr.townRoot())
}

// =============================================================================
// Status Error Path Tests
// =============================================================================

func TestManager_Status_WhenLoadStateFails_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	rigPath := filepath.Join(tmpDir, "testrig")
	require.NoError(t, os.MkdirAll(filepath.Join(rigPath, ".runtime"), 0755))

	// Write invalid JSON
	stateFile := filepath.Join(rigPath, ".runtime", "witness.json")
	require.NoError(t, os.WriteFile(stateFile, []byte("invalid json"), 0644))

	r := &rig.Rig{Name: "testrig", Path: rigPath}
	mgr := NewManager(r, agent.NewDouble(), "claude")

	_, err := mgr.Status()
	assert.Error(t, err)
}

// =============================================================================
// Start Error Path Tests
// =============================================================================

func TestManager_Start_WhenLoadStateFails_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	rigPath := filepath.Join(tmpDir, "testrig")
	require.NoError(t, os.MkdirAll(filepath.Join(rigPath, ".runtime"), 0755))

	// Write invalid JSON
	stateFile := filepath.Join(rigPath, ".runtime", "witness.json")
	require.NoError(t, os.WriteFile(stateFile, []byte("invalid json"), 0644))

	r := &rig.Rig{Name: "testrig", Path: rigPath}
	mgr := NewManager(r, agent.NewDouble(), "claude")

	err := mgr.Start()
	assert.Error(t, err)
}

func TestManager_Start_WhenEnsureSettingsFails_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	rigPath := filepath.Join(tmpDir, "testrig")
	require.NoError(t, os.MkdirAll(filepath.Join(rigPath, ".runtime"), 0755))

	// Create witness directory but make it read-only
	witnessDir := filepath.Join(rigPath, "witness")
	require.NoError(t, os.MkdirAll(witnessDir, 0755))
	require.NoError(t, os.Chmod(witnessDir, 0555))
	defer os.Chmod(witnessDir, 0755)

	r := &rig.Rig{Name: "testrig", Path: rigPath}
	mgr := NewManager(r, agent.NewDouble(), "claude")

	err := mgr.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ensuring runtime settings")
}

func TestManager_Start_WhenSaveStateFails_CleansUpAndReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	rigPath := filepath.Join(tmpDir, "testrig")
	runtimeDir := filepath.Join(rigPath, ".runtime")
	require.NoError(t, os.MkdirAll(runtimeDir, 0755))

	// Create witness directory with settings
	witnessDir := filepath.Join(rigPath, "witness")
	claudeDir := filepath.Join(witnessDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.local.json"), []byte(`{}`), 0644))

	r := &rig.Rig{Name: "testrig", Path: rigPath}
	agents := agent.NewDouble()
	mgr := NewManager(r, agents, "claude")

	// Make runtime directory read-only to cause saveState to fail
	require.NoError(t, os.Chmod(runtimeDir, 0555))
	defer os.Chmod(runtimeDir, 0755)

	err := mgr.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "saving state")

	// Verify agent was cleaned up
	agentID := agent.AgentID(mgr.SessionName())
	assert.False(t, agents.Exists(agentID), "agent should be cleaned up after saveState failure")
}

// =============================================================================
// Stop Error Path Tests
// =============================================================================

func TestManager_Stop_WhenLoadStateFails_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	rigPath := filepath.Join(tmpDir, "testrig")
	require.NoError(t, os.MkdirAll(filepath.Join(rigPath, ".runtime"), 0755))

	// Write invalid JSON
	stateFile := filepath.Join(rigPath, ".runtime", "witness.json")
	require.NoError(t, os.WriteFile(stateFile, []byte("invalid json"), 0644))

	r := &rig.Rig{Name: "testrig", Path: rigPath}
	mgr := NewManager(r, agent.NewDouble(), "claude")

	err := mgr.Stop()
	assert.Error(t, err)
}

func TestManager_Stop_WhenSaveStateFails_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	rigPath := filepath.Join(tmpDir, "testrig")
	runtimeDir := filepath.Join(rigPath, ".runtime")
	require.NoError(t, os.MkdirAll(runtimeDir, 0755))

	r := &rig.Rig{Name: "testrig", Path: rigPath}
	agents := agent.NewDouble()
	mgr := NewManager(r, agents, "claude")

	// Create agent (need running state)
	sessionName := mgr.SessionName()
	agents.CreateAgent(agent.AgentID(sessionName))

	// Write running state
	stateFile := filepath.Join(runtimeDir, "witness.json")
	state := Witness{RigName: "testrig", State: StateRunning}
	data, _ := json.MarshalIndent(state, "", "  ")
	require.NoError(t, os.WriteFile(stateFile, data, 0644))

	// Make runtime directory read-only
	require.NoError(t, os.Chmod(runtimeDir, 0555))
	defer os.Chmod(runtimeDir, 0755)

	err := mgr.Stop()
	assert.Error(t, err)
}
