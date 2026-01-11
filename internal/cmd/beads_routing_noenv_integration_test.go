//go:build integration

// Package cmd contains integration tests for the beads.DB interface.
//
// These tests verify that the beads interface correctly:
// - Discovers databases via cwd (no BEADS_DIR needed)
// - Routes prefixed IDs via routes.jsonl
// - Follows redirect files for worktrees
//
// Run with: go test -tags=integration ./internal/cmd -run TestBeadsInterface -v
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/steveyegge/gastown/internal/beads"
)

// TestBeadsInterface is a comprehensive test suite that verifies the beads.DB
// interface correctly handles:
// - Database discovery via cwd (no BEADS_DIR)
// - Prefix-based routing via routes.jsonl
// - Redirect following for worktrees
// - Init and GetConfig operations
func TestBeadsInterface(t *testing.T) {
	// Skip if bd is not available
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd not installed, skipping interface test")
	}

	// Build gt binary
	gtBinary := buildGT(t)

	// Create town with both tracked and untracked rigs in a single setup
	townRoot := setupBeadsInterfaceTestTown(t, gtBinary)

	// Run all subtests against the same town
	t.Run("TownAgent", func(t *testing.T) {
		testTownAgentRouting(t, townRoot)
	})
	t.Run("UntrackedRig", func(t *testing.T) {
		testUntrackedRigRouting(t, townRoot)
	})
	t.Run("TrackedRig", func(t *testing.T) {
		testTrackedRigRouting(t, townRoot)
	})
	t.Run("CrossRigRouting", func(t *testing.T) {
		testCrossRigRouting(t, townRoot)
	})
	t.Run("WorktreeRedirects", func(t *testing.T) {
		testWorktreeRedirects(t, townRoot)
	})
	t.Run("DefaultPrefixBehavior", func(t *testing.T) {
		testDefaultPrefixBehavior(t, townRoot)
	})
	t.Run("ListIsolation", func(t *testing.T) {
		testListIsolation(t, townRoot)
	})
	t.Run("ExistingPrefixDetection", func(t *testing.T) {
		testExistingPrefixDetection(t, gtBinary)
	})
}

// setupBeadsInterfaceTestTown creates a complete Gas Town using gt install and gt rig add
// to ensure the test matches production behavior exactly.
//
// Creates:
// - Town-level beads (hq-)
// - An untracked rig (unrig) - source repo has no .beads/
// - A tracked rig (trrig) - source repo has .beads/ tracked in git
func setupBeadsInterfaceTestTown(t *testing.T, gtBinary string) string {
	t.Helper()

	tmpDir := t.TempDir()
	townRoot := filepath.Join(tmpDir, "test-town")
	reposDir := filepath.Join(tmpDir, "repos")

	// Create source repos directory
	if err := os.MkdirAll(reposDir, 0755); err != nil {
		t.Fatalf("mkdir repos: %v", err)
	}

	// === Create untracked source repo (no .beads/) ===
	unrigRepo := filepath.Join(reposDir, "unrig-repo")
	createBareGitRepo(t, unrigRepo)

	// === Create tracked source repo (has .beads/) ===
	trrigRepo := filepath.Join(reposDir, "trrig-repo")
	createTrackedBeadsRepo(t, trrigRepo, "tr")

	// === Run gt install ===
	cmd := exec.Command(gtBinary, "install", townRoot, "--name", "test-town")
	cmd.Env = filterOutBeadsDir(os.Environ())
	cmd.Env = append(cmd.Env, "HOME="+tmpDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gt install failed: %v\nOutput: %s", err, output)
	}

	// === Add untracked rig ===
	cmd = exec.Command(gtBinary, "rig", "add", "unrig", unrigRepo, "--prefix", "ur")
	cmd.Dir = townRoot
	cmd.Env = filterOutBeadsDir(os.Environ())
	cmd.Env = append(cmd.Env, "HOME="+tmpDir)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gt rig add unrig failed: %v\nOutput: %s", err, output)
	}

	// === Add tracked rig ===
	cmd = exec.Command(gtBinary, "rig", "add", "trrig", trrigRepo, "--prefix", "tr")
	cmd.Dir = townRoot
	cmd.Env = filterOutBeadsDir(os.Environ())
	cmd.Env = append(cmd.Env, "HOME="+tmpDir)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gt rig add trrig failed: %v\nOutput: %s", err, output)
	}

	// Debug: show what was created
	t.Logf("Town routes.jsonl:")
	if content, err := os.ReadFile(filepath.Join(townRoot, ".beads", "routes.jsonl")); err == nil {
		t.Logf("%s", content)
	}

	// Check if unrig has rig-level routes.jsonl (it shouldn't after our fix)
	unrigRoutes := filepath.Join(townRoot, "unrig", ".beads", "routes.jsonl")
	if _, err := os.Stat(unrigRoutes); err == nil {
		content, _ := os.ReadFile(unrigRoutes)
		t.Logf("unrig has rig-level routes.jsonl (unexpected after fix): %s", content)
	} else {
		t.Log("unrig has no rig-level routes.jsonl (expected after fix)")
	}

	// === Create polecat worktree for unrig ===
	// Syntax: gt polecat add <rig> <name>
	cmd = exec.Command(gtBinary, "polecat", "add", "unrig", "test")
	cmd.Dir = townRoot
	cmd.Env = filterOutBeadsDir(os.Environ())
	cmd.Env = append(cmd.Env, "HOME="+tmpDir)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Logf("gt polecat add failed: %v\nOutput: %s", err, output)
		// Create manually as fallback
		polecatDir := filepath.Join(townRoot, "unrig", "polecats", "test")
		os.MkdirAll(polecatDir, 0755)
		polecatBeads := filepath.Join(polecatDir, ".beads")
		os.MkdirAll(polecatBeads, 0755)
		os.WriteFile(filepath.Join(polecatBeads, "redirect"), []byte("../../.beads\n"), 0644)
	}

	// === Create crew worktree for trrig ===
	// Syntax: gt crew add <rig> <name>
	cmd = exec.Command(gtBinary, "crew", "add", "trrig", "max")
	cmd.Dir = townRoot
	cmd.Env = filterOutBeadsDir(os.Environ())
	cmd.Env = append(cmd.Env, "HOME="+tmpDir)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Logf("gt crew add failed: %v\nOutput: %s", err, output)
		// Create manually as fallback - for tracked rig, redirect to mayor/rig/.beads
		crewDir := filepath.Join(townRoot, "trrig", "crew", "max")
		os.MkdirAll(crewDir, 0755)
		crewBeads := filepath.Join(crewDir, ".beads")
		os.MkdirAll(crewBeads, 0755)
		os.WriteFile(filepath.Join(crewBeads, "redirect"), []byte("../../mayor/rig/.beads\n"), 0644)
	}

	return townRoot
}

