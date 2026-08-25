package memory

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

// openTestDB opens a fresh temp-file database with connection pragmas applied,
// mirroring the setup NewRepository performs. Used to test migrations in
// isolation from the Repository.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", t.TempDir()+"/agis.db")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	db.SetMaxOpenConns(1)
	if err := configureDB(context.Background(), db); err != nil {
		db.Close()
		t.Fatalf("configureDB() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// latestVersion is the highest embedded migration version. Update when a new
// migrations/*.sql file is added.
const latestVersion = 3

func TestMigrations(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	if err := applyMigrations(ctx, db); err != nil {
		t.Fatalf("applyMigrations() error = %v", err)
	}

	// user_version advanced to the latest migration version.
	var v int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&v); err != nil {
		t.Fatalf("reading user_version: %v", err)
	}
	if v != latestVersion {
		t.Errorf("user_version = %d, want %d", v, latestVersion)
	}

	// The base tables, the FTS table, and the learning-loop tables exist.
	for _, table := range []string{
		"conversations", "messages", "observations", "memory_fts", "user_model", "session_events", "skills",
	} {
		var name string
		err := db.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE name = ?`, table).Scan(&name)
		if err != nil {
			t.Errorf("table %q missing: %v", table, err)
		}
	}

	// observations gained the updated_at column.
	var updatedAt string
	if err := db.QueryRowContext(ctx, `SELECT updated_at FROM observations LIMIT 1`).Scan(&updatedAt); err != nil &&
		!errors.Is(err, sql.ErrNoRows) {
		t.Errorf("observations.updated_at missing: %v", err)
	}
}

func TestMigrations_Idempotent(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	if err := applyMigrations(ctx, db); err != nil {
		t.Fatalf("first applyMigrations() error = %v", err)
	}
	// Applying again must be a no-op, not an error.
	if err := applyMigrations(ctx, db); err != nil {
		t.Fatalf("second applyMigrations() error = %v, want idempotent", err)
	}

	var v int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&v); err != nil {
		t.Fatalf("reading user_version: %v", err)
	}
	if v != latestVersion {
		t.Errorf("user_version = %d, want %d", v, latestVersion)
	}
}

// TestMigration_V1ToV2 proves 0002 upgrades a real v1 database: it backfills
// observations.updated_at from created_at and enforces the unique topic_key
// index, ending at user_version 2.
func TestMigration_V1ToV2(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	// Apply only 0001 to simulate a v1 database with data.
	migs, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations() error = %v", err)
	}
	for _, m := range migs {
		if m.version == 1 {
			if err := applyMigration(ctx, db, m); err != nil {
				t.Fatalf("applying 0001: %v", err)
			}
		}
	}

	// A pre-0002 observation has no updated_at column.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO observations (id, topic_key, type, content, importance, created_at, source_ref)
		 VALUES ('obs-1', 'coffee', 'preference', 'dark roast', 4, '2026-01-01T00:00:00.000000000Z', 'conv-1')`); err != nil {
		t.Fatalf("inserting v1 observation: %v", err)
	}

	if err := applyMigrations(ctx, db); err != nil {
		t.Fatalf("applyMigrations() error = %v", err)
	}

	var v int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&v); err != nil {
		t.Fatalf("reading user_version: %v", err)
	}
	if v != latestVersion {
		t.Errorf("user_version = %d, want %d", v, latestVersion)
	}

	// updated_at backfilled from created_at.
	var updatedAt string
	if err := db.QueryRowContext(ctx, `SELECT updated_at FROM observations WHERE id = 'obs-1'`).Scan(&updatedAt); err != nil {
		t.Fatalf("reading updated_at: %v", err)
	}
	if updatedAt != "2026-01-01T00:00:00.000000000Z" {
		t.Errorf("updated_at = %q, want backfilled created_at", updatedAt)
	}

	// The unique topic_key index rejects a duplicate topic.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO observations (id, topic_key, type, content, importance, created_at, updated_at, source_ref)
		 VALUES ('obs-2', 'coffee', 'note', 'dup', 1, '2026-01-01T00:00:00.000000000Z', '2026-01-01T00:00:00.000000000Z', '')`); err == nil {
		t.Fatal("inserting duplicate topic_key succeeded, want unique violation")
	}
}

func TestMigrations_EnforcesForeignKeys(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	if err := applyMigrations(ctx, db); err != nil {
		t.Fatalf("applyMigrations() error = %v", err)
	}

	// A message referencing a nonexistent conversation must be rejected.
	_, err := db.ExecContext(ctx,
		`INSERT INTO messages (conversation_id, role, content, created_at)
		 VALUES ('missing', 'user', 'hi', '2026-01-01T00:00:00.000000000Z')`)
	if err == nil {
		t.Fatal("INSERT with missing conversation succeeded, want foreign key violation")
	}
}
