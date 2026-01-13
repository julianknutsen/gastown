package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/config"
	"github.com/steveyegge/gastown/internal/constants"
	"github.com/steveyegge/gastown/internal/plugin"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/workspace"
)

// Plugin command flags
var (
	pluginListJSON bool
	pluginShowJSON bool
)

var pluginCmd = &cobra.Command{
	Use:     "plugin",
	GroupID: GroupConfig,
	Short:   "Plugin management",
	Long: `Manage plugins that run during Deacon patrol cycles.

Plugins are periodic automation tasks defined by plugin.md files with TOML frontmatter.

PLUGIN LOCATIONS:
  ~/gt/plugins/           Town-level plugins (universal, apply everywhere)
  <rig>/plugins/          Rig-level plugins (project-specific)

GATE TYPES:
  cooldown    Run if enough time has passed (e.g., 1h)
  cron        Run on a schedule (e.g., "0 9 * * *")
  condition   Run if a check command returns exit 0
  event       Run on events (e.g., startup)
  manual      Never auto-run, trigger explicitly

Examples:
  gt plugin list                    # List all discovered plugins
  gt plugin show <name>             # Show plugin details
  gt plugin list --json             # JSON output`,
	RunE: requireSubcommand,
}

var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all discovered plugins",
	Long: `List all plugins from town and rig plugin directories.

Plugins are discovered from:
  - ~/gt/plugins/ (town-level)
  - <rig>/plugins/ for each registered rig

When a plugin exists at both levels, the rig-level version takes precedence.

Examples:
  gt plugin list              # Human-readable output
  gt plugin list --json       # JSON output for scripting`,
	RunE: runPluginList,
}

var pluginShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show plugin details",
	Long: `Show detailed information about a plugin.

Displays the plugin's configuration, gate settings, and instructions.

Examples:
  gt plugin show rebuild-gt
  gt plugin show rebuild-gt --json`,
	Args: cobra.ExactArgs(1),
	RunE: runPluginShow,
}

func init() {
	// List subcommand flags
	pluginListCmd.Flags().BoolVar(&pluginListJSON, "json", false, "Output as JSON")

	// Show subcommand flags
	pluginShowCmd.Flags().BoolVar(&pluginShowJSON, "json", false, "Output as JSON")

	// Add subcommands
	pluginCmd.AddCommand(pluginListCmd)
	pluginCmd.AddCommand(pluginShowCmd)

	rootCmd.AddCommand(pluginCmd)
}

// getPluginScanner creates a scanner with town root and all rig names.
func getPluginScanner() (*plugin.Scanner, string, error) {
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return nil, "", fmt.Errorf("not in a Gas Town workspace: %w", err)
	}

	// Load rigs config to get rig names
	rigsConfigPath := constants.MayorRigsPath(townRoot)
	rigsConfig, err := config.LoadRigsConfig(rigsConfigPath)
	if err != nil {
		rigsConfig = &config.RigsConfig{Rigs: make(map[string]config.RigEntry)}
	}

	// Extract rig names
	rigNames := make([]string, 0, len(rigsConfig.Rigs))
	for name := range rigsConfig.Rigs {
		rigNames = append(rigNames, name)
	}
	sort.Strings(rigNames)

	scanner := plugin.NewScanner(townRoot, rigNames)
	return scanner, townRoot, nil
}

func runPluginList(cmd *cobra.Command, args []string) error {
	scanner, townRoot, err := getPluginScanner()
	if err != nil {
		return err
	}

	plugins, err := scanner.DiscoverAll()
	if err != nil {
		return fmt.Errorf("discovering plugins: %w", err)
	}

	// Sort plugins by name
	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].Name < plugins[j].Name
	})

	if pluginListJSON {
		return outputPluginListJSON(plugins)
	}

	return outputPluginListText(plugins, townRoot)
}

