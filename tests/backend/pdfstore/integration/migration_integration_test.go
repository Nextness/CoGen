// migration_integration_test.go proves the production V00002-to-V00003
// companion upgrade preserves populated evidence and removes only the
// historical attempt-to-blob retention foreign key.

package pdfstore_test

import (
	"analysis/backend/database"
	"analysis/backend/pdfstore"
	"analysis/internal/migrations"
	"analysis/internal/sqliteread"
	testhelpers "analysis/tests/support/go/backend/database"
	"analysis/tests/support/go/testenv"
	"context"
	"database/sql"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// seedCompanionV00002 inserts populated legacy evidence before the V00003 upgrade.
func seedCompanionV00002(t *testing.T, path string) {
	t.Helper()
	conn, err := database.OpenWritable(path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	sharedHash := strings.Repeat("a", 64)
	attemptHash := strings.Repeat("b", 64)
	sharedData := []byte("%PDF-1.7\nshared legacy bytes")
	attemptData := []byte("%PDF-1.7\nattempt-only bytes")
	for _, blob := range []struct {
		hash string
		data []byte
	}{{sharedHash, sharedData}, {attemptHash, attemptData}} {
		if _, err := conn.Exec(`INSERT INTO pdf_blobs (content_hash, byte_size, data, created_at)
			VALUES (?, ?, ?, '2026-01-01T00:00:00Z')`, blob.hash, len(blob.data), blob.data); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := conn.Exec(`INSERT INTO pdf_documents (doi, status, content_hash, inventoried_at, updated_at)
		VALUES ('10.1000/available', 'available', ?, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`, sharedHash); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`INSERT INTO pdf_documents (doi, status, content_hash, inventoried_at, updated_at)
		VALUES ('10.1000/unavailable', 'not_available', NULL, NULL, '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`INSERT INTO pdf_gather_runs
		(correlation_id, started_at, completed_at, status, settings_fingerprint, settings_json, selected_workspaces)
		VALUES ('legacy-gather', '2026-01-01T00:00:00Z', '2026-01-01T00:01:00Z', 'completed', ?, '{}', '[]')`, strings.Repeat("f", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`INSERT INTO pdf_download_attempts
		(id, gather_run_id, doi, source, resolved_url, started_at, completed_at, outcome, http_status, content_hash)
		VALUES (1, 1, '10.1000/available', 'legacy', 'https://example.test/available.pdf', '2026-01-01T00:00:00Z', '2026-01-01T00:00:10Z', 'downloaded', 200, ?)`, sharedHash); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`INSERT INTO pdf_download_attempts
		(id, gather_run_id, doi, source, started_at, completed_at, outcome, http_status, content_hash)
		VALUES (2, 1, '10.1000/unavailable', 'legacy', '2026-01-01T00:00:00Z', '2026-01-01T00:00:10Z', 'failed', 404, ?)`, attemptHash); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`INSERT INTO pdf_audit_outbox
		(event_key, occurred_at, actor, entity_type, entity_id, action, metadata_json, correlation_id)
		VALUES ('legacy-pending', '2026-01-01T00:00:00Z', 'legacy', 'work', '1', 'pdf_inventory_registered', '{}', 'legacy-correlation')`); err != nil {
		t.Fatal(err)
	}
}

// TestProductionCompanionUpgradePreservesLegacyEvidence verifies V00003 keeps
// documents, blobs, attempts, IDs, sequence state, triggers, and pending outbox
// rows while removing the attempt-to-blob retention foreign key.
func TestProductionCompanionUpgradePreservesLegacyEvidence(t *testing.T) {
	ctx := context.Background()
	registry := testenv.Path("config", "database.something")
	pdfPath := filepath.Join(t.TempDir(), "corpus.pdf.db")
	entries := testhelpers.MigrationEntries(t, "database.corpus.pdf.something")
	testhelpers.ApplyMigrationPrefix(t, pdfPath, testenv.Path("migrations", "corpus.pdf"), entries[:2])
	seedCompanionV00002(t, pdfPath)

	store, err := pdfstore.Open(pdfPath, registry)
	if err != nil {
		t.Fatal(err)
	}
	if err := pdfstore.VerifySchema(ctx, store.DB); err != nil {
		store.Close()
		t.Fatal(err)
	}
	sharedHash := queryCompanionOne[string](t, store.DB, "SELECT content_hash FROM pdf_documents WHERE doi = '10.1000/available'")
	if data := queryCompanionOne[[]byte](t, store.DB, "SELECT data FROM pdf_blobs WHERE content_hash = ?", sharedHash); string(data) != "%PDF-1.7\nshared legacy bytes" {
		store.Close()
		t.Fatalf("shared PDF bytes = %q", data)
	}
	if got := queryCompanionOne[int](t, store.DB, "SELECT COUNT(*) FROM pdf_documents WHERE doi = '10.1000/unavailable' AND status = 'not_available'"); got != 1 {
		store.Close()
		t.Fatalf("unavailable document count = %d", got)
	}
	if got := queryCompanionOne[int](t, store.DB, "SELECT COUNT(*) FROM pdf_documents WHERE change_counter = 1"); got != 2 {
		store.Close()
		t.Fatalf("document change counters = %d, want two preserved registrations", got)
	}
	if got := queryCompanionOne[int](t, store.DB, "SELECT COUNT(*) FROM pdf_download_attempts WHERE id IN (1, 2)"); got != 2 {
		store.Close()
		t.Fatalf("legacy attempt count = %d", got)
	}
	if got := queryCompanionOne[int](t, store.DB, "SELECT seq FROM sqlite_sequence WHERE name = 'pdf_download_attempts'"); got != 2 {
		store.Close()
		t.Fatalf("attempt sequence = %d, want 2", got)
	}
	if got := queryCompanionOne[int](t, store.DB, "SELECT COUNT(*) FROM pdf_audit_outbox WHERE event_key = 'legacy-pending' AND before_json IS NULL AND after_json IS NULL AND delivered_at IS NULL"); got != 1 {
		store.Close()
		t.Fatalf("legacy pending outbox count = %d", got)
	}
	for _, trigger := range []string{"pdf_blobs_abort_update", "pdf_documents_abort_delete", "pdf_documents_advance_counter", "pdf_download_attempts_abort_update", "pdf_download_attempts_abort_delete"} {
		if got := queryCompanionOne[int](t, store.DB, "SELECT COUNT(*) FROM sqlite_master WHERE type = 'trigger' AND name = ?", trigger); got != 1 {
			store.Close()
			t.Fatalf("missing trigger %s", trigger)
		}
	}
	foreignKeys := companionForeignKeyTables(t, store.DB, "pdf_download_attempts")
	if !slices.Equal(foreignKeys, []string{"pdf_gather_runs"}) {
		store.Close()
		t.Fatalf("download attempt foreign keys = %v, want only pdf_gather_runs", foreignKeys)
	}

	attemptHash := strings.Repeat("b", 64)
	if _, err := store.DB.Exec("DELETE FROM pdf_blobs WHERE content_hash = ?", attemptHash); err != nil {
		store.Close()
		t.Fatalf("delete attempt-only blob: %v", err)
	}
	if got := queryCompanionOne[int](t, store.DB, "SELECT COUNT(*) FROM pdf_download_attempts WHERE id = 2 AND content_hash = ?", attemptHash); got != 1 {
		store.Close()
		t.Fatalf("attempt logical hash row = %d", got)
	}
	if _, err := store.DB.Exec("DELETE FROM pdf_blobs WHERE content_hash = ?", sharedHash); err == nil {
		store.Close()
		t.Fatal("shared document blob was deleted while still selected")
	}
	if rows := companionForeignKeyViolations(t, store.DB); len(rows) != 0 {
		store.Close()
		t.Fatalf("foreign key violations = %v", rows)
	}
	result, err := store.DB.Exec(`INSERT INTO pdf_download_attempts
		(gather_run_id, doi, source, started_at, outcome, content_hash)
		VALUES (1, '10.1000/new', 'legacy', '2026-01-01T00:02:00Z', 'failed', ?)`, strings.Repeat("c", 64))
	if err != nil {
		store.Close()
		t.Fatalf("insert logical hash attempt: %v", err)
	}
	newID, err := result.LastInsertId()
	if err != nil || newID != 3 {
		store.Close()
		t.Fatalf("new attempt id = %d err=%v, want 3", newID, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := pdfstore.Open(pdfPath, registry)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if got := appliedCompanionMigrations(t, pdfPath); len(got) != len(entries) || got[len(got)-1] != entries[len(entries)-1].Filename {
		t.Fatalf("companion history = %v", got)
	}
	if got := queryCompanionOne[int](t, reopened.DB, "SELECT COUNT(*) FROM pdf_download_attempts"); got != 3 {
		t.Fatalf("attempt count after repeat preparation = %d", got)
	}
}

// queryCompanionOne runs one bounded inspection query.
func queryCompanionOne[T any](t *testing.T, db *sql.DB, query string, args ...any) T {
	t.Helper()
	var value T
	if err := db.QueryRow(query, args...).Scan(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

// companionForeignKeyTables returns the referenced tables of one table's foreign keys.
func companionForeignKeyTables(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()
	rows, err := db.Query("PRAGMA foreign_key_list(" + table + ")")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var id, sequence int
		var referenced, from, to, onUpdate, onDelete, match string
		if err := rows.Scan(&id, &sequence, &referenced, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, referenced)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return tables
}

// companionForeignKeyViolations returns every foreign-key violation row.
func companionForeignKeyViolations(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var violations []string
	for rows.Next() {
		var table, parent string
		var rowID int64
		var constraint int
		if err := rows.Scan(&table, &rowID, &parent, &constraint); err != nil {
			t.Fatal(err)
		}
		violations = append(violations, table)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return violations
}

// appliedCompanionMigrations reads one companion store's migration history.
func appliedCompanionMigrations(t *testing.T, path string) []string {
	t.Helper()
	conn, err := sqliteread.Open(path, "sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.Ping(); err != nil {
		t.Fatal(err)
	}
	applied, err := migrations.Applied(context.Background(), conn)
	if err != nil {
		t.Fatal(err)
	}
	return applied
}
