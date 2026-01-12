//go:build integration

package beads

import (
	"errors"
	"strings"
	"testing"
)

// === Valid Combinations ===

func TestGetAgent_TownAgent_Mayor_FromTown(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-agent-mayor-town", "testrig", "tr")
	bd := env.NewForTown()

	issue, fields, err := bd.GetAgent("mayor")
	if err != nil {
		t.Fatalf("GetAgent(mayor) failed: %v", err)
	}
	if issue == nil {
		t.Fatal("GetAgent(mayor) returned nil issue")
	}
	if issue.ID != "hq-mayor" {
		t.Errorf("ID = %q, want %q", issue.ID, "hq-mayor")
	}
	if fields == nil {
		t.Fatal("GetAgent(mayor) returned nil fields")
	}
	if fields.RoleType != "mayor" {
		t.Errorf("RoleType = %q, want %q", fields.RoleType, "mayor")
	}
}

func TestGetAgent_TownAgent_Deacon_FromTown(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-agent-deacon-town", "testrig", "tr")
	bd := env.NewForTown()

	issue, fields, err := bd.GetAgent("deacon")
	if err != nil {
		t.Fatalf("GetAgent(deacon) failed: %v", err)
	}
	if issue == nil {
		t.Fatal("GetAgent(deacon) returned nil issue")
	}
	if issue.ID != "hq-deacon" {
		t.Errorf("ID = %q, want %q", issue.ID, "hq-deacon")
	}
	if fields != nil && fields.RoleType != "deacon" {
		t.Errorf("RoleType = %q, want %q", fields.RoleType, "deacon")
	}
}

func TestGetAgent_TownAgent_Mayor_FromRig(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-agent-mayor-rig", "testrig", "tr")
	bd := env.NewForRig()

	// Town lookups should work from rig context
	issue, fields, err := bd.GetAgent("mayor")
	if err != nil {
		t.Fatalf("GetAgent(mayor) from rig context failed: %v", err)
	}
	if issue == nil {
		t.Fatal("GetAgent(mayor) returned nil issue")
	}
	if issue.ID != "hq-mayor" {
		t.Errorf("ID = %q, want %q", issue.ID, "hq-mayor")
	}
	if fields == nil {
		t.Fatal("GetAgent(mayor) returned nil fields")
	}
}

func TestGetAgent_TownAgent_Deacon_FromRig(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-agent-deacon-rig", "testrig", "tr")
	bd := env.NewForRig()

	issue, _, err := bd.GetAgent("deacon")
	if err != nil {
		t.Fatalf("GetAgent(deacon) from rig context failed: %v", err)
	}
	if issue == nil {
		t.Fatal("GetAgent(deacon) returned nil issue")
	}
	if issue.ID != "hq-deacon" {
		t.Errorf("ID = %q, want %q", issue.ID, "hq-deacon")
	}
}

func TestGetAgent_RigSingleton_Witness_FromRig(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-agent", "testrig", "tr")
	bd := env.NewForRig()

	// Create witness agent first
	fields := &AgentFields{RoleType: "witness", Rig: "testrig", AgentState: "idle"}
	_, err := bd.CreateRigAgent("witness", "", "Test Witness", fields)
	if err != nil {
		t.Fatalf("CreateRigAgent failed: %v", err)
	}

	// Now read it back
	issue, readFields, err := bd.GetAgent("witness")
	if err != nil {
		t.Fatalf("GetAgent(witness) failed: %v", err)
	}
	if issue == nil {
		t.Fatal("GetAgent(witness) returned nil issue")
	}
	if issue.ID != "tr-testrig-witness" {
		t.Errorf("ID = %q, want %q", issue.ID, "tr-testrig-witness")
	}
	if readFields == nil || readFields.RoleType != "witness" {
		t.Errorf("RoleType mismatch")
	}
}

func TestGetAgent_RigSingleton_Refinery_FromRig(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-agent", "testrig", "tr")
	bd := env.NewForRig()

	// Create refinery agent first
	fields := &AgentFields{RoleType: "refinery", Rig: "testrig", AgentState: "idle"}
	_, err := bd.CreateRigAgent("refinery", "", "Test Refinery", fields)
	if err != nil {
		t.Fatalf("CreateRigAgent failed: %v", err)
	}

	issue, _, err := bd.GetAgent("refinery")
	if err != nil {
		t.Fatalf("GetAgent(refinery) failed: %v", err)
	}
	if issue == nil {
		t.Fatal("GetAgent(refinery) returned nil issue")
	}
	if issue.ID != "tr-testrig-refinery" {
		t.Errorf("ID = %q, want %q", issue.ID, "tr-testrig-refinery")
	}
}

