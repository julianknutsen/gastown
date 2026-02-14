package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/config"
)

// determineRigBeadsPath returns the correct route path for a rig based on its actual layout.
// Uses ResolveBeadsDir to follow any redirects (e.g., rig/.beads/redirect -> mayor/rig/.beads).
// Falls back to the default mayor layout path if the resolved path is invalid or escapes the town root.
func determineRigBeadsPath(townRoot, rigName string) string {
	defaultPath := rigName + "/mayor/rig"
	rigPath := filepath.Join(townRoot, rigName)
	resolved := beads.ResolveBeadsDir(rigPath)

	rel, err := filepath.Rel(townRoot, resolved)
	if err != nil {
		return defaultPath
	}

	// Normalize to forward slashes for consistent string operations on all platforms
	rel = filepath.ToSlash(rel)

	// Validate the resolved path stays within the town root
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return defaultPath
	}

	return strings.TrimSuffix(rel, "/.beads")
}

// RoutesCheck verifies that beads routing is properly configured.
// It checks that routes.jsonl exists, all rigs have routing entries,
// and all routes point to valid locations.
type RoutesCheck struct {
	FixableCheck
}

// NewRoutesCheck creates a new routes configuration check.
func NewRoutesCheck() *RoutesCheck {
	return &RoutesCheck{
		FixableCheck: FixableCheck{
			BaseCheck: BaseCheck{
				CheckName:        "routes-config",
				CheckDescription: "Check beads routing configuration",
				CheckCategory:    CategoryConfig,
			},
		},
	}
}

