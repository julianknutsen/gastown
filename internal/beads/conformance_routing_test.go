package beads_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/steveyegge/gastown/internal/beads"
)

// =============================================================================
// Routing/Context Conformance Tests
// Tests: Database isolation between town and rig, redirect following, BEADS_DIR
// =============================================================================

// RoutingTestEnv sets up a realistic town/rig directory structure for testing.
type RoutingTestEnv struct {
	TownRoot string
	RigRoot  string
	RigBeads string // <rig>/.beads or <rig>/mayor/rig/.beads
	CrewDir  string // <rig>/crew/<name>
}

func setupRoutingTestEnv(t *testing.T) *RoutingTestEnv {
	t.Helper()

	townRoot := t.TempDir()

	// Create directory structure:
	// <town>/
	//   mayor/
	//     rig/
	//       .beads/  <- canonical beads location
	//   <rig>/
	//     .beads/
	//       redirect -> mayor/rig/.beads
	//     crew/
	//       worker1/
	//         .beads/
	//           redirect -> ../../.beads

	rigName := "testrig"
	rigRoot := filepath.Join(townRoot, rigName)
	mayorRigBeads := filepath.Join(rigRoot, "mayor", "rig", ".beads")
	rigBeads := filepath.Join(rigRoot, ".beads")
	crewDir := filepath.Join(rigRoot, "crew", "worker1")
	crewBeads := filepath.Join(crewDir, ".beads")

	// Create directories
	_ = os.MkdirAll(mayorRigBeads, 0755)
	_ = os.MkdirAll(rigBeads, 0755)
	_ = os.MkdirAll(crewBeads, 0755)

	// Create redirect: rig/.beads -> mayor/rig/.beads
	_ = os.WriteFile(filepath.Join(rigBeads, "redirect"), []byte("mayor/rig/.beads\n"), 0644)

	// Create redirect: crew/worker1/.beads -> ../../.beads
	_ = os.WriteFile(filepath.Join(crewBeads, "redirect"), []byte("../../.beads\n"), 0644)

	return &RoutingTestEnv{
		TownRoot: townRoot,
		RigRoot:  rigRoot,
		RigBeads: mayorRigBeads, // Final destination after following redirects
		CrewDir:  crewDir,
	}
}

// --- ResolveBeadsDir Tests ---

func TestConformance_ResolveBeadsDir_NoRedirect(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	_ = os.MkdirAll(beadsDir, 0755)

	resolved := beads.ResolveBeadsDir(tmpDir)
	if resolved != beadsDir {
		t.Errorf("ResolveBeadsDir = %q, want %q", resolved, beadsDir)
	}
}

func TestConformance_ResolveBeadsDir_WithRedirect(t *testing.T) {
	env := setupRoutingTestEnv(t)

	// From rig root, should follow redirect to mayor/rig/.beads
	resolved := beads.ResolveBeadsDir(env.RigRoot)
	if resolved != env.RigBeads {
		t.Errorf("ResolveBeadsDir(rig) = %q, want %q", resolved, env.RigBeads)
	}
}

func TestConformance_ResolveBeadsDir_RedirectChain(t *testing.T) {
	env := setupRoutingTestEnv(t)

	// From crew dir, should follow chain: crew/.beads -> rig/.beads -> mayor/rig/.beads
	resolved := beads.ResolveBeadsDir(env.CrewDir)
	if resolved != env.RigBeads {
		t.Errorf("ResolveBeadsDir(crew) = %q, want %q", resolved, env.RigBeads)
	}
}

// --- Database Isolation Tests ---
// These tests verify that operations in different contexts access the correct database.

