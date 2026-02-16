//go:build e2e_agent

package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/steveyegge/gastown/internal/config"
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
	if _, err := exec.LookPath("tmux"); err != nil {
		return fmt.Errorf("tmux not found: %w", err)
	}
	if _, err := exec.LookPath("claude"); err != nil {
		return fmt.Errorf("claude CLI not found: %w", err)
	}
	// LiteLLM proxy must be running (routes Claude API calls to cheap models).
	// OPENROUTER_API_KEY must be set in the LiteLLM process env.
	resp, err := http.Get("http://localhost:4000/health")
	if err != nil {
		return fmt.Errorf("LiteLLM not reachable at localhost:4000: %w (start with: litellm --config litellm-config.yaml)", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("LiteLLM unhealthy: status %d", resp.StatusCode)
	}
	return nil
}

// Model tiers for e2e tests. Each maps to a Claude model name that LiteLLM
// routes to a cheap backend (see litellm-config.yaml).
// Must use sonnet/haiku (not opus) because fake OAuth creds fail the opus subscription check.
//
//	modelCheap  → --model haiku  → Gemini 2.0 Flash  ($0.10/$0.40)
//	modelSmart  → --model sonnet → Gemini 2.5 Flash  ($0.15/$0.60)
const (
	modelCheap = "haiku"  // simple tasks: spawn, ready, cleanup
	modelSmart = "sonnet" // complex tasks: read instructions, create files, commit
)

