# Tmux to TownSessions Abstraction Migration

Direct tmux usage that should be migrated to TownSessions for proper town-aware session naming (multi-town support).

## Overview

TownSessions provides:
- Automatic town ID suffix on Start()
- Optimistic-with-retry lookup on Stop/Exists/Send (finds new or legacy format)
- List filtering to only show sessions from the current town

## 1. Mayor Manager - 7 call sites

| Line | Operation | Notes |
|------|-----------|-------|
| `manager.go:72` | `Exists` | Check if running |
| `manager.go:79` | `Stop` | Kill zombie |
| `manager.go:106` | `Start` | Create session |
| `manager.go:117` | `SetEnv` | Set environment |
| `manager.go:156` | `Exists` | Stop check |
| `manager.go:169` | `Stop` | Kill session |
| `manager.go:178,185,193` | `Exists`, `GetInfo` | Status checks |

## 2. Deacon Manager - 7 call sites

| Line | Operation | Notes |
|------|-----------|-------|
| `manager.go:73` | `Exists` | Check if running |
| `manager.go:80` | `Stop` | Kill zombie |
| `manager.go:106` | `Start` | Create session |
| `manager.go:117` | `SetEnv` | Set environment |
| `manager.go:142` | `Exists` | Stop check |
| `manager.go:155` | `Stop` | Kill session |
| `manager.go:164,171,179` | `Exists`, `GetInfo` | Status checks |

## 3. Daemon - 10 call sites

| Line | Operation | Notes |
|------|-----------|-------|
| `lifecycle.go:174` | `Exists` | Check session |
| `lifecycle.go:182,192` | `Stop` | Kill sessions |
| `lifecycle.go:531,538` | `SetEnv` | Set environment |
| `daemon.go:289` | `Exists` | Check deacon |
| `daemon.go:352` | `Exists` | Check session |
| `daemon.go:368` | `Stop` | Kill session |
| `daemon.go:729` | `Exists` | Check alive |
| `daemon.go:870` | `SetEnv` | Set environment |

## 4. Polecat SessionManager - 12 call sites

| Line | Operation | Notes |
|------|-----------|-------|
| `session_manager.go:179` | `Exists` | Start check |
| `session_manager.go:215` | `Start` | Create session |
| `session_manager.go:231` | `SetEnv` | Set environment |
| `session_manager.go:271` | `Exists` | Stop check |
| `session_manager.go:293` | `Stop` | Kill session |
| `session_manager.go:310` | `Exists` | IsRunning |
| `session_manager.go:318,400,415,429,445` | `Exists` | Status checks |
| `session_manager.go:334` | `GetInfo` | Get session info |
| `session_manager.go:369` | `List` | List sessions |
| `session_manager.go:423,437` | `Capture` | Get output |

## 5. Polecat Manager - 2 call sites

| Line | Operation | Notes |
|------|-----------|-------|
| `manager.go:656` | `Exists` | Check session |
| `manager.go:690` | `Stop` | Kill session |

## 6. Boot - 4 call sites

| Line | Operation | Notes |
|------|-----------|-------|
| `boot.go:86` | `Exists` | Check if running |
| `boot.go:166,193` | `Stop` | Kill session |
| `boot.go:175` | `NewSession` | Create session |
| `boot.go:185` | `SetEnv` | Set environment |

## 7. Connection (Local) - 5 call sites

| Line | Operation | Notes |
|------|-----------|-------|
| `local.go:161` | `NewSession` | Create session |
| `local.go:166` | `Stop` | Kill session |
| `local.go:176` | `Capture` | Get output |
| `local.go:181` | `Exists` | Check exists |
| `local.go:186` | `List` | List sessions |

## 8. Mail Router - 1 call site

| Line | Operation | Notes |
|------|-----------|-------|
| `router.go:875` | `Exists` | Check before nudge |

## 9. Witness/Refinery/Crew Managers - SetEnv calls

| Location | Operation | Notes |
|----------|-----------|-------|
| `witness/manager.go:182,186,191` | `SetEnv` | Set environment |
| `refinery/manager.go:202` | `SetEnv` | Set environment |
| `crew/manager.go:543` | `SetEnv` | Set environment |

## 10. Commands - Various

| Location | Operation | Notes |
|----------|-----------|-------|
| `cmd/deacon.go:369,498` | `SetEnv`, `GetInfo` | Deacon management |
| `cmd/issue.go:58,77` | `SetEnv` | Issue tracking |
| `cmd/crew_at.go:173` | `SetEnv` | Crew environment |
| `cmd/crew_lifecycle.go:595,685` | `Capture` | Get output |
| `cmd/sling_helpers.go:310` | `Capture` | Check session |

## Migration Strategy

1. Managers should receive `*TownSessions` instead of `*tmux.Tmux`
2. Use `NewTownSessions(tmux.NewTmux(), townRoot)` at construction
3. All session operations automatically get town-aware naming
4. Existing sessions found via optimistic-with-retry (backwards compatible)

## Priority

High priority for multi-town support:
1. Mayor/Deacon managers (town-level services)
2. Daemon (manages all sessions)
3. Polecat managers (rig-level)

Lower priority:
- Boot (system service, may not need town awareness)
- Commands (often already have access to town context)
