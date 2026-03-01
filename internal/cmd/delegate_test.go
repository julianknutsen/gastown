package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestFindCityRoot(t *testing.T) {
	t.Run("city.toml in cwd", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "city.toml"), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
		got := findCityRoot(dir)
		if got != dir {
			t.Errorf("findCityRoot(%q) = %q, want %q", dir, got, dir)
		}
	})

	t.Run("city.toml in parent", func(t *testing.T) {
		parent := t.TempDir()
		child := filepath.Join(parent, "subdir")
		if err := os.Mkdir(child, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(parent, "city.toml"), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
		got := findCityRoot(child)
		if got != parent {
			t.Errorf("findCityRoot(%q) = %q, want %q", child, got, parent)
		}
	})

	t.Run("not found", func(t *testing.T) {
		dir := t.TempDir()
		got := findCityRoot(dir)
		if got != "" {
			t.Errorf("findCityRoot(%q) = %q, want empty string", dir, got)
		}
	})
}

func TestTopLevelCommandName(t *testing.T) {
	t.Run("leaf command", func(t *testing.T) {
		root := &cobra.Command{Use: "gt"}
		child := &cobra.Command{Use: "status"}
		root.AddCommand(child)

		got := topLevelCommandName(child)
		if got != "status" {
			t.Errorf("topLevelCommandName = %q, want %q", got, "status")
		}
	})

	t.Run("subcommand", func(t *testing.T) {
		root := &cobra.Command{Use: "gt"}
		daemon := &cobra.Command{Use: "daemon"}
		start := &cobra.Command{Use: "start"}
		root.AddCommand(daemon)
		daemon.AddCommand(start)

		got := topLevelCommandName(start)
		if got != "daemon" {
			t.Errorf("topLevelCommandName = %q, want %q", got, "daemon")
		}
	})

	t.Run("root command itself", func(t *testing.T) {
		root := &cobra.Command{Use: "gt"}
		got := topLevelCommandName(root)
		if got != "gt" {
			t.Errorf("topLevelCommandName = %q, want %q", got, "gt")
		}
	})
}

func TestDelegatableCommands(t *testing.T) {
	expected := []string{
		"start", "stop", "status", "restart", "rig", "config", "mail", "hook",
		"sling", "convoy", "prime", "handoff", "doctor", "dolt", "daemon", "formula", "version",
	}
	for _, cmd := range expected {
		if !delegatableCommands[cmd] {
			t.Errorf("%q should be delegatable", cmd)
		}
	}
	if len(delegatableCommands) != len(expected) {
		t.Errorf("delegatableCommands has %d entries, want %d", len(delegatableCommands), len(expected))
	}
}

func TestNonDelegatableCommands(t *testing.T) {
	// These should NOT be in the passthrough set (role commands use rewriting instead).
	excluded := []string{"migrate", "mayor", "deacon", "polecat", "crew", "init", "install"}
	for _, cmd := range excluded {
		if delegatableCommands[cmd] {
			t.Errorf("%q should NOT be in delegatableCommands", cmd)
		}
	}
}

