package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ConteMan/muxio/internal/record"
	"github.com/ConteMan/muxio/internal/run"
)

func TestRunLifecycle(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	sourceID := ensureSource(t, store, "notes")

	runID, err := store.StartRun(ctx, sourceID, run.TriggerManual)
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	summary, _, err := store.GetRun(ctx, runID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if summary.Status != run.Running {
		t.Fatalf("status = %q, want %q", summary.Status, run.Running)
	}
	if summary.SourceName != "notes" {
		t.Fatalf("source name = %q", summary.SourceName)
	}
	if summary.FinishedAt != "" {
		t.Fatalf("finished_at = %q, want empty while running", summary.FinishedAt)
	}

	if err := store.FinishRun(ctx, runID, run.Succeeded, ""); err != nil {
		t.Fatalf("FinishRun: %v", err)
	}

	summary, _, err = store.GetRun(ctx, runID)
	if err != nil {
		t.Fatalf("GetRun after finish: %v", err)
	}
	if summary.Status != run.Succeeded || summary.FinishedAt == "" {
		t.Fatalf("summary = %+v, want a finished succeeded run", summary)
	}
}

func TestFinishRunRejectsNonTerminalStatus(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	sourceID := ensureSource(t, store, "notes")
	runID := startRun(t, store, sourceID)

	if err := store.FinishRun(ctx, runID, run.Running, ""); err == nil {
		t.Fatal("a run was allowed to finish in a non-terminal status")
	}
}

// Counts must come from what actually landed, not from a separate tally that
// could drift away from the captures.
func TestCaptureCountsAreRecordedOnTheRun(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	sourceID := ensureSource(t, store, "notes")
	runID := startRun(t, store, sourceID)

	first := normalize(t, record.Record{ExternalID: "note-1", Body: "one"})
	second := normalize(t, record.Record{ExternalID: "note-2", Body: "two"})

	for _, rec := range []record.Record{first, second, first} {
		if _, err := store.AddCapture(ctx, sourceID, runID, rec); err != nil {
			t.Fatalf("AddCapture: %v", err)
		}
	}
	if err := store.RecordFailure(ctx, runID, &run.Event{
		Level: run.LevelError, Message: "line 9 rejected",
	}); err != nil {
		t.Fatalf("RecordFailure: %v", err)
	}

	summary, events, err := store.GetRun(ctx, runID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	want := run.Counts{Imported: 2, Duplicate: 1, Failed: 1}
	if summary.Counts != want {
		t.Fatalf("counts = %+v, want %+v", summary.Counts, want)
	}
	if len(events) != 1 || events[0].Message != "line 9 rejected" {
		t.Fatalf("events = %+v", events)
	}
}

func TestCapturesAreLinkedToTheirRun(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	sourceID := ensureSource(t, store, "notes")
	runID := startRun(t, store, sourceID)

	rec := normalize(t, record.Record{ExternalID: "note-1", Body: "one"})
	if _, err := store.AddCapture(ctx, sourceID, runID, rec); err != nil {
		t.Fatalf("AddCapture: %v", err)
	}

	var linked int
	if err := store.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM captures WHERE run_id = ?`, runID).Scan(&linked); err != nil {
		t.Fatalf("count linked captures: %v", err)
	}
	if linked != 1 {
		t.Fatalf("captures linked to run = %d, want 1", linked)
	}
}

// A run abandoned by a killed process must be swept up, but only after its
// heartbeat has gone stale.
func TestRecoverStaleRunsOnlyTouchesStaleRuns(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	sourceID := ensureSource(t, store, "notes")

	fresh := startRun(t, store, sourceID)
	stale := startRun(t, store, sourceID)

	// Backdate the second run's heartbeat past the staleness threshold.
	backdated := time.Now().UTC().Add(-2 * run.StaleAfter).Format(record.TimeFormat)
	if _, err := store.db.ExecContext(ctx,
		`UPDATE runs SET heartbeat_at = ? WHERE id = ?`, backdated, stale); err != nil {
		t.Fatalf("backdate heartbeat: %v", err)
	}

	recovered, err := store.RecoverStaleRuns(ctx)
	if err != nil {
		t.Fatalf("RecoverStaleRuns: %v", err)
	}
	if recovered != 1 {
		t.Fatalf("recovered = %d, want 1", recovered)
	}

	staleSummary, events, err := store.GetRun(ctx, stale)
	if err != nil {
		t.Fatalf("GetRun stale: %v", err)
	}
	if staleSummary.Status != run.Interrupted {
		t.Fatalf("stale run status = %q, want %q", staleSummary.Status, run.Interrupted)
	}
	if len(events) != 1 {
		t.Fatalf("stale run has %d events, want an explanation", len(events))
	}

	freshSummary, _, err := store.GetRun(ctx, fresh)
	if err != nil {
		t.Fatalf("GetRun fresh: %v", err)
	}
	if freshSummary.Status != run.Running {
		t.Fatalf("a live run was swept up: status = %q", freshSummary.Status)
	}
}

func TestHeartbeatKeepsARunAlive(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	sourceID := ensureSource(t, store, "notes")
	runID := startRun(t, store, sourceID)

	backdated := time.Now().UTC().Add(-2 * run.StaleAfter).Format(record.TimeFormat)
	if _, err := store.db.ExecContext(ctx,
		`UPDATE runs SET heartbeat_at = ? WHERE id = ?`, backdated, runID); err != nil {
		t.Fatalf("backdate heartbeat: %v", err)
	}
	if err := store.Heartbeat(ctx, runID); err != nil {
		t.Fatalf("Heartbeat: %v", err)
	}

	recovered, err := store.RecoverStaleRuns(ctx)
	if err != nil {
		t.Fatalf("RecoverStaleRuns: %v", err)
	}
	if recovered != 0 {
		t.Fatalf("recovered = %d, want 0 after a fresh heartbeat", recovered)
	}
}

func TestPurgeExpiredEventsKeepsRuns(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	sourceID := ensureSource(t, store, "notes")
	runID := startRun(t, store, sourceID)

	if err := store.AppendEvent(ctx, runID, run.Event{
		Level: run.LevelInfo, Message: "recent",
	}); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}
	// Age one event past the retention window.
	expired := time.Now().UTC().Add(-2 * run.EventRetention).Format(record.TimeFormat)
	if _, err := store.db.ExecContext(ctx, `
		INSERT INTO run_events (run_id, level, message, detail_json, occurred_at)
		VALUES (?, ?, ?, '{}', ?)`,
		runID, run.LevelInfo, "ancient", expired); err != nil {
		t.Fatalf("seed expired event: %v", err)
	}

	removed, err := store.PurgeExpiredEvents(ctx)
	if err != nil {
		t.Fatalf("PurgeExpiredEvents: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}

	summary, events, err := store.GetRun(ctx, runID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if summary.ID != runID {
		t.Fatal("purging events removed the run itself")
	}
	if len(events) != 1 || events[0].Message != "recent" {
		t.Fatalf("events = %+v, want only the recent one", events)
	}
}

func TestListRunsIsNewestFirst(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	sourceID := ensureSource(t, store, "notes")

	first := startRun(t, store, sourceID)
	second := startRun(t, store, sourceID)

	summaries, err := store.ListRuns(ctx, 10)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("listed %d runs, want 2", len(summaries))
	}
	if summaries[0].ID != second || summaries[1].ID != first {
		t.Fatalf("order = [%d %d], want [%d %d]",
			summaries[0].ID, summaries[1].ID, second, first)
	}

	limited, err := store.ListRuns(ctx, 1)
	if err != nil {
		t.Fatalf("ListRuns with limit: %v", err)
	}
	if len(limited) != 1 || limited[0].ID != second {
		t.Fatalf("limited = %+v, want only the newest run", limited)
	}
}

func TestGetRunReportsMissingRun(t *testing.T) {
	_, _, err := openTestStore(t).GetRun(context.Background(), 4242)
	if !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("err = %v, want ErrRunNotFound", err)
	}
}

func startRun(t *testing.T, store *Store, sourceID int64) int64 {
	t.Helper()
	runID, err := store.StartRun(context.Background(), sourceID, run.TriggerManual)
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	return runID
}
