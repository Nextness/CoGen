// source_adapter.go provides temporary forwarding adapters that keep the legacy
// flat run-source, source-record, and filter-count repository API working over
// the source family store. The adapters contain no SQL or second
// implementation; remove them after every caller migrates to Database.Source.
package database

import (
	"context"

	"analysis/database/source"
)

// RunSource links a pipeline run to one of its declared sources.
type RunSource struct {
	ID                    int64  `json:"id"`
	PipelineRunID         int64  `json:"pipeline_run_id"`
	SourceName            string `json:"source_name"`
	SourceType            string `json:"source_type"`
	ExpectedFile          string `json:"expected_file"`
	Query                 string `json:"query,omitempty"`
	RequestedFields       string `json:"requested_fields,omitempty"`
	ExpectedResultCount   *int   `json:"expected_result_count,omitempty"`
	ObservedResultCount   *int   `json:"observed_result_count,omitempty"`
	ResultCountComparison string `json:"result_count_comparison,omitempty"`
	ExportDate            string `json:"export_date,omitempty"`
	CreatedAt             string `json:"created_at"`
}

// SourceRecord represents a single raw record loaded from a source.
type SourceRecord struct {
	ID           int64  `json:"id"`
	RunSourceID  int64  `json:"run_source_id"`
	RecordIndex  int    `json:"record_index"`
	RawPayload   string `json:"raw_payload"`
	ContentHash  string `json:"content_hash"`
	ParseStatus  string `json:"parse_status"`
	RejectReason string `json:"reject_reason,omitempty"`
	CreatedAt    string `json:"created_at"`
}

// SourceFilterCount holds per-source filter stage article counts for a run.
type SourceFilterCount struct {
	ID            int64  `json:"id"`
	PipelineRunID int64  `json:"pipeline_run_id"`
	SourceName    string `json:"source_name"`
	FilterData    string `json:"filter_data"` // JSON array of {filters: string[], count: int}
}

// RunSourceRepository forwards the legacy run-source API to the source family store.
type RunSourceRepository struct{ db *Database }

// SourceRecordRepository forwards the legacy source-record API to the source family store.
type SourceRecordRepository struct{ db *Database }

// SourceFilterCountRepository forwards the legacy filter-count API to the source family store.
type SourceFilterCountRepository struct{ db *Database }

// Create inserts a new run source link. Returns the source ID.
func (r *RunSourceRepository) Create(pipelineRunID int64, sourceName, sourceType, expectedFile, query, requestedFields string, expectedResultCount int, exportDate string) (int64, error) {
	return r.db.Source.CreateSource(context.Background(), source.CreateSourceInput{
		RunID:           pipelineRunID,
		Name:            sourceName,
		Type:            sourceType,
		ExpectedFile:    expectedFile,
		Query:           query,
		RequestedFields: requestedFields,
		ExpectedCount:   expectedResultCount,
		ExportDate:      exportDate,
	})
}

// ListByRun returns all sources for a given pipeline run, ordered by ID.
func (r *RunSourceRepository) ListByRun(pipelineRunID int64) ([]*RunSource, error) {
	found, err := r.db.Source.ListSourcesByRun(context.Background(), pipelineRunID)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, nil
	}
	legacy := make([]*RunSource, 0, len(found))
	for _, item := range found {
		legacy = append(legacy, runSourceFromFamily(item))
	}
	return legacy, nil
}

// SetObservedResultCount records the raw export count observed for a source
// and its informational comparison with the count declared in the config.
func (r *RunSourceRepository) SetObservedResultCount(runSourceID int64, observedResultCount int, comparison string) error {
	return r.db.Source.SetObservedResultCount(context.Background(), source.ObservedResultCountInput{
		RunSourceID:         runSourceID,
		ObservedResultCount: observedResultCount,
		Comparison:          comparison,
	})
}

// Create inserts a new source record. Returns the record ID.
func (r *SourceRecordRepository) Create(runSourceID int64, recordIndex int, rawPayload, contentHash string) (int64, error) {
	return r.db.Source.CreateRecord(context.Background(), source.CreateRecordInput{
		RunSourceID: runSourceID,
		RecordIndex: recordIndex,
		RawPayload:  rawPayload,
		ContentHash: contentHash,
	})
}

