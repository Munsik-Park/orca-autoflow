// Package testutil provides helpers for writing tests in orca.
package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// realPath resolves symlinks so paths on macOS (/var → /private/var) are canonical.
func realPath(p string) string {
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		return p
	}
	return r
}

// NewRepo creates a temporary git repository with a single empty commit on
// branch "main". It returns the absolute path to the repository root.
//
// The repository is created in t.TempDir() and cleaned up automatically when
// the test ends. user.email and user.name are configured locally so the test
// does not depend on the global git config.
func NewRepo(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	cmds := [][]string{
		{"git", "init", "-b", "main", root},
		{"git", "-C", root, "config", "user.email", "test@orca.local"},
		{"git", "-C", root, "config", "user.name", "Orca Test"},
		{"git", "-C", root, "commit", "--allow-empty", "-m", "init"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git fixture setup failed [%v]: %s", args, out)
		}
	}

	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("resolve abs path: %v", err)
	}

	return realPath(abs)
}

// NewEmptyRepo creates a temporary git repository with NO commits.
// Use this to test behavior when the base branch has no history.
func NewEmptyRepo(t *testing.T) string {
	t.Helper()

	root, err := os.MkdirTemp("", "orca-empty-repo-*")
	if err != nil {
		t.Fatalf("mkdirtemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })

	cmd := exec.Command("git", "init", "-b", "main", root)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git init empty repo: %s", out)
	}

	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("resolve abs path: %v", err)
	}

	return realPath(abs)
}
