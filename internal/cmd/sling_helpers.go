package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/config"
	"github.com/steveyegge/gastown/internal/constants"
	"github.com/steveyegge/gastown/internal/events"
	"github.com/steveyegge/gastown/internal/git"
	"github.com/steveyegge/gastown/internal/polecat"
	"github.com/steveyegge/gastown/internal/rig"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/tmux"
	"github.com/steveyegge/gastown/internal/workspace"
)

// PrepareAndSpawnParams contains all parameters for the unified spawn workflow.
// This consolidates pre-spawn, spawn, and post-spawn work into a single call.
type PrepareAndSpawnParams struct {
	// Required
	TownRoot string // Town root directory
	RigName  string // Target rig for polecat spawn
	BeadID   string // Original bead (or wisp ID for formula sling)

	// Formula options
	FormulaName string   // Formula to apply ("" = mol-polecat-work, "none" = raw bead)
	FormulaVars []string // Additional formula variables (--var key=value)
	SkipFormula bool     // If true, skip formula application entirely (for formula sling)

	// Spawn options
	Force   bool   // Force spawn even if polecat has uncommitted work
	Account string // Claude Code account handle
	Create  bool   // Create polecat if it doesn't exist
	Agent   string // Agent override (e.g., "gemini", "codex")

	// Bead metadata (stored before spawn)
	Subject string // Subject for nudge prompt
	Args    string // Args for nudge prompt and bead storage

	// Convoy options
	NoConvoy  bool   // Skip auto-convoy creation
	BeadTitle string // Title for auto-convoy (required if !NoConvoy and !SkipFormula)

	// Post-spawn options
	WakeRig bool // Wake witness/refinery after spawn (default true for single, false for batch)
}

// PrepareAndSpawnResult contains the result of the unified spawn workflow.
type PrepareAndSpawnResult struct {
	BeadToHook   string // The bead that was hooked (base bead or wisp)
	WispID       string // The attached wisp ID (if formula applied)
	AgentID      string // The spawned polecat's agent ID (rig/polecats/name)
	PolecatName  string // Just the polecat name
	Pane         string // Tmux pane ID
	ClonePath    string // Path to polecat's git worktree
}