// UpdateParseStatus updates the parse status and optional reject reason for a source record.
func (r *SourceRecordRepository) UpdateParseStatus(recordID int64, status, rejectReason string) error {
	return r.db.Source.UpdateRecordStatus(context.Background(), source.RecordStatusInput{
		RecordID:     recordID,
		Status:       status,
		RejectReason: rejectReason,
	})
}

// ListBySource returns all records for a given run source, ordered by record index.
func (r *SourceRecordRepository) ListBySource(runSourceID int64) ([]*SourceRecord, error) {
	found, err := r.db.Source.ListRecordsBySource(context.Background(), runSourceID)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, nil
	}
	legacy := make([]*SourceRecord, 0, len(found))
	for _, item := range found {
		legacy = append(legacy, sourceRecordFromFamily(item))
	}
	return legacy, nil
}

// CountBySource returns the number of records for a given run source.
func (r *SourceRecordRepository) CountBySource(runSourceID int64) (int, error) {
	return r.db.Source.CountRecordsBySource(context.Background(), runSourceID)
}

// SetFilterData upserts the filter data for a source in a run.
func (r *SourceFilterCountRepository) SetFilterData(pipelineRunID int64, sourceName, filterData string) error {
	return r.db.Source.SetFilterData(context.Background(), source.FilterCountInput{
		RunID:      pipelineRunID,
		SourceName: sourceName,
		FilterData: filterData,
	})
}

// ListByRun returns filter data for all sources in a run, ordered by source name.
func (r *SourceFilterCountRepository) ListByRun(pipelineRunID int64) ([]*SourceFilterCount, error) {
	found, err := r.db.Source.ListFilterCountsByRun(context.Background(), pipelineRunID)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, nil
	}
	legacy := make([]*SourceFilterCount, 0, len(found))
	for _, item := range found {
		legacy = append(legacy, sourceFilterCountFromFamily(item))
	}
	return legacy, nil
}

// GetByRunAndSource returns filter data for a specific source in a run.
func (r *SourceFilterCountRepository) GetByRunAndSource(pipelineRunID int64, sourceName string) (*SourceFilterCount, error) {
	found, err := r.db.Source.GetFilterCount(context.Background(), pipelineRunID, sourceName)
	if err != nil {
		return nil, err
	}
	return sourceFilterCountFromFamily(found), nil
}

// runSourceFromFamily maps a source family source into the legacy application type.
func runSourceFromFamily(found *source.Source) *RunSource {
	if found == nil {
		return nil
	}
	return &RunSource{
		ID:                    found.ID,
		PipelineRunID:         found.PipelineRunID,
		SourceName:            found.SourceName,
		SourceType:            found.SourceType,
		ExpectedFile:          found.ExpectedFile,
		Query:                 found.Query,
		RequestedFields:       found.RequestedFields,
		ExpectedResultCount:   found.ExpectedResultCount,
		ObservedResultCount:   found.ObservedResultCount,
		ResultCountComparison: found.ResultCountComparison,
		ExportDate:            found.ExportDate,
		CreatedAt:             found.CreatedAt,
	}
}

// sourceRecordFromFamily maps a source family record into the legacy application type.
func sourceRecordFromFamily(found *source.Record) *SourceRecord {
	if found == nil {
		return nil
	}
	return &SourceRecord{
		ID:           found.ID,
		RunSourceID:  found.RunSourceID,
		RecordIndex:  found.RecordIndex,
		RawPayload:   found.RawPayload,
		ContentHash:  found.ContentHash,
		ParseStatus:  found.ParseStatus,
		RejectReason: found.RejectReason,
		CreatedAt:    found.CreatedAt,
	}
}

// sourceFilterCountFromFamily maps a source family filter count into the legacy application type.
func sourceFilterCountFromFamily(found *source.FilterCount) *SourceFilterCount {
	if found == nil {
		return nil
	}
	return &SourceFilterCount{
		ID:            found.ID,
		PipelineRunID: found.PipelineRunID,
		SourceName:    found.SourceName,
		FilterData:    found.FilterData,
	}
}
