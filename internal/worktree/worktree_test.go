package worktree_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Munsik-Park/orca-autoflow/internal/testutil"
	"github.com/Munsik-Park/orca-autoflow/internal/worktree"
)

// TestCreateForRun_Happy verifies that CreateForRun creates a valid worktree.
func TestCreateForRun_Happy(t *testing.T) {
	t.Parallel()

	root := testutil.NewRepo(t)
	runID := "run_abc12345"

	path, err := worktree.CreateForRun(root, runID, "main")
	require.NoError(t, err)

	// Worktree directory must exist.
	info, err := os.Stat(path)
	require.NoError(t, err, "worktree directory must exist at %s", path)
	assert.True(t, info.IsDir(), "worktree path must be a directory")

	// Git must consider it a valid repository.
	cmd := exec.Command("git", "-C", path, "rev-parse", "HEAD")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git rev-parse HEAD must succeed in worktree: %s", out)

	// Verify expected path structure: <root>/.orca/runs/<runID>/
	expected := filepath.Join(root, ".orca", "runs", runID)
	assert.Equal(t, expected, path)
}

// TestCreateForRun_EmptyBase verifies ErrEmptyBase for a repo with no commits.
func TestCreateForRun_EmptyBase(t *testing.T) {
	t.Parallel()

	root := testutil.NewEmptyRepo(t)
	_, err := worktree.CreateForRun(root, "run_empty", "main")

	require.ErrorIs(t, err, worktree.ErrEmptyBase,
		"CreateForRun on empty repo must return ErrEmptyBase")
}

// TestArchive_Shipped verifies that Archive moves the worktree and removes it
// from git's worktree list.
func TestArchive_Shipped(t *testing.T) {
	t.Parallel()

	root := testutil.NewRepo(t)
	runID := "run_ship1234"

	_, err := worktree.CreateForRun(root, runID, "main")
	require.NoError(t, err)

	err = worktree.Archive(root, runID, "shipped")
	require.NoError(t, err)

	// Active path must no longer exist.
	active := filepath.Join(root, ".orca", "runs", runID)
	_, err = os.Stat(active)
	assert.True(t, os.IsNotExist(err), "active worktree must be gone after Archive")

	// Archived path must exist.
	archived := filepath.Join(root, ".orca", "runs", "_shipped", runID)
	info, err := os.Stat(archived)
	require.NoError(t, err, "archived worktree must exist at %s", archived)
	assert.True(t, info.IsDir())

	// git worktree list must not contain the runID path.
	cmd := exec.Command("git", "-C", root, "worktree", "list")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err)
	assert.False(t, strings.Contains(string(out), runID),
		"git worktree list must not contain %s after Archive\nlist:\n%s", runID, out)
}

// TestArchive_NotFound verifies ErrNotFound for a non-existent worktree.
func TestArchive_NotFound(t *testing.T) {
	t.Parallel()

	root := testutil.NewRepo(t)
	err := worktree.Archive(root, "run_notexist", "shipped")
	require.ErrorIs(t, err, worktree.ErrNotFound)
}

// TestPath_ReturnsExpectedStructure verifies Path returns the correct absolute path.
func TestPath_ReturnsExpectedStructure(t *testing.T) {
	t.Parallel()

	root := testutil.NewRepo(t)
	runID := "run_pathtest"

	got := worktree.Path(root, runID)
	expected := filepath.Join(root, ".orca", "runs", runID)
	assert.Equal(t, expected, got)
}
