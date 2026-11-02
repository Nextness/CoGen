// resolve_integration_test.go verifies binding-aware bundle preparation
// initializes an unbound companion and rejects missing or unsafe bindings.

package bundle_test

import (
	"analysis/backend/bundle"
	"analysis/backend/database"
	"analysis/backend/database/pdfbinding"
	"analysis/tests/support/go/testfixture"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPrepareResolvedInitializesUnboundCompanion verifies preparation binds and creates the default companion.
func TestPrepareResolvedInitializesUnboundCompanion(t *testing.T) {
	ctx := context.Background()
	root, registry := setupBundle(t)
	metadataPath := filepath.Join(root, "corpus.metadata.db")
	report, err := bundle.PrepareResolved(ctx, registry, metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	wantPDF := filepath.Join(root, pdfbinding.DefaultStoreFilename)
	if report.Resolved.Metadata != metadataPath || report.Resolved.PDF != wantPDF {
		t.Fatalf("resolved paths = %+v, want metadata %q and PDF %q", report.Resolved, metadataPath, wantPDF)
	}
	metadataEntries := productionEntries(t, "database.corpus.metadata.something")
	pdfEntries := productionEntries(t, "database.corpus.pdf.something")
	if got := appliedFilenames(t, metadataPath); len(got) != len(metadataEntries) {
		t.Fatalf("metadata history = %v", got)
	}
	if got := appliedFilenames(t, wantPDF); len(got) != len(pdfEntries) {
		t.Fatalf("PDF history = %v", got)
	}
	db, err := database.OpenExisting(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	relativePath, found, err := db.PDFBindings.Binding(ctx)
	if err != nil || !found || relativePath != pdfbinding.DefaultStoreFilename {
		t.Fatalf("binding = %q found=%v err=%v", relativePath, found, err)
	}
	if got := queryOne[int](t, metadataPath, "SELECT COUNT(*) FROM pipeline_runs"); got != 0 {
		t.Fatalf("preparation created %d research attempts", got)
	}
}

// TestPrepareResolvedRejectsMissingBoundCompanion verifies a lost companion is not silently recreated.
func TestPrepareResolvedRejectsMissingBoundCompanion(t *testing.T) {
	ctx := context.Background()
	root, registry := setupBundle(t)
	metadataPath := filepath.Join(root, "corpus.metadata.db")
	db, err := testfixture.Open(metadataPath, registry)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.PDFBindings.EnsureBinding(ctx, pdfbinding.DefaultStoreFilename); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = bundle.PrepareResolved(ctx, registry, metadataPath)
	if err == nil || !strings.Contains(err.Error(), "missing") || !strings.Contains(err.Error(), "restore") {
		t.Fatalf("error = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, pdfbinding.DefaultStoreFilename)); !os.IsNotExist(statErr) {
		t.Fatalf("missing bound companion was recreated: %v", statErr)
	}
}

// TestPrepareResolvedRejectsEscapingBinding verifies a traversal binding is rejected without creating a file.
func TestPrepareResolvedRejectsEscapingBinding(t *testing.T) {
	ctx := context.Background()
	root, registry := setupBundle(t)
	metadataPath := filepath.Join(root, "corpus.metadata.db")
	db, err := testfixture.Open(metadataPath, registry)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Conn.Exec(`INSERT INTO pdf_store_binding
		(id, relative_path, configured_at, config_fingerprint)
		VALUES (1, '../escape.db', '2026-01-01T00:00:00Z', ?)`, strings.Repeat("e", 64)); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = bundle.PrepareResolved(ctx, registry, metadataPath)
	if err == nil || !strings.Contains(err.Error(), "stay within") {
		t.Fatalf("error = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(filepath.Dir(root), "escape.db")); !os.IsNotExist(statErr) {
		t.Fatalf("escaping binding created a file: %v", statErr)
	}
}
