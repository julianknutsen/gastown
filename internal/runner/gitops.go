package runner

import (
	"fmt"
	"strings"
)

// GitOps provides git operations using a Runner.
// This allows the same git operations to work locally or remotely.
type GitOps struct {
	runner Runner
}

// NewGitOps creates a new GitOps with the given runner.
func NewGitOps(r Runner) *GitOps {
	return &GitOps{runner: r}
}

// Fetch fetches from a remote.
func (g *GitOps) Fetch(repoDir, remote string) error {
	return g.runner.Run(repoDir, "git", "fetch", remote)
}

// WorktreeAdd creates a new worktree with a new branch.
func (g *GitOps) WorktreeAdd(repoDir, path, branch, startPoint string) error {
	return g.runner.Run(repoDir, "git", "worktree", "add", "-b", branch, path, startPoint)
}

// WorktreeRemove removes a worktree.
func (g *GitOps) WorktreeRemove(repoDir, path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	return g.runner.Run(repoDir, "git", args...)
}

// WorktreePrune prunes stale worktree entries.
func (g *GitOps) WorktreePrune(repoDir string) error {
	return g.runner.Run(repoDir, "git", "worktree", "prune")
}

// CurrentBranch returns the current branch name.
func (g *GitOps) CurrentBranch(repoDir string) (string, error) {
	out, err := g.runner.Output(repoDir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Clone clones a repository.
func (g *GitOps) Clone(url, destPath string) error {
	return g.runner.Run("", "git", "clone", url, destPath)
}

// CloneWithBranch clones a repository and checks out a specific branch.
func (g *GitOps) CloneWithBranch(url, destPath, branch string) error {
	return g.runner.Run("", "git", "clone", "-b", branch, url, destPath)
}

// ListBranches lists branches matching a pattern.
func (g *GitOps) ListBranches(repoDir, pattern string) ([]string, error) {
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
func (g *GitOps) DeleteBranch(repoDir, branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	return g.runner.Run(repoDir, "git", "branch", flag, branch)
}

// CheckUncommittedWork checks for uncommitted changes.
// Returns (hasChanges, stashCount, unpushedCount, error)
func (g *GitOps) CheckUncommittedWork(repoDir string) (bool, int, int, error) {
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
func (g *GitOps) CountCommitsBehind(repoDir, ref string) (int, error) {
	out, err := g.runner.Output(repoDir, "git", "rev-list", "--count", "HEAD.."+ref)
	if err != nil {
		return 0, err
	}
	var count int
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &count)
	return count, nil
}
