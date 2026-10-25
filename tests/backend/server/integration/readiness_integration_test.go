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

// TestAuditRelayDeliversPendingEventsAndReportsHealth verifies the serving
// lifecycle starts a bounded relay, drains pending evidence, and reports status.
func TestAuditRelayDeliversPendingEventsAndReportsHealth(t *testing.T) {
	fixture := serverhelpers.NewPDFViewerFixture(t)
	registry := testenv.Path("config", "database.something")
	insertPendingPDFAuditEvent(t, fixture.MetadataPath, registry, "relay-pending")

	status, health := serverhelpers.RequestJSON(t, fixture.Server.Handler(), "/api/health")
	if status != http.StatusOK {
		t.Fatalf("health status = %d", status)
	}
	if health["pdf_audit_pending"] != float64(1) || health["pdf_audit_relay_running"] != false || health["pdf_audit_relay_error"] != "" {
		t.Fatalf("pre-relay health = %v", health)
	}

	fixture.Server.StartAuditRelay(context.Background())
	t.Cleanup(fixture.Server.StopAuditRelay)
	deadline := time.Now().Add(2 * time.Second)
	for {
		_, health = serverhelpers.RequestJSON(t, fixture.Server.Handler(), "/api/health")
		if health["pdf_audit_pending"] == float64(0) && health["pdf_audit_relay_running"] == true && health["pdf_audit_relay_error"] == "" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("relay did not drain pending events: %v", health)
		}
		time.Sleep(10 * time.Millisecond)
	}
	fixture.Server.StopAuditRelay()
	_, health = serverhelpers.RequestJSON(t, fixture.Server.Handler(), "/api/health")
	if health["pdf_audit_relay_running"] != false {
		t.Fatalf("relay still running after stop: %v", health)
	}
}

// TestAuditRelayReportsSafeFailureAndRetries verifies relay failures surface in
// health without losing the pending event, and a later relay drains it.
func TestAuditRelayReportsSafeFailureAndRetries(t *testing.T) {
	fixture := serverhelpers.NewPDFViewerFixture(t)
	registry := testenv.Path("config", "database.something")
	insertPendingPDFAuditEvent(t, fixture.MetadataPath, registry, "relay-retry")
	if _, err := fixture.Server.Metadata.Conn.Exec(`CREATE TRIGGER reject_relay_audit
		BEFORE INSERT ON audit_events
		BEGIN SELECT RAISE(ABORT, 'injected relay failure'); END`); err != nil {
		t.Fatal(err)
	}
	fixture.Server.StartAuditRelay(context.Background())
	t.Cleanup(fixture.Server.StopAuditRelay)
	deadline := time.Now().Add(2 * time.Second)
	for {
		_, health := serverhelpers.RequestJSON(t, fixture.Server.Handler(), "/api/health")
		if health["pdf_audit_relay_error"] != "" && health["pdf_audit_pending"] == float64(1) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("relay failure was not reported: %v", health)
		}
		time.Sleep(10 * time.Millisecond)
	}
	fixture.Server.StopAuditRelay()
	if _, err := fixture.Server.Metadata.Conn.Exec("DROP TRIGGER reject_relay_audit"); err != nil {
		t.Fatal(err)
	}
	fixture.Server.StartAuditRelay(context.Background())
	deadline = time.Now().Add(2 * time.Second)
	for {
		_, health := serverhelpers.RequestJSON(t, fixture.Server.Handler(), "/api/health")
		if health["pdf_audit_relay_error"] == "" && health["pdf_audit_pending"] == float64(0) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("relay retry did not drain pending events: %v", health)
		}
		time.Sleep(10 * time.Millisecond)
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

// insertPendingPDFAuditEvent inserts one undelivered companion audit event.
func insertPendingPDFAuditEvent(t *testing.T, metadataPath, registry, eventKey string) {
	t.Helper()
	pdfPath := filepath.Join(filepath.Dir(metadataPath), pdfbinding.DefaultStoreFilename)
	store, err := pdfstore.Open(pdfPath, registry)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO pdf_audit_outbox
		(event_key, occurred_at, actor, entity_type, entity_id, action, metadata_json, correlation_id)
		VALUES (?, '2026-01-01T00:00:00Z', 'pipeline', 'work', '1', 'pdf_inventory_registered', '{}', ?)`, eventKey, eventKey); err != nil {
		store.Close()
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
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
