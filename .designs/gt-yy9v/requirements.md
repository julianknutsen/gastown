# Requirements: Model Selection by Role

## Original Issue (gh-335)

**Title:** Ability to choose models for different roles

**Problem Statement:**
Currently Opus runs for everything. I'd prefer if only the polecats ran opus and the rest ran on sonet.

**Proposed Solution:**
Role based configuration of model and selection of it. Since claude remembers model selection, model should be passed on every claude code initialisation.

**Alternatives Considered:**
Manually change the model once it starts running.

**Use Case:**
Token optimisation.

---

## Problem Statement

Gas Town currently runs all agents (polecats, witness, refinery, mayor, crew) on the same model (Opus). This is suboptimal because:

1. **Cost inefficiency**: Not all roles require Opus-level reasoning. Supervisory roles (witness), coordination roles (mayor), and routine tasks (refinery merges) could use Sonnet effectively.

2. **No configuration mechanism**: There's no way to specify which model each role should use. The model selection is implicit/hardcoded.

3. **Manual workaround is tedious**: Users must manually switch models after each agent starts, which defeats the automation purpose of Gas Town.

The feature request is to enable role-based model configuration so different agent types can use appropriate models (e.g., polecats on Opus for complex coding, witness on Sonnet for monitoring).

---

## Success Criteria

- [ ] Configuration mechanism exists to specify model per role (polecat, witness, refinery, mayor, crew)
- [ ] Model selection is passed to Claude Code on initialization for each agent
- [ ] Default configuration maintains current behavior (all Opus) for backwards compatibility
- [ ] Users can override defaults via configuration file or environment
- [ ] Model selection persists correctly across agent restarts/respawns
- [ ] Documentation updated to explain model configuration

---

## Constraints

1. **Backwards Compatibility**: Existing setups with no model config must continue working (default to current behavior)

2. **Claude Code API**: Must work within Claude Code's model selection mechanism (`--model` flag or equivalent)

3. **Per-Session Persistence**: Claude remembers model selection per session, so model must be explicitly set on each initialization

4. **Role Awareness**: The spawning mechanism must know which role it's creating to select the appropriate model

5. **Configuration Location**: Should fit Gas Town's existing config patterns (likely in rig config or town-level settings)

---

## Open Questions

1. **Granularity**: Should this be per-role, per-rig, or per-polecat configurable? (e.g., can different polecats in the same rig use different models?)

2. **Config Format**: Where should model mappings live? Options:
   - Town-level config (`~/gt/.config/models.json`)
   - Rig-level config (`gastown/.config/models.json`)
   - Environment variables (`GT_MODEL_POLECAT=opus`)
   - Combination with inheritance

3. **Valid Models**: Which models should be supported? Current Claude models:
   - `claude-opus-4-5-20251101` (opus)
   - `claude-sonnet-4-20250514` (sonnet)
   - `claude-haiku-3-5-20241022` (haiku)

4. **Runtime Override**: Should there be a way to override the configured model at spawn time? (e.g., `gt spawn polecat --model=haiku`)

5. **Validation**: Should Gas Town validate model names or pass them through to Claude Code?

6. **Cost Tracking**: Is there interest in tracking/reporting model usage per role for cost visibility?
