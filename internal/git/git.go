// Package git provides a wrapper for git operations via subprocess.
package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Common errors
var (
	ErrNotARepo       = errors.New("not a git repository")
	ErrMergeConflict  = errors.New("merge conflict")
	ErrAuthFailure    = errors.New("authentication failed")
	ErrRebaseConflict = errors.New("rebase conflict")
)

// Git wraps git operations for a working directory.
type Git struct {
	workDir string
}

// NewGit creates a new Git wrapper for the given directory.
func NewGit(workDir string) *Git {
	return &Git{workDir: workDir}
}

// run executes a git command and returns stdout.
func (g *Git) run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", g.wrapError(err, stderr.String(), args)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// wrapError wraps git errors with context.
func (g *Git) wrapError(err error, stderr string, args []string) error {
	stderr = strings.TrimSpace(stderr)

	// Detect specific error types
	if strings.Contains(stderr, "not a git repository") {
		return ErrNotARepo
	}
	if strings.Contains(stderr, "CONFLICT") || strings.Contains(stderr, "Merge conflict") {
		return ErrMergeConflict
	}
	if strings.Contains(stderr, "Authentication failed") || strings.Contains(stderr, "could not read Username") {
		return ErrAuthFailure
	}
	if strings.Contains(stderr, "needs merge") || strings.Contains(stderr, "rebase in progress") {
		return ErrRebaseConflict
	}

	if stderr != "" {
		return fmt.Errorf("git %s: %s", args[0], stderr)
	}
	return fmt.Errorf("git %s: %w", args[0], err)
}

// Clone clones a repository to the destination.
func (g *Git) Clone(url, dest string) error {
	cmd := exec.Command("git", "clone", url, dest)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return g.wrapError(err, stderr.String(), []string{"clone", url})
	}
	return nil
}

// Checkout checks out the given ref.
func (g *Git) Checkout(ref string) error {
	_, err := g.run("checkout", ref)
	return err
}

// Fetch fetches from the remote.
func (g *Git) Fetch(remote string) error {
	_, err := g.run("fetch", remote)
	return err
}

// FetchBranch fetches a specific branch from the remote.
func (g *Git) FetchBranch(remote, branch string) error {
	_, err := g.run("fetch", remote, branch)
	return err
}

// Pull pulls from the remote branch.
func (g *Git) Pull(remote, branch string) error {
	_, err := g.run("pull", remote, branch)
	return err
}

// Push pushes to the remote branch.
func (g *Git) Push(remote, branch string, force bool) error {
	args := []string{"push", remote, branch}
	if force {
		args = append(args, "--force")
	}
	_, err := g.run(args...)
	return err
}

// Add stages files for commit.
func (g *Git) Add(paths ...string) error {
	args := append([]string{"add"}, paths...)
	_, err := g.run(args...)
	return err
}

// Commit creates a commit with the given message.
func (g *Git) Commit(message string) error {
	_, err := g.run("commit", "-m", message)
	return err
}

// CommitAll stages all changes and commits.
func (g *Git) CommitAll(message string) error {
	_, err := g.run("commit", "-am", message)
	return err
}

// GitStatus represents the status of the working directory.
type GitStatus struct {
	Clean    bool
	Modified []string
	Added    []string
	Deleted  []string
	Untracked []string
}

// Status returns the current git status.
func (g *Git) Status() (*GitStatus, error) {
	out, err := g.run("status", "--porcelain")
	if err != nil {
		return nil, err
	}

	status := &GitStatus{Clean: true}
	if out == "" {
		return status, nil
	}

	status.Clean = false
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 3 {
			continue
		}
		code := line[:2]
		file := line[3:]

		switch {
		case strings.Contains(code, "M"):
			status.Modified = append(status.Modified, file)
		case strings.Contains(code, "A"):
			status.Added = append(status.Added, file)
		case strings.Contains(code, "D"):
			status.Deleted = append(status.Deleted, file)
		case strings.Contains(code, "?"):
			status.Untracked = append(status.Untracked, file)
		}
	}

	return status, nil
}

// CurrentBranch returns the current branch name.
func (g *Git) CurrentBranch() (string, error) {
	return g.run("rev-parse", "--abbrev-ref", "HEAD")
}

// HasUncommittedChanges returns true if there are uncommitted changes.
func (g *Git) HasUncommittedChanges() (bool, error) {
	status, err := g.Status()
	if err != nil {
		return false, err
	}
	return !status.Clean, nil
}

// RemoteURL returns the URL for the given remote.
func (g *Git) RemoteURL(remote string) (string, error) {
	return g.run("remote", "get-url", remote)
}

// Merge merges the given branch into the current branch.
func (g *Git) Merge(branch string) error {
	_, err := g.run("merge", branch)
	return err
}

// Rebase rebases the current branch onto the given ref.
func (g *Git) Rebase(onto string) error {
	_, err := g.run("rebase", onto)
	return err
}

// AbortMerge aborts a merge in progress.
func (g *Git) AbortMerge() error {
	_, err := g.run("merge", "--abort")
	return err
}

