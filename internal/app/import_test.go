package app

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ConteMan/muxio/internal/record"
)

// fakeStore records what an import asked it to write.
type fakeStore struct {
	sourceID  int64
	seen      map[string]bool
	captures  []record.Record
	failAfter int
	calls     int
}

func newFakeStore() *fakeStore {
	return &fakeStore{sourceID: 1, seen: make(map[string]bool), failAfter: -1}
}

func (f *fakeStore) EnsureSource(_ context.Context, _, _ string) (int64, error) {
	return f.sourceID, nil
}

func (f *fakeStore) AddCapture(_ context.Context, _ int64, rec record.Record) (bool, error) {
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
		return false, nil
	}
	f.seen[key] = true
	f.captures = append(f.captures, rec)
	return true, nil
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

	var problems bytes.Buffer
	store := newFakeStore()

	result, err := ImportJSONL(context.Background(), store, strings.NewReader(input), &problems, "notes")
	if err != nil {
		t.Fatalf("ImportJSONL: %v", err)
	}

	if result.Imported != 2 || result.Duplicate != 1 || result.Failed != 2 {
		t.Fatalf("result = %+v, want imported=2 duplicate=1 failed=2", result)
	}
	// Line numbers must point at the real input lines, counting blanks.
	if !strings.Contains(problems.String(), "line 5:") ||
		!strings.Contains(problems.String(), "line 6:") {
		t.Fatalf("problems = %q", problems.String())
	}
}

func TestImportKeepsGoingAfterInvalidLine(t *testing.T) {
	input := strings.Join([]string{
		`{"external_id":"","body":"broken"}`,
		`{"external_id":"good","body":"kept"}`,
	}, "\n")

	store := newFakeStore()
	result, err := ImportJSONL(context.Background(), store, strings.NewReader(input), nil, "notes")
	if err != nil {
		t.Fatalf("ImportJSONL: %v", err)
	}

	if result.Imported != 1 || result.Failed != 1 {
		t.Fatalf("result = %+v, want imported=1 failed=1", result)
	}
	if len(store.captures) != 1 || store.captures[0].ExternalID != "good" {
		t.Fatalf("captures = %+v, want the valid record to survive", store.captures)
	}
}

func TestImportRejectsUnknownFields(t *testing.T) {
	// A misspelled field would otherwise be dropped silently, losing data.
	input := `{"externalId":"a","body":"typo in the field name"}`

	store := newFakeStore()
	result, err := ImportJSONL(context.Background(), store, strings.NewReader(input), nil, "notes")
	if err != nil {
		t.Fatalf("ImportJSONL: %v", err)
	}
	if result.Failed != 1 || result.Imported != 0 {
		t.Fatalf("result = %+v, want the unknown field to fail the line", result)
	}
}

func TestImportAbortsOnStorageFailure(t *testing.T) {
	input := strings.Join([]string{
		`{"external_id":"a","body":"first"}`,
		`{"external_id":"b","body":"second"}`,
		`{"external_id":"c","body":"third"}`,
	}, "\n")

	store := newFakeStore()
	store.failAfter = 1

	result, err := ImportJSONL(context.Background(), store, strings.NewReader(input), nil, "notes")
	if err == nil {
		t.Fatal("a storage failure did not abort the import")
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("err = %v, want the failing line number", err)
	}
	// Whatever committed before the failure is still reported.
	if result.Imported != 1 {
		t.Fatalf("result = %+v, want the first record counted", result)
	}
}

func TestImportRejectsOversizedBody(t *testing.T) {
	oversized := strings.Repeat("x", record.MaxBodyBytes+1)
	input := `{"external_id":"big","body":"` + oversized + `"}`

	store := newFakeStore()
	result, err := ImportJSONL(context.Background(), store, strings.NewReader(input), nil, "notes")
	if err != nil {
		t.Fatalf("ImportJSONL: %v", err)
	}
	if result.Failed != 1 {
		t.Fatalf("result = %+v, want the oversized body rejected", result)
	}
}

func TestImportStopsOnCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	store := newFakeStore()
	_, err := ImportJSONL(ctx, store, strings.NewReader(`{"external_id":"a","body":"x"}`), nil, "notes")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestImportNormalizesBeforeStoring(t *testing.T) {
	input := `{"external_id":"  a  ","body":"line\r\ntext","occurred_at":"2026-08-10T18:00:00+08:00"}`

	store := newFakeStore()
	if _, err := ImportJSONL(context.Background(), store, strings.NewReader(input), nil, "notes"); err != nil {
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
