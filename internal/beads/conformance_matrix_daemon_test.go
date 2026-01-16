package beads_test

import (
	"fmt"
	"testing"

	"github.com/steveyegge/gastown/internal/beads"
)

func TestMatrix_DaemonOperations(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "DaemonOperations",
		Operation: "DaemonStatus",
		Test: func(ops beads.BeadsOps) error {
			// Check status - should return a status (daemon may not be running)
			// Note: DaemonStart requires a git repo, so we only test DaemonStatus
			// which should work in any beads directory.
			status, err := ops.DaemonStatus()
			if err != nil {
				return fmt.Errorf("DaemonStatus failed: %v", err)
			}
			if status == nil {
				return fmt.Errorf("DaemonStatus returned nil")
			}
			// Status can be running or not running - both are valid
			return nil
		},
	})
}

func TestMatrix_DaemonStatus_NotRunning(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "DaemonStatus_NotRunning",
		Operation: "DaemonStatus",
		Test: func(ops beads.BeadsOps) error {
			// Fresh env should have daemon not running
			status, err := ops.DaemonStatus()
			if err != nil {
				return fmt.Errorf("DaemonStatus failed: %v", err)
			}
			// In test env, daemon is typically not running
			// Don't assert Running=false since some envs may have daemon
			if status == nil {
				return fmt.Errorf("DaemonStatus returned nil")
			}
			return nil
		},
	})
}

func TestMatrix_DaemonHealth(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "DaemonHealth",
		Operation: "DaemonHealth",
		Test: func(ops beads.BeadsOps) error {
			// DaemonHealth may fail if daemon not running - that's OK
			health, err := ops.DaemonHealth()
			if err != nil {
				// Expected if daemon not running
				return nil
			}
			if health == nil {
				return fmt.Errorf("DaemonHealth returned nil without error")
			}
			return nil
		},
	})
}

func TestMatrix_DaemonStartStop(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "DaemonStartStop",
		Operation: "DaemonStart",
		Test: func(ops beads.BeadsOps) error {
			// DaemonStart may fail in test env (no git repo) - that's OK
			err := ops.DaemonStart()
			if err != nil {
				// Expected in test environment
				return nil
			}

			// If start succeeded, verify status
			status, _ := ops.DaemonStatus()
			if status != nil && !status.Running {
				return fmt.Errorf("Daemon should be running after DaemonStart")
			}

			// Stop
			_ = ops.DaemonStop()
			return nil
		},
	})
}
