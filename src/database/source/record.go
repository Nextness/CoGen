// record.go provides the handwritten source-record data access layer over the
// generated statements. Raw payloads, content identities, parse statuses, and
// rejection reasons stay exact and records keep their declared index order.
package source

import (
	"context"
	"fmt"

	generated "analysis/database/source/internal/sql"
)

// CreateRecord inserts a new source record. Returns the record ID.
func (s *Store) CreateRecord(ctx context.Context, input CreateRecordInput) (int64, error) {
	result, err := s.queries.InsertSourceRecord(ctx, generated.InsertSourceRecordParams{
		RunSourceID: input.RunSourceID,
		RecordIndex: int64(input.RecordIndex),
		RawPayload:  input.RawPayload,
		ContentHash: input.ContentHash,
	})
	if err != nil {
		return 0, fmt.Errorf("create source record: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// UpdateRecordStatus updates the parse status and optional reject reason for a source record.
func (s *Store) UpdateRecordStatus(ctx context.Context, input RecordStatusInput) error {
	return s.queries.UpdateSourceRecordParseStatus(ctx, generated.UpdateSourceRecordParseStatusParams{
		ParseStatus:  input.Status,
		RejectReason: nullableString(input.RejectReason),
		ID:           input.RecordID,
	})
}

// ListRecordsBySource returns all records for a given run source, ordered by record index.
func (s *Store) ListRecordsBySource(ctx context.Context, runSourceID int64) ([]*Record, error) {
	rows, err := s.queries.ListSourceRecordsBySource(ctx, runSourceID)
	if err != nil {
		return nil, err
	}
	var records []*Record
	for _, row := range rows {
		records = append(records, recordFromGenerated(row))
	}
	return records, nil
}

// CountRecordsBySource returns the number of records for a given run source.
func (s *Store) CountRecordsBySource(ctx context.Context, runSourceID int64) (int, error) {
	count, err := s.queries.CountSourceRecordsBySource(ctx, runSourceID)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// recordFromGenerated maps one generated source-record row into an application
// record and makes the nullable rejection reason explicit.
func recordFromGenerated(row generated.SourceRecord) *Record {
	record := &Record{
		ID:          row.ID,
		RunSourceID: row.RunSourceID,
		RecordIndex: int(row.RecordIndex),
		RawPayload:  row.RawPayload,
		ContentHash: row.ContentHash,
		ParseStatus: row.ParseStatus,
		CreatedAt:   row.CreatedAt,
	}
	if row.RejectReason.Valid {
		record.RejectReason = row.RejectReason.String
	}
	return record
}
