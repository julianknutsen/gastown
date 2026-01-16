package beads_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/steveyegge/gastown/internal/beads"
)

// =============================================================================
// Cross-Rig Routing Tests
//
// These tests verify that bd commands correctly route based on bead ID prefix.
// We assume ALL commands support routing. When a test fails against real bd,
// add the command to KnownBrokenRouting and provide a workaround.
//
// Testing approach:
// 1. Create a multi-rig environment with routes.jsonl
// 2. Create beads in different rigs
// 3. Attempt operations from a different rig's context
// 4. If the operation fails, the command needs a workaround
//
// To run against real bd: go test -tags=integration ./internal/beads/...
// =============================================================================

// CrossRigEnv creates a test environment simulating production:
// - Town root with routes.jsonl
// - Two rigs (gastown with gt- prefix, ai_platform with ap- prefix)
// - Redirects from rig root to mayor/rig/.beads
type CrossRigEnv struct {
	TownRoot    string
	GastownDir  string // Uses "gt-" prefix
	AIPlat      string // Uses "ap-" prefix
	GastownBead string // A bead ID like "gt-1"
	AIPlatBead  string // A bead ID like "ap-1"
}

func setupCrossRigEnv(t *testing.T) *CrossRigEnv {
	t.Helper()

	townRoot := t.TempDir()

	// Create town-level beads directory
	townBeads := filepath.Join(townRoot, ".beads")
	_ = os.MkdirAll(townBeads, 0755)

	// Create two rigs with their own beads directories
	gastownBeads := filepath.Join(townRoot, "gastown", "mayor", "rig", ".beads")
	aiplatBeads := filepath.Join(townRoot, "ai_platform", "mayor", "rig", ".beads")
	_ = os.MkdirAll(gastownBeads, 0755)
	_ = os.MkdirAll(aiplatBeads, 0755)

	// Create redirects from rig root to mayor/rig/.beads
	gastownRigBeads := filepath.Join(townRoot, "gastown", ".beads")
	aiplatRigBeads := filepath.Join(townRoot, "ai_platform", ".beads")
	_ = os.MkdirAll(gastownRigBeads, 0755)
	_ = os.MkdirAll(aiplatRigBeads, 0755)
	_ = os.WriteFile(filepath.Join(gastownRigBeads, "redirect"), []byte("mayor/rig/.beads\n"), 0644)
	_ = os.WriteFile(filepath.Join(aiplatRigBeads, "redirect"), []byte("mayor/rig/.beads\n"), 0644)

	// Create empty issues.jsonl files FIRST - this prevents bd from using our
	// routes.jsonl as its issues backup file
	_ = os.WriteFile(filepath.Join(townBeads, "issues.jsonl"), []byte{}, 0644)
	_ = os.WriteFile(filepath.Join(gastownBeads, "issues.jsonl"), []byte{}, 0644)
	_ = os.WriteFile(filepath.Join(aiplatBeads, "issues.jsonl"), []byte{}, 0644)

	// Write routes.jsonl to town beads
	routes := `{"prefix": "gt-", "path": "gastown/mayor/rig"}
{"prefix": "ap-", "path": "ai_platform/mayor/rig"}
{"prefix": "hq-", "path": "."}
`
	_ = os.WriteFile(filepath.Join(townBeads, "routes.jsonl"), []byte(routes), 0644)

	return &CrossRigEnv{
		TownRoot:   townRoot,
		GastownDir: filepath.Join(townRoot, "gastown"),
		AIPlat:     filepath.Join(townRoot, "ai_platform"),
	}
}

// =============================================================================
// Tests for NeedsRoutingWorkaround
// =============================================================================

func TestNeedsRoutingWorkaround_LocalCommands(t *testing.T) {
	// Local commands don't take bead IDs and work correctly from any directory
	localCommands := []string{
		"list", "create", "ready", "blocked", "sync", "config", "init", "version", "doctor",
	}

	for _, cmd := range localCommands {
		if beads.NeedsRoutingWorkaround(cmd) {
			t.Errorf("NeedsRoutingWorkaround(%q) = true, but local commands don't need workarounds", cmd)
		}
	}
}

