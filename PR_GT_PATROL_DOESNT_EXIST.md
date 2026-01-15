## Summary

Remove non-existent `gt patrol start` command from Mayor's CLAUDE.md bootstrap template.

## Related Issue

None - discovered during mayor session startup.

## Changes

- Remove `- Start patrol: \`gt patrol start\`` line from install.go
- The `gt patrol` command does not exist; the Deacon runs patrol on the Mayor's behalf

## Testing

- [x] Unit tests pass (`go test ./...`)
- [x] Manual testing performed
  - Verified `gt patrol` returns "unknown command"

## Checklist

- [x] Code follows project style
- [x] Documentation updated (if applicable)
- [x] No breaking changes (or documented in summary)
