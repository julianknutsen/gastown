package agent_test

import (
	"testing"

	"github.com/steveyegge/gastown/internal/agent"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Bootstrap Generation Tests
// =============================================================================

func TestGenerateBootstrap_Mayor(t *testing.T) {
	content := agent.GenerateBootstrap("mayor", "beads")

	assert.Contains(t, content, "Mayor Context")
	assert.Contains(t, content, "beads")
	assert.Contains(t, content, "gt prime")
	assert.Contains(t, content, "Recovery")
}

func TestGenerateBootstrap_Refinery(t *testing.T) {
	content := agent.GenerateBootstrap("refinery", "beads")

	assert.Contains(t, content, "Refinery Context")
	assert.Contains(t, content, "beads")
	assert.Contains(t, content, "gt prime")
	assert.Contains(t, content, "gt mq list")
	assert.Contains(t, content, "gt mq process")
}

func TestGenerateBootstrap_Witness(t *testing.T) {
	content := agent.GenerateBootstrap("witness", "beads")

	assert.Contains(t, content, "Witness Context")
	assert.Contains(t, content, "beads")
	assert.Contains(t, content, "gt prime")
	assert.Contains(t, content, "gt patrol status")
	assert.Contains(t, content, "gt polecat spawn")
}

func TestGenerateBootstrap_Crew(t *testing.T) {
	content := agent.GenerateBootstrap("crew", "beads")

	assert.Contains(t, content, "Crew Context")
	assert.Contains(t, content, "beads")
	assert.Contains(t, content, "gt prime")
}

func TestGenerateBootstrap_Polecats(t *testing.T) {
	content := agent.GenerateBootstrap("polecats", "beads")

	assert.Contains(t, content, "Polecat Context")
	assert.Contains(t, content, "beads")
	assert.Contains(t, content, "gt prime")
	assert.Contains(t, content, "gt polecat status")
	assert.Contains(t, content, "gt polecat logs")
}

func TestGenerateBootstrap_UnknownRole(t *testing.T) {
	content := agent.GenerateBootstrap("unknown-role", "beads")

	assert.Contains(t, content, "Agent Context")
	assert.Contains(t, content, "beads")
	assert.Contains(t, content, "gt prime")
}

func TestGenerateBootstrap_AllRolesIncludeRigName(t *testing.T) {
	roles := []string{"mayor", "refinery", "witness", "crew", "polecats", "unknown"}

	for _, role := range roles {
		t.Run(role, func(t *testing.T) {
			content := agent.GenerateBootstrap(role, "my-custom-rig")
			assert.Contains(t, content, "my-custom-rig")
		})
	}
}
