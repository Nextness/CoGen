// viewer.go provides the source family's viewer reads over the family
// connection. It owns the run source result-count projection, including the
// pre-result-count schema compatibility variants, and the run-scoped source
// record page behind the corpus browser.
package source

import (
	"context"
	"database/sql"
	"fmt"
)

// ResultCountFilter identifies one run source result-count projection. The
// include flags describe which optional columns the opened database provides,
// so the viewer can read databases created before the result-count migration.
type ResultCountFilter struct {
	RunID               int64
	IncludeExportDate   bool
	IncludeResultCounts bool
}

// ResultCountRow is one run source with its optional export metadata. Optional
// fields stay nil when the column is absent or the stored value is NULL.
type ResultCountRow struct {
	ID                    int64
	SourceName            string
	SourceType            string
	ExpectedFile          string
	Query                 *string
	ExpectedResultCount   *int64
	ObservedResultCount   *int64
	ResultCountComparison *string
	ExportDate            *string
}

// CorpusRecordFilter identifies one bounded, searchable page of run-scoped
// source records.
type CorpusRecordFilter struct {
	RunID   int64
	Query   string
	Sort    string
	Order   string
	Page    int
	PerPage int
}

// CorpusRecord is one raw source record shown in the corpus browser.
type CorpusRecord struct {
	ID           int64
	RunSourceID  int64
	SourceName   string
	SourceType   string
	RecordIndex  int64
	ParseStatus  string
	RejectReason *string
	ContentHash  string
	CreatedAt    string
}

// CorpusRecordPage is one bounded page of run-scoped source records.
type CorpusRecordPage struct {
	Items []*CorpusRecord
	Total int64
}

// ListResultCountsForRun returns the stored source inventory and result-count
// evidence for a run in ascending ID order. Columns absent from the opened
// database are projected as NULL instead of failing the read.
func (s *Store) ListResultCountsForRun(ctx context.Context, filter ResultCountFilter) ([]*ResultCountRow, error) {
	switch {
	case filter.IncludeExportDate && filter.IncludeResultCounts:
		rows, err := s.queries.ListResultCountsForRun(ctx, filter.RunID)
		if err != nil {
			return nil, fmt.Errorf("list run source result counts: %w", err)
		}
		items := make([]*ResultCountRow, 0, len(rows))
		for _, row := range rows {
			items = append(items, resultCountRow(row.ID, row.SourceName, row.SourceType, row.ExpectedFile, row.Query, row.ExpectedResultCount, row.ObservedResultCount, row.ResultCountComparison, row.ExportDate))
		}
		return items, nil
	case filter.IncludeExportDate:
		rows, err := s.queries.ListResultCountsForRunWithoutResultCounts(ctx, filter.RunID)
		if err != nil {
			return nil, fmt.Errorf("list run source result counts: %w", err)
		}
		items := make([]*ResultCountRow, 0, len(rows))
		for _, row := range rows {
			items = append(items, resultCountRow(row.ID, row.SourceName, row.SourceType, row.ExpectedFile, row.Query, row.ExpectedResultCount, row.ObservedResultCount, row.ResultCountComparison, row.ExportDate))
		}
		return items, nil
	case filter.IncludeResultCounts:
		rows, err := s.queries.ListResultCountsForRunWithoutExportDate(ctx, filter.RunID)
		if err != nil {
			return nil, fmt.Errorf("list run source result counts: %w", err)
		}
		items := make([]*ResultCountRow, 0, len(rows))
		for _, row := range rows {
			items = append(items, resultCountRow(row.ID, row.SourceName, row.SourceType, row.ExpectedFile, row.Query, row.ExpectedResultCount, row.ObservedResultCount, row.ResultCountComparison, row.ExportDate))
		}
		return items, nil
	default:
		rows, err := s.queries.ListResultCountsForRunWithoutResultCountsAndExportDate(ctx, filter.RunID)
		if err != nil {
			return nil, fmt.Errorf("list run source result counts: %w", err)
		}
		items := make([]*ResultCountRow, 0, len(rows))
		for _, row := range rows {
			items = append(items, resultCountRow(row.ID, row.SourceName, row.SourceType, row.ExpectedFile, row.Query, row.ExpectedResultCount, row.ObservedResultCount, row.ResultCountComparison, row.ExportDate))
		}
		return items, nil
	}
}

// resultCountRow maps one generated result-count row into an application row.
func resultCountRow(id int64, sourceName, sourceType, expectedFile string, query sql.NullString, expected, observed sql.NullInt64, comparison, exportDate sql.NullString) *ResultCountRow {
	row := &ResultCountRow{ID: id, SourceName: sourceName, SourceType: sourceType, ExpectedFile: expectedFile}
	if query.Valid {
		row.Query = &query.String
	}
	if expected.Valid {
		row.ExpectedResultCount = &expected.Int64
	}
	if observed.Valid {
		row.ObservedResultCount = &observed.Int64
	}
	if comparison.Valid {
		row.ResultCountComparison = &comparison.String
	}
	if exportDate.Valid {
		row.ExportDate = &exportDate.String
	}
	return row
}

// ListCorpusRecords returns one bounded page of run-scoped source records with
// their source identity.
func (s *Store) ListCorpusRecords(ctx context.Context, filter CorpusRecordFilter) (*CorpusRecordPage, error) {
	where, args, err := corpusRecordWhere(filter)
	if err != nil {
		return nil, err
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM source_records sr JOIN run_sources rs ON rs.id=sr.run_source_id WHERE "+where, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count corpus source records: %w", err)
	}
	query := "SELECT sr.id, sr.run_source_id, rs.source_name, rs.source_type, sr.record_index, sr.parse_status, sr.reject_reason, sr.content_hash, sr.created_at " +
		"FROM source_records sr JOIN run_sources rs ON rs.id=sr.run_source_id WHERE " + where +
		" ORDER BY " + corpusRecordSortExpression(filter.Sort) + " " + sqlDirection(filter.Order) + ", sr.id " + sqlDirection(filter.Order) + " LIMIT ? OFFSET ?"
	queryArgs := append(append([]any(nil), args...), filter.PerPage, (clampPage(filter.Page, filter.PerPage, total)-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("list corpus source records: %w", err)
	}
	defer rows.Close()
	items := make([]*CorpusRecord, 0)
	for rows.Next() {
		var item CorpusRecord
		var rejectReason *string
		if err := rows.Scan(&item.ID, &item.RunSourceID, &item.SourceName, &item.SourceType, &item.RecordIndex, &item.ParseStatus, &rejectReason, &item.ContentHash, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("list corpus source records: %w", err)
		}
		item.RejectReason = rejectReason
		items = append(items, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list corpus source records: %w", err)
	}
	return &CorpusRecordPage{Items: items, Total: total}, nil
}

// clampPage maps an offset request past the end to the final populated page.
func clampPage(page, perPage int, total int64) int {
	totalPages := (total + int64(perPage) - 1) / int64(perPage)
	if totalPages == 0 {
		return 1
	}
	if int64(page) > totalPages {
		return int(totalPages)
	}
	return page
}