func TestRewriteRoleArgs_Singleton(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "mayor attach",
			args: []string{"mayor", "attach"},
			want: []string{"agent", "attach", "mayor"},
		},
		{
			name: "mayor status --json",
			args: []string{"mayor", "status", "--json"},
			want: []string{"agent", "status", "mayor", "--json"},
		},
		{
			name: "deacon drain",
			args: []string{"deacon", "drain"},
			want: []string{"agent", "drain", "deacon"},
		},
		{
			name: "mayor restart rewrites to request-restart",
			args: []string{"mayor", "restart"},
			want: []string{"agent", "request-restart", "mayor"},
		},
		{
			name: "mayor at rewrites to attach",
			args: []string{"mayor", "at"},
			want: []string{"agent", "attach", "mayor"},
		},
		{
			name: "mayor stop rewrites to kill",
			args: []string{"mayor", "stop"},
			want: []string{"agent", "kill", "mayor"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rewriteRoleArgs(tt.args)
			if !sliceEqual(got, tt.want) {
				t.Errorf("rewriteRoleArgs(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestRewriteRoleArgs_Multi(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "crew attach worker1",
			args: []string{"crew", "attach", "worker1"},
			want: []string{"agent", "attach", "worker1"},
		},
		{
			name: "crew list",
			args: []string{"crew", "list"},
			want: []string{"agent", "list"},
		},
		{
			name: "crew status worker1 --json",
			args: []string{"crew", "status", "worker1", "--json"},
			want: []string{"agent", "status", "worker1", "--json"},
		},
		{
			name: "crew remove rewrites to kill",
			args: []string{"crew", "remove", "worker1"},
			want: []string{"agent", "kill", "worker1"},
		},
		{
			name: "crew spawn rewrites to add",
			args: []string{"crew", "spawn", "worker1"},
			want: []string{"agent", "add", "worker1"},
		},
		{
			name: "polecat list",
			args: []string{"polecat", "list"},
			want: []string{"agent", "list"},
		},
		{
			name: "polecat nuke rewrites to kill",
			args: []string{"polecat", "nuke", "worker1"},
			want: []string{"agent", "kill", "worker1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rewriteRoleArgs(tt.args)
			if !sliceEqual(got, tt.want) {
				t.Errorf("rewriteRoleArgs(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestRewriteRoleArgs_TopLevel(t *testing.T) {
	// start maps to top-level gc start, dropping the role name.
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "mayor start → gc start",
			args: []string{"mayor", "start"},
			want: []string{"start"},
		},
		{
			name: "deacon start → gc start",
			args: []string{"deacon", "start"},
			want: []string{"start"},
		},
		{
			name: "crew start → gc start",
			args: []string{"crew", "start"},
			want: []string{"start"},
		},
		{
			name: "crew start with trailing args",
			args: []string{"crew", "start", "--verbose"},
			want: []string{"start", "--verbose"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rewriteRoleArgs(tt.args)
			if !sliceEqual(got, tt.want) {
				t.Errorf("rewriteRoleArgs(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestRewriteRoleArgs_NoEquivalent(t *testing.T) {
	// Subcommands without gc equivalents should return nil.
	tests := []struct {
		name string
		args []string
	}{
		{"deacon heartbeat", []string{"deacon", "heartbeat"}},
		{"deacon zombie-scan", []string{"deacon", "zombie-scan"}},
		{"crew refresh", []string{"crew", "refresh"}},
		{"crew pristine", []string{"crew", "pristine"}},
		{"crew next", []string{"crew", "next"}},
		{"polecat gc", []string{"polecat", "gc"}},
		{"polecat git-state", []string{"polecat", "git-state"}},
		{"too few args", []string{"mayor"}},
		{"not a role", []string{"status", "foo"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rewriteRoleArgs(tt.args)
			if got != nil {
				t.Errorf("rewriteRoleArgs(%v) = %v, want nil", tt.args, got)
			}
		})
	}
}

func TestCityUnsupportedError(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "city.toml"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })

	t.Run("role command with no equivalent errors", func(t *testing.T) {
		os.Args = []string{"gt", "deacon", "heartbeat"}
		root := &cobra.Command{Use: "gt"}
		deacon := &cobra.Command{Use: "deacon"}
		heartbeat := &cobra.Command{Use: "heartbeat"}
		root.AddCommand(deacon)
		deacon.AddCommand(heartbeat)

		err := cityUnsupportedError(heartbeat)
		if err == nil {
			t.Fatal("expected error for unsupported role subcommand in city")
		}
		if got := err.Error(); got != "gt deacon heartbeat is not supported in Gas City; use gc commands directly" {
			t.Errorf("error = %q", got)
		}
	})

	t.Run("non-role command returns nil", func(t *testing.T) {
		os.Args = []string{"gt", "migrate"}
		root := &cobra.Command{Use: "gt"}
		migrate := &cobra.Command{Use: "migrate"}
		root.AddCommand(migrate)

		if err := cityUnsupportedError(migrate); err != nil {
			t.Errorf("expected nil for non-role command, got %v", err)
		}
	})

	t.Run("no city.toml returns nil", func(t *testing.T) {
		noCityDir := t.TempDir()
		if err := os.Chdir(noCityDir); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Chdir(dir) }() // restore to city dir for other subtests

		os.Args = []string{"gt", "deacon", "heartbeat"}
		root := &cobra.Command{Use: "gt"}
		deacon := &cobra.Command{Use: "deacon"}
		heartbeat := &cobra.Command{Use: "heartbeat"}
		root.AddCommand(deacon)
		deacon.AddCommand(heartbeat)

		if err := cityUnsupportedError(heartbeat); err != nil {
			t.Errorf("expected nil outside city, got %v", err)
		}
	})
}

func TestBuildGCArgs_RoleRewrite(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "city.toml"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	origArgs := os.Args
	os.Args = []string{"gt", "mayor", "attach"}
	t.Cleanup(func() { os.Args = origArgs })

	args := buildGCArgs()
	want := []string{"agent", "attach", "mayor"}
	if !sliceEqual(args, want) {
		t.Errorf("buildGCArgs() = %v, want %v", args, want)
	}
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestShouldDelegate_NoCityTOML(t *testing.T) {
	// In a directory without city.toml, shouldDelegate should return false.
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	root := &cobra.Command{Use: "gt"}
	cmd := &cobra.Command{Use: "status"}
	root.AddCommand(cmd)

	if shouldDelegate(cmd) {
		t.Error("shouldDelegate should return false when no city.toml exists")
	}
}

func TestShouldDelegate_NonDelegatableCommand(t *testing.T) {
	// city.toml present but command not in delegatable set.
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "city.toml"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	root := &cobra.Command{Use: "gt"}
	cmd := &cobra.Command{Use: "migrate"}
	root.AddCommand(cmd)

	if shouldDelegate(cmd) {
		t.Error("shouldDelegate should return false for non-delegatable command 'migrate'")
	}
}

func TestBuildGCArgs_SameDir(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "city.toml"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	origArgs := os.Args
	os.Args = []string{"gt", "status", "--json"}
	t.Cleanup(func() { os.Args = origArgs })

	args := buildGCArgs()
	// Should be just the args after the binary name, no --city prepended.
	if len(args) != 2 || args[0] != "status" || args[1] != "--json" {
		t.Errorf("buildGCArgs() = %v, want [status --json]", args)
	}
}

func TestBuildGCArgs_ParentDir(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	child := filepath.Join(parent, "subdir")
	if err := os.Mkdir(child, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "city.toml"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(child); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	origArgs := os.Args
	os.Args = []string{"gt", "start"}
	t.Cleanup(func() { os.Args = origArgs })

	args := buildGCArgs()
	// Should prepend --city <parent>.
	if len(args) < 3 || args[0] != "--city" || args[1] != parent || args[2] != "start" {
		t.Errorf("buildGCArgs() = %v, want [--city %s start]", args, parent)
	}
}
