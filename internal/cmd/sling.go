package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/events"
	"github.com/steveyegge/gastown/internal/queue"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/workspace"
)

var slingCmd = &cobra.Command{
	Use:     "sling <bead-or-formula> [target]",
	GroupID: GroupWork,
	Short:   "Assign work to an agent (THE unified work dispatch command)",
	Long: `Sling work onto an agent's hook and start working immediately.

This is THE command for assigning work in Gas Town. It handles:
  - Existing agents (mayor, crew, witness, refinery)
  - Auto-spawning polecats when target is a rig
  - Dispatching to dogs (Deacon's helper workers)
  - Formula instantiation and wisp creation
  - Auto-convoy creation for dashboard visibility

Auto-Convoy:
  When slinging a single issue (not a formula), sling automatically creates
  a convoy to track the work unless --no-convoy is specified. This ensures
  all work appears in 'gt convoy list', even "swarm of one" assignments.

  gt sling gt-abc gastown              # Creates "Work: <issue-title>" convoy
  gt sling gt-abc gastown --no-convoy  # Skip auto-convoy creation

Target Resolution:
  gt sling gt-abc                       # Self (current agent)
  gt sling gt-abc crew                  # Crew worker in current rig
  gt sling gp-abc greenplace               # Auto-spawn polecat in rig
  gt sling gt-abc greenplace/Toast         # Specific polecat
  gt sling gt-abc mayor                 # Mayor
  gt sling gt-abc deacon/dogs           # Auto-dispatch to idle dog
  gt sling gt-abc deacon/dogs/alpha     # Specific dog

Spawning Options (when target is a rig):
  gt sling gp-abc greenplace --create               # Create polecat if missing
  gt sling gp-abc greenplace --force                # Ignore unread mail
  gt sling gp-abc greenplace --account work         # Use specific Claude account

Natural Language Args:
  gt sling gt-abc --args "patch release"
  gt sling code-review --args "focus on security"

The --args string is stored in the bead and shown via gt prime. Since the
executor is an LLM, it interprets these instructions naturally.

Formula Slinging:
  gt sling mol-release mayor/           # Cook + wisp + attach + nudge
  gt sling towers-of-hanoi --var disks=3

Formula-on-Bead (--on flag):
  gt sling mol-review --on gt-abc       # Apply formula to existing work
  gt sling shiny --on gt-abc crew       # Apply formula, sling to crew

Compare:
  gt hook <bead>      # Just attach (no action)
  gt sling <bead>     # Attach + start now (keep context)
  gt handoff <bead>   # Attach + restart (fresh context)

The propulsion principle: if it's on your hook, YOU RUN IT.

Batch Slinging:
  gt sling gt-abc gt-def gt-ghi gastown   # Sling multiple beads to a rig

  When multiple beads are provided with a rig target, each bead gets its own
  polecat. This parallelizes work dispatch without running gt sling N times.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runSling,
}

var (
	slingSubject  string
	slingMessage  string
	slingDryRun   bool
	slingOnTarget string   // --on flag: target bead when slinging a formula
	slingVars     []string // --var flag: formula variables (key=value)
	slingArgs     string   // --args flag: natural language instructions for executor

	// Flags migrated for polecat spawning (used by sling for work assignment)
	slingCreate   bool   // --create: create polecat if it doesn't exist
	slingForce    bool   // --force: force spawn even if polecat has unread mail
	slingAccount  string // --account: Claude Code account handle to use
	slingAgent    string // --agent: override runtime agent for this sling/spawn
	slingNoConvoy bool   // --no-convoy: skip auto-convoy creation
	slingParallel int    // --parallel: batch parallelism (default 5, use 1 for sequential)
	slingCapacity int    // --capacity: max total polecats running (0 = unlimited)
	slingQueue    bool   // --queue: add to queue and dispatch (opt-in queue workflow)
)

func init() {
	slingCmd.Flags().StringVarP(&slingSubject, "subject", "s", "", "Context subject for the work")
	slingCmd.Flags().StringVarP(&slingMessage, "message", "m", "", "Context message for the work")
	slingCmd.Flags().BoolVarP(&slingDryRun, "dry-run", "n", false, "Show what would be done")
	slingCmd.Flags().StringVar(&slingOnTarget, "on", "", "Apply formula to existing bead (implies wisp scaffolding)")
	slingCmd.Flags().StringArrayVar(&slingVars, "var", nil, "Formula variable (key=value), can be repeated")
	slingCmd.Flags().StringVarP(&slingArgs, "args", "a", "", "Natural language instructions for the executor (e.g., 'patch release')")

	// Flags for polecat spawning (when target is a rig)
	slingCmd.Flags().BoolVar(&slingCreate, "create", false, "Create polecat if it doesn't exist")
	slingCmd.Flags().BoolVar(&slingForce, "force", false, "Force spawn even if polecat has unread mail")
	slingCmd.Flags().StringVar(&slingAccount, "account", "", "Claude Code account handle to use")
	slingCmd.Flags().StringVar(&slingAgent, "agent", "", "Override agent/runtime for this sling (e.g., claude, gemini, codex, or custom alias)")
	slingCmd.Flags().BoolVar(&slingNoConvoy, "no-convoy", false, "Skip auto-convoy creation for single-issue sling")
	slingCmd.Flags().IntVar(&slingParallel, "parallel", 5, "Batch parallelism (number of concurrent polecats, use 1 for sequential)")
	slingCmd.Flags().IntVar(&slingCapacity, "capacity", 0, "Max total polecats running (0 = unlimited, use with --queue)")
	slingCmd.Flags().BoolVar(&slingQueue, "queue", false, "Add to queue and dispatch (opt-in queue workflow)")

	rootCmd.AddCommand(slingCmd)
}

func runSling(cmd *cobra.Command, args []string) error {
	// Polecats cannot sling - check early before writing anything
	if polecatName := os.Getenv("GT_POLECAT"); polecatName != "" {
		return fmt.Errorf("polecats cannot sling (use gt done for handoff)")
	}

	// Get town root early - needed for BEADS_DIR when running bd commands
	// This ensures hq-* beads are accessible even when running from polecat worktree
	townRoot, err := workspace.FindFromCwd()
	if err != nil {
		return fmt.Errorf("finding town root: %w", err)
	}
	townBeadsDir := filepath.Join(townRoot, ".beads")

	// --var is only for standalone formula mode, not formula-on-bead mode
	if slingOnTarget != "" && len(slingVars) > 0 {
		return fmt.Errorf("--var cannot be used with --on (formula-on-bead mode doesn't support variables)")
	}

	// Batch formula-on-bead mode detection:
	// Pattern: gt sling <formula> --on gt-abc,gt-def,gt-ghi <rig>
	// Pattern: gt sling <formula> --on @beads.txt <rig>
	// When --on contains multiple beads (comma-separated or @file) and target is a rig
	if slingOnTarget != "" && len(args) >= 2 {
		beadIDs := parseBatchOnTarget(slingOnTarget)
		if len(beadIDs) > 1 {
			lastArg := args[len(args)-1]
			if rigName, isRig := IsRigName(lastArg); isRig {
				return runBatchSlingFormulaOn(args[0], beadIDs, rigName, townBeadsDir)
			}
			return fmt.Errorf("batch --on mode requires a rig target (got %q)", lastArg)
		}
	}

	// Batch mode detection: multiple beads with rig target
	// Pattern: gt sling gt-abc gt-def gt-ghi gastown
	// When len(args) > 2 and last arg is a rig, sling each bead to its own polecat
	if len(args) > 2 {
		lastArg := args[len(args)-1]
		if rigName, isRig := IsRigName(lastArg); isRig {
			return runBatchSling(args[:len(args)-1], rigName, townBeadsDir)
		}
	}

	// Determine mode based on flags and argument types
	var beadID string
	var formulaName string
	attachedMoleculeID := ""

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
			// It's a verified bead
			beadID = firstArg
		} else {
			// Not a verified bead - try as standalone formula
			if err := verifyFormulaExists(firstArg); err == nil {
				// Standalone formula mode: gt sling <formula> [target]
				return runSlingFormula(args)
			}
			// Not a formula either - check if it looks like a bead ID (routing issue workaround).
			// Accept it and let the actual bd update fail later if the bead doesn't exist.
			// This fixes: gt sling bd-ka761 beads/crew/dave failing with 'not a valid bead or formula'
			if looksLikeBeadID(firstArg) {
				beadID = firstArg
			} else {
				// Neither bead nor formula
				return fmt.Errorf("'%s' is not a valid bead or formula", firstArg)
			}
		}
	}

	// Determine target agent (self or specified)
	var targetAgent string
	var hookWorkDir string // Clone path for polecat spawns (used for molecule attachment)
	var isRigTarget bool   // True when target is a rig name

	if len(args) > 1 {
		target := args[1]

		// Resolve "." to current agent identity (like git's "." meaning current directory)
		if target == "." {
			targetAgent, err = resolveSelfTarget()
			if err != nil {
				return fmt.Errorf("resolving self for '.' target: %w", err)
			}
		} else if dogName, isDog := IsDogTarget(target); isDog {
			if slingDryRun {
				if dogName == "" {
					fmt.Printf("Would dispatch to idle dog in kennel\n")
				} else {
					fmt.Printf("Would dispatch to dog '%s'\n", dogName)
				}
				targetAgent = fmt.Sprintf("deacon/dogs/%s", dogName)
				if dogName == "" {
					targetAgent = "deacon/dogs/<idle>"
				}
				// Dogs are goroutines, not tmux sessions
			} else {
				// Dispatch to dog
				dispatchInfo, dispatchErr := DispatchToDog(dogName, slingCreate)
				if dispatchErr != nil {
					return fmt.Errorf("dispatching to dog: %w", dispatchErr)
				}
				targetAgent = dispatchInfo.AgentID
				fmt.Printf("Dispatched to dog %s\n", dispatchInfo.DogName)
			}
		} else if rigName, isRig := IsRigName(target); isRig {
			// Check if target is a rig name (auto-spawn polecat)
			isRigTarget = true
			if slingDryRun {
				// Dry run - just indicate what would happen
				fmt.Printf("Would spawn fresh polecat in rig '%s'\n", rigName)
			targetAgent = fmt.Sprintf("%s/polecats/<new>", rigName)
		} else if slingQueue && formulaName == "" {
			// --queue mode for plain bead (not formula-on-bead)
			fmt.Printf("Target is rig '%s', queueing and dispatching...\n", rigName)

			// Check bead status before queueing
			info, err := getBeadInfo(beadID)
			if err != nil {
				return fmt.Errorf("checking bead status: %w", err)
			}
			if (info.Status == "pinned" || info.Status == "hooked") && !slingForce {
				return fmt.Errorf("bead %s is already %s to %s\nUse --force to re-sling", beadID, info.Status, info.Assignee)
			}

			// Auto-convoy before dispatch
			if !slingNoConvoy && isTrackedByConvoy(beadID) == "" {
				convoyID, convoyErr := createAutoConvoy(beadID, info.Title)
				if convoyErr != nil {
					fmt.Printf("%s Could not create auto-convoy: %v\n", style.Dim.Render("Warning:"), convoyErr)
				} else {
					fmt.Printf("%s Created convoy 🚚 %s\n", style.Bold.Render("→"), convoyID)
				}
			}

			// Create queue with town-wide ops
			ops := beads.NewRealBeadsOps(townRoot)
			q := queue.New(ops)

			// Add bead to queue
			if err := q.Add(beadID); err != nil {
				return fmt.Errorf("adding bead to queue: %w", err)
			}
			fmt.Printf("%s Bead queued\n", style.Bold.Render("✓"))

			// Create spawner using SpawnAndHookBead
			spawner := &queue.RealSpawner{
				SpawnInFunc: func(spawnRigName, bid string) error {
					_, err := SpawnAndHookBead(townRoot, spawnRigName, bid, SpawnAndHookOptions{
						Force:    slingForce,
						Account:  slingAccount,
						Create:   slingCreate,
						Agent:    slingAgent,
						Subject:  slingSubject,
						Args:     slingArgs,
						LogEvent: true,
					})
					return err
				},
			}
			dispatcher := queue.NewDispatcher(q, spawner)

			// Load and dispatch
			if _, err := q.Load(); err != nil {
				return fmt.Errorf("loading queue: %w", err)
			}
			if _, err := dispatcher.Dispatch(); err != nil {
				return fmt.Errorf("dispatching from queue: %w", err)
			}

			// Wake witness and refinery
			wakeRigAgents(townRoot, rigName)

			// SpawnAndHookBead did everything - return early
			return nil
		} else if slingQueue && formulaName != "" {
			// --queue mode for formula-on-bead
			fmt.Printf("Target is rig '%s', instantiating formula and queueing...\n", rigName)

			// Check bead status before queueing
			info, err := getBeadInfo(beadID)
			if err != nil {
				return fmt.Errorf("checking bead status: %w", err)
			}
			if (info.Status == "pinned" || info.Status == "hooked") && !slingForce {
				return fmt.Errorf("bead %s is already %s to %s\nUse --force to re-sling", beadID, info.Status, info.Assignee)
			}

			// Auto-convoy before formula instantiation
			if !slingNoConvoy && isTrackedByConvoy(beadID) == "" {
				convoyID, convoyErr := createAutoConvoy(beadID, info.Title)
				if convoyErr != nil {
					fmt.Printf("%s Could not create auto-convoy: %v\n", style.Dim.Render("Warning:"), convoyErr)
				} else {
					fmt.Printf("%s Created convoy 🚚 %s\n", style.Bold.Render("→"), convoyID)
				}
			}

			// Formula instantiation -> compound ID (must happen before queue)
			compoundID, err := InstantiateFormulaOnBead(townRoot, formulaName, beadID, info.Title)
			if err != nil {
				return err
			}

			// Create queue with town-wide ops
			ops := beads.NewRealBeadsOps(townRoot)
			q := queue.New(ops)

			// Add compound to queue (not the original bead)
			if err := q.Add(compoundID); err != nil {
				return fmt.Errorf("adding compound to queue: %w", err)
			}
			fmt.Printf("%s Compound queued\n", style.Bold.Render("✓"))

			// Create spawner using SpawnAndHookBead
			spawner := &queue.RealSpawner{
				SpawnInFunc: func(spawnRigName, bid string) error {
					_, err := SpawnAndHookBead(townRoot, spawnRigName, bid, SpawnAndHookOptions{
						Force:    slingForce,
						Account:  slingAccount,
						Create:   slingCreate,
						Agent:    slingAgent,
						Subject:  slingSubject,
						Args:     slingArgs,
						LogEvent: true,
					})
					return err
				},
			}
			dispatcher := queue.NewDispatcher(q, spawner)

			// Load and dispatch
			if _, err := q.Load(); err != nil {
				return fmt.Errorf("loading queue: %w", err)
			}
			if _, err := dispatcher.Dispatch(); err != nil {
				return fmt.Errorf("dispatching from queue: %w", err)
			}

			// Wake witness and refinery
			wakeRigAgents(townRoot, rigName)

			// SpawnAndHookBead did everything - return early
			return nil
		} else if formulaName == "" {
				// Non-queue plain bead to rig - use SpawnAndHookBead
				fmt.Printf("Target is rig '%s', spawning fresh polecat...\n", rigName)

				// Check bead status before spawning
				info, err := getBeadInfo(beadID)
				if err != nil {
					return fmt.Errorf("checking bead status: %w", err)
				}
				if (info.Status == "pinned" || info.Status == "hooked") && !slingForce {
					return fmt.Errorf("bead %s is already %s to %s\nUse --force to re-sling", beadID, info.Status, info.Assignee)
				}

				// Auto-convoy before spawn
				if !slingNoConvoy && isTrackedByConvoy(beadID) == "" {
					convoyID, convoyErr := createAutoConvoy(beadID, info.Title)
					if convoyErr != nil {
						fmt.Printf("%s Could not create auto-convoy: %v\n", style.Dim.Render("Warning:"), convoyErr)
					} else {
						fmt.Printf("%s Created convoy 🚚 %s\n", style.Bold.Render("→"), convoyID)
					}
				}

				// SpawnAndHookBead does spawn + hook + nudge
				_, spawnErr := SpawnAndHookBead(townRoot, rigName, beadID, SpawnAndHookOptions{
					Force:    slingForce,
					Account:  slingAccount,
					Create:   slingCreate,
					Agent:    slingAgent,
					Subject:  slingSubject,
					Args:     slingArgs,
					LogEvent: true,
				})
				if spawnErr != nil {
					return fmt.Errorf("spawning polecat: %w", spawnErr)
				}

				// Wake witness and refinery
				wakeRigAgents(townRoot, rigName)

				// SpawnAndHookBead did everything - return early
				return nil
			} else {
				// Non-queue formula-on-bead to rig
				fmt.Printf("Target is rig '%s'...\n", rigName)

				// Check bead status before doing anything
				info, err := getBeadInfo(beadID)
				if err != nil {
					return fmt.Errorf("checking bead status: %w", err)
				}
				if (info.Status == "pinned" || info.Status == "hooked") && !slingForce {
					return fmt.Errorf("bead %s is already %s to %s\nUse --force to re-sling", beadID, info.Status, info.Assignee)
				}

				// Auto-convoy before formula instantiation
				if !slingNoConvoy && isTrackedByConvoy(beadID) == "" {
					convoyID, convoyErr := createAutoConvoy(beadID, info.Title)
					if convoyErr != nil {
						fmt.Printf("%s Could not create auto-convoy: %v\n", style.Dim.Render("Warning:"), convoyErr)
					} else {
						fmt.Printf("%s Created convoy 🚚 %s\n", style.Bold.Render("→"), convoyID)
					}
				}

				// Formula instantiation -> compound ID
				compoundID, err := InstantiateFormulaOnBead(townRoot, formulaName, beadID, info.Title)
				if err != nil {
					return err
				}

				// Spawn and hook the compound
				_, spawnErr := SpawnAndHookBead(townRoot, rigName, compoundID, SpawnAndHookOptions{
					Force:    slingForce,
					Account:  slingAccount,
					Create:   slingCreate,
					Agent:    slingAgent,
					Subject:  slingSubject,
					Args:     slingArgs,
					LogEvent: true,
				})
				if spawnErr != nil {
					return fmt.Errorf("spawning polecat: %w", spawnErr)
				}

				wakeRigAgents(townRoot, rigName)
				return nil
			}
		} else {
			// Slinging to an existing agent
			targetAgent, err = resolveTargetAgent(target)
			if err != nil {
				// Check if this is a dead polecat (no active session)
				// If so, spawn a fresh polecat instead of failing
				if isPolecatTarget(target) {
					// Extract rig name from polecat target (format: rig/polecats/name)
					parts := strings.Split(target, "/")
					if len(parts) >= 3 && parts[1] == "polecats" {
						rigName := parts[0]
						fmt.Printf("Target polecat has no active session, spawning fresh polecat in rig '%s'...\n", rigName)

						if formulaName == "" {
							// Plain bead - check status, convoy, use SpawnAndHookBead
							info, infoErr := getBeadInfo(beadID)
							if infoErr != nil {
								return fmt.Errorf("checking bead status: %w", infoErr)
							}
							if (info.Status == "pinned" || info.Status == "hooked") && !slingForce {
								return fmt.Errorf("bead %s is already %s to %s\nUse --force to re-sling", beadID, info.Status, info.Assignee)
							}

							// Auto-convoy
							if !slingNoConvoy && isTrackedByConvoy(beadID) == "" {
								convoyID, convoyErr := createAutoConvoy(beadID, info.Title)
								if convoyErr != nil {
									fmt.Printf("%s Could not create auto-convoy: %v\n", style.Dim.Render("Warning:"), convoyErr)
								} else {
									fmt.Printf("%s Created convoy 🚚 %s\n", style.Bold.Render("→"), convoyID)
								}
							}

							_, spawnErr := SpawnAndHookBead(townRoot, rigName, beadID, SpawnAndHookOptions{
								Force:    slingForce,
								Account:  slingAccount,
								Create:   slingCreate,
								Agent:    slingAgent,
								Subject:  slingSubject,
								Args:     slingArgs,
								LogEvent: true,
							})
							if spawnErr != nil {
								return fmt.Errorf("spawning polecat: %w", spawnErr)
							}
							wakeRigAgents(townRoot, rigName)
							return nil
						}

						// Formula-on-bead - do formula instantiation first, then spawn+hook compound
						info, infoErr := getBeadInfo(beadID)
						if infoErr != nil {
							return fmt.Errorf("checking bead status: %w", infoErr)
						}
						if (info.Status == "pinned" || info.Status == "hooked") && !slingForce {
							return fmt.Errorf("bead %s is already %s to %s\nUse --force to re-sling", beadID, info.Status, info.Assignee)
						}

						// Auto-convoy
						if !slingNoConvoy && isTrackedByConvoy(beadID) == "" {
							convoyID, convoyErr := createAutoConvoy(beadID, info.Title)
							if convoyErr != nil {
								fmt.Printf("%s Could not create auto-convoy: %v\n", style.Dim.Render("Warning:"), convoyErr)
							} else {
								fmt.Printf("%s Created convoy 🚚 %s\n", style.Bold.Render("→"), convoyID)
							}
						}

						// Formula instantiation -> compound ID
						compoundID, instErr := InstantiateFormulaOnBead(townRoot, formulaName, beadID, info.Title)
						if instErr != nil {
							return instErr
						}

						// Spawn and hook the compound
						_, spawnErr := SpawnAndHookBead(townRoot, rigName, compoundID, SpawnAndHookOptions{
							Force:    slingForce,
							Account:  slingAccount,
							Create:   slingCreate,
							Agent:    slingAgent,
							Subject:  slingSubject,
							Args:     slingArgs,
							LogEvent: true,
						})
						if spawnErr != nil {
							return fmt.Errorf("spawning polecat: %w", spawnErr)
						}
						wakeRigAgents(townRoot, rigName)
						return nil
					} else {
						return fmt.Errorf("resolving target: %w", err)
					}
				} else {
					return fmt.Errorf("resolving target: %w", err)
				}
			}
		}
	} else {
		// Slinging to self
		targetAgent, err = resolveSelfTarget()
		if err != nil {
			return err
		}
	}

	// --queue requires a rig target (spawning polecats)
	if slingQueue && !isRigTarget {
		return fmt.Errorf("--queue requires a rig target (e.g., 'gt sling %s gastown --queue')", beadID)
	}

	// Display what we're doing
	if formulaName != "" {
		fmt.Printf("%s Slinging formula %s on %s to %s...\n", style.Bold.Render("🎯"), formulaName, beadID, targetAgent)
	} else {
		fmt.Printf("%s Slinging %s to %s...\n", style.Bold.Render("🎯"), beadID, targetAgent)
	}

	// Check if bead is already pinned (guard against accidental re-sling)
	info, err := getBeadInfo(beadID)
	if err != nil {
		return fmt.Errorf("checking bead status: %w", err)
	}
	if info.Status == "pinned" && !slingForce {
		assignee := info.Assignee
		if assignee == "" {
			assignee = "(unknown)"
		}
		return fmt.Errorf("bead %s is already pinned to %s\nUse --force to re-sling", beadID, assignee)
	}

	// Auto-convoy: check if issue is already tracked by a convoy
	// If not, create one for dashboard visibility (unless --no-convoy is set)
	// Applies to both plain bead sling and formula-on-bead (single)
	if !slingNoConvoy {
		existingConvoy := isTrackedByConvoy(beadID)
		if existingConvoy == "" {
			if slingDryRun {
				fmt.Printf("Would create convoy 'Work: %s'\n", info.Title)
				fmt.Printf("Would add tracking relation to %s\n", beadID)
			} else {
				convoyID, err := createAutoConvoy(beadID, info.Title)
				if err != nil {
					// Log warning but don't fail - convoy is optional
					fmt.Printf("%s Could not create auto-convoy: %v\n", style.Dim.Render("Warning:"), err)
				} else {
					fmt.Printf("%s Created convoy 🚚 %s\n", style.Bold.Render("→"), convoyID)
					fmt.Printf("  Tracking: %s\n", beadID)
				}
			}
		} else {
			fmt.Printf("%s Already tracked by convoy %s\n", style.Dim.Render("○"), existingConvoy)
		}
	}

	if slingDryRun {
		if formulaName != "" {
			fmt.Printf("Would instantiate formula %s:\n", formulaName)
			fmt.Printf("  1. bd cook %s\n", formulaName)
			fmt.Printf("  2. bd mol wisp %s --var feature=\"%s\" --var issue=\"%s\"\n", formulaName, info.Title, beadID)
			fmt.Printf("  3. bd mol bond <wisp-root> %s\n", beadID)
			fmt.Printf("  4. bd update <compound-root> --status=hooked --assignee=%s\n", targetAgent)
		} else {
			fmt.Printf("Would run: bd update %s --status=hooked --assignee=%s\n", beadID, targetAgent)
		}
		if slingSubject != "" {
			fmt.Printf("  subject (in nudge): %s\n", slingSubject)
		}
		if slingMessage != "" {
			fmt.Printf("  context: %s\n", slingMessage)
		}
		if slingArgs != "" {
			fmt.Printf("  args (in nudge): %s\n", slingArgs)
		}
		fmt.Printf("Would nudge agent: %s\n", targetAgent)
		return nil
	}

	// Formula-on-bead mode for non-rig targets (existing agents)
	// Rig targets with formula-on-bead return early above
	if formulaName != "" {
		// Formula instantiation -> compound ID
		compoundID, err := InstantiateFormulaOnBead(townRoot, formulaName, beadID, info.Title)
		if err != nil {
			return err
		}

		// Record attached molecule after other description updates to avoid overwrite.
		attachedMoleculeID = compoundID

		// Hook the compound instead of bare bead
		beadID = compoundID
	}

	// Hook the bead using bd update.
	// See: https://github.com/steveyegge/gastown/issues/148
	hookCmd := exec.Command("bd", "--no-daemon", "update", beadID, "--status=hooked", "--assignee="+targetAgent)
	hookCmd.Dir = beads.ResolveHookDir(townRoot, beadID, "")
	hookCmd.Stderr = os.Stderr
	if err := hookCmd.Run(); err != nil {
		return fmt.Errorf("hooking bead: %w", err)
	}

	fmt.Printf("%s Work attached to hook (status=hooked)\n", style.Bold.Render("✓"))

	// Log sling event to activity feed
	actor := detectActor()
	_ = events.LogFeed(events.TypeSling, actor, events.SlingPayload(beadID, targetAgent))

	// Update agent bead's hook_bead field (ZFC: agents track their current work)
	updateAgentHookBead(targetAgent, beadID, "", townBeadsDir)

	// Auto-attach mol-polecat-work to polecat agent beads
	// This ensures polecats have the standard work molecule attached for guidance
	if strings.Contains(targetAgent, "/polecats/") {
		if err := attachPolecatWorkMolecule(targetAgent, hookWorkDir, townRoot); err != nil {
			// Warn but don't fail - polecat will still work without molecule
			fmt.Printf("%s Could not attach work molecule: %v\n", style.Dim.Render("Warning:"), err)
		}
	}

	// Store dispatcher in bead description (enables completion notification to dispatcher)
	if err := storeDispatcherInBead(beadID, actor); err != nil {
		// Warn but don't fail - polecat will still complete work
		fmt.Printf("%s Could not store dispatcher in bead: %v\n", style.Dim.Render("Warning:"), err)
	}

	// Store args in bead description (no-tmux mode: beads as data plane)
	if slingArgs != "" {
		if err := storeArgsInBead(beadID, slingArgs); err != nil {
			// Warn but don't fail - args will still be in the nudge prompt
			fmt.Printf("%s Could not store args in bead: %v\n", style.Dim.Render("Warning:"), err)
		} else {
			fmt.Printf("%s Args stored in bead (durable)\n", style.Bold.Render("✓"))
		}
	}

	// Record the attached molecule in the wisp's description.
	// This is required for gt hook to recognize the molecule attachment.
	if attachedMoleculeID != "" {
		if err := storeAttachedMoleculeInBead(beadID, attachedMoleculeID); err != nil {
			// Warn but don't fail - polecat can still work through steps
			fmt.Printf("%s Could not store attached_molecule: %v\n", style.Dim.Render("Warning:"), err)
		}
	}

	// Try to inject the "start now" prompt (graceful if no tmux)
	// Use targetAgent (logical format) directly - translation to tmux happens in session layer
	if targetAgent == "" {
		fmt.Printf("%s No agent to nudge (work will be discovered via gt prime)\n", style.Dim.Render("○"))
	} else {
		agentID, err := addressToAgentID(targetAgent)
		if err != nil {
			// Placeholder or invalid address - skip nudging
			fmt.Printf("%s Agent address %q not nudgeable: %v\n", style.Dim.Render("○"), targetAgent, err)
		} else {
			// Ensure agent is ready before nudging (prevents race condition where
			// message arrives before Claude has fully started - see issue #115)
			if err := ensureAgentReady(townRoot, agentID); err != nil {
				// Non-fatal: warn and continue, agent will discover work via gt prime
				fmt.Printf("%s Could not verify agent ready: %v\n", style.Dim.Render("○"), err)
			}

			if err := injectStartPrompt(townRoot, agentID, beadID, slingSubject, slingArgs); err != nil {
				// Graceful fallback for no-tmux mode
				fmt.Printf("%s Could not nudge (no tmux?): %v\n", style.Dim.Render("○"), err)
				fmt.Printf("  Agent will discover work via gt prime / bd show\n")
			} else {
				fmt.Printf("%s Start prompt sent\n", style.Bold.Render("▶"))
			}
		}
	}

	return nil
}
