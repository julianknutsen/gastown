package git

import (
	"fmt"
	"strings"

	"github.com/steveyegge/gastown/internal/runner"
)

// Ops provides git operations using a Runner.
// This allows the same git operations to work locally or remotely.
// Unlike the Git struct which works with a fixed working directory,
// Ops accepts the working directory as a parameter to each method.
type Ops struct {
	runner runner.Runner
}

// NewOps creates a new Ops with the given runner.
func NewOps(r runner.Runner) *Ops {
	return &Ops{runner: r}
}

// Fetch fetches from a remote.
func (g *Ops) Fetch(repoDir, remote string) error {
	return g.runner.Run(repoDir, "git", "fetch", remote)
}

// WorktreeAdd creates a new worktree with a new branch.
func (g *Ops) WorktreeAdd(repoDir, path, branch, startPoint string) error {
	return g.runner.Run(repoDir, "git", "worktree", "add", "-b", branch, path, startPoint)
}

// WorktreeRemove removes a worktree.
func (g *Ops) WorktreeRemove(repoDir, path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	return g.runner.Run(repoDir, "git", args...)
}

// WorktreePrune prunes stale worktree entries.
func (g *Ops) WorktreePrune(repoDir string) error {
	return g.runner.Run(repoDir, "git", "worktree", "prune")
}

// CurrentBranch returns the current branch name.
func (g *Ops) CurrentBranch(repoDir string) (string, error) {
	out, err := g.runner.Output(repoDir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Clone clones a repository.
func (g *Ops) Clone(url, destPath string) error {
	return g.runner.Run("", "git", "clone", url, destPath)
}

// CloneWithBranch clones a repository and checks out a specific branch.
func (g *Ops) CloneWithBranch(url, destPath, branch string) error {
	return g.runner.Run("", "git", "clone", "-b", branch, url, destPath)
}

// ListBranches lists branches matching a pattern.
func (g *Ops) ListBranches(repoDir, pattern string) ([]string, error) {
	args := []string{"branch", "--list"}
	if pattern != "" {
		args = append(args, pattern)
	}
	out, err := g.runner.Output(repoDir, "git", args...)
	if err != nil {
		return nil, err
	}

	var branches []string
	for _, line := range strings.Split(string(out), "\n") {
		// Remove leading whitespace and * marker for current branch
		branch := strings.TrimSpace(line)
		branch = strings.TrimPrefix(branch, "* ")
		if branch != "" {
			branches = append(branches, branch)
		}
	}
	return branches, nil
}

// DeleteBranch deletes a branch.
func (g *Ops) DeleteBranch(repoDir, branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	return g.runner.Run(repoDir, "git", "branch", flag, branch)
}

// CheckUncommittedWork checks for uncommitted changes.
// Returns (hasChanges, stashCount, unpushedCount, error)
func (g *Ops) CheckUncommittedWork(repoDir string) (bool, int, int, error) {
	// Check for uncommitted changes
	statusOut, err := g.runner.Output(repoDir, "git", "status", "--porcelain")
	if err != nil {
		return false, 0, 0, fmt.Errorf("git status: %w", err)
	}
	hasChanges := len(strings.TrimSpace(string(statusOut))) > 0

	// Check stash count
	stashOut, err := g.runner.Output(repoDir, "git", "stash", "list")
	stashCount := 0
	if err == nil {
		for _, line := range strings.Split(string(stashOut), "\n") {
			if strings.TrimSpace(line) != "" {
				stashCount++
			}
		}
	}

	// Check unpushed commits (compare with origin)
	unpushedCount := 0
	revOut, err := g.runner.Output(repoDir, "git", "rev-list", "--count", "@{u}..HEAD")
	if err == nil {
		fmt.Sscanf(strings.TrimSpace(string(revOut)), "%d", &unpushedCount)
	}

	return hasChanges, stashCount, unpushedCount, nil
}

// CountCommitsBehind counts commits behind a reference.
func (g *Ops) CountCommitsBehind(repoDir, ref string) (int, error) {
	out, err := g.runner.Output(repoDir, "git", "rev-list", "--count", "HEAD.."+ref)
	if err != nil {
		return 0, err
	}
	var count int
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &count)
	return count, nil
}

// Status returns the git status --porcelain output as lines.
func (g *Ops) Status(repoDir string) ([]string, error) {
	out, err := g.runner.Output(repoDir, "git", "status", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("git status: %w", err)
	}

	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			// Extract filename (skip the status prefix XY)
			if len(line) > 3 {
				files = append(files, line[3:])
			} else {
				files = append(files, line)
			}
		}
	}
	return files, nil
}

// StashCount returns the number of stashed entries.
func (g *Ops) StashCount(repoDir string) (int, error) {
	out, err := g.runner.Output(repoDir, "git", "stash", "list")
	if err != nil {
		return 0, nil // stash list failing is not fatal
	}

	count := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count, nil
}

// UnpushedCommitCount returns the number of commits ahead of the upstream.
func (g *Ops) UnpushedCommitCount(repoDir string) (int, error) {
	out, err := g.runner.Output(repoDir, "git", "rev-list", "--count", "@{u}..HEAD")
	if err != nil {
		return 0, nil // may not have upstream configured
	}
	var count int
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &count)
	return count, nil
}

// HasContentDiffFromRef checks if there's any actual content difference from a ref.
// This handles the case where commits exist but content is identical (e.g., after squash merge).
func (g *Ops) HasContentDiffFromRef(repoDir, ref string) (bool, error) {
	err := g.runner.Run(repoDir, "git", "diff", ref, "HEAD", "--quiet")
	if err != nil {
		// Exit code 1 means there's a diff
		return true, nil
	}
	// Exit code 0 means no diff
	return false, nil
}