// CheckConflicts performs a test merge to check if source can be merged into target
// without conflicts. Returns a list of conflicting files, or empty slice if clean.
// The merge is always aborted after checking - no actual changes are made.
//
// The caller must ensure the working directory is clean before calling this.
// After return, the working directory is restored to the target branch.
func (g *Git) CheckConflicts(source, target string) ([]string, error) {
	// Checkout the target branch
	if err := g.Checkout(target); err != nil {
		return nil, fmt.Errorf("checkout target %s: %w", target, err)
	}

	// Attempt test merge with --no-commit --no-ff
	// We need to capture both stdout and stderr to detect conflicts
	_, mergeErr := g.runMergeCheck("merge", "--no-commit", "--no-ff", source)

	if mergeErr != nil {
		// Check if there are unmerged files (indicates conflict)
		conflicts, err := g.getConflictingFiles()
		if err == nil && len(conflicts) > 0 {
			// Abort the test merge
			g.AbortMerge()
			return conflicts, nil
		}

		// Check if it's a conflict error from wrapper
		if errors.Is(mergeErr, ErrMergeConflict) {
			g.AbortMerge()
			return conflicts, nil
		}

		// Some other merge error
		g.AbortMerge()
		return nil, mergeErr
	}

	// Merge succeeded (no conflicts) - abort the test merge
	// Use reset since --abort won't work on successful merge
	g.run("reset", "--hard", "HEAD")
	return nil, nil
}

// runMergeCheck runs a git merge command and returns error info from both stdout and stderr.
// This is needed because git merge outputs CONFLICT info to stdout.
func (g *Git) runMergeCheck(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// Check stdout for CONFLICT message (git sends it there)
		stdoutStr := stdout.String()
		if strings.Contains(stdoutStr, "CONFLICT") {
			return "", ErrMergeConflict
		}
		// Fall back to stderr check
		return "", g.wrapError(err, stderr.String(), args)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// getConflictingFiles returns the list of files with merge conflicts.
func (g *Git) getConflictingFiles() ([]string, error) {
	// git diff --name-only --diff-filter=U shows unmerged files
	out, err := g.run("diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, err
	}

	if out == "" {
		return nil, nil
	}

	files := strings.Split(out, "\n")
	// Filter out empty strings
	var result []string
	for _, f := range files {
		if f != "" {
			result = append(result, f)
		}
	}
	return result, nil
}

// AbortRebase aborts a rebase in progress.
func (g *Git) AbortRebase() error {
	_, err := g.run("rebase", "--abort")
	return err
}

// CreateBranch creates a new branch.
func (g *Git) CreateBranch(name string) error {
	_, err := g.run("branch", name)
	return err
}

// CreateBranchFrom creates a new branch from a specific ref.
func (g *Git) CreateBranchFrom(name, ref string) error {
	_, err := g.run("branch", name, ref)
	return err
}

// BranchExists checks if a branch exists locally.
func (g *Git) BranchExists(name string) (bool, error) {
	_, err := g.run("show-ref", "--verify", "--quiet", "refs/heads/"+name)
	if err != nil {
		// Exit code 1 means branch doesn't exist
		if strings.Contains(err.Error(), "exit status 1") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// RemoteBranchExists checks if a branch exists on the remote.
func (g *Git) RemoteBranchExists(remote, branch string) (bool, error) {
	_, err := g.run("ls-remote", "--heads", remote, branch)
	if err != nil {
		return false, err
	}
	// ls-remote returns empty if branch doesn't exist, need to check output
	out, err := g.run("ls-remote", "--heads", remote, branch)
	if err != nil {
		return false, err
	}
	return out != "", nil
}

// DeleteBranch deletes a branch.
func (g *Git) DeleteBranch(name string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := g.run("branch", flag, name)
	return err
}

// Rev returns the commit hash for the given ref.
func (g *Git) Rev(ref string) (string, error) {
	return g.run("rev-parse", ref)
}

// IsAncestor checks if ancestor is an ancestor of descendant.
func (g *Git) IsAncestor(ancestor, descendant string) (bool, error) {
	_, err := g.run("merge-base", "--is-ancestor", ancestor, descendant)
	if err != nil {
		// Exit code 1 means not an ancestor, not an error
		if strings.Contains(err.Error(), "exit status 1") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// WorktreeAdd creates a new worktree at the given path with a new branch.
// The new branch is created from the current HEAD.
func (g *Git) WorktreeAdd(path, branch string) error {
	_, err := g.run("worktree", "add", "-b", branch, path)
	return err
}

// WorktreeAddDetached creates a new worktree at the given path with a detached HEAD.
func (g *Git) WorktreeAddDetached(path, ref string) error {
	_, err := g.run("worktree", "add", "--detach", path, ref)
	return err
}

// WorktreeAddExisting creates a new worktree at the given path for an existing branch.
func (g *Git) WorktreeAddExisting(path, branch string) error {
	_, err := g.run("worktree", "add", path, branch)
	return err
}

// WorktreeRemove removes a worktree.
func (g *Git) WorktreeRemove(path string, force bool) error {
	args := []string{"worktree", "remove", path}
	if force {
		args = append(args, "--force")
	}
	_, err := g.run(args...)
	return err
}

// WorktreePrune removes worktree entries for deleted paths.
func (g *Git) WorktreePrune() error {
	_, err := g.run("worktree", "prune")
	return err
}

// Worktree represents a git worktree.
type Worktree struct {
	Path   string
	Branch string
	Commit string
}

// WorktreeList returns all worktrees for this repository.
func (g *Git) WorktreeList() ([]Worktree, error) {
	out, err := g.run("worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}

	var worktrees []Worktree
	var current Worktree

	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			if current.Path != "" {
				worktrees = append(worktrees, current)
				current = Worktree{}
			}
			continue
		}

		switch {
		case strings.HasPrefix(line, "worktree "):
			current.Path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "HEAD "):
			current.Commit = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
		}
	}

	// Don't forget the last one
	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	return worktrees, nil
}
