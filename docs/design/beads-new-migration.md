# beads.New() Migration to ForTown/ForRig

This document audits all `beads.New()` usages and recommends migration to explicit `ForTown()` or `ForRig()` constructors.

## Background

`beads.New(path)` automatically detects townRoot but doesn't make intent clear at the call site. The new constructors:
- `beads.ForTown(townRoot)` - targets town-level beads (hq-* prefix)
- `beads.ForRig(rigPath)` - targets rig-level beads (gt-*, ap-* prefix)

Both enable cross-rig routing for ID-based operations. The difference is which database non-ID operations (List, Create, Ready) target.

---

## Migration Summary

| Category | Count | Action |
|----------|-------|--------|
| ForTown | 54 | Migrate to `beads.ForTown()` |
| ForRig | 77 | Migrate to `beads.ForRig()` |
| Test files | 20 | Optional - keep for flexibility |
| **Total** | 151 | |

---

## ForTown Migrations (54 calls)

These use `townRoot`, `townBeads`, `townBeadsPath`, or `townPath` - clearly targeting town-level beads.

### internal/doctor/

| File:Line | Current | Migration |
|-----------|---------|-----------|
| agent_beads_check.go:78 | `beads.New(townBeadsPath)` | `beads.ForTown(townBeadsPath)` |
| agent_beads_check.go:165 | `beads.New(townBeadsPath)` | `beads.ForTown(townBeadsPath)` |
| role_beads_check.go:41 | `beads.New(townBeadsPath)` | `beads.ForTown(townBeadsPath)` |

### internal/web/

| File:Line | Current | Migration |
|-----------|---------|-----------|
| fetcher.go:39 | `beads.New(filepath.Dir(f.townBeads))` | `beads.ForTown(filepath.Dir(f.townBeads))` |
| fetcher.go:239 | `beads.New(filepath.Dir(f.townBeads))` | `beads.ForTown(filepath.Dir(f.townBeads))` |

### internal/cmd/