// Run checks the beads routing configuration.
func (c *RoutesCheck) Run(ctx *CheckContext) *CheckResult {
	beadsDir := filepath.Join(ctx.TownRoot, ".beads")
	routesPath := filepath.Join(beadsDir, beads.RoutesFileName)

	// Check if .beads directory exists
	if _, err := os.Stat(beadsDir); os.IsNotExist(err) {
		return &CheckResult{
			Name:    c.Name(),
			Status:  StatusWarning,
			Message: "No .beads directory at town root",
			FixHint: "Run 'bd init' to initialize beads",
		}
	}

	// Check if routes.jsonl exists
	if _, err := os.Stat(routesPath); os.IsNotExist(err) {
		return &CheckResult{
			Name:    c.Name(),
			Status:  StatusWarning,
			Message: "No routes.jsonl file (prefix routing not configured)",
			FixHint: "Run 'gt doctor --fix' to create routes.jsonl",
		}
	}

	// Load existing routes
	routes, err := beads.LoadRoutes(beadsDir)
	if err != nil {
		return &CheckResult{
			Name:    c.Name(),
			Status:  StatusError,
			Message: fmt.Sprintf("Failed to load routes.jsonl: %v", err),
		}
	}

	// Build maps of existing routes
	routeByPrefix := make(map[string]string) // prefix -> path
	routeByPath := make(map[string]string)   // path -> prefix
	for _, r := range routes {
		routeByPrefix[r.Prefix] = r.Path
		routeByPath[r.Path] = r.Prefix
	}

	var details []string
	var missingTownRoute bool
	var missingConvoyRoute bool

	// Check town root route exists (hq- -> .)
	if _, hasTownRoute := routeByPrefix["hq-"]; !hasTownRoute {
		missingTownRoute = true
		details = append(details, "Town root route (hq- -> .) is missing")
	}

	// Check convoy route exists (hq-cv- -> .)
	if _, hasConvoyRoute := routeByPrefix["hq-cv-"]; !hasConvoyRoute {
		missingConvoyRoute = true
		details = append(details, "Convoy route (hq-cv- -> .) is missing")
	}

	// Load rigs registry
	rigsPath := filepath.Join(ctx.TownRoot, "mayor", "rigs.json")
	rigsConfig, err := config.LoadRigsConfig(rigsPath)
	if err != nil {
		// No rigs config - check for missing town/convoy routes and validate existing routes
		if missingTownRoute || missingConvoyRoute {
			return &CheckResult{
				Name:    c.Name(),
				Status:  StatusWarning,
				Message: "Required town routes are missing",
				Details: details,
				FixHint: "Run 'gt doctor --fix' to add missing routes",
			}
		}
		return c.checkRoutesValid(ctx, routes)
	}

	var missingRigs []string
	var invalidRoutes []string
	var suboptimalRoutes []string

	// Check each rig has a route (by path, not just prefix from rigs.json)
	for rigName, rigEntry := range rigsConfig.Rigs {
		// Determine the correct path based on actual rig layout
		expectedPath := determineRigBeadsPath(ctx.TownRoot, rigName)

		prefix := ""
		if rigEntry.BeadsConfig != nil && rigEntry.BeadsConfig.Prefix != "" {
			prefix = rigEntry.BeadsConfig.Prefix + "-"
		}

		// Check if there's already a route for this rig (by path)
		if _, hasRoute := routeByPath[expectedPath]; hasRoute {
			// Rig already has a route with the correct path
			continue
		}

		// No route with expected path — check if there's one by prefix
		if prefix != "" {
			if existingPath, found := routeByPrefix[prefix]; found {
				// Route exists but points to a different path than expected.
				// This happens with legacy rigs where the route points to the rig
				// root (e.g., "crom") instead of the canonical beads location
				// (e.g., "crom/mayor/rig"). The rig-root path relies on a .beads/redirect
				// file, which breaks with bd's routing resolver (beads#1749).
				if existingPath != expectedPath {
					suboptimalRoutes = append(suboptimalRoutes, prefix)
					details = append(details, fmt.Sprintf("Route %s -> %s should be %s -> %s (avoids redirect resolution bug)", prefix, existingPath, prefix, expectedPath))
				}
			} else {
				missingRigs = append(missingRigs, rigName)
				details = append(details, fmt.Sprintf("Rig '%s' (prefix: %s) has no routing entry", rigName, prefix))
			}
		}
	}

	// Check each route points to a valid location
	for _, r := range routes {
		rigPath := filepath.Join(ctx.TownRoot, r.Path)
		beadsPath := filepath.Join(rigPath, ".beads")

		// Special case: "." path is town root, already checked
		if r.Path == "." {
			continue
		}

		// Check if the path exists
		if _, err := os.Stat(rigPath); os.IsNotExist(err) {
			invalidRoutes = append(invalidRoutes, r.Prefix)
			details = append(details, fmt.Sprintf("Route %s -> %s: path does not exist", r.Prefix, r.Path))
			continue
		}

		// Check if .beads directory exists (or redirect file)
		redirectPath := filepath.Join(beadsPath, "redirect")
		_, beadsErr := os.Stat(beadsPath)
		_, redirectErr := os.Stat(redirectPath)

		if os.IsNotExist(beadsErr) && os.IsNotExist(redirectErr) {
			invalidRoutes = append(invalidRoutes, r.Prefix)
			details = append(details, fmt.Sprintf("Route %s -> %s: no .beads directory", r.Prefix, r.Path))
		}
	}

	// Determine result
	if missingTownRoute || missingConvoyRoute || len(missingRigs) > 0 || len(invalidRoutes) > 0 || len(suboptimalRoutes) > 0 {
		status := StatusWarning
		var messageParts []string

		if missingTownRoute {
			messageParts = append(messageParts, "town root route missing")
		}
		if missingConvoyRoute {
			messageParts = append(messageParts, "convoy route missing")
		}
		if len(missingRigs) > 0 {
			messageParts = append(messageParts, fmt.Sprintf("%d rig(s) missing routes", len(missingRigs)))
		}
		if len(invalidRoutes) > 0 {
			messageParts = append(messageParts, fmt.Sprintf("%d invalid route(s)", len(invalidRoutes)))
		}
		if len(suboptimalRoutes) > 0 {
			messageParts = append(messageParts, fmt.Sprintf("%d route(s) using redirect instead of canonical path", len(suboptimalRoutes)))
		}

		return &CheckResult{
			Name:    c.Name(),
			Status:  status,
			Message: strings.Join(messageParts, ", "),
			Details: details,
			FixHint: "Run 'gt doctor --fix' to fix routing issues",
		}
	}

	return &CheckResult{
		Name:    c.Name(),
		Status:  StatusOK,
		Message: fmt.Sprintf("Routes configured correctly (%d routes)", len(routes)),
	}
}

