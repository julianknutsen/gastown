package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/migrate"
	"github.com/steveyegge/gastown/internal/style"
)

var migrateCmd = &cobra.Command{
	Use:     "migrate [TOWN_DIR]",
	GroupID: GroupWorkspace,
	Short:   "Generate a gc city.toml from this Gas Town",
	Long: `Reads your town config and generates a city.toml for Gas City.

The generated config references the gastown topology remotely —
no prompts or formulas are copied. They live in the gascity repo
at examples/gastown/topologies/ and gc fetches them at start time.

If TOWN_DIR is not provided, walks up from the current directory
to find the Gas Town root (looks for mayor/town.json).`,
	Args: cobra.MaximumNArgs(1),
	RunE: runMigrate,
}

func init() {
	migrateCmd.Flags().StringP("output", "o", ".", "Directory to write city.toml")
	rootCmd.AddCommand(migrateCmd)
}

func runMigrate(cmd *cobra.Command, args []string) error {
	// Determine town root.
	var townRoot string
	if len(args) > 0 {
		townRoot = args[0]
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}
		townRoot = beads.FindTownRoot(cwd)
		if townRoot == "" {
			return fmt.Errorf("not inside a Gas Town (no mayor/town.json found above %s)", cwd)
		}
	}

	// Make absolute.
	var err error
	townRoot, err = filepath.Abs(townRoot)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	// Read town config.
	snap, err := migrate.ReadTownSnapshot(townRoot)
	if err != nil {
		return fmt.Errorf("reading town config: %w", err)
	}

	// Generate city.toml.
	content, err := migrate.GenerateCityTOML(snap)
	if err != nil {
		return fmt.Errorf("generating city.toml: %w", err)
	}

	// Determine output path.
	outputDir, _ := cmd.Flags().GetString("output")
	outputDir, err = filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("resolving output path: %w", err)
	}
	outputPath := filepath.Join(outputDir, "city.toml")

	// Write the file.
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing city.toml: %w", err)
	}

	// Print instructions.
	fmt.Fprintf(cmd.OutOrStdout(), "%s Generated %s\n\n", style.Bold.Render("✓"), outputPath)
	fmt.Fprintf(cmd.OutOrStdout(), "Next steps:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  1. Review the generated city.toml\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  2. Install gc: go install github.com/steveyegge/gascity/cmd/gc@latest\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  3. Initialize: gc init --file %s <city-dir>\n", outputPath)
	fmt.Fprintf(cmd.OutOrStdout(), "  4. Start: cd <city-dir> && gc start\n")
	return nil
}
