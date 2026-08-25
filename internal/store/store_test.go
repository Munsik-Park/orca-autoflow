package store_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Munsik-Park/orca-autoflow/internal/store"
)

// TestOpen_Idempotent verifies that calling Open twice on the same path does not fail.
func TestOpen_Idempotent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "orca.db")

	s1, err := store.Open(path)
	require.NoError(t, err, "first Open must succeed")
	require.NoError(t, s1.Close())

	s2, err := store.Open(path)
	require.NoError(t, err, "second Open (re-migration) must succeed")
	require.NoError(t, s2.Close())
}

// TestOpen_Memory verifies that Open(":memory:") works for in-process tests.
func TestOpen_Memory(t *testing.T) {
	t.Parallel()

	s, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	// Probe DB is alive.
	db := s.DB()
	require.NotNil(t, db)
	var n int
	require.NoError(t, db.QueryRow("SELECT 1").Scan(&n))
	assert.Equal(t, 1, n)
}

// TestSchema_Tables verifies the runs table has the expected columns.
func TestSchema_Tables(t *testing.T) {
	t.Parallel()

	s, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	// PRAGMA table_info returns one row per column.
	rows, err := s.DB().Query("PRAGMA table_info(runs)")
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	var cols []string
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dfltValue, pk interface{}
		require.NoError(t, rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk))
		cols = append(cols, name)
	}
	require.NoError(t, rows.Err())

	required := []string{"id", "short_ref", "pod_id", "status", "prompt", "created_at", "updated_at"}
	for _, col := range required {
		assert.Contains(t, cols, col, "runs table must have column %q", col)
	}
}

// TestSchema_FTS5Available verifies that the logs_fts virtual table was created
// and that inserting a row into logs propagates to FTS5 search.
func TestSchema_FTS5Available(t *testing.T) {
	t.Parallel()

	s, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	db := s.DB()

	// logs_fts must exist in sqlite_master.
	var name string
	err = db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='logs_fts'`).Scan(&name)
	require.NoError(t, err, "logs_fts virtual table must exist")
	assert.Equal(t, "logs_fts", name)

	// Insert a pod + run so we can insert a log entry (FK constraint).
	_, err = db.Exec(`INSERT INTO pods(id, name, base_branch, adapter) VALUES ('p1','mypod','main','claude-code')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO runs(id, short_ref, pod_id, prompt) VALUES ('r1','run_aabbccdd','p1','fix the bug')`)
	require.NoError(t, err)

	// Insert a log; the trigger logs_ai should push it to FTS5.
	_, err = db.Exec(`INSERT INTO logs(run_id, message) VALUES ('r1', 'all tests passed')`)
	require.NoError(t, err)

	// Query FTS5 — must find the inserted row.
	var count int
	err = db.QueryRow(`SELECT count(*) FROM logs_fts WHERE message MATCH 'tests'`).Scan(&count)
	require.NoError(t, err, "FTS5 query must succeed — confirms build flag fts5 is active")
	assert.Equal(t, 1, count, "FTS5 search for 'tests' must find 1 row")
}