// PrepareAndSpawnPolecat is the unified spawn workflow for slinging beads to rigs.
// It handles all pre-spawn, spawn, and post-spawn work in the correct order:
//
// PRE-SPAWN (bead manipulation):
//  1. Apply formula (if needed) → get beadToHook, wispID
//  2. Store attached_molecule in beadToHook
//  3. Store dispatcher and args in bead
//  4. Create auto-convoy (unless NoConvoy or batch)
//  5. Pre-allocate polecat name → know agentID before spawn
//  6. Hook bead (bd update --status=hooked --assignee=agentID)
//
// SPAWN:
//  7. Spawn polecat with pre-allocated name and HookBead set
//     (agent bead hook_bead slot is set atomically during AddWithOptions)
//
// POST-SPAWN:
//  8. Log sling event to feed
//  9. Nudge polecat to start working
//  10. Wake rig agents (witness, refinery) if requested
func PrepareAndSpawnPolecat(p PrepareAndSpawnParams) (*PrepareAndSpawnResult, error) {
	result := &PrepareAndSpawnResult{}

	// ═══════════════════════════════════════════════════════════════════════════
	// PRE-SPAWN PHASE (bead manipulation)
	// ═══════════════════════════════════════════════════════════════════════════

	// Step 1: Apply formula (if needed)
	beadToHook := p.BeadID
	var wispID string
	beadTitle := p.BeadTitle

	if !p.SkipFormula {
		formulaName := p.FormulaName
		if formulaName == "" {
			formulaName = "mol-polecat-work"
		}
		if formulaName != "none" {
			// Get bead info for formula variables
			info, err := getBeadInfo(p.BeadID)
			if err != nil {
				return nil, fmt.Errorf("getting bead info: %w", err)
			}
			if beadTitle == "" {
				beadTitle = info.Title
			}

			// Cook formula
			if err := CookFormula(formulaName, p.TownRoot); err != nil {
				// Non-fatal - proceed with raw bead
				fmt.Printf("%s Warning: could not cook %s: %v\n", style.Dim.Render("○"), formulaName, err)
			} else {
				// Apply formula
				formulaResult, err := InstantiateFormulaOnBead(formulaName, p.BeadID, info.Title, "", p.TownRoot, true)
				if err != nil {
					// Non-fatal - proceed with raw bead
					fmt.Printf("%s Warning: could not apply %s: %v\n", style.Dim.Render("○"), formulaName, err)
				} else {
					beadToHook = formulaResult.BeadToHook
					wispID = formulaResult.WispRootID
				}
			}
		}
	}
	result.BeadToHook = beadToHook
	result.WispID = wispID

	// Step 2: Store attached_molecule in beadToHook (if wisp was created)
	if wispID != "" {
		if err := storeAttachedMoleculeInBead(beadToHook, wispID); err != nil {
			fmt.Printf("%s Warning: could not store attached_molecule: %v\n", style.Dim.Render("○"), err)
		}
	}

	// Step 3: Store dispatcher and args in bead (before spawn)
	actor := detectActor()
	_ = storeDispatcherInBead(beadToHook, actor)
	if p.Args != "" {
		_ = storeArgsInBead(beadToHook, p.Args)
	}

	// Step 4: Create auto-convoy (unless NoConvoy)
	if !p.NoConvoy && beadTitle != "" {
		existingConvoy := isTrackedByConvoy(p.BeadID)
		if existingConvoy == "" {
			convoyID, err := createAutoConvoy(p.BeadID, beadTitle)
			if err != nil {
				fmt.Printf("%s Could not create auto-convoy: %v\n", style.Dim.Render("Warning:"), err)
			} else {
				fmt.Printf("%s Created convoy 🚚 %s\n", style.Bold.Render("→"), convoyID)
			}
		} else {
			fmt.Printf("%s Already tracked by convoy %s\n", style.Dim.Render("○"), existingConvoy)
		}
	}

	// Step 5: Pre-allocate polecat name
	polecatName, err := allocatePolecatName(p.TownRoot, p.RigName)
	if err != nil {
		return nil, fmt.Errorf("allocating polecat name: %w", err)
	}
	agentID := fmt.Sprintf("%s/polecats/%s", p.RigName, polecatName)
	result.AgentID = agentID
	result.PolecatName = polecatName

	// Step 6: Hook bead BEFORE spawn
	hookWorkDir := beads.ResolveHookDir(p.TownRoot, beadToHook, "")
	hookCmd := exec.Command("bd", "--no-daemon", "update", beadToHook, "--status=hooked", "--assignee="+agentID)
	hookCmd.Dir = hookWorkDir
	if err := hookCmd.Run(); err != nil {
		return nil, fmt.Errorf("hooking bead before spawn: %w", err)
	}
	fmt.Printf("%s Bead hooked to %s\n", style.Bold.Render("✓"), agentID)

	// ═══════════════════════════════════════════════════════════════════════════
	// SPAWN PHASE
	// ═══════════════════════════════════════════════════════════════════════════

	// Step 7: Spawn polecat with pre-allocated name
	spawnOpts := SlingSpawnOptions{
		Force:    p.Force,
		Account:  p.Account,
		Create:   p.Create,
		Agent:    p.Agent,
		HookBead: beadToHook, // Also set in polecat state for consistency
	}
	spawnInfo, err := SpawnPolecatForSlingWithName(p.RigName, polecatName, spawnOpts)
	if err != nil {
		return nil, fmt.Errorf("spawning polecat: %w", err)
	}
	result.Pane = spawnInfo.Pane
	result.ClonePath = spawnInfo.ClonePath

	// ═══════════════════════════════════════════════════════════════════════════
	// POST-SPAWN PHASE
	// ═══════════════════════════════════════════════════════════════════════════
	// Note: hook_bead was already set atomically during agent bead creation in AddWithOptions.
	// No need to call updateAgentHookBead here.

	// Step 8: Log sling event
	_ = events.LogFeed(events.TypeSling, actor, events.SlingPayload(beadToHook, agentID))

	// Step 9: Nudge polecat
	if spawnInfo.Pane != "" {
		_ = injectStartPrompt(spawnInfo.Pane, beadToHook, p.Subject, p.Args)
	}

	// Step 10: Wake rig agents (if requested)
	if p.WakeRig {
		wakeRigAgents(p.RigName)
	}

	return result, nil
}

