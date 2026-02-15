//go:build e2e_agent

package cmd

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestMain checks prerequisites for agent e2e tests and skips all tests if
// any are missing. Requires: gt binary, tmux, claude CLI, LiteLLM /health
// endpoint, and GEMINI_API_KEY env var.
func TestMain(m *testing.M) {
	if err := checkAgentPrereqs(); err != nil {
		fmt.Fprintf(os.Stderr, "Skipping e2e_agent tests: %v\n", err)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func checkAgentPrereqs() error {
	if os.Getenv("GEMINI_API_KEY") == "" {
		return fmt.Errorf("GEMINI_API_KEY not set")
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		return fmt.Errorf("tmux not found: %w", err)
	}
	if _, err := exec.LookPath("claude"); err != nil {
		return fmt.Errorf("claude CLI not found: %w", err)
	}
	resp, err := http.Get("http://localhost:4000/health")
	if err != nil {
		return fmt.Errorf("LiteLLM not reachable: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("LiteLLM unhealthy: status %d", resp.StatusCode)
	}
	return nil
}

// setupAgentTestTown creates a town with beads, dolt, and a rig for agent tests.
// Returns (hqPath, env, cleanup). The caller must defer cleanup().
func setupAgentTestTown(t *testing.T, gtBinary string) (string, []string) {
	t.Helper()

	tmpDir := t.TempDir()
	hqPath := filepath.Join(tmpDir, "test-hq")

	env := cleanE2EEnv()
	env = append(env, "HOME="+tmpDir)
	env = append(env, "ANTHROPIC_BASE_URL=http://localhost:4000")
	env = append(env, "ANTHROPIC_AUTH_TOKEN=fake")

	// Install town with beads
	runGTCmd(t, gtBinary, tmpDir, env, "install", hqPath, "--name", "test-town", "--git")

	// Start dolt (kill stale servers from previous tests)
	_ = exec.Command("pkill", "-f", "dolt sql-server").Run()
	configureDoltIdentity(t, env)
	runGTCmd(t, gtBinary, hqPath, env, "dolt", "init-rig", "hq")
	runGTCmd(t, gtBinary, hqPath, env, "dolt", "start")
	t.Cleanup(func() {
		cmd := exec.Command(gtBinary, "dolt", "stop")
		cmd.Dir = hqPath
		cmd.Env = env
		_ = cmd.Run()
	})

	// Add rig
	runGTCmd(t, gtBinary, hqPath, env, "rig", "add", "testrig",
		"https://github.com/octocat/Hello-World.git", "--prefix", "tr")

	return hqPath, env
}

// createTestBead creates a bead in the town's beads database and returns its ID.
func createTestBead(t *testing.T, hqPath string, env []string, beadID, title string) {
	t.Helper()
	cmd := exec.Command("bd", "create", title, "--id="+beadID, "--force")
	cmd.Dir = hqPath
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bd create %s failed: %v\n%s", beadID, err, out)
	}
}

// slingToRig slings a bead to a rig and returns the polecat's tmux session name.
func slingToRig(t *testing.T, gtBinary, hqPath string, env []string, beadID, rigName string) string {
	t.Helper()

	slingCmd := exec.Command(gtBinary, "sling", beadID, rigName)
	slingCmd.Dir = hqPath
	slingCmd.Env = env
	slingOut, err := slingCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gt sling %s %s failed: %v\n%s", beadID, rigName, err, slingOut)
	}
	t.Logf("Sling output:\n%s", slingOut)

	// Find the tmux session created by sling
	prefix := fmt.Sprintf("gt-%s-", rigName)
	var sessionName string
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		listCmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}")
		listOut, err := listCmd.Output()
		if err == nil {
			for _, line := range strings.Split(strings.TrimSpace(string(listOut)), "\n") {
				if strings.HasPrefix(line, prefix) {
					sessionName = line
					break
				}
			}
		}
		if sessionName != "" {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if sessionName == "" {
		t.Fatalf("no tmux session found with prefix %q after sling", prefix)
	}
	t.Logf("Found session: %s", sessionName)
	return sessionName
}

// waitForReady polls the tmux pane for the Claude Code ready prompt (❯).
func waitForReady(t *testing.T, sessionName string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		captureCmd := exec.Command("tmux", "capture-pane", "-t", sessionName, "-p")
		out, err := captureCmd.Output()
		if err == nil {
			// Normalize NBSP for prompt matching (same as matchesPromptPrefix in tmux.go)
			content := strings.ReplaceAll(string(out), "\u00a0", " ")
			if strings.Contains(content, "❯") {
				t.Logf("Agent ready after %s", time.Since(deadline.Add(-timeout)).Round(time.Second))
				return
			}
		}
		time.Sleep(2 * time.Second)
	}
	// Capture final pane state for debugging
	captureCmd := exec.Command("tmux", "capture-pane", "-t", sessionName, "-p")
	out, _ := captureCmd.Output()
	t.Fatalf("agent did not show ready prompt (❯) within %s\nFinal pane content:\n%s", timeout, out)
}

