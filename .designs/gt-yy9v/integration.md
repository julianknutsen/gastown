# Integration Analysis: Model Selection by Role

## Existing Components Affected

### Config Package (`internal/config/`)

| File | Changes | Impact |
|------|---------|--------|
| `agents.go` | None (Option A) or minor | Ship preset definitions |
| `types.go` | None (Option A) | No struct changes |
| `loader.go` | Minor | Inject `--model` in command building |

### Command Package (`internal/cmd/`)

| File | Changes | Impact |
|------|---------|--------|
| `start.go` | None | Managers already use agent config |
| `sling.go` | Optional | Add `--model` flag override |
| `polecat_spawn.go` | None | Uses existing config resolution |

### Role Managers

| Component | Location | Changes |
|-----------|----------|---------|
| Polecat Manager | `internal/polecat/` | None |
| Witness Manager | `internal/witness/` | None |
| Refinery Manager | `internal/refinery/` | None |
| Crew Manager | `internal/crew/` | None |
| Mayor Manager | `internal/mayor/` | None |
| Deacon Manager | `internal/deacon/` | None |

All managers use `config.ResolveRoleAgentConfig()` which handles resolution.

## Dependencies

### What This Feature Needs

| Dependency | Purpose | Status |
|------------|---------|--------|
| Config loading | Read role → agent mapping | Exists |
| Agent registry | Look up agent presets | Exists |
| Command building | Inject `--model` flag | Needs enhancement |
| Claude Code | Accept `--model` flag | Exists |

### What This Feature Adds

| New Capability | Consumers |
|----------------|-----------|
| Per-role model selection | All role managers |
| Model name validation | Config loading |
| Model in status display | `gt status`, `gt peek` |

## Dependents

### What Will Depend on This

| Component | How It Uses Model Selection |
|-----------|----------------------------|
| Cost reporting | Show model breakdown |
| Status display | Show current model per role |
| Logs | Include model in spawn logs |
| Documentation | Explain configuration |

### No Breaking Changes to Dependents

Existing code that doesn't care about models:
- Continues to work unchanged
- Gets default model (Opus)
- No API changes required

## Migration Path

### From Current State to Target State

**Phase 1: Ship Example Configs** (No code changes)
1. Create example `agents.json` with model presets
2. Create example `config.json` with role mappings
3. Document in README
4. Users can opt-in immediately

**Phase 2: Enhance Status Display** (Minor code change)
1. Add model name to `gt status` output
2. Add model name to `gt peek` output
3. Add model breakdown to `gt costs`

**Phase 3: Optional CLI Enhancements** (If desired)
1. Add `--model` flag to `gt sling`
2. Add `gt config set-model` command
3. Add `gt config models` command

### Migration Commands

For users migrating:
```bash
# Copy example configs
cp examples/agents.json ~/gt/settings/
cp examples/config.json ~/gt/settings/

# Verify
gt status  # Should show model per role

# No restart needed - takes effect on next spawn
```

## Backwards Compatibility

### What Might Break

**Nothing breaks** because:
- Empty `role_agents` → use defaults
- Missing config → use defaults
- Unknown agent in `role_agents` → warning + defaults

### Compatibility Matrix

| Old Config | New Code | Result |
|------------|----------|--------|
| No config | ✓ | Use defaults (Opus) |
| Old config, no model | ✓ | Use defaults (Opus) |
| Old config, with model | ✓ | Use specified model |

### Deprecation Path

None needed - no APIs removed or changed.

## Testing Strategy

### Unit Tests

```go
// Test role resolution
func TestResolveRoleAgentConfig(t *testing.T) {
    tests := []struct {
        name     string
        role     string
        rigCfg   *RigSettings
        townCfg  *TownSettings
        expected string
    }{
        {"default", "polecat", nil, nil, "claude"},
        {"town-level", "witness", nil, &TownSettings{RoleAgents: map[string]string{"witness": "claude-sonnet"}}, "claude-sonnet"},
        {"rig-override", "witness", &RigSettings{RoleAgents: map[string]string{"witness": "claude-haiku"}}, &TownSettings{RoleAgents: map[string]string{"witness": "claude-sonnet"}}, "claude-haiku"},
    }
    // ...
}
```

### Integration Tests

```go
// Test full spawn with model
func TestSpawnWithModel(t *testing.T) {
    // Setup config with model preset
    // Spawn polecat
    // Verify command includes --model flag
    // Verify correct model name
}
```

### E2E Tests

```bash
# Test model selection in real spawn
gt sling test-issue gastown
gt peek gastown/polecat  # Verify model shown
```

### Manual Testing Checklist

- [ ] Default config → Opus for all roles
- [ ] Town-level role_agents → correct model
- [ ] Rig-level override → overrides town
- [ ] Invalid agent name → warning, fallback
- [ ] Invalid model name → Claude error, clear message
- [ ] `gt status` shows model
- [ ] `gt costs` shows model breakdown

## Where Does Code Live

### Option A Implementation Locations

| Change | File | Lines |
|--------|------|-------|
| Example agents.json | `examples/agents.json` | New file |
| Example config.json | `examples/config.json` | New file |
| Status display | `internal/cmd/status.go` | ~10 lines |
| Peek display | `internal/cmd/peek.go` | ~10 lines |
| Documentation | `README.md` | ~50 lines |

### Option B Implementation Locations

| Change | File | Lines |
|--------|------|-------|
| RoleModels field | `internal/config/types.go` | ~10 lines |
| ResolveRoleModel | `internal/config/loader.go` | ~30 lines |
| Model validation | `internal/config/agents.go` | ~20 lines |
| CLI flag | `internal/cmd/sling.go` | ~15 lines |
| All Option A changes | Various | ~70 lines |

## Feature Flag / Gradual Rollout

### Not Needed

This feature is:
- Opt-in via configuration
- No impact if not configured
- Backwards compatible

Users "roll out" by:
1. Adding config files
2. Model selection takes effect on next spawn
3. Can revert by removing config

### If Feature Flag Was Desired

```go
// In config
type TownSettings struct {
    EnableRoleModels bool `json:"enable_role_models,omitempty"`
}

// In resolution
if !townSettings.EnableRoleModels {
    return defaultAgent
}
```

Not recommended - adds complexity without benefit.

## Rollout Plan

### Phase 1: Documentation + Examples (Week 1)
- Write configuration documentation
- Create example config files
- Update README

### Phase 2: Status Enhancements (Week 2)
- Add model to `gt status`
- Add model to `gt peek`
- Add model breakdown to `gt costs`

### Phase 3: CLI Enhancements (Optional)
- Add `--model` flag to `gt sling`
- Add `gt config set-model`
- Add `gt config models`

### Phase 4: Validation
- User feedback
- Iterate on UX
- Finalize documentation