| File:Line | Current | Migration |
|-----------|---------|-----------|
| deacon.go:938 | `beads.New(townRoot)` | `beads.ForTown(townRoot)` |
| deacon.go:967 | `beads.New(townRoot)` | `beads.ForTown(townRoot)` |
| sling_helpers.go:33 | `beads.New(townRoot)` | `beads.ForTown(townRoot)` |
| sling_helpers.go:48 | `beads.New(townRoot)` | `beads.ForTown(townRoot)` |
| prime_output.go:238 | `beads.New(ctx.TownRoot)` | `beads.ForTown(ctx.TownRoot)` |
| status.go:239 | `beads.New(townBeadsPath)` | `beads.ForTown(townBeadsPath)` |
| formula.go:412 | `beads.New(townBeads)` | `beads.ForTown(townBeads)` |
| ready.go:123 | `beads.New(townBeadsPath)` | `beads.ForTown(townBeadsPath)` |
| notify.go:71 | `beads.New(townRoot)` | `beads.ForTown(townRoot)` |
| convoy.go:316 | `beads.New(townBeads)` | `beads.ForTown(townBeads)` |
| convoy.go:371 | `beads.New(townBeads)` | `beads.ForTown(townBeads)` |
| convoy.go:447 | `beads.New(townBeads)` | `beads.ForTown(townBeads)` |
| convoy.go:568 | `beads.New(townBeads)` | `beads.ForTown(townBeads)` |
| convoy.go:682 | `beads.New(townBeads)` | `beads.ForTown(townBeads)` |
| convoy.go:727 | `beads.New(townBeads)` | `beads.ForTown(townBeads)` |
| convoy.go:780 | `beads.New(townBeads)` | `beads.ForTown(townBeads)` |
| convoy.go:875 | `beads.New(townBeads)` | `beads.ForTown(townBeads)` |
| convoy.go:912 | `beads.New(townBeads)` | `beads.ForTown(townBeads)` |
| convoy.go:1360 | `beads.New(townBeads)` | `beads.ForTown(townBeads)` |
| dnd.go:68 | `beads.New(townRoot)` | `beads.ForTown(townRoot)` |
| install.go:361 | `beads.New(townPath)` | `beads.ForTown(townPath)` |
| install.go:464 | `beads.New(townPath)` | `beads.ForTown(townPath)` |
| nudge.go:439 | `beads.New(townRoot)` | `beads.ForTown(townRoot)` |
| mail_channel.go:118 | `beads.New(townRoot)` | `beads.ForTown(townRoot)` |
| mail_channel.go:162 | `beads.New(townRoot)` | `beads.ForTown(townRoot)` |
| mail_channel.go:245 | `beads.New(townRoot)` | `beads.ForTown(townRoot)` |
| mail_channel.go:287 | `beads.New(townRoot)` | `beads.ForTown(townRoot)` |
| escalate_impl.go:70 | `beads.New(beads.ResolveBeadsDir(townRoot))` | `beads.ForTown(beads.ResolveBeadsDir(townRoot))` |
| escalate_impl.go:161 | `beads.New(beads.ResolveBeadsDir(townRoot))` | `beads.ForTown(beads.ResolveBeadsDir(townRoot))` |
| escalate_impl.go:227 | `beads.New(beads.ResolveBeadsDir(townRoot))` | `beads.ForTown(beads.ResolveBeadsDir(townRoot))` |
| escalate_impl.go:256 | `beads.New(beads.ResolveBeadsDir(townRoot))` | `beads.ForTown(beads.ResolveBeadsDir(townRoot))` |
| escalate_impl.go:288 | `beads.New(beads.ResolveBeadsDir(townRoot))` | `beads.ForTown(beads.ResolveBeadsDir(townRoot))` |
| escalate_impl.go:469 | `beads.New(beads.ResolveBeadsDir(townRoot))` | `beads.ForTown(beads.ResolveBeadsDir(townRoot))` |
| mail_send.go:115 | `beads.New(townRoot)` | `beads.ForTown(townRoot)` |
| mail_group.go:123 | `beads.New(townRoot)` | `beads.ForTown(townRoot)` |
| mail_group.go:161 | `beads.New(townRoot)` | `beads.ForTown(townRoot)` |
| mail_group.go:222 | `beads.New(townRoot)` | `beads.ForTown(townRoot)` |
| mail_group.go:255 | `beads.New(townRoot)` | `beads.ForTown(townRoot)` |
| mail_group.go:273 | `beads.New(townRoot)` | `beads.ForTown(townRoot)` |
| mail_group.go:290 | `beads.New(townRoot)` | `beads.ForTown(townRoot)` |
| rig.go:558 | `beads.New(townRoot)` | `beads.ForTown(townRoot)` |
| dog.go:255 | `beads.New(townRoot)` | `beads.ForTown(townRoot)` |
| dog.go:297 | `beads.New(townRoot)` | `beads.ForTown(townRoot)` |
| dog.go:709 | `beads.New(townRoot)` | `beads.ForTown(townRoot)` |
| sling_convoy.go:76 | `beads.New(townBeads)` | `beads.ForTown(townBeads)` |

---

## ForRig Migrations (77 calls)

These use `rigPath`, `r.Path`, `r.BeadsPath()`, `rigBeadsPath`, `rigDir`, or context indicates rig-level operations.

### internal/doctor/

