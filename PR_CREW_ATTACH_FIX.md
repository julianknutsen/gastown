## Summary

Fix `gt crew at <name>` restarting Claude when run from inside an existing crew session where the agent is already running.

## Related Issue

<!-- No tracked issue yet -->

## Changes

- Add `IsAgentRunning` check in `crew_at.go` when already inside target tmux session
- Skip agent startup if agent is already running, print informational message instead
- Matches existing behavior for external attach case (lines 228-260)

## Testing

- [x] Unit tests pass (`go test ./...`)
- [x] Manual testing performed
  - Verified `gt crew at pm` no longer restarts Claude when already in the session
  - Verified agent still starts correctly when at shell prompt (agent exited)
  - Verified attach from outside tmux still works correctly

## Checklist

- [x] Code follows project style
- [x] Documentation updated (if applicable)
- [x] No breaking changes (or documented in summary)

---

**PR Title:** `fix(crew): prevent restart when attaching to session with running agent`
