package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// buildGT builds the gt binary and returns its path.
// It caches the build across tests in the same run.
var cachedGTBinary string

func buildGT(t *testing.T) string {
	t.Helper()

	if cachedGTBinary != "" {
		// Verify cached binary still exists
		if _, err := os.Stat(cachedGTBinary); err == nil {
			return cachedGTBinary
		}
		// Binary was cleaned up, rebuild
		cachedGTBinary = ""
	}

	// Find project root (where go.mod is)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	// Walk up to find go.mod
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

	// Build gt binary to a persistent temp location (not per-test)
	tmpDir := os.TempDir()
	binaryName := "gt-integration-test"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	tmpBinary := filepath.Join(tmpDir, binaryName)
	// Must set BuiltProperly=1 via ldflags, otherwise binary refuses to run
	ldflags := "-X github.com/steveyegge/gastown/internal/cmd.BuiltProperly=1"
	cmd := exec.Command("go", "build", "-ldflags", ldflags, "-o", tmpBinary, "./cmd/gt")
	cmd.Dir = projectRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build gt: %v\nOutput: %s", err, output)
	}

	cachedGTBinary = tmpBinary
	return tmpBinary
}

// cleanE2EEnv returns os.Environ() with all GT_* variables removed.
// This ensures tests don't inherit stale role environment from CI or previous tests.
func cleanE2EEnv() []string {
	var clean []string
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, "GT_") {
			clean = append(clean, env)
		}
	}
	return clean
}

// configureDoltIdentity sets dolt global config in the test's HOME directory.
// Tests override HOME to a temp dir for isolation, so dolt can't find the
// container's build-time global config. This must run before gt dolt init-rig.
func configureDoltIdentity(t *testing.T, env []string) {
	t.Helper()
	for _, args := range [][]string{
		{"config", "--global", "--add", "user.name", "Test User"},
		{"config", "--global", "--add", "user.email", "test@test.com"},
	} {
		cmd := exec.Command("dolt", args...)
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("dolt %v failed: %v\n%s", args, err, out)
		}
	}
}

// runGTCmd runs a gt command and fails the test if it fails.
func runGTCmd(t *testing.T, binary, dir string, env []string, args ...string) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gt %v failed: %v\n%s", args, err, out)
	}
}