| File:Line | Current | Migration |
|-----------|---------|-----------|
| rig_beads_check.go:83 | `beads.New(rigBeadsPath)` | `beads.ForRig(rigBeadsPath)` |
| rig_beads_check.go:147 | `beads.New(rigBeadsPath)` | `beads.ForRig(rigBeadsPath)` |
| patrol_check.go:121 | `beads.New(rigPath)` | `beads.ForRig(rigPath)` |
| agent_beads_check.go:115 | `beads.New(rigBeadsPath)` | `beads.ForRig(rigBeadsPath)` |
| agent_beads_check.go:224 | `beads.New(rigBeadsPath)` | `beads.ForRig(rigBeadsPath)` |
| repo_fingerprint_check.go:71 | `beads.New(workDir)` | `beads.ForRig(workDir)` - fingerprint is per-repo |
| repo_fingerprint_check.go:144 | `beads.New(workDir)` | `beads.ForRig(workDir)` |
| hook_check.go:87 | `beads.New(filepath.Dir(beadsDir))` | `beads.ForRig(filepath.Dir(beadsDir))` |
| hook_check.go:176 | `beads.New(filepath.Dir(inv.pinnedBeadDir))` | `beads.ForRig(filepath.Dir(inv.pinnedBeadDir))` |
| hook_check.go:268 | `beads.New(filepath.Dir(beadsDir))` | `beads.ForRig(filepath.Dir(beadsDir))` |
| hook_check.go:312 | `beads.New(filepath.Dir(dup.beadsDir))` | `beads.ForRig(filepath.Dir(dup.beadsDir))` |
| hook_check.go:402 | `beads.New(filepath.Dir(beadsDir))` | `beads.ForRig(filepath.Dir(beadsDir))` |
| beads_check.go:151 | `beads.New(ctx.RigPath())` | `beads.ForRig(ctx.RigPath())` |

### internal/crew/

| File:Line | Current | Migration |
|-----------|---------|-----------|
| manager.go:413 | `beads.New(dir)` | `beads.ForRig(dir)` - crew operates in rig |

### internal/polecat/

| File:Line | Current | Migration |
|-----------|---------|-----------|
| session_manager.go:282 | `beads.New(workDir)` | `beads.ForRig(workDir)` - polecat workDir is in rig |
| session_manager.go:455 | `beads.New(workDir)` | `beads.ForRig(workDir)` |

### internal/cmd/ - Rig operations

