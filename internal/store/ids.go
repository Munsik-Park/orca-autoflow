package store

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ErrAmbiguousRef is returned when a short-ref prefix matches multiple run IDs.
var ErrAmbiguousRef = errors.New("ambiguous short ref: matches multiple runs")

// ErrRefNotFound is returned when a short-ref matches no known run.
var ErrRefNotFound = errors.New("short ref not found")

// NewRunID generates a new UUIDv7-based run identifier.
// UUIDv7 provides monotonically increasing IDs suitable for B-tree indexes.
func NewRunID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generate UUIDv7: %w", err)
	}
	return id.String(), nil
}

// ShortRef converts a full UUID run ID to the human-readable short reference
// format "run_<first8hexchars>".
// Example: "0193e5f4-ab12-7abc-8def-1234567890ab" → "run_0193e5f4"
func ShortRef(id string) string {
	compact := strings.ReplaceAll(id, "-", "")
	if len(compact) < 8 {
		return "run_" + compact
	}
	return "run_" + compact[:8]
}

// ResolveShortRef looks up a run ID by its short-ref prefix in the store.
// It returns ErrAmbiguousRef if more than one run matches, ErrRefNotFound if none.
func (s *Store) ResolveShortRef(prefix string) (string, error) {
	// Normalize: strip the "run_" prefix if present for raw hex matching.
	hex := strings.TrimPrefix(prefix, "run_")

	rows, err := s.db.Query(
		`SELECT id FROM runs WHERE replace(id, '-', '') LIKE ? || '%'`,
		hex,
	)
	if err != nil {
		return "", fmt.Errorf("query short ref: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", fmt.Errorf("scan run id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterate short ref results: %w", err)
	}

	switch len(ids) {
	case 0:
		return "", ErrRefNotFound
	case 1:
		return ids[0], nil
	default:
		return "", fmt.Errorf("%w: %s", ErrAmbiguousRef, prefix)
	}
}
