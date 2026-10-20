// Shared helpers for database test files.
package testhelpers

import "analysis/tests/support/go/testfixture"

import "analysis/tests/support/go/testenv"

import (
	"path/filepath"
	"testing"
)

// configPath is resolved relative to src/ (where tests run).
var TestConfigPath = testenv.Path("config", "database.something")

// OpenTestDB supports the package test suite's open test db setup or assertions.
func OpenTestDB(t *testing.T) *testfixture.Database {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := testfixture.Open(dbPath, TestConfigPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return db
}
