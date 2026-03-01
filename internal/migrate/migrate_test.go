package migrate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/steveyegge/gastown/internal/config"
)

// writeTownFixture creates a minimal Gas Town directory structure for testing.
func writeTownFixture(t *testing.T, dir string, townCfg *config.TownConfig, settings *config.TownSettings, rigs *config.RigsConfig) {
	t.Helper()

	// Write mayor/town.json.
	mayorDir := filepath.Join(dir, "mayor")
	if err := os.MkdirAll(mayorDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(mayorDir, "town.json"), townCfg)

	// Write settings/config.json.
	settingsDir := filepath.Join(dir, "settings")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(settingsDir, "config.json"), settings)

	// Write mayor/rigs.json (optional).
	if rigs != nil {
		writeJSON(t, filepath.Join(mayorDir, "rigs.json"), rigs)
	}
}

func writeJSON(t *testing.T, path string, v interface{}) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestReadTownSnapshot(t *testing.T) {
	dir := t.TempDir()

	townCfg := &config.TownConfig{
		Type:    "town",
		Version: 2,
		Name:    "bright-lights",
	}
	settings := &config.TownSettings{
		Type:         "town-settings",
		Version:      1,
		DefaultAgent: "claude",
		Agents: map[string]*config.RuntimeConfig{
			"opus-46": {
				Command: "claude",
				Args:    []string{"--model", "opus", "--dangerously-skip-permissions"},
			},
		},
		RoleAgents: map[string]string{
			"mayor":   "opus-46",
			"polecat": "claude",
		},
	}
	rigs := &config.RigsConfig{
		Version: 1,
		Rigs: map[string]config.RigEntry{
			"myproject": {
				GitURL:    "https://github.com/user/myproject.git",
				LocalRepo: "/home/user/myproject",
			},
		},
	}
	writeTownFixture(t, dir, townCfg, settings, rigs)

	snap, err := ReadTownSnapshot(dir)
	if err != nil {
		t.Fatalf("ReadTownSnapshot: %v", err)
	}

	if snap.Name != "bright-lights" {
		t.Errorf("Name = %q, want %q", snap.Name, "bright-lights")
	}
	if snap.DefaultAgent != "claude" {
		t.Errorf("DefaultAgent = %q, want %q", snap.DefaultAgent, "claude")
	}
	if len(snap.Agents) != 1 {
		t.Errorf("len(Agents) = %d, want 1", len(snap.Agents))
	}
	if snap.Agents["opus-46"] == nil {
		t.Error("Agents[opus-46] is nil")
	}
	if len(snap.RoleAgents) != 2 {
		t.Errorf("len(RoleAgents) = %d, want 2", len(snap.RoleAgents))
	}
	if snap.RoleAgents["mayor"] != "opus-46" {
		t.Errorf("RoleAgents[mayor] = %q, want %q", snap.RoleAgents["mayor"], "opus-46")
	}
	if len(snap.Rigs) != 1 {
		t.Errorf("len(Rigs) = %d, want 1", len(snap.Rigs))
	}
	if snap.Rigs["myproject"].LocalRepo != "/home/user/myproject" {
		t.Errorf("Rigs[myproject].LocalRepo = %q, want %q", snap.Rigs["myproject"].LocalRepo, "/home/user/myproject")
	}
}

func TestReadTownSnapshot_NotATown(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadTownSnapshot(dir)
	if err == nil {
		t.Fatal("expected error for non-town directory")
	}
	if !strings.Contains(err.Error(), "not a Gas Town root") {
		t.Errorf("error = %q, want to contain 'not a Gas Town root'", err.Error())
	}
}

func TestReadTownSnapshot_NoRigs(t *testing.T) {
	dir := t.TempDir()
	townCfg := &config.TownConfig{
		Type:    "town",
		Version: 2,
		Name:    "empty-town",
	}
	settings := config.NewTownSettings()
	writeTownFixture(t, dir, townCfg, settings, nil)

	snap, err := ReadTownSnapshot(dir)
	if err != nil {
		t.Fatalf("ReadTownSnapshot: %v", err)
	}
	if len(snap.Rigs) != 0 {
		t.Errorf("len(Rigs) = %d, want 0", len(snap.Rigs))
	}
}

