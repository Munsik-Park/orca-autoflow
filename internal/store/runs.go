package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Munsik-Park/orca-autoflow/internal/state"
)

// ErrRunNotFound is returned when a run ID does not exist in the database.
var ErrRunNotFound = errors.New("run not found")

// Run represents a single coding agent execution stored in the database.
type Run struct {
	ID           string
	ShortRef     string
	PodID        string
	Status       state.Status
	Prompt       string
	WorktreePath string
	Adapter      string
	Branch       string
	StartedAt    *time.Time
	CompletedAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Filter specifies optional criteria for listing runs.
type Filter struct {
	// PodID restricts results to runs belonging to the given pod (empty = all pods).
	PodID string
	// Status restricts results to runs with the given status (empty = all statuses).
	Status state.Status
	// Limit caps the number of results (0 = no limit).
	Limit int
}

// Create inserts a new Run into the database.
// r.ID and r.ShortRef must be populated by the caller (use NewRunID + ShortRef).
func (s *Store) Create(ctx context.Context, r Run) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO runs (id, short_ref, pod_id, status, prompt, worktree_path, adapter, branch)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.ShortRef, r.PodID, string(r.Status), r.Prompt,
		r.WorktreePath, r.Adapter, r.Branch,
	)
	if err != nil {
		return fmt.Errorf("create run: %w", err)
	}
	return nil
}

// GetByID retrieves a run by its full UUID.
// Returns ErrRunNotFound if no such run exists.
func (s *Store) GetByID(ctx context.Context, id string) (Run, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, short_ref, pod_id, status, prompt, worktree_path, adapter, branch,
		       started_at, completed_at, created_at, updated_at
		FROM runs WHERE id = ?`, id)

	return scanRun(row)
}

// List returns runs matching the given filter criteria.
func (s *Store) List(ctx context.Context, f Filter) ([]Run, error) {
	query := `
		SELECT id, short_ref, pod_id, status, prompt, worktree_path, adapter, branch,
		       started_at, completed_at, created_at, updated_at
		FROM runs WHERE 1=1`
	args := []interface{}{}

	if f.PodID != "" {
		query += " AND pod_id = ?"
		args = append(args, f.PodID)
	}
	if f.Status != "" {
		query += " AND status = ?"
		args = append(args, string(f.Status))
	}
	query += " ORDER BY created_at ASC"
	if f.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", f.Limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var runs []Run
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, r)
	}
	return runs, rows.Err()
}

// UpdateStatus sets a run's status and updates updated_at to the current time.
// Returns ErrRunNotFound if no run with that ID exists.
func (s *Store) UpdateStatus(ctx context.Context, id string, status state.Status) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE runs SET status = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		WHERE id = ?`,
		string(status), id,
	)
	if err != nil {
		return fmt.Errorf("update run status: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return ErrRunNotFound
	}
	return nil
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...interface{}) error
}

func scanRun(s scanner) (Run, error) {
	var r Run
	var status string
	var startedAt, completedAt sql.NullString
	var createdAt, updatedAt string

	err := s.Scan(
		&r.ID, &r.ShortRef, &r.PodID, &status, &r.Prompt,
		&r.WorktreePath, &r.Adapter, &r.Branch,
		&startedAt, &completedAt,
		&createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Run{}, ErrRunNotFound
	}
	if err != nil {
		return Run{}, fmt.Errorf("scan run: %w", err)
	}

	r.Status = state.Status(status)

	const layout = "2006-01-02T15:04:05Z"
	r.CreatedAt, _ = time.Parse(layout, createdAt)
	r.UpdatedAt, _ = time.Parse(layout, updatedAt)

	if startedAt.Valid {
		t, _ := time.Parse(layout, startedAt.String)
		r.StartedAt = &t
	}
	if completedAt.Valid {
		t, _ := time.Parse(layout, completedAt.String)
		r.CompletedAt = &t
	}

	return r, nil
}
