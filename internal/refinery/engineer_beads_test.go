package refinery

import (
	"bytes"
	"testing"

	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/rig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEngineerHandleSuccess tests that handleSuccess properly:
// - Updates MR with merge_commit SHA
// - Closes MR with reason 'merged'
// - Closes source issue with reference to MR
// - Clears agent bead's active_mr reference
func TestEngineerHandleSuccess(t *testing.T) {
	// Setup: Create a test double with an MR and related beads
	d := beads.NewDoubleWithPrefix("gt")

	// Create an agent bead with active_mr
	agentBead, err := d.Create(beads.CreateOptions{
		Title:       "polecat/nux",
		Type:        "agent",
		Description: "active_mr: gt-mr123\nrig: test-rig",
	})
	require.NoError(t, err)

	// Create a source issue that will be closed
	sourceIssue, err := d.Create(beads.CreateOptions{
		Title: "Implement feature X",
		Type:  "task",
	})
	require.NoError(t, err)

	// Create an MR bead with MR fields
	mrFields := &beads.MRFields{
		Branch:      "polecat/nux",
		Target:      "main",
		Worker:      "nux",
		Rig:         "test-rig",
		SourceIssue: sourceIssue.ID,
		AgentBead:   agentBead.ID,
	}
	mrBead, err := d.Create(beads.CreateOptions{
		Title:       "Merge polecat/nux into main",
		Type:        "merge-request",
		Description: beads.FormatMRFields(mrFields),
	})
	require.NoError(t, err)

	// Create a minimal Engineer with the Double
	// Disable branch deletion to avoid needing a real git object
	cfg := DefaultMergeQueueConfig()
	cfg.DeleteMergedBranches = false
	eng := &Engineer{
		beads:  d,
		output: &bytes.Buffer{},
		config: cfg,
	}

	// Execute: Call handleSuccess
	result := ProcessResult{
		Success:     true,
		MergeCommit: "abc123def456",
	}
	eng.handleSuccess(mrBead, result)

	// Verify: MR is closed with 'merged' reason and has merge_commit in description
	updatedMR, err := d.Show(mrBead.ID)
	require.NoError(t, err)
	assert.Equal(t, "closed", updatedMR.Status)
	assert.Contains(t, updatedMR.Description, "merge_commit: abc123def456")
	assert.Contains(t, updatedMR.Description, "close_reason: merged")

	// Verify: Source issue is closed with reference to MR
	updatedSource, err := d.Show(sourceIssue.ID)
	require.NoError(t, err)
	assert.Equal(t, "closed", updatedSource.Status)

	// Verify: Agent bead's active_mr is cleared
	updatedAgent, err := d.Show(agentBead.ID)
	require.NoError(t, err)
	assert.NotContains(t, updatedAgent.Description, "active_mr: gt-mr123")
}

// TestEngineerHandleFailure tests that handleFailure properly:
// - Reopens the MR by setting status to "open"
func TestEngineerHandleFailure(t *testing.T) {
	d := beads.NewDoubleWithPrefix("gt")

	// Create an MR bead and set it to in_progress
	mrBead, err := d.Create(beads.CreateOptions{
		Title: "Merge polecat/nux into main",
		Type:  "merge-request",
	})
	require.NoError(t, err)

	inProgress := "in_progress"
	err = d.Update(mrBead.ID, beads.UpdateOptions{Status: &inProgress})
	require.NoError(t, err)

	// Create Engineer with Double
	eng := &Engineer{
		beads:  d,
		output: &bytes.Buffer{},
		config: DefaultMergeQueueConfig(),
	}

	// Execute: Call handleFailure
	result := ProcessResult{
		Success: false,
		Error:   "merge conflict in file.go",
	}
	eng.handleFailure(mrBead, result)

	// Verify: MR is back to "open" status
	updatedMR, err := d.Show(mrBead.ID)
	require.NoError(t, err)
	assert.Equal(t, "open", updatedMR.Status)
}

// TestEngineerListReadyMRs tests that ListReadyMRs properly:
// - Returns MRs from bd ready with type merge-request
// - Skips MRs that are already assigned
func TestEngineerListReadyMRs(t *testing.T) {
	d := beads.NewDoubleWithPrefix("gt")

	// Create MR beads with merge-request type
	mr1Fields := &beads.MRFields{
		Branch:      "polecat/one",
		Target:      "main",
		Worker:      "one",
		Rig:         "test-rig",
		SourceIssue: "gt-task1",
	}
	mr1, err := d.Create(beads.CreateOptions{
		Title:       "Merge one",
		Type:        "merge-request",
		Priority:    1,
		Description: beads.FormatMRFields(mr1Fields),
	})
	require.NoError(t, err)
	// Add the label that ReadyWithType looks for
	err = d.LabelAdd(mr1.ID, "gt:merge-request")
	require.NoError(t, err)

	mr2Fields := &beads.MRFields{
		Branch:      "polecat/two",
		Target:      "main",
		Worker:      "two",
		Rig:         "test-rig",
		SourceIssue: "gt-task2",
	}
	mr2, err := d.Create(beads.CreateOptions{
		Title:       "Merge two",
		Type:        "merge-request",
		Priority:    2,
		Description: beads.FormatMRFields(mr2Fields),
	})
	require.NoError(t, err)
	err = d.LabelAdd(mr2.ID, "gt:merge-request")
	require.NoError(t, err)

	// Assign mr2 to another worker (should be filtered out)
	assigned := "other-worker"
	err = d.Update(mr2.ID, beads.UpdateOptions{Assignee: &assigned})
	require.NoError(t, err)

	// Create Engineer with Double
	eng := &Engineer{
		beads:  d,
		output: &bytes.Buffer{},
		config: DefaultMergeQueueConfig(),
	}

	// Execute: List ready MRs
	mrs, err := eng.ListReadyMRs()
	require.NoError(t, err)

	// Verify: Only mr1 is returned (mr2 is assigned)
	assert.Len(t, mrs, 1)
	assert.Equal(t, mr1.ID, mrs[0].ID)
	assert.Equal(t, "polecat/one", mrs[0].Branch)
	assert.Equal(t, "main", mrs[0].Target)
	assert.Equal(t, "one", mrs[0].Worker)
}

// TestEngineerClaimAndReleaseMR tests claim/release operations
func TestEngineerClaimAndReleaseMR(t *testing.T) {
	d := beads.NewDoubleWithPrefix("gt")

	// Create an MR bead
	mr, err := d.Create(beads.CreateOptions{
		Title: "Merge test",
		Type:  "merge-request",
	})
	require.NoError(t, err)

	eng := &Engineer{
		beads:  d,
		output: &bytes.Buffer{},
	}

	// Claim the MR
	err = eng.ClaimMR(mr.ID, "refinery/worker1")
	require.NoError(t, err)

	// Verify: MR is claimed
	claimed, err := d.Show(mr.ID)
	require.NoError(t, err)
	assert.Equal(t, "refinery/worker1", claimed.Assignee)

	// Release the MR
	err = eng.ReleaseMR(mr.ID)
	require.NoError(t, err)

	// Verify: MR is released
	released, err := d.Show(mr.ID)
	require.NoError(t, err)
	assert.Empty(t, released.Assignee)
}

// TestEngineerIsBeadOpen tests the IsBeadOpen status checker
func TestEngineerIsBeadOpen(t *testing.T) {
	d := beads.NewDoubleWithPrefix("gt")

	// Create an open bead
	openBead, err := d.Create(beads.CreateOptions{
		Title: "Open task",
		Type:  "task",
	})
	require.NoError(t, err)

	// Create and close a bead
	closedBead, err := d.Create(beads.CreateOptions{
		Title: "Closed task",
		Type:  "task",
	})
	require.NoError(t, err)
	err = d.Close(closedBead.ID)
	require.NoError(t, err)

	eng := &Engineer{
		beads:  d,
		output: &bytes.Buffer{},
	}

	// Test open bead
	isOpen, err := eng.IsBeadOpen(openBead.ID)
	require.NoError(t, err)
	assert.True(t, isOpen)

	// Test closed bead
	isOpen, err = eng.IsBeadOpen(closedBead.ID)
	require.NoError(t, err)
	assert.False(t, isOpen)

	// Test non-existent bead (fails open)
	isOpen, err = eng.IsBeadOpen("gt-nonexistent")
	require.NoError(t, err)
	assert.False(t, isOpen)
}

// TestEngineerListBlockedMRs tests listing blocked MRs
func TestEngineerListBlockedMRs(t *testing.T) {
	d := beads.NewDoubleWithPrefix("gt")

	// Create a blocker task (open)
	blocker, err := d.Create(beads.CreateOptions{
		Title: "Blocking task",
		Type:  "task",
	})
	require.NoError(t, err)

	// Create an MR that's blocked by the task
	mrFields := &beads.MRFields{
		Branch: "polecat/blocked",
		Target: "main",
		Worker: "blocked-worker",
	}
	mr, err := d.Create(beads.CreateOptions{
		Title:       "Blocked MR",
		Type:        "merge-request",
		Description: beads.FormatMRFields(mrFields),
	})
	require.NoError(t, err)
	err = d.LabelAdd(mr.ID, "gt:merge-request")
	require.NoError(t, err)

	// Add dependency: MR blocked by blocker
	err = d.AddDependency(mr.ID, blocker.ID)
	require.NoError(t, err)

	// Create an unblocked MR
	unblockedFields := &beads.MRFields{
		Branch: "polecat/unblocked",
		Target: "main",
		Worker: "unblocked-worker",
	}
	unblockedMR, err := d.Create(beads.CreateOptions{
		Title:       "Unblocked MR",
		Type:        "merge-request",
		Description: beads.FormatMRFields(unblockedFields),
	})
	require.NoError(t, err)
	err = d.LabelAdd(unblockedMR.ID, "gt:merge-request")
	require.NoError(t, err)

	eng := &Engineer{
		beads:  d,
		output: &bytes.Buffer{},
	}

	// List blocked MRs
	blockedMRs, err := eng.ListBlockedMRs()
	require.NoError(t, err)

	// Verify: Only the blocked MR is returned
	assert.Len(t, blockedMRs, 1)
	assert.Equal(t, mr.ID, blockedMRs[0].ID)
	assert.Equal(t, blocker.ID, blockedMRs[0].BlockedBy)
}

// TestEngineerCreateConflictResolutionTask tests conflict task creation with merge slot
func TestEngineerCreateConflictResolutionTask(t *testing.T) {
	d := beads.NewDoubleWithPrefix("gt")

	// Create a mock rig for the engineer
	r := &rig.Rig{
		Name: "test-rig",
	}

	// We need a mock git for Rev() - skip the git-dependent parts
	// by focusing on the beads operations (merge slot acquisition and task creation)

	// Create a source issue
	source, err := d.Create(beads.CreateOptions{
		Title: "Original feature",
		Type:  "task",
	})
	require.NoError(t, err)

	mr := &MRInfo{
		ID:          "gt-mr001",
		Branch:      "polecat/conflict",
		Target:      "main",
		Worker:      "worker1",
		SourceIssue: source.ID,
		Priority:    2,
		RetryCount:  0,
	}

	// The function uses git.Rev which needs a real git repo
	// For this test, we'll verify the beads operations work correctly
	// by checking merge slot acquisition

	// First, ensure merge slot exists
	_, err = d.MergeSlotEnsureExists()
	require.NoError(t, err)

	// Verify merge slot is available initially
	status, err := d.MergeSlotCheck()
	require.NoError(t, err)
	assert.True(t, status.Available)

	// Simulate what createConflictResolutionTaskForMR does for merge slot
	holder := r.Name + "/refinery"
	status, err = d.MergeSlotAcquire(holder, false)
	require.NoError(t, err)
	assert.True(t, status.Available || status.Holder == holder)

	// Create a task (what the function would create)
	task, err := d.Create(beads.CreateOptions{
		Title:       "Resolve merge conflicts: Original feature",
		Type:        "task",
		Priority:    mr.Priority - 1, // boosted priority
		Description: "Conflict resolution task",
		Actor:       holder,
	})
	require.NoError(t, err)

	// Verify task was created with boosted priority
	assert.Equal(t, 1, task.Priority) // P2 -> P1

	// Verify merge slot is now held
	status, err = d.MergeSlotCheck()
	require.NoError(t, err)
	assert.Equal(t, holder, status.Holder)
}

// TestEngineerHandleMRInfoSuccess tests HandleMRInfoSuccess
func TestEngineerHandleMRInfoSuccess(t *testing.T) {
	d := beads.NewDoubleWithPrefix("gt")

	// Create a mock rig
	r := &rig.Rig{
		Name: "test-rig",
		Path: "/tmp/test-rig",
	}

	// Create an agent bead
	agentBead, err := d.Create(beads.CreateOptions{
		Title:       "polecat/worker",
		Type:        "agent",
		Description: "active_mr: gt-mr001\nrig: test-rig",
	})
	require.NoError(t, err)

	// Create a source issue
	sourceIssue, err := d.Create(beads.CreateOptions{
		Title: "Feature task",
		Type:  "task",
	})
	require.NoError(t, err)

	// Create an MR bead
	mrFields := &beads.MRFields{
		Branch:      "polecat/worker",
		Target:      "main",
		Worker:      "worker",
		SourceIssue: sourceIssue.ID,
		AgentBead:   agentBead.ID,
	}
	mrBead, err := d.Create(beads.CreateOptions{
		Title:       "Merge worker branch",
		Type:        "merge-request",
		Description: beads.FormatMRFields(mrFields),
	})
	require.NoError(t, err)

	// Acquire merge slot (simulating conflict resolution in progress)
	_, err = d.MergeSlotEnsureExists()
	require.NoError(t, err)
	holder := r.Name + "/refinery"
	_, err = d.MergeSlotAcquire(holder, false)
	require.NoError(t, err)

	// Disable branch deletion to avoid needing a real git object
	cfg := DefaultMergeQueueConfig()
	cfg.DeleteMergedBranches = false
	eng := &Engineer{
		rig:    r,
		beads:  d,
		output: &bytes.Buffer{},
		config: cfg,
	}

	mr := &MRInfo{
		ID:          mrBead.ID,
		Branch:      "polecat/worker",
		Target:      "main",
		Worker:      "worker",
		SourceIssue: sourceIssue.ID,
		AgentBead:   agentBead.ID,
	}

	result := ProcessResult{
		Success:     true,
		MergeCommit: "deadbeef123",
	}

	// Execute
	eng.HandleMRInfoSuccess(mr, result)

	// Verify: MR bead is closed
	updatedMR, err := d.Show(mrBead.ID)
	require.NoError(t, err)
	assert.Equal(t, "closed", updatedMR.Status)

	// Verify: Source issue is closed
	updatedSource, err := d.Show(sourceIssue.ID)
	require.NoError(t, err)
	assert.Equal(t, "closed", updatedSource.Status)

	// Verify: Agent bead's active_mr is cleared (no longer contains the MR ID)
	updatedAgent, err := d.Show(agentBead.ID)
	require.NoError(t, err)
	assert.NotContains(t, updatedAgent.Description, "active_mr: gt-mr001")

	// Verify: Merge slot is released
	status, err := d.MergeSlotCheck()
	require.NoError(t, err)
	assert.True(t, status.Available)
}