// createBareGitRepo creates a minimal git repo without .beads/
func createBareGitRepo(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}

	// git init
	cmd := exec.Command("git", "init")
	cmd.Dir = path
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\nOutput: %s", err, output)
	}

	// Create a file and commit
	if err := os.WriteFile(filepath.Join(path, "README.md"), []byte("# Test Repo\n"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = path
	cmd.CombinedOutput()

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = path
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@test.com")
	cmd.CombinedOutput()
}

// createTrackedBeadsRepo creates a git repo with .beads/ tracked
func createTrackedBeadsRepo(t *testing.T, path, prefix string) {
	t.Helper()

	// First create basic repo
	createBareGitRepo(t, path)

	// Initialize beads
	beadsDir := filepath.Join(path, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}

	// Run bd init
	cmd := exec.Command("bd", "--no-daemon", "init", "--prefix", prefix)
	cmd.Dir = path
	cmd.Env = filterOutBeadsDir(os.Environ())
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bd init failed: %v\nOutput: %s", err, output)
	}

	// Add .beads to git (simulating tracked beads)
	cmd = exec.Command("git", "add", ".beads")
	cmd.Dir = path
	cmd.CombinedOutput()

	cmd = exec.Command("git", "commit", "-m", "Add beads")
	cmd.Dir = path
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@test.com")
	cmd.CombinedOutput()
}

