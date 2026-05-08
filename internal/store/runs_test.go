package store_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/orca-cli/orca/internal/state"
	"github.com/orca-cli/orca/internal/store"
)

// openTestStore opens a fresh in-memory store and inserts a seed pod so runs
// can satisfy the FK constraint.
func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	_, err = s.DB().ExecContext(context.Background(),
		`INSERT INTO pods(id, name, base_branch, adapter) VALUES ('pod1','mypod','main','claude-code')`)
	require.NoError(t, err)
	return s
}

func makeRun(t *testing.T, prompt string) store.Run {
	t.Helper()
	id, err := store.NewRunID()
	require.NoError(t, err)
	return store.Run{
		ID:       id,
		ShortRef: store.ShortRef(id),
		PodID:    "pod1",
		Status:   state.Queued,
		Prompt:   prompt,
	}
}

// TestRun_RoundTrip verifies Create followed by GetByID returns the same run.
func TestRun_RoundTrip(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)
	ctx := context.Background()

	r := makeRun(t, "fix the login bug")
	require.NoError(t, s.Create(ctx, r))

	got, err := s.GetByID(ctx, r.ID)
	require.NoError(t, err)

	assert.Equal(t, r.ID, got.ID)
	assert.Equal(t, r.ShortRef, got.ShortRef)
	assert.Equal(t, r.Prompt, got.Prompt)
	assert.Equal(t, state.Queued, got.Status)
}

// TestRun_GetByID_NotFound verifies ErrRunNotFound for unknown IDs.
func TestRun_GetByID_NotFound(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)

	_, err := s.GetByID(context.Background(), "00000000-0000-0000-0000-000000000000")
	require.ErrorIs(t, err, store.ErrRunNotFound)
}

// TestRun_List_FilterByStatus verifies that List returns only runs matching the filter.
func TestRun_List_FilterByStatus(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)
	ctx := context.Background()

	r1 := makeRun(t, "task 1")
	r1.Status = state.Queued
	require.NoError(t, s.Create(ctx, r1))

	r2 := makeRun(t, "task 2")
	r2.Status = state.Ready
	require.NoError(t, s.Create(ctx, r2))

	r3 := makeRun(t, "task 3")
	r3.Status = state.Ready
	require.NoError(t, s.Create(ctx, r3))

	readyRuns, err := s.List(ctx, store.Filter{Status: state.Ready})
	require.NoError(t, err)
	require.Len(t, readyRuns, 2, "exactly 2 ready runs must be returned")

	for _, r := range readyRuns {
		assert.Equal(t, state.Ready, r.Status)
	}

	queuedRuns, err := s.List(ctx, store.Filter{Status: state.Queued})
	require.NoError(t, err)
	require.Len(t, queuedRuns, 1)
	assert.Equal(t, r1.ID, queuedRuns[0].ID)
}

// TestRun_UpdateStatus_BumpsUpdatedAt verifies that UpdateStatus advances updated_at.
func TestRun_UpdateStatus_BumpsUpdatedAt(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)
	ctx := context.Background()

	r := makeRun(t, "bump timestamp test")
	require.NoError(t, s.Create(ctx, r))

	// SQLite CURRENT_TIMESTAMP resolution is 1 second. To test that UpdateStatus
	// changes updated_at we first pin it to a fixed past value, read that as
	// "before", then call UpdateStatus and assert the value changed.
	_, err := s.DB().ExecContext(ctx,
		`UPDATE runs SET updated_at = '2020-01-01T00:00:00Z' WHERE id = ?`, r.ID)
	require.NoError(t, err)

	before, err := s.GetByID(ctx, r.ID)
	require.NoError(t, err)

	require.NoError(t, s.UpdateStatus(ctx, r.ID, state.Running))

	after, err := s.GetByID(ctx, r.ID)
	require.NoError(t, err)

	assert.Equal(t, state.Running, after.Status, "status must be updated")
	assert.True(t, after.UpdatedAt.After(before.UpdatedAt),
		"updated_at must advance: before=%v after=%v", before.UpdatedAt, after.UpdatedAt)
}

// TestRun_UpdateStatus_NotFound verifies ErrRunNotFound for unknown IDs.
func TestRun_UpdateStatus_NotFound(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)

	err := s.UpdateStatus(context.Background(), "00000000-0000-0000-0000-000000000000", state.Running)
	require.ErrorIs(t, err, store.ErrRunNotFound)
}

// TestRun_UpdateStatus_Concurrent verifies two goroutines updating the same run
// in WAL mode don't produce "database is locked".
// NOTE: :memory: doesn't use WAL, so this test uses a file-based DB.
func TestRun_UpdateStatus_Concurrent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	s, err := store.Open(dir + "/orca.db")
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	_, err = s.DB().ExecContext(ctx,
		`INSERT INTO pods(id, name, base_branch, adapter) VALUES ('pod1','p','main','claude-code')`)
	require.NoError(t, err)

	r := makeRun(t, "concurrent update test")
	require.NoError(t, s.Create(ctx, r))

	statuses := []state.Status{state.Running, state.Blocked}
	var wg sync.WaitGroup
	errs := make([]error, len(statuses))

	for i, st := range statuses {
		i, st := i, st
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = s.UpdateStatus(ctx, r.ID, st)
		}()
	}
	wg.Wait()

	for i, err := range errs {
		assert.NoError(t, err, "goroutine %d must not get database is locked", i)
	}
}

// TestShortRef_Format verifies ShortRef produces the expected prefix format.
func TestShortRef_Format(t *testing.T) {
	t.Parallel()

	id := "0193e5f4-ab12-7abc-8def-1234567890ab"
	ref := store.ShortRef(id)
	assert.Equal(t, "run_0193e5f4", ref)
}

// TestNewRunID_Unique verifies two consecutive IDs are different.
func TestNewRunID_Unique(t *testing.T) {
	t.Parallel()

	id1, err := store.NewRunID()
	require.NoError(t, err)

	id2, err := store.NewRunID()
	require.NoError(t, err)

	assert.NotEqual(t, id1, id2, "consecutive UUIDv7 IDs must be unique")
}
