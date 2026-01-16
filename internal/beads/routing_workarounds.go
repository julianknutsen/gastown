// Package beads provides routing workarounds for bd commands that don't
// support prefix-based routing. Each workaround is tracked here so it can
// be removed when the underlying bd command is fixed.
//
// APPROACH: We assume all bd commands correctly support prefix-based routing.
// Tests in routing_workarounds_test.go verify this assumption. When a test
// fails against the real bd CLI, we add that command to KnownBrokenRouting
// and provide a documented workaround.
//
// This approach ensures we only add workarounds for commands that are
// actually broken, not based on assumptions.
package beads

// KnownBrokenRouting tracks bd commands that have been VERIFIED to not support
// prefix-based routing. Only add commands here after a test failure proves
// the command doesn't route correctly.
//
// Format: command name -> issue/date when broken behavior was confirmed
//
// When bd is fixed, remove the command from this map and delete its
// Workaround* method.
//
// Tested with bd v0.46.0 on 2026-01-15 using cross-rig scenario with:
// - mayor/town.json present (bd's town root marker)
// - routes.jsonl at townRoot/.beads/
// - Created bead ap-xxx in ai_platform/mayor/rig
// - Ran commands from gastown/mayor/rig
//
// WORKING (bd routes via mayor/town.json -> routes.jsonl):
// - show, update, close, close --reason
// - slot show
//
// BROKEN (bd does NOT route these commands):
// - reopen, delete, label add, comment, dep add
// - label remove, dep rm (discovered broken 2026-01-15 despite earlier testing)
//
// NOTE: bd bond, bd track, bd agent state don't exist as commands.
// The Implementation has methods for them but they call non-existent subcommands.
var KnownBrokenRouting = map[string]string{
	"reopen":       "Confirmed 2026-01-15: bd reopen ap-xxx fails from gastown dir with 'issue not found'",
	"delete":       "Confirmed 2026-01-15: bd delete ap-xxx fails from gastown dir with 'issue not found'",
	"label-add":    "Confirmed 2026-01-15: bd label add ap-xxx fails from cross-rig context",
	"label-remove": "Confirmed 2026-01-15: bd label remove ap-xxx fails from cross-rig context",
	"comment":      "Confirmed 2026-01-15: bd comment ap-xxx fails from gastown dir with 'issue not found'",
	"dep-add":      "Confirmed 2026-01-15: bd dep add ap-xxx fails from cross-rig context",
	"dep-rm":       "Confirmed 2026-01-15: bd dep rm ap-xxx fails from cross-rig context",
}

// NeedsRoutingWorkaround returns true if the given bd command has been
// verified to require manual directory resolution before invocation.
// Returns false for unknown commands (optimistic default - assume routing works).
func NeedsRoutingWorkaround(command string) bool {
	_, broken := KnownBrokenRouting[command]
	return broken
}

// =============================================================================
// Workaround Methods
//
// These methods are available for use IF AND WHEN testing reveals that a
// particular bd command doesn't support prefix-based routing. Do not use
// these preemptively - first verify the command is broken by running the
// cross-rig tests below, then add the command to KnownBrokenRouting.
// =============================================================================

// WorkaroundUpdate runs bd update with proper routing.
// Use only if "update" is in KnownBrokenRouting.
func (b *Implementation) WorkaroundUpdate(townRoot, id string, opts UpdateOptions) error {
	workDir := ResolveHookDir(townRoot, id, b.workDir)
	routed := New(workDir)
	return routed.Update(id, opts)
}

// WorkaroundClose runs bd close with proper routing.
// Use only if "close" is in KnownBrokenRouting.
func (b *Implementation) WorkaroundClose(townRoot string, ids ...string) error {
	if len(ids) == 0 {
		return nil
	}
	workDir := ResolveHookDir(townRoot, ids[0], b.workDir)
	routed := New(workDir)
	return routed.Close(ids...)
}

// WorkaroundShow runs bd show with proper routing.
// Use only if "show" is in KnownBrokenRouting.
func (b *Implementation) WorkaroundShow(townRoot, id string) (*Issue, error) {
	workDir := ResolveHookDir(townRoot, id, b.workDir)
	routed := New(workDir)
	return routed.Show(id)
}

// WorkaroundReopen runs bd reopen with proper routing.
// Use only if "reopen" is in KnownBrokenRouting.
func (b *Implementation) WorkaroundReopen(townRoot, id string) error {
	workDir := ResolveHookDir(townRoot, id, b.workDir)
	routed := New(workDir)
	return routed.Reopen(id)
}

// WorkaroundDelete runs bd delete with proper routing.
// Use only if "delete" is in KnownBrokenRouting.
func (b *Implementation) WorkaroundDelete(townRoot string, ids ...string) error {
	if len(ids) == 0 {
		return nil
	}
	workDir := ResolveHookDir(townRoot, ids[0], b.workDir)
	routed := New(workDir)
	return routed.Delete(ids...)
}

// WorkaroundAddDependency runs bd depends add with proper routing.
// Use only if "depends" is in KnownBrokenRouting.
func (b *Implementation) WorkaroundAddDependency(townRoot, issue, dependsOn string) error {
	workDir := ResolveHookDir(townRoot, issue, b.workDir)
	routed := New(workDir)
	return routed.AddDependency(issue, dependsOn)
}
