// errors_integration_test.go verifies the source family propagates connection
// failures as errors instead of panicking or returning partial results.
//go:build integration

package source_test

import (
	"context"
	"testing"

	"analysis/database/source"
)

// TestStoreOperationsPropagateConnectionErrors verifies every public store
// operation reports a closed connection as an error rather than a value.
func TestStoreOperationsPropagateConnectionErrors(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		call func() error
	}{
		{"CreateSource", func() error {
			_, err := store.CreateSource(ctx, source.CreateSourceInput{RunID: 1, Name: "closed", Type: "csv", ExpectedFile: "closed.csv"})
			return err
		}},
		{"ListSourcesByRun", func() error { _, err := store.ListSourcesByRun(ctx, 1); return err }},
		{"SetObservedResultCount", func() error {
			return store.SetObservedResultCount(ctx, source.ObservedResultCountInput{RunSourceID: 1, ObservedResultCount: 1, Comparison: "match"})
		}},
		{"CreateRecord", func() error {
			_, err := store.CreateRecord(ctx, source.CreateRecordInput{RunSourceID: 1, RecordIndex: 0, RawPayload: "{}", ContentHash: "hash"})
			return err
		}},
		{"UpdateRecordStatus", func() error {
			return store.UpdateRecordStatus(ctx, source.RecordStatusInput{RecordID: 1, Status: "parsed"})
		}},
		{"ListRecordsBySource", func() error { _, err := store.ListRecordsBySource(ctx, 1); return err }},
		{"CountRecordsBySource", func() error { _, err := store.CountRecordsBySource(ctx, 1); return err }},
		{"SetFilterData", func() error {
			return store.SetFilterData(ctx, source.FilterCountInput{RunID: 1, SourceName: "closed", FilterData: "[]"})
		}},
		{"ListFilterCountsByRun", func() error { _, err := store.ListFilterCountsByRun(ctx, 1); return err }},
		{"GetFilterCount", func() error { _, err := store.GetFilterCount(ctx, 1, "closed"); return err }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil {
				t.Fatal("closed connection did not produce an error")
			}
		})
	}
}
