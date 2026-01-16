package doctor

import (
	"fmt"

	"github.com/steveyegge/gastown/internal/beads"
)

// RoleBeadsCheck verifies that role definition beads exist.
// Role beads are templates that define role characteristics and lifecycle hooks.
// They are stored in town beads (~/.beads/) with hq- prefix:
//   - hq-mayor-role, hq-deacon-role, hq-dog-role
//   - hq-witness-role, hq-refinery-role, hq-polecat-role, hq-crew-role
//
// Role beads are created by gt install, but creation may fail silently.
// Without role beads, agents fall back to defaults which may differ from
// user expectations.
type RoleBeadsCheck struct {
	FixableCheck
	missing []string // Track missing role beads for fix
}

// NewRoleBeadsCheck creates a new role beads check.
func NewRoleBeadsCheck() *RoleBeadsCheck {
	return &RoleBeadsCheck{
		FixableCheck: FixableCheck{
			BaseCheck: BaseCheck{
				CheckName:        "role-beads-exist",
				CheckDescription: "Verify role definition beads exist",
				CheckCategory:    CategoryConfig,
			},
		},
	}
}

// Run checks if role beads exist.
func (c *RoleBeadsCheck) Run(ctx *CheckContext) *CheckResult {
	c.missing = nil // Reset

	townBeadsPath := beads.GetTownBeadsPath(ctx.TownRoot)
	bd := beads.New(townBeadsPath)

	var missing []string
	roleDefs := beads.AllRoleBeadDefs()

	for _, role := range roleDefs {
		if _, err := bd.Show(role.ID); err != nil {
			missing = append(missing, role.ID)
		}
	}

	c.missing = missing

	if len(missing) == 0 {
		return &CheckResult{
			Name:     c.Name(),
			Status:   StatusOK,
			Message:  fmt.Sprintf("All %d role beads exist", len(roleDefs)),
			Category: c.Category(),
		}
	}

	return &CheckResult{
		Name:     c.Name(),
		Status:   StatusWarning, // Warning, not error - agents work without role beads
		Message:  fmt.Sprintf("%d role bead(s) missing (agents will use defaults)", len(missing)),
		Details:  missing,
		FixHint:  "Run 'gt doctor --fix' to create missing role beads",
		Category: c.Category(),
	}
}

// Fix creates missing role beads.
func (c *RoleBeadsCheck) Fix(ctx *CheckContext) error {
	// Re-run check to populate missing if needed
	if c.missing == nil {
		result := c.Run(ctx)
		if result.Status == StatusOK {
			return nil // Nothing to fix
		}
	}

	if len(c.missing) == 0 {
		return nil
	}

	// Build lookup map for role definitions
	roleDefMap := make(map[string]beads.RoleBeadDef)
	for _, role := range beads.AllRoleBeadDefs() {
		roleDefMap[role.ID] = role
	}

	// BeadsOps Migration: cmd.Dir=ctx.TownRoot (REQUIRED - town beads), BEADS_DIR N/A
	b := beads.New(ctx.TownRoot)

	// Create missing role beads
	for _, id := range c.missing {
		role, ok := roleDefMap[id]
		if !ok {
			continue // Shouldn't happen
		}

		// Create role bead using CreateWithID for specific ID
		if _, err := b.CreateWithID(role.ID, beads.CreateOptions{
			Type:        "role",
			Title:       role.Title,
			Description: role.Desc,
		}); err != nil {
			return fmt.Errorf("creating %s: %v", role.ID, err)
		}
	}

	return nil
}
