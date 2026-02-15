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

// polecatWorktree locates the git worktree directory for a polecat.
// Checks new structure (polecats/<name>/<rigname>/) first, falls back to old.
func polecatWorktree(t *testing.T, hqPath, rigName, polecatName string) string {
	t.Helper()
	// New structure: <rig>/polecats/<name>/<rigname>/
	newPath := filepath.Join(hqPath, rigName, "polecats", polecatName, rigName)
	if info, err := os.Stat(newPath); err == nil && info.IsDir() {
		return newPath
	}
	// Old structure: <rig>/polecats/<name>/
	oldPath := filepath.Join(hqPath, rigName, "polecats", polecatName)
	if info, err := os.Stat(oldPath); err == nil && info.IsDir() {
		return oldPath
	}
	t.Fatalf("polecat worktree not found at %s or %s", newPath, oldPath)
	return ""
}

// capturePaneTail returns the last non-empty lines from a tmux pane.
func capturePaneTail(sessionName string, n int) string {
	cmd := exec.Command("tmux", "capture-pane", "-t", sessionName, "-p")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Sprintf("(capture failed: %v)", err)
	}
	lines := strings.Split(string(out), "\n")
	var tail []string
	for i := len(lines) - 1; i >= 0 && len(tail) < n; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			tail = append([]string{lines[i]}, tail...)
		}
	}
	return strings.Join(tail, "\n")
}

// TestAgentSlingTextChange verifies the full agent work pipeline end-to-end:
// sling a task → model creates a file → file is committed to the polecat branch.
//
// This is the strongest e2e test: it proves the model receives work instructions
// (via beacon + gt prime), executes tool calls through LiteLLM, and produces a
// committed git change — touching every layer of the pipeline.
func TestAgentSlingTextChange(t *testing.T) {
	gtBinary := buildGT(t)
	hqPath, env := setupAgentTestTown(t, gtBinary)

	// Create bead with explicit, verifiable instructions.
	// The model sees this as the Title in "gt prime --hook" output.
	beadID := "hq-sling1"
	createTestBead(t, hqPath, env, beadID,
		"Create and commit a file named test-output.txt containing exactly: e2e-agent-test-ok")

	// Sling to rig — auto-spawns polecat, sends beacon with "Run gt prime --hook"
	sessionName := slingToRig(t, gtBinary, hqPath, env, beadID, "testrig")
	t.Cleanup(func() {
		_ = exec.Command("tmux", "kill-session", "-t", sessionName).Run()
	})

	// Wait for agent ready
	waitForReady(t, sessionName, 90*time.Second)

	// Find the polecat worktree where the model works
	polecatName := strings.TrimPrefix(sessionName, "gt-testrig-")
	worktree := polecatWorktree(t, hqPath, "testrig", polecatName)
	t.Logf("Polecat %s worktree: %s", polecatName, worktree)

	// Poll for the file to be committed to the branch.
	// The model needs to: read the task → create the file → git add → git commit.
	targetFile := "test-output.txt"
	expectedContent := "e2e-agent-test-ok"
	timeout := 3 * time.Minute
	deadline := time.Now().Add(timeout)
	lastLog := time.Now()
	committed := false

	for time.Now().Before(deadline) {
		// Fail fast if session died (agent crashed)
		if err := exec.Command("tmux", "has-session", "-t", sessionName).Run(); err != nil {
			t.Fatalf("agent session died while waiting for text change\nLast pane:\n%s",
				capturePaneTail(sessionName, 10))
		}

		// Check if file is committed to the branch via git show HEAD:<file>
		gitShow := exec.Command("git", "show", "HEAD:"+targetFile)
		gitShow.Dir = worktree
		if out, err := gitShow.Output(); err == nil {
			if strings.TrimSpace(string(out)) == expectedContent {
				committed = true
				t.Logf("File committed with correct content after %s",
					time.Since(deadline.Add(-timeout)).Round(time.Second))
				break
			}
		}

		// Periodic debug logging
		if time.Since(lastLog) >= 15*time.Second {
			filePath := filepath.Join(worktree, targetFile)
			if content, err := os.ReadFile(filePath); err == nil {
				t.Logf("File in worktree (not yet committed): %q", strings.TrimSpace(string(content)))
			}
			t.Logf("Pane tail:\n%s", capturePaneTail(sessionName, 5))
			lastLog = time.Now()
		}

		time.Sleep(5 * time.Second)
	}

	if !committed {
		// Distinguish "file exists but uncommitted" from "file never created"
		filePath := filepath.Join(worktree, targetFile)
		if content, err := os.ReadFile(filePath); err == nil {
			t.Fatalf("file exists with content %q but was not committed to branch\nPane:\n%s",
				strings.TrimSpace(string(content)), capturePaneTail(sessionName, 15))
		}
		t.Fatalf("agent did not create and commit %s within %s\nPane:\n%s",
			targetFile, timeout, capturePaneTail(sessionName, 15))
	}

	// Verify the commit is on the polecat's branch (not detached HEAD etc)
	gitLog := exec.Command("git", "log", "--oneline", "--", targetFile)
	gitLog.Dir = worktree
	logOut, err := gitLog.Output()
	if err != nil || strings.TrimSpace(string(logOut)) == "" {
		t.Errorf("no git log entries for %s despite git show succeeding", targetFile)
	} else {
		t.Logf("Commits touching %s:\n%s", targetFile, strings.TrimSpace(string(logOut)))
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
