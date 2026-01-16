package crew

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/steveyegge/gastown/internal/agent"
	"github.com/steveyegge/gastown/internal/git"
	"github.com/steveyegge/gastown/internal/rig"
)

func TestManagerAddAndGet(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "crew-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a mock rig
	rigPath := filepath.Join(tmpDir, "test-rig")
	if err := os.MkdirAll(rigPath, 0755); err != nil {
		t.Fatalf("failed to create rig dir: %v", err)
	}

	// Initialize git repo for the rig
	g := git.NewGit(rigPath)

	// For testing, we need a git URL - use a local bare repo
	bareRepoPath := filepath.Join(tmpDir, "bare-repo.git")
	cmd := []string{"git", "init", "--bare", bareRepoPath}
	if err := runCmd(cmd[0], cmd[1:]...); err != nil {
		t.Fatalf("failed to create bare repo: %v", err)
	}

	r := &rig.Rig{
		Name:   "test-rig",
		Path:   rigPath,
		GitURL: bareRepoPath,
	}

	mgr := NewManager(agent.NewDouble(), r, g, "claude")

	// Test Add
	worker, err := mgr.Add("dave", false)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	if worker.Name != "dave" {
		t.Errorf("expected name 'dave', got '%s'", worker.Name)
	}
	if worker.Rig != "test-rig" {
		t.Errorf("expected rig 'test-rig', got '%s'", worker.Rig)
	}
	if worker.Branch != "main" {
		t.Errorf("expected branch 'main', got '%s'", worker.Branch)
	}

	// Verify directory structure
	crewDir := filepath.Join(rigPath, "crew", "dave")
	if _, err := os.Stat(crewDir); os.IsNotExist(err) {
		t.Error("crew directory was not created")
	}

	mailDir := filepath.Join(crewDir, "mail")
	if _, err := os.Stat(mailDir); os.IsNotExist(err) {
		t.Error("mail directory was not created")
	}

	// NOTE: CLAUDE.md is NOT created by Add() - it's injected via SessionStart hook
	// See manager.go line 107-110 for why we skip CLAUDE.md creation

	stateFile := filepath.Join(crewDir, "state.json")
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Error("state.json was not created")
	}

	// Test Get
	retrieved, err := mgr.Get("dave")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if retrieved.Name != "dave" {
		t.Errorf("expected name 'dave', got '%s'", retrieved.Name)
	}

	// Test duplicate Add
	_, err = mgr.Add("dave", false)
	if err != ErrCrewExists {
		t.Errorf("expected ErrCrewExists, got %v", err)
	}

	// Test Get non-existent
	_, err = mgr.Get("nonexistent")
	if err != ErrCrewNotFound {
		t.Errorf("expected ErrCrewNotFound, got %v", err)
	}
}

