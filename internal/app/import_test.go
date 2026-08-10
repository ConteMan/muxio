package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/ConteMan/muxio/internal/record"
	"github.com/ConteMan/muxio/internal/run"
)

// fakeStore records what an import asked it to write.
type fakeStore struct {
	seen     map[string]bool
	captures []record.Record
	events   []run.Event

	runID       int64
	counts      run.Counts
	finalStatus run.Status
	finalError  string
	heartbeats  int
	recovered   int
	purged      int64

	failAfter int
	calls     int
}

func newFakeStore() *fakeStore {
	return &fakeStore{seen: make(map[string]bool), runID: 7, failAfter: -1}
}

func (f *fakeStore) EnsureSource(context.Context, string, string) (int64, error) { return 1, nil }

func (f *fakeStore) StartRun(context.Context, int64, string) (int64, error) { return f.runID, nil }

func (f *fakeStore) Heartbeat(context.Context, int64) error {
	f.heartbeats++
	return nil
}

func (f *fakeStore) FinishRun(_ context.Context, _ int64, status run.Status, lastError string) error {
	f.finalStatus = status
	f.finalError = lastError
	return nil
}

func (f *fakeStore) AppendEvent(_ context.Context, _ int64, event run.Event) error {
	f.events = append(f.events, event)
	return nil
}

func (f *fakeStore) RecordFailure(_ context.Context, _ int64, event *run.Event) error {
	f.counts.Failed++
	if event != nil {
		f.events = append(f.events, *event)
	}
	return nil
}

func (f *fakeStore) RecoverStaleRuns(context.Context) (int, error) { return f.recovered, nil }

func (f *fakeStore) PurgeExpiredEvents(context.Context, time.Duration) (int64, error) {
	return f.purged, nil
}

func (f *fakeStore) AddCapture(_ context.Context, _, _ int64, rec record.Record) (bool, error) {
	f.calls++
	if f.failAfter >= 0 && f.calls > f.failAfter {
		return false, errors.New("database is not writable")
	}

	contentHash, err := rec.ContentHash()
	if err != nil {
		return false, err
	}
	key := rec.ExternalID + "\x00" + contentHash
	if f.seen[key] {
		f.counts.Duplicate++
		return false, nil
	}
	f.seen[key] = true
	f.captures = append(f.captures, rec)
	f.counts.Imported++
	return true, nil
}

// messages returns every stored event message, for substring assertions.
func (f *fakeStore) messages() string {
	var builder strings.Builder
	for _, event := range f.events {
		builder.WriteString(event.Message)
		builder.WriteString("\n")
	}
	return builder.String()
}

