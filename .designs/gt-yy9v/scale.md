# Scalability Analysis: Model Selection by Role

## Scale Dimensions

### What Scales in This Feature

| Dimension | Current | 10x | 100x | Impact |
|-----------|---------|-----|------|--------|
| Roles | 6 | 6 | 6 | Fixed (not a scaling concern) |
| Agent presets | ~10 | ~30 | ~100 | Config file size only |
| Rigs | 1-3 | 10-30 | 100+ | Per-rig config files |
| Polecats per rig | 1-5 | 10-50 | 100+ | No impact (config per role, not per polecat) |
| Config reads/sec | ~1 | ~10 | ~100 | File read, cached |

### Key Insight

Model selection is **configuration-time**, not **runtime**. The config is read once at agent spawn and cached. This feature doesn't introduce any runtime scaling concerns.

## Resource Usage

### Memory

| Component | Usage | Notes |
|-----------|-------|-------|
| Agent registry | ~10KB | Loaded once, cached |
| Config files | <1KB each | Read on demand |
| Role mappings | ~100 bytes | In-memory map |

**Total**: Negligible (<100KB even with 100 agent presets)

### CPU

| Operation | Frequency | Cost |
|-----------|-----------|------|
| Config load | On startup | O(1) file read |
| Role resolution | Per spawn | O(1) map lookup |
| Model validation | Per spawn | O(1) string compare |

**Total**: Negligible (microseconds per operation)

### Disk

| File | Size | Growth |
|------|------|--------|
| agents.json | ~2KB | Linear with presets |
| config.json | ~500B | Fixed |
| Per-rig config | ~500B | O(rigs) |

**Total**: O(rigs + presets), both bounded by practical limits

### Network

No network impact. Model selection is local config.

## Bottlenecks

### What Limits Growth

1. **File System**: Config files are small, no concern
2. **Memory**: Registry is bounded, no concern
3. **Human Factors**: Managing 100+ agent presets is unwieldy (but not a technical limit)

### Non-Bottlenecks

- This feature does NOT:
  - Add database queries
  - Add network calls
  - Add per-request overhead
  - Increase session memory usage

## Complexity Analysis

### Time Complexity

| Operation | Complexity | Notes |
|-----------|------------|-------|
| Load config | O(1) | Fixed file size |
| Resolve role | O(1) | Hash map lookup |
| Build command | O(1) | String concatenation |

### Space Complexity

| Structure | Complexity | Notes |
|-----------|------------|-------|
| Agent registry | O(presets) | ~10 presets typical |
| Role mappings | O(roles) | 6 roles, fixed |
| Config cache | O(1) | Single instance |

## Caching Opportunities

### Current Caching

The codebase already caches:
- Agent registry: loaded once, cached in `globalRegistry`
- Loaded paths tracked in `loadedPaths` map to avoid re-reads

```go
// From agents.go
var (
    globalRegistry      *AgentRegistry
    loadedPaths         = make(map[string]bool)
    registryInitialized bool
)
```

### No Additional Caching Needed

- Config files are small enough that caching is overkill
- Spawns are infrequent (seconds to minutes apart)
- Re-reading allows config changes without restart

## Scale Testing Scenarios

### 10x Scale (10 rigs, 50 polecats)

Expected behavior:
- 10 rig config files loaded
- 50 spawns read config → 50 file reads (fast)
- No performance impact

### 100x Scale (100 rigs, 500 polecats)

Expected behavior:
- 100 rig config files in memory if all accessed
- File reads remain O(1) per spawn
- No degradation expected

### 1000x Scale (Not realistic for Gas Town)

If somehow reached:
- File system might become a concern (many small files)
- Could consolidate to single config file if needed
- But this scale is unrealistic for the use case

## Hard Limits

### Technical Limits

| Limit | Value | Practical Impact |
|-------|-------|------------------|
| Max agent presets | ~unlimited | JSON parser limit (~MB) |
| Max roles | 6 (hardcoded) | By design, not configurable |
| Max rigs | ~unlimited | File system limit |
| Max config file size | ~100MB | JSON parser limit |

### Practical Limits

| Limit | Value | Why |
|-------|-------|-----|
| Agent presets | ~20 | Human manageability |
| Rigs per town | ~10 | Resource constraints |
| Models available | ~5 | What Claude offers |

## Optimization Recommendations

### Keep Simple

This feature is inherently scalable due to:
- Static configuration (read once)
- O(1) lookups
- No runtime overhead

### Don't Over-Engineer

**Avoid**:
- Database storage for config (overkill)
- Caching layer (unnecessary)
- Async config loading (premature)
- Config hot-reloading (complexity without benefit)

**Do**:
- Use simple file reads
- Cache in memory (existing pattern)
- Re-read on spawn (allows config changes)

### Future Considerations

If Gas Town ever needs 1000+ rigs:
- Consolidate config into single town-level file
- Use lazy loading for rig configs
- Consider SQLite for config (but unlikely needed)
