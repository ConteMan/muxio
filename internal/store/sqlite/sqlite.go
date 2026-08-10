// Package sqlite owns every database concern: connections, migrations, and the
// transactions that keep captures idempotent.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ConteMan/muxio/internal/record"

	_ "modernc.org/sqlite" // pure Go driver, see ADR-004
)

// MemoryPath opens a private in-memory database. Tests use it; nothing else should.
const MemoryPath = ":memory:"

// busyTimeout bounds how long a writer waits for the database lock. ADR-002
// requires bounded waits rather than indefinite blocking.
const busyTimeout = 5 * time.Second

// Store is a handle to the Muxio database.
type Store struct {
	db *sql.DB
}

// Open connects to the database at path and applies the connection settings the
// architecture depends on. It does not create the parent directory.
func Open(ctx context.Context, path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// A single connection keeps the PRAGMAs below authoritative and matches the
	// single-writer model: `muxio serve` is the only long-lived writer.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open database %s: %w", path, err)
	}

	store := &Store{db: db}
	if err := store.applyPragmas(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) applyPragmas(ctx context.Context) error {
	settings := []string{
		fmt.Sprintf("PRAGMA busy_timeout = %d", busyTimeout.Milliseconds()),
		"PRAGMA foreign_keys = ON",
		"PRAGMA synchronous = NORMAL",
	}
	for _, pragma := range settings {
		if _, err := s.db.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("apply %q: %w", pragma, err)
		}
	}

	// journal_mode reports the resulting mode. In-memory databases cannot use
	// WAL and answer "memory", which is expected in tests.
	var mode string
	if err := s.db.QueryRowContext(ctx, "PRAGMA journal_mode = WAL").Scan(&mode); err != nil {
		return fmt.Errorf("enable WAL: %w", err)
	}
	if mode != "wal" && mode != "memory" {
		return fmt.Errorf("journal mode is %q, want wal", mode)
	}

	var foreignKeys int
	if err := s.db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		return fmt.Errorf("read foreign_keys: %w", err)
	}
	if foreignKeys != 1 {
		return fmt.Errorf("foreign key enforcement is off")
	}
	return nil
}

// Migrate brings the database up to the schema this binary knows.
func (s *Store) Migrate(ctx context.Context) error {
	return Migrate(ctx, s.db)
}

// SchemaVersion reports the highest applied migration.
func (s *Store) SchemaVersion(ctx context.Context) (int, error) {
	return SchemaVersion(ctx, s.db)
}

// Close releases the database handle.
func (s *Store) Close() error {
	return s.db.Close()
}

// EnsureSource returns the id of the named source, creating it when absent.
// An existing source keeps its connector kind: import never rewrites it.
func (s *Store) EnsureSource(ctx context.Context, name, connectorKind string) (int64, error) {
	now := record.Now()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO sources (name, connector_kind, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (name) DO NOTHING`,
		name, connectorKind, now, now); err != nil {
		return 0, fmt.Errorf("create source %q: %w", name, err)
	}

	var id int64
	if err := s.db.QueryRowContext(ctx,
		`SELECT id FROM sources WHERE name = ?`, name).Scan(&id); err != nil {
		return 0, fmt.Errorf("read source %q: %w", name, err)
	}
	return id, nil
}

// AddCapture writes one immutable capture and reports whether it was new.
// Re-observing the same version is a no-op, which is what makes repeated
// imports idempotent.
func (s *Store) AddCapture(ctx context.Context, sourceID int64, rec record.Record) (bool, error) {
	contentHash, err := rec.ContentHash()
	if err != nil {
		return false, err
	}
	metadata, err := rec.MetadataJSON()
	if err != nil {
		return false, err
	}

	var occurredAt any
	if rec.OccurredAt != "" {
		occurredAt = rec.OccurredAt
	}

	// One capture is one transaction. Run counters and checkpoint advancement
	// join this transaction when later specs introduce them.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin capture: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `
		INSERT INTO captures (
		    source_id, external_id, content_hash, title, body,
		    mime_type, canonical_url, occurred_at, captured_at, metadata_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (source_id, external_id, content_hash) DO NOTHING`,
		sourceID, rec.ExternalID, contentHash, rec.Title, rec.Body,
		rec.MIMEType, rec.CanonicalURL, occurredAt, record.Now(), metadata)
	if err != nil {
		return false, fmt.Errorf("insert capture %q: %w", rec.ExternalID, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("inspect insert result: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit capture %q: %w", rec.ExternalID, err)
	}
	return affected == 1, nil
}

// CountCaptures reports how many captures a source holds. It supports
// verification and tests; read paths arrive with later specs.
func (s *Store) CountCaptures(ctx context.Context, sourceID int64) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM captures WHERE source_id = ?`, sourceID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count captures: %w", err)
	}
	return count, nil
}