// PrepareAndSpawnPolecatWithName is a variant for batch operations where
// polecat names are pre-allocated upfront to avoid race conditions.
// The workflow is the same as PrepareAndSpawnPolecat but skips name allocation
// and convoy creation (batch creates convoy externally).
func PrepareAndSpawnPolecatWithName(p PrepareAndSpawnParams, polecatName string) (*PrepareAndSpawnResult, error) {
	result := &PrepareAndSpawnResult{}

	// ═══════════════════════════════════════════════════════════════════════════
	// PRE-SPAWN PHASE (formula + attached_molecule already handled by batch caller)
	// ═══════════════════════════════════════════════════════════════════════════

	beadToHook := p.BeadID // In batch mode, this is already the correct beadToHook
	agentID := fmt.Sprintf("%s/polecats/%s", p.RigName, polecatName)
	result.BeadToHook = beadToHook
	result.AgentID = agentID
	result.PolecatName = polecatName

	// Store dispatcher and args in bead (before spawn)
	actor := detectActor()
	_ = storeDispatcherInBead(beadToHook, actor)
	if p.Args != "" {
		_ = storeArgsInBead(beadToHook, p.Args)
	}

	// Note: Convoy creation handled by batch caller (creates batch convoy, not per-bead)

	// Hook bead BEFORE spawn
	hookWorkDir := beads.ResolveHookDir(p.TownRoot, beadToHook, "")
	hookCmd := exec.Command("bd", "--no-daemon", "update", beadToHook, "--status=hooked", "--assignee="+agentID)
	hookCmd.Dir = hookWorkDir
	if err := hookCmd.Run(); err != nil {
		return nil, fmt.Errorf("hooking bead before spawn: %w", err)
	}

	// ═══════════════════════════════════════════════════════════════════════════
	// SPAWN PHASE
	// ═══════════════════════════════════════════════════════════════════════════

	spawnOpts := SlingSpawnOptions{
		Force:    p.Force,
		Account:  p.Account,
		Create:   p.Create,
		Agent:    p.Agent,
		HookBead: beadToHook,
	}
	spawnInfo, err := SpawnPolecatForSlingWithName(p.RigName, polecatName, spawnOpts)
	if err != nil {
		return nil, fmt.Errorf("spawning polecat: %w", err)
	}
	result.Pane = spawnInfo.Pane
	result.ClonePath = spawnInfo.ClonePath

	// ═══════════════════════════════════════════════════════════════════════════
	// POST-SPAWN PHASE
	// ═══════════════════════════════════════════════════════════════════════════
	// Note: hook_bead was already set atomically during agent bead creation in AddWithOptions.
	// No need to call updateAgentHookBead here.

	// Log sling event
	_ = events.LogFeed(events.TypeSling, actor, events.SlingPayload(beadToHook, agentID))

	// Nudge polecat
	if spawnInfo.Pane != "" {
		_ = injectStartPrompt(spawnInfo.Pane, beadToHook, p.Subject, p.Args)
	}

	// Wake rig agents (if requested - usually false for batch, done once at end)
	if p.WakeRig {
		wakeRigAgents(p.RigName)
	}

	return result, nil
}

// allocatePolecatName allocates a single polecat name from the rig's name pool.
func allocatePolecatName(townRoot, rigName string) (string, error) {
	rigsConfigPath := filepath.Join(townRoot, "mayor", "rigs.json")
	rigsConfig, err := config.LoadRigsConfig(rigsConfigPath)
	if err != nil {
		rigsConfig = &config.RigsConfig{Rigs: make(map[string]config.RigEntry)}
	}

	g := git.NewGit(townRoot)
	rigMgr := rig.NewManager(townRoot, rigsConfig, g)
	r, err := rigMgr.GetRig(rigName)
	if err != nil {
		return "", fmt.Errorf("rig '%s' not found: %w", rigName, err)
	}

	polecatGit := git.NewGit(r.Path)
	t := tmux.NewTmux()
	polecatMgr := polecat.NewManager(r, polecatGit, t)

	return polecatMgr.AllocateName()
}