// createTrackedBeadsRepoWithIssues creates a git repo with .beads/ tracked that contains existing issues.
// This simulates a clone of a repo that has tracked beads with issues exported to issues.jsonl.
// The beads.db is NOT included (gitignored), so prefix must be detected from issues.jsonl.
func createTrackedBeadsRepoWithIssues(t *testing.T, path, prefix string, numIssues int) {
	t.Helper()

	// First create basic repo
	createBareGitRepo(t, path)

	// Initialize beads
	beadsDir := filepath.Join(path, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}

	// Run bd init
	cmd := exec.Command("bd", "--no-daemon", "init", "--prefix", prefix)
	cmd.Dir = path
	cmd.Env = filterOutBeadsDir(os.Environ())
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bd init failed: %v\nOutput: %s", err, output)
	}

	// Create issues
	for i := 1; i <= numIssues; i++ {
		cmd = exec.Command("bd", "--no-daemon", "-q", "create",
			"--type", "task", "--title", fmt.Sprintf("Test issue %d", i))
		cmd.Dir = path
		cmd.Env = filterOutBeadsDir(os.Environ())
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("bd create issue %d failed: %v\nOutput: %s", i, err, output)
		}
	}

	// Add .beads to git (simulating tracked beads)
	cmd = exec.Command("git", "add", ".beads")
	cmd.Dir = path
	cmd.CombinedOutput()

	cmd = exec.Command("git", "commit", "-m", "Add beads with issues")
	cmd.Dir = path
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@test.com")
	cmd.CombinedOutput()

	// Remove beads.db to simulate what a clone would look like
	// (beads.db is gitignored, so cloned repos don't have it)
	dbPath := filepath.Join(beadsDir, "beads.db")
	os.Remove(dbPath)
}

// testTownAgentRouting tests bd operations from town-level agent directories
func testTownAgentRouting(t *testing.T, townRoot string) {
	// Debug: show routes.jsonl content
	routesFile := filepath.Join(townRoot, ".beads", "routes.jsonl")
	routesContent, err := os.ReadFile(routesFile)
	if err != nil {
		t.Logf("Could not read routes.jsonl: %v", err)
	} else {
		t.Logf("Town routes.jsonl content:\n%s", routesContent)
	}

	// Test from deacon directory
	deaconDir := filepath.Join(townRoot, "deacon")
	if err := os.MkdirAll(deaconDir, 0755); err != nil {
		t.Fatalf("mkdir deacon: %v", err)
	}

	t.Run("CreateHqBead", func(t *testing.T) {
		// Create an hq- bead from deacon directory WITHOUT BEADS_DIR
		issueID := createBead(t, deaconDir, "hq-", "test-from-deacon")
		if !strings.HasPrefix(issueID, "hq-") {
			t.Errorf("expected hq- prefix, got %s", issueID)
		}

		// Verify we can read it back
		showBead(t, deaconDir, issueID)
	})

	t.Run("ReadRigBeadFromTown", func(t *testing.T) {
		// First create a bead in unrig
		unrigPath := filepath.Join(townRoot, "unrig")
		issueID := createBead(t, unrigPath, "ur-", "test-for-cross-read")

		// Now read it from deacon (should use routes.jsonl)
		showBead(t, deaconDir, issueID)
	})
}

// testUntrackedRigRouting tests bd operations from untracked rig directories
func testUntrackedRigRouting(t *testing.T, townRoot string) {
	unrigPath := filepath.Join(townRoot, "unrig")

	t.Run("CreateOwnPrefixBead", func(t *testing.T) {
		// Create a ur- bead from rig root WITHOUT BEADS_DIR
		issueID := createBead(t, unrigPath, "ur-", "test-own-prefix")
		if !strings.HasPrefix(issueID, "ur-") {
			t.Errorf("expected ur- prefix, got %s", issueID)
		}

		// Verify we can read it back
		showBead(t, unrigPath, issueID)
	})

	t.Run("CreateHqBeadFromRig", func(t *testing.T) {
		// Create an hq- bead from rig directory
		// This should route to town beads via routes.jsonl walk-up
		issueID := createBead(t, unrigPath, "hq-", "test-hq-from-rig")
		if !strings.HasPrefix(issueID, "hq-") {
			t.Errorf("expected hq- prefix, got %s", issueID)
		}

		// Verify we can read it back (still from rig dir)
		showBead(t, unrigPath, issueID)
	})

	t.Run("FromPolecatWorktree", func(t *testing.T) {
		polecatDir := filepath.Join(unrigPath, "polecats", "test")

		// Create bead from polecat (should follow redirect to rig beads)
		issueID := createBead(t, polecatDir, "ur-", "test-from-polecat")
		if !strings.HasPrefix(issueID, "ur-") {
			t.Errorf("expected ur- prefix from polecat, got %s", issueID)
		}

		// Verify we can read it back
		showBead(t, polecatDir, issueID)

		// Also verify we can read it from rig root (same database)
		showBead(t, unrigPath, issueID)
	})
}

