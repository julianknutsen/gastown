package beads

// BdRoutingBugs tracks which BeadsOps operations have prefix-based routing
// bugs in the real bd CLI. When an operation is marked false (has bug),
// Implementation adds a workaround to route correctly.
//
// Test matrix:
//   - Double: always passes (defines correct behavior)
//   - Implementation: always passes (adds workarounds for bugs)
//   - RawBd: fails for operations with bugs, passes when fixed
//
// When a RawBd test unexpectedly passes, bd has been fixed for that operation.
// Update this map to true and remove the workaround from Implementation.
var BdRoutingBugs = map[string]bool{
	// === ID-based operations requiring cross-rig routing ===
	// These operations take a bead ID and need to route to the correct
	// database based on the ID's prefix.

	"Show":        true,  // bd routes Show by prefix (verified with TrueRawBdOps)
	"Update":      true,  // bd routes Update by prefix (verified with TrueRawBdOps)
	"Close":       true,  // bd routes Close by prefix (verified with TrueRawBdOps)
	"Delete":      false, // bd doesn't route Delete by prefix (verified with TrueRawBdOps)
	"Reopen":      false, // bd doesn't route Reopen by prefix (verified with TrueRawBdOps)
	"LabelAdd":    false, // bd doesn't route LabelAdd by prefix (verified with TrueRawBdOps)
	"LabelRemove": false, // bd doesn't route LabelRemove by prefix
	"AgentState":  false, // bd doesn't route AgentState by prefix (not tested yet)
	"Comment":     false, // bd doesn't route Comment by prefix

	// === Operations that don't need routing ===
	// These either operate on the current database or bd handles them correctly.

	"List":             true, // Lists from current db only
	"Create":           true, // Creates in current db
	"CreateWithID":     true, // Creates in current db with specific ID
	"ShowMultiple":     true, // Each ID routed individually (if implemented)
	"CloseWithReason":  true,
	"Ready":            true, // Lists from current db
	"ReadyWithLabel":   true, // Lists from current db
	"Blocked":          true, // Lists from current db
	"AddDependency":    true, // Both IDs should be in same db
	"RemoveDependency": true, // Both IDs should be in same db
	"Sync":             true,
	"SyncFromMain":     true,
	"SyncImportOnly":   true,
	"GetSyncStatus":    true,
	"ConfigGet":        true,
	"ConfigSet":        true,
	"Init":             true,
	"Migrate":          true,
	"DaemonStart":      true,
	"DaemonStop":       true,
	"DaemonStatus":     true,
	"DaemonHealth":     true,
	"MolSeed":          true,
	"MolCurrent":       true,
	"MolCatalog":       true,
	"WispCreate":       true,
	"WispList":         true,
	"WispGC":           true,
	"GateShow":         true,
	"GateWait":         true,
	"GateList":         true,
	"GateResolve":      true,
	"GateAddWaiter":    true,
	"GateCheck":        true,
	"SwarmStatus":      true,
	"SwarmCreate":      true,
	"SwarmList":        true,
	"SwarmValidate":    true,
	"FormulaShow":      true,
	"Cook":             true,
	"LegAdd":           true,
	"SlotShow":         true,
	"SlotSet":          true,
	"SlotClear":        true,
	"Search":           true,
	"Version":          true,
	"Doctor":           true,
	"Prime":            true,
	"Stats":            true,
	"StatsJSON":        true,
	"Flush":            true,
	"Burn":             true,
	"IsBeadsRepo":      true,
	"Run":              true,
}

// IsBdFixed returns whether bd has fixed the routing bug for an operation.
// Returns true if the operation works correctly in bd (no workaround needed).
// Returns true for unknown operations (assume they work).
func IsBdFixed(operation string) bool {
	fixed, ok := BdRoutingBugs[operation]
	if !ok {
		return true // Assume fixed if not listed
	}
	return fixed
}

// BrokenOperations returns a list of operations that have routing bugs in bd.
func BrokenOperations() []string {
	var broken []string
	for op, fixed := range BdRoutingBugs {
		if !fixed {
			broken = append(broken, op)
		}
	}
	return broken
}
