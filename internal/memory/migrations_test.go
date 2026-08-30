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
const latestVersion = 7

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
		"conversations", "messages", "observations", "memory_fts", "user_model", "session_events", "skills", "snapshots", "audit_log", "embeddings", "attachments",
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

// TestMigration_V6ToV7 proves 0007 upgrades a v6 database: creates attachments table and index.
func TestMigration_V6ToV7(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	// Apply migrations 1 through 6.
	migs, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations() error = %v", err)
	}
	for _, m := range migs {
		if m.version <= 6 {
			if err := applyMigration(ctx, db, m); err != nil {
				t.Fatalf("applying migration %s: %v", m.name, err)
			}
		}
	}

	// Verify user_version is 6 and attachments table does not yet exist.
	var v int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&v); err != nil {
		t.Fatalf("reading user_version: %v", err)
	}
	if v != 6 {
		t.Fatalf("user_version before upgrade = %d, want 6", v)
	}

	// Create a conversation and message
	_, err = db.ExecContext(ctx,
		`INSERT INTO conversations (id, title, created_at, updated_at, summary, message_count)
		 VALUES ('conv-m7', 'test', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z', '', 1)`)
	if err != nil {
		t.Fatalf("inserting conversation: %v", err)
	}

	res, err := db.ExecContext(ctx,
		`INSERT INTO messages (conversation_id, role, content, created_at)
		 VALUES ('conv-m7', 'user', 'with media', '2026-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatalf("inserting message: %v", err)
	}
	msgID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("getting msgID: %v", err)
	}

	// Now apply remaining migrations (0007).
	if err := applyMigrations(ctx, db); err != nil {
		t.Fatalf("applyMigrations() error = %v", err)
	}

	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&v); err != nil {
		t.Fatalf("reading user_version: %v", err)
	}
	if v != 7 {
		t.Errorf("user_version after upgrade = %d, want 7", v)
	}

	// Insert attachment for the message
	_, err = db.ExecContext(ctx,
		`INSERT INTO attachments (id, message_id, type, mime_type, data, url, name, created_at)
		 VALUES ('att-1', ?, 'image', 'image/png', X'89504E47', 'https://example.com/img.png', 'img.png', '2026-01-01T00:00:00Z')`, msgID)
	if err != nil {
		t.Fatalf("inserting attachment: %v", err)
	}

	// Verify attachment is present
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM attachments WHERE message_id = ?`, msgID).Scan(&count); err != nil {
		t.Fatalf("querying attachments count: %v", err)
	}
	if count != 1 {
		t.Errorf("attachments count = %d, want 1", count)
	}

	// Test cascade delete on conversation deletion
	_, err = db.ExecContext(ctx, `DELETE FROM conversations WHERE id = 'conv-m7'`)
	if err != nil {
		t.Fatalf("deleting conversation: %v", err)
	}

	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM attachments WHERE id = 'att-1'`).Scan(&count); err != nil {
		t.Fatalf("querying attachments after cascade: %v", err)
	}
	if count != 0 {
		t.Errorf("attachments count after cascade delete = %d, want 0", count)
	}
}

func TestMigration_V5ToV6(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	// Apply migrations 1 through 5.
	migs, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations() error = %v", err)
	}
	for _, m := range migs {
		if m.version <= 5 {
			if err := applyMigration(ctx, db, m); err != nil {
				t.Fatalf("applying migration %s: %v", m.name, err)
			}
		}
	}

	// Verify user_version is 5 and embeddings table does not yet exist.
	var v int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&v); err != nil {
		t.Fatalf("reading user_version: %v", err)
	}
	if v != 5 {
		t.Fatalf("user_version before upgrade = %d, want 5", v)
	}

	// Now apply remaining migrations (0006).
	if err := applyMigrations(ctx, db); err != nil {
		t.Fatalf("applyMigrations() error = %v", err)
	}

	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&v); err != nil {
		t.Fatalf("reading user_version: %v", err)
	}
	if v != latestVersion {
		t.Errorf("user_version after upgrade = %d, want %d", v, latestVersion)
	}

	// Verify embeddings table and unique constraint on (doc_type, doc_id).
	_, err = db.ExecContext(ctx,
		`INSERT INTO embeddings (id, doc_type, doc_id, dimension, vector, created_at, updated_at)
		 VALUES ('emb-1', 'observation', 'obs-1', 4, X'0000803f000000400000404000008040', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatalf("inserting into embeddings table: %v", err)
	}

	// Duplicate doc_type, doc_id should fail unique constraint
	_, err = db.ExecContext(ctx,
		`INSERT INTO embeddings (id, doc_type, doc_id, dimension, vector, created_at, updated_at)
		 VALUES ('emb-2', 'observation', 'obs-1', 4, X'0000803f000000400000404000008040', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	if err == nil {
		t.Fatal("inserting duplicate (doc_type, doc_id) into embeddings succeeded, want unique constraint violation")
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
