// Package runner provides command execution abstraction for local and remote operations.
package runner

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Runner executes shell commands either locally or remotely.
type Runner interface {
	// Run executes a command in the given directory.
	// If dir is empty, uses the current working directory.
	Run(dir, name string, args ...string) error

	// Output executes a command and returns its stdout.
	Output(dir, name string, args ...string) ([]byte, error)

	// CombinedOutput executes a command and returns combined stdout/stderr.
	CombinedOutput(dir, name string, args ...string) ([]byte, error)

	// RunInteractive runs a command with stdin/stdout/stderr attached.
	// Used for interactive commands like tmux attach.
	RunInteractive(name string, args ...string) error
}

// Local executes commands on the local machine.
type Local struct{}

// NewLocal creates a new local runner.
func NewLocal() *Local {
	return &Local{}
}

func (r *Local) Run(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	return cmd.Run()
}

func (r *Local) Output(dir, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	return cmd.Output()
}

func (r *Local) CombinedOutput(dir, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	return cmd.CombinedOutput()
}

func (r *Local) RunInteractive(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// SSH executes commands on a remote machine via SSH.
type SSH struct {
	sshCmd string // e.g., "ssh user@host" or "ssh -i key user@host"
}

// NewSSH creates a new SSH runner.
func NewSSH(sshCmd string) *SSH {
	return &SSH{sshCmd: sshCmd}
}

func (r *SSH) Run(dir, name string, args ...string) error {
	remoteCmd := r.buildRemoteCommand(dir, name, args)
	return exec.Command("sh", "-c", remoteCmd).Run()
}

func (r *SSH) Output(dir, name string, args ...string) ([]byte, error) {
	remoteCmd := r.buildRemoteCommand(dir, name, args)
	return exec.Command("sh", "-c", remoteCmd).Output()
}

func (r *SSH) CombinedOutput(dir, name string, args ...string) ([]byte, error) {
	remoteCmd := r.buildRemoteCommand(dir, name, args)
	return exec.Command("sh", "-c", remoteCmd).CombinedOutput()
}

func (r *SSH) RunInteractive(name string, args ...string) error {
	// Build the remote command
	var cmdParts []string
	cmdParts = append(cmdParts, name)
	for _, arg := range args {
		cmdParts = append(cmdParts, shellEscape(arg))
	}
	innerCmd := strings.Join(cmdParts, " ")

	// Run via SSH with -t for TTY allocation (required for interactive commands)
	fullCmd := fmt.Sprintf("%s -t %s", r.sshCmd, shellEscape(innerCmd))
	cmd := exec.Command("sh", "-c", fullCmd)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// buildRemoteCommand constructs the full SSH command string.
func (r *SSH) buildRemoteCommand(dir, name string, args []string) string {
	// Build the command to run on remote
	var cmdParts []string
	if dir != "" {
		cmdParts = append(cmdParts, fmt.Sprintf("cd %s &&", shellEscape(dir)))
	}
	cmdParts = append(cmdParts, name)
	for _, arg := range args {
		cmdParts = append(cmdParts, shellEscape(arg))
	}
	innerCmd := strings.Join(cmdParts, " ")

	// Wrap in SSH
	return fmt.Sprintf("%s %s", r.sshCmd, shellEscape(innerCmd))
}

// shellEscape escapes a string for safe use in shell commands.
func shellEscape(s string) string {
	// Wrap in single quotes, escape existing single quotes
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