// beadInfo holds status and assignee for a bead.
type beadInfo struct {
	Title    string `json:"title"`
	Status   string `json:"status"`
	Assignee string `json:"assignee"`
}

// verifyBeadExists checks that the bead exists using bd show.
// Uses bd's native prefix-based routing via routes.jsonl - do NOT set BEADS_DIR
// as that overrides routing and breaks resolution of rig-level beads.
//
// Uses --no-daemon with --allow-stale to avoid daemon socket timing issues
// while still finding beads when database is out of sync with JSONL.
// For existence checks, stale data is acceptable - we just need to know it exists.
func verifyBeadExists(beadID string) error {
	cmd := exec.Command("bd", "--no-daemon", "show", beadID, "--json", "--allow-stale")
	// Run from town root so bd can find routes.jsonl for prefix-based routing.
	// Do NOT set BEADS_DIR - that overrides routing and breaks rig bead resolution.
	if townRoot, err := workspace.FindFromCwd(); err == nil {
		cmd.Dir = townRoot
	}
	// Use Output() instead of Run() to detect bd --no-daemon exit 0 bug:
	// when issue not found, --no-daemon exits 0 but produces empty stdout.
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("bead '%s' not found (bd show failed)", beadID)
	}
	if len(out) == 0 {
		return fmt.Errorf("bead '%s' not found", beadID)
	}
	return nil
}

// getBeadInfo returns status and assignee for a bead.
// Uses bd's native prefix-based routing via routes.jsonl.
// Uses --no-daemon with --allow-stale for consistency with verifyBeadExists.
func getBeadInfo(beadID string) (*beadInfo, error) {
	cmd := exec.Command("bd", "--no-daemon", "show", beadID, "--json", "--allow-stale")
	// Run from town root so bd can find routes.jsonl for prefix-based routing.
	if townRoot, err := workspace.FindFromCwd(); err == nil {
		cmd.Dir = townRoot
	}
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("bead '%s' not found", beadID)
	}
	// Handle bd --no-daemon exit 0 bug: when issue not found,
	// --no-daemon exits 0 but produces empty stdout (error goes to stderr).
	if len(out) == 0 {
		return nil, fmt.Errorf("bead '%s' not found", beadID)
	}
	// bd show --json returns an array (issue + dependents), take first element
	var infos []beadInfo
	if err := json.Unmarshal(out, &infos); err != nil {
		return nil, fmt.Errorf("parsing bead info: %w", err)
	}
	if len(infos) == 0 {
		return nil, fmt.Errorf("bead '%s' not found", beadID)
	}
	return &infos[0], nil
}

// storeArgsInBead stores args in the bead's description using attached_args field.
// This enables no-tmux mode where agents discover args via gt prime / bd show.
func storeArgsInBead(beadID, args string) error {
	// Get the bead to preserve existing description content
	showCmd := exec.Command("bd", "--no-daemon", "show", beadID, "--json", "--allow-stale")
	out, err := showCmd.Output()
	if err != nil {
		return fmt.Errorf("fetching bead: %w", err)
	}
	// Handle bd --no-daemon exit 0 bug: empty stdout means not found
	if len(out) == 0 {
		return fmt.Errorf("bead not found")
	}

	// Parse the bead
	var issues []beads.Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		if os.Getenv("GT_TEST_ATTACHED_MOLECULE_LOG") == "" {
			return fmt.Errorf("parsing bead: %w", err)
		}
	}
	issue := &beads.Issue{}
	if len(issues) > 0 {
		issue = &issues[0]
	} else if os.Getenv("GT_TEST_ATTACHED_MOLECULE_LOG") == "" {
		return fmt.Errorf("bead not found")
	}

	// Get or create attachment fields
	fields := beads.ParseAttachmentFields(issue)
	if fields == nil {
		fields = &beads.AttachmentFields{}
	}

	// Set the args
	fields.AttachedArgs = args

	// Update the description
	newDesc := beads.SetAttachmentFields(issue, fields)
	if logPath := os.Getenv("GT_TEST_ATTACHED_MOLECULE_LOG"); logPath != "" {
		_ = os.WriteFile(logPath, []byte(newDesc), 0644)
	}

	// Update the bead
	updateCmd := exec.Command("bd", "--no-daemon", "update", beadID, "--description="+newDesc)
	updateCmd.Stderr = os.Stderr
	if err := updateCmd.Run(); err != nil {
		return fmt.Errorf("updating bead description: %w", err)
	}

	return nil
}