func testOptions() Options {
	return Options{
		MaxBodyBytes:   record.DefaultMaxBodyBytes,
		EventRetention: run.DefaultEventRetention,
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func importFrom(t *testing.T, store *fakeStore, input string) (ImportResult, error) {
	t.Helper()
	return ImportJSONL(context.Background(), store, strings.NewReader(input),
		discardLogger(), "notes", testOptions())
}

func TestImportCountsOutcomes(t *testing.T) {
	input := strings.Join([]string{
		`{"external_id":"a","title":"A","body":"first"}`,
		`{"external_id":"b","body":"second"}`,
		`{"external_id":"a","title":"A","body":"first"}`, // duplicate
		``, // blank lines are skipped
		`{"external_id":"","body":"no identity"}`, // failed
		`not json`, // failed
	}, "\n")

	store := newFakeStore()
	result, err := importFrom(t, store, input)
	if err != nil {
		t.Fatalf("ImportJSONL: %v", err)
	}

	if result.Imported != 2 || result.Duplicate != 1 || result.Failed != 2 {
		t.Fatalf("result = %+v, want imported=2 duplicate=1 failed=2", result)
	}
	if result.RunID != store.runID {
		t.Fatalf("RunID = %d, want %d", result.RunID, store.runID)
	}
	// Line numbers must point at the real input lines, counting blanks.
	if !strings.Contains(store.messages(), "line 5 rejected") ||
		!strings.Contains(store.messages(), "line 6 rejected") {
		t.Fatalf("events = %q", store.messages())
	}
}

// Rejected lines are line-level failures. The run itself succeeded: it read
// everything it was given and committed everything valid.
func TestImportSucceedsDespiteRejectedLines(t *testing.T) {
	input := strings.Join([]string{
		`{"external_id":"","body":"broken"}`,
		`{"external_id":"good","body":"kept"}`,
	}, "\n")

	store := newFakeStore()
	result, err := importFrom(t, store, input)
	if err != nil {
		t.Fatalf("ImportJSONL: %v", err)
	}

	if result.Imported != 1 || result.Failed != 1 {
		t.Fatalf("result = %+v, want imported=1 failed=1", result)
	}
	if store.finalStatus != run.Succeeded {
		t.Fatalf("final status = %q, want %q", store.finalStatus, run.Succeeded)
	}
	if len(store.captures) != 1 || store.captures[0].ExternalID != "good" {
		t.Fatalf("captures = %+v, want the valid record to survive", store.captures)
	}
}

func TestImportRejectsUnknownFields(t *testing.T) {
	// A misspelled field would otherwise be dropped silently, losing data.
	store := newFakeStore()
	result, err := importFrom(t, store, `{"externalId":"a","body":"typo in the field name"}`)
	if err != nil {
		t.Fatalf("ImportJSONL: %v", err)
	}
	if result.Failed != 1 || result.Imported != 0 {
		t.Fatalf("result = %+v, want the unknown field to fail the line", result)
	}
}

// A storage failure after something committed leaves the run partial: some
// records are in, so the run neither fully succeeded nor fully failed.
func TestImportIsPartialWhenStorageFailsAfterCommitting(t *testing.T) {
	input := strings.Join([]string{
		`{"external_id":"a","body":"first"}`,
		`{"external_id":"b","body":"second"}`,
		`{"external_id":"c","body":"third"}`,
	}, "\n")

	store := newFakeStore()
	store.failAfter = 1

	result, err := importFrom(t, store, input)
	if err == nil {
		t.Fatal("a storage failure did not abort the import")
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("err = %v, want the failing line number", err)
	}
	if result.Imported != 1 {
		t.Fatalf("result = %+v, want the first record counted", result)
	}
	if store.finalStatus != run.Partial {
		t.Fatalf("final status = %q, want %q", store.finalStatus, run.Partial)
	}
}

// Failing before anything commits is a plain failure, not a partial run.
func TestImportFailsWhenStorageFailsImmediately(t *testing.T) {
	store := newFakeStore()
	store.failAfter = 0

	if _, err := importFrom(t, store, `{"external_id":"a","body":"first"}`); err == nil {
		t.Fatal("a storage failure did not abort the import")
	}
	if store.finalStatus != run.Failed {
		t.Fatalf("final status = %q, want %q", store.finalStatus, run.Failed)
	}
}

func TestImportRejectsOversizedBody(t *testing.T) {
	oversized := strings.Repeat("x", record.DefaultMaxBodyBytes+1)

	store := newFakeStore()
	result, err := importFrom(t, store, `{"external_id":"big","body":"`+oversized+`"}`)
	if err != nil {
		t.Fatalf("ImportJSONL: %v", err)
	}
	if result.Failed != 1 {
		t.Fatalf("result = %+v, want the oversized body rejected", result)
	}
}

// Cancellation must still close the run, otherwise it would look abandoned and
// later be swept up as interrupted.
func TestImportRecordsCanceledRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	store := newFakeStore()
	_, err := ImportJSONL(ctx, store, strings.NewReader(`{"external_id":"a","body":"x"}`),
		discardLogger(), "notes", testOptions())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if store.finalStatus != run.Canceled {
		t.Fatalf("final status = %q, want %q", store.finalStatus, run.Canceled)
	}
}

// One bad input file must not be able to flood the event table.
func TestImportStopsWritingEventsAtTheLimit(t *testing.T) {
	var builder strings.Builder
	for range run.MaxEventsPerRun + 500 {
		builder.WriteString(`{"external_id":"","body":"bad"}` + "\n")
	}

	store := newFakeStore()
	result, err := importFrom(t, store, builder.String())
	if err != nil {
		t.Fatalf("ImportJSONL: %v", err)
	}

	// Every bad line is still counted, even after events stop.
	if result.Failed != run.MaxEventsPerRun+500 {
		t.Fatalf("failed = %d, want %d", result.Failed, run.MaxEventsPerRun+500)
	}
	// The budget is spent, plus the truncation marker itself.
	if len(store.events) > run.MaxEventsPerRun+1 {
		t.Fatalf("stored %d events, want at most %d", len(store.events), run.MaxEventsPerRun+1)
	}
	if !strings.Contains(store.messages(), "event log truncated") {
		t.Fatal("no truncation marker was stored")
	}
}

func TestImportRunsHousekeepingBeforeStarting(t *testing.T) {
	store := newFakeStore()
	store.recovered = 2
	store.purged = 15

	if _, err := importFrom(t, store, `{"external_id":"a","body":"x"}`); err != nil {
		t.Fatalf("ImportJSONL: %v", err)
	}
	// Housekeeping must not disturb the import itself.
	if store.finalStatus != run.Succeeded {
		t.Fatalf("final status = %q", store.finalStatus)
	}
}

func TestImportNormalizesBeforeStoring(t *testing.T) {
	input := `{"external_id":"  a  ","body":"line\r\ntext","occurred_at":"2026-08-10T18:00:00+08:00"}`

	store := newFakeStore()
	if _, err := importFrom(t, store, input); err != nil {
		t.Fatalf("ImportJSONL: %v", err)
	}

	if len(store.captures) != 1 {
		t.Fatalf("captures = %d, want 1", len(store.captures))
	}
	stored := store.captures[0]
	if stored.ExternalID != "a" {
		t.Errorf("ExternalID = %q", stored.ExternalID)
	}
	if stored.Body != "line\ntext" {
		t.Errorf("Body = %q", stored.Body)
	}
	if stored.OccurredAt != "2026-08-10T10:00:00Z" {
		t.Errorf("OccurredAt = %q, want UTC", stored.OccurredAt)
	}
}
