## Summary

Fix incorrect `bd list` command syntax in mol-witness-patrol formula. The patrol instructions used non-existent `--wisp` flag and `--labels` instead of `--label`.

## Related Issue

None - discovered during patrol execution.

## Changes

- Remove `--wisp` flag from `bd list` commands (flag doesn't exist)
- Change `--labels=` to `--label` (correct flag name is singular)
- Fix 5 occurrences in both source and generated files

## Testing

- [x] Unit tests pass (`go test ./...`)
- [x] Manual testing performed
  - Verified `bd list --wisp` returns "unknown flag: --wisp"
  - Verified `bd list --label cleanup --status=open` works correctly

## Checklist

- [x] Code follows project style
- [x] Documentation updated (if applicable)
- [x] No breaking changes (or documented in summary)