// checkRoutesValid checks that existing routes point to valid locations.
func (c *RoutesCheck) checkRoutesValid(ctx *CheckContext, routes []beads.Route) *CheckResult {
	var details []string
	var invalidCount int

	for _, r := range routes {
		if r.Path == "." {
			continue // Town root is valid
		}

		rigPath := filepath.Join(ctx.TownRoot, r.Path)
		if _, err := os.Stat(rigPath); os.IsNotExist(err) {
			invalidCount++
			details = append(details, fmt.Sprintf("Route %s -> %s: path does not exist", r.Prefix, r.Path))
		}
	}

	if invalidCount > 0 {
		return &CheckResult{
			Name:    c.Name(),
			Status:  StatusWarning,
			Message: fmt.Sprintf("%d invalid route(s) in routes.jsonl", invalidCount),
			Details: details,
			FixHint: "Remove invalid routes or recreate the missing rigs",
		}
	}

	return &CheckResult{
		Name:    c.Name(),
		Status:  StatusOK,
		Message: fmt.Sprintf("Routes configured correctly (%d routes)", len(routes)),
	}
}

// Fix attempts to add missing routing entries and rewrite suboptimal ones.
func (c *RoutesCheck) Fix(ctx *CheckContext) error {
	beadsDir := filepath.Join(ctx.TownRoot, ".beads")

	// Ensure .beads directory exists
	if _, err := os.Stat(beadsDir); os.IsNotExist(err) {
		return fmt.Errorf(".beads directory does not exist; run 'bd init' first")
	}

	// Load existing routes
	routes, err := beads.LoadRoutes(beadsDir)
	if err != nil {
		routes = []beads.Route{} // Start fresh if can't load
	}

	// Build map of existing prefixes to route index for fast lookup
	routeMap := make(map[string]int) // prefix -> index in routes slice
	for i, r := range routes {
		routeMap[r.Prefix] = i
	}

	// Ensure town root route exists (hq- -> .)
	// This is normally created by gt install but may be missing if routes.jsonl was corrupted
	modified := false
	if _, exists := routeMap["hq-"]; !exists {
		routeMap["hq-"] = len(routes)
		routes = append(routes, beads.Route{Prefix: "hq-", Path: "."})
		modified = true
	}

	// Ensure convoy route exists (hq-cv- -> .)
	// Convoys use hq-cv-* IDs for visual distinction from other town beads
	if _, exists := routeMap["hq-cv-"]; !exists {
		routeMap["hq-cv-"] = len(routes)
		routes = append(routes, beads.Route{Prefix: "hq-cv-", Path: "."})
		modified = true
	}

	// Load rigs registry
	rigsPath := filepath.Join(ctx.TownRoot, "mayor", "rigs.json")
	rigsConfig, err := config.LoadRigsConfig(rigsPath)
	if err != nil {
		// No rigs config - just write town root route if we added it
		if modified {
			return beads.WriteRoutes(beadsDir, routes)
		}
		return nil
	}

	// Add missing routes and rewrite suboptimal ones for each rig.
	// Suboptimal routes point to the rig root (e.g., "crom") which relies on
	// a .beads/redirect file. The beads routing resolver has a bug resolving
	// redirects (beads#1749), so we rewrite to the canonical path (e.g.,
	// "crom/mayor/rig") which has a real .beads directory and needs no redirect.
	for rigName, rigEntry := range rigsConfig.Rigs {
		prefix := ""
		if rigEntry.BeadsConfig != nil && rigEntry.BeadsConfig.Prefix != "" {
			prefix = rigEntry.BeadsConfig.Prefix + "-"
		}

		if prefix == "" {
			continue
		}

		// Determine the correct canonical path based on actual rig layout
		rigRoutePath := determineRigBeadsPath(ctx.TownRoot, rigName)
		canonicalPath := filepath.Join(ctx.TownRoot, rigRoutePath)

		if idx, exists := routeMap[prefix]; exists {
			// Route exists — check if it points to the canonical path
			if routes[idx].Path != rigRoutePath {
				if _, err := os.Stat(canonicalPath); err == nil {
					routes[idx].Path = rigRoutePath
					modified = true
				}
			}
		} else {
			// Route missing — add it if the canonical path exists
			if _, err := os.Stat(canonicalPath); err == nil {
				routeMap[prefix] = len(routes)
				routes = append(routes, beads.Route{
					Prefix: prefix,
					Path:   rigRoutePath,
				})
				modified = true
			}
		}
	}

	if modified {
		return beads.WriteRoutes(beadsDir, routes)
	}

	return nil
}
