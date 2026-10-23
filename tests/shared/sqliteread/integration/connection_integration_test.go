// Package sqliteread_test verifies the actual read pool used for metadata and PDF delivery.
package sqliteread_test

import (
	"analysis/internal/sqliteread"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// TestReadPoolCannotCreateOrWrite verifies the factory configures the production connection itself.
func TestReadPoolCannotCreateOrWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "read-only.db")
	missing, err := sqliteread.Open(path, "sqlite")
	if err != nil {
		t.Fatal(err)
	}
	if err := missing.Ping(); err == nil {
		t.Fatal("read pool created a missing database")
	}
	_ = missing.Close()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("missing database stat=%v", err)
	}
	writable, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writable.Exec("CREATE TABLE evidence (id INTEGER)"); err != nil {
		t.Fatal(err)
	}
	if err := writable.Close(); err != nil {
		t.Fatal(err)
	}
	reader, err := sqliteread.Open(path, "sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if err := reader.Ping(); err != nil {
		t.Fatal(err)
	}
	var queryOnly int
	if err := reader.QueryRow("PRAGMA query_only").Scan(&queryOnly); err != nil || queryOnly != 1 {
		t.Fatalf("query_only=%d err=%v, want 1", queryOnly, err)
	}
	if _, err := reader.Exec("INSERT INTO evidence VALUES (1)"); err == nil {
		t.Fatal("read pool accepted a write")
	}
	if reader.Stats().MaxOpenConnections != 1 {
		t.Fatalf("pool=%+v", reader.Stats())
	}
}
