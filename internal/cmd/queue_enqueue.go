package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/workspace"
)

// Package-level flag vars for gt queue <id> flags.
var (
	queueOnTarget    string   // --on <bead>: formula-on-bead mode
	queueFormula     string   // --formula
	queueHookRawBead bool     // --hook-raw-bead
	queueArgs        string   // --args / -a
	queueVars        []string // --var (repeated)
	queueMerge       string   // --merge
	queueBaseBranch  string   // --base-branch
	queueNoConvoy    bool     // --no-convoy
	queueOwned       bool     // --owned
	queueNoMerge     bool     // --no-merge
	queueForce       bool     // --force
	queueDryRun      bool     // --dry-run / -n
	queueAccount     string   // --account
	queueAgent       string   // --agent
	queueRalph       bool     // --ralph
)

func init() {
	queueCmd.Flags().StringVar(&queueOnTarget, "on", "", "Apply formula to existing bead (first arg becomes formula name)")
	queueCmd.Flags().StringVar(&queueFormula, "formula", "", "Formula to apply (default: mol-polecat-work)")
	queueCmd.Flags().BoolVar(&queueHookRawBead, "hook-raw-bead", false, "Hook raw bead without formula")
	queueCmd.Flags().StringVarP(&queueArgs, "args", "a", "", "Natural language instructions for executor")
	queueCmd.Flags().StringArrayVar(&queueVars, "var", nil, "Formula variable (key=value)")
	queueCmd.Flags().StringVar(&queueMerge, "merge", "", "Merge strategy: direct/mr/local")
	queueCmd.Flags().StringVar(&queueBaseBranch, "base-branch", "", "Override base branch for polecat worktree")
	queueCmd.Flags().BoolVar(&queueNoConvoy, "no-convoy", false, "Skip auto-convoy creation")
	queueCmd.Flags().BoolVar(&queueOwned, "owned", false, "Mark auto-convoy as caller-managed lifecycle")
	queueCmd.Flags().BoolVar(&queueNoMerge, "no-merge", false, "Skip merge queue on completion")
	queueCmd.Flags().BoolVar(&queueForce, "force", false, "Force enqueue even if bead is hooked/in_progress")
	queueCmd.Flags().BoolVarP(&queueDryRun, "dry-run", "n", false, "Show what would be done without acting")
	queueCmd.Flags().StringVar(&queueAccount, "account", "", "Claude Code account handle")
	queueCmd.Flags().StringVar(&queueAgent, "agent", "", "Agent override (e.g., gemini, codex)")
	queueCmd.Flags().BoolVar(&queueRalph, "ralph", false, "Enable Ralph Wiggum loop mode")
}

// detectQueueIDType determines what kind of ID was passed to gt queue.
// Returns "convoy", "epic", or "task".
func detectQueueIDType(id string) (string, error) {
	// Fast path: hq-cv-* is always a convoy
	if strings.HasPrefix(id, "hq-cv-") {
		return "convoy", nil
	}

	// Query bead for issue_type and labels
	info, err := getBeadInfo(id)
	if err != nil {
		return "", fmt.Errorf("cannot resolve bead '%s': %w", id, err)
	}

	// Check issue_type field first
	switch info.IssueType {
	case "epic":
		return "epic", nil
	case "convoy":
		return "convoy", nil
	}

	// Fallback: check gt:<type> labels (issue_type is deprecated in favor of labels)
	for _, label := range info.Labels {
		switch label {
		case "gt:epic":
			return "epic", nil
		case "gt:convoy":
			return "convoy", nil
		}
	}

	return "task", nil
}

// taskOnlyFlagNames lists flags that only apply to task bead enqueue,
// not convoy or epic mode. Used to reject silent flag dropping.
var taskOnlyFlagNames = []string{
	"account", "agent", "ralph", "args", "var",
	"merge", "base-branch", "no-convoy", "owned", "no-merge",
}

