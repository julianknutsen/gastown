package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

// delegatableCommands lists gt commands that have exact gc equivalents.
// Args pass through unchanged: gt start → gc start.
var delegatableCommands = map[string]bool{
	"start":   true,
	"stop":    true,
	"status":  true,
	"restart": true,
	"rig":     true,
	"config":  true,
	"mail":    true,
	"hook":    true,
	"sling":   true,
	"convoy":  true,
	"prime":   true,
	"handoff": true,
	"doctor":  true,
	"dolt":    true,
	"daemon":  true,
	"formula": true,
	"version": true,
}

// roleKind distinguishes singleton roles (name IS the agent) from multi roles
// (agent name comes from user args).
type roleKind int

const (
	roleSingleton roleKind = iota + 1 // mayor, deacon — role name is the agent name
	roleMulti                         // crew, polecat — agent name in user args
)

// roleCommands maps gt role commands to their kind.
// These delegate to "gc agent <sub> ..." with arg rewriting.
var roleCommands = map[string]roleKind{
	"mayor":   roleSingleton,
	"deacon":  roleSingleton,
	"polecat": roleMulti,
	"crew":    roleMulti,
}

// roleSubRewrites maps gt subcommand names to gc agent subcommand names
// where they differ.
var roleSubRewrites = map[string]string{
	"at":      "attach",
	"restart": "request-restart",
	"remove":  "kill",
	"stop":    "kill",
	"nuke":    "kill",
	"spawn":   "add",
}

// roleTopLevelRewrites maps role subcommands to top-level gc commands
// (not gc agent subcommands). These drop the role name entirely.
//   gt mayor start → gc start
//   gt crew start  → gc start
var roleTopLevelRewrites = map[string]string{
	"start": "start",
}

// gcAgentSubs is the set of valid gc agent subcommands.
// A role subcommand (after rewriting) must land in this set to delegate.
var gcAgentSubs = map[string]bool{
	"add":             true,
	"attach":          true,
	"drain":           true,
	"drain-ack":       true,
	"drain-check":     true,
	"kill":            true,
	"list":            true,
	"nudge":           true,
	"peek":            true,
	"request-restart": true,
	"resume":          true,
	"status":          true,
	"suspend":         true,
	"undrain":         true,
}

// findCityRoot walks up from startDir looking for city.toml.
// Returns the directory containing city.toml, or "" if not found.
func findCityRoot(startDir string) string {
	dir := startDir
	for {
		cityFile := filepath.Join(dir, "city.toml")
		if _, err := os.Stat(cityFile); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "" // Reached filesystem root
		}
		dir = parent
	}
}

// topLevelCommandName walks up cobra's command tree to find the direct child
// of root. PersistentPreRunE receives the leaf command, so "gt daemon start"
// gives cmd.Name()=="start" but we need "daemon".
func topLevelCommandName(cmd *cobra.Command) string {
	for cmd.HasParent() && cmd.Parent().HasParent() {
		cmd = cmd.Parent()
	}
	return cmd.Name()
}

// shouldDelegate returns true when the current command should delegate to gc.
// Handles both passthrough commands (start, stop, ...) and role commands
// (mayor, crew, ...) that need arg rewriting.
func shouldDelegate(cmd *cobra.Command) bool {
	name := topLevelCommandName(cmd)

	canDelegate := delegatableCommands[name]
	if !canDelegate {
		// Check role commands — only delegatable if the subcommand maps.
		if _, isRole := roleCommands[name]; isRole {
			canDelegate = rewriteRoleArgs(os.Args[1:]) != nil
		}
	}
	if !canDelegate {
		return false
	}

	cwd, err := os.Getwd()
	if err != nil {
		return false
	}
	cityRoot := findCityRoot(cwd)
	if cityRoot == "" {
		return false
	}

	if _, err := exec.LookPath("gc"); err != nil {
		fmt.Fprintf(os.Stderr, "gt: city.toml found at %s but gc is not in PATH; falling back to gt\n", cityRoot)
		return false
	}

	return true
}

