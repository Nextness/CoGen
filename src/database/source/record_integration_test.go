// record_integration_test.go exercises the source-record store against a
// database created by the production migration runner.
//go:build integration

package source_test

import (
	"context"
	"testing"

	"analysis/database/source"
)

// TestStoreRecordLifecyclePreservesPayloadsAndOrdering verifies raw payload identity, rejection reasons, and record-index ordering.
func TestStoreRecordLifecyclePreservesPayloadsAndOrdering(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "record-lifecycle")
	sourceID, err := store.CreateSource(ctx, source.CreateSourceInput{
		RunID: runID, Name: "scopus", Type: "csv", ExpectedFile: "scopus.csv",
	})
	if err != nil {
		t.Fatal(err)
	}

	firstID, err := store.CreateRecord(ctx, source.CreateRecordInput{
		RunSourceID: sourceID, RecordIndex: 0, RawPayload: `{"title":"Good Article"}`, ContentHash: "hash-abc",
	})
	if err != nil {
		t.Fatalf("create first record: %v", err)
	}
	secondID, err := store.CreateRecord(ctx, source.CreateRecordInput{
		RunSourceID: sourceID, RecordIndex: 1, RawPayload: `{"title":""}`, ContentHash: "hash-def",
	})
	if err != nil {
		t.Fatalf("create second record: %v", err)
	}
	if firstID == 0 || secondID == firstID {
		t.Fatalf("record ids = %d %d", firstID, secondID)
	}
	if err := store.UpdateRecordStatus(ctx, source.RecordStatusInput{RecordID: firstID, Status: "parsed"}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateRecordStatus(ctx, source.RecordStatusInput{
		RecordID: secondID, Status: "rejected", RejectReason: "missing required fields",
	}); err != nil {
		t.Fatal(err)
	}

	records, err := store.ListRecordsBySource(ctx, sourceID)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0].ID != firstID || records[1].ID != secondID {
		t.Fatalf("record list = %+v, want ids [%d %d]", records, firstID, secondID)
	}
	if records[0].RecordIndex != 0 || records[1].RecordIndex != 1 {
		t.Fatalf("record indexes = %d %d, want 0 1", records[0].RecordIndex, records[1].RecordIndex)
	}
	if records[0].RawPayload != `{"title":"Good Article"}` || records[0].ContentHash != "hash-abc" ||
		records[0].ParseStatus != "parsed" || records[0].RejectReason != "" {
		t.Fatalf("parsed record = %+v", records[0])
	}
	if records[1].RawPayload != `{"title":""}` || records[1].ContentHash != "hash-def" ||
		records[1].ParseStatus != "rejected" || records[1].RejectReason != "missing required fields" {
		t.Fatalf("rejected record = %+v", records[1])
	}

	count, err := store.CountRecordsBySource(ctx, sourceID)
	if err != nil || count != 2 {
		t.Fatalf("record count = %d err=%v, want 2", count, err)
	}
	empty, err := store.ListRecordsBySource(ctx, 999)
	if err != nil || empty != nil {
		t.Fatalf("empty record list = %+v err=%v, want nil without error", empty, err)
	}
	emptyCount, err := store.CountRecordsBySource(ctx, 999)
	if err != nil || emptyCount != 0 {
		t.Fatalf("empty record count = %d err=%v, want 0", emptyCount, err)
	}
}

// TestStoreRecordListOrdersByDeclaredIndex verifies record_index ordering rather than insertion order.
func TestStoreRecordListOrdersByDeclaredIndex(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "record-order")
	sourceID, err := store.CreateSource(ctx, source.CreateSourceInput{
		RunID: runID, Name: "ieee", Type: "csv", ExpectedFile: "ieee.csv",
	})
	if err != nil {
		t.Fatal(err)
	}
	lastID, err := store.CreateRecord(ctx, source.CreateRecordInput{
		RunSourceID: sourceID, RecordIndex: 2, RawPayload: "third", ContentHash: "hash-2",
	})
	if err != nil {
		t.Fatal(err)
	}
	firstID, err := store.CreateRecord(ctx, source.CreateRecordInput{
		RunSourceID: sourceID, RecordIndex: 0, RawPayload: "first", ContentHash: "hash-0",
	})
	if err != nil {
		t.Fatal(err)
	}

	records, err := store.ListRecordsBySource(ctx, sourceID)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0].ID != firstID || records[1].ID != lastID {
		t.Fatalf("record order = %+v, want declared indexes 0 then 2", records)
	}
}
