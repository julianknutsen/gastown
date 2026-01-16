package beads_test

import (
	"fmt"
	"testing"

	"github.com/steveyegge/gastown/internal/beads"
)

func TestMatrix_Dependencies(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Dependencies",
		Operation: "AddDependency",
		Test: func(ops beads.BeadsOps) error {
			// Create two issues
			parent, err := ops.Create(beads.CreateOptions{Title: "Parent", Type: "task"})
			if err != nil {
				return fmt.Errorf("Create parent failed: %v", err)
			}
			child, err := ops.Create(beads.CreateOptions{Title: "Child", Type: "task"})
			if err != nil {
				return fmt.Errorf("Create child failed: %v", err)
			}

			// Add dependency: child depends on parent
			err = ops.AddDependency(child.ID, parent.ID)
			if err != nil {
				return fmt.Errorf("AddDependency failed: %v", err)
			}

			// Child should be blocked (parent not closed)
			blocked, err := ops.Blocked()
			if err != nil {
				return fmt.Errorf("Blocked failed: %v", err)
			}
			found := false
			for _, b := range blocked {
				if b.ID == child.ID {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("Child should be in blocked list")
			}

			// Parent should be ready (no dependencies)
			ready, err := ops.Ready()
			if err != nil {
				return fmt.Errorf("Ready failed: %v", err)
			}
			found = false
			for _, r := range ready {
				if r.ID == parent.ID {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("Parent should be in ready list")
			}

			// Close parent, child should become ready
			_ = ops.Close(parent.ID)
			ready, err = ops.Ready()
			if err != nil {
				return fmt.Errorf("Ready after close failed: %v", err)
			}
			found = false
			for _, r := range ready {
				if r.ID == child.ID {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("Child should be ready after parent is closed")
			}
			return nil
		},
	})
}

func TestMatrix_Ready_EmptyRepo(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Ready_EmptyRepo",
		Operation: "Ready",
		Test: func(ops beads.BeadsOps) error {
			ready, err := ops.Ready()
			if err != nil {
				return fmt.Errorf("Ready on empty repo failed: %v", err)
			}
			if len(ready) != 0 {
				return fmt.Errorf("Ready on empty repo returned %d issues, want 0", len(ready))
			}
			return nil
		},
	})
}

func TestMatrix_Ready_ExcludesBlocked(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Ready_ExcludesBlocked",
		Operation: "Ready",
		Test: func(ops beads.BeadsOps) error {
			// Create dependency chain: blocker <- blocked
			blocker, _ := ops.Create(beads.CreateOptions{Title: "Blocker", Type: "task"})
			blocked, _ := ops.Create(beads.CreateOptions{Title: "Blocked", Type: "task"})
			independent, _ := ops.Create(beads.CreateOptions{Title: "Independent", Type: "task"})
			_ = ops.AddDependency(blocked.ID, blocker.ID)

			ready, err := ops.Ready()
			if err != nil {
				return fmt.Errorf("Ready failed: %v", err)
			}

			// Check results
			foundBlocker := false
			foundIndependent := false
			foundBlocked := false
			for _, r := range ready {
				switch r.ID {
				case blocker.ID:
					foundBlocker = true
				case independent.ID:
					foundIndependent = true
				case blocked.ID:
					foundBlocked = true
				}
			}

			if !foundBlocker {
				return fmt.Errorf("Blocker should be in Ready results")
			}
			if !foundIndependent {
				return fmt.Errorf("Independent should be in Ready results")
			}
			if foundBlocked {
				return fmt.Errorf("Blocked should NOT be in Ready results")
			}
			return nil
		},
	})
}

func TestMatrix_Blocked_EmptyRepo(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Blocked_EmptyRepo",
		Operation: "Blocked",
		Test: func(ops beads.BeadsOps) error {
			blocked, err := ops.Blocked()
			if err != nil {
				return fmt.Errorf("Blocked on empty repo failed: %v", err)
			}
			if len(blocked) != 0 {
				return fmt.Errorf("Blocked on empty repo returned %d issues, want 0", len(blocked))
			}
			return nil
		},
	})
}

