# Codebase Context: Model Selection by Role

## Relevant Files

| File | Purpose | Relevance |
|------|---------|-----------|
| `internal/config/types.go` | Config type definitions | Contains `TownSettings`, `RigSettings` with `RoleAgents` map - infrastructure exists but for agents, not models |
| `internal/config/agents.go` | Agent registry and presets | Defines agent presets (claude, gemini, etc.) and `AgentPresetInfo` - could be extended for model selection |
| `internal/config/loader.go` | Config loading and resolution | `ResolveRoleAgentConfig()` resolves role → agent mapping; command building would need `--model` injection |
| `internal/cmd/polecat_spawn.go` | Polecat spawning | `SpawnPolecatForSling()` creates polecats; uses `BuildPolecatStartupCommandWithAgentOverride()` |
| `internal/cmd/start.go` | Agent startup orchestration | Manager `Start()` methods for each role type |
| `internal/polecat/session_manager.go` | Session management | `SessionStartOptions` - could pass model selection here |
| `internal/cmd/sling.go` | Work dispatch | `gt sling` supports `--agent` flag, could add `--model` |

## Existing Patterns

### Role-Based Agent Selection (Already Exists)

The codebase already supports per-role agent selection via `RoleAgents`:

```go
// types.go lines 53-58 (TownSettings)
RoleAgents map[string]string `json:"role_agents,omitempty"`
// Example: {"mayor": "claude", "witness": "gemini", "polecat": "claude"}

// types.go lines 221-226 (RigSettings)
RoleAgents map[string]string `json:"role_agents,omitempty"`
// Example: {"witness": "claude-haiku", "polecat": "claude-sonnet"}
```

### Resolution Hierarchy

Config resolution follows this precedence (loader.go):
1. `agentOverride` flag (e.g., `--agent gemini`)
2. Rig-level `RigSettings.Agent` or `RigSettings.RoleAgents[role]`
3. Town-level `TownSettings.DefaultAgent` or `TownSettings.RoleAgents[role]`
4. Ultimate fallback: `"claude"`

### Command Building

Startup commands are built with:
```go
// loader.go line 1294
BuildPolecatStartupCommand(rigName, polecatName, rigPath, prompt string) string
// Results in: "claude --dangerously-skip-permissions ..."
```

The `Args` array from `AgentPresetInfo` or `RuntimeConfig` gets appended to the command.

## Key Data Structures

### AgentPresetInfo (agents.go)
```go
type AgentPresetInfo struct {
    Name    AgentPreset // "claude"
    Command string      // "claude"
    Args    []string    // ["--dangerously-skip-permissions"]
    // ... other fields
}
```

### RuntimeConfig (types.go)
```go
type RuntimeConfig struct {
    Command string   // "claude"
    Args    []string // ["--dangerously-skip-permissions"]
    // ... other fields
}
```

**Key insight**: `Args` is where `--model opus` would be injected.

### TownSettings / RigSettings
Both have:
- `DefaultAgent string` - which agent preset to use
- `Agents map[string]*RuntimeConfig` - custom agent definitions
- `RoleAgents map[string]string` - role → agent mapping

## Integration Points

### Where Model Selection Could Be Injected

1. **At Config Level** - New field in `TownSettings`/`RigSettings`:
   ```go
   RoleModels map[string]string `json:"role_models,omitempty"`
   // {"polecat": "opus", "witness": "sonnet", "refinery": "haiku"}
   ```

2. **At Agent Definition Level** - Add `Model` to `AgentPresetInfo`:
   ```go
   type AgentPresetInfo struct {
       // ...existing fields...
       Model string `json:"model,omitempty"` // "opus", "sonnet", "haiku"
   }
   ```

3. **At Command Building** - Inject `--model` in `BuildStartupCommand`:
   ```go
   // In BuildStartupCommand or normalizeRuntimeConfig
   if model != "" {
       args = append(args, "--model", model)
   }
   ```

4. **At Spawn Time** - Add `--model` flag to `gt sling`:
   ```bash
   gt sling <bead> <rig> --model sonnet
   ```

### Claude Code --model Flag

Claude Code accepts `--model` flag:
```bash
claude --model opus-4.5          # Use Opus 4.5
claude --model sonnet-4          # Use Sonnet 4
claude --model haiku-3.5         # Use Haiku 3.5
```

This is the mechanism to leverage.

## Constraints from Code

1. **Backwards Compatibility**: `RoleAgents` already exists - new model config must coexist
2. **Command Building**: All paths must flow through `BuildStartupCommand*` functions
3. **Agent Presets**: Model could be per-preset (define "claude-opus", "claude-sonnet") OR per-role
4. **Session Persistence**: Claude remembers model per session - must set on every init
5. **Multiple Resolution Layers**: Town → Rig → Role → Override hierarchy must be maintained

## Recommended Approach

Based on the existing patterns, the cleanest approach is:

**Option A: Model as Agent Variant** (Minimal Changes)
- Define agent presets: `"claude-opus"`, `"claude-sonnet"`, `"claude-haiku"`
- Use existing `RoleAgents` map: `{"polecat": "claude-opus", "witness": "claude-sonnet"}`
- Add `--model` to preset Args

**Option B: Separate RoleModels Config** (More Flexible)
- Add `RoleModels map[string]string` to `TownSettings`/`RigSettings`
- Resolve model separately from agent
- Inject `--model` in command building

**Option C: Model in RuntimeConfig** (Most Granular)
- Add `Model string` field to `RuntimeConfig`
- Allow per-agent model specification
- Most flexible but more complex

Given the issue request ("role-based configuration"), **Option A** is simplest and works with existing infrastructure.
