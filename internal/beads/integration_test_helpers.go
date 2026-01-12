//go:build integration

package beads

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestEnv provides a standard test environment for Beads API integration tests.
// It uses gt install for proper production-like setup.
type TestEnv struct {
	TownRoot string
	RigPath  string // Optional - set if rig was added
	RigName  string // Optional
	GTBinary string
}

// cachedGTBinary caches the gt binary path across tests
var cachedGTBinary string

// buildGT builds the gt binary and returns its path.
// It caches the build across tests in the same run.
func buildGT(t *testing.T) string {
	t.Helper()

	if cachedGTBinary != "" {
		if _, err := os.Stat(cachedGTBinary); err == nil {
			return cachedGTBinary
		}
		cachedGTBinary = ""
	}

	// Find project root
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	projectRoot := wd
	for {
		if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(projectRoot)
		if parent == projectRoot {
			t.Fatal("could not find project root (go.mod)")
		}
		projectRoot = parent
	}

	tmpBinary := filepath.Join(os.TempDir(), "gt-integration-test")
	cmd := exec.Command("go", "build", "-o", tmpBinary, "./cmd/gt")
	cmd.Dir = projectRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build gt: %v\nOutput: %s", err, output)
	}

	cachedGTBinary = tmpBinary
	return tmpBinary
}

// SetupTownEnv creates a production-like Gas Town HQ using gt install.
// This is the preferred setup for testing Beads API operations.
func SetupTownEnv(t *testing.T, name string) *TestEnv {
	t.Helper()

	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd not installed")
	}

	gtBinary := buildGT(t)
	tmpDir := t.TempDir()
	townRoot := filepath.Join(tmpDir, "town")

	// Run gt install
	cmd := exec.Command(gtBinary, "install", townRoot, "--name", name)
	cmd.Env = filterOutBeadsDir(os.Environ())
	cmd.Env = append(cmd.Env, "HOME="+tmpDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gt install failed: %v\nOutput: %s", err, output)
	}

	return &TestEnv{
		TownRoot: townRoot,
		GTBinary: gtBinary,
	}
}

// SetupTownWithRigEnv creates a Gas Town HQ with an untracked rig.
// This is useful for testing rig-context operations.
func SetupTownWithRigEnv(t *testing.T, townName, rigName, rigPrefix string) *TestEnv {
	t.Helper()

	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd not installed")
	}

	gtBinary := buildGT(t)
	tmpDir := t.TempDir()
	townRoot := filepath.Join(tmpDir, "town")
	reposDir := filepath.Join(tmpDir, "repos")

	os.MkdirAll(reposDir, 0755)

	// Create a source repo for the rig
	rigRepo := filepath.Join(reposDir, rigName+"-repo")
	createBareGitRepoHelper(t, rigRepo)

	// Run gt install
	cmd := exec.Command(gtBinary, "install", townRoot, "--name", townName)
	cmd.Env = filterOutBeadsDir(os.Environ())
	cmd.Env = append(cmd.Env, "HOME="+tmpDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gt install failed: %v\nOutput: %s", err, output)
	}

	// Add the rig
	cmd = exec.Command(gtBinary, "rig", "add", rigName, rigRepo, "--prefix", rigPrefix)
	cmd.Dir = townRoot
	cmd.Env = filterOutBeadsDir(os.Environ())
	cmd.Env = append(cmd.Env, "HOME="+tmpDir)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gt rig add failed: %v\nOutput: %s", err, output)
	}

	return &TestEnv{
		TownRoot: townRoot,
		RigPath:  filepath.Join(townRoot, rigName),
		RigName:  rigName,
		GTBinary: gtBinary,
	}
}

// createBareGitRepoHelper creates a minimal git repo without .beads/
func createBareGitRepoHelper(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}

	cmd := exec.Command("git", "init")
	cmd.Dir = path
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\nOutput: %s", err, output)
	}

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

// NewForTown creates a Beads API instance for town-level operations.
func (env *TestEnv) NewForTown() *Beads {
	return New(env.TownRoot)
}

// NewForRig creates a Beads API instance for rig-level operations.
// Requires the environment to have been set up with SetupTownWithRigEnv.
func (env *TestEnv) NewForRig() *Beads {
	if env.RigPath == "" {
		panic("NewForRig called on TestEnv without rig - use SetupTownWithRigEnv")
	}
	return New(env.TownRoot, env.RigPath)
}

// CreateTestBead creates a test bead using the Beads API.
// This is preferred over raw bd commands for test setup.
func (env *TestEnv) CreateTestBead(t *testing.T, title string, labels ...string) *Issue {
	t.Helper()
	bd := env.NewForTown()
	issue, err := bd.Create(CreateOptions{
		Title:  title,
		Type:   "task",
		Labels: labels,
	})
	if err != nil {
		t.Fatalf("CreateTestBead failed: %v", err)
	}
	return issue
}

// CreateTestBeadWithID creates a test bead with a specific ID using the Beads API.
func (env *TestEnv) CreateTestBeadWithID(t *testing.T, id, title string, labels ...string) *Issue {
	t.Helper()
	bd := env.NewForTown()
	issue, err := bd.Create(CreateOptions{
		ID:     id,
		Title:  title,
		Type:   "task",
		Labels: labels,
	})
	if err != nil {
		t.Fatalf("CreateTestBeadWithID failed: %v", err)
	}
	return issue
}

// CreateTestAgentBead creates a test agent bead using the Beads API.
func (env *TestEnv) CreateTestAgentBead(t *testing.T, roleType, name, title string, fields *AgentFields) *Issue {
	t.Helper()
	if env.RigPath == "" {
		// Use town agent creation
		bd := env.NewForTown()
		issue, err := bd.CreateTownAgent(roleType, title, fields)
		if err != nil {
			t.Fatalf("CreateTownAgent failed: %v", err)
		}
		return issue
	}
	// Use rig agent creation
	bd := env.NewForRig()
	issue, err := bd.CreateRigAgent(roleType, name, title, fields)
	if err != nil {
		t.Fatalf("CreateRigAgent failed: %v", err)
	}
	return issue
}

// CreateRigTestBead creates a test bead in the rig database using the Beads API.
func (env *TestEnv) CreateRigTestBead(t *testing.T, title string, labels ...string) *Issue {
	t.Helper()
	if env.RigPath == "" {
		t.Fatal("CreateRigTestBead called on TestEnv without rig")
	}
	bd := env.NewForRig()
	issue, err := bd.Create(CreateOptions{
		Title:  title,
		Type:   "task",
		Labels: labels,
	})
	if err != nil {
		t.Fatalf("CreateRigTestBead failed: %v", err)
	}
	return issue
}

// SetupLegacyTownEnv creates a minimal environment using bd init directly.
// This is provided for backward compatibility but SetupTownEnv is preferred.
// DEPRECATED: Use SetupTownEnv instead.
func SetupLegacyTownEnv(t *testing.T, prefix string) string {
	t.Helper()

	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd not installed")
	}

	tmpDir := t.TempDir()
	townRoot := filepath.Join(tmpDir, "town")
	os.MkdirAll(townRoot, 0755)

	cmd := exec.Command("bd", "init", "--prefix", prefix)
	cmd.Dir = townRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bd init failed: %v\nOutput: %s", err, output)
	}

	return townRoot
}