func TestGenerateCityTOML_Basic(t *testing.T) {
	snap := &TownSnapshot{
		Name:         "bright-lights",
		DefaultAgent: "claude",
		Agents:       map[string]*config.RuntimeConfig{},
		RoleAgents:   map[string]string{},
		Rigs:         map[string]config.RigEntry{},
	}

	got, err := GenerateCityTOML(snap)
	if err != nil {
		t.Fatalf("GenerateCityTOML: %v", err)
	}

	// Verify it's valid TOML.
	var parsed map[string]interface{}
	if _, err := toml.Decode(got, &parsed); err != nil {
		t.Fatalf("output is not valid TOML: %v\n---\n%s", err, got)
	}

	// Check key fields.
	if !strings.Contains(got, `name = "bright-lights"`) {
		t.Error("missing workspace name")
	}
	if !strings.Contains(got, `provider = "claude"`) {
		t.Error("missing workspace provider")
	}
	if !strings.Contains(got, `topology = "gastown"`) {
		t.Error("missing workspace topology")
	}
	if !strings.Contains(got, `[topologies.gastown]`) {
		t.Error("missing topologies section")
	}
	if !strings.Contains(got, `source = "https://github.com/steveyegge/gascity.git"`) {
		t.Error("missing topology source")
	}
	if !strings.Contains(got, `patrol_interval = "30s"`) {
		t.Error("missing daemon patrol_interval")
	}
	// No [[rigs]] section when empty.
	if strings.Contains(got, "[[rigs]]") {
		t.Error("should not have [[rigs]] section when no rigs")
	}
}

func TestGenerateCityTOML_WithRigs(t *testing.T) {
	snap := &TownSnapshot{
		Name:         "test-town",
		DefaultAgent: "claude",
		Agents:       map[string]*config.RuntimeConfig{},
		RoleAgents:   map[string]string{},
		Rigs: map[string]config.RigEntry{
			"alpha": {
				GitURL:    "https://github.com/user/alpha.git",
				LocalRepo: "/home/user/alpha",
			},
			"beta": {
				LocalRepo: "/home/user/beta",
			},
		},
	}

	got, err := GenerateCityTOML(snap)
	if err != nil {
		t.Fatalf("GenerateCityTOML: %v", err)
	}

	// Verify valid TOML.
	var parsed map[string]interface{}
	if _, err := toml.Decode(got, &parsed); err != nil {
		t.Fatalf("output is not valid TOML: %v\n---\n%s", err, got)
	}

	if !strings.Contains(got, "[[rigs]]") {
		t.Error("missing [[rigs]] section")
	}
	if !strings.Contains(got, `name = "alpha"`) {
		t.Error("missing rig alpha")
	}
	if !strings.Contains(got, `name = "beta"`) {
		t.Error("missing rig beta")
	}
	if !strings.Contains(got, `path = "/home/user/alpha"`) {
		t.Error("missing rig alpha path")
	}
	// alpha has git_url, should appear as comment
	if !strings.Contains(got, `# git_url = "https://github.com/user/alpha.git"`) {
		t.Error("missing git_url comment for alpha")
	}
	// beta has no git_url, no comment
	betaSection := got[strings.Index(got, `name = "beta"`):]
	nextRig := strings.Index(betaSection, "[[rigs]]")
	if nextRig > 0 {
		betaSection = betaSection[:nextRig]
	}
	if strings.Contains(betaSection, "git_url") {
		t.Error("beta should not have git_url comment")
	}
}

