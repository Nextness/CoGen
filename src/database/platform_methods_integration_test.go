// platform_methods_integration_test.go verifies the narrow platform methods
// that let health checks and approved coordinators avoid the exported raw
// connection.
//go:build integration

package database

import (
	"context"
	"path/filepath"
	"testing"
)

// TestDatabasePingContext verifies the platform ping reports open and closed connections.
func TestDatabasePingContext(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("PingContext open = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := db.PingContext(ctx); err == nil {
		t.Fatal("PingContext closed connection succeeded")
	}
}

// TestDatabaseWritableReflectsQueryOnly verifies the platform writability check follows the connection-local pragma.
func TestDatabaseWritableReflectsQueryOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corpus.metadata.db")
	created, err := Open(path, testConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := created.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := OpenExisting(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	writable, err := db.Writable(ctx)
	if err != nil || !writable {
		t.Fatalf("Writable = %v err=%v, want true", writable, err)
	}
	if _, err := db.DB.Exec("PRAGMA query_only=1"); err != nil {
		t.Fatal(err)
	}
	writable, err = db.Writable(ctx)
	if err != nil || writable {
		t.Fatalf("Writable = %v err=%v, want false", writable, err)
	}
}

// TestDatabaseBeginTxCommitAndRollback verifies the platform transaction entry point commits and rolls back related writes.
func TestDatabaseBeginTxCommitAndRollback(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO searches (search_id) VALUES ('rollback')"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	assertSearchCount(t, db, "rollback", 0)

	tx, err = db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO searches (search_id) VALUES ('commit')"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	assertSearchCount(t, db, "commit", 1)
}

// assertSearchCount verifies the number of searches with the supplied identifier.
func assertSearchCount(t *testing.T, db *Database, searchID string, want int) {
	t.Helper()
	var count int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM searches WHERE search_id=?", searchID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("searches %q = %d, want %d", searchID, count, want)
	}
}