func TestMatrix_Blocked_WithOpenDependency(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Blocked_WithOpenDependency",
		Operation: "Blocked",
		Test: func(ops beads.BeadsOps) error {
			// Create issues with dependency
			blocker, _ := ops.Create(beads.CreateOptions{Title: "Blocker", Type: "task"})
			blocked, _ := ops.Create(beads.CreateOptions{Title: "Blocked", Type: "task"})
			_ = ops.AddDependency(blocked.ID, blocker.ID)

			blockedList, err := ops.Blocked()
			if err != nil {
				return fmt.Errorf("Blocked failed: %v", err)
			}

			found := false
			for _, b := range blockedList {
				if b.ID == blocked.ID {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("Issue with open dependency should be in Blocked list")
			}
			return nil
		},
	})
}

func TestMatrix_AddDependencyWithType(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "AddDependencyWithType_Tracks",
		Operation: "AddDependencyWithType",
		Test: func(ops beads.BeadsOps) error {
			// Create two issues
			tracker, err := ops.Create(beads.CreateOptions{Title: "Tracker", Type: "task"})
			if err != nil {
				return fmt.Errorf("Create tracker failed: %v", err)
			}
			tracked, err := ops.Create(beads.CreateOptions{Title: "Tracked", Type: "task"})
			if err != nil {
				return fmt.Errorf("Create tracked failed: %v", err)
			}

			// Add "tracks" dependency: tracker tracks tracked
			// This should be NON-BLOCKING - used for tracking without blocking
			err = ops.AddDependencyWithType(tracker.ID, tracked.ID, "tracks")
			if err != nil {
				return fmt.Errorf("AddDependencyWithType tracks failed: %v", err)
			}

			// Verify tracker is NOT blocked (tracks is non-blocking)
			blocked, err := ops.Blocked()
			if err != nil {
				return fmt.Errorf("Blocked failed: %v", err)
			}
			for _, b := range blocked {
				if b.ID == tracker.ID {
					return fmt.Errorf("Tracker should NOT be blocked - 'tracks' is non-blocking")
				}
			}

			// Verify tracker is still in ready list
			ready, err := ops.Ready()
			if err != nil {
				return fmt.Errorf("Ready failed: %v", err)
			}
			foundInReady := false
			for _, r := range ready {
				if r.ID == tracker.ID {
					foundInReady = true
					break
				}
			}
			if !foundInReady {
				return fmt.Errorf("Tracker should be in Ready list - 'tracks' is non-blocking")
			}

			return nil
		},
	})
}

func TestMatrix_AddDependencyWithType_DependsOn(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "AddDependencyWithType_DependsOn",
		Operation: "AddDependencyWithType",
		Test: func(ops beads.BeadsOps) error {
			// Create two issues
			child, err := ops.Create(beads.CreateOptions{Title: "Child", Type: "task"})
			if err != nil {
				return fmt.Errorf("Create child failed: %v", err)
			}
			parent, err := ops.Create(beads.CreateOptions{Title: "Parent", Type: "task"})
			if err != nil {
				return fmt.Errorf("Create parent failed: %v", err)
			}

			// Add "depends-on" dependency: child depends on parent
			// NOTE: "depends-on" is NON-BLOCKING (like "tracks")
			// Only "blocks" type (default for dep add without --type) is blocking
			err = ops.AddDependencyWithType(child.ID, parent.ID, "depends-on")
			if err != nil {
				return fmt.Errorf("AddDependencyWithType depends-on failed: %v", err)
			}

			// Verify child is NOT blocked (depends-on is non-blocking)
			blocked, err := ops.Blocked()
			if err != nil {
				return fmt.Errorf("Blocked failed: %v", err)
			}
			for _, b := range blocked {
				if b.ID == child.ID {
					return fmt.Errorf("Child should NOT be blocked - 'depends-on' is non-blocking")
				}
			}

			// Verify child is still in ready list
			ready, err := ops.Ready()
			if err != nil {
				return fmt.Errorf("Ready failed: %v", err)
			}
			foundInReady := false
			for _, r := range ready {
				if r.ID == child.ID {
					foundInReady = true
					break
				}
			}
			if !foundInReady {
				return fmt.Errorf("Child should be in Ready list - 'depends-on' is non-blocking")
			}

			return nil
		},
	})
}
