# Tmux to Agent Abstraction Migration

Direct tmux usage that should be migrated to the Agent abstraction layer for proper readiness tracking.

## 1. NudgeSession (Message Delivery) - 13 call sites

These are the highest priority - they would benefit from Agent's readiness tracking to fix race conditions like issue #470.

| Location | Purpose |
|----------|---------|
| `mayor/manager.go:145` | Propulsion nudge after startup |
| `mail/router.go:882` | Mail notifications to agents |
| `daemon/lifecycle.go:408` | Propulsion nudge after restart |
| `daemon/daemon.go:375` | Health check nudge |
| `cmd/deacon.go:409,705` | Propulsion + health check |
| `cmd/sling_helpers.go:452-453` | Notify witness/refinery of dispatch |
| `cmd/boot.go:312` | Health check to deacon |
| `cmd/nudge.go` (4 sites) | User-facing `gt nudge` command |
| `cmd/broadcast.go:104` | Broadcast to all agents |
| `polecat/pending.go:154` | Trigger pending polecat |
| `runtime/runtime.go:85` | Startup fallback command |

## 2. SendControl (Ctrl-C / UI Navigation) - 10 call sites

Lower priority - shutdown doesn't need readiness checks (if session exists, can send Ctrl-C).

| Location | Purpose |
|----------|---------|
| `mayor/manager.go:165` | Graceful shutdown |
| `deacon/manager.go:151` | Graceful shutdown |
| `session/town.go:43` | Town session shutdown |
| `witness/handlers.go:663` | Interrupt |
| `cmd/deacon.go:432` | Graceful shutdown |
| `cmd/down.go:387` | Graceful shutdown |
| `cmd/start.go:473` | Escape interrupt |
| `polecat/session_manager.go:289` | Graceful shutdown |
| `agent/hooks.go:77,85` | Dialog navigation (Down/Enter) |

## 3. Send/SendKeys (Commands) - 5 call sites

| Location | Purpose |
|----------|---------|
| `daemon/lifecycle.go:384` | Send start command |
| `daemon/daemon.go:884` | Send start command |
| `boot/boot.go:196` | Send start command |
| `connection/local.go:171` | Local connection passthrough |
| `polecat/session_manager.go:458` | Inject message |

## Migration Strategy

1. Add `ready` field to Agent struct
2. Have `Agent.Send()` wait for readiness on first call
3. Create Agent instances for all managed sessions
4. Route nudges through Agent instead of raw tmux
5. This fixes race conditions where messages arrive before Claude Code is ready
