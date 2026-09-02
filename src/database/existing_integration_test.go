//go:build integration

package database

import (
	"os"
	"path/filepath"
	"testing"
)

// TestExistingDatabaseLifecycle verifies migration is explicit and existing-only opening never creates or migrates.
func TestExistingDatabaseLifecycle(t *testing.T) {
	directory := t.TempDir()
	missing := filepath.Join(directory, "missing.db")
	registry := filepath.Join("..", "..", "config", "database.something")
	if _, err := OpenExisting(missing); err == nil {
		t.Fatal("OpenExisting created a missing database")
	}
	if err := MigrateExisting(missing, registry); err == nil {
		t.Fatal("MigrateExisting created a missing database")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("missing database was created: %v", err)
	}
	path := filepath.Join(directory, "existing.db")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	existing, err := OpenExisting(path)
	if err != nil {
		t.Fatal(err)
	}
	var tables int
	if err := existing.DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables != 0 {
		t.Fatalf("OpenExisting applied schema: tables=%d", tables)
	}
	if err := existing.Close(); err != nil {
		t.Fatal(err)
	}
	if err := MigrateExisting(path, registry); err != nil {
		t.Fatal(err)
	}
	migrated, err := OpenExisting(path)
	if err != nil {
		t.Fatal(err)
	}
	defer migrated.Close()
	var migrations int
	if err := migrated.DB.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&migrations); err != nil || migrations != 27 {
		t.Fatalf("migrations=%d err=%v", migrations, err)
	}
}
