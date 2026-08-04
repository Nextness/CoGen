// main_integration_test.go tests the pdf-store CLI tool, verifying that Add
// requires a corpus-matching DOI and preserves existing downloads.
//go:build integration

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"analysis/database"
	"analysis/pdfstore"
)

// TestAddRequiresCorpusDOIAndPreservesExistingDownload verifies add requires corpus doi and preserves existing download.
func TestAddRequiresCorpusDOIAndPreservesExistingDownload(t *testing.T) {
	registry := filepath.Join("..", "..", "..", "config", "database.something")
	tempDir := t.TempDir()
	metadataPath := filepath.Join(tempDir, "corpus.metadata.db")
	metadata, err := database.Open(metadataPath, registry)
	if err != nil {
		t.Fatal(err)
	}
	workID, err := metadata.Works.CreateByDOI("10.1000/manual")
	if err != nil {
		t.Fatal(err)
	}
	duplicateWorkID, err := metadata.Works.CreateByDOI("10.1000/manual-duplicate")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := metadata.Works.CreateByDOI("10.1000/manual-unnormalized"); err != nil {
		t.Fatal(err)
	}
	runID, err := metadata.PipelineRuns.StartRun("manual-pdf-test", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, work := range []struct {
		id    int64
		title string
	}{
		{workID, "Manual PDF"},
		{duplicateWorkID, "Manual duplicate PDF"},
	} {
		if _, err := metadata.WorkRevisions.Create(&database.WorkRevision{
			WorkID: work.id, PipelineRunID: runID,
			ProducerStage: database.ProducerStageNormalize, Title: work.title,
		}); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	if err := pdfstore.BindStore(ctx, metadata.DB, pdfstore.DefaultStoreFilename); err != nil {
		t.Fatal(err)
	}
	store, err := pdfstore.Open(filepath.Join(tempDir, pdfstore.DefaultStoreFilename), registry)
	if err != nil {
		t.Fatal(err)
	}
	for _, work := range []struct {
		doi string
		id  int64
	}{{"10.1000/manual", workID}, {"10.1000/manual-duplicate", duplicateWorkID}} {
		if _, err := store.Register(ctx, work.doi, work.id, runID); err != nil {
			store.Close()
			t.Fatal(err)
		}
	}
	if _, err := store.FlushAuditOutbox(ctx, metadata.DB); err != nil {
		store.Close()
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := metadata.Close(); err != nil {
		t.Fatal(err)
	}
	firstPath := filepath.Join(tempDir, "first.pdf")
	if err := os.WriteFile(firstPath, []byte("%PDF-1.7\nmanual first"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := addWithRegistry(metadataPath, "https://doi.org/10.1000/MANUAL", firstPath, registry); err != nil {
		t.Fatal(err)
	}
	store, err = pdfstore.Open(filepath.Join(tempDir, "corpus.pdf.db"), registry)
	if err != nil {
		t.Fatal(err)
	}
	document, err := store.Document(context.Background(), "10.1000/manual")
	if err != nil {
		t.Fatal(err)
	}
	if document == nil || document.Status != pdfstore.StatusAvailable || document.InventoriedAt == "" {
		t.Fatalf("manual document = %+v", document)
	}
	firstHash := document.ContentHash
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	secondPath := filepath.Join(tempDir, "second.pdf")
	if err := os.WriteFile(secondPath, []byte("%PDF-1.7\nmanual replacement"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := addWithRegistry(metadataPath, "10.1000/manual", secondPath, registry); err != nil {
		t.Fatal(err)
	}
	store, err = pdfstore.Open(filepath.Join(tempDir, "corpus.pdf.db"), registry)
	if err != nil {
		t.Fatal(err)
	}
	document, err = store.Document(context.Background(), "10.1000/manual")
	if err != nil {
		t.Fatal(err)
	}
	if document.ContentHash != firstHash {
		t.Fatalf("existing selected PDF changed: %s != %s", document.ContentHash, firstHash)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := addWithRegistry(metadataPath, "10.1000/manual-duplicate", firstPath, registry); err != nil {
		t.Fatal(err)
	}
	store, err = pdfstore.Open(filepath.Join(tempDir, "corpus.pdf.db"), registry)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var blobs, documents int
	if err := store.DB.QueryRow("SELECT COUNT(*) FROM pdf_blobs").Scan(&blobs); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.QueryRow("SELECT COUNT(*) FROM pdf_documents WHERE status='available'").Scan(&documents); err != nil {
		t.Fatal(err)
	}
	if blobs != 1 || documents != 2 {
		t.Fatalf("manual deduplication state: blobs=%d documents=%d", blobs, documents)
	}

	if err := addWithRegistry(metadataPath, "10.1000/unknown", firstPath, registry); err == nil || !strings.Contains(err.Error(), "does not belong") {
		t.Fatalf("unknown DOI error = %v", err)
	}
	if err := addWithRegistry(metadataPath, "10.1000/manual-unnormalized", firstPath, registry); err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("unnormalized DOI error = %v", err)
	}
	metadata, err = database.Open(metadataPath, registry)
	if err != nil {
		t.Fatal(err)
	}
	defer metadata.Close()
	var addedEvents int
	if err := metadata.DB.QueryRow("SELECT COUNT(*) FROM audit_events WHERE entity_type='work' AND entity_id=? AND action='pdf_document_inventoried'", workID).Scan(&addedEvents); err != nil {
		t.Fatal(err)
	}
	if addedEvents != 1 {
		t.Fatalf("manual add audit events = %d, want 1", addedEvents)
	}
	var duplicateEvents int
	if err := metadata.DB.QueryRow("SELECT COUNT(*) FROM audit_events WHERE entity_type='work' AND entity_id=? AND action='pdf_document_inventoried'", duplicateWorkID).Scan(&duplicateEvents); err != nil {
		t.Fatal(err)
	}
	if duplicateEvents != 1 {
		t.Fatalf("manual duplicate-content audit events = %d, want 1", duplicateEvents)
	}
}

// TestAddRejectsInvalidAndOversizedPDFs verifies add rejects invalid and oversized pd fs.
func TestAddRejectsInvalidAndOversizedPDFs(t *testing.T) {
	registry := filepath.Join("..", "..", "..", "config", "database.something")
	tempDir := t.TempDir()
	metadataPath := filepath.Join(tempDir, "corpus.metadata.db")
	metadata, err := database.Open(metadataPath, registry)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := metadata.Works.CreateByDOI("10.1000/manual-invalid"); err != nil {
		t.Fatal(err)
	}
	metadata.Close()
	invalidPath := filepath.Join(tempDir, "invalid.pdf")
	if err := os.WriteFile(invalidPath, []byte("not a PDF"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := addWithRegistry(metadataPath, "10.1000/manual-invalid", invalidPath, registry); err == nil || !strings.Contains(err.Error(), "PDF header") {
		t.Fatalf("invalid PDF error = %v", err)
	}
	oversizedPath := filepath.Join(tempDir, "oversized.pdf")
	oversized := append([]byte("%PDF-1.7\n"), make([]byte, pdfstore.DefaultMaxPDFBytes)...)
	if err := os.WriteFile(oversizedPath, oversized, 0644); err != nil {
		t.Fatal(err)
	}
	if err := addWithRegistry(metadataPath, "10.1000/manual-invalid", oversizedPath, registry); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized PDF error = %v", err)
	}
}
