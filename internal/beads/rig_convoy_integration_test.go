//go:build integration

package beads

import (
	"strings"
	"testing"
)

// === CreateRigBead Tests ===

func TestCreateRigBead_Basic(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-rigbead", "testrig", "tr")
	bd := env.NewForRig()

	fields := &RigFields{
		Repo:   "git@github.com:test/repo.git",
		Prefix: "tr",
		State:  "active",
	}

	issue, err := bd.CreateRigBead("tr-rig-test", "Test Rig", fields)
	if err != nil {
		t.Fatalf("CreateRigBead() failed: %v", err)
	}
	if issue == nil {
		t.Fatal("Expected issue, got nil")
	}
	if issue.ID != "tr-rig-test" {
		t.Errorf("ID = %q, want %q", issue.ID, "tr-rig-test")
	}

	// Check gt:rig label via Show
	shown, err := bd.Show(issue.ID)
	if err != nil {
		t.Fatalf("Show() failed: %v", err)
	}
	hasLabel := false
	for _, l := range shown.Labels {
		if l == "gt:rig" {
			hasLabel = true
			break
		}
	}
	if !hasLabel {
		t.Errorf("Missing gt:rig label: %v", shown.Labels)
	}
}

func TestCreateRigBead_NilFields(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-rigbead-nil", "testrig", "tr")
	bd := env.NewForRig()

	issue, err := bd.CreateRigBead("tr-rig-nil", "Rig Nil Fields", nil)
	if err != nil {
		t.Fatalf("CreateRigBead() with nil fields failed: %v", err)
	}
	if issue == nil {
		t.Fatal("Expected issue, got nil")
	}
}

// === CreateConvoy Tests ===

func TestCreateConvoy_Basic(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-convoy", "testrig", "tr")
	bd := env.NewForRig()

	fields := &ConvoyFields{
		Notify:   "test@example.com",
		Molecule: "tr-mol-1",
	}

	issue, err := bd.CreateConvoy("tr-convoy1", "Test Convoy", 5, fields)
	if err != nil {
		t.Fatalf("CreateConvoy() failed: %v", err)
	}
	if issue == nil {
		t.Fatal("Expected issue, got nil")
	}
	if issue.ID != "tr-convoy1" {
		t.Errorf("ID = %q, want %q", issue.ID, "tr-convoy1")
	}
}

func TestCreateConvoy_WithFields(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-convoy-fields", "testrig", "tr")
	bd := env.NewForRig()

	fields := &ConvoyFields{
		Notify:   "webhook@example.com",
		Molecule: "tr-mol-2",
	}

	issue, err := bd.CreateConvoy("tr-convoy2", "Convoy With Fields", 2, fields)
	if err != nil {
		t.Fatalf("CreateConvoy() failed: %v", err)
	}
	if issue == nil {
		t.Fatal("Expected issue, got nil")
	}
}

// === CreateTownConvoy Tests ===

func TestCreateTownConvoy_Basic(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-townconvoy")
	bd := env.NewForTown()

	fields := &ConvoyFields{
		Notify: "town@example.com",
	}

	issue, err := bd.CreateTownConvoy("Town Convoy", 3, fields)
	if err != nil {
		t.Fatalf("CreateTownConvoy() failed: %v", err)
	}
	if issue == nil {
		t.Fatal("Expected issue, got nil")
	}
	// ID should start with hq-cv- (town convoy prefix)
	if !strings.HasPrefix(issue.ID, "hq-cv-") {
		t.Errorf("ID = %q, should start with 'hq-cv-'", issue.ID)
	}
}

// === CreateRigAgent Tests (requires rig context) ===

func TestCreateRigAgent_FromTownOnly_Error(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-rigagent-error")
	bd := env.NewForTown() // Town-only context

	fields := &AgentFields{RoleType: "polecat"}

	_, err := bd.CreateRigAgent("polecat", "Toast", "Polecat Toast", fields)
	if err == nil {
		t.Error("CreateRigAgent() from town-only context should fail")
	}
}

func TestCreateRigAgent_WithRigContext(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-rigagent", "testrig", "tr")
	bd := env.NewForRig()

	fields := &AgentFields{
		RoleType:   "polecat",
		Rig:        "testrig",
		AgentState: "spawning",
	}

	issue, err := bd.CreateRigAgent("polecat", "Toast", "Polecat Toast", fields)
	if err != nil {
		t.Fatalf("CreateRigAgent() failed: %v", err)
	}
	if issue == nil {
		t.Fatal("Expected issue, got nil")
	}
	// ID format: {prefix}-{rigName}-{roleType}-{name}
	expectedID := "tr-testrig-polecat-Toast"
	if issue.ID != expectedID {
		t.Errorf("ID = %q, want %q", issue.ID, expectedID)
	}
}

// === CreateOrReopenRigAgentBead Tests ===

func TestCreateOrReopenRigAgentBead_FromTownOnly_Error(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-reopen-error")
	bd := env.NewForTown() // Town-only context

	fields := &AgentFields{RoleType: "polecat"}

	_, err := bd.CreateOrReopenRigAgentBead("polecat", "Toast", "Polecat Toast", fields)
	if err == nil {
		t.Error("CreateOrReopenRigAgentBead() from town-only context should fail")
	}
}

// === CreateRigIdentityBead Tests ===

func TestCreateRigIdentityBead_FromTownOnly_Error(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-rigident-error")
	bd := env.NewForTown() // Town-only context

	fields := &RigFields{
		Repo: "git@github.com:test/repo.git",
	}

	_, err := bd.CreateRigIdentityBead(fields)
	if err == nil {
		t.Error("CreateRigIdentityBead() from town-only context should fail")
	}
}

func TestCreateRigIdentityBead_WithRigContext(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-rigident", "testrig", "tr")
	bd := env.NewForRig()

	fields := &RigFields{
		Repo:   "git@github.com:test/repo.git",
		Prefix: "tr",
		State:  "active",
	}

	issue, err := bd.CreateRigIdentityBead(fields)
	if err != nil {
		t.Fatalf("CreateRigIdentityBead() failed: %v", err)
	}
	if issue == nil {
		t.Fatal("Expected issue, got nil")
	}
}
