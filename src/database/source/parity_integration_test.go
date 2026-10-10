// parity_integration_test.go verifies the legacy flat repositories and the
// source family store return and persist identical source, record, and filter
// count evidence on separate copies of one migrated fixture.
//go:build integration

package source_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"analysis/database"
	"analysis/database/source"
)

// TestSourceFamilyParityWithLegacyRepositories verifies the adapters and family store agree on reads, writes, and errors.
func TestSourceFamilyParityWithLegacyRepositories(t *testing.T) {
	ctx := context.Background()
	fixturePath := filepath.Join(t.TempDir(), "fixture.db")
	fixture, err := database.Open(fixturePath, familyConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	runID, err := fixture.Run.StartRun(ctx, "source-parity", "")
	if err != nil {
		t.Fatal(err)
	}
	sourceID, err := fixture.RunSources.Create(runID, "scopus", "csv", "scopus.csv", "TITLE-ABS-KEY(test)", "doi,title", 4, "2026-01-02")
	if err != nil {
		t.Fatal(err)
	}
	recordID, err := fixture.SourceRecords.Create(sourceID, 0, `{"title":"Test"}`, "hash-abc")
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.SourceRecords.UpdateParseStatus(recordID, "rejected", "missing fields"); err != nil {
		t.Fatal(err)
	}
	if err := fixture.SourceFilterCounts.SetFilterData(runID, "scopus", `[{"filters":["open access"],"count":2}]`); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.DB.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		t.Fatal(err)
	}
	if err := fixture.Close(); err != nil {
		t.Fatal(err)
	}

	legacyPath := filepath.Join(t.TempDir(), "legacy.db")
	familyPath := filepath.Join(t.TempDir(), "family.db")
	copyDatabaseFile(t, fixturePath, legacyPath)
	copyDatabaseFile(t, fixturePath, familyPath)

	legacy, err := database.OpenExisting(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	defer legacy.Close()
	family, err := database.OpenExisting(familyPath)
	if err != nil {
		t.Fatal(err)
	}
	defer family.Close()
	store := source.New(family.DB)

	// Read parity for sources, records, counts, and filter counts.
	legacySources, err := legacy.RunSources.ListByRun(runID)
	if err != nil {
		t.Fatal(err)
	}
	familySources, err := store.ListSourcesByRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	assertSourcesEqual(t, "ListByRun", legacySources, familySources)

	legacyRecords, err := legacy.SourceRecords.ListBySource(sourceID)
	if err != nil {
		t.Fatal(err)
	}
	familyRecords, err := store.ListRecordsBySource(ctx, sourceID)
	if err != nil {
		t.Fatal(err)
	}
	assertRecordsEqual(t, "ListBySource", legacyRecords, familyRecords)

	legacyCount, err := legacy.SourceRecords.CountBySource(sourceID)
	if err != nil {
		t.Fatal(err)
	}
	familyCount, err := store.CountRecordsBySource(ctx, sourceID)
	if err != nil {
		t.Fatal(err)
	}
	if legacyCount != familyCount {
		t.Fatalf("CountBySource parity: legacy=%d family=%d", legacyCount, familyCount)
	}

	legacyCounts, err := legacy.SourceFilterCounts.ListByRun(runID)
	if err != nil {
		t.Fatal(err)
	}
	familyCounts, err := store.ListFilterCountsByRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	assertFilterCountsEqual(t, "ListFilterCountsByRun", legacyCounts, familyCounts)

	legacyFilter, err := legacy.SourceFilterCounts.GetByRunAndSource(runID, "scopus")
	if err != nil {
		t.Fatal(err)
	}
	familyFilter, err := store.GetFilterCount(ctx, runID, "scopus")
	if err != nil {
		t.Fatal(err)
	}
	if legacyFilter == nil || familyFilter == nil || legacyFilter.FilterData != familyFilter.FilterData {
		t.Fatalf("GetByRunAndSource parity: legacy=%+v family=%+v", legacyFilter, familyFilter)
	}

	// Missing-read identity parity.
	legacyMissing, err := legacy.SourceFilterCounts.GetByRunAndSource(999, "absent")
	if err != nil || legacyMissing != nil {
		t.Fatalf("legacy missing filter count = %+v err=%v", legacyMissing, err)
	}
	familyMissing, err := store.GetFilterCount(ctx, 999, "absent")
	if err != nil || familyMissing != nil {
		t.Fatalf("family missing filter count = %+v err=%v", familyMissing, err)
	}

	// Source write parity.
	legacyNewSource, err := legacy.RunSources.Create(runID, "ieee", "csv", "ieee.csv", "", "", 2, "")
	if err != nil {
		t.Fatal(err)
	}
	familyNewSource, err := store.CreateSource(ctx, source.CreateSourceInput{
		RunID: runID, Name: "ieee", Type: "csv", ExpectedFile: "ieee.csv", ExpectedCount: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if legacyNewSource != familyNewSource {
		t.Fatalf("CreateSource ID parity: legacy=%d family=%d", legacyNewSource, familyNewSource)
	}
	assertQueryParity(t, legacy.DB, family.DB,
		"SELECT id, pipeline_run_id, source_name, source_type, expected_file, query, requested_fields, expected_result_count, observed_result_count, result_count_comparison, export_date FROM run_sources WHERE id=?",
		legacyNewSource)

	if err := legacy.RunSources.SetObservedResultCount(legacyNewSource, 3, "above"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetObservedResultCount(ctx, source.ObservedResultCountInput{RunSourceID: familyNewSource, ObservedResultCount: 3, Comparison: "above"}); err != nil {
		t.Fatal(err)
	}
	assertQueryParity(t, legacy.DB, family.DB,
		"SELECT observed_result_count, result_count_comparison FROM run_sources WHERE id=?", legacyNewSource)

	// Record write parity.
	legacyNewRecord, err := legacy.SourceRecords.Create(legacyNewSource, 0, `{"title":""}`, "hash-def")
	if err != nil {
		t.Fatal(err)
	}
	familyNewRecord, err := store.CreateRecord(ctx, source.CreateRecordInput{
		RunSourceID: familyNewSource, RecordIndex: 0, RawPayload: `{"title":""}`, ContentHash: "hash-def",
	})
	if err != nil {
		t.Fatal(err)
	}
	if legacyNewRecord != familyNewRecord {
		t.Fatalf("CreateRecord ID parity: legacy=%d family=%d", legacyNewRecord, familyNewRecord)
	}
	if err := legacy.SourceRecords.UpdateParseStatus(legacyNewRecord, "rejected", "empty title"); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateRecordStatus(ctx, source.RecordStatusInput{RecordID: familyNewRecord, Status: "rejected", RejectReason: "empty title"}); err != nil {
		t.Fatal(err)
	}
	assertQueryParity(t, legacy.DB, family.DB,
		"SELECT id, run_source_id, record_index, raw_payload, content_hash, parse_status, reject_reason FROM source_records WHERE id=?",
		legacyNewRecord)

	// Filter count write parity.
	if err := legacy.SourceFilterCounts.SetFilterData(runID, "ieee", `[{"filters":[],"count":2}]`); err != nil {
		t.Fatal(err)
	}
	if err := store.SetFilterData(ctx, source.FilterCountInput{RunID: runID, SourceName: "ieee", FilterData: `[{"filters":[],"count":2}]`}); err != nil {
		t.Fatal(err)
	}
	assertQueryParity(t, legacy.DB, family.DB,
		"SELECT pipeline_run_id, source_name, filter_data FROM source_filter_counts WHERE pipeline_run_id=? ORDER BY source_name", runID)

	// Error parity for uniqueness and invalid comparison values.
	assertErrorParity(t, "duplicate source",
		legacyDuplicateSourceError(legacy, runID),
		familyDuplicateSourceError(ctx, store, runID))
	assertErrorParity(t, "invalid comparison",
		legacy.RunSources.SetObservedResultCount(legacyNewSource, 1, "invalid"),
		store.SetObservedResultCount(ctx, source.ObservedResultCountInput{RunSourceID: familyNewSource, ObservedResultCount: 1, Comparison: "invalid"}))
}

// legacyDuplicateSourceError attempts one duplicate run source and returns only its error.
func legacyDuplicateSourceError(db *database.Database, runID int64) error {
	_, err := db.RunSources.Create(runID, "scopus", "csv", "scopus.csv", "", "", 0, "")
	return err
}

// familyDuplicateSourceError attempts one duplicate run source and returns only its error.
func familyDuplicateSourceError(ctx context.Context, store *source.Store, runID int64) error {
	_, err := store.CreateSource(ctx, source.CreateSourceInput{RunID: runID, Name: "scopus", Type: "csv", ExpectedFile: "scopus.csv"})
	return err
}

// assertSourcesEqual compares ordered legacy and family source projections.
func assertSourcesEqual(t *testing.T, operation string, legacy []*database.RunSource, family []*source.Source) {
	t.Helper()
	if len(legacy) != len(family) {
		t.Fatalf("%s length mismatch: legacy=%d family=%d", operation, len(legacy), len(family))
	}
	for index := range legacy {
		left, right := legacy[index], family[index]
		if left.ID != right.ID ||
			left.PipelineRunID != right.PipelineRunID ||
			left.SourceName != right.SourceName ||
			left.SourceType != right.SourceType ||
			left.ExpectedFile != right.ExpectedFile ||
			left.Query != right.Query ||
			left.RequestedFields != right.RequestedFields ||
			(left.ExpectedResultCount == nil) != (right.ExpectedResultCount == nil) ||
			(left.ObservedResultCount == nil) != (right.ObservedResultCount == nil) ||
			left.ResultCountComparison != right.ResultCountComparison ||
			left.ExportDate != right.ExportDate ||
			left.CreatedAt != right.CreatedAt {
			t.Fatalf("%s item %d mismatch: legacy=%+v family=%+v", operation, index, left, right)
		}
	}
}

// assertRecordsEqual compares ordered legacy and family record projections.
func assertRecordsEqual(t *testing.T, operation string, legacy []*database.SourceRecord, family []*source.Record) {
	t.Helper()
	if len(legacy) != len(family) {
		t.Fatalf("%s length mismatch: legacy=%d family=%d", operation, len(legacy), len(family))
	}
	for index := range legacy {
		left, right := legacy[index], family[index]
		if left.ID != right.ID ||
			left.RunSourceID != right.RunSourceID ||
			left.RecordIndex != right.RecordIndex ||
			left.RawPayload != right.RawPayload ||
			left.ContentHash != right.ContentHash ||
			left.ParseStatus != right.ParseStatus ||
			left.RejectReason != right.RejectReason ||
			left.CreatedAt != right.CreatedAt {
			t.Fatalf("%s item %d mismatch: legacy=%+v family=%+v", operation, index, left, right)
		}
	}
}

// assertFilterCountsEqual compares ordered legacy and family filter count projections.
func assertFilterCountsEqual(t *testing.T, operation string, legacy []*database.SourceFilterCount, family []*source.FilterCount) {
	t.Helper()
	if len(legacy) != len(family) {
		t.Fatalf("%s length mismatch: legacy=%d family=%d", operation, len(legacy), len(family))
	}
	for index := range legacy {
		left, right := legacy[index], family[index]
		if left.ID != right.ID || left.PipelineRunID != right.PipelineRunID ||
			left.SourceName != right.SourceName || left.FilterData != right.FilterData {
			t.Fatalf("%s item %d mismatch: legacy=%+v family=%+v", operation, index, left, right)
		}
	}
}

// assertQueryParity runs the same read on both databases and compares every returned row.
func assertQueryParity(t *testing.T, legacy, family *sql.DB, query string, args ...any) {
	t.Helper()
	legacyRows := queryRowValues(t, legacy, query, args...)
	familyRows := queryRowValues(t, family, query, args...)
	if !reflect.DeepEqual(legacyRows, familyRows) {
		t.Fatalf("query parity mismatch:\nlegacy=%v\nfamily=%v", legacyRows, familyRows)
	}
}

// queryRowValues renders every row of one query as ordered comparable text.
func queryRowValues(t *testing.T, db *sql.DB, query string, args ...any) [][]string {
	t.Helper()
	rows, err := db.Query(query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	names, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var result [][]string
	for rows.Next() {
		values := make([]any, len(names))
		pointers := make([]any, len(names))
		for index := range values {
			pointers[index] = &values[index]
		}
		if err := rows.Scan(pointers...); err != nil {
			t.Fatal(err)
		}
		rendered := make([]string, len(names))
		for index, value := range values {
			rendered[index] = formatDatabaseValue(value)
		}
		result = append(result, rendered)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

// copyDatabaseFile copies one checkpointed fixture into an independent working copy.
func copyDatabaseFile(t *testing.T, source, destination string) {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// formatDatabaseValue renders one scanned SQLite value as comparable text.
func formatDatabaseValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return "<null>"
	case []byte:
		return string(typed)
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

// assertErrorParity verifies legacy and family operations reject input with the same error text.
func assertErrorParity(t *testing.T, operation string, legacyErr, familyErr error) {
	t.Helper()
	if (legacyErr == nil) != (familyErr == nil) {
		t.Fatalf("%s error mismatch: legacy=%v family=%v", operation, legacyErr, familyErr)
	}
	if legacyErr != nil && legacyErr.Error() != familyErr.Error() {
		t.Fatalf("%s error text mismatch: legacy=%q family=%q", operation, legacyErr, familyErr)
	}
}
