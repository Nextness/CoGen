// platform_methods_integration_test.go verifies health and writability checks
// through the metadata platform's narrow API.
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
	if _, err := db.RawDBForTest().Exec("PRAGMA query_only=1"); err != nil {
		t.Fatal(err)
	}
	writable, err = db.Writable(ctx)
	if err != nil || writable {
		t.Fatalf("Writable = %v err=%v, want false", writable, err)
	}
}
