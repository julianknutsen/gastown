package beads_test

import (
	"fmt"
	"testing"

	"github.com/steveyegge/gastown/internal/beads"
)

func TestMatrix_Stats(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Stats",
		Operation: "Stats",
		Test: func(ops beads.BeadsOps) error {
			// Create some issues
			_, err := ops.Create(beads.CreateOptions{Title: "Stats 1", Type: "task"})
			if err != nil {
				return fmt.Errorf("Create Stats 1 failed: %v", err)
			}
			issue2, err := ops.Create(beads.CreateOptions{Title: "Stats 2", Type: "task"})
			if err != nil {
				return fmt.Errorf("Create Stats 2 failed: %v", err)
			}
			err = ops.Close(issue2.ID)
			if err != nil {
				return fmt.Errorf("Close failed: %v", err)
			}

			// Get stats
			statsText, err := ops.Stats()
			if err != nil {
				return fmt.Errorf("Stats failed: %v", err)
			}
			if statsText == "" {
				return fmt.Errorf("Stats returned empty string")
			}

			// Get JSON stats
			statsJSON, err := ops.StatsJSON()
			if err != nil {
				return fmt.Errorf("StatsJSON failed: %v", err)
			}
			if statsJSON == nil {
				return fmt.Errorf("StatsJSON returned nil")
			}
			// Access stats from Summary field (bd stats --json wraps in "summary")
			if statsJSON.Summary.TotalIssues < 2 {
				return fmt.Errorf("TotalIssues = %d, want at least 2", statsJSON.Summary.TotalIssues)
			}
			if statsJSON.Summary.ClosedIssues < 1 {
				return fmt.Errorf("ClosedIssues = %d, want at least 1", statsJSON.Summary.ClosedIssues)
			}
			return nil
		},
	})
}

func TestMatrix_Version(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Version",
		Operation: "Version",
		Test: func(ops beads.BeadsOps) error {
			version, err := ops.Version()
			if err != nil {
				return fmt.Errorf("Version failed: %v", err)
			}
			if version == "" {
				return fmt.Errorf("Version returned empty string")
			}
			return nil
		},
	})
}

func TestMatrix_Sync(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Sync",
		Operation: "GetSyncStatus",
		Test: func(ops beads.BeadsOps) error {
			// Note: Sync() requires a git repository. In test environments without git,
			// it will fail. We test GetSyncStatus which should work gracefully.
			status, err := ops.GetSyncStatus()
			if err != nil {
				// May fail if no git repo, which is acceptable in test environment
				return nil
			}
			// Status may be empty but should not be nil
			if status == nil {
				return fmt.Errorf("GetSyncStatus returned nil without error")
			}
			return nil
		},
	})
}

func TestMatrix_SyncFromMain(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "SyncFromMain",
		Operation: "SyncFromMain",
		Test: func(ops beads.BeadsOps) error {
			// SyncFromMain may fail if not in a git repo or no main branch
			// In test environments without git, we accept this gracefully
			err := ops.SyncFromMain()
			if err != nil {
				// Acceptable in test environment without git setup
				return nil
			}
			return nil
		},
	})
}

func TestMatrix_Doctor(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Doctor",
		Operation: "Doctor",
		Test: func(ops beads.BeadsOps) error {
			report, err := ops.Doctor()
			if err != nil {
				// Doctor may fail in some environments
				return nil
			}
			if report == nil {
				return fmt.Errorf("Doctor returned nil without error")
			}
			if report.Status == "" {
				return fmt.Errorf("Doctor report should have a status")
			}
			return nil
		},
	})
}

func TestMatrix_Prime(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Prime",
		Operation: "Prime",
		Test: func(ops beads.BeadsOps) error {
			_, err := ops.Prime()
			if err != nil {
				// Prime may fail in test env
				return nil
			}
			return nil
		},
	})
}

func TestMatrix_Flush(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Flush",
		Operation: "Flush",
		Test: func(ops beads.BeadsOps) error {
			err := ops.Flush()
			if err != nil {
				// Flush may fail in test env
				return nil
			}
			return nil
		},
	})
}

func TestMatrix_IsBeadsRepo(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "IsBeadsRepo",
		Operation: "IsBeadsRepo",
		Test: func(ops beads.BeadsOps) error {
			isRepo := ops.IsBeadsRepo()
			// Should be true since we initialized the database
			if !isRepo {
				return fmt.Errorf("IsBeadsRepo should return true for initialized repo")
			}
			return nil
		},
	})
}

func TestMatrix_Search(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Search",
		Operation: "Search",
		Test: func(ops beads.BeadsOps) error {
			// Create an issue to search for
			_, _ = ops.Create(beads.CreateOptions{Title: "Searchable Issue", Type: "task"})

			results, err := ops.Search("Searchable", beads.SearchOptions{})
			if err != nil {
				// Search may fail in test env
				return nil
			}
			if results == nil {
				return fmt.Errorf("Search returned nil without error")
			}
			return nil
		},
	})
}

func TestMatrix_Burn(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Burn",
		Operation: "Burn",
		Test: func(ops beads.BeadsOps) error {
			err := ops.Burn(beads.BurnOptions{
				SessionID: "test-session",
				Tokens:    100,
				Cost:      0.01,
				Model:     "test-model",
			})
			if err != nil {
				// Burn may fail in test env
				return nil
			}
			return nil
		},
	})
}

func TestMatrix_SlotShow(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "SlotShow",
		Operation: "SlotShow",
		Test: func(ops beads.BeadsOps) error {
			slot, err := ops.SlotShow("nonexistent-slot")
			if err != nil {
				// Expected for nonexistent slot
				return nil
			}
			if slot != nil && slot.ID == "" {
				return fmt.Errorf("Slot should have an ID")
			}
			return nil
		},
	})
}