func TestManagerAddUsesLocalRepoReference(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "crew-test-local-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	rigPath := filepath.Join(tmpDir, "test-rig")
	if err := os.MkdirAll(rigPath, 0755); err != nil {
		t.Fatalf("failed to create rig dir: %v", err)
	}

	remoteRepoPath := filepath.Join(tmpDir, "remote.git")
	if err := runCmd("git", "init", "--bare", remoteRepoPath); err != nil {
		t.Fatalf("failed to create bare repo: %v", err)
	}

	localRepoPath := filepath.Join(tmpDir, "local-repo")
	if err := runCmd("git", "init", localRepoPath); err != nil {
		t.Fatalf("failed to init local repo: %v", err)
	}
	if err := runCmd("git", "-C", localRepoPath, "config", "user.email", "test@test.com"); err != nil {
		t.Fatalf("failed to configure email: %v", err)
	}
	if err := runCmd("git", "-C", localRepoPath, "config", "user.name", "Test"); err != nil {
		t.Fatalf("failed to configure name: %v", err)
	}
	if err := runCmd("git", "-C", localRepoPath, "remote", "add", "origin", remoteRepoPath); err != nil {
		t.Fatalf("failed to add origin: %v", err)
	}

	if err := os.WriteFile(filepath.Join(localRepoPath, "README.md"), []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := runCmd("git", "-C", localRepoPath, "add", "."); err != nil {
		t.Fatalf("failed to add file: %v", err)
	}
	if err := runCmd("git", "-C", localRepoPath, "commit", "-m", "initial"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	r := &rig.Rig{
		Name:      "test-rig",
		Path:      rigPath,
		GitURL:    remoteRepoPath,
		LocalRepo: localRepoPath,
	}

	mgr := NewManager(agent.NewDouble(), r, git.NewGit(rigPath), "claude")

	worker, err := mgr.Add("dave", false)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	alternates := filepath.Join(worker.ClonePath, ".git", "objects", "info", "alternates")
	if _, err := os.Stat(alternates); err != nil {
		t.Fatalf("expected alternates file: %v", err)
	}
}

func TestManagerAddWithBranch(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "crew-test-branch-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a mock rig
	rigPath := filepath.Join(tmpDir, "test-rig")
	if err := os.MkdirAll(rigPath, 0755); err != nil {
		t.Fatalf("failed to create rig dir: %v", err)
	}

	g := git.NewGit(rigPath)

	// Create a local repo with initial commit for branch testing
	sourceRepoPath := filepath.Join(tmpDir, "source-repo")
	if err := os.MkdirAll(sourceRepoPath, 0755); err != nil {
		t.Fatalf("failed to create source repo dir: %v", err)
	}

	// Initialize source repo with a commit
	cmds := [][]string{
		{"git", "-C", sourceRepoPath, "init"},
		{"git", "-C", sourceRepoPath, "config", "user.email", "test@test.com"},
		{"git", "-C", sourceRepoPath, "config", "user.name", "Test"},
	}
	for _, cmd := range cmds {
		if err := runCmd(cmd[0], cmd[1:]...); err != nil {
			t.Fatalf("failed to run %v: %v", cmd, err)
		}
	}

	// Create initial file and commit
	testFile := filepath.Join(sourceRepoPath, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	cmds = [][]string{
		{"git", "-C", sourceRepoPath, "add", "."},
		{"git", "-C", sourceRepoPath, "commit", "-m", "Initial commit"},
	}
	for _, cmd := range cmds {
		if err := runCmd(cmd[0], cmd[1:]...); err != nil {
			t.Fatalf("failed to run %v: %v", cmd, err)
		}
	}

	r := &rig.Rig{
		Name:   "test-rig",
		Path:   rigPath,
		GitURL: sourceRepoPath,
	}

	mgr := NewManager(agent.NewDouble(), r, g, "claude")

	// Test Add with branch
	worker, err := mgr.Add("emma", true)
	if err != nil {
		t.Fatalf("Add with branch failed: %v", err)
	}

	if worker.Branch != "crew/emma" {
		t.Errorf("expected branch 'crew/emma', got '%s'", worker.Branch)
	}
}

func TestManagerList(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "crew-test-list-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a mock rig
	rigPath := filepath.Join(tmpDir, "test-rig")
	if err := os.MkdirAll(rigPath, 0755); err != nil {
		t.Fatalf("failed to create rig dir: %v", err)
	}

	g := git.NewGit(rigPath)

	// Create a bare repo for cloning
	bareRepoPath := filepath.Join(tmpDir, "bare-repo.git")
	if err := runCmd("git", "init", "--bare", bareRepoPath); err != nil {
		t.Fatalf("failed to create bare repo: %v", err)
	}

	r := &rig.Rig{
		Name:   "test-rig",
		Path:   rigPath,
		GitURL: bareRepoPath,
	}

	mgr := NewManager(agent.NewDouble(), r, g, "claude")

	// Initially empty
	workers, err := mgr.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(workers) != 0 {
		t.Errorf("expected 0 workers, got %d", len(workers))
	}

	// Add some workers
	_, err = mgr.Add("alice", false)
	if err != nil {
		t.Fatalf("Add alice failed: %v", err)
	}
	_, err = mgr.Add("bob", false)
	if err != nil {
		t.Fatalf("Add bob failed: %v", err)
	}

	workers, err = mgr.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(workers) != 2 {
		t.Errorf("expected 2 workers, got %d", len(workers))
	}
}

func TestManagerRemove(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "crew-test-remove-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a mock rig
	rigPath := filepath.Join(tmpDir, "test-rig")
	if err := os.MkdirAll(rigPath, 0755); err != nil {
		t.Fatalf("failed to create rig dir: %v", err)
	}

	g := git.NewGit(rigPath)

	// Create a bare repo for cloning
	bareRepoPath := filepath.Join(tmpDir, "bare-repo.git")
	if err := runCmd("git", "init", "--bare", bareRepoPath); err != nil {
		t.Fatalf("failed to create bare repo: %v", err)
	}

	r := &rig.Rig{
		Name:   "test-rig",
		Path:   rigPath,
		GitURL: bareRepoPath,
	}

	mgr := NewManager(agent.NewDouble(), r, g, "claude")

	// Add a worker
	_, err = mgr.Add("charlie", false)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Remove it (with force since CLAUDE.md is uncommitted)
	err = mgr.Remove("charlie", true)
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Verify it's gone
	_, err = mgr.Get("charlie")
	if err != ErrCrewNotFound {
		t.Errorf("expected ErrCrewNotFound, got %v", err)
	}

	// Remove non-existent
	err = mgr.Remove("nonexistent", false)
	if err != ErrCrewNotFound {
		t.Errorf("expected ErrCrewNotFound, got %v", err)
	}
}

// =============================================================================
// Lifecycle Tests (Start, Stop, IsRunning)
// =============================================================================

func TestManagerStart_CreatesSession(t *testing.T) {
	tmpDir, rigPath, bareRepoPath := setupTestRigWithRepo(t)
	defer os.RemoveAll(tmpDir)

	r := &rig.Rig{
		Name:   "test-rig",
		Path:   rigPath,
		GitURL: bareRepoPath,
	}

	sess := agent.NewDouble()
	mgr := NewManager(sess, r, git.NewGit(rigPath), "claude")

	// Add a worker first
	_, err := mgr.Add("alice", false)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Start the worker
	err = mgr.Start("alice", StartOptions{})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify session exists
	running, err := mgr.IsRunning("alice")
	if err != nil {
		t.Fatalf("IsRunning failed: %v", err)
	}
	if !running {
		t.Error("expected IsRunning = true after Start")
	}
}

func TestManagerStart_AutoCreatesWorker(t *testing.T) {
	tmpDir, rigPath, bareRepoPath := setupTestRigWithRepo(t)
	defer os.RemoveAll(tmpDir)

	r := &rig.Rig{
		Name:   "test-rig",
		Path:   rigPath,
		GitURL: bareRepoPath,
	}

	sess := agent.NewDouble()
	mgr := NewManager(sess, r, git.NewGit(rigPath), "claude")

	// Start without Add - should auto-create
	err := mgr.Start("bob", StartOptions{})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Worker should exist
	worker, err := mgr.Get("bob")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if worker.Name != "bob" {
		t.Errorf("expected worker name 'bob', got '%s'", worker.Name)
	}
}

func TestManagerStart_AlreadyRunning_ReturnsError(t *testing.T) {
	tmpDir, rigPath, bareRepoPath := setupTestRigWithRepo(t)
	defer os.RemoveAll(tmpDir)

	r := &rig.Rig{
		Name:   "test-rig",
		Path:   rigPath,
		GitURL: bareRepoPath,
	}

	sess := agent.NewDouble()
	mgr := NewManager(sess, r, git.NewGit(rigPath), "claude")

	// Start once
	err := mgr.Start("charlie", StartOptions{})
	if err != nil {
		t.Fatalf("First Start failed: %v", err)
	}

	// Start again should fail
	err = mgr.Start("charlie", StartOptions{})
	if err == nil {
		t.Error("expected error for already running session")
	}
}

func TestManagerStart_KillExisting(t *testing.T) {
	tmpDir, rigPath, bareRepoPath := setupTestRigWithRepo(t)
	defer os.RemoveAll(tmpDir)

	r := &rig.Rig{
		Name:   "test-rig",
		Path:   rigPath,
		GitURL: bareRepoPath,
	}

	sess := agent.NewDouble()
	mgr := NewManager(sess, r, git.NewGit(rigPath), "claude")

	// Start once
	err := mgr.Start("dave", StartOptions{})
	if err != nil {
		t.Fatalf("First Start failed: %v", err)
	}

	// Start again with KillExisting should succeed
	err = mgr.Start("dave", StartOptions{KillExisting: true})
	if err != nil {
		t.Fatalf("Start with KillExisting failed: %v", err)
	}
}

func TestManagerStart_InvalidName_ReturnsError(t *testing.T) {
	tmpDir, rigPath, bareRepoPath := setupTestRigWithRepo(t)
	defer os.RemoveAll(tmpDir)

	r := &rig.Rig{
		Name:   "test-rig",
		Path:   rigPath,
		GitURL: bareRepoPath,
	}

	sess := agent.NewDouble()
	mgr := NewManager(sess, r, git.NewGit(rigPath), "claude")

	// Start with empty name
	err := mgr.Start("", StartOptions{})
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestManagerStop_TerminatesSession(t *testing.T) {
	tmpDir, rigPath, bareRepoPath := setupTestRigWithRepo(t)
	defer os.RemoveAll(tmpDir)

	r := &rig.Rig{
		Name:   "test-rig",
		Path:   rigPath,
		GitURL: bareRepoPath,
	}

	sess := agent.NewDouble()
	mgr := NewManager(sess, r, git.NewGit(rigPath), "claude")

	// Start
	err := mgr.Start("emma", StartOptions{})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Stop
	err = mgr.Stop("emma")
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify not running
	running, _ := mgr.IsRunning("emma")
	if running {
		t.Error("expected IsRunning = false after Stop")
	}
}

func TestManagerStop_NotRunning_ReturnsError(t *testing.T) {
	tmpDir, rigPath, bareRepoPath := setupTestRigWithRepo(t)
	defer os.RemoveAll(tmpDir)

	r := &rig.Rig{
		Name:   "test-rig",
		Path:   rigPath,
		GitURL: bareRepoPath,
	}

	sess := agent.NewDouble()
	mgr := NewManager(sess, r, git.NewGit(rigPath), "claude")

	// Add but don't start
	_, err := mgr.Add("frank", false)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Stop should fail
	err = mgr.Stop("frank")
	if err != ErrNotRunning {
		t.Errorf("expected ErrNotRunning, got %v", err)
	}
}

func TestManagerStop_InvalidName_ReturnsError(t *testing.T) {
	tmpDir, rigPath, bareRepoPath := setupTestRigWithRepo(t)
	defer os.RemoveAll(tmpDir)

	r := &rig.Rig{
		Name:   "test-rig",
		Path:   rigPath,
		GitURL: bareRepoPath,
	}

	sess := agent.NewDouble()
	mgr := NewManager(sess, r, git.NewGit(rigPath), "claude")

	// Stop with empty name
	err := mgr.Stop("")
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestManagerRemove_RunningWorker_Fails(t *testing.T) {
	tmpDir, rigPath, bareRepoPath := setupTestRigWithRepo(t)
	defer os.RemoveAll(tmpDir)

	r := &rig.Rig{
		Name:   "test-rig",
		Path:   rigPath,
		GitURL: bareRepoPath,
	}

	sess := agent.NewDouble()
	mgr := NewManager(sess, r, git.NewGit(rigPath), "claude")

	// Start a worker
	err := mgr.Start("grace", StartOptions{})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Remove without force should fail
	err = mgr.Remove("grace", false)
	if err == nil {
		t.Error("expected error when removing running worker without force")
	}
}

func TestManagerIsRunning_CorrectState(t *testing.T) {
	tmpDir, rigPath, bareRepoPath := setupTestRigWithRepo(t)
	defer os.RemoveAll(tmpDir)

	r := &rig.Rig{
		Name:   "test-rig",
		Path:   rigPath,
		GitURL: bareRepoPath,
	}

	sess := agent.NewDouble()
	mgr := NewManager(sess, r, git.NewGit(rigPath), "claude")

	// Not running initially
	running, _ := mgr.IsRunning("henry")
	if running {
		t.Error("expected IsRunning = false before Start")
	}

	// Start
	_ = mgr.Start("henry", StartOptions{})
	running, _ = mgr.IsRunning("henry")
	if !running {
		t.Error("expected IsRunning = true after Start")
	}

	// Stop
	_ = mgr.Stop("henry")
	running, _ = mgr.IsRunning("henry")
	if running {
		t.Error("expected IsRunning = false after Stop")
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

func setupTestRigWithRepo(t *testing.T) (tmpDir, rigPath, bareRepoPath string) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "crew-lifecycle-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	rigPath = filepath.Join(tmpDir, "test-rig")
	if err := os.MkdirAll(rigPath, 0755); err != nil {
		t.Fatalf("failed to create rig dir: %v", err)
	}

	bareRepoPath = filepath.Join(tmpDir, "bare-repo.git")
	if err := runCmd("git", "init", "--bare", bareRepoPath); err != nil {
		t.Fatalf("failed to create bare repo: %v", err)
	}

	return tmpDir, rigPath, bareRepoPath
}

// Helper to run commands
func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Run()
}