// validateNoTaskOnlyFlags checks that no task-only flags were set.
// Returns an error listing which unsupported flags were used.
func validateNoTaskOnlyFlags(cmd *cobra.Command, mode string) error {
	var used []string
	for _, name := range taskOnlyFlagNames {
		if f := cmd.Flags().Lookup(name); f != nil && f.Changed {
			used = append(used, "--"+name)
		}
	}
	if len(used) > 0 {
		return fmt.Errorf("%s mode does not support: %s\nThese flags only apply to task bead enqueue",
			mode, strings.Join(used, ", "))
	}
	return nil
}

// runQueueEnqueue is the unified entry point for gt queue <id>.
// It auto-detects the ID type and routes to the appropriate handler.
func runQueueEnqueue(cmd *cobra.Command, args []string) error {
	// Normalize: trim trailing slashes from args to handle tab-completion
	// artifacts like "gt queue gt-abc testrig/" → "gt queue gt-abc testrig".
	// Matches the normalization in runSling (sling.go).
	for i := range args {
		args[i] = strings.TrimRight(args[i], "/")
	}

	// --on mode: formula-on-bead
	if queueOnTarget != "" {
		return runFormulaOnBeadEnqueue(args)
	}

	// Detect ID type from first arg
	idType, err := detectQueueIDType(args[0])
	if err != nil {
		return err
	}

	switch idType {
	case "convoy":
		if len(args) > 1 {
			return fmt.Errorf("convoy mode accepts exactly one convoy ID, got %d args\nTo queue multiple convoys, run gt queue for each one separately", len(args))
		}
		if err := validateNoTaskOnlyFlags(cmd, "convoy"); err != nil {
			return err
		}
		return runConvoyQueueByID(args[0], convoyQueueOpts{
			Formula:     resolveFormula(queueFormula, queueHookRawBead),
			HookRawBead: queueHookRawBead,
			Force:       queueForce,
			DryRun:      queueDryRun,
		})
	case "epic":
		if len(args) > 1 {
			return fmt.Errorf("epic mode accepts exactly one epic ID, got %d args\nTo queue multiple epics, run gt queue for each one separately", len(args))
		}
		if err := validateNoTaskOnlyFlags(cmd, "epic"); err != nil {
			return err
		}
		return runEpicQueueByID(args[0], epicQueueOpts{
			Formula:     resolveFormula(queueFormula, queueHookRawBead),
			HookRawBead: queueHookRawBead,
			Force:       queueForce,
			DryRun:      queueDryRun,
		})
	default:
		return runTaskQueueEnqueue(args)
	}
}

// runFormulaOnBeadEnqueue handles gt queue <formula> --on <bead> [rig].
func runFormulaOnBeadEnqueue(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("--on requires a formula name as first argument (e.g., gt queue mol-review --on %s)", queueOnTarget)
	}
	if len(args) > 2 {
		return fmt.Errorf("--on mode accepts: gt queue <formula> --on <bead> [rig] (got %d args, expected 1 or 2)", len(args))
	}
	if queueFormula != "" {
		return fmt.Errorf("cannot use --formula with --on (the first argument is the formula name in --on mode)")
	}
	if queueHookRawBead {
		return fmt.Errorf("cannot use --hook-raw-bead with --on (--on already specifies the formula as the first argument)")
	}

	formulaName := args[0]
	beadID := queueOnTarget

	// Check if a trailing rig was provided
	explicitRig := ""
	if len(args) == 2 {
		if rigName, isRig := IsRigName(args[1]); isRig {
			explicitRig = rigName
		} else {
			return fmt.Errorf("unexpected argument %q (expected a rig name)", args[1])
		}
	}

	// Resolve rig
	rigName := explicitRig
	if rigName == "" {
		townRoot, err := workspace.FindFromCwdOrError()
		if err != nil {
			return err
		}
		rigName = resolveRigForBead(townRoot, beadID)
		if rigName == "" {
			return fmt.Errorf("cannot resolve rig for '%s' (use: gt queue %s --on %s <rig>)", beadID, formulaName, beadID)
		}
	}

	formula := formulaName

	return enqueueBead(beadID, rigName, EnqueueOptions{
		Formula:     formula,
		Args:        queueArgs,
		Vars:        queueVars,
		Merge:       queueMerge,
		BaseBranch:  queueBaseBranch,
		NoConvoy:    queueNoConvoy,
		Owned:       queueOwned,
		DryRun:      queueDryRun,
		Force:       queueForce,
		NoMerge:     queueNoMerge,
		Account:     queueAccount,
		Agent:       queueAgent,
		HookRawBead: queueHookRawBead,
		Ralph:       queueRalph,
	})
}

