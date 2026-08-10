package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/ConteMan/muxio/internal/record"
)

func TestMigrateIsIdempotentAndRecorded(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	version, err := store.SchemaVersion(ctx)
	if err != nil {
		t.Fatalf("SchemaVersion: %v", err)
	}

	// Every embedded migration must be applied and recorded exactly once.
	available, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations: %v", err)
	}
	if version != len(available) {
		t.Fatalf("schema version = %d, want %d", version, len(available))
	}

	// Re-running must be a no-op rather than an error or a duplicate row.
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}

	var applied int
	if err := store.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM schema_migrations`).Scan(&applied); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if applied != len(available) {
		t.Fatalf("schema_migrations has %d rows, want %d", applied, len(available))
	}
}

func TestMigrateRefusesNewerSchema(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	// Simulate a database written by a future binary.
	if _, err := store.db.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, name, applied_at) VALUES (99, 'future', ?)`,
		record.Now()); err != nil {
		t.Fatalf("seed future migration: %v", err)
	}

	err := store.Migrate(ctx)
	if !errors.Is(err, ErrSchemaAhead) {
		t.Fatalf("err = %v, want ErrSchemaAhead", err)
	}
}

func TestAddCaptureIsIdempotent(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	sourceID := ensureSource(t, store, "notes")

	rec := normalize(t, record.Record{ExternalID: "note-1", Title: "Title", Body: "body"})

	inserted, err := store.AddCapture(ctx, sourceID, 0, rec)
	if err != nil {
		t.Fatalf("first AddCapture: %v", err)
	}
	if !inserted {
		t.Fatal("first insert reported a duplicate")
	}

	inserted, err = store.AddCapture(ctx, sourceID, 0, rec)
	if err != nil {
		t.Fatalf("second AddCapture: %v", err)
	}
	if inserted {
		t.Fatal("re-observing the same version inserted a second row")
	}

	assertCaptureCount(t, store, sourceID, 1)
}

