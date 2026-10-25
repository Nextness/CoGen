// readiness_integration_test.go verifies the viewer opener checks both schema
// targets, maintenance ownership, companion capabilities, and health evidence.

package server_test

import (
	"analysis/backend/database"
	"analysis/backend/database/pdfbinding"
	"analysis/backend/pdfstore"
	"analysis/backend/server"
	databasehelpers "analysis/tests/support/go/backend/database"
	serverhelpers "analysis/tests/support/go/backend/server"
	"analysis/tests/support/go/testenv"
	"analysis/tests/support/go/testfixture"
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestServerOpenRejectsOutdatedMetadata verifies an older metadata schema fails readiness.
func TestServerOpenRejectsOutdatedMetadata(t *testing.T) {
	registry := testenv.Path("config", "database.something")
	dbPath := filepath.Join(t.TempDir(), "corpus.metadata.db")
	entries := databasehelpers.MigrationEntries(t, "database.corpus.metadata.something")
	databasehelpers.ApplyMigrationPrefix(t, dbPath, testenv.Path("migrations", "corpus.metadata"), entries[:len(entries)-1])
	if _, err := server.Open(dbPath, registry); err == nil || !strings.Contains(err.Error(), "metadata schema is not current") {
		t.Fatalf("error = %v", err)
	}
}

// TestServerOpenRejectsOutdatedPDF verifies an older bound companion schema fails readiness.
func TestServerOpenRejectsOutdatedPDF(t *testing.T) {
	ctx := context.Background()
	registry := testenv.Path("config", "database.something")
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "corpus.metadata.db")
	pdfPath := filepath.Join(tempDir, pdfbinding.DefaultStoreFilename)
	db, err := testfixture.Open(dbPath, registry)
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
	entries := databasehelpers.MigrationEntries(t, "database.corpus.pdf.something")
	databasehelpers.ApplyMigrationPrefix(t, pdfPath, testenv.Path("migrations", "corpus.pdf"), entries[:len(entries)-1])
	if _, err := server.Open(dbPath, registry); err == nil || !strings.Contains(err.Error(), "pdf schema is not current") {
		t.Fatalf("error = %v", err)
	}
}

// TestServerOpenAcceptsReadOnlyCurrentPDF verifies read-only evidence remains available with an explicit capability.
func TestServerOpenAcceptsReadOnlyCurrentPDF(t *testing.T) {
	ctx := context.Background()
	registry := testenv.Path("config", "database.something")
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "corpus.metadata.db")
	pdfPath := filepath.Join(tempDir, pdfbinding.DefaultStoreFilename)
	db, err := testfixture.Open(dbPath, registry)
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
	store, err := pdfstore.Open(pdfPath, registry)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(pdfPath, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(pdfPath, 0o644) })
	viewer, err := server.Open(dbPath, registry)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	if viewer.PDFStoreWritable() {
		t.Fatal("read-only companion reported a writable connection")
	}
	status, health := serverhelpers.RequestJSON(t, viewer.Handler(), "/api/health")
	if status != http.StatusOK {
		t.Fatalf("health status = %d", status)
	}
	if health["pdf_store_writable"] != false || health["pdf_store_write_reason"] == "" {
		t.Fatalf("read-only PDF capability = %v", health)
	}
	schema := health["schema"].(map[string]any)
	if schema["ready"] != true || schema["pdf"].(map[string]any)["ready"] != true {
		t.Fatalf("schema readiness = %v", schema)
	}
	review := health["review"].(map[string]any)
	if review["pdf_store_read_only"] != true {
		t.Fatalf("legacy read-only capability = %v", review)
	}
}

// TestServerOpenRejectsMissingBoundPDF verifies a lost companion is not recreated while serving.
func TestServerOpenRejectsMissingBoundPDF(t *testing.T) {
	ctx := context.Background()
	registry := testenv.Path("config", "database.something")
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "corpus.metadata.db")
	db, err := testfixture.Open(dbPath, registry)
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
	if _, err := server.Open(dbPath, registry); err == nil || !strings.Contains(err.Error(), "missing") || !strings.Contains(err.Error(), "restore") {
		t.Fatalf("error = %v", err)
	}
}

// TestServerOpenRejectsEscapingBinding verifies an unsafe companion path fails readiness.
func TestServerOpenRejectsEscapingBinding(t *testing.T) {
	registry := testenv.Path("config", "database.something")
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "corpus.metadata.db")
	db, err := testfixture.Open(dbPath, registry)
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
	if _, err := server.Open(dbPath, registry); err == nil || !strings.Contains(err.Error(), "stay within") {
		t.Fatalf("error = %v", err)
	}
}

// TestServerHealthReportsSchemaReadiness verifies health reports actual, target, and writable companion capability.
func TestServerHealthReportsSchemaReadiness(t *testing.T) {
	fixture := serverhelpers.NewPDFViewerFixture(t)
	status, health := serverhelpers.RequestJSON(t, fixture.Server.Handler(), "/api/health")
	if status != http.StatusOK {
		t.Fatalf("health status = %d", status)
	}
	schema := health["schema"].(map[string]any)
	metadata := schema["metadata"].(map[string]any)
	if metadata["actual"] != "V00028_immutable_cache_responses.sql" || metadata["target"] != "V00028_immutable_cache_responses.sql" || metadata["ready"] != true {
		t.Fatalf("metadata schema = %v", metadata)
	}
	pdf := schema["pdf"].(map[string]any)
	if pdf["actual"] != "V00003_pdf_document_management.sql" || pdf["target"] != "V00003_pdf_document_management.sql" || pdf["ready"] != true {
		t.Fatalf("PDF schema = %v", pdf)
	}
	if schema["ready"] != true || health["pdf_store_writable"] != true || health["pdf_store_write_reason"] != "" {
		t.Fatalf("bundle readiness = %v", health)
	}
	review := health["review"].(map[string]any)
	if review["pdf_store_read_only"] != false {
		t.Fatalf("writable companion reported read-only = %v", review)
	}
}

// TestMultipleServersShareMaintenanceOwnership verifies current viewers coexist and block schema maintenance.
func TestMultipleServersShareMaintenanceOwnership(t *testing.T) {
	registry := testenv.Path("config", "database.something")
	path, _, _, _ := serverhelpers.ViewerFixture(t)
	first, err := server.Open(path, registry)
	if err != nil {
		t.Fatal(err)
	}
	second, err := server.Open(path, registry)
	if err != nil {
		first.Close()
		t.Fatal(err)
	}
	assertExclusiveMaintenanceBlocked(t, path)
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	assertExclusiveMaintenanceBlocked(t, path)
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	lock, err := database.AcquireMaintenanceLock(context.Background(), path, database.MaintenanceExclusive)
	if err != nil {
		t.Fatalf("maintenance ownership leaked after all viewers closed: %v", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
}

// assertExclusiveMaintenanceBlocked verifies a viewer holds shared maintenance ownership.
func assertExclusiveMaintenanceBlocked(t *testing.T, path string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := database.AcquireMaintenanceLock(ctx, path, database.MaintenanceExclusive); err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("exclusive maintenance error = %v", err)
	}
}
