# API & Interface Design: Model Selection by Role

## Command-Line Interface

### Option A: Using Existing RoleAgents (Recommended)

No new CLI flags needed. Users define agent presets with models baked in:

```bash
# Town-level config: ~/gt/settings/agents.json
{
  "version": 1,
  "agents": {
    "claude-opus": {
      "command": "claude",
      "args": ["--dangerously-skip-permissions", "--model", "opus-4.5"]
    },
    "claude-sonnet": {
      "command": "claude",
      "args": ["--dangerously-skip-permissions", "--model", "sonnet-4"]
    },
    "claude-haiku": {
      "command": "claude",
      "args": ["--dangerously-skip-permissions", "--model", "haiku-3.5"]
    }
  }
}

# Town-level settings: ~/gt/settings/config.json
{
  "type": "town-settings",
  "version": 1,
  "default_agent": "claude-opus",
  "role_agents": {
    "polecat": "claude-opus",
    "witness": "claude-sonnet",
    "refinery": "claude-haiku",
    "mayor": "claude-sonnet",
    "deacon": "claude-sonnet",
    "crew": "claude-sonnet"
  }
}
```

### Option B: New --model Flag (Alternative)

Add explicit model flag to spawn commands:

```bash
# Override at spawn time
gt sling <bead> <rig> --model sonnet

# Show current model config
gt config models

# Set role model
gt config set role-model polecat opus
gt config set role-model witness sonnet
```

### Subcommand Ergonomics

If implementing Option B, new commands would be:

```bash
# View all model assignments
gt config models
# Output:
# Role       Model         Source
# ----       -----         ------
# polecat    opus-4.5      rig (gastown)
# witness    sonnet-4      town
# refinery   haiku-3.5     default
# crew       sonnet-4      town

# Set model for a role
gt config set-model <role> <model>

# List available models
gt config list-models
# Output: opus-4.5, sonnet-4, haiku-3.5
```

## Programmatic API

### Configuration Interface (Go)

For Option A (agent presets with models):
```go
// No API changes needed - use existing RoleAgents

// Get agent for role (existing function)
func ResolveRoleAgentConfig(role string, rigSettings *RigSettings, townSettings *TownSettings) (*AgentPresetInfo, error)
```

For Option B (explicit model field):
```go
// New function to resolve model
func ResolveRoleModel(role string, rigSettings *RigSettings, townSettings *TownSettings) string

// Extended TownSettings
type TownSettings struct {
    // ...existing fields...
    RoleModels map[string]string `json:"role_models,omitempty"`
}

// Extended RigSettings
type RigSettings struct {
    // ...existing fields...
    RoleModels map[string]string `json:"role_models,omitempty"`
}
```

### Return Types

Model resolution returns a simple string:
- `"opus-4.5"` | `"sonnet-4"` | `"haiku-3.5"` | `""`

Empty string means "use Claude's default" (currently Opus 4.5).

## Configuration Interface

### File Locations

| Level | File | Purpose |
|-------|------|---------|
| Town | `~/gt/settings/agents.json` | Custom agent definitions with model args |
| Town | `~/gt/settings/config.json` | Default agent and role_agents mapping |
| Rig | `<rig>/settings/agents.json` | Rig-specific agent definitions |
| Rig | `<rig>/settings/config.json` | Rig-specific overrides |

### Environment Variables

For runtime override (useful for debugging/testing):
```bash
GT_MODEL_OVERRIDE=haiku  # Forces all roles to use haiku
```

Not recommended for production use - config files are better for persistence.

## Error Messages

### Invalid Model Name
```
Error: unknown model "gpt4"
Available models: opus-4.5, sonnet-4, haiku-3.5
```

### Invalid Role Name
```
Error: unknown role "worker"
Valid roles: polecat, witness, refinery, mayor, deacon, crew
```

### Config Validation
```
Warning: role_agents references undefined agent "claude-fast"
Available agents: claude, claude-opus, claude-sonnet, claude-haiku, gemini, codex
```

## Help Text

```
MODEL SELECTION

Gas Town supports per-role model selection for cost optimization.

Configuration:
  Define agent presets with model args in ~/gt/settings/agents.json:

  {
    "agents": {
      "claude-opus": {"command": "claude", "args": ["--model", "opus-4.5"]},
      "claude-sonnet": {"command": "claude", "args": ["--model", "sonnet-4"]}
    }
  }

  Then assign to roles in ~/gt/settings/config.json:

  {
    "role_agents": {
      "polecat": "claude-opus",
      "witness": "claude-sonnet"
    }
  }

Resolution order:
  1. Rig role_agents (if set)
  2. Town role_agents (if set)
  3. Rig default_agent (if set)
  4. Town default_agent (if set)
  5. Built-in default: "claude"
```

## Naming Conventions

### Model Names
Use Claude Code's short names for ergonomics:
- `opus` → `opus-4.5` (aliases work)
- `sonnet` → `sonnet-4`
- `haiku` → `haiku-3.5`

### Agent Preset Names
Convention: `<cli>-<model>` for model variants:
- `claude-opus` (Claude with Opus)
- `claude-sonnet` (Claude with Sonnet)
- `claude-haiku` (Claude with Haiku)

## Discoverability

### How Users Learn This Feature

1. **Documentation**: Add section to Gas Town README and `gt help config`
2. **Example configs**: Ship example `agents.json` with model presets
3. **Status command**: `gt status` shows current model for each active agent
4. **Cost reporting**: `gt costs` shows model breakdown (existing feature)

### Happy Path

1. User wants to reduce costs
2. Runs `gt help config` or reads docs
3. Copies example agent definitions to settings
4. Maps roles to agents via `role_agents`
5. Starts Gas Town - polecats run on Opus, others on Sonnet/Haiku

### Edge Cases

1. **Undefined agent**: Config references agent not in registry → warning, fall back to default
2. **Invalid model**: Claude rejects model name → agent fails to start, error in logs
3. **Mixed models in rig**: Different polecats in same rig can't use different models (by design)

## Consistency with Existing Interfaces

### Follows Existing Patterns

- Uses same config file locations (`settings/config.json`, `settings/agents.json`)
- Uses same resolution hierarchy (rig → town → default)
- Uses same JSON schema patterns (`map[string]string` for mappings)
- Uses existing `RoleAgents` field (Option A)

### Changes from Existing Patterns

- Option B would add a new `RoleModels` field (parallel to `RoleAgents`)
- New CLI flags if implementing `gt config set-model`
