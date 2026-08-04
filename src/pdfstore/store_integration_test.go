// store_integration_test.go tests PDF store operations against real
// temporary SQLite databases.
//go:build integration

package pdfstore

import (
	"context"
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