func TestNeedsRoutingWorkaround_WorkingCommands(t *testing.T) {
	// These commands support prefix routing via bd's native mechanism
	// (mayor/town.json -> routes.jsonl). No workaround needed.
	// Tested with bd v0.46.0 on 2026-01-15
	// NOTE: bond, track, agent-state don't exist as bd commands
	workingCommands := []string{
		"show", "update", "close",
		"slot-show",
	}

	for _, cmd := range workingCommands {
		if beads.NeedsRoutingWorkaround(cmd) {
			t.Errorf("NeedsRoutingWorkaround(%q) = true, but bd routes this command natively", cmd)
		}
	}
}

func TestNeedsRoutingWorkaround_BrokenCommands(t *testing.T) {
	// These commands have been verified to NOT support prefix routing
	// when mayor/town.json is present for town root discovery.
	// Tested with bd v0.46.0 on 2026-01-15
	brokenCommands := []string{
		"reopen", "delete", "label-add", "label-remove", "comment", "dep-add", "dep-rm",
	}

	for _, cmd := range brokenCommands {
		if !beads.NeedsRoutingWorkaround(cmd) {
			t.Errorf("NeedsRoutingWorkaround(%q) = false, but command was verified broken", cmd)
		}
	}
}

func TestKnownBrokenRouting_DocumentedCommands(t *testing.T) {
	// Verify all documented broken commands are tracked
	// Note: show, update, close work via bd's native routing
	expected := []string{"reopen", "delete", "label-add", "label-remove", "comment", "dep-add", "dep-rm"}

	for _, cmd := range expected {
		if _, exists := beads.KnownBrokenRouting[cmd]; !exists {
			t.Errorf("Command %q is broken but not in KnownBrokenRouting", cmd)
		}
	}
}

// =============================================================================
// Cross-Rig Routing Verification Tests
//
// These tests assume routing works. When run against real bd, failures here
// indicate commands that need to be added to KnownBrokenRouting.
// =============================================================================

func TestCrossRig_Show(t *testing.T) {
	// Test: Can we show an ap- bead from gastown context?
	// Double should route by prefix, matching real bd behavior
	env := setupCrossRigEnv(t)

	// Create unified Double with prefix-based routing
	d := beads.NewDoubleWithPrefix("gt")
	d.ConfigureRoute(env.GastownDir, "gt")
	d.ConfigureRoute(env.AIPlat, "ap")

	// Create a bead in ai_platform context
	d.SetWorkDir(env.AIPlat)
	apBead, err := d.Create(beads.CreateOptions{Title: "AP Bead", Type: "task"})
	if err != nil {
		t.Fatalf("Create in ai_platform failed: %v", err)
	}

	// Try to show from gastown context - should route by prefix
	d.SetWorkDir(env.GastownDir)
	_, err = d.Show(apBead.ID)

	// Document expected vs actual:
	// - Expected (if bd routes correctly): err == nil, bead returned
	// - Actual (if bd doesn't route): err == ErrNotFound
	t.Logf("Cross-rig Show: apBead=%s, err=%v", apBead.ID, err)
	t.Logf("If err != nil, 'show' may need to be added to KnownBrokenRouting")
}

func TestCrossRig_Update(t *testing.T) {
	// Test: Can we update an ap- bead from gastown context?
	env := setupCrossRigEnv(t)

	d := beads.NewDoubleWithPrefix("gt")
	d.ConfigureRoute(env.GastownDir, "gt")
	d.ConfigureRoute(env.AIPlat, "ap")

	// Create a bead in ai_platform
	d.SetWorkDir(env.AIPlat)
	apBead, err := d.Create(beads.CreateOptions{Title: "AP Bead", Type: "task"})
	if err != nil {
		t.Fatalf("Create in ai_platform failed: %v", err)
	}

	// Try to update from gastown context - should route by prefix
	d.SetWorkDir(env.GastownDir)
	status := "in_progress"
	err = d.Update(apBead.ID, beads.UpdateOptions{Status: &status})

	t.Logf("Cross-rig Update: apBead=%s, err=%v", apBead.ID, err)
	t.Logf("If err != nil, 'update' may need to be added to KnownBrokenRouting")
}

