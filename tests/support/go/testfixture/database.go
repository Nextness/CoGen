// database.go owns configured connections used by family integration fixtures.
package testfixture

import (
	"analysis/backend/database"
	"analysis/internal/sqliteuri"
	"database/sql"
	"testing"
)

// Database keeps fixture SQL on the same configured connection as its family stores.
type Database struct {
	*database.Database
	Conn *sql.DB
	Path string
}

// Open applies production metadata migrations and binds all fixture stores to that connection.
func Open(path, registry string) (*Database, error) {
	conn, err := database.OpenConfigured(path, registry, database.StoreCorpusMetadata)
	if err != nil {
		return nil, err
	}
	return &Database{Database: database.New(conn), Conn: conn, Path: path}, nil
}

// observe opens a test-owned inspection connection after a composition-root API succeeds.
// This connection observes committed state only; transaction and connection-local assertions use Open's owned Conn.
func observe(t testing.TB, opened *database.Database, path string, err error) (*Database, error) {
	t.Helper()
	if err != nil {
		return nil, err
	}
	conn, err := sql.Open("sqlite", sqliteuri.File(path, map[string][]string{"mode": {"rw"}, "_pragma": {"busy_timeout(5000)", "foreign_keys(1)"}}))
	if err != nil {
		_ = opened.Close()
		return nil, err
	}
	conn.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = conn.Close() })
	return &Database{Database: opened, Conn: conn, Path: path}, nil
}

// OpenPlatform exercises the production composition root and exposes a separate committed-state observer.
func OpenPlatform(t testing.TB, path, registry string) (*Database, error) {
	opened, err := database.Open(path, registry)
	return observe(t, opened, path, err)
}

// OpenExistingPlatform exercises existing-only opening before creating its committed-state observer.
func OpenExistingPlatform(t testing.TB, path string) (*Database, error) {
	opened, err := database.OpenExisting(path)
	return observe(t, opened, path, err)
}

// OpenExistingDriverPlatform exercises driver-selectable existing-only opening before observation.
func OpenExistingDriverPlatform(t testing.TB, path, driver string) (*Database, error) {
	opened, err := database.OpenExistingWithDriver(path, driver)
	return observe(t, opened, path, err)
}

// ClosedDB returns a closed lazy SQLite handle for proving validation happens before storage access.
// Closing before the first operation means this helper opens no database or filesystem resource.
func ClosedDB() *sql.DB {
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		panic(err)
	}
	if err := conn.Close(); err != nil {
		panic(err)
	}
	return conn
}
