// store_integration_test.go tests PDF store operations against real
// temporary SQLite databases.
//go:build integration

package pdfstore

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"analysis/database"
)

// TestAddNormalizesDOIDeduplicatesBlobsAndPreservesExistingDocument verifies add normalizes doi deduplicates blobs and preserves existing document.
func TestAddNormalizesDOIDeduplicatesBlobsAndPreservesExistingDocument(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	data := []byte("%PDF-1.7\nsame content")
	for index, doi := range []string{"https://doi.org/10.1000/ONE", "10.1000/two"} {
		registered, err := store.Register(ctx, doi, int64(index+1), 10)
		if err != nil || !registered {
			t.Fatalf("register %d: registered=%v err=%v", index, registered, err)
		}
		result, err := store.Add(ctx, doi, int64(index+1), data)
		if err != nil || !result.Added {
			t.Fatalf("add %d: %+v err=%v", index, result, err)
		}
	}
	document, err := store.Document(ctx, "10.1000/one")
	if err != nil {
		t.Fatal(err)
	}
	if document == nil || document.DOI != "10.1000/one" || document.Status != StatusAvailable || document.InventoriedAt == "" {
		t.Fatalf("unexpected document: %+v", document)
	}
	firstHash := document.ContentHash
	unchanged, err := store.Add(ctx, "10.1000/one", 1, []byte("%PDF-1.7\nreplacement"))
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Added || unchanged.ContentHash != firstHash || unchanged.ByteSize != len(data) {
		t.Fatalf("existing document was not preserved: %+v", unchanged)
	}
	var blobs, documents, events int
	if err := store.DB.QueryRow("SELECT COUNT(*) FROM pdf_blobs").Scan(&blobs); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.QueryRow("SELECT COUNT(*) FROM pdf_documents WHERE status='available'").Scan(&documents); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.QueryRow("SELECT COUNT(*) FROM pdf_audit_outbox WHERE action IN ('pdf_inventory_registered', 'pdf_document_inventoried')").Scan(&events); err != nil {
		t.Fatal(err)
	}
	if blobs != 1 || documents != 2 || events != 4 {
		t.Fatalf("manual store state: blobs=%d documents=%d events=%d", blobs, documents, events)
	}
}