| File:Line | Current | Intent | Migration |
|-----------|---------|--------|-----------|
| agent_state.go:209 | `beads.New(filepath.Dir(beadsDir))` | Agent state in rig | `beads.ForRig(...)` |
| agent_state.go:249 | `beads.New(filepath.Dir(beadsDir))` | Agent state in rig | `beads.ForRig(...)` |
| sling_helpers.go:309 | `beads.New(bdWorkDir)` | Sling targets rig | `beads.ForRig(bdWorkDir)` |
| sling_helpers.go:368 | `beads.New(rigDir)` | Explicit rigDir | `beads.ForRig(rigDir)` |
| molecule_lifecycle.go:61 | `beads.New(workDir)` | Molecules in rig | `beads.ForRig(workDir)` |
| molecule_lifecycle.go:165 | `beads.New(workDir)` | Molecules in rig | `beads.ForRig(workDir)` |
| mq_next.go:59 | `beads.New(r.BeadsPath())` | MQ in rig | `beads.ForRig(r.BeadsPath())` |
| prime_output.go:342 | `beads.New(ctx.WorkDir)` | Prime in rig | `beads.ForRig(ctx.WorkDir)` |
| molecule_status.go:151 | `beads.New(workDir)` | Molecules in rig | `beads.ForRig(workDir)` |
| molecule_status.go:318 | `beads.New(workDir)` | Molecules in rig | `beads.ForRig(workDir)` |
| molecule_status.go:719 | `beads.New(workDir)` | Molecules in rig | `beads.ForRig(workDir)` |
| molecule_status.go:923 | `beads.New(rigBeadsDir)` | Explicit rig | `beads.ForRig(rigBeadsDir)` |
| status.go:269 | `beads.New(rigBeadsPath)` | Rig status | `beads.ForRig(rigBeadsPath)` |
| status.go:1141 | `beads.New(r.BeadsPath())` | Rig beads | `beads.ForRig(r.BeadsPath())` |
| molecule_await_signal.go:298 | `beads.New(workDir)` | Molecules in rig | `beads.ForRig(workDir)` |
| molecule_await_signal.go:347 | `beads.New(filepath.Dir(beadsDir))` | Molecules in rig | `beads.ForRig(...)` |
| sling.go:381 | `beads.New(formulaWorkDir)` | Formula in rig | `beads.ForRig(formulaWorkDir)` |
| sling.go:424 | `beads.New(hookDir)` | Hook dir in rig | `beads.ForRig(hookDir)` |
| swarm.go:238 | `beads.New(beadsPath)` | Swarm in rig | `beads.ForRig(beadsPath)` |
| swarm.go:312 | `beads.New(r.BeadsPath())` | Swarm in rig | `beads.ForRig(r.BeadsPath())` |
| swarm.go:374 | `beads.New(r.BeadsPath())` | Swarm in rig | `beads.ForRig(r.BeadsPath())` |
| swarm.go:509 | `beads.New(r.BeadsPath())` | Swarm in rig | `beads.ForRig(r.BeadsPath())` |
| swarm.go:614 | `beads.New(r.BeadsPath())` | Swarm in rig | `beads.ForRig(r.BeadsPath())` |
| swarm.go:675 | `beads.New(r.BeadsPath())` | Swarm in rig | `beads.ForRig(r.BeadsPath())` |
| swarm.go:750 | `beads.New(r.BeadsPath())` | Swarm in rig | `beads.ForRig(r.BeadsPath())` |
| polecat_helpers.go:106 | `beads.New(target.r.Path)` | Polecat rig | `beads.ForRig(target.r.Path)` |
| polecat_helpers.go:204 | `beads.New(target.r.Path)` | Polecat rig | `beads.ForRig(target.r.Path)` |
| ready.go:145 | `beads.New(rigBeadsPath)` | Ready in rig | `beads.ForRig(rigBeadsPath)` |
| mail_queue.go:141 | `beads.New(filepath.Dir(beadsDir))` | Mail queue in rig context | `beads.ForRig(...)` |
| mail_queue.go:204 | `beads.New(filepath.Dir(beadsDir))` | Mail queue in rig context | `beads.ForRig(...)` |
| mail_queue.go:276 | `beads.New(filepath.Dir(beadsDir))` | Mail queue in rig context | `beads.ForRig(...)` |
| mail_queue.go:314 | `beads.New(filepath.Dir(beadsDir))` | Mail queue in rig context | `beads.ForRig(...)` |
| rig_dock.go:83 | `beads.New(r.BeadsPath())` | Rig dock | `beads.ForRig(r.BeadsPath())` |
| rig_dock.go:184 | `beads.New(r.BeadsPath())` | Rig dock | `beads.ForRig(r.BeadsPath())` |
| rig_dock.go:241 | `beads.New(beadsPath)` | Rig dock | `beads.ForRig(beadsPath)` |
| migrate_agents.go:106 | `beads.New(sourceBeadsDir)` | Migration source | `beads.ForRig(sourceBeadsDir)` |
| checkpoint_cmd.go:223 | `beads.New(workDir)` | Checkpoint in rig | `beads.ForRig(workDir)` |
| checkpoint_cmd.go:266 | `beads.New(workDir)` | Checkpoint in rig | `beads.ForRig(workDir)` |
| polecat.go:580 | `beads.New(p.ClonePath)` | Polecat clone | `beads.ForRig(p.ClonePath)` |
| polecat.go:1219 | `beads.New(filepath.Join(p.r.Path, "mayor", "rig"))` | Mayor rig | `beads.ForRig(...)` |
| install.go:423 | `beads.New(beadsPath)` | Install rig beads | `beads.ForRig(beadsPath)` |
| install.go:438 | `beads.New(beadsPath)` | Install rig beads | `beads.ForRig(beadsPath)` |
| install.go:558 | `beads.New(workDir)` | Install in rig | `beads.ForRig(workDir)` |
| polecat_identity.go:724 | `beads.New(beadsQueryPath)` | Identity in rig | `beads.ForRig(beadsQueryPath)` |
| statusline.go:733 | `beads.New(beadsDir)` | Statusline rig | `beads.ForRig(beadsDir)` |
| statusline.go:770 | `beads.New(workDir)` | Statusline rig | `beads.ForRig(workDir)` |
| mq_list.go:26 | `beads.New(r.BeadsPath())` | MQ list rig | `beads.ForRig(r.BeadsPath())` |
| molecule_step.go:88 | `beads.New(workDir)` | Molecule in rig | `beads.ForRig(workDir)` |
| molecule_step.go:291 | `beads.New(gitRoot)` | Molecule in rig | `beads.ForRig(gitRoot)` |
| molecule_step.go:370 | `beads.New(workDir)` | Molecule in rig | `beads.ForRig(workDir)` |
| molecule_step.go:381 | `beads.New(gitRoot)` | Molecule in rig | `beads.ForRig(gitRoot)` |
| unsling.go:105 | `beads.New(beadsPath)` | Unsling in rig | `beads.ForRig(beadsPath)` |
| patrol_helpers.go:29 | `beads.New(cfg.BeadsDir)` | Patrol in rig | `beads.ForRig(cfg.BeadsDir)` |
| patrol_helpers.go:85 | `beads.New(cfg.BeadsDir)` | Patrol in rig | `beads.ForRig(cfg.BeadsDir)` |
| audit.go:263 | `beads.New(gastownBeadsPath)` | Audit gastown | `beads.ForRig(gastownBeadsPath)` |
| up.go:670 | `beads.New(polecatPath)` | Polecat path | `beads.ForRig(polecatPath)` |
| hook.go:145 | `beads.New(workDir)` | Hook in rig | `beads.ForRig(workDir)` |
| hook.go:286 | `beads.New(workDir)` | Hook in rig | `beads.ForRig(workDir)` |
| crew_add.go:59 | `beads.New(beads.ResolveBeadsDir(r.Path))` | Crew in rig | `beads.ForRig(...)` |
| rig.go:392 | `beads.New(beadsWorkDir)` | Rig beads | `beads.ForRig(beadsWorkDir)` |
| rig.go:560 | `beads.New(cwd)` | Rig context | `beads.ForRig(cwd)` |
| sling_batch.go:95 | `beads.New(hookDir)` | Sling batch | `beads.ForRig(hookDir)` |
| resume.go:97 | `beads.New(cloneRoot)` | Resume in clone | `beads.ForRig(cloneRoot)` |
| sling_formula.go:212 | `beads.New(hookDir)` | Formula hook | `beads.ForRig(hookDir)` |
| molecule_attach_from_mail.go:80 | `beads.New(workDir)` | Molecule in rig | `beads.ForRig(workDir)` |
| molecule_attach.go:39 | `beads.New(workDir)` | Molecule in rig | `beads.ForRig(workDir)` |
| molecule_attach.go:106 | `beads.New(workDir)` | Molecule in rig | `beads.ForRig(workDir)` |
| molecule_attach.go:143 | `beads.New(workDir)` | Molecule in rig | `beads.ForRig(workDir)` |
| mq_status.go:62 | `beads.New(workDir)` | MQ status rig | `beads.ForRig(workDir)` |
| init.go:161 | `beads.New(workDir)` | Init rig | `beads.ForRig(workDir)` |
| prime_molecule.go:16 | `beads.New(workDir)` | Prime molecule | `beads.ForRig(workDir)` |
| prime_molecule.go:95 | `beads.New(ctx.WorkDir)` | Prime molecule | `beads.ForRig(ctx.WorkDir)` |
| prime.go:345 | `beads.New(workDir)` | Prime in rig | `beads.ForRig(workDir)` |
| prime.go:395 | `beads.New(ctx.WorkDir)` | Prime in rig | `beads.ForRig(ctx.WorkDir)` |
| prime.go:666 | `beads.New(ctx.WorkDir)` | Prime in rig | `beads.ForRig(ctx.WorkDir)` |
| prime_state.go:51 | `beads.New(ctx.WorkDir)` | Prime state | `beads.ForRig(ctx.WorkDir)` |

