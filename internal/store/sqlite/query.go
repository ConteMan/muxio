package sqlite

import (
	"context"
	"fmt"
	"strings"
)

// SourceSummary is one configured source as read paths see it.
type SourceSummary struct {
	ID            int64
	Name          string
	ConnectorKind string
	Enabled       bool
	CreatedAt     string
	UpdatedAt     string
}

// Page bounds a listing. Cursor paging on a descending id keeps pages stable
// while new rows arrive, which offset paging does not.
type Page struct {
	Limit  int
	Before int64
}

func (p Page) limitOrDefault() int {
	if p.Limit <= 0 {
		return 20
	}
	return p.Limit
}

// ListSources returns sources newest first.
func (s *Store) ListSources(ctx context.Context, page Page) ([]SourceSummary, error) {
	query := `
		SELECT id, name, connector_kind, enabled, created_at, updated_at
		FROM sources`
	args := []any{}
	if page.Before > 0 {
		query += ` WHERE id < ?`
		args = append(args, page.Before)
	}
	query += ` ORDER BY id DESC LIMIT ?`
	args = append(args, page.limitOrDefault())

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var summaries []SourceSummary
	for rows.Next() {
		var summary SourceSummary
		if err := rows.Scan(&summary.ID, &summary.Name, &summary.ConnectorKind,
			&summary.Enabled, &summary.CreatedAt, &summary.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan source: %w", err)
		}
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sources: %w", err)
	}
	return summaries, nil
}

// RunFilter narrows a run listing.
type RunFilter struct {
	SourceID int64
}

// ListRunsPage returns runs newest first, optionally filtered by source.
func (s *Store) ListRunsPage(ctx context.Context, page Page, filter RunFilter) ([]RunSummary, error) {
	conditions := []string{}
	args := []any{}
	if page.Before > 0 {
		conditions = append(conditions, "r.id < ?")
		args = append(args, page.Before)
	}
	if filter.SourceID > 0 {
		conditions = append(conditions, "r.source_id = ?")
		args = append(args, filter.SourceID)
	}

	query := runSelect
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY r.id DESC LIMIT ?"
	args = append(args, page.limitOrDefault())

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var summaries []RunSummary
	for rows.Next() {
		summary, err := scanRunSummary(rows)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate runs: %w", err)
	}
	return summaries, nil
}

// ListRunEvents returns a page of events for one run, oldest first. Events read
// as a narrative, so ascending order is the useful one; the cursor still walks
// forward from the last id seen.
func (s *Store) ListRunEvents(ctx context.Context, runID int64, page Page) ([]EventRecord, error) {
	query := `
		SELECT id, run_id, level, message, detail_json, occurred_at
		FROM run_events WHERE run_id = ?`
	args := []any{runID}
	if page.Before > 0 {
		query += ` AND id > ?`
		args = append(args, page.Before)
	}
	query += ` ORDER BY id ASC LIMIT ?`
	args = append(args, page.limitOrDefault())

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list run events: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var events []EventRecord
	for rows.Next() {
		var event EventRecord
		if err := rows.Scan(&event.ID, &event.RunID, &event.Level,
			&event.Message, &event.Detail, &event.OccurredAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate run events: %w", err)
	}
	return events, nil
}

// RunExists reports whether a run id is known, so a listing can distinguish an
// unknown run from one that simply has no events yet.
func (s *Store) RunExists(ctx context.Context, runID int64) (bool, error) {
	var exists int
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM runs WHERE id = ?)`, runID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check run %d: %w", runID, err)
	}
	return exists == 1, nil
}

// Ping reports whether the database is reachable.
func (s *Store) Ping(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database unreachable: %w", err)
	}
	// A reachable file is not enough: the schema must also be readable.
	if _, err := s.SchemaVersion(ctx); err != nil {
		return err
	}
	return nil
}
