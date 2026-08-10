package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ConteMan/muxio/internal/record"
	"github.com/ConteMan/muxio/internal/run"
)

// RunSummary is one row of run history.
type RunSummary struct {
	ID         int64
	SourceID   int64
	SourceName string
	Trigger    string
	Status     run.Status
	StartedAt  string
	FinishedAt string
	Counts     run.Counts
	Attempt    int
	LastError  string
}

// EventRecord is one stored event with its identity.
type EventRecord struct {
	ID         int64
	RunID      int64
	Level      string
	Message    string
	Detail     string
	OccurredAt string
}

// ErrRunNotFound reports a run id with no matching row.
var ErrRunNotFound = errors.New("run not found")

// StartRun opens a run in the running state and returns its id.
func (s *Store) StartRun(ctx context.Context, sourceID int64, trigger string) (int64, error) {
	now := record.Now()
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO runs (source_id, trigger, status, started_at, heartbeat_at)
		VALUES (?, ?, ?, ?, ?)`,
		sourceID, trigger, string(run.Running), now, now)
	if err != nil {
		return 0, fmt.Errorf("start run: %w", err)
	}

	runID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read run id: %w", err)
	}
	return runID, nil
}

// Heartbeat refreshes the liveness marker of a running run.
func (s *Store) Heartbeat(ctx context.Context, runID int64) error {
	if _, err := s.db.ExecContext(ctx,
		`UPDATE runs SET heartbeat_at = ? WHERE id = ? AND status = ?`,
		record.Now(), runID, string(run.Running)); err != nil {
		return fmt.Errorf("heartbeat run %d: %w", runID, err)
	}
	return nil
}

// FinishRun moves a run to a terminal state. Counts are already stored by the
// capture and failure paths, so only the outcome is written here.
func (s *Store) FinishRun(ctx context.Context, runID int64, status run.Status, lastError string) error {
	if !status.IsTerminal() {
		return fmt.Errorf("cannot finish run %d in non-terminal status %q", runID, status)
	}

	var storedError any
	if lastError != "" {
		storedError = lastError
	}

	if _, err := s.db.ExecContext(ctx, `
		UPDATE runs SET status = ?, finished_at = ?, heartbeat_at = ?, last_error = ?
		WHERE id = ?`,
		string(status), record.Now(), record.Now(), storedError, runID); err != nil {
		return fmt.Errorf("finish run %d: %w", runID, err)
	}
	return nil
}

// AppendEvent stores one explanation of what happened during a run.
func (s *Store) AppendEvent(ctx context.Context, runID int64, event run.Event) error {
	detail, err := encodeDetail(event.Detail)
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO run_events (run_id, level, message, detail_json, occurred_at)
		VALUES (?, ?, ?, ?, ?)`,
		runID, event.Level, event.Message, detail, record.Now()); err != nil {
		return fmt.Errorf("append event to run %d: %w", runID, err)
	}
	return nil
}

// RecordFailure counts a failed input line and, when event is non-nil, stores
// the reason. Both happen in one transaction so the count never disagrees with
// the recorded explanations.
func (s *Store) RecordFailure(ctx context.Context, runID int64, event *run.Event) error {
	var detail string
	if event != nil {
		encoded, err := encodeDetail(event.Detail)
		if err != nil {
			return err
		}
		detail = encoded
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin failure record: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`UPDATE runs SET failed_count = failed_count + 1 WHERE id = ?`, runID); err != nil {
		return fmt.Errorf("count failure on run %d: %w", runID, err)
	}
	if event != nil {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO run_events (run_id, level, message, detail_json, occurred_at)
			VALUES (?, ?, ?, ?, ?)`,
			runID, event.Level, event.Message, detail, record.Now()); err != nil {
			return fmt.Errorf("record failure on run %d: %w", runID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit failure record: %w", err)
	}
	return nil
}

// RecoverStaleRuns marks non-terminal runs whose heartbeat has gone stale as
// interrupted. The heartbeat is what distinguishes an abandoned run from one
// that another process is still working on.
func (s *Store) RecoverStaleRuns(ctx context.Context) (int, error) {
	cutoff := time.Now().UTC().Add(-run.StaleAfter).Format(record.TimeFormat)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin recovery: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
		SELECT id FROM runs
		WHERE status IN (?, ?) AND heartbeat_at < ?`,
		string(run.Running), string(run.Queued), cutoff)
	if err != nil {
		return 0, fmt.Errorf("find stale runs: %w", err)
	}

	var stale []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return 0, fmt.Errorf("scan stale run: %w", err)
		}
		stale = append(stale, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, fmt.Errorf("iterate stale runs: %w", err)
	}
	_ = rows.Close()

	now := record.Now()
	for _, id := range stale {
		if _, err := tx.ExecContext(ctx, `
			UPDATE runs SET status = ?, finished_at = ?, last_error = ?
			WHERE id = ?`,
			string(run.Interrupted), now,
			"process exited without finishing this run", id); err != nil {
			return 0, fmt.Errorf("interrupt run %d: %w", id, err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO run_events (run_id, level, message, detail_json, occurred_at)
			VALUES (?, ?, ?, ?, ?)`,
			id, run.LevelWarn,
			"run marked interrupted because its heartbeat went stale",
			"{}", now); err != nil {
			return 0, fmt.Errorf("record interruption of run %d: %w", id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit recovery: %w", err)
	}
	return len(stale), nil
}

// PurgeExpiredEvents drops events past the retention window. Runs are kept.
func (s *Store) PurgeExpiredEvents(ctx context.Context) (int64, error) {
	cutoff := time.Now().UTC().Add(-run.EventRetention).Format(record.TimeFormat)

	result, err := s.db.ExecContext(ctx,
		`DELETE FROM run_events WHERE occurred_at < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("purge expired events: %w", err)
	}
	removed, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("inspect purge result: %w", err)
	}
	return removed, nil
}