// storeDispatcherInBead stores the dispatcher agent ID in the bead's description.
// This enables polecats to notify the dispatcher when work is complete.
func storeDispatcherInBead(beadID, dispatcher string) error {
	if dispatcher == "" {
		return nil
	}

	// Get the bead to preserve existing description content
	showCmd := exec.Command("bd", "show", beadID, "--json")
	out, err := showCmd.Output()
	if err != nil {
		return fmt.Errorf("fetching bead: %w", err)
	}

	// Parse the bead
	var issues []beads.Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		return fmt.Errorf("parsing bead: %w", err)
	}
	if len(issues) == 0 {
		return fmt.Errorf("bead not found")
	}
	issue := &issues[0]

	// Get or create attachment fields
	fields := beads.ParseAttachmentFields(issue)
	if fields == nil {
		fields = &beads.AttachmentFields{}
	}

	// Set the dispatcher
	fields.DispatchedBy = dispatcher

	// Update the description
	newDesc := beads.SetAttachmentFields(issue, fields)

	// Update the bead
	updateCmd := exec.Command("bd", "update", beadID, "--description="+newDesc)
	updateCmd.Stderr = os.Stderr
	if err := updateCmd.Run(); err != nil {
		return fmt.Errorf("updating bead description: %w", err)
	}

	return nil
}

// storeAttachedMoleculeInBead sets the attached_molecule field in a bead's description.
// This is required for gt hook to recognize that a molecule is attached to the bead.
// Called after bonding a formula wisp to a bead via "gt sling <formula> --on <bead>".
func storeAttachedMoleculeInBead(beadID, moleculeID string) error {
	if moleculeID == "" {
		return nil
	}
	logPath := os.Getenv("GT_TEST_ATTACHED_MOLECULE_LOG")
	if logPath != "" {
		_ = os.WriteFile(logPath, []byte("called"), 0644)
	}

	issue := &beads.Issue{}
	if logPath == "" {
		// Get the bead to preserve existing description content
		showCmd := exec.Command("bd", "show", beadID, "--json")
		out, err := showCmd.Output()
		if err != nil {
			return fmt.Errorf("fetching bead: %w", err)
		}

		// Parse the bead
		var issues []beads.Issue
		if err := json.Unmarshal(out, &issues); err != nil {
			return fmt.Errorf("parsing bead: %w", err)
		}
		if len(issues) == 0 {
			return fmt.Errorf("bead not found")
		}
		issue = &issues[0]
	}

	// Get or create attachment fields
	fields := beads.ParseAttachmentFields(issue)
	if fields == nil {
		fields = &beads.AttachmentFields{}
	}

	// Set the attached molecule
	fields.AttachedMolecule = moleculeID
	if fields.AttachedAt == "" {
		fields.AttachedAt = time.Now().UTC().Format(time.RFC3339)
	}

	// Update the description
	newDesc := beads.SetAttachmentFields(issue, fields)
	if logPath != "" {
		_ = os.WriteFile(logPath, []byte(newDesc), 0644)
	}

	// Update the bead
	updateCmd := exec.Command("bd", "update", beadID, "--description="+newDesc)
	updateCmd.Stderr = os.Stderr
	if err := updateCmd.Run(); err != nil {
		return fmt.Errorf("updating bead description: %w", err)
	}

	return nil
}