func TestGetAgent_RigNamed_Polecat_FromRig(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-agent", "testrig", "tr")
	bd := env.NewForRig()

	// Create polecat agent first
	fields := &AgentFields{RoleType: "polecat", Rig: "testrig", AgentState: "idle"}
	_, err := bd.CreateRigAgent("polecat", "Toast", "Test Polecat", fields)
	if err != nil {
		t.Fatalf("CreateRigAgent failed: %v", err)
	}

	issue, readFields, err := bd.GetAgent("polecat", "Toast")
	if err != nil {
		t.Fatalf("GetAgent(polecat, Toast) failed: %v", err)
	}
	if issue == nil {
		t.Fatal("GetAgent(polecat, Toast) returned nil issue")
	}
	if issue.ID != "tr-testrig-polecat-Toast" {
		t.Errorf("ID = %q, want %q", issue.ID, "tr-testrig-polecat-Toast")
	}
	if readFields == nil || readFields.RoleType != "polecat" {
		t.Errorf("RoleType mismatch")
	}
}

func TestGetAgent_RigNamed_Crew_FromRig(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-agent", "testrig", "tr")
	bd := env.NewForRig()

	// Create crew agent first
	fields := &AgentFields{RoleType: "crew", Rig: "testrig", AgentState: "idle"}
	_, err := bd.CreateRigAgent("crew", "Max", "Test Crew", fields)
	if err != nil {
		t.Fatalf("CreateRigAgent failed: %v", err)
	}

	issue, _, err := bd.GetAgent("crew", "Max")
	if err != nil {
		t.Fatalf("GetAgent(crew, Max) failed: %v", err)
	}
	if issue == nil {
		t.Fatal("GetAgent(crew, Max) returned nil issue")
	}
	if issue.ID != "tr-testrig-crew-Max" {
		t.Errorf("ID = %q, want %q", issue.ID, "tr-testrig-crew-Max")
	}
}

// === Invalid Combinations - Wrong Context ===

func TestGetAgent_RigSingleton_Witness_FromTownOnly_Error(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-agent", "testrig", "tr")
	bd := env.NewForTown() // town-only context

	_, _, err := bd.GetAgent("witness")
	if err == nil {
		t.Error("GetAgent(witness) from town-only context should fail")
	}
	if !strings.Contains(err.Error(), "rig context") {
		t.Errorf("Error should mention rig context, got: %v", err)
	}
}

func TestGetAgent_RigSingleton_Refinery_FromTownOnly_Error(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-agent", "testrig", "tr")
	bd := env.NewForTown()

	_, _, err := bd.GetAgent("refinery")
	if err == nil {
		t.Error("GetAgent(refinery) from town-only context should fail")
	}
	if !strings.Contains(err.Error(), "rig context") {
		t.Errorf("Error should mention rig context, got: %v", err)
	}
}

func TestGetAgent_RigNamed_Polecat_FromTownOnly_Error(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-agent", "testrig", "tr")
	bd := env.NewForTown()

	_, _, err := bd.GetAgent("polecat", "Toast")
	if err == nil {
		t.Error("GetAgent(polecat, Toast) from town-only context should fail")
	}
	if !strings.Contains(err.Error(), "rig context") {
		t.Errorf("Error should mention rig context, got: %v", err)
	}
}

func TestGetAgent_RigNamed_Crew_FromTownOnly_Error(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-agent", "testrig", "tr")
	bd := env.NewForTown()

	_, _, err := bd.GetAgent("crew", "Max")
	if err == nil {
		t.Error("GetAgent(crew, Max) from town-only context should fail")
	}
	if !strings.Contains(err.Error(), "rig context") {
		t.Errorf("Error should mention rig context, got: %v", err)
	}
}

// === Invalid Combinations - Wrong Name Usage ===

func TestGetAgent_TownAgent_Mayor_WithName_Error(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-agent", "testrig", "tr")
	bd := env.NewForTown()

	_, _, err := bd.GetAgent("mayor", "SomeName")
	if err == nil {
		t.Error("GetAgent(mayor, SomeName) should fail - town roles don't take names")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("Error should mention name, got: %v", err)
	}
}

func TestGetAgent_TownAgent_Deacon_WithName_Error(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-agent", "testrig", "tr")
	bd := env.NewForTown()

	_, _, err := bd.GetAgent("deacon", "SomeName")
	if err == nil {
		t.Error("GetAgent(deacon, SomeName) should fail - town roles don't take names")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("Error should mention name, got: %v", err)
	}
}

func TestGetAgent_RigSingleton_Witness_WithName_Error(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-agent", "testrig", "tr")
	bd := env.NewForRig()

	_, _, err := bd.GetAgent("witness", "SomeName")
	if err == nil {
		t.Error("GetAgent(witness, SomeName) should fail - singletons don't take names")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("Error should mention name, got: %v", err)
	}
}

