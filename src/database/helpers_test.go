// Shared helpers for database test files.
package database

import (
	"path/filepath"
	"testing"
)

// configPath is resolved relative to src/ (where tests run).
var testConfigPath = filepath.Join("..", "..", "config", "database.something")

// openTestDB supports the package test suite's open test db setup or assertions.
func openTestDB(t *testing.T) *Database {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(dbPath, testConfigPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return db
}
