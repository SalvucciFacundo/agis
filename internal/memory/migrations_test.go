package memory

import (
	"context"
	"database/sql"
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

func TestMigrations(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	if err := applyMigrations(ctx, db); err != nil {
		t.Fatalf("applyMigrations() error = %v", err)
	}

	// user_version advanced to 1.
	var v int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&v); err != nil {
		t.Fatalf("reading user_version: %v", err)
	}
	if v != 1 {
		t.Errorf("user_version = %d, want 1", v)
	}

	// The three base tables plus the FTS table exist.
	for _, table := range []string{"conversations", "messages", "observations", "memory_fts"} {
		var name string
		err := db.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE name = ?`, table).Scan(&name)
		if err != nil {
			t.Errorf("table %q missing: %v", table, err)
		}
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
	if v != 1 {
		t.Errorf("user_version = %d, want 1", v)
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
