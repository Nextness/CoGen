// delivery_integration_test.go tests delivered-event audit recording against
// real temporary SQLite databases created by the production migration runner.
//go:build integration

package pdfbinding_test

import (
	"context"
	"testing"

	"analysis/database/pdfbinding"
)

// TestRecordDeliveredAuditIsIdempotent verifies a repeated event key reuses the existing audit link.
func TestRecordDeliveredAuditIsIdempotent(t *testing.T) {
	ctx := context.Background()
	store, metadata, _ := openTestStore(t)
	event := pdfbinding.DeliveredEvent{
		EventKey: "event-1", OccurredAt: "2026-01-01T00:00:00Z", Actor: "pipeline",
		EntityType: "work", EntityID: "1", Action: "pdf_inventory_registered",
		MetadataJSON: `{"doi":"10.1000/one"}`, CorrelationID: "correlation-1",
	}
	for attempt := 0; attempt < 2; attempt++ {
		tx, err := store.BeginTx(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.RecordDeliveredAudit(ctx, tx, event); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	events, err := metadata.Audit.ListByEntity(ctx, "work", "1")
	if err != nil {
		t.Fatal(err)
	}
	// The delivered-event link table is internal to the cross-database outbox
	// protocol and has no family read API, so this direct integrity check
	// counts its rows to prove the retry guard reused the existing link.
	var links int
	if err := metadata.RawDBForTest().QueryRow("SELECT COUNT(*) FROM pdf_audit_links").Scan(&links); err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || links != 1 {
		t.Fatalf("idempotent delivery events=%d links=%d, want one each", len(events), links)
	}
}

// TestRecordDeliveredAuditToleratesStaleRun verifies a missing pipeline run binds a NULL run link.
func TestRecordDeliveredAuditToleratesStaleRun(t *testing.T) {
	ctx := context.Background()
	store, metadata, _ := openTestStore(t)
	tx, err := store.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := store.RecordDeliveredAudit(ctx, tx, pdfbinding.DeliveredEvent{
		EventKey: "stale-run", OccurredAt: "2026-01-01T00:00:00Z", Actor: "pipeline",
		PipelineRunID: 999, EntityType: "work", EntityID: "1",
		Action: "pdf_inventory_registered", MetadataJSON: `{}`, CorrelationID: "stale-run",
	}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	events, err := metadata.Audit.ListByEntity(ctx, "work", "1")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].PipelineRunID != nil {
		t.Fatalf("stale run events = %+v, want one event with a NULL run link", events)
	}
}

// TestRecordDeliveredAuditBindsExistingRun verifies an existing pipeline run is linked.
func TestRecordDeliveredAuditBindsExistingRun(t *testing.T) {
	ctx := context.Background()
	store, metadata, _ := openTestStore(t)
	runID, err := metadata.Run.StartRun(ctx, "pdfbinding-test", "")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := store.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := store.RecordDeliveredAudit(ctx, tx, pdfbinding.DeliveredEvent{
		EventKey: "existing-run", OccurredAt: "2026-01-01T00:00:00Z", Actor: "pipeline",
		PipelineRunID: runID, EntityType: "work", EntityID: "1",
		Action: "pdf_inventory_registered", MetadataJSON: `{}`, CorrelationID: "existing-run",
	}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	events, err := metadata.Audit.ListByEntity(ctx, "work", "1")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].PipelineRunID == nil || *events[0].PipelineRunID != runID {
		t.Fatalf("linked run events = %+v, want one event linked to run %d", events, runID)
	}
}

// TestRecordDeliveredAuditRollsBackWithTransaction verifies an uncommitted delivery leaves no audit evidence.
func TestRecordDeliveredAuditRollsBackWithTransaction(t *testing.T) {
	ctx := context.Background()
	store, metadata, _ := openTestStore(t)
	tx, err := store.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordDeliveredAudit(ctx, tx, pdfbinding.DeliveredEvent{
		EventKey: "rolled-back", OccurredAt: "2026-01-01T00:00:00Z", Actor: "pipeline",
		EntityType: "work", EntityID: "1", Action: "pdf_inventory_registered",
		MetadataJSON: `{}`, CorrelationID: "rolled-back",
	}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	events, err := metadata.Audit.ListByEntity(ctx, "work", "1")
	if err != nil {
		t.Fatal(err)
	}
	// The delivered-event link table is internal to the cross-database outbox
	// protocol and has no family read API, so this direct integrity check
	// counts its rows to prove the rollback left no link behind.
	var links int
	if err := metadata.RawDBForTest().QueryRow("SELECT COUNT(*) FROM pdf_audit_links").Scan(&links); err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 || links != 0 {
		t.Fatalf("rolled-back delivery events=%d links=%d, want zero", len(events), links)
	}
}
