# User Experience Analysis: Model Selection by Role

## Mental Model

### How Users Think About This

Users conceptualize model selection as:
- **Cost tiers**: Opus = expensive/smart, Sonnet = balanced, Haiku = cheap/fast
- **Role importance**: Polecats do "real work" → need best model. Witnesses "just watch" → cheaper is fine.
- **Set and forget**: Configure once, don't think about it again

### Mapping to Reality

| User Mental Model | System Reality |
|-------------------|----------------|
| "Use Opus for polecats" | `role_agents: {polecat: "claude-opus"}` |
| "Save money on witnesses" | `role_agents: {witness: "claude-haiku"}` |
| "This rig needs smarter agents" | Rig-level override in `settings/config.json` |

### Key Insight

Users don't want to think about "agent presets" - they want to think about:
1. **What role** (polecat, witness, etc.)
2. **What model** (opus, sonnet, haiku)

The agent preset abstraction (Option A) is an implementation detail they shouldn't need to understand.

## Workflow Integration

### Where This Fits in Daily Use

**Initial Setup** (one-time):
1. User sets up Gas Town
2. Optionally configures model preferences
3. Most users will use defaults initially

**Cost Optimization** (when needed):
1. User notices high API costs
2. Runs `gt costs` to see breakdown
3. Discovers witnesses using Opus
4. Reconfigures to use cheaper models for non-critical roles

**Per-Project Tuning** (advanced):
1. User has complex project needing smart polecats
2. Simple project just needs basic automation
3. Configures rig-level overrides for different projects

### Typical Workflow

```
Day 1: Install Gas Town, use defaults (all Opus)
       ↓
Week 2: Check costs, realize witness doesn't need Opus
       ↓
Week 2: Edit config to use Sonnet for witness/refinery
       ↓
Month 2: Everything works, forget about it
```

## Learning Curve

### Progressive Disclosure

**Level 1 - Beginner** (default):
- Don't configure anything
- Everything works on defaults
- Maybe expensive, but functional

**Level 2 - Cost-Conscious**:
- Learn about `role_agents` in settings
- Copy example config
- Understand role → model mapping

**Level 3 - Power User**:
- Create custom agent presets
- Per-rig overrides
- Understand resolution hierarchy

### Discovery Path

1. **Documentation**: README mentions cost optimization
2. **Cost report**: `gt costs` shows model breakdown
3. **Help**: `gt help config` explains settings
4. **Example**: Ship example config with comments
5. **Status**: `gt status` shows current model per role

## Error Experience

### What Happens When Things Go Wrong

**Typo in model name**:
```
$ gt start
Error: Agent "claude-opus" specifies unknown model "opsu-4.5"
Did you mean "opus-4.5"?
```

**Undefined agent reference**:
```
$ gt start
Warning: role_agents references undefined agent "claude-fast"
Falling back to default agent: claude
```

**Claude rejects model** (at runtime):
```
# In agent session
Error: Invalid model "opus-5.0" - model not available
```

### Error Recovery

| Error | Recovery |
|-------|----------|
| Typo in config | Edit config, restart agent |
| Invalid model | Claude fails with clear error, agent doesn't start |
| Missing config | Falls back to defaults, everything works |

### Fail-Safe Design

- Invalid config → fall back to defaults
- Don't crash, warn and continue
- Log configuration issues for debugging

## Feedback

### How Users Know It's Working

**At Startup**:
```
$ gt start
Starting gastown rig...
  witness: claude-sonnet (sonnet-4)
  refinery: claude-haiku (haiku-3.5)
  crew/max: claude-sonnet (sonnet-4)
```

**In Status**:
```
$ gt status
Rig: gastown
Polecats:
  Toast [opus-4.5] - Working on gt-123
  Nux [opus-4.5] - Idle
Witness: [sonnet-4] - Patrolling
Refinery: [haiku-3.5] - Idle
```

**In Costs**:
```
$ gt costs --today
Model         Tokens      Cost
opus-4.5      150,000    $4.50
sonnet-4       50,000    $0.30
haiku-3.5      20,000    $0.02
Total                    $4.82
```

### Visual Cues

- Model shown in tmux status bar (optional)
- Model shown in `gt peek` output
- Model breakdown in cost reports

## User Goals

### Primary Goals

1. **Reduce costs** without sacrificing quality where it matters
2. **Simple configuration** - don't want to be a Gas Town expert
3. **Predictable behavior** - same config = same behavior

### Secondary Goals

1. **Experiment** with different models for different tasks
2. **Fine-tune** per project/rig
3. **Debug** when something seems wrong (which model is running?)

## Minimum Viable Interaction

### Simplest Path to Cost Savings

1. Copy example config to `~/gt/settings/`:
   ```bash
   cp /path/to/examples/cost-optimized-agents.json ~/gt/settings/agents.json
   cp /path/to/examples/cost-optimized-config.json ~/gt/settings/config.json
   ```

2. Restart Gas Town
3. Done - witnesses and refineries now use cheaper models

### One-Command Setup (Future Enhancement)

```bash
gt config apply cost-optimized
# Applies preset configuration for cost optimization
```

## Power Users vs Beginners

### Beginner Experience

- Zero configuration required
- Defaults work out of the box
- Cost optimization is optional

### Power User Experience

- Full control via config files
- Per-rig overrides
- Custom agent presets with any args
- Resolution hierarchy for complex setups

### No In-Between Pain

- Simple use case (defaults) is trivial
- Complex use case (per-rig models) is well-documented
- No awkward middle ground
