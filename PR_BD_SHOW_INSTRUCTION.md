# PR: docs(templates): add bd show instruction to help agents view hooked work

**Branch:** `docs-bd-show-instruction`

---

## Summary

Add `bd show <bead-id>` instruction to all role templates so agents know how to view details of their hooked work assignment. Previously, agents would try various commands before finding `bd show`, wasting context.

## Related Issue

None - observed behavior in production agents.

## Changes

- Add `bd show <bead-id>` line under "Step 2: Work hooked?" in polecat.md.tmpl
- Add `bd show <bead-id>` line under "Step 2: Work hooked?" in crew.md.tmpl
- Add `bd show <bead-id>` line under "Step 2: Work hooked?" in mayor.md.tmpl
- Add `bd show <bead-id>` line under "Step 2: Work hooked?" in deacon.md.tmpl
- Add `bd show <bead-id>` line under "Step 2: Work hooked?" in witness.md.tmpl

## Testing

- [ ] Unit tests pass (`go test ./...`)
- [ ] Manual testing performed

## Checklist

- [x] Code follows project style
- [x] Documentation updated (if applicable)
- [x] No breaking changes (or documented in summary)
