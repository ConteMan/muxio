package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/ConteMan/muxio/internal/record"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// ErrSchemaAhead reports a database written by a newer binary. Downgrading a
// schema is never attempted: the safe action is to stop and tell the operator.
var ErrSchemaAhead = errors.New("database schema is newer than this binary")

type migration struct {
	version int
	name    string
	sql     string
}

// Migrate applies every pending migration inside a single transaction.
func Migrate(ctx context.Context, db *sql.DB) error {
	available, err := loadMigrations()
	if err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
		    version    INTEGER PRIMARY KEY,
		    name       TEXT NOT NULL,
		    applied_at TEXT NOT NULL
		) STRICT;`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	var applied int
	if err := db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&applied); err != nil {
		return fmt.Errorf("read applied schema version: %w", err)
	}

	highest := 0
	if len(available) > 0 {
		highest = available[len(available)-1].version
	}
	if applied > highest {
		return fmt.Errorf("%w: database at %d, binary knows %d",
			ErrSchemaAhead, applied, highest)
	}

	pending := make([]migration, 0, len(available))
	for _, candidate := range available {
		if candidate.version > applied {
			pending = append(pending, candidate)
		}
	}
	if len(pending) == 0 {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, step := range pending {
		if _, err := tx.ExecContext(ctx, step.sql); err != nil {
			return fmt.Errorf("apply migration %04d_%s: %w", step.version, step.name, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
			step.version, step.name, record.Now()); err != nil {
			return fmt.Errorf("record migration %04d_%s: %w", step.version, step.name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}
	return nil
}

// SchemaVersion reports the highest applied migration, or zero for a fresh
// database.
func SchemaVersion(ctx context.Context, db *sql.DB) (int, error) {
	var version int
	err := db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	return version, nil
}

// loadMigrations reads embedded migrations ordered by version. Published
// migrations are append-only, so a gap or duplicate is a build-time defect.
func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}

	loaded := make([]migration, 0, len(entries))
	seen := make(map[int]string, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		version, name, err := parseMigrationName(entry.Name())
		if err != nil {
			return nil, err
		}
		if previous, duplicate := seen[version]; duplicate {
			return nil, fmt.Errorf("duplicate migration version %d: %s and %s",
				version, previous, entry.Name())
		}
		seen[version] = entry.Name()

		content, err := migrationFiles.ReadFile(path.Join("migrations", entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		loaded = append(loaded, migration{version: version, name: name, sql: string(content)})
	}

	sort.Slice(loaded, func(i, j int) bool { return loaded[i].version < loaded[j].version })
	for index, step := range loaded {
		if step.version != index+1 {
			return nil, fmt.Errorf("migration versions must be contiguous from 1, found %d at position %d",
				step.version, index+1)
		}
	}
	return loaded, nil
}

func parseMigrationName(filename string) (int, string, error) {
	trimmed := strings.TrimSuffix(filename, ".sql")
	prefix, name, found := strings.Cut(trimmed, "_")
	if !found || name == "" {
		return 0, "", fmt.Errorf("migration %s must be named NNNN_name.sql", filename)
	}
	version, err := strconv.Atoi(prefix)
	if err != nil || version <= 0 {
		return 0, "", fmt.Errorf("migration %s has an invalid version prefix", filename)
	}
	return version, name, nil
}
