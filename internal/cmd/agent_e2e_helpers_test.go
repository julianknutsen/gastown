//go:build e2e_agent

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeJSONFile marshals v as indented JSON and writes to path, creating parent dirs.
func writeJSONFile(t *testing.T, path string, v any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal JSON for %s: %v", path, err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// readJSONFile reads and unmarshals a JSON file into dest.
func readJSONFile(t *testing.T, path string, dest any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, dest); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
}

// pollUntil calls check every interval until it returns true or timeout expires.
// Returns true if check succeeded, false on timeout.
func pollUntil(t *testing.T, desc string, timeout, interval time.Duration, check func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return true
		}
		time.Sleep(interval)
	}
	return false
}

// pollUntilFatal is like pollUntil but calls t.Fatalf on timeout.
func pollUntilFatal(t *testing.T, desc string, timeout, interval time.Duration, check func() bool) {
	t.Helper()
	if !pollUntil(t, desc, timeout, interval, check) {
		t.Fatalf("%s: timed out after %s", desc, timeout)
	}
}

// extractPolecatName parses "Allocated polecat: <name>" from sling output.
func extractPolecatName(t *testing.T, slingOutput string) string {
	t.Helper()
	for _, line := range strings.Split(slingOutput, "\n") {
		if strings.Contains(line, "Allocated polecat:") {
			parts := strings.Split(line, "Allocated polecat: ")
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	t.Fatalf("no polecat name found in sling output")
	return ""
}

// tryExtractWispID extracts wisp ID from sling output, or "" if not found.
// Matches both "Formula wisp created: <id>" and "Wisp created: <id>".
func tryExtractWispID(slingOutput string) string {
	for _, line := range strings.Split(slingOutput, "\n") {
		if strings.Contains(line, "isp created:") {
			parts := strings.Split(line, "isp created: ")
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

// --- tmux wrappers ---

// tmuxHasSession returns true if the named tmux session exists.
func tmuxHasSession(sessionName string) bool {
	return exec.Command("tmux", "has-session", "-t", sessionName).Run() == nil
}

// tmuxCapture returns the visible pane content for a session.
func tmuxCapture(sessionName string) (string, error) {
	out, err := exec.Command("tmux", "capture-pane", "-t", sessionName, "-p").Output()
	return string(out), err
}

// tmuxCaptureScrollback returns full scrollback history for a session.
func tmuxCaptureScrollback(sessionName string) (string, error) {
	out, err := exec.Command("tmux", "capture-pane", "-t", sessionName, "-p", "-S", "-").Output()
	return string(out), err
}

// tmuxSendKeys sends keys to a tmux session.
func tmuxSendKeys(sessionName string, keys ...string) error {
	args := append([]string{"send-keys", "-t", sessionName}, keys...)
	return exec.Command("tmux", args...).Run()
}

// tmuxKillSession kills a tmux session, ignoring errors if already gone.
func tmuxKillSession(sessionName string) {
	_ = exec.Command("tmux", "kill-session", "-t", sessionName).Run()
}

// tmuxListSessions returns all tmux session names, or nil on error.
func tmuxListSessions() []string {
	out, err := exec.Command("tmux", "list-sessions", "-F", "#{session_name}").Output()
	if err != nil {
		return nil
	}
	var sessions []string
	for _, s := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if s = strings.TrimSpace(s); s != "" {
			sessions = append(sessions, s)
		}
	}
	return sessions
}

// --- Session filtering and cleanup ---

// isTestSession returns true if the session name matches the test rig prefix patterns.
func isTestSession(name string) bool {
	return strings.HasPrefix(name, "tr-") || strings.HasPrefix(name, "gt-testrig-")
}

// killAllTestSessions kills all tmux sessions matching isTestSession.
func killAllTestSessions(t *testing.T) {
	t.Helper()
	for _, s := range tmuxListSessions() {
		if isTestSession(s) {
			tmuxKillSession(s)
			t.Logf("Killed session: %s", s)
		}
	}
}

// cleanupTestSessions captures all test sessions then kills them.
// Standard cleanup to register in every test.
func cleanupTestSessions(t *testing.T) {
	t.Helper()
	captureAllSessions(t, t.Name())
	killAllTestSessions(t)
}

// waitForSession polls until a tmux session appears (10s timeout).
func waitForSession(t *testing.T, sessionName string) {
	t.Helper()
	pollUntilFatal(t, fmt.Sprintf("session %s", sessionName), 10*time.Second, 500*time.Millisecond, func() bool {
		return tmuxHasSession(sessionName)
	})
}

// --- Unified sling helper ---

// slingResult holds the parsed output from a gt sling command.
type slingResult struct {
	SessionName string // tmux session: <rigPrefix>-<polecatName>
	PolecatName string // e.g., "rust"
	WispID      string // only set when formula sling produces one
}

// slingAndWait slings a bead to a rig, extracts the polecat name (and optionally wisp ID),
// and waits for the tmux session to appear. Replaces both slingToRig and slingFormulaToRig.
func slingAndWait(t *testing.T, gtBinary, hqPath string, env []string,
	beadID, rigName, rigPrefix string) slingResult {
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

	output := string(slingOut)
	polecatName := extractPolecatName(t, output)
	sessionName := rigPrefix + "-" + polecatName

	// Extract wisp ID if present (formula slings produce one)
	var wispID string
	if w := tryExtractWispID(output); w != "" {
		wispID = w
		t.Logf("Wisp ID: %s", wispID)
	}

	waitForSession(t, sessionName)
	t.Logf("Found session: %s (polecat: %s)", sessionName, polecatName)

	return slingResult{
		SessionName: sessionName,
		PolecatName: polecatName,
		WispID:      wispID,
	}
}
