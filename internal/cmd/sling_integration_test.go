//go:build integration

// Package cmd contains integration tests for gt sling formula-on-bead functionality.
//
// Run with: go test -tags=integration ./internal/cmd -run TestSling -v
package cmd

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestSlingFormulaOnRigBead verifies that gt sling <formula> --on <rig-bead> <rig>
// creates the wisp in the rig's beads database and successfully bonds it to the bead.
//
// This test caught the bug where wisp was created in town beads (hq-*) instead of
// rig beads, causing bd mol bond to fail with "not found" error.
//
// The fix in sling.go uses BEADS_DIR env var to direct wisp creation to rig beads.
func TestSlingFormulaOnRigBead(t *testing.T) {
	// Skip if bd is not available
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd not installed, skipping test")
	}

	// Build gt binary
	gtBinary := buildGT(t)

	// Set up town using gt install (real codepath)
	townRoot := t.TempDir()
	installCmd := exec.Command(gtBinary, "install", townRoot, "--name", "test-town")
	installCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	if output, err := installCmd.CombinedOutput(); err != nil {
		t.Fatalf("gt install failed: %v\nOutput: %s", err, output)
	}

	// Create test git repo
	gitURL := createTestGitRepo(t, "slingtest")

	// Add rig using gt rig add (real codepath - handles routes and config)
	rigAddCmd := exec.Command(gtBinary, "rig", "add", "testrig", gitURL, "--prefix", "tr")
	rigAddCmd.Dir = townRoot
	if output, err := rigAddCmd.CombinedOutput(); err != nil {
		t.Fatalf("gt rig add failed: %v\nOutput: %s", err, output)
	}

	rigPath := filepath.Join(townRoot, "testrig")

	// Create formula ONLY in town beads (not in rig beads)
	// This mimics production where formulas are provisioned at install time to town beads.
	// The fix must ensure wisp is created in rig beads while still finding the formula in town.
	testFormula := `description = "Test formula for sling integration test"
formula = "test-formula"
type = "workflow"
version = 1

[[steps]]
id = "step1"
title = "Test step for {{feature}}"
description = "A simple test step"

[vars]
[vars.feature]
description = "The feature name"
required = true
`
	townFormulasDir := filepath.Join(townRoot, ".beads", "formulas")
	if err := os.MkdirAll(townFormulasDir, 0755); err != nil {
		t.Fatalf("mkdir town formulas: %v", err)
	}
	if err := os.WriteFile(filepath.Join(townFormulasDir, "test-formula.formula.toml"), []byte(testFormula), 0644); err != nil {
		t.Fatalf("write town formula: %v", err)
	}

	// Create a bead in rig beads (run from rig root where .beads/ lives)
	createCmd := exec.Command("bd", "--no-daemon", "create",
		"--title", "Test issue for formula sling",
		"--type", "task",
		"--json")
	createCmd.Dir = rigPath
	createOutput, err := createCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bd create failed: %v\nOutput: %s", err, createOutput)
	}

	// Parse bead ID
	var createResult struct {
		ID string `json:"id"`
	}
	outputStr := string(createOutput)
	jsonStart := strings.Index(outputStr, "{")
	if jsonStart == -1 {
		t.Fatalf("no JSON in create output: %s", createOutput)
	}
	if err := json.Unmarshal([]byte(outputStr[jsonStart:]), &createResult); err != nil {
		t.Fatalf("parsing create output: %v\nOutput: %s", err, createOutput)
	}
	beadID := createResult.ID
	t.Logf("Created test bead: %s", beadID)

	// Verify bead has rig prefix
	if !strings.HasPrefix(beadID, "tr-") {
		t.Fatalf("expected bead ID with tr- prefix, got %s", beadID)
	}

	// Run gt sling <formula> --on <beadID> <rig> --naked
	// This is the actual command we're testing!
	slingCmd := exec.Command(gtBinary, "sling", "test-formula", "--on", beadID, "testrig", "--naked")
	slingCmd.Dir = townRoot
	slingOutput, err := slingCmd.CombinedOutput()
	slingOutputStr := string(slingOutput)
	t.Logf("gt sling output:\n%s", slingOutputStr)

	if err != nil {
		// Check for the specific cross-database bond error (the bug we fixed)
		if strings.Contains(slingOutputStr, "not found") && strings.Contains(slingOutputStr, "bond") {
			t.Fatalf("BUG: gt sling --on failed with cross-database bond error.\n"+
				"This happens when wisp is created in town beads (hq-*) but target bead is in rig beads (tr-*).\n"+
				"Fix: Use BEADS_DIR env var to direct wisp creation to rig beads.\n"+
				"Output: %s", slingOutputStr)
		}
		t.Fatalf("gt sling failed: %v\nOutput: %s", err, slingOutputStr)
	}

	// Verify success indicators in output
	if !strings.Contains(slingOutputStr, "Formula wisp created") {
		t.Errorf("expected 'Formula wisp created' in output")
	}
	if !strings.Contains(slingOutputStr, "Formula bonded to") {
		t.Errorf("expected 'Formula bonded to' in output")
	}

	// Verify wisp was created with rig prefix (tr-) not town prefix (hq-)
	if strings.Contains(slingOutputStr, "hq-wisp") {
		t.Errorf("BUG: wisp created with town prefix (hq-) instead of rig prefix (tr-)")
	}

	t.Logf("SUCCESS: gt sling --on created wisp in rig beads and bonded successfully")
}
