// dynamic_integration_test.go verifies the retained source-record builder
// rejects injection attempts, follows its declared ordering, and caps its
// search and page inputs.
//go:build integration

package source_test

import (
	"context"
	"strings"
	"testing"

	"analysis/database/source"
)

// TestStoreListCorpusRecordsConstrainsDynamicInputs verifies the retained
// source-record builder binds search values, rejects unknown sort and order
// values, follows its declared ordering, and caps oversized inputs.
func TestStoreListCorpusRecordsConstrainsDynamicInputs(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "viewer-corpus-records-dynamic")

	alphaSource, err := store.CreateSource(ctx, source.CreateSourceInput{RunID: runID, Name: "alpha", Type: "csv", ExpectedFile: "alpha.csv"})
	if err != nil {
		t.Fatal(err)
	}
	betaSource, err := store.CreateSource(ctx, source.CreateSourceInput{RunID: runID, Name: "beta", Type: "json", ExpectedFile: "beta.json"})
	if err != nil {
		t.Fatal(err)
	}
	alphaRecord, err := store.CreateRecord(ctx, source.CreateRecordInput{RunSourceID: alphaSource, RecordIndex: 1, RawPayload: "{}", ContentHash: "alpha-hash"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateRecord(ctx, source.CreateRecordInput{RunSourceID: betaSource, RecordIndex: 1, RawPayload: "{}", ContentHash: "beta-hash"}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateRecordStatus(ctx, source.RecordStatusInput{RecordID: alphaRecord, Status: "rejected", RejectReason: `x' OR '1'='1`}); err != nil {
		t.Fatal(err)
	}

	ascending, err := store.ListCorpusRecords(ctx, source.CorpusRecordFilter{RunID: runID, Sort: "source_name", Order: "ASC", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(ascending.Items) != 2 || ascending.Items[0].SourceName != "alpha" || ascending.Items[1].SourceName != "beta" {
		t.Fatalf("ascending source order = %+v", ascending.Items)
	}
	descending, err := store.ListCorpusRecords(ctx, source.CorpusRecordFilter{RunID: runID, Sort: "source_name", Order: "DESC", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(descending.Items) != 2 || descending.Items[0].SourceName != "beta" || descending.Items[1].SourceName != "alpha" {
		t.Fatalf("descending source order = %+v", descending.Items)
	}

	// A malicious search value is bound as a literal LIKE pattern and matches
	// only the record that stores the same text.
	literal, err := store.ListCorpusRecords(ctx, source.CorpusRecordFilter{RunID: runID, Query: `' OR '1'='1`, Sort: "id", Order: "ASC", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatal(err)
	}
	if literal.Total != 1 || len(literal.Items) != 1 || literal.Items[0].ID != alphaRecord {
		t.Fatalf("literal search = %+v, want only the stored malicious text", literal.Items)
	}

	for _, test := range []struct {
		name   string
		filter source.CorpusRecordFilter
	}{
		{"injected sort", source.CorpusRecordFilter{RunID: runID, Sort: "id; DROP TABLE source_records; --", Order: "ASC", Page: 1, PerPage: 20}},
		{"injected order", source.CorpusRecordFilter{RunID: runID, Sort: "id", Order: "ASC; DROP TABLE source_records; --", Page: 1, PerPage: 20}},
		{"oversized query", source.CorpusRecordFilter{RunID: runID, Query: strings.Repeat("x", 201), Sort: "id", Order: "ASC", Page: 1, PerPage: 20}},
		{"oversized page size", source.CorpusRecordFilter{RunID: runID, Sort: "id", Order: "ASC", Page: 1, PerPage: 1001}},
		{"zero page", source.CorpusRecordFilter{RunID: runID, Sort: "id", Order: "ASC", Page: 0, PerPage: 20}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := store.ListCorpusRecords(ctx, test.filter); err == nil {
				t.Fatal("expected the retained builder to reject the input")
			}
		})
	}
}