func TestConformance_Routing_TownAndRigIsolation(t *testing.T) {
	// Create directory structure for two separate beads locations
	tmpDir := t.TempDir()
	townWorkDir := filepath.Join(tmpDir, "town")
	rigWorkDir := filepath.Join(tmpDir, "rig")

	_ = os.MkdirAll(townWorkDir, 0755)
	_ = os.MkdirAll(rigWorkDir, 0755)

	// Create a unified Double with workDir-based routing
	d := beads.NewDoubleWithPrefix("hq")
	d.ConfigureRoute(townWorkDir, "hq")
	d.ConfigureRoute(rigWorkDir, "rig")

	// Create issue from town context
	d.SetWorkDir(townWorkDir)
	townIssue, err := d.Create(beads.CreateOptions{Title: "Town Issue", Type: "task"})
	if err != nil {
		t.Fatalf("Create from town failed: %v", err)
	}
	if len(townIssue.ID) < 3 || townIssue.ID[:3] != "hq-" {
		t.Errorf("Town issue ID = %q, want prefix 'hq-'", townIssue.ID)
	}

	// Create issue from rig context
	d.SetWorkDir(rigWorkDir)
	rigIssue, err := d.Create(beads.CreateOptions{Title: "Rig Issue", Type: "task"})
	if err != nil {
		t.Fatalf("Create from rig failed: %v", err)
	}
	if len(rigIssue.ID) < 4 || rigIssue.ID[:4] != "rig-" {
		t.Errorf("Rig issue ID = %q, want prefix 'rig-'", rigIssue.ID)
	}

	// List from town should only see town issues
	d.SetWorkDir(townWorkDir)
	townIssues, err := d.List(beads.ListOptions{Priority: -1})
	if err != nil {
		t.Fatalf("List from town failed: %v", err)
	}
	if len(townIssues) != 1 {
		t.Errorf("Town has %d issues, want 1", len(townIssues))
	}
	if len(townIssues) > 0 && townIssues[0].ID != townIssue.ID {
		t.Errorf("Town list returned wrong issue")
	}

	// List from rig should only see rig issues
	d.SetWorkDir(rigWorkDir)
	rigIssues, err := d.List(beads.ListOptions{Priority: -1})
	if err != nil {
		t.Fatalf("List from rig failed: %v", err)
	}
	if len(rigIssues) != 1 {
		t.Errorf("Rig has %d issues, want 1", len(rigIssues))
	}
	if len(rigIssues) > 0 && rigIssues[0].ID != rigIssue.ID {
		t.Errorf("Rig list returned wrong issue")
	}
}

func TestConformance_Routing_CrewSeesRigBeads(t *testing.T) {
	// Set up environment with redirects
	env := setupRoutingTestEnv(t)

	// Create a unified Double with both rig and crew routing to same database
	// This simulates crew's redirect to rig's database
	d := beads.NewDoubleWithPrefix("rig")
	d.ConfigureRoute(env.RigRoot, "rig")
	d.ConfigureRoute(env.CrewDir, "rig") // Crew shares rig's database via redirect

	// Create issue from rig root
	d.SetWorkDir(env.RigRoot)
	rigIssue, err := d.Create(beads.CreateOptions{Title: "Rig Issue", Type: "task"})
	if err != nil {
		t.Fatalf("Create from rig failed: %v", err)
	}

	// List from crew should see the rig issue (via shared database)
	d.SetWorkDir(env.CrewDir)
	crewIssues, err := d.List(beads.ListOptions{Priority: -1})
	if err != nil {
		t.Fatalf("List from crew failed: %v", err)
	}
	if len(crewIssues) != 1 {
		t.Errorf("Crew sees %d issues, want 1", len(crewIssues))
	}
	if len(crewIssues) > 0 && crewIssues[0].ID != rigIssue.ID {
		t.Errorf("Crew list should see rig issue")
	}

	// Create issue from crew worker
	crewIssue, err := d.Create(beads.CreateOptions{Title: "Crew Issue", Type: "task"})
	if err != nil {
		t.Fatalf("Create from crew failed: %v", err)
	}

	// List from rig should see both issues
	d.SetWorkDir(env.RigRoot)
	rigIssues, err := d.List(beads.ListOptions{Priority: -1})
	if err != nil {
		t.Fatalf("List from rig failed: %v", err)
	}
	if len(rigIssues) != 2 {
		t.Errorf("Rig sees %d issues, want 2", len(rigIssues))
	}

	// Both issues should be in the list
	ids := make(map[string]bool)
	for _, issue := range rigIssues {
		ids[issue.ID] = true
	}
	if !ids[rigIssue.ID] {
		t.Error("Rig list should include rig issue")
	}
	if !ids[crewIssue.ID] {
		t.Error("Rig list should include crew issue (via shared database)")
	}
}

