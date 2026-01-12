//go:build integration

package beads

import (
	"testing"
)

// === Valid Combinations - All role types from town context ===

func TestGetRoleConfig_Mayor_FromTown(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-roleconfig-mayor", "testrig", "tr")
	bd := env.NewForTown()

	config, err := bd.GetRoleConfigByType("mayor")
	if err != nil {
		t.Fatalf("GetRoleConfig(mayor) failed: %v", err)
	}
	// Config may be nil if role bead has no config fields, but no error
	_ = config
}

func TestGetRoleConfig_Deacon_FromTown(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-roleconfig-deacon", "testrig", "tr")
	bd := env.NewForTown()

	config, err := bd.GetRoleConfigByType("deacon")
	if err != nil {
		t.Fatalf("GetRoleConfig(deacon) failed: %v", err)
	}
	_ = config
}

func TestGetRoleConfig_Witness_FromTown(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-roleconfig-witness", "testrig", "tr")
	bd := env.NewForTown()

	config, err := bd.GetRoleConfigByType("witness")
	if err != nil {
		t.Fatalf("GetRoleConfig(witness) failed: %v", err)
	}
	_ = config
}

func TestGetRoleConfig_Refinery_FromTown(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-roleconfig-refinery", "testrig", "tr")
	bd := env.NewForTown()

	config, err := bd.GetRoleConfigByType("refinery")
	if err != nil {
		t.Fatalf("GetRoleConfig(refinery) failed: %v", err)
	}
	_ = config
}

func TestGetRoleConfig_Polecat_FromTown(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-roleconfig-polecat", "testrig", "tr")
	bd := env.NewForTown()

	config, err := bd.GetRoleConfigByType("polecat")
	if err != nil {
		t.Fatalf("GetRoleConfig(polecat) failed: %v", err)
	}
	_ = config
}

func TestGetRoleConfig_Crew_FromTown(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-roleconfig-crew", "testrig", "tr")
	bd := env.NewForTown()

	config, err := bd.GetRoleConfigByType("crew")
	if err != nil {
		t.Fatalf("GetRoleConfig(crew) failed: %v", err)
	}
	_ = config
}

func TestGetRoleConfig_Dog_FromTown(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-roleconfig-dog", "testrig", "tr")
	bd := env.NewForTown()

	config, err := bd.GetRoleConfigByType("dog")
	if err != nil {
		t.Fatalf("GetRoleConfig(dog) failed: %v", err)
	}
	_ = config
}

// === Context Works From Rig Too ===

func TestGetRoleConfig_Mayor_FromRig(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-roleconfig-mayor-rig", "testrig", "tr")
	bd := env.NewForRig()

	// Role beads are town-level, but should be accessible from rig context
	config, err := bd.GetRoleConfigByType("mayor")
	if err != nil {
		t.Fatalf("GetRoleConfig(mayor) from rig context failed: %v", err)
	}
	_ = config
}

func TestGetRoleConfig_Witness_FromRig(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-roleconfig-witness-rig", "testrig", "tr")
	bd := env.NewForRig()

	config, err := bd.GetRoleConfigByType("witness")
	if err != nil {
		t.Fatalf("GetRoleConfig(witness) from rig context failed: %v", err)
	}
	_ = config
}

// === Invalid Parameters ===

func TestGetRoleConfig_EmptyRoleType_Error(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-roleconfig-empty", "testrig", "tr")
	bd := env.NewForTown()

	_, err := bd.GetRoleConfigByType("")
	if err == nil {
		t.Error("GetRoleConfig('') should fail - empty role type")
	}
}

func TestGetRoleConfig_InvalidRoleType_Error(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-roleconfig-invalid", "testrig", "tr")
	bd := env.NewForTown()

	_, err := bd.GetRoleConfigByType("fakerole")
	if err == nil {
		t.Error("GetRoleConfig(fakerole) should fail - invalid role type")
	}
}

func TestGetRoleConfig_AgentNotRole_Error(t *testing.T) {
	t.Parallel()
	env := SetupTownWithRigEnv(t, "test-roleconfig-agent", "testrig", "tr")
	bd := env.NewForTown()

	_, err := bd.GetRoleConfigByType("agent")
	if err == nil {
		t.Error("GetRoleConfig(agent) should fail - 'agent' is not a role type")
	}
}

// === Not Found ===

func TestGetRoleConfig_NotFound_Error(t *testing.T) {
	// Skip: gt install creates all role beads, and the Beads API doesn't expose
	// a Delete method to remove them for testing. The bd delete command behavior
	// is also unclear. This scenario is hard to test with production setup.
	t.Skip("Cannot test not-found: gt install creates all role beads, no Delete API")
}