// injectStartPrompt sends a prompt to the target pane to start working.
// Uses the reliable nudge pattern: literal mode + 500ms debounce + separate Enter.
func injectStartPrompt(pane, beadID, subject, args string) error {
	if pane == "" {
		return fmt.Errorf("no target pane")
	}

	// Skip nudge during tests to prevent agent self-interruption
	if os.Getenv("GT_TEST_NO_NUDGE") != "" {
		return nil
	}

	// Build the prompt to inject
	var prompt string
	if args != "" {
		// Args provided - include them prominently in the prompt
		if subject != "" {
			prompt = fmt.Sprintf("Work slung: %s (%s). Args: %s. Start working now - use these args to guide your execution.", beadID, subject, args)
		} else {
			prompt = fmt.Sprintf("Work slung: %s. Args: %s. Start working now - use these args to guide your execution.", beadID, args)
		}
	} else if subject != "" {
		prompt = fmt.Sprintf("Work slung: %s (%s). Start working on it now - no questions, just begin.", beadID, subject)
	} else {
		prompt = fmt.Sprintf("Work slung: %s. Start working on it now - run `gt hook` to see the hook, then begin.", beadID)
	}

	// Use the reliable nudge pattern (same as gt nudge / tmux.NudgeSession)
	t := tmux.NewTmux()
	return t.NudgePane(pane, prompt)
}

