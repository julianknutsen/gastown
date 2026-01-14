# Security Analysis: Model Selection by Role

## Trust Boundaries

### Trust Model

```
┌─────────────────────────────────────────────────┐
│ User's Machine (fully trusted)                  │
│                                                 │
│  ┌──────────────┐    ┌───────────────────────┐ │
│  │ Config Files │───>│ Gas Town CLI          │ │
│  │ (user-owned) │    │ (reads config)        │ │
│  └──────────────┘    └───────────┬───────────┘ │
│                                  │             │
│                      ┌───────────▼───────────┐ │
│                      │ Claude Code CLI       │ │
│                      │ (receives --model)    │ │
│                      └───────────┬───────────┘ │
└──────────────────────────────────┼─────────────┘
                                   │
                       ┌───────────▼───────────┐
                       │ Anthropic API         │
                       │ (validates model)     │
                       │ (external, untrusted) │
                       └───────────────────────┘
```

### Trust Levels

| Component | Trust Level | Notes |
|-----------|-------------|-------|
| Config files | Full | User-owned, on local machine |
| Gas Town CLI | Full | User-installed |
| Claude Code | Full | User-installed |
| Anthropic API | Partial | Validates model names, rate limits |

## Attack Surface

### New Inputs

| Input | Source | Validation |
|-------|--------|------------|
| Model name | Config file | Validated against known models |
| Agent name | Config file | Validated against registry |
| Role name | Config file | Validated against fixed list |

### New Attack Vectors

**Config File Injection**: None introduced
- Config files are already trusted input
- No new file types or formats
- Same JSON parsing as existing config

**Command Injection via Model Name**: Low risk
- Model name is passed to `--model` flag
- Claude Code validates model name
- Invalid names are rejected by Claude

**Privilege Escalation**: None
- Model selection doesn't grant new permissions
- Same execution context as before

## Threat Model

### Who Might Attack

| Attacker | Motivation | Capability |
|----------|------------|------------|
| Malicious user | Abuse API | Has local access (not our threat model) |
| Malicious config | Inject commands | Config is user-owned (trusted) |
| MITM | Intercept API calls | HTTPS prevents |
| Anthropic | Bill more | Trust boundary issue, not Gas Town's |

### Attack Scenarios

**Scenario 1: Malicious model name**
```json
{"role_agents": {"polecat": "claude; rm -rf /"}}
```
- **Risk**: Command injection
- **Mitigation**: Agent name validated against registry; invalid names rejected
- **Impact**: None - agent won't spawn

**Scenario 2: Cost inflation attack**
```json
{"role_agents": {"witness": "claude-opus"}}
```
- **Risk**: Higher costs
- **Mitigation**: This is expected behavior (user choice)
- **Impact**: User pays more (user's decision)

**Scenario 3: Config file tampering**
- **Risk**: Attacker modifies config to use expensive model
- **Mitigation**: File permissions (user-owned, mode 644)
- **Impact**: Same as any local config tampering

## Sensitive Data

### What's Exposed

| Data | Where | Risk |
|------|-------|------|
| Model name | Config file, CLI args, logs | Low - not secret |
| API costs | Anthropic billing | Low - user's own data |
| Role assignments | Config file | Low - operational data |

### What's NOT Exposed

- API keys (handled by Claude Code)
- Conversation content (unchanged)
- User credentials (unchanged)

### Logging Considerations

Model names may appear in:
- Gas Town logs (acceptable)
- Claude Code logs (acceptable)
- `ps` output (acceptable)
- Shell history (acceptable)

None of these expose sensitive data.

## Authentication/Authorization

### No Changes to Auth

- Claude Code handles API authentication
- Gas Town doesn't touch API keys
- Model selection doesn't require new permissions

### Authorization Model

| Action | Authorization |
|--------|---------------|
| Read config | File system permissions |
| Modify config | File system permissions |
| Use expensive model | User's API quota |
| Use any model | Anthropic account permissions |

## Input Validation

### Model Name Validation

```go
// Validate model name
func ValidateModel(m string) error {
    valid := []string{"opus-4.5", "opus", "sonnet-4", "sonnet", "haiku-3.5", "haiku"}
    for _, v := range valid {
        if m == v {
            return nil
        }
    }
    return fmt.Errorf("unknown model: %s", m)
}
```

### Agent Name Validation

```go
// Validate agent name (existing)
func IsKnownPreset(name string) bool {
    _, ok := globalRegistry.Agents[name]
    return ok
}
```

### Role Name Validation

```go
// Validate role name
var validRoles = []string{"polecat", "witness", "refinery", "mayor", "deacon", "crew"}

func ValidateRole(r string) error {
    for _, v := range validRoles {
        if r == v {
            return nil
        }
    }
    return fmt.Errorf("unknown role: %s", r)
}
```

## Worst Case Analysis

### If This Is Exploited

| Attack | Worst Case | Likelihood |
|--------|------------|------------|
| Invalid model | Agent fails to start | High (user typos) |
| Expensive model | Higher API costs | Medium (misconfiguration) |
| Config tampering | Attacker controls model selection | Low (requires local access) |

### Mitigation Summary

1. **Validate inputs**: Check model/agent/role names before use
2. **Fail safely**: Invalid config → fall back to defaults
3. **Log clearly**: Make it obvious what model is being used
4. **Trust boundaries**: Config files are user-owned (trusted)

## Recommendations

### Must Have

1. Validate model names against known list
2. Validate agent names against registry
3. Log model selection at spawn time

### Should Have

1. Warning for expensive model selections
2. Documentation of cost implications
3. Example configs with cost notes

### Nice to Have

1. Cost estimation before spawn
2. Daily cost alerts
3. Per-rig cost limits (future feature)
