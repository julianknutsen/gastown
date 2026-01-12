//go:build integration

package beads

import (
	"errors"
	"testing"
)

// === GetAgentBead Tests ===
// GetAgentBead(id) returns (*Issue, *AgentFields, error)

func TestGetAgentBead_Exists(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-getagentbead")
	bd := env.NewForTown()

	// Create agent bead using API
	created, err := bd.Create(CreateOptions{
		ID:     "hq-agent-test",
		Title:  "Test Agent",
		Type:   "agent",
		Labels: []string{"gt:agent"},
	})
	if err != nil {
		t.Fatalf("Create agent bead failed: %v", err)
	}

	agent, fields, err := bd.GetAgentBead(created.ID)
	if err != nil {
		t.Fatalf("GetAgentBead() failed: %v", err)
	}
	if agent == nil {
		t.Fatal("GetAgentBead() returned nil issue")
	}
	if agent.ID != created.ID {
		t.Errorf("ID = %q, want %q", agent.ID, created.ID)
	}
	// Fields may be nil or empty for basic agent
	_ = fields
}

func TestGetAgentBead_NotFound(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-getagentbead-notfound")
	bd := env.NewForTown()

	_, _, err := bd.GetAgentBead("hq-nonexistent-agent")
	if err == nil {
		t.Fatal("GetAgentBead() for non-existent should return error")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Expected ErrNotFound, got: %v", err)
	}
}

func TestGetAgentBead_NotAgent(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-getagentbead-notagent")
	bd := env.NewForTown()

	// Create non-agent bead
	created := env.CreateTestBead(t, "Not Agent")

	_, _, err := bd.GetAgentBead(created.ID)
	if err == nil {
		t.Error("GetAgentBead() for non-agent should return error")
	}
}

func TestGetAgentBead_EmptyID(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-getagentbead-empty")
	bd := env.NewForTown()

	_, _, err := bd.GetAgentBead("")
	if err == nil {
		t.Error("GetAgentBead('') should return error")
	}
}

func TestGetAgentBead_ParsesFields(t *testing.T) {
	t.Parallel()
	env := SetupTownEnv(t, "test-getagentbead-fields")
	bd := env.NewForTown()

	// Create agent with description containing fields
	desc := `role_type: polecat
agent_state: active
rig: testrig`
	created, err := bd.Create(CreateOptions{
		ID:          "hq-agent-fields",
		Title:       "Agent with Fields",
		Type:        "agent",
		Labels:      []string{"gt:agent"},
		Description: desc,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	agent, fields, err := bd.GetAgentBead(created.ID)
	if err != nil {
		t.Fatalf("GetAgentBead() failed: %v", err)
	}
	if agent == nil {
		t.Fatal("Expected agent issue")
	}
	if fields == nil {
		t.Fatal("Expected Fields to be parsed")
	}
	if fields.RoleType != "polecat" {
		t.Errorf("RoleType = %q, want %q", fields.RoleType, "polecat")
	}
}
