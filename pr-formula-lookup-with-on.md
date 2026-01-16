# PR: fix(sling): fix formula lookup in --on mode

**Title:** fix(sling): fix formula lookup in --on mode

---

## Summary

Fixes formula lookup failure when using `gt sling <formula> --on <bead>`. The formula instantiation was failing because `bd cook` and `bd mol wisp` couldn't find formulas in the expected locations.

## Related Issue

Fixes #422

## Changes

- Remove `cookCmd.Dir` assignment so `bd cook` runs from cwd (like `gt formula show`)
- Add `GT_ROOT` environment variable to `wispCmd` so `bd mol wisp` can find formulas in the orchestrator path

### Root Cause

When running `gt sling <formula> --on <bead>`:

1. `bd cook` was running from the rig directory where formulas might not exist locally
2. `bd mol wisp` couldn't find formulas because it didn't have access to `$GT_ROOT/.beads/formulas/`

### Fix Details

| Command | Before | After |
|---------|--------|-------|
| `bd cook` | Ran from `formulaWorkDir` (rig dir) | Runs from cwd (formulas exist) |
| `bd mol wisp` | No GT_ROOT in env | GT_ROOT passed for formula lookup |

## Testing

- [x] Unit tests pass (`go test ./...`)
- [x] Manual testing performed
  - Verified `gt sling <formula> --on <bead>` works from orchestrator context
  - Confirmed formula instantiation creates wisp correctly

## Checklist

- [x] Code follows project style
- [x] Documentation updated (if applicable) - N/A, internal fix
- [x] No breaking changes (or documented in summary)