// getSessionFromPane extracts session name from a pane target.
// Pane targets can be:
// - "%9" (pane ID) - need to query tmux for session
// - "gt-rig-name:0.0" (session:window.pane) - extract session name
func getSessionFromPane(pane string) string {
	if strings.HasPrefix(pane, "%") {
		// Pane ID format - query tmux for the session
		cmd := exec.Command("tmux", "display-message", "-t", pane, "-p", "#{session_name}")
		out, err := cmd.Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
	// Session:window.pane format - extract session name
	if idx := strings.Index(pane, ":"); idx > 0 {
		return pane[:idx]
	}
	return pane
}

// ensureAgentReady waits for an agent to be ready before nudging an existing session.
// Uses a pragmatic approach: wait for the pane to leave a shell, then (Claude-only)
// accept the bypass permissions warning and give it a moment to finish initializing.
func ensureAgentReady(sessionName string) error {
	t := tmux.NewTmux()

	// If an agent is already running, assume it's ready (session was started earlier)
	if t.IsAgentRunning(sessionName) {
		return nil
	}

	// Agent not running yet - wait for it to start (shell → program transition)
	if err := t.WaitForCommand(sessionName, constants.SupportedShells, constants.ClaudeStartTimeout); err != nil {
		return fmt.Errorf("waiting for agent to start: %w", err)
	}

	// Claude-only: accept bypass permissions warning if present
	if t.IsClaudeRunning(sessionName) {
		_ = t.AcceptBypassPermissionsWarning(sessionName)

		// PRAGMATIC APPROACH: fixed delay rather than prompt detection.
		// Claude startup takes ~5-8 seconds on typical machines.
		time.Sleep(8 * time.Second)
	} else {
		time.Sleep(1 * time.Second)
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

// detectActor returns the current agent's actor string for event logging.
func detectActor() string {
	roleInfo, err := GetRole()
	if err != nil {
		return "unknown"
	}
	return roleInfo.ActorString()
}

// agentIDToBeadID converts an agent ID to its corresponding agent bead ID.
// Uses canonical naming: prefix-rig-role-name
// Town-level agents (Mayor, Deacon) use hq- prefix and are stored in town beads.
// Rig-level agents use the rig's configured prefix (default "gt-").
// townRoot is needed to look up the rig's configured prefix.
func agentIDToBeadID(agentID, townRoot string) string {
	// Normalize: strip trailing slash (resolveSelfTarget returns "mayor/" not "mayor")
	agentID = strings.TrimSuffix(agentID, "/")

	// Handle simple cases (town-level agents with hq- prefix)
	if agentID == "mayor" {
		return beads.MayorBeadIDTown()
	}
	if agentID == "deacon" {
		return beads.DeaconBeadIDTown()
	}

	// Parse path-style agent IDs
	parts := strings.Split(agentID, "/")
	if len(parts) < 2 {
		return ""
	}

	rig := parts[0]
	prefix := beads.GetPrefixForRig(townRoot, rig)

	switch {
	case len(parts) == 2 && parts[1] == "witness":
		return beads.WitnessBeadIDWithPrefix(prefix, rig)
	case len(parts) == 2 && parts[1] == "refinery":
		return beads.RefineryBeadIDWithPrefix(prefix, rig)
	case len(parts) == 3 && parts[1] == "crew":
		return beads.CrewBeadIDWithPrefix(prefix, rig, parts[2])
	case len(parts) == 3 && parts[1] == "polecats":
		return beads.PolecatBeadIDWithPrefix(prefix, rig, parts[2])
	default:
		return ""
	}
}

// updateAgentHookBead updates the agent bead's state and hook when work is slung.
// This enables the witness to see that each agent is working.
//
// We run from the polecat's workDir (which redirects to the rig's beads database)
// WITHOUT setting BEADS_DIR, so the redirect mechanism works for gt-* agent beads.
//
// For rig-level beads (same database), we set the hook_bead slot directly.
// For cross-database scenarios (agent in rig db, hook bead in town db),
// the slot set may fail - this is handled gracefully with a warning.
// The work is still correctly attached via `bd update <bead> --assignee=<agent>`.
func updateAgentHookBead(agentID, beadID, workDir, townBeadsDir string) {
	_ = townBeadsDir // Not used - BEADS_DIR breaks redirect mechanism

	// Determine the directory to run bd commands from:
	// - If workDir is provided (polecat's clone path), use it for redirect-based routing
	// - Otherwise fall back to town root
	bdWorkDir := workDir
	townRoot, err := workspace.FindFromCwd()
	if err != nil {
		// Not in a Gas Town workspace - can't update agent bead
		fmt.Fprintf(os.Stderr, "Warning: couldn't find town root to update agent hook: %v\n", err)
		return
	}
	if bdWorkDir == "" {
		bdWorkDir = townRoot
	}

	// Convert agent ID to agent bead ID
	// Format examples (canonical: prefix-rig-role-name):
	//   greenplace/crew/max -> gt-greenplace-crew-max
	//   greenplace/polecats/Toast -> gt-greenplace-polecat-Toast
	//   mayor -> hq-mayor
	//   greenplace/witness -> gt-greenplace-witness
	agentBeadID := agentIDToBeadID(agentID, townRoot)
	if agentBeadID == "" {
		return
	}

	// Resolve the correct working directory for the agent bead.
	// Agent beads with rig-level prefixes (e.g., go-) live in rig databases,
	// not the town database. Use prefix-based resolution to find the correct path.
	// This fixes go-19z: bd slot commands failing for go-* prefixed beads.
	agentWorkDir := beads.ResolveHookDir(townRoot, agentBeadID, bdWorkDir)

	// Run from agentWorkDir WITHOUT BEADS_DIR to enable redirect-based routing.
	// Set hook_bead to the slung work (gt-zecmc: removed agent_state update).
	// Agent liveness is observable from tmux - no need to record it in bead.
	// For cross-database scenarios, slot set may fail gracefully (warning only).
	bd := beads.New(agentWorkDir)
	if err := bd.SetHookBead(agentBeadID, beadID); err != nil {
		// Log warning instead of silent ignore - helps debug cross-beads issues
		fmt.Fprintf(os.Stderr, "Warning: couldn't set agent %s hook: %v\n", agentBeadID, err)
		return
	}
}

// wakeRigAgents wakes the witness and refinery for a rig after polecat dispatch.
// This ensures the patrol agents are ready to monitor and merge.
func wakeRigAgents(rigName string) {
	// Boot the rig (idempotent - no-op if already running)
	bootCmd := exec.Command("gt", "rig", "boot", rigName)
	_ = bootCmd.Run() // Ignore errors - rig might already be running

	// Nudge witness and refinery to clear any backoff
	t := tmux.NewTmux()
	witnessSession := fmt.Sprintf("gt-%s-witness", rigName)
	refinerySession := fmt.Sprintf("gt-%s-refinery", rigName)

	// Silent nudges - sessions might not exist yet
	_ = t.NudgeSession(witnessSession, "Polecat dispatched - check for work")
	_ = t.NudgeSession(refinerySession, "Polecat dispatched - check for merge requests")
}

// isPolecatTarget checks if the target string refers to a polecat.
// Returns true if the target format is "rig/polecats/name".
// This is used to determine if we should respawn a dead polecat
// instead of failing when slinging work.
func isPolecatTarget(target string) bool {
	parts := strings.Split(target, "/")
	return len(parts) >= 3 && parts[1] == "polecats"
}

// FormulaOnBeadResult contains the result of instantiating a formula on a bead.
type FormulaOnBeadResult struct {
	WispRootID string // The wisp root ID (guidance molecule)
	BeadToHook string // The bead ID to hook (BASE bead, not wisp)
}

// InstantiateFormulaOnBead creates a wisp from a formula to guide work on a bead.
// This is the formula-on-bead pattern used by issue #288 for auto-applying mol-polecat-work.
//
// The wisp provides GUIDANCE for working on the base bead, not a prerequisite.
// We intentionally do NOT use `bd mol bond` because bonding creates a blocking
// dependency that prevents the base bead from appearing in `bd ready`.
//
// Instead, the caller stores the wisp ID as `attached_molecule` in the base bead's
// description. This allows:
//   - Base bead appears in `bd ready` (respecting other real dependencies)
//   - gt prime reads wisp steps from attached_molecule
//   - gt done closes attached_molecule (wisp) before closing base bead
//
// Parameters:
//   - formulaName: the formula to instantiate (e.g., "mol-polecat-work")
//   - beadID: the base bead to attach the wisp to
//   - title: the bead title (used for --var feature=<title>)
//   - hookWorkDir: working directory for bd commands (polecat's worktree)
//   - townRoot: the town root directory
//   - skipCook: if true, skip cooking (for batch mode optimization where cook happens once)
//
// Returns BeadToHook (base bead) and WispRootID (guidance molecule).
// Caller should store WispRootID as attached_molecule in the base bead.
func InstantiateFormulaOnBead(formulaName, beadID, title, hookWorkDir, townRoot string, skipCook bool) (*FormulaOnBeadResult, error) {
	// Route bd mutations to the correct beads context for the target bead.
	formulaWorkDir := beads.ResolveHookDir(townRoot, beadID, hookWorkDir)

	// Step 1: Cook the formula (ensures proto exists)
	if !skipCook {
		cookCmd := exec.Command("bd", "--no-daemon", "cook", formulaName)
		cookCmd.Dir = formulaWorkDir
		cookCmd.Stderr = os.Stderr
		if err := cookCmd.Run(); err != nil {
			return nil, fmt.Errorf("cooking formula %s: %w", formulaName, err)
		}
	}

	// Step 2: Create wisp with feature and issue variables from bead
	featureVar := fmt.Sprintf("feature=%s", title)
	issueVar := fmt.Sprintf("issue=%s", beadID)
	wispArgs := []string{"--no-daemon", "mol", "wisp", formulaName, "--var", featureVar, "--var", issueVar, "--json"}
	wispCmd := exec.Command("bd", wispArgs...)
	wispCmd.Dir = formulaWorkDir
	wispCmd.Env = append(os.Environ(), "GT_ROOT="+townRoot)
	wispCmd.Stderr = os.Stderr
	wispOut, err := wispCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("creating wisp for formula %s: %w", formulaName, err)
	}

	// Parse wisp output to get the root ID
	wispRootID, err := parseWispIDFromJSON(wispOut)
	if err != nil {
		return nil, fmt.Errorf("parsing wisp output: %w", err)
	}

	// NOTE: We do NOT bond the wisp to the bead here.
	// Bonding creates a blocking dependency that prevents the base bead from
	// appearing in `bd ready`. Instead, the caller stores the wisp ID as
	// `attached_molecule` in the base bead's description field.

	return &FormulaOnBeadResult{
		WispRootID: wispRootID,
		BeadToHook: beadID,
	}, nil
}

// CookFormula cooks a formula to ensure its proto exists.
// This is useful for batch mode where we cook once before processing multiple beads.
func CookFormula(formulaName, workDir string) error {
	cookCmd := exec.Command("bd", "--no-daemon", "cook", formulaName)
	cookCmd.Dir = workDir
	cookCmd.Stderr = os.Stderr
	return cookCmd.Run()
}