// setupAgentTestTown creates a town with beads, dolt, and a rig for agent tests.
// model selects which Claude model name to use (routed by LiteLLM to cheap backends).
// Returns (hqPath, env). The caller must defer cleanup (registered via t.Cleanup).
func setupAgentTestTown(t *testing.T, gtBinary, model string) (string, []string) {
	t.Helper()

	tmpDir := t.TempDir()
	hqPath := filepath.Join(tmpDir, "test-hq")

	env := cleanE2EEnv()
	env = append(env, "HOME="+tmpDir)

	// Install town with beads (gt install handles dolt init + start)
	configureDoltIdentity(t, env)
	runGTCmd(t, gtBinary, tmpDir, env, "install", hqPath, "--name", "test-town", "--git")
	t.Cleanup(func() {
		cmd := exec.Command(gtBinary, "down")
		cmd.Dir = hqPath
		cmd.Env = env
		_ = cmd.Run()
	})

	// --- Fake Claude Code credentials ---
	// Claude Code requires OAuth creds to skip the login prompt.
	// We create a fake .credentials.json and point to it via accounts.json.
	// LiteLLM ignores the auth token and uses its own OpenRouter key.
	claudeConfigDir := filepath.Join(tmpDir, ".claude-litellm")
	if err := os.MkdirAll(claudeConfigDir, 0755); err != nil {
		t.Fatalf("mkdir claude config: %v", err)
	}
	credJSON := `{"claudeAiOauth":{"accessToken":"sk-ant-fake","refreshToken":"sk-ant-fake","expiresAt":4102444800000,"scopes":["user:inference"],"subscriptionType":"max","rateLimitTier":"default_claude_max_20x"}}`
	if err := os.WriteFile(filepath.Join(claudeConfigDir, ".credentials.json"), []byte(credJSON), 0644); err != nil {
		t.Fatalf("write credentials: %v", err)
	}

	// Write accounts.json so gastown resolves CLAUDE_CONFIG_DIR for the polecat
	mayorDir := filepath.Join(hqPath, "mayor")
	if err := os.MkdirAll(mayorDir, 0755); err != nil {
		t.Fatalf("mkdir mayor: %v", err)
	}
	accountsCfg := &config.AccountsConfig{
		Version: config.CurrentAccountsVersion,
		Accounts: map[string]config.Account{
			"litellm": {
				Email:       "test@example.com",
				Description: "LiteLLM test account",
				ConfigDir:   claudeConfigDir,
			},
		},
		Default: "litellm",
	}
	accountsPath := filepath.Join(mayorDir, "accounts.json")
	if err := config.SaveAccountsConfig(accountsPath, accountsCfg); err != nil {
		t.Fatalf("save accounts.json: %v", err)
	}

	// --- TownSettings: agent command + env vars ---
	// ANTHROPIC_BASE_URL routes API calls to LiteLLM proxy.
	// --model selects which Claude model name (LiteLLM routes to cheap backend).
	// --dangerously-skip-permissions skips interactive prompts.
	settingsPath := filepath.Join(hqPath, "settings", "config.json")
	settingsData, err := os.ReadFile(settingsPath)
	var settings config.TownSettings
	if err == nil {
		_ = json.Unmarshal(settingsData, &settings)
	} else {
		settings = *config.NewTownSettings()
	}
	settings.Agents["claude"] = &config.RuntimeConfig{
		Command: "claude",
		Args:    []string{"--model", model, "--dangerously-skip-permissions"},
		Env: map[string]string{
			"ANTHROPIC_BASE_URL": "http://localhost:4000",
		},
	}
	settings.DefaultAgent = "claude"
	if err := config.SaveTownSettings(settingsPath, &settings); err != nil {
		t.Fatalf("save town settings: %v", err)
	}
	t.Logf("Agent: claude --model %s, ANTHROPIC_BASE_URL=http://localhost:4000, credentials=%s", model, claudeConfigDir)

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
// The session name is constructed from the sling output ("Allocated polecat: <name>")
// and the rig prefix (e.g. "tr"), giving "tr-<name>" (e.g. "tr-rust").
func slingToRig(t *testing.T, gtBinary, hqPath string, env []string, beadID, rigName, rigPrefix string) string {
	t.Helper()

	slingCmd := exec.Command(gtBinary, "sling", beadID, rigName,
		"--var", "setup_command=",
		"--var", "typecheck_command=",
		"--var", "lint_command=",
		"--var", "build_command=",
	)
	slingCmd.Dir = hqPath
	slingCmd.Env = env
	slingOut, err := slingCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gt sling %s %s failed: %v\n%s", beadID, rigName, err, slingOut)
	}
	t.Logf("Sling output:\n%s", slingOut)

	// Extract polecat name from sling output
	var polecatName string
	for _, line := range strings.Split(string(slingOut), "\n") {
		if strings.Contains(line, "Allocated polecat:") {
			parts := strings.Split(line, "Allocated polecat: ")
			if len(parts) == 2 {
				polecatName = strings.TrimSpace(parts[1])
			}
		}
	}
	if polecatName == "" {
		t.Fatalf("no polecat name found in sling output")
	}

	// Polecat session name = <rigPrefix>-<polecatName> (e.g. "tr-rust")
	sessionName := rigPrefix + "-" + polecatName

	// Wait for session to appear
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if exec.Command("tmux", "has-session", "-t", sessionName).Run() == nil {
			t.Logf("Found session: %s (polecat: %s)", sessionName, polecatName)
			return sessionName
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("tmux session %q not found after sling", sessionName)
	return ""
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
	hqPath, env := setupAgentTestTown(t, gtBinary, modelCheap)

	// Create a test bead to sling
	beadID := "hq-test1"
	createTestBead(t, hqPath, env, beadID, "Test agent spawn task")

	// Sling bead to rig — auto-spawns a polecat with Claude Code
	sessionName := slingToRig(t, gtBinary, hqPath, env, beadID, "testrig", "tr")
	t.Cleanup(func() {
		captureAllSessions(t, t.Name())
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

// captureFullPane captures the full scrollback history from a tmux pane.
func captureFullPane(sessionName string) string {
	// Capture full scrollback into a tmux buffer, then dump it
	capCmd := exec.Command("tmux", "capture-pane", "-t", sessionName, "-p", "-S", "-")
	out, err := capCmd.Output()
	if err != nil {
		return fmt.Sprintf("(capture failed: %v)", err)
	}
	return string(out)
}

// captureAllSessions captures full scrollback from all tmux sessions matching
// the test's rig prefix and saves them to files for CI artifact upload.
//
// Output structure: /tmp/e2e-agent-sessions/<testName>/<sessionName>.log
func captureAllSessions(t *testing.T, testName string) {
	t.Helper()
	outDir := filepath.Join(os.TempDir(), "e2e-agent-sessions", testName)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Logf("WARNING: failed to create session capture dir %s: %v", outDir, err)
		return
	}

	listCmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}")
	listOut, err := listCmd.Output()
	if err != nil {
		t.Logf("WARNING: no tmux sessions to capture")
		return
	}

	var captured []string
	for _, session := range strings.Split(strings.TrimSpace(string(listOut)), "\n") {
		session = strings.TrimSpace(session)
		if session == "" {
			continue
		}
		// Capture sessions belonging to the test rig (tr-* and gt-testrig-*)
		if !strings.HasPrefix(session, "tr-") && !strings.HasPrefix(session, "gt-testrig-") {
			continue
		}

		fullLog := captureFullPane(session)
		logPath := filepath.Join(outDir, session+".log")
		if err := os.WriteFile(logPath, []byte(fullLog), 0644); err != nil {
			t.Logf("WARNING: failed to save session %s: %v", session, err)
			continue
		}
		captured = append(captured, session)

		// Log last 30 non-empty lines for quick debugging in test output
		lines := strings.Split(fullLog, "\n")
		start := len(lines) - 30
		if start < 0 {
			start = 0
		}
		var tail []string
		for _, line := range lines[start:] {
			if strings.TrimSpace(line) != "" {
				tail = append(tail, line)
			}
		}
		t.Logf("=== Session %s (%d bytes, %d lines) ===\n%s\n=== end %s ===",
			session, len(fullLog), len(lines), strings.Join(tail, "\n"), session)
	}

	if len(captured) > 0 {
		t.Logf("Session logs saved to %s/ (%d sessions: %s)",
			outDir, len(captured), strings.Join(captured, ", "))
	}
}

// TestAgentSlingTextChange verifies the full agent work pipeline end-to-end:
// sling a task → model creates a file → file is committed to the polecat branch.
//
// This is the strongest e2e test: it proves the model receives work instructions
// (via beacon + gt prime), executes tool calls through LiteLLM, and produces a
// committed git change — touching every layer of the pipeline.
func TestAgentSlingTextChange(t *testing.T) {
	gtBinary := buildGT(t)
	hqPath, env := setupAgentTestTown(t, gtBinary, modelSmart)

	// Create bead with explicit, verifiable instructions.
	// The model sees this as the Title in "gt prime --hook" output.
	beadID := "hq-sling1"
	createTestBead(t, hqPath, env, beadID,
		"Create and commit a file named test-output.txt containing exactly: e2e-agent-test-ok")

	// Sling to rig — auto-spawns polecat, sends beacon with "Run gt prime --hook"
	sessionName := slingToRig(t, gtBinary, hqPath, env, beadID, "testrig", "tr")
	t.Cleanup(func() {
		captureAllSessions(t, t.Name())
		_ = exec.Command("tmux", "kill-session", "-t", sessionName).Run()
	})

	// Wait for agent ready
	waitForReady(t, sessionName, 90*time.Second)

	// Find the polecat worktree where the model works
	polecatName := strings.TrimPrefix(sessionName, "tr-")
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

// TestHookVisibility is a diagnostic test verifying that a hooked bead is
// discoverable from the polecat's worktree. This tests the same codepath as
// findAgentWork() in gt prime --hook.
//
// If this test fails, it means gt prime --hook will tell the model
// "NO WORK on your hook" even though work was slung, explaining why
// models (regardless of capability) immediately run gt done and exit.
func TestHookVisibility(t *testing.T) {
	gtBinary := buildGT(t)
	hqPath, env := setupAgentTestTown(t, gtBinary, modelCheap)

	beadID := "hq-hook1"
	createTestBead(t, hqPath, env, beadID, "Test hook visibility")

	// Sling and extract polecat name from output (raw bead, no formula needed for diagnostic)
	slingCmd := exec.Command(gtBinary, "sling", beadID, "testrig", "--hook-raw-bead")
	slingCmd.Dir = hqPath
	slingCmd.Env = env
	slingOut, err := slingCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gt sling failed: %v\n%s", err, slingOut)
	}
	t.Logf("Sling output:\n%s", slingOut)

	// Extract polecat name
	var polecatName string
	for _, line := range strings.Split(string(slingOut), "\n") {
		if strings.Contains(line, "Allocated polecat:") {
			parts := strings.Split(line, "Allocated polecat: ")
			if len(parts) == 2 {
				polecatName = strings.TrimSpace(parts[1])
			}
		}
	}
	if polecatName == "" {
		t.Fatalf("no polecat name found in sling output")
	}
	t.Logf("Polecat: %s", polecatName)

	// Capture sessions and kill only those spawned by this test
	t.Cleanup(func() {
		captureAllSessions(t, t.Name())
		listCmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}")
		if out, err := listCmd.Output(); err == nil {
			for _, s := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				if strings.HasPrefix(s, "gt-testrig-") || strings.HasPrefix(s, "tr-") {
					_ = exec.Command("tmux", "kill-session", "-t", s).Run()
				}
			}
		}
	})

	// Construct worktree path directly
	worktree := polecatWorktree(t, hqPath, "testrig", polecatName)
	t.Logf("Worktree: %s", worktree)

	agentID := fmt.Sprintf("testrig/polecats/%s", polecatName)

	// === DIAGNOSTIC CHECKS ===

	// Sync beads database after sling modified the JSONL
	syncCmd := exec.Command("bd", "sync", "--import-only")
	syncCmd.Dir = hqPath
	syncCmd.Env = env
	if syncOut, err := syncCmd.CombinedOutput(); err != nil {
		t.Logf("bd sync --import-only failed: %v\n%s", err, syncOut)
	}

	// 1. Verify bead status from HQ
	t.Log("--- Check 1: bd show from HQ ---")
	showCmd := exec.Command("bd", "show", beadID, "--json")
	showCmd.Dir = hqPath
	showCmd.Env = env
	showOut, err := showCmd.CombinedOutput()
	if err != nil {
		t.Logf("bd show (from HQ) FAILED: %v\n%s", err, showOut)
	} else {
		t.Logf("Bead from HQ:\n%s", showOut)
	}

	// 2. Verify bead status from polecat worktree
	t.Log("--- Check 2: bd show from worktree ---")
	showCmd2 := exec.Command("bd", "show", beadID, "--json")
	showCmd2.Dir = worktree
	showCmd2.Env = env
	showOut2, err := showCmd2.CombinedOutput()
	if err != nil {
		t.Logf("bd show (from worktree) FAILED: %v\n%s", err, showOut2)
	} else {
		t.Logf("Bead from worktree:\n%s", showOut2)
	}

	// 3. List hooked beads from HQ
	t.Log("--- Check 3: bd list --status=hooked from HQ ---")
	listCmd := exec.Command("bd", "list", "--status=hooked", "--json")
	listCmd.Dir = hqPath
	listCmd.Env = env
	listOut, err := listCmd.CombinedOutput()
	if err != nil {
		t.Logf("FAILED: %v\n%s", err, listOut)
	} else {
		t.Logf("Hooked beads from HQ:\n%s", listOut)
	}

	// 4. List hooked beads from worktree
	t.Log("--- Check 4: bd list --status=hooked from worktree ---")
	listCmd2 := exec.Command("bd", "list", "--status=hooked", "--json")
	listCmd2.Dir = worktree
	listCmd2.Env = env
	listOut2, err := listCmd2.CombinedOutput()
	if err != nil {
		t.Logf("FAILED: %v\n%s", err, listOut2)
	} else {
		t.Logf("Hooked beads from worktree:\n%s", listOut2)
	}

	// 5. List hooked beads with assignee from worktree
	t.Logf("--- Check 5: bd list --status=hooked --assignee=%s from worktree ---", agentID)
	listCmd3 := exec.Command("bd", "list", "--status=hooked", "--assignee="+agentID, "--json")
	listCmd3.Dir = worktree
	listCmd3.Env = env
	listOut3, err := listCmd3.CombinedOutput()
	if err != nil {
		t.Logf("FAILED: %v\n%s", err, listOut3)
	} else {
		t.Logf("Result:\n%s", listOut3)
	}

	// 6. Check .beads/redirect in worktree
	t.Log("--- Check 6: .beads/redirect in worktree ---")
	redirectPath := filepath.Join(worktree, ".beads", "redirect")
	if content, err := os.ReadFile(redirectPath); err != nil {
		t.Logf(".beads/redirect NOT FOUND: %v", err)
	} else {
		t.Logf(".beads/redirect: %s", strings.TrimSpace(string(content)))
	}

	// 7. Run gt prime --dry-run from worktree
	t.Log("--- Check 7: gt prime --dry-run from worktree ---")
	primeCmd := exec.Command(gtBinary, "prime", "--dry-run")
	primeCmd.Dir = worktree
	primeCmd.Env = env
	primeOut, err := primeCmd.CombinedOutput()
	if err != nil {
		t.Logf("gt prime --dry-run FAILED: %v\n%s", err, primeOut)
	} else {
		output := string(primeOut)
		if strings.Contains(output, "AUTONOMOUS WORK MODE") {
			t.Logf("GOOD: gt prime found AUTONOMOUS WORK MODE")
		} else if strings.Contains(output, "NO WORK on your hook") {
			t.Errorf("BUG: gt prime says 'NO WORK on your hook' even though bead %s was slung to %s", beadID, agentID)
		}
		// Show relevant sections only
		for _, line := range strings.Split(output, "\n") {
			if strings.Contains(line, "AUTONOMOUS") || strings.Contains(line, "NO WORK") ||
				strings.Contains(line, "STARTUP PROTOCOL") || strings.Contains(line, "hooked") ||
				strings.Contains(line, "hook") || strings.Contains(line, "Bead ID") {
				t.Logf("  prime> %s", line)
			}
		}
		t.Logf("Full gt prime output: %d bytes", len(primeOut))
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
	hqPath, env := setupAgentTestTown(t, gtBinary, modelCheap)

	// Create and sling a bead
	beadID := "hq-test2"
	createTestBead(t, hqPath, env, beadID, "Test agent done task")
	sessionName := slingToRig(t, gtBinary, hqPath, env, beadID, "testrig", "tr")

	// Wait for agent to be ready before nuking
	waitForReady(t, sessionName, 90*time.Second)

	// Capture sessions before nuke destroys them
	captureAllSessions(t, t.Name())

	// Extract polecat name from session name (format: gt-<rig>-<polecat>)
	polecatName := strings.TrimPrefix(sessionName, "tr-")
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
