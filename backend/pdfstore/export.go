// export.go provides a pinned companion read snapshot for explicit PDF
// exports. The snapshot holds one SQLite read transaction so preflight and
// emission cannot observe different selections for the same archive.
package pdfstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"analysis/backend/database/work"
	generated "analysis/backend/pdfstore/internal/sql"
)

// ExportSnapshot is one bounded companion read transaction used by an export.
type ExportSnapshot struct {
	tx      *sql.Tx
	queries *generated.Queries
	closed  bool
}

// BeginExportSnapshot starts one read transaction over the already configured
// companion connection. Callers must close it on every path.
func (s *Store) BeginExportSnapshot(ctx context.Context) (*ExportSnapshot, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("begin PDF export snapshot: %w", err)
	}
	return &ExportSnapshot{tx: tx, queries: s.queries.WithTx(tx)}, nil
}

// Available reads and validates one available document inside the pinned
// snapshot. It returns nil document and nil data when the DOI has no selected
// content. A stored blob that fails identity validation returns ErrPDFIntegrity.
func (s *ExportSnapshot) Available(ctx context.Context, doi string) (*AvailableDocument, []byte, error) {
	if s == nil || s.closed {
		return nil, nil, fmt.Errorf("PDF export snapshot is closed")
	}
	row, err := s.queries.GetAvailablePDFDocument(ctx, work.NormalizeDOI(doi))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get available PDF document: %w", err)
	}
	document := &AvailableDocument{
		ContentHash: row.ContentHash.String, InventoriedAt: row.InventoriedAt.String,
		ByteSize: row.ByteSize, ChangeCounter: row.ChangeCounter,
	}
	if document.ContentHash == "" || document.ByteSize < 1 || document.ByteSize > DefaultMaxPDFBytes {
		return nil, nil, ErrPDFIntegrity
	}
	data, err := s.queries.GetPDFBlobData(ctx, document.ContentHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrPDFIntegrity
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get PDF blob data: %w", err)
	}
	digest, err := ValidatePDF(data, DefaultMaxPDFBytes)
	if err != nil || digest != document.ContentHash || int64(len(data)) != document.ByteSize {
		return nil, nil, ErrPDFIntegrity
	}
	return document, data, nil
}

// Close releases the pinned read transaction.
func (s *ExportSnapshot) Close() error {
	if s == nil || s.closed {
		return nil
	}
	s.closed = true
	if err := s.tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return err
	}
	return nil
}