// TestAddAndAuditOutboxAreTransactionalAndIdempotent verifies add and audit outbox are transactional and idempotent.
func TestAddAndAuditOutboxAreTransactionalAndIdempotent(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	metadata, err := database.Open(filepath.Join(t.TempDir(), "corpus.metadata.db"), filepath.Join("..", "..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer metadata.Close()
	workID, err := metadata.Works.CreateByDOI("10.1000/audit")
	if err != nil {
		t.Fatal(err)
	}
	runID, err := metadata.PipelineRuns.StartRun("pdf-inventory-test", "")
	if err != nil {
		t.Fatal(err)
	}
	registered, err := store.Register(ctx, "10.1000/audit", workID, runID)
	if err != nil || !registered {
		t.Fatalf("register audit inventory: registered=%v err=%v", registered, err)
	}
	registered, err = store.Register(ctx, "10.1000/audit", workID, runID)
	if err != nil || registered {
		t.Fatalf("duplicate registration: registered=%v err=%v", registered, err)
	}
	if _, err := store.Add(ctx, "10.1000/audit", workID, []byte("%PDF-1.7\naudit")); err != nil {
		t.Fatal(err)
	}
	first, err := store.FlushAuditOutbox(ctx, metadata.DB)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.FlushAuditOutbox(ctx, metadata.DB)
	if err != nil {
		t.Fatal(err)
	}
	if first != 2 || second != 0 {
		t.Fatalf("flush counts = %d then %d, want 2 then 0", first, second)
	}
	var events, links int
	if err := metadata.DB.QueryRow("SELECT COUNT(*) FROM audit_events WHERE action IN ('pdf_inventory_registered', 'pdf_document_inventoried')").Scan(&events); err != nil {
		t.Fatal(err)
	}
	if err := metadata.DB.QueryRow("SELECT COUNT(*) FROM pdf_audit_links").Scan(&links); err != nil {
		t.Fatal(err)
	}
	if events != 2 || links != 2 {
		t.Fatalf("metadata PDF audit events=%d links=%d, want two each", events, links)
	}
	var registeredRunID int64
	if err := metadata.DB.QueryRow("SELECT pipeline_run_id FROM audit_events WHERE action='pdf_inventory_registered'").Scan(&registeredRunID); err != nil {
		t.Fatal(err)
	}
	if registeredRunID != runID {
		t.Fatalf("registration audit run ID = %d, want %d", registeredRunID, runID)
	}
}

// TestAddRejectsCorruptExistingDocument verifies unchanged adds do not bless damaged PDF blobs.
func TestAddRejectsCorruptExistingDocument(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	if _, err := store.Register(ctx, "10.1000/corrupt", 1, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Add(ctx, "10.1000/corrupt", 1, []byte("%PDF-1.7\noriginal")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`DROP TRIGGER pdf_blobs_abort_update`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`UPDATE pdf_blobs SET data=? WHERE content_hash=(SELECT content_hash FROM pdf_documents WHERE doi=?)`, []byte("%PDF-1.7\nchanged!"), "10.1000/corrupt"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Add(ctx, "10.1000/corrupt", 1, []byte("%PDF-1.7\nretry")); err == nil || !strings.Contains(err.Error(), "corrupt") {
		t.Fatalf("corrupt unchanged add error = %v", err)
	}
}

// TestFlushAuditOutboxDropsStalePipelineRun verifies a flush preserves an
// outbox event whose pipeline run no longer exists in the bound metadata
// database, matching the durable PDF store across metadata iterations.
func TestFlushAuditOutboxDropsStalePipelineRun(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	metadata, err := database.Open(filepath.Join(t.TempDir(), "corpus.metadata.db"), filepath.Join("..", "..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer metadata.Close()
	workID, err := metadata.Works.CreateByDOI("10.1000/stale")
	if err != nil {
		t.Fatal(err)
	}
	// Register with a run ID that does not exist in this metadata database,
	// simulating an outbox event carried over from an older metadata iteration.
	registered, err := store.Register(ctx, "10.1000/stale", workID, 999)
	if err != nil || !registered {
		t.Fatalf("register stale inventory: registered=%v err=%v", registered, err)
	}
	flushed, err := store.FlushAuditOutbox(ctx, metadata.DB)
	if err != nil {
		t.Fatal(err)
	}
	if flushed != 1 {
		t.Fatalf("flush count = %d, want 1", flushed)
	}
	var events, links int
	if err := metadata.DB.QueryRow("SELECT COUNT(*) FROM audit_events WHERE action='pdf_inventory_registered'").Scan(&events); err != nil {
		t.Fatal(err)
	}
	if err := metadata.DB.QueryRow("SELECT COUNT(*) FROM pdf_audit_links").Scan(&links); err != nil {
		t.Fatal(err)
	}
	if events != 1 || links != 1 {
		t.Fatalf("metadata PDF audit events=%d links=%d, want one each", events, links)
	}
	var runID any
	if err := metadata.DB.QueryRow("SELECT pipeline_run_id FROM audit_events WHERE action='pdf_inventory_registered'").Scan(&runID); err != nil {
		t.Fatal(err)
	}
	if runID != nil {
		t.Fatalf("stale registration audit run ID = %v, want NULL", runID)
	}
}

// TestFlushAuditOutboxDrainsBoundedBatches verifies a full flush processes ordered batches without retaining the backlog.
func TestFlushAuditOutboxDrainsBoundedBatches(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	metadata, err := database.Open(filepath.Join(t.TempDir(), "corpus.metadata.db"), filepath.Join("..", "..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer metadata.Close()
	for index := 0; index < auditOutboxBatchSize+2; index++ {
		if _, err := store.DB.Exec(`INSERT INTO pdf_audit_outbox
			(event_key, occurred_at, actor, entity_type, entity_id, action, metadata_json, correlation_id)
			VALUES (?, ?, 'test', 'work', ?, 'pdf_inventory_registered', '{}', ?)`,
			fmt.Sprintf("batch-%03d", index), fmt.Sprintf("2026-01-01T00:00:%02dZ", index%60), index, fmt.Sprintf("correlation-%03d", index)); err != nil {
			t.Fatal(err)
		}
	}
	flushed, err := store.FlushAuditOutbox(ctx, metadata.DB)
	if err != nil {
		t.Fatal(err)
	}
	if flushed != auditOutboxBatchSize+2 {
		t.Fatalf("flushed=%d, want %d", flushed, auditOutboxBatchSize+2)
	}
	var remaining, events int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM pdf_audit_outbox WHERE delivered_at IS NULL`).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if err := metadata.DB.QueryRow(`SELECT COUNT(*) FROM audit_events WHERE actor='test'`).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 || events != auditOutboxBatchSize+2 {
		t.Fatalf("batch drain remaining=%d events=%d", remaining, events)
	}
}

// TestFlushAuditOutboxContinuesAfterOneBadEvent verifies a failed event does not block later evidence.
func TestFlushAuditOutboxContinuesAfterOneBadEvent(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	metadata, err := database.Open(filepath.Join(t.TempDir(), "corpus.metadata.db"), filepath.Join("..", "..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer metadata.Close()
	if _, err := metadata.DB.Exec(`CREATE TRIGGER reject_bad_pdf_audit BEFORE INSERT ON audit_events
		WHEN NEW.actor='bad' BEGIN SELECT RAISE(ABORT, 'injected audit failure'); END`); err != nil {
		t.Fatal(err)
	}
	for _, event := range []struct{ key, actor string }{{"bad-event", "bad"}, {"good-event", "good"}} {
		if _, err := store.DB.Exec(`INSERT INTO pdf_audit_outbox
			(event_key, occurred_at, actor, entity_type, entity_id, action, metadata_json, correlation_id)
			VALUES (?, '2026-01-01T00:00:00Z', ?, 'work', '1', 'pdf_inventory_registered', '{}', ?)`, event.key, event.actor, event.key); err != nil {
			t.Fatal(err)
		}
	}
	flushed, err := store.FlushAuditOutbox(ctx, metadata.DB)
	if flushed != 1 || err == nil || !strings.Contains(err.Error(), "injected audit failure") {
		t.Fatalf("failure-isolating flush=%d err=%v", flushed, err)
	}
	var goodEvents, remaining int
	if err := metadata.DB.QueryRow(`SELECT COUNT(*) FROM audit_events WHERE actor='good'`).Scan(&goodEvents); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM pdf_audit_outbox WHERE delivered_at IS NULL`).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if goodEvents != 1 || remaining != 1 {
		t.Fatalf("good events=%d remaining=%d", goodEvents, remaining)
	}
}

// TestAddRollsBackWhenAuditOutboxWriteFails verifies add rolls back when audit outbox write fails.
func TestAddRollsBackWhenAuditOutboxWriteFails(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	if _, err := store.Register(ctx, "10.1000/rollback", 1, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`CREATE TRIGGER reject_pdf_outbox
		BEFORE INSERT ON pdf_audit_outbox BEGIN
			SELECT RAISE(ABORT, 'injected outbox failure');
		END`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Add(ctx, "10.1000/rollback", 1, []byte("%PDF-1.7\nrollback")); err == nil || !strings.Contains(err.Error(), "injected outbox failure") {
		t.Fatalf("outbox failure error = %v", err)
	}
	var blobs, documents int
	if err := store.DB.QueryRow("SELECT COUNT(*) FROM pdf_blobs").Scan(&blobs); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.QueryRow("SELECT COUNT(*) FROM pdf_documents").Scan(&documents); err != nil {
		t.Fatal(err)
	}
	if blobs != 0 || documents != 1 {
		t.Fatalf("failed add left blobs=%d documents=%d", blobs, documents)
	}
	document, err := store.Document(ctx, "10.1000/rollback")
	if err != nil || document == nil || document.Status != StatusNotAvailable {
		t.Fatalf("failed add inventory document=%+v err=%v", document, err)
	}
}

// TestBoundStorePathUsesDefaultAndPreservesExistingBinding verifies bound store path uses default and preserves existing binding.
func TestBoundStorePathUsesDefaultAndPreservesExistingBinding(t *testing.T) {
	ctx := context.Background()
	registry := filepath.Join("..", "..", "config", "database.something")
	tempDir := t.TempDir()
	metadataPath := filepath.Join(tempDir, "corpus.metadata.db")
	metadata, err := database.Open(metadataPath, registry)
	if err != nil {
		t.Fatal(err)
	}
	defer metadata.Close()
	path, err := BoundStorePath(ctx, metadata.DB, metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(tempDir, DefaultStoreFilename) {
		t.Fatalf("default PDF store path = %q", path)
	}
	if err := BindStore(ctx, metadata.DB, "other.pdf.db"); err == nil || !strings.Contains(err.Error(), "already bound") {
		t.Fatalf("conflicting binding error = %v", err)
	}
}

// TestStoreIntegrationInvalidInputs verifies store integration invalid inputs.
func TestStoreIntegrationInvalidInputs(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.Add(context.Background(), "", 1, []byte("%PDF-1.7\nmissing DOI")); err == nil {
		t.Fatal("manual insertion accepted an empty DOI")
	}
	if _, err := store.Add(context.Background(), "10.1000/not-normalized", 1, []byte("%PDF-1.7\nunregistered")); err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("unregistered DOI error = %v", err)
	}
}
