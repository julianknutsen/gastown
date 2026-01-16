package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/style"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// PatrolConfig holds role-specific patrol configuration.
type PatrolConfig struct {
	RoleName      string   // "deacon", "witness", "refinery"
	PatrolMolName string   // "mol-deacon-patrol", etc.
	BeadsDir      string   // where to look for beads
	Assignee      string   // agent identity for pinning
	HeaderEmoji   string   // display emoji
	HeaderTitle   string   // "Patrol Status", etc.
	WorkLoopSteps []string // role-specific instructions
	CheckInProgress bool   // whether to check in_progress status first (witness/refinery do, deacon doesn't)
}

// findActivePatrol finds an active patrol molecule for the role.
// Returns the patrol ID, display line, and whether one was found.
func findActivePatrol(cfg PatrolConfig) (patrolID, patrolLine string, found bool) {
	b := beads.New(cfg.BeadsDir)

	// Check for in-progress patrol first (if configured)
	if cfg.CheckInProgress {
		issues, err := b.List(beads.ListOptions{Status: "in_progress", Type: "epic"})
		if err != nil {
			fmt.Fprintf(os.Stderr, "bd list: %v\n", err)
		} else {
			for _, issue := range issues {
				if strings.Contains(issue.Title, cfg.PatrolMolName) && !strings.Contains(issue.Title, "[template]") {
					displayLine := fmt.Sprintf("%s %s [%s]", issue.ID, issue.Title, issue.Status)
					return issue.ID, displayLine, true
				}
			}
		}
	}

	// Check for open patrols with open children (active wisp)
	issues, err := b.List(beads.ListOptions{Status: "open", Type: "epic"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "bd list: %v\n", err)
	} else {
		for _, issue := range issues {
			if strings.Contains(issue.Title, cfg.PatrolMolName) && !strings.Contains(issue.Title, "[template]") {
				// Check if this molecule has open children
				fullIssue, err := b.Show(issue.ID)
				if err != nil {
					fmt.Fprintf(os.Stderr, "bd show: %v\n", err)
				} else {
					// Check dependents for open/in_progress status
					hasOpenChildren := false
					for _, dep := range fullIssue.Dependents {
						if dep.Status == "open" {
							hasOpenChildren = true
							break
						}
						if cfg.CheckInProgress && dep.Status == "in_progress" {
							hasOpenChildren = true
							break
						}
					}
					if hasOpenChildren {
						displayLine := fmt.Sprintf("%s %s [%s]", issue.ID, issue.Title, issue.Status)
						return issue.ID, displayLine, true
					}
				}
			}
		}
	}

	return "", "", false
}

// autoSpawnPatrol creates and pins a new patrol wisp.
// Returns the patrol ID or an error.
func autoSpawnPatrol(cfg PatrolConfig) (string, error) {
	b := beads.New(cfg.BeadsDir)

	// Find the proto ID for the patrol molecule
	protos, err := b.MolCatalog()
	if err != nil {
		return "", fmt.Errorf("failed to list molecule catalog: %w", err)
	}

	// Find patrol molecule in catalog
	var protoID string
	for _, proto := range protos {
		if strings.Contains(proto.Name, cfg.PatrolMolName) || strings.Contains(proto.ID, cfg.PatrolMolName) {
			protoID = proto.ID
			break
		}
	}

	if protoID == "" {
		return "", fmt.Errorf("proto %s not found in catalog", cfg.PatrolMolName)
	}

	// Create the patrol wisp
	wisp, err := b.WispCreate(protoID, cfg.RoleName)
	if err != nil {
		return "", fmt.Errorf("failed to create patrol wisp: %w", err)
	}

	patrolID := wisp.ID

	// Hook the wisp to the agent so gt mol status sees it
	status := beads.StatusHooked
	if err := b.Update(patrolID, beads.UpdateOptions{Status: &status, Assignee: &cfg.Assignee}); err != nil {
		return patrolID, fmt.Errorf("created wisp %s but failed to hook", patrolID)
	}

	return patrolID, nil
}

// outputPatrolContext is the main function that handles patrol display logic.
// It finds or creates a patrol and outputs the status and work loop.
func outputPatrolContext(cfg PatrolConfig) {
	fmt.Println()
	fmt.Printf("%s\n\n", style.Bold.Render(fmt.Sprintf("## %s %s", cfg.HeaderEmoji, cfg.HeaderTitle)))

	// Try to find an active patrol
	patrolID, patrolLine, hasPatrol := findActivePatrol(cfg)

	if !hasPatrol {
		// No active patrol - auto-spawn one
		fmt.Printf("Status: **No active patrol** - creating %s...\n", cfg.PatrolMolName)
		fmt.Println()

		var err error
		patrolID, err = autoSpawnPatrol(cfg)
		if err != nil {
			if patrolID != "" {
				fmt.Printf("⚠ %s\n", err.Error())
			} else {
				fmt.Println(style.Dim.Render(err.Error()))
				fmt.Println(style.Dim.Render(fmt.Sprintf("Run `bd mol catalog` to troubleshoot.")))
				return
			}
		} else {
			fmt.Printf("✓ Created and hooked patrol wisp: %s\n", patrolID)
		}
	} else {
		// Has active patrol - show status
		fmt.Println("Status: **Patrol Active**")
		fmt.Printf("Patrol: %s\n\n", strings.TrimSpace(patrolLine))
	}

	// Show patrol work loop instructions
	fmt.Printf("**%s Patrol Work Loop:**\n", cases.Title(language.English).String(cfg.RoleName))
	for i, step := range cfg.WorkLoopSteps {
		fmt.Printf("%d. %s\n", i+1, step)
	}

	if patrolID != "" {
		fmt.Println()
		fmt.Printf("Current patrol ID: %s\n", patrolID)
	}
}