// runTaskQueueEnqueue handles gt queue <bead>... [rig] for task beads.
func runTaskQueueEnqueue(args []string) error {
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return err
	}

	// Check if last arg is an explicit rig
	explicitRig := ""
	beadArgs := args
	if len(args) >= 2 {
		if rigName, isRig := IsRigName(args[len(args)-1]); isRig {
			explicitRig = rigName
			beadArgs = args[:len(args)-1]
		}
	}

	formula := resolveFormula(queueFormula, queueHookRawBead)

	// Single bead
	if len(beadArgs) == 1 {
		rigName := explicitRig
		if rigName == "" {
			rigName = resolveRigForBead(townRoot, beadArgs[0])
			if rigName == "" {
				prefix := beads.ExtractPrefix(beadArgs[0])
				return fmt.Errorf("cannot resolve rig for '%s' from prefix %q (use: gt queue %s <rig>)", beadArgs[0], prefix, beadArgs[0])
			}
		}
		return enqueueBead(beadArgs[0], rigName, EnqueueOptions{
			Formula:     formula,
			Args:        queueArgs,
			Vars:        queueVars,
			Merge:       queueMerge,
			BaseBranch:  queueBaseBranch,
			NoConvoy:    queueNoConvoy,
			Owned:       queueOwned,
			DryRun:      queueDryRun,
			Force:       queueForce,
			NoMerge:     queueNoMerge,
			Account:     queueAccount,
			Agent:       queueAgent,
			HookRawBead: queueHookRawBead,
			Ralph:       queueRalph,
		})
	}

	// Batch: validate no mixed ID types (epics/convoys in a task batch).
	// Skip beadArgs[0] — it was already detected as "task" by runQueueEnqueue,
	// avoiding a redundant getBeadInfo call (each call spawns a bd subprocess).
	for _, beadID := range beadArgs[1:] {
		idType, err := detectQueueIDType(beadID)
		if err != nil {
			return fmt.Errorf("cannot resolve bead '%s': %w\nEach ID in a batch must be a valid bead", beadID, err)
		}
		if idType != "task" {
			return fmt.Errorf("mixed ID types in batch: '%s' is a %s, not a task bead\nConvoys and epics must be queued individually: gt queue %s", beadID, idType, beadID)
		}
	}

	// Batch: enqueue each bead
	if queueDryRun {
		fmt.Printf("%s Would queue %d beads:\n", style.Bold.Render("DRY-RUN"), len(beadArgs))
	}

	successCount := 0
	for _, beadID := range beadArgs {
		rigName := explicitRig
		if rigName == "" {
			rigName = resolveRigForBead(townRoot, beadID)
			if rigName == "" {
				prefix := beads.ExtractPrefix(beadID)
				fmt.Printf("  %s %s: cannot resolve rig from prefix %q\n", style.Dim.Render("✗"), beadID, prefix)
				continue
			}
		}
		if err := enqueueBead(beadID, rigName, EnqueueOptions{
			Formula:     formula,
			Args:        queueArgs,
			Vars:        queueVars,
			Merge:       queueMerge,
			BaseBranch:  queueBaseBranch,
			NoConvoy:    queueNoConvoy,
			Owned:       queueOwned,
			DryRun:      queueDryRun,
			Force:       queueForce,
			NoMerge:     queueNoMerge,
			Account:     queueAccount,
			Agent:       queueAgent,
			HookRawBead: queueHookRawBead,
			Ralph:       queueRalph,
		}); err != nil {
			fmt.Printf("  %s %s: %v\n", style.Dim.Render("✗"), beadID, err)
			continue
		}
		successCount++
	}

	verb := "Queued"
	if queueDryRun {
		verb = "Would queue"
	}
	fmt.Printf("\n%s %s %d/%d beads\n", style.Bold.Render("📊"), verb, successCount, len(beadArgs))
	if successCount == 0 {
		return fmt.Errorf("all %d enqueue attempts failed", len(beadArgs))
	}
	return nil
}