// ListRuns returns recent runs, newest first.
func (s *Store) ListRuns(ctx context.Context, limit int) ([]RunSummary, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, r.source_id, s.name, r.trigger, r.status, r.started_at,
		       COALESCE(r.finished_at, ''), r.imported_count, r.duplicate_count,
		       r.failed_count, r.attempt, COALESCE(r.last_error, '')
		FROM runs r
		JOIN sources s ON s.id = r.source_id
		ORDER BY r.id DESC
		LIMIT ?`, limit)
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

// GetRun returns one run with its events in chronological order.
func (s *Store) GetRun(ctx context.Context, runID int64) (RunSummary, []EventRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT r.id, r.source_id, s.name, r.trigger, r.status, r.started_at,
		       COALESCE(r.finished_at, ''), r.imported_count, r.duplicate_count,
		       r.failed_count, r.attempt, COALESCE(r.last_error, '')
		FROM runs r
		JOIN sources s ON s.id = r.source_id
		WHERE r.id = ?`, runID)

	summary, err := scanRunSummary(row)
	if errors.Is(err, sql.ErrNoRows) {
		return RunSummary{}, nil, fmt.Errorf("%w: %d", ErrRunNotFound, runID)
	}
	if err != nil {
		return RunSummary{}, nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, run_id, level, message, detail_json, occurred_at
		FROM run_events WHERE run_id = ? ORDER BY id`, runID)
	if err != nil {
		return RunSummary{}, nil, fmt.Errorf("read events of run %d: %w", runID, err)
	}
	defer func() { _ = rows.Close() }()

	var events []EventRecord
	for rows.Next() {
		var event EventRecord
		if err := rows.Scan(&event.ID, &event.RunID, &event.Level,
			&event.Message, &event.Detail, &event.OccurredAt); err != nil {
			return RunSummary{}, nil, fmt.Errorf("scan event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return RunSummary{}, nil, fmt.Errorf("iterate events: %w", err)
	}
	return summary, events, nil
}

// CountEvents reports how many events a run has stored.
func (s *Store) CountEvents(ctx context.Context, runID int64) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM run_events WHERE run_id = ?`, runID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count events: %w", err)
	}
	return count, nil
}

// scanner covers both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanRunSummary(src scanner) (RunSummary, error) {
	var summary RunSummary
	err := src.Scan(&summary.ID, &summary.SourceID, &summary.SourceName,
		&summary.Trigger, &summary.Status, &summary.StartedAt, &summary.FinishedAt,
		&summary.Counts.Imported, &summary.Counts.Duplicate, &summary.Counts.Failed,
		&summary.Attempt, &summary.LastError)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RunSummary{}, err
		}
		return RunSummary{}, fmt.Errorf("scan run: %w", err)
	}
	return summary, nil
}

func encodeDetail(detail map[string]any) (string, error) {
	if len(detail) == 0 {
		return "{}", nil
	}
	encoded, err := json.Marshal(detail)
	if err != nil {
		return "", fmt.Errorf("encode event detail: %w", err)
	}
	return string(encoded), nil
}
