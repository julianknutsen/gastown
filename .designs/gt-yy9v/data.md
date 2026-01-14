# Data Model Design: Model Selection by Role

## Data Structures

### Option A: Model via Agent Presets (Recommended)

No new data structures needed. Leverage existing types:

```go
// AgentPresetInfo (existing, unchanged)
type AgentPresetInfo struct {
    Name    AgentPreset
    Command string
    Args    []string  // <-- Model goes here: ["--model", "opus-4.5"]
    // ... other fields
}

// TownSettings.RoleAgents (existing, unchanged)
RoleAgents map[string]string  // {"polecat": "claude-opus"}

// RigSettings.RoleAgents (existing, unchanged)
RoleAgents map[string]string  // Override per-rig
```

### Option B: Explicit Model Field

New field in existing structs:

```go
// TownSettings (extended)
type TownSettings struct {
    // ...existing fields...
    RoleModels map[string]string `json:"role_models,omitempty"`
}

// RigSettings (extended)
type RigSettings struct {
    // ...existing fields...
    RoleModels map[string]string `json:"role_models,omitempty"`
}
```

### Model Name Validation

```go
// Valid model identifiers
var ValidModels = []string{
    "opus-4.5", "opus",      // Full and short form
    "sonnet-4", "sonnet",
    "haiku-3.5", "haiku",
}

// Normalization: short → full
func NormalizeModel(m string) string {
    switch m {
    case "opus":
        return "opus-4.5"
    case "sonnet":
        return "sonnet-4"
    case "haiku":
        return "haiku-3.5"
    default:
        return m
    }
}
```

## Storage Format

### JSON Configuration Files

All model configuration stored in existing JSON config files:

**Town-level agents** (`~/gt/settings/agents.json`):
```json
{
  "version": 1,
  "agents": {
    "claude-opus": {
      "name": "claude-opus",
      "command": "claude",
      "args": ["--dangerously-skip-permissions", "--model", "opus-4.5"],
      "process_names": ["node"],
      "session_id_env": "CLAUDE_SESSION_ID",
      "resume_flag": "--resume",
      "resume_style": "flag",
      "supports_hooks": true
    },
    "claude-sonnet": {
      "name": "claude-sonnet",
      "command": "claude",
      "args": ["--dangerously-skip-permissions", "--model", "sonnet-4"],
      "process_names": ["node"],
      "session_id_env": "CLAUDE_SESSION_ID",
      "resume_flag": "--resume",
      "resume_style": "flag",
      "supports_hooks": true
    }
  }
}
```

**Town-level settings** (`~/gt/settings/config.json`):
```json
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

**Rig-level settings** (`<rig>/settings/config.json`):
```json
{
  "type": "rig-settings",
  "version": 1,
  "role_agents": {
    "polecat": "claude-opus"
  }
}
```

### No Database Changes

- No SQLite changes needed
- No new tables
- Beads system unchanged
- Model selection is purely configuration

## Schema Design

### Option A Schema (Agent Presets)

No schema changes. Existing AgentRegistry schema supports this:

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "version": {"type": "integer"},
    "agents": {
      "type": "object",
      "additionalProperties": {
        "type": "object",
        "properties": {
          "name": {"type": "string"},
          "command": {"type": "string"},
          "args": {"type": "array", "items": {"type": "string"}},
          "process_names": {"type": "array", "items": {"type": "string"}},
          "session_id_env": {"type": "string"},
          "resume_flag": {"type": "string"},
          "resume_style": {"type": "string"},
          "supports_hooks": {"type": "boolean"},
          "supports_fork_session": {"type": "boolean"}
        }
      }
    }
  }
}
```

### Option B Schema (RoleModels)

Would add to TownSettings/RigSettings:

```json
{
  "properties": {
    "role_models": {
      "type": "object",
      "additionalProperties": {
        "type": "string",
        "enum": ["opus-4.5", "opus", "sonnet-4", "sonnet", "haiku-3.5", "haiku"]
      }
    }
  }
}
```

## Migration Strategy

### Option A: No Migration Needed

Existing configs continue to work. Users opt-in by:
1. Adding agent presets to `settings/agents.json`
2. Updating `role_agents` in `settings/config.json`

### Option B: Version Bump

```go
// Bump TownSettings version
const CurrentTownSettingsVersion = 2  // Was 1

// Migration: v1 → v2
func MigrateTownSettings(v1 *TownSettingsV1) *TownSettings {
    return &TownSettings{
        Type:         v1.Type,
        Version:      2,
        DefaultAgent: v1.DefaultAgent,
        Agents:       v1.Agents,
        RoleAgents:   v1.RoleAgents,
        RoleModels:   make(map[string]string),  // New field, empty default
    }
}
```

### Backwards Compatibility

Both options maintain backwards compatibility:
- Empty `RoleAgents` / `RoleModels` → use defaults
- Missing config files → use built-in presets
- Old configs without model specs → Claude uses its default model

## Data Lifecycle

### Creation
- User edits `settings/agents.json` to define presets
- User edits `settings/config.json` to assign presets to roles
- Files created manually or via `gt config` commands

### Updates
- User edits config files
- Changes take effect on next agent spawn
- Running agents unaffected until restart

### Deletion
- Remove entries from config files
- Fall back to defaults

### No Persistence of Runtime State
- Model selection is not stored at runtime
- Each spawn reads config fresh
- No caching of model assignments

## Data Growth

### Static Data
- Config files are small (<1KB typically)
- Number of roles is fixed (6)
- Number of agent presets is bounded

### No Growth Concerns
- This feature adds no new data that grows over time
- Config size is O(roles × agents) = O(6 × ~10) = O(60)

## Access Patterns

### Read
- On agent spawn: read config, resolve role → agent → model
- On status display: read config to show current mappings
- On config validation: read and validate all entries

### Write
- User-initiated only (editing config files)
- No automated writes
- No concurrent write concerns

### Query Pattern
```go
// Primary query: role → model
model := ResolveRoleModel(role, rigSettings, townSettings)

// Secondary query: list all role mappings
mappings := ListRoleModelMappings(rigSettings, townSettings)
```