// TestAgentSpawnAndReady exercises the full polecat spawn pipeline:
// gt install → gt rig add → bd create → gt sling → wait for ❯ prompt.
//
// This test verifies:
//   - Town setup with beads and dolt works
//   - Sling auto-spawns a polecat with Claude Code
//   - Claude Code starts via LiteLLM proxy to Gemini Flash
//   - Runtime ready detection (❯ prompt) succeeds
//   - Tmux session remains alive after ready
func TestAgentSpawnAndReady(t *testing.T) {
	gtBinary := buildGT(t)
	hqPath, env := setupAgentTestTown(t, gtBinary)

	// Create a test bead to sling
	beadID := "hq-test1"
	createTestBead(t, hqPath, env, beadID, "Test agent spawn task")

	// Sling bead to rig — auto-spawns a polecat with Claude Code
	sessionName := slingToRig(t, gtBinary, hqPath, env, beadID, "testrig")
	t.Cleanup(func() {
		_ = exec.Command("tmux", "kill-session", "-t", sessionName).Run()
	})

	// Wait for Claude Code to become ready (❯ prompt appears)
	waitForReady(t, sessionName, 90*time.Second)

	// Verify session is still alive after ready detection
	hasSession := exec.Command("tmux", "has-session", "-t", sessionName)
	if err := hasSession.Run(); err != nil {
		t.Fatalf("session %s died after ready detection", sessionName)
	}
}

// TestAgentDone tests polecat cleanup after agent work completes.
//
// After spawning a polecat via sling and waiting for it to be ready,
// this test verifies that gt polecat nuke properly cleans up:
//   - Tmux session is killed
//   - Polecat worktree directory is removed
func TestAgentDone(t *testing.T) {
	gtBinary := buildGT(t)
	hqPath, env := setupAgentTestTown(t, gtBinary)

	// Create and sling a bead
	beadID := "hq-test2"
	createTestBead(t, hqPath, env, beadID, "Test agent done task")
	sessionName := slingToRig(t, gtBinary, hqPath, env, beadID, "testrig")

	// Wait for agent to be ready before nuking
	waitForReady(t, sessionName, 90*time.Second)

	// Extract polecat name from session name (format: gt-<rig>-<polecat>)
	polecatName := strings.TrimPrefix(sessionName, "gt-testrig-")
	t.Logf("Nuking polecat: testrig/%s", polecatName)

	// Nuke the polecat (tests cleanup path: session kill + worktree removal)
	nukeCmd := exec.Command(gtBinary, "polecat", "nuke", "testrig/"+polecatName, "--force")
	nukeCmd.Dir = hqPath
	nukeCmd.Env = env
	if out, err := nukeCmd.CombinedOutput(); err != nil {
		t.Fatalf("gt polecat nuke failed: %v\n%s", err, out)
	}

	// Verify session is gone
	time.Sleep(1 * time.Second)
	hasSession := exec.Command("tmux", "has-session", "-t", sessionName)
	if err := hasSession.Run(); err == nil {
		t.Errorf("session %s still exists after nuke", sessionName)
	}

	// Verify polecat worktree directory is removed
	polecatDir := filepath.Join(hqPath, "testrig", "polecats", polecatName)
	if _, err := os.Stat(polecatDir); !os.IsNotExist(err) {
		t.Errorf("polecat directory %s still exists after nuke", polecatDir)
	}
}
