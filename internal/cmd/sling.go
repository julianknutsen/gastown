package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/tmux"
	"github.com/steveyegge/gastown/internal/wisp"
)

var slingCmd = &cobra.Command{
	Use:     "sling <bead-or-formula> [target]",
	GroupID: GroupWork,
	Short:   "Hook work and start immediately (no restart)",
	Long: `Sling work onto an agent's hook and start working immediately.

Unlike 'gt handoff', sling does NOT restart the session. It:
  1. Attaches the work to the hook (durability)
  2. Injects a prompt to start working NOW

This preserves current context while kicking off work. Use when:
  - You've been chatting with an agent and want to kick off a workflow
  - You want to assign work to another agent that has useful context
  - You (Overseer) want to start work then attend to another window

The hook provides durability - the agent can restart, compact, or hand off,
but until the hook is changed or closed, that agent owns the work.

Examples:
  gt sling gt-abc                       # Hook bead and start now
  gt sling gt-abc -s "Fix the bug"      # With context subject
  gt sling gt-abc crew                  # Sling bead to crew worker
  gt sling gt-abc gastown/crew/max      # Sling bead to specific agent

Standalone formula slinging:
  gt sling mol-town-shutdown mayor/     # Cook + wisp + attach + nudge
  gt sling towers-of-hanoi --var disks=3  # With formula variables

When the first argument is a formula (not a bead), sling will:
  1. Cook the formula (bd cook)
  2. Create a wisp instance (bd wisp)
  3. Attach the wisp to the target's hook
  4. Nudge the target to start

Formula-on-bead scaffolding (--on flag):
  gt sling shiny --on gt-abc            # Apply shiny formula to existing work
  gt sling mol-review --on gt-abc crew  # Apply review formula, sling to crew

When --on is specified, the formula shapes execution of the target bead.

Compare:
  gt hook <bead>      # Just attach (no action)
  gt sling <bead>     # Attach + start now (keep context)
  gt handoff <bead>   # Attach + restart (fresh context)

The propulsion principle: if it's on your hook, YOU RUN IT.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runSling,
}

var (
	slingSubject  string
	slingMessage  string
	slingDryRun   bool
	slingOnTarget string   // --on flag: target bead when slinging a formula
	slingVars     []string // --var flag: formula variables (key=value)
)

func init() {
	slingCmd.Flags().StringVarP(&slingSubject, "subject", "s", "", "Context subject for the work")
	slingCmd.Flags().StringVarP(&slingMessage, "message", "m", "", "Context message for the work")
	slingCmd.Flags().BoolVarP(&slingDryRun, "dry-run", "n", false, "Show what would be done")
	slingCmd.Flags().StringVar(&slingOnTarget, "on", "", "Apply formula to existing bead (implies wisp scaffolding)")
	slingCmd.Flags().StringArrayVar(&slingVars, "var", nil, "Formula variable (key=value), can be repeated")
	rootCmd.AddCommand(slingCmd)
}

func runSling(cmd *cobra.Command, args []string) error {
	// Polecats cannot sling - check early before writing anything
	if polecatName := os.Getenv("GT_POLECAT"); polecatName != "" {
		return fmt.Errorf("polecats cannot sling (use gt done for handoff)")
	}

	// --var is only for standalone formula mode, not formula-on-bead mode
	if slingOnTarget != "" && len(slingVars) > 0 {
		return fmt.Errorf("--var cannot be used with --on (formula-on-bead mode doesn't support variables)")
	}

	// Determine mode based on flags and argument types
	var beadID string
	var formulaName string

	if slingOnTarget != "" {
		// Formula-on-bead mode: gt sling <formula> --on <bead>
		formulaName = args[0]
		beadID = slingOnTarget
		// Verify both exist
		if err := verifyBeadExists(beadID); err != nil {
			return err
		}
		if err := verifyFormulaExists(formulaName); err != nil {
			return err
		}
	} else {
		// Could be bead mode or standalone formula mode
		firstArg := args[0]

		// Try as bead first
		if err := verifyBeadExists(firstArg); err == nil {
			// It's a bead
			beadID = firstArg
		} else {
			// Not a bead - try as standalone formula
			if err := verifyFormulaExists(firstArg); err == nil {
				// Standalone formula mode: gt sling <formula> [target]
				return runSlingFormula(args)
			}
			// Neither bead nor formula
			return fmt.Errorf("'%s' is not a valid bead or formula", firstArg)
		}
	}

	// Determine target agent (self or specified)
	var targetAgent string
	var targetPane string
	var hookRoot string // Where to store the hook (role's home)
	var err error

	if len(args) > 1 {
		// Slinging to another agent
		targetAgent, targetPane, hookRoot, err = resolveTargetAgent(args[1])
		if err != nil {
			return fmt.Errorf("resolving target: %w", err)
		}
	} else {
		// Slinging to self
		targetAgent, targetPane, hookRoot, err = resolveSelfTarget()
		if err != nil {
			return err
		}
	}

	// Create the slung work wisp
	sw := wisp.NewSlungWork(beadID, targetAgent)
	sw.Subject = slingSubject
	sw.Context = slingMessage
	sw.Formula = formulaName

	// Display what we're doing
	if formulaName != "" {
		fmt.Printf("%s Slinging formula %s on %s to %s...\n", style.Bold.Render("🎯"), formulaName, beadID, targetAgent)
	} else {
		fmt.Printf("%s Slinging %s to %s...\n", style.Bold.Render("🎯"), beadID, targetAgent)
	}

	if slingDryRun {
		fmt.Printf("Would create wisp: %s\n", wisp.HookPath(hookRoot, targetAgent))
		fmt.Printf("  bead_id: %s\n", beadID)
		if formulaName != "" {
			fmt.Printf("  formula: %s\n", formulaName)
		}
		fmt.Printf("  agent: %s\n", targetAgent)
		fmt.Printf("  hook_root: %s\n", hookRoot)
		if slingSubject != "" {
			fmt.Printf("  subject: %s\n", slingSubject)
		}
		if slingMessage != "" {
			fmt.Printf("  context: %s\n", slingMessage)
		}
		fmt.Printf("Would inject start prompt to pane: %s\n", targetPane)
		return nil
	}

	// Write the wisp to the hook
	if err := wisp.WriteSlungWork(hookRoot, targetAgent, sw); err != nil {
		return fmt.Errorf("writing wisp: %w", err)
	}

	fmt.Printf("%s Work attached to hook\n", style.Bold.Render("✓"))

	// Inject the "start now" prompt
	if err := injectStartPrompt(targetPane, beadID, slingSubject); err != nil {
		return fmt.Errorf("injecting start prompt: %w", err)
	}

	fmt.Printf("%s Start prompt sent\n", style.Bold.Render("▶"))
	return nil
}

// injectStartPrompt sends a prompt to the target pane to start working.
// Uses the reliable nudge pattern: literal mode + 500ms debounce + separate Enter.
func injectStartPrompt(pane, beadID, subject string) error {
	if pane == "" {
		return fmt.Errorf("no target pane")
	}

	// Build the prompt to inject
	var prompt string
	if subject != "" {
		prompt = fmt.Sprintf("Work slung: %s (%s). Start working on it now - no questions, just begin.", beadID, subject)
	} else {
		prompt = fmt.Sprintf("Work slung: %s. Start working on it now - run `gt mol status` to see the hook, then begin.", beadID)
	}

	// Use the reliable nudge pattern (same as gt nudge / tmux.NudgeSession)
	t := tmux.NewTmux()
	return t.NudgePane(pane, prompt)
}

// resolveTargetAgent converts a target spec to agent ID, pane, and hook root.
func resolveTargetAgent(target string) (agentID string, pane string, hookRoot string, err error) {
	// First resolve to session name
	sessionName, err := resolveRoleToSession(target)
	if err != nil {
		return "", "", "", err
	}

	// Get the pane for that session
	pane, err = getSessionPane(sessionName)
	if err != nil {
		return "", "", "", fmt.Errorf("getting pane for %s: %w", sessionName, err)
	}

	// Get the target's working directory for hook storage
	t := tmux.NewTmux()
	hookRoot, err = t.GetPaneWorkDir(sessionName)
	if err != nil {
		return "", "", "", fmt.Errorf("getting working dir for %s: %w", sessionName, err)
	}

	// Convert session name back to agent ID format
	agentID = sessionToAgentID(sessionName)
	return agentID, pane, hookRoot, nil
}

// sessionToAgentID converts a session name to agent ID format.
func sessionToAgentID(session string) string {
	switch {
	case session == "gt-mayor":
		return "mayor"
	case session == "gt-deacon":
		return "deacon"
	case strings.Contains(session, "-crew-"):
		// gt-gastown-crew-max -> gastown/crew/max
		parts := strings.Split(session, "-")
		for i, p := range parts {
			if p == "crew" && i > 1 && i < len(parts)-1 {
				rig := strings.Join(parts[1:i], "-")
				name := strings.Join(parts[i+1:], "-")
				return fmt.Sprintf("%s/crew/%s", rig, name)
			}
		}
	case strings.HasSuffix(session, "-witness"):
		rig := strings.TrimPrefix(session, "gt-")
		rig = strings.TrimSuffix(rig, "-witness")
		return fmt.Sprintf("%s/witness", rig)
	case strings.HasSuffix(session, "-refinery"):
		rig := strings.TrimPrefix(session, "gt-")
		rig = strings.TrimSuffix(rig, "-refinery")
		return fmt.Sprintf("%s/refinery", rig)
	}
	return session
}

// verifyBeadExists checks that the bead exists using bd show.
func verifyBeadExists(beadID string) error {
	cmd := exec.Command("bd", "show", beadID, "--json")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bead '%s' not found (bd show failed)", beadID)
	}
	return nil
}

// detectCloneRoot finds the root of the current git clone.
func detectCloneRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not in a git repository")
	}
	return strings.TrimSpace(string(out)), nil
}

// resolveSelfTarget determines agent identity, pane, and hook root for slinging to self.
func resolveSelfTarget() (agentID string, pane string, hookRoot string, err error) {
	roleInfo, err := GetRole()
	if err != nil {
		return "", "", "", fmt.Errorf("detecting role: %w", err)
	}

	// Build agent identity from role
	switch roleInfo.Role {
	case RoleMayor:
		agentID = "mayor"
	case RoleDeacon:
		agentID = "deacon"
	case RoleWitness:
		agentID = fmt.Sprintf("%s/witness", roleInfo.Rig)
	case RoleRefinery:
		agentID = fmt.Sprintf("%s/refinery", roleInfo.Rig)
	case RolePolecat:
		agentID = fmt.Sprintf("%s/polecats/%s", roleInfo.Rig, roleInfo.Polecat)
	case RoleCrew:
		agentID = fmt.Sprintf("%s/crew/%s", roleInfo.Rig, roleInfo.Polecat)
	default:
		return "", "", "", fmt.Errorf("cannot determine agent identity (role: %s)", roleInfo.Role)
	}

	pane = os.Getenv("TMUX_PANE")
	hookRoot = roleInfo.Home
	if hookRoot == "" {
		// Fallback to git root if home not determined
		hookRoot, err = detectCloneRoot()
		if err != nil {
			return "", "", "", fmt.Errorf("detecting clone root: %w", err)
		}
	}

	return agentID, pane, hookRoot, nil
}

// verifyFormulaExists checks that the formula exists using bd formula show.
// Formulas can be formula files (.formula.json/.formula.toml).
func verifyFormulaExists(formulaName string) error {
	// Try bd formula show (handles all formula file formats)
	cmd := exec.Command("bd", "formula", "show", formulaName)
	if err := cmd.Run(); err == nil {
		return nil
	}

	// Try with mol- prefix
	cmd = exec.Command("bd", "formula", "show", "mol-"+formulaName)
	if err := cmd.Run(); err == nil {
		return nil
	}

	return fmt.Errorf("formula '%s' not found (check 'bd formula list')", formulaName)
}

// runSlingFormula handles standalone formula slinging.
// Flow: cook → wisp → attach to hook → nudge
func runSlingFormula(args []string) error {
	formulaName := args[0]

	// Determine target (self or specified)
	var target string
	if len(args) > 1 {
		target = args[1]
	}

	// Resolve target agent and pane
	var targetAgent string
	var targetPane string
	var hookRoot string
	var err error

	if target != "" {
		// Slinging to another agent
		targetAgent, targetPane, hookRoot, err = resolveTargetAgent(target)
		if err != nil {
			return fmt.Errorf("resolving target: %w", err)
		}
	} else {
		// Slinging to self
		targetAgent, targetPane, hookRoot, err = resolveSelfTarget()
		if err != nil {
			return err
		}
	}

	fmt.Printf("%s Slinging formula %s to %s...\n", style.Bold.Render("🎯"), formulaName, targetAgent)

	if slingDryRun {
		fmt.Printf("Would cook formula: %s\n", formulaName)
		fmt.Printf("Would create wisp and attach to hook: %s\n", wisp.HookPath(hookRoot, targetAgent))
		for _, v := range slingVars {
			fmt.Printf("  --var %s\n", v)
		}
		fmt.Printf("Would nudge pane: %s\n", targetPane)
		return nil
	}

	// Step 1: Cook the formula (ensures proto exists)
	fmt.Printf("  Cooking formula...\n")
	cookArgs := []string{"cook", formulaName}
	cookCmd := exec.Command("bd", cookArgs...)
	cookCmd.Stderr = os.Stderr
	if err := cookCmd.Run(); err != nil {
		return fmt.Errorf("cooking formula: %w", err)
	}

	// Step 2: Create wisp instance (ephemeral)
	fmt.Printf("  Creating wisp...\n")
	wispArgs := []string{"wisp", formulaName}
	for _, v := range slingVars {
		wispArgs = append(wispArgs, "--var", v)
	}
	wispArgs = append(wispArgs, "--json")

	wispCmd := exec.Command("bd", wispArgs...)
	wispCmd.Stderr = os.Stderr // Show wisp errors to user
	wispOut, err := wispCmd.Output()
	if err != nil {
		return fmt.Errorf("creating wisp: %w", err)
	}

	// Parse wisp output to get the root ID
	var wispResult struct {
		RootID string `json:"root_id"`
	}
	if err := json.Unmarshal(wispOut, &wispResult); err != nil {
		// Fallback: use formula name as identifier, but warn user
		fmt.Printf("%s Could not parse wisp output, using formula name as ID\n", style.Dim.Render("Warning:"))
		wispResult.RootID = formulaName
	}

	fmt.Printf("%s Wisp created: %s\n", style.Bold.Render("✓"), wispResult.RootID)

	// Step 3: Attach to hook
	sw := wisp.NewSlungWork(wispResult.RootID, targetAgent)
	sw.Subject = slingSubject
	if sw.Subject == "" {
		sw.Subject = fmt.Sprintf("Formula: %s", formulaName)
	}
	sw.Context = slingMessage
	sw.Formula = formulaName

	if err := wisp.WriteSlungWork(hookRoot, targetAgent, sw); err != nil {
		return fmt.Errorf("writing to hook: %w", err)
	}
	fmt.Printf("%s Attached to hook\n", style.Bold.Render("✓"))

	// Step 4: Nudge to start
	if targetPane == "" {
		fmt.Printf("%s No pane to nudge (target may need manual start)\n", style.Dim.Render("○"))
		return nil
	}

	prompt := fmt.Sprintf("Formula %s slung. Run `gt mol status` to see your hook, then execute the steps.", formulaName)
	t := tmux.NewTmux()
	if err := t.NudgePane(targetPane, prompt); err != nil {
		return fmt.Errorf("nudging: %w", err)
	}
	fmt.Printf("%s Nudged to start\n", style.Bold.Render("▶"))

	return nil
}
