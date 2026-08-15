// Package memory implements the core.Repository port on SQLite + FTS5.
//
// The schema is embedded and versioned with PRAGMA user_version so AGIS stays
// a single static binary with no external migration tooling. The single
// standalone memory_fts table (doc_type, doc_id, content) indexes both
// messages and observations, synced explicitly in the same transaction as the
// base write rather than through triggers.
package memory

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// configureDB applies connection-level, non-transactional pragmas. foreign_keys
// and journal_mode are set here rather than in a migration because they are
// no-ops inside a transaction and must be active before any DML runs.
func configureDB(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("enabling foreign keys: %w", err)
	}
	var mode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode = WAL").Scan(&mode); err != nil {
		return fmt.Errorf("setting WAL: %w", err)
	}
	var timeout int
	if err := db.QueryRowContext(ctx, "PRAGMA busy_timeout = 5000").Scan(&timeout); err != nil {
		return fmt.Errorf("setting busy_timeout: %w", err)
	}
	return nil
}

// migration is one embedded SQL file keyed by its numeric version prefix.
type migration struct {
	version int
	name    string
}

// applyMigrations runs every embedded migrations/*.sql file whose numeric
// prefix is greater than the database's current PRAGMA user_version. Each
// file applies inside its own transaction, then advances user_version to that
// file's number. Files are named NNNN_description.sql.
func applyMigrations(ctx context.Context, db *sql.DB) error {
	migs, err := loadMigrations()
	if err != nil {
		return err
	}

	current, err := userVersion(ctx, db)
	if err != nil {
		return err
	}

	for _, m := range migs {
		if m.version <= current {
			continue
		}
		if err := applyMigration(ctx, db, m); err != nil {
			return fmt.Errorf("applying migration %s: %w", m.name, err)
		}
	}
	return nil
}

// loadMigrations lists and sorts the embedded SQL files by version.
func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("reading embedded migrations: %w", err)
	}

	migs := make([]migration, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		version, err := versionFromName(e.Name())
		if err != nil {
			return nil, err
		}
		migs = append(migs, migration{version: version, name: e.Name()})
	}
	sort.Slice(migs, func(i, j int) bool { return migs[i].version < migs[j].version })
	return migs, nil
}

// versionFromName extracts the leading integer of a NNNN_description.sql name.
func versionFromName(name string) (int, error) {
	end := 0
	for end < len(name) && name[end] >= '0' && name[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0, fmt.Errorf("migration %q has no numeric version prefix", name)
	}
	return strconv.Atoi(name[:end])
}

// userVersion reads the database's current schema version.
func userVersion(ctx context.Context, db *sql.DB) (int, error) {
	var v int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&v); err != nil {
		return 0, fmt.Errorf("reading user_version: %w", err)
	}
	return v, nil
}

// applyMigration runs a single migration file inside a transaction and bumps
// user_version to its number on success.
func applyMigration(ctx context.Context, db *sql.DB, m migration) error {
	data, err := migrationsFS.ReadFile("migrations/" + m.name)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, string(data)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", m.version)); err != nil {
		return err
	}
	return tx.Commit()
}
