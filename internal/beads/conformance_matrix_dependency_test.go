package beads_test

import (
	"fmt"
	"testing"

	"github.com/steveyegge/gastown/internal/beads"
)

// TestMatrix_AddDependency_CrossRig tests cross-rig routing for AddDependency.
// The scenario: working from gt rig, add dependency between two issues in ap rig.
// Without routedImpl, bd would fail because dep.go has NO routing support.
//
// This test uses a custom setup to create two issues in the ap rig, then tests
// adding a dependency between them while operating from the gt rig.
func TestMatrix_AddDependency_CrossRig(t *testing.T) {
	if !isBdInstalled() {
		t.Skip("bd not installed")
	}

	env := setupTestEnv(t)
	initRealBd(t, env)

	// Create two issues in the ap rig
	apOps := beads.ForRig(env.AIPlatDir)
	parent, err := apOps.Create(beads.CreateOptions{Title: "CrossRig Parent", Type: "task"})
	if err != nil {
		t.Fatalf("Create parent in ap failed: %v", err)
	}
	child, err := apOps.Create(beads.CreateOptions{Title: "CrossRig Child", Type: "task"})
	if err != nil {
		t.Fatalf("Create child in ap failed: %v", err)
	}

	// Test with Implementation (has routedImpl) - should pass
	t.Run("Implementation/cross-rig", func(t *testing.T) {
		// Implementation is configured for gt rig but has routedImpl to handle ap- prefix
		impl := beads.NewWithTownRoot(env.GastownDir, env.TownRoot)
		err := impl.AddDependency(child.ID, parent.ID)
		if err != nil {
			t.Errorf("Implementation AddDependency failed: %v", err)
		}
	})

	// Test with RawBd (no routedImpl) - should fail for cross-rig
	t.Run("RawBd/cross-rig", func(t *testing.T) {
		// Create new issues for this test to avoid interference
		parent2, _ := apOps.Create(beads.CreateOptions{Title: "CrossRig Parent2", Type: "task"})
		child2, _ := apOps.Create(beads.CreateOptions{Title: "CrossRig Child2", Type: "task"})

		rawBd := &TrueRawBdOps{workDir: env.GastownDir, testHome: env.TestHome}
		err := rawBd.AddDependency(child2.ID, parent2.ID)

		// RawBd should fail because bd has no routing for dep command
		if err == nil {
			t.Log("WARNING: RawBd AddDependency passed in cross-rig context - bd may have been fixed!")
		} else {
			t.Logf("Verified: bd routing bug for AddDependency still exists (%v)", err)
		}
	})
}

// TestMatrix_AddDependencyWithType_CrossRig tests cross-rig routing for AddDependencyWithType.
func TestMatrix_AddDependencyWithType_CrossRig(t *testing.T) {
	if !isBdInstalled() {
		t.Skip("bd not installed")
	}

	env := setupTestEnv(t)
	initRealBd(t, env)

	// Create two issues in the ap rig
	apOps := beads.ForRig(env.AIPlatDir)
	parent, err := apOps.Create(beads.CreateOptions{Title: "TypedDep Parent", Type: "task"})
	if err != nil {
		t.Fatalf("Create parent in ap failed: %v", err)
	}
	child, err := apOps.Create(beads.CreateOptions{Title: "TypedDep Child", Type: "task"})
	if err != nil {
		t.Fatalf("Create child in ap failed: %v", err)
	}

	// Test with Implementation (has routedImpl) - should pass
	t.Run("Implementation/cross-rig", func(t *testing.T) {
		impl := beads.NewWithTownRoot(env.GastownDir, env.TownRoot)
		err := impl.AddDependencyWithType(child.ID, parent.ID, "tracks")
		if err != nil {
			t.Errorf("Implementation AddDependencyWithType failed: %v", err)
		}
	})

	// Test with RawBd (no routedImpl) - should fail for cross-rig
	t.Run("RawBd/cross-rig", func(t *testing.T) {
		parent2, _ := apOps.Create(beads.CreateOptions{Title: "TypedDep Parent2", Type: "task"})
		child2, _ := apOps.Create(beads.CreateOptions{Title: "TypedDep Child2", Type: "task"})

		rawBd := &TrueRawBdOps{workDir: env.GastownDir, testHome: env.TestHome}
		err := rawBd.AddDependencyWithType(child2.ID, parent2.ID, "tracks")

		// RawBd should fail because bd has no routing for dep command
		if err == nil {
			t.Log("WARNING: RawBd AddDependencyWithType passed in cross-rig context - bd may have been fixed!")
		} else {
			t.Logf("Verified: bd routing bug for AddDependencyWithType still exists (%v)", err)
		}
	})
}

// TestMatrix_RemoveDependency_CrossRig tests cross-rig routing for RemoveDependency.
func TestMatrix_RemoveDependency_CrossRig(t *testing.T) {
	if !isBdInstalled() {
		t.Skip("bd not installed")
	}

	env := setupTestEnv(t)
	initRealBd(t, env)

	// Create two issues in the ap rig and add a dependency
	apOps := beads.ForRig(env.AIPlatDir)
	parent, err := apOps.Create(beads.CreateOptions{Title: "RemoveDep Parent", Type: "task"})
	if err != nil {
		t.Fatalf("Create parent in ap failed: %v", err)
	}
	child, err := apOps.Create(beads.CreateOptions{Title: "RemoveDep Child", Type: "task"})
	if err != nil {
		t.Fatalf("Create child in ap failed: %v", err)
	}

	// Add the dependency using apOps (same rig, will work)
	if err := apOps.AddDependency(child.ID, parent.ID); err != nil {
		t.Fatalf("Setup AddDependency failed: %v", err)
	}

	// Test with Implementation (has routedImpl) - should pass
	t.Run("Implementation/cross-rig", func(t *testing.T) {
		impl := beads.NewWithTownRoot(env.GastownDir, env.TownRoot)
		err := impl.RemoveDependency(child.ID, parent.ID)
		if err != nil {
			t.Errorf("Implementation RemoveDependency failed: %v", err)
		}
	})

	// Test with RawBd (no routedImpl) - should fail for cross-rig
	t.Run("RawBd/cross-rig", func(t *testing.T) {
		// Create new issues and dependency for this test
		parent2, _ := apOps.Create(beads.CreateOptions{Title: "RemoveDep Parent2", Type: "task"})
		child2, _ := apOps.Create(beads.CreateOptions{Title: "RemoveDep Child2", Type: "task"})
		_ = apOps.AddDependency(child2.ID, parent2.ID)

		rawBd := &TrueRawBdOps{workDir: env.GastownDir, testHome: env.TestHome}
		err := rawBd.RemoveDependency(child2.ID, parent2.ID)

		// RawBd should fail because bd has no routing for dep command
		if err == nil {
			t.Log("WARNING: RawBd RemoveDependency passed in cross-rig context - bd may have been fixed!")
		} else {
			t.Logf("Verified: bd routing bug for RemoveDependency still exists (%v)", err)
		}
	})
}

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
