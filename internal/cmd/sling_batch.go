package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/steveyegge/gastown/internal/style"
)

// runBatchSling handles slinging multiple beads to a rig.
// Each bead gets its own freshly spawned polecat.
func runBatchSling(beadIDs []string, rigName string, townBeadsDir string) error {
	// Validate all beads exist before spawning any polecats
	for _, beadID := range beadIDs {
		if err := verifyBeadExists(beadID); err != nil {
			return fmt.Errorf("bead '%s' not found", beadID)
		}
	}

	if slingDryRun {
		fmt.Printf("%s Batch slinging %d beads to rig '%s':\n", style.Bold.Render("🎯"), len(beadIDs), rigName)
		for _, beadID := range beadIDs {
			fmt.Printf("  Would spawn polecat for: %s\n", beadID)
		}
		return nil
	}

	fmt.Printf("%s Batch slinging %d beads to rig '%s'...\n", style.Bold.Render("🎯"), len(beadIDs), rigName)

	// Track results for summary
	type slingResult struct {
		beadID  string
		polecat string
		success bool
		errMsg  string
	}
	results := make([]slingResult, 0, len(beadIDs))

	townRoot := filepath.Dir(townBeadsDir)

	// Spawn a polecat for each bead and sling it
	for i, beadID := range beadIDs {
		fmt.Printf("\n[%d/%d] Slinging %s...\n", i+1, len(beadIDs), beadID)

		spawnInfo, err := slingBeadToPolecat(beadID, rigName, SlingBeadOptions{
			Force:    slingForce,
			Account:  slingAccount,
			Create:   slingCreate,
			Agent:    slingAgent,
			NoConvoy: slingNoConvoy,
			Args:     slingArgs,
			Subject:  slingSubject,
			TownRoot: townRoot,
			BeadsDir: townBeadsDir,
		})
		if err != nil {
			polecatName := ""
			if spawnInfo != nil {
				polecatName = spawnInfo.PolecatName
			}
			results = append(results, slingResult{beadID: beadID, polecat: polecatName, success: false, errMsg: err.Error()})
			fmt.Printf("  %s %v\n", style.Dim.Render("✗"), err)
			continue
		}

		results = append(results, slingResult{beadID: beadID, polecat: spawnInfo.PolecatName, success: true})
	}

	// Wake witness and refinery once at the end
	wakeRigAgents(rigName)

	// Print summary
	successCount := 0
	for _, r := range results {
		if r.success {
			successCount++
		}
	}

	fmt.Printf("\n%s Batch sling complete: %d/%d succeeded\n", style.Bold.Render("📊"), successCount, len(beadIDs))
	if successCount < len(beadIDs) {
		for _, r := range results {
			if !r.success {
				fmt.Printf("  %s %s: %s\n", style.Dim.Render("✗"), r.beadID, r.errMsg)
			}
		}
	}

	return nil
}
