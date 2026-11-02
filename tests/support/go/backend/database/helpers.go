// Shared helpers for database test files.
package testhelpers

import "analysis/backend/database"

import "analysis/internal/migrations"

import "analysis/tests/support/go/testfixture"

import "analysis/tests/support/go/testenv"

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestConfigPath locates the production migration registry from the repository root.
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

// MigrationEntries loads one production migration chain in declaration order.
func MigrationEntries(t *testing.T, configName string) []migrations.Entry {
	t.Helper()
	entries, err := migrations.LoadChain(testenv.Path("config", configName))
	if err != nil {
		t.Fatalf("load %s: %v", configName, err)
	}
	return entries
}

// ApplyMigrationPrefix creates one store at a production chain prefix.
func ApplyMigrationPrefix(t *testing.T, dbPath, migrationsDir string, entries []migrations.Entry) {
	t.Helper()
	conn, err := database.OpenWritable(dbPath)
	if err != nil {
		t.Fatalf("open prefix database: %v", err)
	}
	defer conn.Close()
	configPath := filepath.Join(t.TempDir(), "migration-prefix.something")
	WriteMigrationConfig(t, configPath, entries)
	if _, err := migrations.ApplyContext(context.Background(), conn, migrationsDir, configPath); err != nil {
		t.Fatalf("apply migration prefix: %v", err)
	}
}

// WriteMigrationConfig writes one synthetic migration configuration for entries.
func WriteMigrationConfig(t *testing.T, path string, entries []migrations.Entry) {
	t.Helper()
	var builder strings.Builder
	builder.WriteString("db_migration: setup = { filename: string; }\n")
	for _, entry := range entries {
		fmt.Fprintf(&builder, "#iteration(\"_db_migration\"): db_migration = { filename = %q, };\n", entry.Filename)
	}
	if err := os.WriteFile(path, []byte(builder.String()), 0o644); err != nil {
		t.Fatalf("write migration config: %v", err)
	}
}
