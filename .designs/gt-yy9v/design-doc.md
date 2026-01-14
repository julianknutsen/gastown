# Design: Model Selection by Role (gt-yy9v)

> **TL;DR**: Add `agents` with `--model` flags and `role_agents` mappings to `~/gt/settings/config.json`. No code changes needed. Works today.

## Quick Start

Copy this to `~/gt/settings/config.json`:

```json
{
  "type": "town-settings",
  "version": 1,
  "agents": {
    "claude-opus": {"command": "claude", "args": ["--model", "opus", "--dangerously-skip-permissions"]},
    "claude-sonnet": {"command": "claude", "args": ["--model", "sonnet", "--dangerously-skip-permissions"]}
  },
  "role_agents": {
    "polecat": "claude-opus",
    "witness": "claude-sonnet",
    "refinery": "claude-sonnet",
    "crew": "claude-sonnet"
  }
}
```

Restart Gas Town. Polecats now use Opus, everything else uses Sonnet.

---

## Executive Summary

This design enables Gas Town users to configure different Claude models (Opus, Sonnet, Haiku) for different agent roles (polecat, witness, refinery, etc.) to optimize costs without sacrificing capability where it matters.

The recommended approach leverages existing infrastructure: define agent variants with model-specific `--model` flags, then use the existing `role_agents` configuration to map roles to these variants. **This requires no code changes** - users can start using this feature immediately.

The alternative approach (adding explicit `RoleModels` config) offers more flexibility but requires code changes and schema version bumps. Given the feature request's simplicity, the preset-based approach is recommended.

## Problem Statement

Currently Gas Town runs all agents on Claude's default model (Opus). Users want to:

1. **Reduce costs** by using cheaper models (Sonnet, Haiku) for roles that don't need Opus-level reasoning
2. **Configure per-role** so polecats (doing complex coding) use Opus while witnesses (monitoring) use Sonnet
3. **Set and forget** without manual intervention at each agent spawn

The user's specific request: "I'd prefer if only the polecats ran opus and the rest ran on sonnet."

## Proposed Design

### Overview

**How It Works**