---

## Ambiguous - cwd-based (needs context analysis)

These use `cwd` which could be either town or rig depending on where the command runs.

| File:Line | Current | Analysis Required |
|-----------|---------|-------------------|
| sling_helpers.go:64 | `beads.New(cwd)` | Depends on sling context - likely ForRig |
| sling_helpers.go:100 | `beads.New(cwd)` | Depends on sling context - likely ForRig |
| release.go:47 | `beads.New(cwd)` | Release runs in rig - ForRig |
| callbacks.go:343 | `beads.New(cwd)` | Callback context - likely ForRig |
| handoff.go:659 | `beads.New(cwd)` | Handoff check - needs analysis |
| handoff.go:719 | `beads.New(cwd)` | Handoff check - needs analysis |
| gate.go:92 | `beads.New(cwd)` | Gate runs in rig - ForRig |
| formula.go:175 | `beads.New(cwd)` | Formula in rig - ForRig |
| formula.go:249 | `beads.New(cwd)` | Formula in rig - ForRig |
| convoy.go:626 | `beads.New(cwd)` | Convoy can run anywhere - needs analysis |
| convoy.go:1138 | `beads.New(cwd)` | Convoy attach - needs analysis |
| convoy.go:1174 | `beads.New(cwd)` | Convoy attach - needs analysis |
| mq_submit.go:109 | `beads.New(cwd)` | MQ submit in rig - ForRig |
| park.go:93 | `beads.New(cwd)` | Park in rig - ForRig |
| park.go:114 | `beads.New(workDir)` | Park workDir - ForRig |
| done.go:282 | `beads.New(beads.ResolveBeadsDir(cwd))` | Done runs in rig - ForRig |
| done.go:379 | `beads.New(beads.ResolveBeadsDir(cwd))` | Done runs in rig - ForRig |
| done.go:551 | `beads.New(beadsPath)` | Done beads - ForRig |
| done.go:627 | `beads.New(beads.ResolveBeadsDir(cwd))` | Done runs in rig - ForRig |
| costs.go:414 | `beads.New(location)` | Costs location - needs analysis |
| costs.go:487 | `beads.New(cwd)` | Costs cwd - likely ForRig |
| costs.go:767 | `beads.New(cwd)` | Costs cwd - likely ForRig |
| costs.go:1022 | `beads.New(cwd)` | Costs cwd - likely ForRig |
| costs.go:1142 | `beads.New(cwd)` | Costs cwd - likely ForRig |
| costs.go:1172 | `beads.New(cwd)` | Costs cwd - likely ForRig |
| costs.go:1248 | `beads.New(cwd)` | Costs cwd - likely ForRig |
| sling_formula.go:54 | `beads.New(cwd)` | Formula in rig - ForRig |
| sling_formula.go:179 | `beads.New(cwd)` | Formula in rig - ForRig |

