// platform_methods_integration_test.go verifies health and writability checks
// through the metadata platform's narrow API.

package database_test

import subject "analysis/backend/database"

import testhelpers "analysis/tests/support/go/backend/database"

import (
	"context"
	"path/filepath"
	"testing"
)

// TestDatabasePingContext verifies the platform ping reports open and closed connections.
func TestDatabasePingContext(t *testing.T) {
	db := testhelpers.OpenTestDB(t)
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
	conn, err := subject.OpenConfigured(path, testhelpers.TestConfigPath, subject.StoreCorpusMetadata)
	if err != nil {
		t.Fatal(err)
	}
	db := subject.New(conn)
	defer db.Close()

	ctx := context.Background()
	writable, err := db.Writable(ctx)
	if err != nil || !writable {
		t.Fatalf("Writable = %v err=%v, want true", writable, err)
	}
	if _, err := conn.Exec("PRAGMA query_only=1"); err != nil {
		t.Fatal(err)
	}
	writable, err = db.Writable(ctx)
	if err != nil || writable {
		t.Fatalf("Writable = %v err=%v, want false", writable, err)
	}
}