func TestConformance_Routing_WorkDirOverride(t *testing.T) {
	env := setupRoutingTestEnv(t)

	// Create a unified Double that routes different workDirs to different databases
	d := beads.NewDoubleWithPrefix("override")
	d.ConfigureRoute(env.CrewDir, "override")  // Initial route to override
	d.ConfigureRoute(env.RigRoot, "rig")

	// Create issue from crew - should use override database
	d.SetWorkDir(env.CrewDir)
	issue, err := d.Create(beads.CreateOptions{Title: "Override Issue", Type: "task"})
	if err != nil {
		t.Fatalf("Create with override route failed: %v", err)
	}

	// Should use override prefix
	if len(issue.ID) < 9 || issue.ID[:9] != "override-" {
		t.Errorf("Issue ID = %q, want prefix 'override-'", issue.ID)
	}

	// Change crew's route to point to rig database (like changing BEADS_DIR)
	d.ConfigureRoute(env.CrewDir, "rig")
	d.SetWorkDir(env.CrewDir) // Re-apply to pick up new route

	// New issue should go to rig database
	rigIssue, err := d.Create(beads.CreateOptions{Title: "Rig Issue", Type: "task"})
	if err != nil {
		t.Fatalf("Create with new route failed: %v", err)
	}

	if len(rigIssue.ID) < 4 || rigIssue.ID[:4] != "rig-" {
		t.Errorf("Issue ID = %q, want prefix 'rig-'", rigIssue.ID)
	}
}

// --- Prefix Handling Tests ---

func TestConformance_Routing_PrefixFromConfig(t *testing.T) {
	tmpDir := t.TempDir()
	townWorkDir := filepath.Join(tmpDir, "town")
	rigWorkDir := filepath.Join(tmpDir, "rig")

	_ = os.MkdirAll(townWorkDir, 0755)
	_ = os.MkdirAll(rigWorkDir, 0755)

	// Create a unified Double with different prefixes per workDir
	d := beads.NewDoubleWithPrefix("hq")
	d.ConfigureRoute(townWorkDir, "hq")
	d.ConfigureRoute(rigWorkDir, "toast")

	// Issue created from town gets ID "hq-1"
	d.SetWorkDir(townWorkDir)
	townIssue, err := d.Create(beads.CreateOptions{Title: "Town Issue", Type: "task"})
	if err != nil {
		t.Fatalf("Create from town failed: %v", err)
	}
	if townIssue.ID != "hq-1" {
		t.Errorf("Town issue ID = %q, want 'hq-1'", townIssue.ID)
	}

	// Issue created from rig gets ID "toast-1"
	d.SetWorkDir(rigWorkDir)
	rigIssue, err := d.Create(beads.CreateOptions{Title: "Rig Issue", Type: "task"})
	if err != nil {
		t.Fatalf("Create from rig failed: %v", err)
	}
	if rigIssue.ID != "toast-1" {
		t.Errorf("Rig issue ID = %q, want 'toast-1'", rigIssue.ID)
	}
}

// --- IsBeadsRepo Tests ---

func TestConformance_IsBeadsRepo_NotInitialized(t *testing.T) {
	tmpDir := t.TempDir()

	// No .beads directory
	impl := beads.ForRig(tmpDir)
	if impl.IsBeadsRepo() {
		t.Error("IsBeadsRepo should return false for uninitialized directory")
	}
}

func TestConformance_IsBeadsRepo_Initialized(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	_ = os.MkdirAll(beadsDir, 0755)

	impl := beads.ForRig(tmpDir)
	if !impl.IsBeadsRepo() {
		t.Error("IsBeadsRepo should return true for initialized directory")
	}
}

