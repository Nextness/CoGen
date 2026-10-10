// filter_count_integration_test.go exercises the source filter-count store
// against a database created by the production migration runner.
//go:build integration

package source_test

import (
	"context"
	"testing"

	"analysis/database/source"
)

// TestStoreFilterCountUpsertPreservesLatestJSON verifies replace-by-key semantics and exact JSON preservation.
func TestStoreFilterCountUpsertPreservesLatestJSON(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "filter-count")

	first := `[{"filters":["original"],"count":1}]`
	if err := store.SetFilterData(ctx, source.FilterCountInput{RunID: runID, SourceName: "scopus", FilterData: first}); err != nil {
		t.Fatal(err)
	}
	found, err := store.GetFilterCount(ctx, runID, "scopus")
	if err != nil || found == nil {
		t.Fatalf("get filter count = %+v err=%v", found, err)
	}
	if found.PipelineRunID != runID || found.SourceName != "scopus" || found.FilterData != first {
		t.Fatalf("filter count = %+v", found)
	}

	replaced := `[{"filters":["replaced"],"count":99}]`
	if err := store.SetFilterData(ctx, source.FilterCountInput{RunID: runID, SourceName: "scopus", FilterData: replaced}); err != nil {
		t.Fatal(err)
	}
	found, err = store.GetFilterCount(ctx, runID, "scopus")
	if err != nil || found == nil || found.FilterData != replaced {
		t.Fatalf("replaced filter count = %+v err=%v", found, err)
	}

	missing, err := store.GetFilterCount(ctx, 999, "absent")
	if err != nil || missing != nil {
		t.Fatalf("missing filter count = %+v err=%v, want nil without error", missing, err)
	}
	empty, err := store.ListFilterCountsByRun(ctx, 999)
	if err != nil || empty != nil {
		t.Fatalf("empty filter count list = %+v err=%v, want nil without error", empty, err)
	}
}

// TestStoreFilterCountListOrdersBySourceName verifies the declared source-name ordering.
func TestStoreFilterCountListOrdersBySourceName(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "filter-count-order")

	if err := store.SetFilterData(ctx, source.FilterCountInput{RunID: runID, SourceName: "openalex", FilterData: `[{"filters":[],"count":10}]`}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetFilterData(ctx, source.FilterCountInput{RunID: runID, SourceName: "crossref", FilterData: `[{"filters":[],"count":5}]`}); err != nil {
		t.Fatal(err)
	}

	counts, err := store.ListFilterCountsByRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(counts) != 2 || counts[0].SourceName != "crossref" || counts[1].SourceName != "openalex" {
		t.Fatalf("filter count order = %+v, want crossref then openalex", counts)
	}
}
