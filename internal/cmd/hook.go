package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/wisp"
)

var hookCmd = &cobra.Command{
	Use:     "hook <bead-id>",
	GroupID: GroupWork,
	Short:   "Attach work to your hook (durable across restarts)",
	Long: `Attach a bead (issue) to your hook for durable work tracking.

The hook is the "durability primitive" - work on your hook survives session
restarts, context compaction, and handoffs. When you restart (via gt handoff),
your SessionStart hook finds the attached work and you continue from where
you left off.

This is "assign without action" - use gt sling to also start immediately,
or gt handoff to hook and restart with fresh context.

Examples:
  gt hook gt-abc                    # Attach issue gt-abc to your hook
  gt hook gt-abc -s "Fix the bug"   # With subject for handoff mail
  gt hook gt-abc -m "Check tests"   # With context message

Related commands:
  gt mol status      # See what's on your hook
  gt sling <bead>    # Hook + start now (keep context)
  gt handoff <bead>  # Hook + restart (fresh context)
  gt nudge <agent>   # Send message to trigger execution`,
	Args: cobra.ExactArgs(1),
	RunE: runHook,
}

var (
	hookSubject string
	hookMessage string
	hookDryRun  bool
)

func init() {
	hookCmd.Flags().StringVarP(&hookSubject, "subject", "s", "", "Subject for handoff mail (optional)")
	hookCmd.Flags().StringVarP(&hookMessage, "message", "m", "", "Message for handoff mail (optional)")
	hookCmd.Flags().BoolVarP(&hookDryRun, "dry-run", "n", false, "Show what would be done")
	rootCmd.AddCommand(hookCmd)
}

func runHook(cmd *cobra.Command, args []string) error {
	beadID := args[0]

	// Polecats cannot hook - they use gt done for lifecycle
	if polecatName := os.Getenv("GT_POLECAT"); polecatName != "" {
		return fmt.Errorf("polecats cannot hook work (use gt done for handoff)")
	}

	// Verify the bead exists
	if err := verifyBeadExists(beadID); err != nil {
		return err
	}

	// Determine agent identity and clone root
	agentID, _, cloneRoot, err := resolveSelfTarget()
	if err != nil {
		return fmt.Errorf("detecting agent identity: %w", err)
	}

	// Create the slung work wisp
	sw := wisp.NewSlungWork(beadID, agentID)
	sw.Subject = hookSubject
	sw.Context = hookMessage

	fmt.Printf("%s Hooking %s...\n", style.Bold.Render("🪝"), beadID)

	if hookDryRun {
		fmt.Printf("Would create wisp: %s\n", wisp.HookPath(cloneRoot, agentID))
		fmt.Printf("  bead_id: %s\n", beadID)
		fmt.Printf("  agent: %s\n", agentID)
		if hookSubject != "" {
			fmt.Printf("  subject: %s\n", hookSubject)
		}
		if hookMessage != "" {
			fmt.Printf("  context: %s\n", hookMessage)
		}
		return nil
	}

	// Write the wisp to the hook
	if err := wisp.WriteSlungWork(cloneRoot, agentID, sw); err != nil {
		return fmt.Errorf("writing wisp: %w", err)
	}

	fmt.Printf("%s Work attached to hook\n", style.Bold.Render("✓"))
	fmt.Printf("  Use 'gt handoff' to restart with this work\n")
	fmt.Printf("  Use 'gt mol status' to see hook status\n")

	return nil
}

// verifyBeadExists checks that the bead exists using bd show.
// Defined in sling.go but duplicated here for clarity. Will be consolidated
// when sling.go is removed.
func verifyBeadExistsForHook(beadID string) error {
	cmd := exec.Command("bd", "show", beadID, "--json")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bead '%s' not found (bd show failed)", beadID)
	}
	return nil
}