func outputPluginListJSON(plugins []*plugin.Plugin) error {
	summaries := make([]plugin.PluginSummary, len(plugins))
	for i, p := range plugins {
		summaries[i] = p.Summary()
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(summaries)
}

func outputPluginListText(plugins []*plugin.Plugin, townRoot string) error {
	if len(plugins) == 0 {
		fmt.Printf("%s No plugins discovered\n", style.Dim.Render("○"))
		fmt.Printf("\n  Plugin directories:\n")
		fmt.Printf("    %s/plugins/\n", townRoot)
		fmt.Printf("\n  Create a plugin by adding a directory with plugin.md\n")
		return nil
	}

	fmt.Printf("%s Discovered %d plugin(s)\n\n", style.Success.Render("●"), len(plugins))

	// Group by location
	townPlugins := make([]*plugin.Plugin, 0)
	rigPlugins := make(map[string][]*plugin.Plugin)

	for _, p := range plugins {
		if p.Location == plugin.LocationTown {
			townPlugins = append(townPlugins, p)
		} else {
			rigPlugins[p.RigName] = append(rigPlugins[p.RigName], p)
		}
	}

	// Print town-level plugins
	if len(townPlugins) > 0 {
		fmt.Printf("  %s\n", style.Bold.Render("Town-level plugins:"))
		for _, p := range townPlugins {
			printPluginSummary(p)
		}
		fmt.Println()
	}

	// Print rig-level plugins by rig
	rigNames := make([]string, 0, len(rigPlugins))
	for name := range rigPlugins {
		rigNames = append(rigNames, name)
	}
	sort.Strings(rigNames)

	for _, rigName := range rigNames {
		fmt.Printf("  %s\n", style.Bold.Render(fmt.Sprintf("Rig %s:", rigName)))
		for _, p := range rigPlugins[rigName] {
			printPluginSummary(p)
		}
		fmt.Println()
	}

	return nil
}

func printPluginSummary(p *plugin.Plugin) {
	gateType := "manual"
	if p.Gate != nil && p.Gate.Type != "" {
		gateType = string(p.Gate.Type)
	}

	desc := p.Description
	if len(desc) > 50 {
		desc = desc[:47] + "..."
	}

	fmt.Printf("    %s %s\n", style.Bold.Render(p.Name), style.Dim.Render(fmt.Sprintf("[%s]", gateType)))
	if desc != "" {
		fmt.Printf("      %s\n", style.Dim.Render(desc))
	}
}

func runPluginShow(cmd *cobra.Command, args []string) error {
	name := args[0]

	scanner, _, err := getPluginScanner()
	if err != nil {
		return err
	}

	p, err := scanner.GetPlugin(name)
	if err != nil {
		return err
	}

	if pluginShowJSON {
		return outputPluginShowJSON(p)
	}

	return outputPluginShowText(p)
}

func outputPluginShowJSON(p *plugin.Plugin) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(p)
}

func outputPluginShowText(p *plugin.Plugin) error {
	fmt.Printf("%s %s\n", style.Bold.Render("Plugin:"), p.Name)
	fmt.Printf("%s %s\n", style.Bold.Render("Path:"), p.Path)

	if p.Description != "" {
		fmt.Printf("%s %s\n", style.Bold.Render("Description:"), p.Description)
	}

	// Location
	locStr := string(p.Location)
	if p.RigName != "" {
		locStr = fmt.Sprintf("%s (%s)", p.Location, p.RigName)
	}
	fmt.Printf("%s %s\n", style.Bold.Render("Location:"), locStr)

	fmt.Printf("%s %d\n", style.Bold.Render("Version:"), p.Version)

	// Gate
	fmt.Println()
	fmt.Printf("%s\n", style.Bold.Render("Gate:"))
	if p.Gate != nil {
		fmt.Printf("  Type: %s\n", p.Gate.Type)
		if p.Gate.Duration != "" {
			fmt.Printf("  Duration: %s\n", p.Gate.Duration)
		}
		if p.Gate.Schedule != "" {
			fmt.Printf("  Schedule: %s\n", p.Gate.Schedule)
		}
		if p.Gate.Check != "" {
			fmt.Printf("  Check: %s\n", p.Gate.Check)
		}
		if p.Gate.On != "" {
			fmt.Printf("  On: %s\n", p.Gate.On)
		}
	} else {
		fmt.Printf("  Type: manual (no gate section)\n")
	}

	// Tracking
	if p.Tracking != nil {
		fmt.Println()
		fmt.Printf("%s\n", style.Bold.Render("Tracking:"))
		if len(p.Tracking.Labels) > 0 {
			fmt.Printf("  Labels: %s\n", strings.Join(p.Tracking.Labels, ", "))
		}
		fmt.Printf("  Digest: %v\n", p.Tracking.Digest)
	}

	// Execution
	if p.Execution != nil {
		fmt.Println()
		fmt.Printf("%s\n", style.Bold.Render("Execution:"))
		if p.Execution.Timeout != "" {
			fmt.Printf("  Timeout: %s\n", p.Execution.Timeout)
		}
		fmt.Printf("  Notify on failure: %v\n", p.Execution.NotifyOnFailure)
		if p.Execution.Severity != "" {
			fmt.Printf("  Severity: %s\n", p.Execution.Severity)
		}
	}

	// Instructions preview
	if p.Instructions != "" {
		fmt.Println()
		fmt.Printf("%s\n", style.Bold.Render("Instructions:"))
		lines := strings.Split(p.Instructions, "\n")
		preview := lines
		if len(lines) > 10 {
			preview = lines[:10]
		}
		for _, line := range preview {
			fmt.Printf("  %s\n", line)
		}
		if len(lines) > 10 {
			fmt.Printf("  %s\n", style.Dim.Render(fmt.Sprintf("... (%d more lines)", len(lines)-10)))
		}
	}

	return nil
}