// testTrackedRigRouting tests bd operations from tracked rig directories
func testTrackedRigRouting(t *testing.T, townRoot string) {
	trrigPath := filepath.Join(townRoot, "trrig")

	t.Run("CreateOwnPrefixBead", func(t *testing.T) {
		// GitHub Issue #72: Tracked repo should use configured prefix, not directory name
		// https://github.com/steveyegge/gastown/issues/72
		//
		// Currently FAILS: bd uses directory name "rig" as prefix instead of "tr"
		// because the cloned .beads/ has no database (gitignored) and bd init
		// wasn't run with the correct prefix.

		// Create a tr- bead from rig root
		// Should follow redirect to mayor/rig/.beads
		issueID := createBead(t, trrigPath, "tr-", "test-tracked-prefix")
		if !strings.HasPrefix(issueID, "tr-") {
			t.Errorf("expected tr- prefix, got %s", issueID)
		}

		// Verify we can read it back
		showBead(t, trrigPath, issueID)
	})

	t.Run("CreateHqBeadFromTrackedRig", func(t *testing.T) {
		// Create an hq- bead from tracked rig directory
		// This works because hq- routes to town beads, not the tracked rig beads
		issueID := createBead(t, trrigPath, "hq-", "test-hq-from-tracked")
		if !strings.HasPrefix(issueID, "hq-") {
			t.Errorf("expected hq- prefix, got %s", issueID)
		}

		showBead(t, trrigPath, issueID)
	})

	t.Run("FromMayorRigDir", func(t *testing.T) {
		// GitHub Issue #72: Tracked repo should use configured prefix
		// https://github.com/steveyegge/gastown/issues/72

		mayorRigDir := filepath.Join(trrigPath, "mayor", "rig")

		// Create bead from mayor/rig (where the tracked .beads actually lives)
		issueID := createBead(t, mayorRigDir, "tr-", "test-from-mayor-rig")
		if !strings.HasPrefix(issueID, "tr-") {
			t.Errorf("expected tr- prefix from mayor/rig, got %s", issueID)
		}

		showBead(t, mayorRigDir, issueID)
	})

	t.Run("FromCrewWorktree", func(t *testing.T) {
		// GitHub Issue #72: Tracked repo should use configured prefix
		// https://github.com/steveyegge/gastown/issues/72

		crewDir := filepath.Join(trrigPath, "crew", "max")

		// Create bead from crew (should follow redirect to mayor/rig/.beads)
		issueID := createBead(t, crewDir, "tr-", "test-from-crew")
		if !strings.HasPrefix(issueID, "tr-") {
			t.Errorf("expected tr- prefix from crew, got %s", issueID)
		}

		showBead(t, crewDir, issueID)

		// Verify same bead readable from rig root
		showBead(t, trrigPath, issueID)
	})
}

// testCrossRigRouting tests reading beads across rigs via routes.jsonl
func testCrossRigRouting(t *testing.T, townRoot string) {
	unrigPath := filepath.Join(townRoot, "unrig")
	trrigPath := filepath.Join(townRoot, "trrig")

	// Create bead in unrig
	urIssue := createBead(t, unrigPath, "ur-", "cross-rig-test-1")

	// Create bead in trrig
	trIssue := createBead(t, trrigPath, "tr-", "cross-rig-test-2")

	t.Run("ReadUnrigFromTrrig", func(t *testing.T) {
		// Read ur- bead from trrig directory
		showBead(t, trrigPath, urIssue)
	})

	t.Run("ReadTrrigFromUnrig", func(t *testing.T) {
		// Read tr- bead from unrig directory
		showBead(t, unrigPath, trIssue)
	})

	t.Run("ReadBothFromTown", func(t *testing.T) {
		deaconDir := filepath.Join(townRoot, "deacon")
		showBead(t, deaconDir, urIssue)
		showBead(t, deaconDir, trIssue)
	})
}

// testWorktreeRedirects verifies that redirects are followed correctly
func testWorktreeRedirects(t *testing.T, townRoot string) {
	t.Run("PolecatSharesRigDb", func(t *testing.T) {
		unrigPath := filepath.Join(townRoot, "unrig")
		polecatDir := filepath.Join(unrigPath, "polecats", "test")

		// Create from rig root
		issueID := createBead(t, unrigPath, "ur-", "shared-test-1")

		// Read from polecat (should see same bead via redirect)
		showBead(t, polecatDir, issueID)

		// Update from polecat
		updateBead(t, polecatDir, issueID, "Updated from polecat")

		// Verify update visible from rig root
		issue := showBead(t, unrigPath, issueID)
		if !strings.Contains(issue.Description, "Updated from polecat") {
			t.Errorf("update not visible from rig root: %s", issue.Description)
		}
	})

	t.Run("CrewSharesTrackedDb", func(t *testing.T) {
		trrigPath := filepath.Join(townRoot, "trrig")
		crewDir := filepath.Join(trrigPath, "crew", "max")

		// Create from rig root (follows redirect to mayor/rig/.beads)
		issueID := createBead(t, trrigPath, "tr-", "shared-tracked-test")

		// Read from crew (should see same bead via redirect)
		showBead(t, crewDir, issueID)

		// Update from crew
		updateBead(t, crewDir, issueID, "Updated from crew")

		// Verify update visible from rig root
		issue := showBead(t, trrigPath, issueID)
		if !strings.Contains(issue.Description, "Updated from crew") {
			t.Errorf("update not visible from rig root: %s", issue.Description)
		}
	})
}

