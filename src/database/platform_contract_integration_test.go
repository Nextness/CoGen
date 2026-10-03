// platform_contract_integration_test.go verifies the connection and migration
// contracts preserved by the platform split.
//go:build integration

package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// metadataMigrationEntries resolves the production metadata migration chain used by the platform contract tests.
func metadataMigrationEntries(t *testing.T) []migrationEntry {
	t.Helper()
	config, err := ResolveMigrationConfig(testConfigPath, StoreCorpusMetadata)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := loadMigrationChain(config.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

// TestOpenConfiguredConnectionPragmas verifies writable connections retain WAL, foreign-key, and busy-timeout settings.
func TestOpenConfiguredConnectionPragmas(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corpus.metadata.db")
	conn, err := OpenConfigured(path, testConfigPath, StoreCorpusMetadata)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	var journalMode string
	if err := conn.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatal(err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}
	var foreignKeys, busyTimeout int
	if err := conn.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 || busyTimeout != 5000 {
		t.Fatalf("pragmas foreign_keys=%d busy_timeout=%d, want 1 and 5000", foreignKeys, busyTimeout)
	}
}

// TestOpenExistingConnectionContract verifies existing-only connections stay writable with the shared foreign-key and busy-timeout pragmas.
func TestOpenExistingConnectionContract(t *testing.T) {
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
	if _, err := db.DB.Exec("INSERT INTO searches (search_id) VALUES ('contract')"); err != nil {
		t.Fatalf("existing-only connection rejected a write: %v", err)
	}
	var foreignKeys, busyTimeout int
	if err := db.DB.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 || busyTimeout != 5000 {
		t.Fatalf("pragmas foreign_keys=%d busy_timeout=%d, want 1 and 5000", foreignKeys, busyTimeout)
	}
}

// TestOpenExistingWithDriverRejectsMissingDatabase verifies driver-selectable existing-only opens never create databases or accept invalid paths.
func TestOpenExistingWithDriverRejectsMissingDatabase(t *testing.T) {
	directory := t.TempDir()
	if _, err := OpenExistingWithDriver(filepath.Join(directory, "missing.db"), "sqlite"); err == nil {
		t.Fatal("OpenExistingWithDriver created a missing database")
	}
	if _, err := OpenExistingWithDriver(filepath.Join(directory, "missing.db"), ""); err == nil || !strings.Contains(err.Error(), "driver") {
		t.Fatalf("OpenExistingWithDriver empty driver error = %v", err)
	}
	if _, err := OpenExisting(directory); err == nil {
		t.Fatal("OpenExisting accepted a directory")
	}
}

// TestSchemaVersionContract verifies schema-version reporting returns the last applied migration and an empty version without error when none has been recorded.
func TestSchemaVersionContract(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corpus.metadata.db")
	metadata, err := Open(path, testConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	version, err := metadata.SchemaVersion()
	if err != nil {
		metadata.Close()
		t.Fatal(err)
	}
	if err := metadata.Close(); err != nil {
		t.Fatal(err)
	}
	entries := metadataMigrationEntries(t)
	if len(entries) == 0 || version != entries[len(entries)-1].filename {
		t.Fatalf("schema version = %q, want last configured migration", version)
	}

	emptyPath := filepath.Join(t.TempDir(), "empty-tracking.db")
	seed, err := sql.Open("sqlite", emptyPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := seed.Exec(`CREATE TABLE schema_migrations (
		filename   TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (datetime('now')),
		checksum   TEXT NOT NULL
	)`); err != nil {
		seed.Close()
		t.Fatal(err)
	}
	if err := seed.Close(); err != nil {
		t.Fatal(err)
	}
	empty, err := OpenExisting(emptyPath)
	if err != nil {
		t.Fatal(err)
	}
	version, err = empty.SchemaVersion()
	if err != nil || version != "" {
		empty.Close()
		t.Fatalf("empty schema version = %q err=%v, want empty nil", version, err)
	}
	if err := empty.Close(); err != nil {
		t.Fatal(err)
	}

	uninitializedPath := filepath.Join(t.TempDir(), "uninitialized.db")
	if err := os.WriteFile(uninitializedPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	uninitialized, err := OpenExisting(uninitializedPath)
	if err != nil {
		t.Fatal(err)
	}
	defer uninitialized.Close()
	if version, err := uninitialized.SchemaVersion(); err != nil || version != "" {
		t.Fatalf("uninitialized schema version = %q err=%v, want empty nil", version, err)
	}
}

// TestConcurrentOpenAppliesMigrationsOnce verifies the migration lock keeps simultaneous openers from applying or recording migrations twice.
func TestConcurrentOpenAppliesMigrationsOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corpus.metadata.db")
	const openers = 3
	results := make(chan error, openers)
	for i := 0; i < openers; i++ {
		go func() {
			opened, err := Open(path, testConfigPath)
			if err == nil {
				err = opened.Close()
			}
			results <- err
		}()
	}
	for i := 0; i < openers; i++ {
		if err := <-results; err != nil {
			t.Fatalf("concurrent Open: %v", err)
		}
	}
	metadata, err := OpenExisting(path)
	if err != nil {
		t.Fatal(err)
	}
	defer metadata.Close()
	var applied int
	if err := metadata.DB.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&applied); err != nil {
		t.Fatal(err)
	}
	entries := metadataMigrationEntries(t)
	if applied != len(entries) {
		t.Fatalf("applied migrations = %d, want %d", applied, len(entries))
	}
}
