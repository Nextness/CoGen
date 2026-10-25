// schema.go verifies the companion schema exposes the document-management
// columns, indexes, and protection triggers required before writes.
package pdfstore

import (
	"context"
	"database/sql"
	"fmt"
)

// VerifySchema verifies that the companion database contains the V00003
// document-management columns, content-hash index, outbox evidence columns,
// and protection triggers. It is a readiness check and does not migrate.
func VerifySchema(ctx context.Context, db *sql.DB) error {
	requiredTables := []struct {
		name    string
		columns []string
	}{
		{"pdf_blobs", []string{"content_hash", "byte_size", "data"}},
		{"pdf_documents", []string{"doi", "status", "content_hash", "inventoried_at", "change_counter"}},
		{"pdf_audit_outbox", []string{"event_key", "before_json", "after_json", "delivered_at"}},
		{"pdf_download_attempts", []string{"content_hash"}},
	}
	for _, required := range requiredTables {
		var found int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", required.name).Scan(&found); err != nil {
			return fmt.Errorf("inspect companion PDF table %q: %w", required.name, err)
		}
		if found != 1 {
			return fmt.Errorf("companion PDF store is missing required table %q; run analysis migrate", required.name)
		}
		columns, err := companionColumns(ctx, db, required.name)
		if err != nil {
			return err
		}
		for _, column := range required.columns {
			if !columns[column] {
				return fmt.Errorf("companion PDF store table %q is missing required column %q; run analysis migrate", required.name, column)
			}
		}
	}
	var indexFound int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_pdf_documents_content_hash'").Scan(&indexFound); err != nil {
		return fmt.Errorf("inspect companion PDF content-hash index: %w", err)
	}
	if indexFound != 1 {
		return fmt.Errorf("companion PDF store is missing required content-hash index; run analysis migrate")
	}
	for _, trigger := range []string{
		"pdf_documents_abort_delete",
		"pdf_documents_advance_counter",
		"pdf_blobs_abort_update",
		"pdf_download_attempts_abort_update",
		"pdf_download_attempts_abort_delete",
	} {
		var found int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='trigger' AND name=?", trigger).Scan(&found); err != nil {
			return fmt.Errorf("inspect companion PDF trigger %q: %w", trigger, err)
		}
		if found != 1 {
			return fmt.Errorf("companion PDF store is missing required protection trigger %q; run analysis migrate", trigger)
		}
	}
	return nil
}

// companionColumns returns the column names present in one companion table.
func companionColumns(ctx context.Context, db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%q)", table))
	if err != nil {
		return nil, fmt.Errorf("inspect companion PDF table %q: %w", table, err)
	}
	defer rows.Close()
	columns := make(map[string]bool)
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, fmt.Errorf("inspect companion PDF table %q: %w", table, err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("inspect companion PDF table %q: %w", table, err)
	}
	return columns, nil
}