// === Helper functions ===

// filterOutBeadsDir removes BEADS_DIR from environment
func filterOutBeadsDir(environ []string) []string {
	result := make([]string, 0, len(environ))
	for _, e := range environ {
		if !strings.HasPrefix(e, "BEADS_DIR=") {
			result = append(result, e)
		}
	}
	return result
}

// initBeadsViaInterface initializes a beads database using the interface
func initBeadsViaInterface(t *testing.T, workDir, prefix string) {
	t.Helper()
	db := beads.New(workDir)
	if err := db.Init(prefix); err != nil {
		t.Fatalf("beads.Init in %s failed: %v", workDir, err)
	}
}

// getConfigViaInterface reads a config value using the interface
func getConfigViaInterface(t *testing.T, workDir, key string) string {
	t.Helper()
	db := beads.New(workDir)
	value, err := db.GetConfig(key)
	if err != nil {
		t.Fatalf("beads.GetConfig(%s) in %s failed: %v", key, workDir, err)
	}
	return value
}

// addRoute adds a route to town-level routes.jsonl
func addRoute(t *testing.T, townRoot, prefix, path string) {
	t.Helper()
	addRouteToDir(t, townRoot, prefix, path)
}

// addRouteToDir adds a route to a specific directory's routes.jsonl
func addRouteToDir(t *testing.T, dir, prefix, path string) {
	t.Helper()
	routesFile := filepath.Join(dir, ".beads", "routes.jsonl")

	// Read existing routes
	existing, _ := os.ReadFile(routesFile)

	// Append new route
	route := map[string]string{"prefix": prefix, "path": path}
	routeJSON, _ := json.Marshal(route)

	content := string(existing)
	if !strings.HasSuffix(content, "\n") && content != "" {
		content += "\n"
	}
	content += string(routeJSON) + "\n"

	if err := os.WriteFile(routesFile, []byte(content), 0644); err != nil {
		t.Fatalf("write route failed: %v", err)
	}
}

// createBead creates a bead using the interface and returns its ID
func createBead(t *testing.T, workDir, prefix, title string) string {
	t.Helper()

	db := beads.New(workDir)
	issue, err := db.CreateWithPrefix(strings.TrimSuffix(prefix, "-"), beads.CreateOptions{
		Title: title,
		Type:  "task",
	})
	if err != nil {
		t.Fatalf("beads.CreateWithPrefix in %s failed: %v", workDir, err)
	}

	return issue.ID
}

// showBead reads a bead using the interface and returns it
func showBead(t *testing.T, workDir, issueID string) *beads.Issue {
	t.Helper()

	db := beads.New(workDir)
	issue, err := db.Show(issueID)
	if err != nil {
		t.Fatalf("beads.Show(%s) from %s failed: %v", issueID, workDir, err)
	}

	return issue
}

// updateBead updates a bead using the interface
func updateBead(t *testing.T, workDir, issueID, notes string) {
	t.Helper()

	db := beads.New(workDir)
	desc := notes
	err := db.Update(issueID, beads.UpdateOptions{
		Description: &desc,
	})
	if err != nil {
		t.Fatalf("beads.Update(%s) from %s failed: %v", issueID, workDir, err)
	}
}

// createBeadDefaultPrefix creates a bead WITHOUT explicit prefix.
// This tests that the interface uses the database's configured prefix.
// Returns the created bead ID.
func createBeadDefaultPrefix(t *testing.T, workDir, title string) string {
	t.Helper()

	db := beads.New(workDir)
	// Use Create (not CreateWithPrefix) - should use database's configured prefix
	issue, err := db.Create(beads.CreateOptions{
		Title: title,
		Type:  "task",
	})
	if err != nil {
		t.Fatalf("beads.Create in %s failed: %v", workDir, err)
	}

	return issue.ID
}

