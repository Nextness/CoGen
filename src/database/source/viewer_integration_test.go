// viewer_integration_test.go exercises the source family's viewer reads
// against a database created by the production migration runner.
//go:build integration

package source_test

import (
	"context"
	"testing"

	"analysis/database/source"
)

// TestStoreListResultCountsForRunProjectsOptionalColumns verifies the result
// count projection returns stored values and projects absent columns as NULL.
func TestStoreListResultCountsForRunProjectsOptionalColumns(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "viewer-result-counts")

	sourceID, err := store.CreateSource(ctx, source.CreateSourceInput{
		RunID: runID, Name: "scopus", Type: "csv", ExpectedFile: "scopus.csv",
		Query: "query", ExpectedCount: 4, ExportDate: "2026-01-02",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetObservedResultCount(ctx, source.ObservedResultCountInput{RunSourceID: sourceID, ObservedResultCount: 3, Comparison: "below"}); err != nil {
		t.Fatal(err)
	}

	rows, err := store.ListResultCountsForRun(ctx, source.ResultCountFilter{RunID: runID, IncludeExportDate: true, IncludeResultCounts: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("result count rows = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.SourceName != "scopus" || row.ExpectedResultCount == nil || *row.ExpectedResultCount != 4 || row.ObservedResultCount == nil || *row.ObservedResultCount != 3 {
		t.Fatalf("result count row = %+v", row)
	}
	if row.ResultCountComparison == nil || *row.ResultCountComparison != "below" || row.ExportDate == nil || *row.ExportDate != "2026-01-02" {
		t.Fatalf("result count evidence = %+v", row)
	}

	legacy, err := store.ListResultCountsForRun(ctx, source.ResultCountFilter{RunID: runID})
	if err != nil {
		t.Fatal(err)
	}
	if len(legacy) != 1 || legacy[0].ExpectedResultCount != nil || legacy[0].ObservedResultCount != nil || legacy[0].ResultCountComparison != nil || legacy[0].ExportDate != nil {
		t.Fatalf("legacy result count row = %+v, want unavailable optional columns", legacy)
	}

	exportOnly, err := store.ListResultCountsForRun(ctx, source.ResultCountFilter{RunID: runID, IncludeExportDate: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(exportOnly) != 1 || exportOnly[0].ExportDate == nil || *exportOnly[0].ExportDate != "2026-01-02" || exportOnly[0].ExpectedResultCount != nil {
		t.Fatalf("export-only result count row = %+v", exportOnly)
	}

	countsOnly, err := store.ListResultCountsForRun(ctx, source.ResultCountFilter{RunID: runID, IncludeResultCounts: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(countsOnly) != 1 || countsOnly[0].ExportDate != nil || countsOnly[0].ExpectedResultCount == nil || *countsOnly[0].ExpectedResultCount != 4 {
		t.Fatalf("counts-only result count row = %+v", countsOnly)
	}
}

// TestStoreListCorpusRecordsOrdersSearchesAndClamps verifies the run-scoped
// source record page follows its declared ordering, binds the searchable
// predicate, preserves nullable reject reasons, and clamps a page past the end.
func TestStoreListCorpusRecordsOrdersSearchesAndClamps(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "viewer-corpus-records")

	firstSource, err := store.CreateSource(ctx, source.CreateSourceInput{RunID: runID, Name: "alpha", Type: "csv", ExpectedFile: "alpha.csv"})
	if err != nil {
		t.Fatal(err)
	}
	secondSource, err := store.CreateSource(ctx, source.CreateSourceInput{RunID: runID, Name: "beta", Type: "json", ExpectedFile: "beta.json"})
	if err != nil {
		t.Fatal(err)
	}
	var recordIDs []int64
	for index, payload := range []string{`{"title":"one"}`, `{"title":"two"}`, `{"title":"three"}`} {
		recordID, err := store.CreateRecord(ctx, source.CreateRecordInput{
			RunSourceID: firstSource, RecordIndex: index + 1, RawPayload: payload, ContentHash: payload,
		})
		if err != nil {
			t.Fatal(err)
		}
		recordIDs = append(recordIDs, recordID)
	}
	if _, err := store.CreateRecord(ctx, source.CreateRecordInput{RunSourceID: secondSource, RecordIndex: 1, RawPayload: "{}", ContentHash: "other"}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateRecordStatus(ctx, source.RecordStatusInput{RecordID: recordIDs[0], Status: "rejected", RejectReason: "missing title"}); err != nil {
		t.Fatal(err)
	}

	page, err := store.ListCorpusRecords(ctx, source.CorpusRecordFilter{RunID: runID, Sort: "id", Order: "DESC", Page: 1, PerPage: 2})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 4 || len(page.Items) != 2 {
		t.Fatalf("corpus page = %d items total=%d, want 2 items of 4", len(page.Items), page.Total)
	}
	if page.Items[0].SourceName != "beta" || page.Items[1].RunSourceID != firstSource {
		t.Fatalf("corpus order = %+v", page.Items)
	}
	if page.Items[0].RejectReason != nil {
		t.Fatalf("beta reject reason = %v, want nil", page.Items[0].RejectReason)
	}

	rejected, err := store.ListCorpusRecords(ctx, source.CorpusRecordFilter{RunID: runID, Query: "missing title", Sort: "id", Order: "ASC", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatal(err)
	}
	if rejected.Total != 1 || len(rejected.Items) != 1 || rejected.Items[0].RejectReason == nil || *rejected.Items[0].RejectReason != "missing title" {
		t.Fatalf("rejected page = %+v", rejected.Items)
	}

	clamped, err := store.ListCorpusRecords(ctx, source.CorpusRecordFilter{RunID: runID, Sort: "source_name", Order: "ASC", Page: 9, PerPage: 2})
	if err != nil {
		t.Fatal(err)
	}
	if clamped.Total != 4 || len(clamped.Items) != 2 {
		t.Fatalf("clamped page = %d items total=%d, want the final populated page", len(clamped.Items), clamped.Total)
	}
	if _, err := store.ListCorpusRecords(ctx, source.CorpusRecordFilter{RunID: runID, Sort: "unknown", Order: "ASC", Page: 1, PerPage: 2}); err == nil {
		t.Fatal("expected an unsupported sort error")
	}
}
