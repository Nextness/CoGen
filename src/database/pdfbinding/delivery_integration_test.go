// delivery_integration_test.go tests delivered-event audit recording against
// real temporary SQLite databases created by the production migration runner.
//go:build integration

package pdfbinding

import (
	"context"
	"testing"
)

// TestRecordDeliveredAuditIsIdempotent verifies a repeated event key reuses the existing audit link.
func TestRecordDeliveredAuditIsIdempotent(t *testing.T) {
	ctx := context.Background()
	store, metadata, _ := openTestStore(t)
	event := DeliveredEvent{
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
	var events, links int
	if err := metadata.DB.QueryRow("SELECT COUNT(*) FROM audit_events").Scan(&events); err != nil {
		t.Fatal(err)
	}
	if err := metadata.DB.QueryRow("SELECT COUNT(*) FROM pdf_audit_links").Scan(&links); err != nil {
		t.Fatal(err)
	}
	if events != 1 || links != 1 {
		t.Fatalf("idempotent delivery events=%d links=%d, want one each", events, links)
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
	if err := store.RecordDeliveredAudit(ctx, tx, DeliveredEvent{
		EventKey: "stale-run", OccurredAt: "2026-01-01T00:00:00Z", Actor: "pipeline",
		PipelineRunID: 999, EntityType: "work", EntityID: "1",
		Action: "pdf_inventory_registered", MetadataJSON: `{}`, CorrelationID: "stale-run",
	}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var runID any
	if err := metadata.DB.QueryRow("SELECT pipeline_run_id FROM audit_events WHERE action='pdf_inventory_registered'").Scan(&runID); err != nil {
		t.Fatal(err)
	}
	if runID != nil {
		t.Fatalf("stale run ID = %v, want NULL", runID)
	}
}

// TestRecordDeliveredAuditBindsExistingRun verifies an existing pipeline run is linked.
func TestRecordDeliveredAuditBindsExistingRun(t *testing.T) {
	ctx := context.Background()
	store, metadata, _ := openTestStore(t)
	runID, err := metadata.PipelineRuns.StartRun("pdfbinding-test", "")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := store.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := store.RecordDeliveredAudit(ctx, tx, DeliveredEvent{
		EventKey: "existing-run", OccurredAt: "2026-01-01T00:00:00Z", Actor: "pipeline",
		PipelineRunID: runID, EntityType: "work", EntityID: "1",
		Action: "pdf_inventory_registered", MetadataJSON: `{}`, CorrelationID: "existing-run",
	}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var linkedRunID int64
	if err := metadata.DB.QueryRow("SELECT pipeline_run_id FROM audit_events WHERE action='pdf_inventory_registered'").Scan(&linkedRunID); err != nil {
		t.Fatal(err)
	}
	if linkedRunID != runID {
		t.Fatalf("linked run ID = %d, want %d", linkedRunID, runID)
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
	if err := store.RecordDeliveredAudit(ctx, tx, DeliveredEvent{
		EventKey: "rolled-back", OccurredAt: "2026-01-01T00:00:00Z", Actor: "pipeline",
		EntityType: "work", EntityID: "1", Action: "pdf_inventory_registered",
		MetadataJSON: `{}`, CorrelationID: "rolled-back",
	}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	var events, links int
	if err := metadata.DB.QueryRow("SELECT COUNT(*) FROM audit_events").Scan(&events); err != nil {
		t.Fatal(err)
	}
	if err := metadata.DB.QueryRow("SELECT COUNT(*) FROM pdf_audit_links").Scan(&links); err != nil {
		t.Fatal(err)
	}
	if events != 0 || links != 0 {
		t.Fatalf("rolled-back delivery events=%d links=%d, want zero", events, links)
	}
}
