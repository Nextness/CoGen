// migration_sql_integration_test.go verifies invalid migration files cannot be recorded.
//go:build integration

package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// TestRunMigrationsRejectsInvalidMarkersWithoutTrackingRow verifies malformed SQL is rejected before its schema or tracking row is committed.
func TestRunMigrationsRejectsInvalidMarkersWithoutTrackingRow(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	migrationsDir := filepath.Join(root, "migrations")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(migrationsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "database.something")
	config := `db_migration: setup = { filename: string; }
#iteration("_db_migration"): db_migration = { filename = "V00001_bad.sql"; };`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migrationsDir, "V00001_bad.sql"), []byte("-- ==DOWN==\nCREATE TABLE escaped (id INTEGER);\n-- ==UP=="), 0o644); err != nil {
		t.Fatal(err)
	}
	conn, err := sql.Open("sqlite", filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	db := &Database{DB: conn, migrations: migrationsDir}
	if err := db.runMigrations(configPath); err == nil {
		t.Fatal("expected invalid migration marker error")
	}
	var tracked, table int
	if err := conn.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE filename='V00001_bad.sql'").Scan(&tracked); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='escaped'").Scan(&table); err != nil {
		t.Fatal(err)
	}
	if tracked != 0 || table != 0 {
		t.Fatalf("invalid migration tracked=%d table=%d, want neither", tracked, table)
	}
}