---

## Test Files (20 calls - optional migration)

Test files can keep `beads.New()` for flexibility, or migrate for consistency.

| File | Count | Recommendation |
|------|-------|----------------|
| conformance_matrix_dependency_test.go | 3 | Keep - testing |
| conformance_routing_test.go | 3 | Keep - testing routing |
| routing_workarounds_test.go | 1 | Keep - commented out |
| conformance_matrix_common_test.go | 1 | Keep - testing |
| beads_routing_integration_test.go | 3 | Keep - testing |
| prime_test.go | 1 | Keep - testing |
| done_test.go | 6 | Keep - testing |
| hook_slot_integration_test.go | 7 | Optional - could migrate to ForRig |

---

## Migration Order

Recommended migration order by risk level:

### Phase 1: Clear ForTown (low risk)
All calls explicitly using `townRoot`, `townBeads`, `townBeadsPath` parameters.

### Phase 2: Clear ForRig (low risk)
All calls explicitly using `rigPath`, `r.Path`, `r.BeadsPath()` parameters.

### Phase 3: Context-dependent (needs review)
Calls using `cwd` or `workDir` that require understanding the execution context.

### Phase 4: Tests (optional)
Update test files for consistency if desired.

---

## Deprecation Plan

After migration is complete:
1. Add deprecation comment to `beads.New()`
2. Update linter/CI to warn on `beads.New()` usage
3. Eventually remove `beads.New()` in favor of explicit constructors