func TestGetAgent_RigSingleton_Refinery_WithName_Error(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-agent", "testrig", "tr")
	bd := env.NewForRig()

	_, _, err := bd.GetAgent("refinery", "SomeName")
	if err == nil {
		t.Error("GetAgent(refinery, SomeName) should fail - singletons don't take names")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("Error should mention name, got: %v", err)
	}
}

func TestGetAgent_RigNamed_Polecat_NoName_Error(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-agent", "testrig", "tr")
	bd := env.NewForRig()

	_, _, err := bd.GetAgent("polecat")
	if err == nil {
		t.Error("GetAgent(polecat) without name should fail")
	}
	if !strings.Contains(err.Error(), "requires name") {
		t.Errorf("Error should mention requires name, got: %v", err)
	}
}

func TestGetAgent_RigNamed_Crew_NoName_Error(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-agent", "testrig", "tr")
	bd := env.NewForRig()

	_, _, err := bd.GetAgent("crew")
	if err == nil {
		t.Error("GetAgent(crew) without name should fail")
	}
	if !strings.Contains(err.Error(), "requires name") {
		t.Errorf("Error should mention requires name, got: %v", err)
	}
}

// === Invalid Role Type ===

func TestGetAgent_InvalidRole_FromTown_Error(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-agent", "testrig", "tr")
	bd := env.NewForTown()

	_, _, err := bd.GetAgent("fakrole")
	if err == nil {
		t.Error("GetAgent(fakerole) should fail - invalid role")
	}
	if !strings.Contains(err.Error(), "invalid") && !strings.Contains(err.Error(), "unknown") {
		t.Errorf("Error should mention invalid/unknown role, got: %v", err)
	}
}

func TestGetAgent_InvalidRole_FromRig_Error(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-agent", "testrig", "tr")
	bd := env.NewForRig()

	_, _, err := bd.GetAgent("fakerole")
	if err == nil {
		t.Error("GetAgent(fakerole) should fail - invalid role")
	}
	if !strings.Contains(err.Error(), "invalid") && !strings.Contains(err.Error(), "unknown") {
		t.Errorf("Error should mention invalid/unknown role, got: %v", err)
	}
}

// === Not Found Cases ===

func TestGetAgent_RigNamed_NotFound(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-agent", "testrig", "tr")
	bd := env.NewForRig()

	// Don't create the agent - should return ErrNotFound
	_, _, err := bd.GetAgent("polecat", "NonExistent")
	if err == nil {
		t.Fatal("GetAgent for non-existent should return error")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Expected ErrNotFound, got: %v", err)
	}
}

func TestGetAgent_RigSingleton_NotFound(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-agent", "testrig", "tr")
	bd := env.NewForRig()

	// Witness not created yet - should return ErrNotFound
	_, _, err := bd.GetAgent("witness")
	if err == nil {
		t.Fatal("GetAgent for non-existent should return error")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Expected ErrNotFound, got: %v", err)
	}
}

// === Route Lookup Verification ===

func TestGetAgent_RigAgent_UsesCorrectPrefix(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-agent-prefix", "testrig", "tr")
	bd := env.NewForRig()

	// Create agent
	fields := &AgentFields{RoleType: "witness", Rig: "testrig", AgentState: "idle"}
	created, err := bd.CreateRigAgent("witness", "", "Test", fields)
	if err != nil {
		t.Fatalf("CreateRigAgent failed: %v", err)
	}

	// Verify prefix
	if !strings.HasPrefix(created.ID, "tr-") {
		t.Errorf("Created ID = %q, should have prefix 'tr-'", created.ID)
	}

	// Read back and verify
	issue, _, err := bd.GetAgent("witness")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}
	if issue.ID != created.ID {
		t.Errorf("GetAgent returned ID = %q, want %q", issue.ID, created.ID)
	}
}

func TestGetAgent_DifferentRig_DifferentPrefix(t *testing.T) {
	t.Parallel()
	// Create env with different prefix
	env := SetupTownWithRigEnv(t, "test-agent-diff-prefix", "testrig", "xx")
	bd := env.NewForRig()

	// Create agent
	fields := &AgentFields{RoleType: "witness", Rig: "testrig", AgentState: "idle"}
	created, err := bd.CreateRigAgent("witness", "", "Test", fields)
	if err != nil {
		t.Fatalf("CreateRigAgent failed: %v", err)
	}

	// Verify prefix is "xx-" not "tr-" or "gt-"
	if !strings.HasPrefix(created.ID, "xx-") {
		t.Errorf("Created ID = %q, should have prefix 'xx-'", created.ID)
	}

	// Read back and verify
	issue, _, err := bd.GetAgent("witness")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}
	if !strings.HasPrefix(issue.ID, "xx-") {
		t.Errorf("GetAgent returned ID = %q, should have prefix 'xx-'", issue.ID)
	}
}
