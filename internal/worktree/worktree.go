package worktree

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrEmptyBase is returned when CreateForRun is called on a branch with no commits.
var ErrEmptyBase = errors.New("base branch has no commits")

// ErrNotFound is returned when Archive or Path is called with a runID that has
// no active worktree under .orca/runs/.
var ErrNotFound = errors.New("worktree not found")

// repoRoot resolves the absolute path of the primary worktree's root by running
// git rev-parse --show-toplevel. It never reads os.Getwd() to avoid operating
// from inside a worktree's subdirectory.
// On macOS, symlinks like /var → /private/var are resolved to the real path.
func repoRoot(root string) (string, error) {
	out, err := git(root, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("resolve repo root: %w", err)
	}
	p := strings.TrimSpace(out)
	// Resolve symlinks so all paths are canonical (needed on macOS /var → /private/var).
	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		return p, nil // fall back to unresolved if EvalSymlinks fails
	}
	return real, nil
}

// activePath returns the absolute path for an active worktree:
// <repo>/.orca/runs/<runID>/
func activePath(root, runID string) string {
	return filepath.Join(root, ".orca", "runs", runID)
}

// archivedPath returns the path for an archived worktree:
// <repo>/.orca/runs/_<kind>/<runID>/
func archivedPath(root, runID, kind string) string {
	return filepath.Join(root, ".orca", "runs", "_"+kind, runID)
}

// CreateForRun creates a new git worktree at <repo>/.orca/runs/<runID>/
// tracking baseBranch. Returns the absolute path of the new worktree.
//
// Returns ErrEmptyBase if baseBranch has no commits (git worktree add would fail).
func CreateForRun(root, runID, baseBranch string) (string, error) {
	abs, err := repoRoot(root)
	if err != nil {
		return "", err
	}

	// Detect empty branch: if git rev-parse <branch> fails, there are no commits.
	if _, err := git(abs, "rev-parse", baseBranch); err != nil {
		return "", ErrEmptyBase
	}

	dest := activePath(abs, runID)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", fmt.Errorf("create worktree parent dir: %w", err)
	}

	// Create a new branch for this run (named after the runID) based on baseBranch.
	// We cannot check out baseBranch directly because it is already checked out by
	// the primary worktree; git forbids two worktrees on the same branch.
	runBranch := "orca/" + runID
	if _, err := git(abs, "worktree", "add", "-b", runBranch, dest, baseBranch); err != nil {
		return "", fmt.Errorf("git worktree add: %w", err)
	}

	return dest, nil
}

// Archive moves an active worktree to .orca/runs/_<kind>/<runID>/ and removes
// it from git's worktree registry. kind must be "shipped" or "killed".
//
// Returns ErrNotFound if no active worktree exists for runID.
func Archive(root, runID, kind string) error {
	abs, err := repoRoot(root)
	if err != nil {
		return err
	}

	src := activePath(abs, runID)
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return ErrNotFound
	}

	dest := archivedPath(abs, runID, kind)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create archive dir: %w", err)
	}

	// Move the directory first so data is preserved.
	if err := os.Rename(src, dest); err != nil {
		return fmt.Errorf("move worktree to archive: %w", err)
	}

	// Prune stale worktree entries now that the directory is gone from its
	// original location. This removes it from `git worktree list`.
	if _, err := git(abs, "worktree", "prune"); err != nil {
		// Non-fatal: the worktree is already moved; the prune failure only
		// leaves a stale .git/worktrees entry that will be cleaned up on next prune.
		_ = err
	}

	return nil
}

// Path returns the absolute path of the active worktree for runID.
// It does not check whether the path exists on disk.
func Path(root, runID string) string {
	abs, err := repoRoot(root)
	if err != nil {
		// Best-effort: return a constructed path even if repo root resolution fails.
		return activePath(root, runID)
	}
	return activePath(abs, runID)
}

// git runs a git subcommand in the given working directory and returns combined output.
func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\noutput: %s", strings.Join(args, " "), err, out)
	}
	return string(out), nil
}