func TestChangedContentKeepsBothVersions(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	sourceID := ensureSource(t, store, "notes")

	original := normalize(t, record.Record{ExternalID: "note-1", Title: "Title", Body: "first"})
	if _, err := store.AddCapture(ctx, sourceID, 0, original); err != nil {
		t.Fatalf("AddCapture original: %v", err)
	}

	revised := normalize(t, record.Record{ExternalID: "note-1", Title: "Title", Body: "second"})
	inserted, err := store.AddCapture(ctx, sourceID, 0, revised)
	if err != nil {
		t.Fatalf("AddCapture revised: %v", err)
	}
	if !inserted {
		t.Fatal("changed content was treated as a duplicate")
	}

	assertCaptureCount(t, store, sourceID, 2)

	// The original row must still carry its original body: captures are immutable.
	var bodies []string
	rows, err := store.db.QueryContext(ctx,
		`SELECT body FROM captures WHERE source_id = ? ORDER BY id`, sourceID)
	if err != nil {
		t.Fatalf("query bodies: %v", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var body string
		if err := rows.Scan(&body); err != nil {
			t.Fatalf("scan body: %v", err)
		}
		bodies = append(bodies, body)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate bodies: %v", err)
	}
	if len(bodies) != 2 || bodies[0] != "first" || bodies[1] != "second" {
		t.Fatalf("bodies = %q, want [first second]", bodies)
	}
}

func TestSameExternalIDAcrossSourcesStaysSeparate(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	first := ensureSource(t, store, "feed-a")
	second := ensureSource(t, store, "feed-b")

	rec := normalize(t, record.Record{ExternalID: "shared", Body: "body"})

	for _, sourceID := range []int64{first, second} {
		inserted, err := store.AddCapture(ctx, sourceID, 0, rec)
		if err != nil {
			t.Fatalf("AddCapture: %v", err)
		}
		if !inserted {
			t.Fatalf("source %d treated another source's record as a duplicate", sourceID)
		}
	}

	assertCaptureCount(t, store, first, 1)
	assertCaptureCount(t, store, second, 1)
}

func TestEnsureSourceReusesExistingSource(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	first, err := store.EnsureSource(ctx, "notes", "manual")
	if err != nil {
		t.Fatalf("EnsureSource: %v", err)
	}
	// A second call with a different kind must not create or rewrite a source.
	second, err := store.EnsureSource(ctx, "notes", "file")
	if err != nil {
		t.Fatalf("EnsureSource again: %v", err)
	}
	if first != second {
		t.Fatalf("source id changed from %d to %d", first, second)
	}

	var kind string
	if err := store.db.QueryRowContext(ctx,
		`SELECT connector_kind FROM sources WHERE id = ?`, first).Scan(&kind); err != nil {
		t.Fatalf("read connector kind: %v", err)
	}
	if kind != "manual" {
		t.Fatalf("connector_kind = %q, want the original manual", kind)
	}
}

func TestForeignKeysAreEnforced(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	rec := normalize(t, record.Record{ExternalID: "orphan", Body: "body"})
	if _, err := store.AddCapture(ctx, 9999, 0, rec); err == nil {
		t.Fatal("a capture referencing a missing source was accepted")
	}
}

// Two independent handles on the same file exercise the bounded busy timeout:
// concurrent writers must serialize rather than fail or duplicate.
func TestConcurrentWritersStayIdempotent(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "muxio.db")

	primary := openFileStore(t, databasePath)
	sourceID := ensureSource(t, primary, "notes")

	secondary := openFileStore(t, databasePath)

	const writers = 4
	rec := normalize(t, record.Record{ExternalID: "note-1", Title: "Title", Body: "body"})

	var wait sync.WaitGroup
	inserts := make([]bool, writers)
	failures := make([]error, writers)

	for index := range writers {
		store := primary
		if index%2 == 1 {
			store = secondary
		}
		wait.Add(1)
		go func() {
			defer wait.Done()
			inserted, err := store.AddCapture(ctx, sourceID, 0, rec)
			inserts[index] = inserted
			failures[index] = err
		}()
	}
	wait.Wait()

	insertedCount := 0
	for index, err := range failures {
		if err != nil {
			t.Fatalf("writer %d: %v", index, err)
		}
		if inserts[index] {
			insertedCount++
		}
	}
	if insertedCount != 1 {
		t.Fatalf("%d writers reported an insert, want exactly 1", insertedCount)
	}

	assertCaptureCount(t, primary, sourceID, 1)
}

// A database left behind by an interrupted process must reopen cleanly and keep
// exactly what was committed.
func TestReopenAfterInterruptKeepsCommittedRows(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "muxio.db")

	first := openFileStore(t, databasePath)
	sourceID := ensureSource(t, first, "notes")
	rec := normalize(t, record.Record{ExternalID: "note-1", Body: "body"})
	if _, err := first.AddCapture(ctx, sourceID, 0, rec); err != nil {
		t.Fatalf("AddCapture: %v", err)
	}
	// Drop the handle without a graceful shutdown path.
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened := openFileStore(t, databasePath)
	inserted, err := reopened.AddCapture(ctx, sourceID, 0, rec)
	if err != nil {
		t.Fatalf("AddCapture after reopen: %v", err)
	}
	if inserted {
		t.Fatal("reopening produced a duplicate row")
	}
	assertCaptureCount(t, reopened, sourceID, 1)
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	return openFileStore(t, filepath.Join(t.TempDir(), "muxio.db"))
}

func openFileStore(t *testing.T, path string) *Store {
	t.Helper()
	ctx := context.Background()

	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return store
}

func ensureSource(t *testing.T, store *Store, name string) int64 {
	t.Helper()
	sourceID, err := store.EnsureSource(context.Background(), name, "manual")
	if err != nil {
		t.Fatalf("EnsureSource: %v", err)
	}
	return sourceID
}

func normalize(t *testing.T, rec record.Record) record.Record {
	t.Helper()
	normalized, err := rec.Normalize()
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	return normalized
}

func assertCaptureCount(t *testing.T, store *Store, sourceID int64, want int) {
	t.Helper()
	count, err := store.CountCaptures(context.Background(), sourceID)
	if err != nil {
		t.Fatalf("CountCaptures: %v", err)
	}
	if count != want {
		t.Fatalf("captures = %d, want %d", count, want)
	}
}