func TestGenerateCityTOML_WithCustomAgents(t *testing.T) {
	snap := &TownSnapshot{
		Name:         "agent-town",
		DefaultAgent: "claude",
		Agents: map[string]*config.RuntimeConfig{
			"opus-46": {
				Command: "claude",
				Args:    []string{"--model", "opus", "--dangerously-skip-permissions"},
			},
			"gemini": {
				Command:    "gemini",
				Args:       []string{"--sandbox"},
				PromptMode: "none",
				Tmux: &config.RuntimeTmuxConfig{
					ProcessNames:    []string{"gemini"},
					ReadyDelayMs:    5000,
					ReadyPromptPrefix: "❯ ",
				},
			},
		},
		RoleAgents: map[string]string{
			"mayor":   "opus-46",
			"polecat": "claude",
		},
		Rigs: map[string]config.RigEntry{},
	}

	got, err := GenerateCityTOML(snap)
	if err != nil {
		t.Fatalf("GenerateCityTOML: %v", err)
	}

	// Verify valid TOML.
	var parsed map[string]interface{}
	if _, err := toml.Decode(got, &parsed); err != nil {
		t.Fatalf("output is not valid TOML: %v\n---\n%s", err, got)
	}

	// Check provider sections.
	if !strings.Contains(got, "[providers.gemini]") {
		t.Error("missing providers.gemini section")
	}
	if !strings.Contains(got, "[providers.opus-46]") {
		t.Error("missing providers.opus-46 section")
	}
	if !strings.Contains(got, `command = "gemini"`) {
		t.Error("missing gemini command")
	}
	if !strings.Contains(got, `prompt_mode = "none"`) {
		t.Error("missing gemini prompt_mode")
	}
	if !strings.Contains(got, `ready_delay_ms = 5000`) {
		t.Error("missing gemini ready_delay_ms")
	}
	if !strings.Contains(got, `ready_prompt_prefix = "❯ "`) {
		t.Error("missing gemini ready_prompt_prefix")
	}
	if !strings.Contains(got, `process_names = ["gemini"]`) {
		t.Error("missing gemini process_names")
	}

	// Role agents should appear as comments.
	if !strings.Contains(got, "# Role-agent mappings") {
		t.Error("missing role agents comment header")
	}
	if !strings.Contains(got, `# mayor = "opus-46"`) {
		t.Error("missing mayor role agent comment")
	}
	if !strings.Contains(got, `# polecat = "claude"`) {
		t.Error("missing polecat role agent comment")
	}
}

func TestGenerateCityTOML_NoRigs(t *testing.T) {
	snap := &TownSnapshot{
		Name:         "no-rigs",
		DefaultAgent: "claude",
		Agents:       map[string]*config.RuntimeConfig{},
		RoleAgents:   map[string]string{},
		Rigs:         map[string]config.RigEntry{},
	}

	got, err := GenerateCityTOML(snap)
	if err != nil {
		t.Fatalf("GenerateCityTOML: %v", err)
	}

	if strings.Contains(got, "[[rigs]]") {
		t.Error("should not have [[rigs]] section when no rigs")
	}
}

func TestGenerateCityTOML_EmptyName(t *testing.T) {
	snap := &TownSnapshot{
		Name:   "",
		Agents: map[string]*config.RuntimeConfig{},
		Rigs:   map[string]config.RigEntry{},
	}

	_, err := GenerateCityTOML(snap)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestGenerateCityTOML_DefaultProvider(t *testing.T) {
	snap := &TownSnapshot{
		Name:         "test",
		DefaultAgent: "", // empty should default to "claude"
		Agents:       map[string]*config.RuntimeConfig{},
		RoleAgents:   map[string]string{},
		Rigs:         map[string]config.RigEntry{},
	}

	got, err := GenerateCityTOML(snap)
	if err != nil {
		t.Fatalf("GenerateCityTOML: %v", err)
	}
	if !strings.Contains(got, `provider = "claude"`) {
		t.Error("empty DefaultAgent should produce provider = claude")
	}
}

func TestGenerateCityTOML_AgentWithEnv(t *testing.T) {
	snap := &TownSnapshot{
		Name:         "env-town",
		DefaultAgent: "claude",
		Agents: map[string]*config.RuntimeConfig{
			"opencode": {
				Command: "opencode",
				Env: map[string]string{
					"OPENCODE_PERMISSION": "true",
				},
			},
		},
		RoleAgents: map[string]string{},
		Rigs:       map[string]config.RigEntry{},
	}

	got, err := GenerateCityTOML(snap)
	if err != nil {
		t.Fatalf("GenerateCityTOML: %v", err)
	}

	var parsed map[string]interface{}
	if _, err := toml.Decode(got, &parsed); err != nil {
		t.Fatalf("output is not valid TOML: %v\n---\n%s", err, got)
	}

	if !strings.Contains(got, `OPENCODE_PERMISSION = "true"`) {
		t.Error("missing env var in provider section")
	}
}