// cityUnsupportedError returns an error if the user runs a role command
// inside a Gas City directory that has no gc equivalent. This catches
// gastown-specific subcommands (heartbeat, zombie-scan, pristine, etc.)
// and gives a clear message instead of silently falling through to gt.
// Returns nil if not in a city, not a role command, or the subcommand maps.
func cityUnsupportedError(cmd *cobra.Command) error {
	name := topLevelCommandName(cmd)
	if _, isRole := roleCommands[name]; !isRole {
		return nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}
	if findCityRoot(cwd) == "" {
		return nil
	}

	// We're in a city with a role command that didn't delegate.
	// Build a readable command string from os.Args.
	gtCmd := "gt " + name
	if len(os.Args) > 2 {
		gtCmd += " " + os.Args[2]
	}
	return fmt.Errorf("%s is not supported in Gas City; use gc commands directly", gtCmd)
}

// rewriteRoleArgs rewrites role command args to gc agent args.
// Returns nil if the subcommand has no gc agent equivalent.
//
// Singleton (mayor, deacon):
//
//	["mayor", "attach", "--json"] → ["agent", "attach", "mayor", "--json"]
//
// Multi (crew, polecat):
//
//	["crew", "attach", "worker1"] → ["agent", "attach", "worker1"]
//	["crew", "list"]              → ["agent", "list"]
func rewriteRoleArgs(args []string) []string {
	if len(args) < 2 {
		return nil
	}

	role := args[0]
	kind, isRole := roleCommands[role]
	if !isRole {
		return nil
	}

	sub := args[1]

	// Top-level rewrites: subcommand maps to a gc top-level command,
	// not gc agent. Drop the role name entirely.
	//   gt mayor start → gc start
	if gcCmd, ok := roleTopLevelRewrites[sub]; ok {
		return append([]string{gcCmd}, args[2:]...)
	}

	if rewrite, ok := roleSubRewrites[sub]; ok {
		sub = rewrite
	}
	if !gcAgentSubs[sub] {
		return nil
	}

	rest := args[2:]
	switch kind {
	case roleSingleton:
		// gt mayor attach --json → gc agent attach mayor --json
		out := make([]string, 0, 3+len(rest))
		out = append(out, "agent", sub, role)
		out = append(out, rest...)
		return out
	case roleMulti:
		// gt crew attach worker1 → gc agent attach worker1
		out := make([]string, 0, 2+len(rest))
		out = append(out, "agent", sub)
		out = append(out, rest...)
		return out
	}
	return nil
}

// buildGCArgs constructs the argument list for the gc subprocess.
// Handles passthrough commands (args unchanged) and role commands (rewritten
// to gc agent ...). If the city root differs from cwd, prepends --city <path>.
func buildGCArgs() []string {
	raw := os.Args[1:]

	// Try role rewriting first.
	args := rewriteRoleArgs(raw)
	if args == nil {
		args = raw
	}

	cwd, err := os.Getwd()
	if err != nil {
		return args
	}
	cityRoot := findCityRoot(cwd)
	if cityRoot == "" || cityRoot == cwd {
		return args
	}

	// City root is a parent directory — tell gc where it is.
	return append([]string{"--city", cityRoot}, args...)
}

// delegateToGC spawns gc with the current arguments and exits with gc's exit code.
// This function does not return.
func delegateToGC() {
	if os.Getenv("GT_DELEGATE_QUIET") != "1" {
		cwd, _ := os.Getwd()
		cityRoot := findCityRoot(cwd)
		fmt.Fprintf(os.Stderr, "gt: delegating to gc (city.toml found at %s)\n", cityRoot)
	}

	gcPath, _ := exec.LookPath("gc") // already verified in shouldDelegate
	args := buildGCArgs()

	gcCmd := exec.Command(gcPath, args...)
	gcCmd.Stdin = os.Stdin
	gcCmd.Stdout = os.Stdout
	gcCmd.Stderr = os.Stderr

	if err := gcCmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
	os.Exit(0)
}
