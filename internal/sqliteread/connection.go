// Package sqliteread constructs existing-only query-only SQLite connections for viewer metadata and companion reads.
package sqliteread

import (
	"database/sql"
	"path/filepath"

	"analysis/internal/sqliteuri"
)

// Open creates a lazy read-only pool through the registered driver; callers verify readability with PingContext.
func Open(path, driverName string) (*sql.DB, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	conn, err := sql.Open(driverName, sqliteuri.File(absolute, map[string][]string{"mode": {"ro"}, "_pragma": {"query_only(1)"}}))
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(1)
	conn.SetConnMaxLifetime(0)
	return conn, nil
}
