//go:build integration || e2e

// Package testfixture seeds historical and partial rows for database integration tests.
// Production writers use the guarded family APIs instead.
package testfixture

import (
	"context"
	"database/sql"
)

// Run inserts an unplanned run so tests can exercise nullable ancestry and isolated run-owned evidence.
func Run(ctx context.Context, db *sql.DB, step, query string) (int64, error) {
	result, err := db.ExecContext(ctx, `INSERT INTO pipeline_runs (step, started_at, status, search_query)
		VALUES (?, datetime('now'), 'running', ?)`, step, sql.NullString{String: query, Valid: query != ""})
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// Artifact inserts metadata without bytes to exercise missing blobs and relationship projections.
func Artifact(ctx context.Context, db *sql.DB, hash, contentType string, byteSize int64) (int64, error) {
	result, err := db.ExecContext(ctx, `INSERT INTO artifacts (content_hash, content_type, byte_size) VALUES (?, ?, ?)`, hash, contentType, byteSize)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}