1. Define custom "agent variants" with different models in config
2. Assign these variants to roles (polecat, witness, etc.)
3. When an agent spawns, it uses the model configured for its role

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│ config.json     │────>│ role_agents      │────>│ Command Built   │
│ agents:         │     │ polecat: opus    │     │ claude --model  │
│   "claude-opus" │     │ witness: sonnet  │     │   opus          │
│   "claude-sonnet"│    │ refinery: haiku  │     └─────────────────┘
└─────────────────┘     └──────────────────┘
```

### Key Components

**Single Config File** (`~/gt/settings/config.json`)

Define model-specific agents in `agents` map, then reference them in `role_agents`:

```json
{
  "type": "town-settings",
  "version": 1,
  "default_agent": "claude",
  "agents": {
    "claude-opus": {
      "command": "claude",
      "args": ["--model", "opus", "--dangerously-skip-permissions"]
    },
    "claude-sonnet": {
      "command": "claude",
      "args": ["--model", "sonnet", "--dangerously-skip-permissions"]
    },
    "claude-haiku": {
      "command": "claude",
      "args": ["--model", "haiku", "--dangerously-skip-permissions"]
    }
  },
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

Note: Use short model names (`opus`, `sonnet`, `haiku`) - Claude Code normalizes them.

**Resolution Hierarchy** (how Gas Town picks which agent to use)
1. Rig-level `role_agents` override
2. Town-level `role_agents`
3. Rig-level `default_agent`
4. Town-level `default_agent`
5. Fallback: "claude"

### Interface

**Configuration-based** (recommended):
- Edit `~/gt/settings/config.json` to define agents and map roles
- Single file contains both agent definitions (`agents`) and role mappings (`role_agents`)
- No CLI commands needed

**Optional CLI enhancements** (future):
```bash
gt config models           # Show role → model mappings
gt config set-model <role> <model>  # Set mapping
```

### Data Model

**No schema changes required** for Option A.

Existing types support this:
- `TownSettings.Agents` map defines custom agent configs with `RuntimeConfig`
- `TownSettings.RoleAgents` maps role → agent name (built-in or custom)
- `RigSettings.Agents` / `RigSettings.RoleAgents` provide per-rig overrides
- `RuntimeConfig.Args` holds `["--model", "opus", ...]`

## Trade-offs and Decisions

### Decisions Made

| Decision | Rationale |
|----------|-----------|
| Use agent presets (Option A) | Leverages existing infrastructure, no code changes |
| Town + Rig level config | Follows existing resolution pattern |
| No runtime model switching | Simplicity; model set at spawn time |
| No per-polecat models | Complexity not justified; per-role is sufficient |

### Open Questions (Need Human Input)

1. **⚠️ Should we ship pre-defined model presets?**
   - Current: Users must add agent variants to their `config.json`
   - Alternative: Ship `claude-opus`, `claude-sonnet`, `claude-haiku` as built-in agents
   - Trade-off: Convenience vs. maintenance burden when model names change

2. **⚠️ Status display enhancement?**
   - Current: `gt status` doesn't show model
   - Proposal: Add model name to status output
   - Trade-off: Nice-to-have vs. code change

3. **⚠️ Cost estimation feature?**
   - Future: Show estimated cost impact of model choices
   - Trade-off: Useful but complex (need pricing data)

### Trade-offs

| Trade-off | Chosen | Alternative | Why |
|-----------|--------|-------------|-----|
| Config complexity | Simple (agent presets) | Explicit RoleModels | Works today, no code changes |
| Granularity | Per-role | Per-polecat | Sufficient for use case |
| Validation | At Claude | At Gas Town | Less code to maintain |

## Edge Cases

| Scenario | What Happens | Recommendation |
|----------|--------------|----------------|
| `role_agents` references undefined agent | Falls back to default agent | Log warning, continue |
| Invalid model name in args | Claude rejects at startup | Clear error: "model not found" |
| Old Claude Code without --model | Flag ignored, uses default | Document minimum version |
| Multiple --model flags in args | Last one wins (Claude behavior) | Document: avoid duplicates |
| Different models per polecat | Not supported | Per-role is sufficient for MVP |
| Empty args array | Uses Claude defaults | Works correctly |
| Rig overrides town config | Rig wins (by design) | Expected behavior |
| Config file doesn't exist | Uses built-in defaults | Everything works |
| JSON syntax error in config | Load fails | Clear parse error message |

### Migration Edge Cases

- **No existing config**: Works immediately - defaults to built-in `claude` agent
- **Existing config without model agents**: Add `agents` and `role_agents` sections; existing settings preserved
- **Partial role_agents**: Unlisted roles use `default_agent` or fallback

## Risks and Mitigations

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| Typo in model name | Agent fails to start | Medium | Clear error from Claude |
| Cost misconfiguration | Higher bills | Low | Documentation, cost reports |
| Model name changes | Config breaks | Low | Update docs when models change |
| Config file errors | JSON parse failure | Medium | Validate on load, warn |
| Reference to undefined agent | Unexpected default model | Medium | Log warning at startup |

## Implementation Plan

### Phase 1: MVP (No Code Changes)

**Deliverables:**
- [ ] Example `config.json` with model-specific agents and role mappings
- [ ] Documentation in README explaining how to configure models by role
- [ ] Cost optimization guide with recommended configurations

**Effort:** Documentation only

### Phase 2: Polish (Minor Code Changes)

**Deliverables:**
- [ ] Show model in `gt status` output
- [ ] Show model in `gt peek` output
- [ ] Model breakdown in `gt costs`
- [ ] Built-in model presets (if decided)

**Effort:** ~50 lines of code changes

### Phase 3: Future (If Needed)

**Potential enhancements:**
- `gt config set-model` CLI command
- `--model` flag on `gt sling`
- Cost estimation before spawn
- Per-rig cost limits

## Appendix: Dimension Analyses

- [API & Interface Design](api.md) - CLI and config interface design
- [Data Model](data.md) - Schema and storage design
- [User Experience](ux.md) - User workflow and learning curve
- [Scalability](scale.md) - Performance and resource analysis
- [Security](security.md) - Trust boundaries and validation
- [Integration](integration.md) - Codebase integration and testing

---

*Design document for GitHub issue #335: "Ability to choose models for different roles"*