// listBeads lists beads from a directory using the interface.
// Returns slice of bead IDs.
func listBeads(t *testing.T, workDir string) []string {
	t.Helper()

	db := beads.New(workDir)
	issues, err := db.List(beads.ListOptions{})
	if err != nil {
		// Empty list is not an error
		return nil
	}

	var ids []string
	for _, issue := range issues {
		ids = append(ids, issue.ID)
	}
	return ids
}

// testDefaultPrefixBehavior tests that bd create without --prefix uses the
// database's configured prefix. This matches production behavior where agents
// run `bd create --type task --title "..."` without specifying prefix.
func testDefaultPrefixBehavior(t *testing.T, townRoot string) {
	t.Run("CreateFromTownUsesHqPrefix", func(t *testing.T) {
		// Create from town root - should use hq- prefix
		deaconDir := filepath.Join(townRoot, "deacon")
		issueID := createBeadDefaultPrefix(t, deaconDir, "test-town-default-prefix")

		if !strings.HasPrefix(issueID, "hq-") {
			t.Errorf("expected hq- prefix from town, got %s", issueID)
		}

		// Verify it's in town beads
		showBead(t, deaconDir, issueID)
	})

	t.Run("CreateFromRigUsesRigPrefix", func(t *testing.T) {
		// Create from untracked rig - should use ur- prefix
		unrigPath := filepath.Join(townRoot, "unrig")
		issueID := createBeadDefaultPrefix(t, unrigPath, "test-rig-default-prefix")

		if !strings.HasPrefix(issueID, "ur-") {
			t.Errorf("expected ur- prefix from unrig, got %s", issueID)
		}

		// Verify it's in rig beads (readable from rig)
		showBead(t, unrigPath, issueID)
	})

	t.Run("CreateFromWorktreeUsesRigPrefix", func(t *testing.T) {
		// Create from polecat worktree - should follow redirect to rig beads
		polecatDir := filepath.Join(townRoot, "unrig", "polecats", "test")
		issueID := createBeadDefaultPrefix(t, polecatDir, "test-worktree-default-prefix")

		if !strings.HasPrefix(issueID, "ur-") {
			t.Errorf("expected ur- prefix from worktree, got %s", issueID)
		}

		// Verify it's in rig beads (readable from rig root too)
		unrigPath := filepath.Join(townRoot, "unrig")
		showBead(t, unrigPath, issueID)
	})

	t.Run("CreateFromTrackedRigUsesConfiguredPrefix", func(t *testing.T) {
		// GitHub Issue #72: Tracked rig should use configured prefix
		// https://github.com/steveyegge/gastown/issues/72
		//
		// Currently may FAIL: bd might use directory name as prefix instead
		// of the configured prefix if database was recreated incorrectly.

		trrigPath := filepath.Join(townRoot, "trrig")
		issueID := createBeadDefaultPrefix(t, trrigPath, "test-tracked-default-prefix")

		if !strings.HasPrefix(issueID, "tr-") {
			t.Errorf("expected tr- prefix from tracked rig, got %s (see issue #72)", issueID)
		}
	})
}

// testListIsolation verifies that bd list only returns beads from the local
// database, not from other databases. This is important for agents that expect
// to see only their own work.
func testListIsolation(t *testing.T, townRoot string) {
	// Create unique beads in each location
	deaconDir := filepath.Join(townRoot, "deacon")
	unrigPath := filepath.Join(townRoot, "unrig")
	trrigPath := filepath.Join(townRoot, "trrig")

	// Create beads with distinctive titles
	townBead := createBead(t, deaconDir, "hq-", "isolation-test-town")
	unrigBead := createBead(t, unrigPath, "ur-", "isolation-test-unrig")
	trrigBead := createBead(t, trrigPath, "tr-", "isolation-test-trrig")

	t.Run("ListFromTownOnlyShowsTownBeads", func(t *testing.T) {
		ids := listBeads(t, deaconDir)

		// Should contain town bead
		if !containsID(ids, townBead) {
			t.Errorf("town list should contain %s, got %v", townBead, ids)
		}

		// Should NOT contain rig beads
		if containsID(ids, unrigBead) {
			t.Errorf("town list should NOT contain rig bead %s", unrigBead)
		}
		if containsID(ids, trrigBead) {
			t.Errorf("town list should NOT contain tracked rig bead %s", trrigBead)
		}
	})

	t.Run("ListFromRigOnlyShowsRigBeads", func(t *testing.T) {
		ids := listBeads(t, unrigPath)

		// Should contain rig bead
		if !containsID(ids, unrigBead) {
			t.Errorf("rig list should contain %s, got %v", unrigBead, ids)
		}

		// Should NOT contain town or other rig beads
		if containsID(ids, townBead) {
			t.Errorf("rig list should NOT contain town bead %s", townBead)
		}
		if containsID(ids, trrigBead) {
			t.Errorf("rig list should NOT contain other rig bead %s", trrigBead)
		}
	})

	t.Run("ListFromWorktreeShowsRigBeads", func(t *testing.T) {
		// Worktree should see rig beads via redirect
		polecatDir := filepath.Join(unrigPath, "polecats", "test")
		ids := listBeads(t, polecatDir)

		// Should contain rig bead (via redirect)
		if !containsID(ids, unrigBead) {
			t.Errorf("worktree list should contain rig bead %s, got %v", unrigBead, ids)
		}

		// Should NOT contain town beads
		if containsID(ids, townBead) {
			t.Errorf("worktree list should NOT contain town bead %s", townBead)
		}
	})
}

