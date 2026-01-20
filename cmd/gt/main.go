// gt is the Gas Town CLI for managing multi-agent workspaces.
package main

import (
	"os"

	"github.com/steveyegge/gastown/internal/cmd"
)

// main is the entry point for the Gas Town CLI. It delegates all command
// parsing and execution to cmd.Execute() and exits with its return code.
func main() {
	os.Exit(cmd.Execute())
}