func TestConformance_IsBeadsRepo_WithRedirect(t *testing.T) {
	env := setupRoutingTestEnv(t)

	// From crew dir (which has redirect), should still be considered a beads repo
	// because the redirect points to a valid beads directory
	impl := beads.ForRig(env.CrewDir)
	if !impl.IsBeadsRepo() {
		t.Error("IsBeadsRepo should return true for directory with valid redirect")
	}
}

// --- Double Routing Support Tests ---
// These tests verify Double-specific routing behavior

func TestDouble_Routing_SetActive(t *testing.T) {
	double := beads.NewDouble()

	// Default is active
	if !double.IsBeadsRepo() {
		t.Error("Double should be active by default")
	}

	// Can be deactivated
	double.SetActive(false)
	if double.IsBeadsRepo() {
		t.Error("Double should not be active after SetActive(false)")
	}

	// Operations should fail when not active
	_, err := double.List(beads.ListOptions{Priority: -1})
	if err == nil {
		t.Error("List should fail when Double is not active")
	}
}

func TestDouble_Routing_Prefix(t *testing.T) {
	// Verify Double respects prefix setting
	double := beads.NewDoubleWithPrefix("custom")

	issue, err := double.Create(beads.CreateOptions{Title: "Test", Type: "task"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// ID should use custom prefix
	if len(issue.ID) < 7 || issue.ID[:7] != "custom-" {
		t.Errorf("Issue ID = %q, want prefix 'custom-'", issue.ID)
	}
}

// --- Double WorkDir Routing Tests ---

func TestDouble_WorkDirRouting(t *testing.T) {
	tmpDir := t.TempDir()
	workDir1 := filepath.Join(tmpDir, "rig1")
	workDir2 := filepath.Join(tmpDir, "rig2")

	_ = os.MkdirAll(workDir1, 0755)
	_ = os.MkdirAll(workDir2, 0755)

	// Create Double with two databases via workDir routing
	d := beads.NewDoubleWithPrefix("rig1")
	d.ConfigureRoute(workDir1, "rig1")
	d.ConfigureRoute(workDir2, "rig2")

	// Create issue from workDir1
	d.SetWorkDir(workDir1)
	issue1, err := d.Create(beads.CreateOptions{Title: "Issue 1", Type: "task"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if issue1.ID != "rig1-1" {
		t.Errorf("Issue ID = %q, want 'rig1-1'", issue1.ID)
	}

	// Create issue from workDir2
	d.SetWorkDir(workDir2)
	issue2, err := d.Create(beads.CreateOptions{Title: "Issue 2", Type: "task"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if issue2.ID != "rig2-1" {
		t.Errorf("Issue ID = %q, want 'rig2-1'", issue2.ID)
	}

	// List from workDir1 should only see rig1 issues
	d.SetWorkDir(workDir1)
	issues1, err := d.List(beads.ListOptions{Priority: -1})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(issues1) != 1 || issues1[0].ID != "rig1-1" {
		t.Error("List from workDir1 should only see rig1 issue")
	}

	// List from workDir2 should only see rig2 issues
	d.SetWorkDir(workDir2)
	issues2, err := d.List(beads.ListOptions{Priority: -1})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(issues2) != 1 || issues2[0].ID != "rig2-1" {
		t.Error("List from workDir2 should only see rig2 issue")
	}
}

func TestDouble_ImplementsBeadsOps(t *testing.T) {
	d := beads.NewDouble()

	// Test that Double works as BeadsOps
	var ops beads.BeadsOps = d

	issue, err := ops.Create(beads.CreateOptions{Title: "Test", Type: "task"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	retrieved, err := ops.Show(issue.ID)
	if err != nil {
		t.Fatalf("Show failed: %v", err)
	}
	if retrieved.Title != "Test" {
		t.Errorf("Title = %q, want %q", retrieved.Title, "Test")
	}
}