func TestCrossRig_Close(t *testing.T) {
	// Test: Can we close an ap- bead from gastown context?
	env := setupCrossRigEnv(t)

	d := beads.NewDoubleWithPrefix("gt")
	d.ConfigureRoute(env.GastownDir, "gt")
	d.ConfigureRoute(env.AIPlat, "ap")

	d.SetWorkDir(env.AIPlat)
	apBead, _ := d.Create(beads.CreateOptions{Title: "AP Bead", Type: "task"})

	d.SetWorkDir(env.GastownDir)
	err := d.Close(apBead.ID)

	t.Logf("Cross-rig Close: apBead=%s, err=%v", apBead.ID, err)
	t.Logf("If err != nil, 'close' may need to be added to KnownBrokenRouting")
}

func TestCrossRig_Reopen(t *testing.T) {
	// Test: Can we reopen an ap- bead from gastown context?
	env := setupCrossRigEnv(t)

	d := beads.NewDoubleWithPrefix("gt")
	d.ConfigureRoute(env.GastownDir, "gt")
	d.ConfigureRoute(env.AIPlat, "ap")

	d.SetWorkDir(env.AIPlat)
	apBead, _ := d.Create(beads.CreateOptions{Title: "AP Bead", Type: "task"})
	_ = d.Close(apBead.ID)

	d.SetWorkDir(env.GastownDir)
	err := d.Reopen(apBead.ID)

	t.Logf("Cross-rig Reopen: apBead=%s, err=%v", apBead.ID, err)
	t.Logf("If err != nil, 'reopen' may need to be added to KnownBrokenRouting")
}

func TestCrossRig_Delete(t *testing.T) {
	// Test: Can we delete an ap- bead from gastown context?
	env := setupCrossRigEnv(t)

	d := beads.NewDoubleWithPrefix("gt")
	d.ConfigureRoute(env.GastownDir, "gt")
	d.ConfigureRoute(env.AIPlat, "ap")

	d.SetWorkDir(env.AIPlat)
	apBead, _ := d.Create(beads.CreateOptions{Title: "AP Bead", Type: "task"})

	d.SetWorkDir(env.GastownDir)
	err := d.Delete(apBead.ID)

	t.Logf("Cross-rig Delete: apBead=%s, err=%v", apBead.ID, err)
	t.Logf("If err != nil, 'delete' may need to be added to KnownBrokenRouting")
}

func TestCrossRig_AddDependency(t *testing.T) {
	// Test: Can we add dependency between beads from different rigs?
	env := setupCrossRigEnv(t)

	d := beads.NewDoubleWithPrefix("gt")
	d.ConfigureRoute(env.GastownDir, "gt")
	d.ConfigureRoute(env.AIPlat, "ap")

	d.SetWorkDir(env.AIPlat)
	apBead, _ := d.Create(beads.CreateOptions{Title: "AP Bead", Type: "task"})

	d.SetWorkDir(env.GastownDir)
	gtBead, _ := d.Create(beads.CreateOptions{Title: "GT Bead", Type: "task"})

	// Try to add dependency: gt bead depends on ap bead
	err := d.AddDependency(gtBead.ID, apBead.ID)

	t.Logf("Cross-rig AddDependency: %s depends on %s, err=%v", gtBead.ID, apBead.ID, err)
	t.Logf("If err != nil, 'depends' may need to be added to KnownBrokenRouting")
}

// =============================================================================
// FindTownRoot Verification
// =============================================================================

func TestFindTownRoot_FindsRoutesJsonl(t *testing.T) {
	env := setupCrossRigEnv(t)

	// From town root - should find itself
	townRoot, err := beads.FindTownRoot(env.TownRoot)
	if err != nil {
		t.Errorf("FindTownRoot from town root failed: %v", err)
	}
	if townRoot != env.TownRoot {
		t.Errorf("FindTownRoot = %q, want %q", townRoot, env.TownRoot)
	}

	// From rig directory - should find town root
	townRoot, err = beads.FindTownRoot(env.GastownDir)
	if err != nil {
		t.Errorf("FindTownRoot from rig failed: %v", err)
	}
	if townRoot != env.TownRoot {
		t.Errorf("FindTownRoot from rig = %q, want %q", townRoot, env.TownRoot)
	}

	// From deep inside rig - should find town root
	deepDir := filepath.Join(env.GastownDir, "mayor", "rig", ".beads")
	townRoot, err = beads.FindTownRoot(deepDir)
	if err != nil {
		t.Errorf("FindTownRoot from deep dir failed: %v", err)
	}
	if townRoot != env.TownRoot {
		t.Errorf("FindTownRoot from deep dir = %q, want %q", townRoot, env.TownRoot)
	}
}

