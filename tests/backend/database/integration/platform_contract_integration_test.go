// platform_contract_integration_test.go verifies the connection and migration
// contracts preserved by the platform split.

package database_test

import "analysis/tests/support/go/testfixture"

import "analysis/internal/migrations"

import subject "analysis/backend/database"
import testhelpers "analysis/tests/support/go/backend/database"

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"modernc.org/sqlite"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// metadataMigrationEntries resolves the production metadata migration chain used by the platform contract tests.
func metadataMigrationEntries(t *testing.T) []migrations.Entry {
	t.Helper()
	config, err := subject.ResolveMigrationConfig(testhelpers.TestConfigPath, subject.StoreCorpusMetadata)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := migrations.LoadChain(config.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

// TestOpenConfiguredConnectionPragmas verifies writable connections retain WAL, foreign-key, and busy-timeout settings.
func TestOpenConfiguredConnectionPragmas(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corpus.metadata.db")
	conn, err := subject.OpenConfigured(path, testhelpers.TestConfigPath, subject.StoreCorpusMetadata)
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
	created, err := testfixture.OpenPlatform(t, path, testhelpers.TestConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := created.Close(); err != nil {
		t.Fatal(err)
	}

	probe := &connectionProbe{}
	driverName := fmt.Sprintf("test-existing-contract-%d", connectionProbeID.Add(1))
	sql.Register(driverName, probe)
	db, err := subject.OpenExistingWithDriver(path, driverName)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Search.FindOrCreateSearch(context.Background(), "contract"); err != nil {
		t.Fatalf("existing-only connection rejected a write: %v", err)
	}
	probe.mu.Lock()
	defer probe.mu.Unlock()
	if probe.foreignKeys != int64(1) || probe.busyTimeout != int64(5000) {
		t.Fatalf("production connection pragmas foreign_keys=%v busy_timeout=%v", probe.foreignKeys, probe.busyTimeout)
	}
	if !strings.Contains(probe.uri, "mode=rw") || !strings.Contains(probe.uri, "_txlock=immediate") {
		t.Fatalf("existing-only URI=%q", probe.uri)
	}
}

// TestOpenExistingWithDriverRejectsMissingDatabase verifies driver-selectable existing-only opens never create databases or accept invalid paths.
func TestOpenExistingWithDriverRejectsMissingDatabase(t *testing.T) {
	directory := t.TempDir()
	if _, err := testfixture.OpenExistingDriverPlatform(t, filepath.Join(directory, "missing.db"), "sqlite"); err == nil {
		t.Fatal("OpenExistingWithDriver created a missing database")
	}
	if _, err := testfixture.OpenExistingDriverPlatform(t, filepath.Join(directory, "missing.db"), ""); err == nil || !strings.Contains(err.Error(), "driver") {
		t.Fatalf("OpenExistingWithDriver empty driver error = %v", err)
	}
	if _, err := testfixture.OpenExistingPlatform(t, directory); err == nil {
		t.Fatal("OpenExisting accepted a directory")
	}
}

// TestSchemaVersionContract verifies schema-version reporting returns the last applied migration and an empty version without error when none has been recorded.
func TestSchemaVersionContract(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corpus.metadata.db")
	metadata, err := testfixture.OpenPlatform(t, path, testhelpers.TestConfigPath)
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
	if len(entries) == 0 || version != entries[len(entries)-1].Filename {
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
	empty, err := testfixture.OpenExistingPlatform(t, emptyPath)
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
	uninitialized, err := testfixture.OpenExistingPlatform(t, uninitializedPath)
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
			opened, err := testfixture.OpenPlatform(t, path, testhelpers.TestConfigPath)
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
	metadata, err := testfixture.OpenExistingPlatform(t, path)
	if err != nil {
		t.Fatal(err)
	}
	defer metadata.Close()
	var applied int
	if err := metadata.Conn.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&applied); err != nil {
		t.Fatal(err)
	}
	entries := metadataMigrationEntries(t)
	if applied != len(entries) {
		t.Fatalf("applied migrations = %d, want %d", applied, len(entries))
	}
}

// connectionProbeID gives each standard-driver probe its own registration.
var connectionProbeID atomic.Uint64

// connectionProbe inspects the actual connection returned to the production composition root.
type connectionProbe struct {
	mu                       sync.Mutex
	uri                      string
	foreignKeys, busyTimeout driver.Value
}

// Open records configured pragmas before handing the same SQLite connection to the application.
func (p *connectionProbe) Open(uri string) (driver.Conn, error) {
	conn, err := (&sqlite.Driver{}).Open(uri)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.uri = uri
	for _, item := range []struct {
		query  string
		target *driver.Value
	}{
		{"PRAGMA foreign_keys", &p.foreignKeys}, {"PRAGMA busy_timeout", &p.busyTimeout},
	} {
		rows, queryErr := conn.(driver.QueryerContext).QueryContext(context.Background(), item.query, nil)
		if queryErr != nil {
			_ = conn.Close()
			return nil, queryErr
		}
		values := make([]driver.Value, 1)
		queryErr = rows.Next(values)
		_ = rows.Close()
		if queryErr != nil {
			_ = conn.Close()
			return nil, queryErr
		}
		*item.target = values[0]
	}
	return conn, nil
}