// containsID checks if a slice contains the given ID
func containsID(ids []string, target string) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

// testExistingPrefixDetection tests that gt rig add detects existing beads prefix.
// This is a regression test for https://github.com/steveyegge/gastown/issues/72
func testExistingPrefixDetection(t *testing.T, gtBinary string) {
	tmpDir := t.TempDir()

	t.Run("TrackedRepoWithExistingPrefix", func(t *testing.T) {
		// GitHub Issue #72: gt rig add should detect existing prefix from tracked beads
		// https://github.com/steveyegge/gastown/issues/72
		//
		// This tests that when a tracked beads repo has existing issues in issues.jsonl,
		// gt rig add can detect the prefix from those issues WITHOUT --prefix flag.

		townRoot := filepath.Join(tmpDir, "town-prefix-test")
		reposDir := filepath.Join(tmpDir, "repos")
		os.MkdirAll(reposDir, 0755)

		// Create a repo with existing beads prefix "existing-prefix" AND issues
		// This creates issues.jsonl with issues like "existing-prefix-1", etc.
		existingRepo := filepath.Join(reposDir, "existing-repo")
		createTrackedBeadsRepoWithIssues(t, existingRepo, "existing-prefix", 3)

		// Install town
		cmd := exec.Command(gtBinary, "install", townRoot, "--name", "prefix-test")
		cmd.Env = filterOutBeadsDir(os.Environ())
		cmd.Env = append(cmd.Env, "HOME="+tmpDir)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("gt install failed: %v\nOutput: %s", err, output)
		}

		// Add rig WITHOUT specifying --prefix - should detect "existing-prefix" from issues.jsonl
		cmd = exec.Command(gtBinary, "rig", "add", "myrig", existingRepo)
		cmd.Dir = townRoot
		cmd.Env = filterOutBeadsDir(os.Environ())
		cmd.Env = append(cmd.Env, "HOME="+tmpDir)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("gt rig add failed: %v\nOutput: %s", err, output)
		}

		// Verify routes.jsonl has the existing prefix, not an auto-generated one
		routesContent, err := os.ReadFile(filepath.Join(townRoot, ".beads", "routes.jsonl"))
		if err != nil {
			t.Fatalf("read routes.jsonl: %v", err)
		}

		if !strings.Contains(string(routesContent), `"prefix":"existing-prefix-"`) {
			t.Errorf("routes.jsonl should contain existing-prefix-, got:\n%s", routesContent)
		}
	})

	t.Run("TrackedRepoWithNoIssuesRequiresPrefix", func(t *testing.T) {
		// Regression test: When a tracked beads repo has NO issues (fresh init),
		// gt rig add must use the --prefix flag since there's nothing to detect from.

		townRoot := filepath.Join(tmpDir, "town-no-issues")
		reposDir := filepath.Join(tmpDir, "repos-no-issues")
		os.MkdirAll(reposDir, 0755)

		// Create a tracked beads repo with NO issues (just bd init)
		emptyRepo := filepath.Join(reposDir, "empty-repo")
		createTrackedBeadsRepo(t, emptyRepo, "empty-prefix")
		// Remove the db to simulate a clone
		os.Remove(filepath.Join(emptyRepo, ".beads", "beads.db"))

		// Install town
		cmd := exec.Command(gtBinary, "install", townRoot, "--name", "no-issues-test")
		cmd.Env = filterOutBeadsDir(os.Environ())
		cmd.Env = append(cmd.Env, "HOME="+tmpDir)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("gt install failed: %v\nOutput: %s", err, output)
		}

		// Add rig WITH --prefix since we can't detect from empty issues.jsonl
		cmd = exec.Command(gtBinary, "rig", "add", "emptyrig", emptyRepo, "--prefix", "empty-prefix")
		cmd.Dir = townRoot
		cmd.Env = filterOutBeadsDir(os.Environ())
		cmd.Env = append(cmd.Env, "HOME="+tmpDir)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("gt rig add with --prefix failed: %v\nOutput: %s", err, output)
		}

		// Verify routes.jsonl has the prefix
		routesContent, err := os.ReadFile(filepath.Join(townRoot, ".beads", "routes.jsonl"))
		if err != nil {
			t.Fatalf("read routes.jsonl: %v", err)
		}

		if !strings.Contains(string(routesContent), `"prefix":"empty-prefix-"`) {
			t.Errorf("routes.jsonl should contain empty-prefix-, got:\n%s", routesContent)
		}

		// Verify bd operations work with the configured prefix
		rigPath := filepath.Join(townRoot, "emptyrig")
		cmd = exec.Command("bd", "--no-daemon", "--json", "-q", "create",
			"--type", "task", "--title", "test-from-empty-repo")
		cmd.Dir = rigPath
		cmd.Env = filterOutBeadsDir(os.Environ())
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("bd create failed: %v", err)
		}

		var result struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(output, &result); err != nil {
			t.Fatalf("parse output: %v", err)
		}

		if !strings.HasPrefix(result.ID, "empty-prefix-") {
			t.Errorf("expected empty-prefix- prefix, got %s", result.ID)
		}
	})

	t.Run("TrackedRepoUsesCorrectPrefixForOperations", func(t *testing.T) {
		// Related to GitHub Issue #72: After cloning tracked beads, operations should
		// use the correct prefix from config, not directory name.
		// https://github.com/steveyegge/gastown/issues/72
		//
		// Currently FAILS: bd uses directory name as prefix instead of config.yaml prefix.

		townRoot := filepath.Join(tmpDir, "town-ops-test")
		reposDir := filepath.Join(tmpDir, "repos2")
		os.MkdirAll(reposDir, 0755)

		// Create tracked repo with specific prefix
		trackedRepo := filepath.Join(reposDir, "tracked-repo")
		createTrackedBeadsRepo(t, trackedRepo, "myprefix")

		// Install and add rig with explicit prefix
		cmd := exec.Command(gtBinary, "install", townRoot, "--name", "ops-test")
		cmd.Env = filterOutBeadsDir(os.Environ())
		cmd.Env = append(cmd.Env, "HOME="+tmpDir)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("gt install failed: %v\nOutput: %s", err, output)
		}

		cmd = exec.Command(gtBinary, "rig", "add", "tracked", trackedRepo, "--prefix", "myprefix")
		cmd.Dir = townRoot
		cmd.Env = filterOutBeadsDir(os.Environ())
		cmd.Env = append(cmd.Env, "HOME="+tmpDir)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("gt rig add failed: %v\nOutput: %s", err, output)
		}

		// Create a bead without explicit --prefix flag - should use rig's configured prefix
		rigPath := filepath.Join(townRoot, "tracked")
		cmd = exec.Command("bd", "--no-daemon", "--json", "-q", "create",
			"--type", "task", "--title", "test-default-prefix")
		cmd.Dir = rigPath
		cmd.Env = filterOutBeadsDir(os.Environ())
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("bd create failed: %v", err)
		}

		var result struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(output, &result); err != nil {
			t.Fatalf("parse output: %v", err)
		}

		// The bead should have myprefix-, not rig- or tracked-
		if !strings.HasPrefix(result.ID, "myprefix-") {
			t.Errorf("expected myprefix- prefix, got %s", result.ID)
		}
	})

	t.Run("UntrackedRepoWithSyncedBeads", func(t *testing.T) {
		// GitHub Issue #72: Repo with beads on sync branch should use that prefix
		// https://github.com/steveyegge/gastown/issues/72
		//
		// This tests the case where a repo doesn't have .beads/ tracked on main,
		// but has issues synced to a beads-sync branch with a specific prefix.
		t.Skip("SKIP: Reproduces https://github.com/steveyegge/gastown/issues/72 - should detect prefix from sync branch")

		// This would require creating a repo with a beads-sync branch containing
		// issues with a specific prefix. More complex to set up.
		t.Log("TODO: Implement sync branch prefix detection test")
	})
}