func TestFindTownRoot_ErrorsWhenNotFound(t *testing.T) {
	// Create a temp dir without routes.jsonl
	tmpDir := t.TempDir()

	_, err := beads.FindTownRoot(tmpDir)
	if err == nil {
		t.Error("FindTownRoot should error when no routes.jsonl found")
	}
	if !errors.Is(err, beads.ErrTownRootNotFound) {
		t.Errorf("FindTownRoot error = %v, want ErrTownRootNotFound", err)
	}
}

// =============================================================================
// ResolveHookDir Verification
// =============================================================================

func TestResolveHookDir_ResolvesCorrectRig(t *testing.T) {
	// Verify ResolveHookDir correctly maps prefix to rig path
	env := setupCrossRigEnv(t)

	tests := []struct {
		beadID      string
		expectedDir string
	}{
		{"ap-123", filepath.Join(env.TownRoot, "ai_platform", "mayor", "rig")},
		{"gt-456", filepath.Join(env.TownRoot, "gastown", "mayor", "rig")},
		{"hq-789", filepath.Join(env.TownRoot, ".")},
	}

	for _, tc := range tests {
		workDir := beads.ResolveHookDir(env.TownRoot, tc.beadID, env.GastownDir)
		if workDir != tc.expectedDir {
			t.Errorf("ResolveHookDir(%q) = %q, want %q", tc.beadID, workDir, tc.expectedDir)
		}
	}
}

// =============================================================================
// Workaround Pattern Documentation
//
// When a command IS broken, here's how to use the workaround pattern:
// =============================================================================

func TestWorkaroundPattern_WhenNeeded(t *testing.T) {
	// This test documents the workaround pattern for when a command IS broken.
	// Use this pattern only after verifying the command fails cross-rig.
	env := setupCrossRigEnv(t)

	beadID := "ap-123" // Bead from ai_platform

	// Step 1: Resolve the correct working directory
	workDir := beads.ResolveHookDir(env.TownRoot, beadID, env.GastownDir)

	// Step 2: Verify it resolved to ai_platform (the ap- rig)
	expectedDir := filepath.Join(env.TownRoot, "ai_platform", "mayor", "rig")
	if workDir != expectedDir {
		t.Errorf("ResolveHookDir = %q, want %q", workDir, expectedDir)
	}

	// Step 3: Use workaround method (only if command is in KnownBrokenRouting)
	// impl := beads.New(env.GastownDir)
	// if beads.NeedsRoutingWorkaround("update") {
	//     err := impl.WorkaroundUpdate(env.TownRoot, beadID, opts)
	// } else {
	//     err := impl.Update(beadID, opts)
	// }

	t.Logf("Workaround pattern: beadID=%s resolved to workDir=%s", beadID, workDir)
}

// =============================================================================
// TODO: Full Permutation Coverage
// =============================================================================
//
// The current cross-rig tests only verify operations work when routing is
// correct. We need full permutation coverage to understand true bd behavior:
//
// For each operation (Show, Update, Close, etc.):
//   - Test from townRoot working directory
//   - Test from gastown working directory (with gastown beadsDir)
//   - Test from ai_platform working directory (with ai_platform beadsDir)
//   - Test from a subdirectory with redirect to shared beadsDir
//
// For each test, document:
//   - Does the operation succeed?
//   - Does it hit the correct database (based on bead ID prefix)?
//   - What error is returned if it fails?
//
// This will help us understand:
//   1. Which operations bd routes correctly natively
//   2. Which operations need our routedImpl() workaround
//   3. Whether BEADS_DIR, working directory, or prefix affects behavior
//
// See conformance_matrix_test.go for the current test matrix implementation
// that tests Double, Implementation, and TrueRawBdOps across same-rig and
// cross-rig contexts.

