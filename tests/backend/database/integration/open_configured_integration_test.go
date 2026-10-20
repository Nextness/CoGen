// open_configured_integration_test.go verifies writable database URIs preserve filesystem paths.

package database_test

import "analysis/tests/support/go/testenv"

import subject "analysis/backend/database"

import (
	"path/filepath"
	"testing"
)

// TestOpenConfiguredEscapesDatabasePath verifies URI-significant filename bytes remain part of the SQLite filename.
func TestOpenConfiguredEscapesDatabasePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corpus #? space.db")
	conn, err := subject.OpenConfigured(path, testenv.Path("config", "database.something"), subject.StoreCorpusMetadata)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var enabled int
	if err := conn.QueryRow("PRAGMA foreign_keys").Scan(&enabled); err != nil {
		t.Fatal(err)
	}
	if enabled != 1 {
		t.Fatalf("foreign_keys = %d, want 1", enabled)
	}
}
