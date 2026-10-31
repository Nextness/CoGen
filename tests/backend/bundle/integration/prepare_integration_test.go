// prepare_integration_test.go verifies coordinated bundle preparation,
// forward retry, readiness, and maintenance ownership against production
// migration chains and temporary database files.

package bundle_test

import (
	"analysis/backend/bundle"
	"analysis/backend/database"
	"analysis/internal/migrations"
	"analysis/internal/sqliteread"
	"analysis/tests/support/go/testenv"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// setupBundle copies production configuration and migration SQL into an
// isolated bundle root and returns its registry path.
func setupBundle(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	for _, directory := range []string{"config", "migrations/corpus.metadata", "migrations/corpus.pdf"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"database.something", "database.corpus.metadata.something", "database.corpus.pdf.something"} {
		copyFile(t, testenv.Path("config", name), filepath.Join(root, "config", name))
	}
	copySQLDirectory(t, testenv.Path("migrations", "corpus.metadata"), filepath.Join(root, "migrations", "corpus.metadata"))
	copySQLDirectory(t, testenv.Path("migrations", "corpus.pdf"), filepath.Join(root, "migrations", "corpus.pdf"))
	return root, filepath.Join(root, "config", "database.something")
}

// copyFile copies one checked-in file into the temporary bundle.
func copyFile(t *testing.T, source, destination string) {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// copySQLDirectory copies every SQL migration into the temporary bundle.
func copySQLDirectory(t *testing.T, source, destination string) {
	t.Helper()
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		copyFile(t, filepath.Join(source, entry.Name()), filepath.Join(destination, entry.Name()))
	}
}

// productionEntries returns one production migration chain in declaration order.
func productionEntries(t *testing.T, configName string) []migrations.Entry {
	t.Helper()
	entries, err := migrations.LoadChain(testenv.Path("config", configName))
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

// writeChain writes one synthetic migration configuration for a chain prefix.
func writeChain(t *testing.T, path string, entries []migrations.Entry) {
	t.Helper()
	var builder strings.Builder
	builder.WriteString("db_migration: setup = { filename: string; }\n")
	for _, entry := range entries {
		fmt.Fprintf(&builder, "#iteration(\"_db_migration\"): db_migration = { filename = %q, };\n", entry.Filename)
	}
	if err := os.WriteFile(path, []byte(builder.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// applyPrefix creates one store at a production chain prefix.
func applyPrefix(t *testing.T, dbPath, migrationsDir string, entries []migrations.Entry) {
	t.Helper()
	conn, err := database.OpenWritable(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	configPath := filepath.Join(t.TempDir(), "prefix.something")
	writeChain(t, configPath, entries)
	if _, err := migrations.ApplyContext(context.Background(), conn, migrationsDir, configPath); err != nil {
		t.Fatal(err)
	}
}

// appliedFilenames reads one store's recorded migration history read-only.
func appliedFilenames(t *testing.T, path string) []string {
	t.Helper()
	conn, err := sqliteread.Open(path, "sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.Ping(); err != nil {
		t.Fatal(err)
	}
	applied, err := migrations.Applied(context.Background(), conn)
	if err != nil {
		t.Fatal(err)
	}
	return applied
}

// queryOne runs one bounded inspection query against an existing database.
func queryOne[T any](t *testing.T, path, query string, args ...any) T {
	t.Helper()
	conn, err := sqliteread.Open(path, "sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var value T
	if err := conn.QueryRow(query, args...).Scan(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

// seedMetadata inserts one preserved search row before preparation.
func seedMetadata(t *testing.T, path string) {
	t.Helper()
	conn, err := database.OpenWritable(path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Exec("INSERT INTO searches (search_id) VALUES ('preserved-search')"); err != nil {
		t.Fatal(err)
	}
}

// seedPDF inserts available, unavailable, and pending-audit evidence before preparation.
func seedPDF(t *testing.T, path string) {
	t.Helper()
	conn, err := database.OpenWritable(path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	hash := strings.Repeat("a", 64)
	data := []byte("%PDF-1.7\npreserved bytes")
	if _, err := conn.Exec("INSERT INTO pdf_blobs (content_hash, byte_size, data, created_at) VALUES (?, ?, ?, '2026-01-01T00:00:00Z')", hash, len(data), data); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec("INSERT INTO pdf_documents (doi, status, content_hash, inventoried_at, updated_at) VALUES ('10.1000/available', 'available', ?, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')", hash); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec("INSERT INTO pdf_documents (doi, status, content_hash, inventoried_at, updated_at) VALUES ('10.1000/unavailable', 'not_available', NULL, NULL, '2026-01-01T00:00:00Z')"); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`INSERT INTO pdf_audit_outbox
		(event_key, occurred_at, actor, entity_type, entity_id, action, metadata_json, correlation_id)
		VALUES ('pending-event', '2026-01-01T00:00:00Z', 'pipeline', 'work', '1', 'pdf_inventory_registered', '{}', 'pending-correlation')`); err != nil {
		t.Fatal(err)
	}
}

// TestPrepareUpgradesOlderPairPreservesDataAndIsIdempotent verifies coordinated
// preparation reaches both targets, preserves existing evidence, creates no
// research attempt, and repeats safely.
func TestPrepareUpgradesOlderPairPreservesDataAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	root, registry := setupBundle(t)
	metadataPath := filepath.Join(root, "corpus.metadata.db")
	pdfPath := filepath.Join(root, "corpus.pdf.db")
	metadataEntries := productionEntries(t, "database.corpus.metadata.something")
	pdfEntries := productionEntries(t, "database.corpus.pdf.something")
	applyPrefix(t, metadataPath, testenv.Path("migrations", "corpus.metadata"), metadataEntries[:len(metadataEntries)-1])
	applyPrefix(t, pdfPath, testenv.Path("migrations", "corpus.pdf"), pdfEntries[:len(pdfEntries)-1])
	seedMetadata(t, metadataPath)
	seedPDF(t, pdfPath)

	report, err := bundle.Prepare(ctx, registry, bundle.Paths{Metadata: metadataPath, PDF: pdfPath})
	if err != nil {
		t.Fatal(err)
	}
	if report.Metadata.Target != metadataEntries[len(metadataEntries)-1].Filename || report.Metadata.Applied != 1 || report.Metadata.Skipped != len(metadataEntries)-1 {
		t.Fatalf("metadata report = %+v", report.Metadata)
	}
	if report.PDF.Target != pdfEntries[len(pdfEntries)-1].Filename || report.PDF.Applied != 1 || report.PDF.Skipped != len(pdfEntries)-1 {
		t.Fatalf("PDF report = %+v", report.PDF)
	}
	if got := appliedFilenames(t, metadataPath); len(got) != len(metadataEntries) || got[len(got)-1] != metadataEntries[len(metadataEntries)-1].Filename {
		t.Fatalf("metadata history = %v", got)
	}
	if got := appliedFilenames(t, pdfPath); len(got) != len(pdfEntries) || got[len(got)-1] != pdfEntries[len(pdfEntries)-1].Filename {
		t.Fatalf("PDF history = %v", got)
	}
	if got := queryOne[int](t, metadataPath, "SELECT COUNT(*) FROM searches WHERE search_id = 'preserved-search'"); got != 1 {
		t.Fatalf("preserved search count = %d", got)
	}
	if got := queryOne[int](t, metadataPath, "SELECT COUNT(*) FROM pipeline_runs"); got != 0 {
		t.Fatalf("preparation created %d research attempts", got)
	}
	hash := queryOne[string](t, pdfPath, "SELECT content_hash FROM pdf_documents WHERE doi = '10.1000/available'")
	data := queryOne[[]byte](t, pdfPath, "SELECT data FROM pdf_blobs WHERE content_hash = ?", hash)
	if string(data) != "%PDF-1.7\npreserved bytes" {
		t.Fatalf("preserved PDF bytes = %q", data)
	}
	if got := queryOne[int](t, pdfPath, "SELECT change_counter FROM pdf_documents WHERE doi = '10.1000/available'"); got != 1 {
		t.Fatalf("initial change_counter = %d", got)
	}
	if got := queryOne[int](t, pdfPath, "SELECT COUNT(*) FROM pdf_documents WHERE doi = '10.1000/unavailable' AND status = 'not_available'"); got != 1 {
		t.Fatalf("unavailable document count = %d", got)
	}
	if got := queryOne[int](t, pdfPath, "SELECT COUNT(*) FROM pdf_audit_outbox WHERE event_key = 'pending-event' AND before_json IS NULL AND after_json IS NULL"); got != 1 {
		t.Fatalf("legacy pending outbox count = %d", got)
	}
	for _, trigger := range []string{"pdf_documents_abort_delete", "pdf_documents_advance_counter", "pdf_download_attempts_abort_update", "pdf_download_attempts_abort_delete"} {
		if got := queryOne[int](t, pdfPath, "SELECT COUNT(*) FROM sqlite_master WHERE type = 'trigger' AND name = ?", trigger); got != 1 {
			t.Fatalf("missing trigger %s", trigger)
		}
	}

	second, err := bundle.Prepare(ctx, registry, bundle.Paths{Metadata: metadataPath, PDF: pdfPath})
	if err != nil {
		t.Fatal(err)
	}
	if second.Metadata.Applied != 0 || second.Metadata.Skipped != len(metadataEntries) || second.PDF.Applied != 0 || second.PDF.Skipped != len(pdfEntries) {
		t.Fatalf("repeated preparation = %+v", second)
	}
	readiness, err := bundle.VerifyReadiness(ctx, registry, bundle.Paths{Metadata: metadataPath, PDF: pdfPath})
	if err != nil {
		t.Fatal(err)
	}
	if readiness.Metadata != metadataEntries[len(metadataEntries)-1].Filename || readiness.PDF != pdfEntries[len(pdfEntries)-1].Filename {
		t.Fatalf("readiness = %+v", readiness)
	}
}

// TestPrepareValidatesInputsBeforeChangingMetadata verifies a broken second-store
// migration is rejected before the first store changes.
func TestPrepareValidatesInputsBeforeChangingMetadata(t *testing.T) {
	root, registry := setupBundle(t)
	metadataPath := filepath.Join(root, "corpus.metadata.db")
	pdfPath := filepath.Join(root, "corpus.pdf.db")
	metadataEntries := productionEntries(t, "database.corpus.metadata.something")
	pdfEntries := productionEntries(t, "database.corpus.pdf.something")
	applyPrefix(t, metadataPath, testenv.Path("migrations", "corpus.metadata"), metadataEntries[:len(metadataEntries)-1])
	applyPrefix(t, pdfPath, testenv.Path("migrations", "corpus.pdf"), pdfEntries[:len(pdfEntries)-1])
	if err := os.WriteFile(filepath.Join(root, "migrations", "corpus.pdf", pdfEntries[len(pdfEntries)-1].Filename), []byte("-- ==DOWN==\nSELECT 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := bundle.Prepare(context.Background(), registry, bundle.Paths{Metadata: metadataPath, PDF: pdfPath})
	if err == nil || !strings.Contains(err.Error(), "validate PDF migration inputs") || !strings.Contains(err.Error(), "marker") {
		t.Fatalf("error = %v", err)
	}
	if got := appliedFilenames(t, metadataPath); len(got) != len(metadataEntries)-1 || got[len(got)-1] != metadataEntries[len(metadataEntries)-2].Filename {
		t.Fatalf("metadata history changed after input rejection: %v", got)
	}
	if got := appliedFilenames(t, pdfPath); len(got) != len(pdfEntries)-1 {
		t.Fatalf("PDF history changed after input rejection: %v", got)
	}
}

// TestPrepareRetainsMetadataProgressAndResumesPDFForward verifies a failed PDF
// upgrade leaves metadata committed, releases ownership, and retries forward.
func TestPrepareRetainsMetadataProgressAndResumesPDFForward(t *testing.T) {
	ctx := context.Background()
	root, registry := setupBundle(t)
	metadataPath := filepath.Join(root, "corpus.metadata.db")
	pdfPath := filepath.Join(root, "corpus.pdf.db")
	metadataEntries := productionEntries(t, "database.corpus.metadata.something")
	pdfEntries := productionEntries(t, "database.corpus.pdf.something")
	applyPrefix(t, metadataPath, testenv.Path("migrations", "corpus.metadata"), metadataEntries[:len(metadataEntries)-1])
	applyPrefix(t, pdfPath, testenv.Path("migrations", "corpus.pdf"), pdfEntries[:len(pdfEntries)-1])
	seedPDF(t, pdfPath)
	conn, err := database.OpenWritable(pdfPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec("CREATE TRIGGER pdf_documents_abort_delete BEFORE DELETE ON pdf_documents BEGIN SELECT RAISE(ABORT, 'conflict'); END"); err != nil {
		conn.Close()
		t.Fatal(err)
	}
	conn.Close()

	report, err := bundle.Prepare(ctx, registry, bundle.Paths{Metadata: metadataPath, PDF: pdfPath})
	if err == nil || !strings.Contains(err.Error(), "apply pdf migrations") || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error = %v", err)
	}
	if report.Metadata.Target != metadataEntries[len(metadataEntries)-1].Filename || report.Metadata.Applied != 1 {
		t.Fatalf("metadata progress after PDF failure = %+v", report.Metadata)
	}
	if got := appliedFilenames(t, metadataPath); len(got) != len(metadataEntries) || got[len(got)-1] != metadataEntries[len(metadataEntries)-1].Filename {
		t.Fatalf("metadata history after PDF failure = %v", got)
	}
	if got := appliedFilenames(t, pdfPath); len(got) != len(pdfEntries)-1 {
		t.Fatalf("PDF history after failed upgrade = %v", got)
	}
	lock, err := database.AcquireMaintenanceLock(ctx, metadataPath, database.MaintenanceExclusive)
	if err != nil {
		t.Fatalf("maintenance ownership leaked after failure: %v", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}

	conn, err = database.OpenWritable(pdfPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec("DROP TRIGGER pdf_documents_abort_delete"); err != nil {
		conn.Close()
		t.Fatal(err)
	}
	conn.Close()

	retry, err := bundle.Prepare(ctx, registry, bundle.Paths{Metadata: metadataPath, PDF: pdfPath})
	if err != nil {
		t.Fatal(err)
	}
	if retry.Metadata.Applied != 0 || retry.Metadata.Skipped != len(metadataEntries) || retry.PDF.Applied != 1 || retry.PDF.Skipped != len(pdfEntries)-1 {
		t.Fatalf("forward retry = %+v", retry)
	}
	if got := appliedFilenames(t, metadataPath); len(got) != len(metadataEntries) {
		t.Fatalf("metadata history after retry = %v", got)
	}
	if got := appliedFilenames(t, pdfPath); len(got) != len(pdfEntries) || got[len(got)-1] != pdfEntries[len(pdfEntries)-1].Filename {
		t.Fatalf("PDF history after retry = %v", got)
	}
	hash := queryOne[string](t, pdfPath, "SELECT content_hash FROM pdf_documents WHERE doi = '10.1000/available'")
	if data := queryOne[[]byte](t, pdfPath, "SELECT data FROM pdf_blobs WHERE content_hash = ?", hash); string(data) != "%PDF-1.7\npreserved bytes" {
		t.Fatalf("preserved PDF bytes after retry = %q", data)
	}
}

// TestPrepareRejectsUnknownHistoryBeforeChangingEitherStore verifies newer or
// foreign migration history cannot be silently adopted or downgraded.
func TestPrepareRejectsUnknownHistoryBeforeChangingEitherStore(t *testing.T) {
	root, registry := setupBundle(t)
	metadataPath := filepath.Join(root, "corpus.metadata.db")
	pdfPath := filepath.Join(root, "corpus.pdf.db")
	metadataEntries := productionEntries(t, "database.corpus.metadata.something")
	pdfEntries := productionEntries(t, "database.corpus.pdf.something")
	applyPrefix(t, metadataPath, testenv.Path("migrations", "corpus.metadata"), metadataEntries[:len(metadataEntries)-1])
	applyPrefix(t, pdfPath, testenv.Path("migrations", "corpus.pdf"), pdfEntries[:len(pdfEntries)-1])
	conn, err := database.OpenWritable(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec("INSERT INTO schema_migrations (filename, checksum) VALUES ('V00099_future.sql', 'future')"); err != nil {
		conn.Close()
		t.Fatal(err)
	}
	conn.Close()

	if _, err := bundle.VerifyReadiness(context.Background(), registry, bundle.Paths{Metadata: metadataPath, PDF: pdfPath}); err == nil || !strings.Contains(err.Error(), "unknown applied migration") {
		t.Fatalf("readiness unknown history error = %v", err)
	}
	_, err = bundle.Prepare(context.Background(), registry, bundle.Paths{Metadata: metadataPath, PDF: pdfPath})
	if err == nil || !strings.Contains(err.Error(), "unknown applied migration") {
		t.Fatalf("error = %v", err)
	}
	if got := appliedFilenames(t, metadataPath); slices.Contains(got, metadataEntries[len(metadataEntries)-1].Filename) {
		t.Fatalf("metadata was upgraded despite unknown history: %v", got)
	}
	if got := appliedFilenames(t, pdfPath); len(got) != len(pdfEntries)-1 {
		t.Fatalf("PDF history changed despite unknown metadata history: %v", got)
	}
}

// TestVerifyReadinessReportsActualTargetsWithoutMigrating verifies readiness is
// a non-mutating check, reports both actual versions, and accepts a read-only
// current PDF store.
func TestVerifyReadinessReportsActualTargetsWithoutMigrating(t *testing.T) {
	ctx := context.Background()
	root, registry := setupBundle(t)
	metadataPath := filepath.Join(root, "corpus.metadata.db")
	pdfPath := filepath.Join(root, "corpus.pdf.db")
	metadataEntries := productionEntries(t, "database.corpus.metadata.something")
	pdfEntries := productionEntries(t, "database.corpus.pdf.something")
	applyPrefix(t, metadataPath, testenv.Path("migrations", "corpus.metadata"), metadataEntries[:len(metadataEntries)-1])
	applyPrefix(t, pdfPath, testenv.Path("migrations", "corpus.pdf"), pdfEntries[:len(pdfEntries)-1])

	readiness, err := bundle.VerifyReadiness(ctx, registry, bundle.Paths{Metadata: metadataPath, PDF: pdfPath})
	if err == nil {
		t.Fatal("readiness accepted an outdated pair")
	}
	if !strings.Contains(err.Error(), "metadata schema is not current") || !strings.Contains(err.Error(), "pdf schema is not current") {
		t.Fatalf("error = %v", err)
	}
	if readiness.Metadata != metadataEntries[len(metadataEntries)-2].Filename || readiness.PDF != pdfEntries[len(pdfEntries)-2].Filename {
		t.Fatalf("actual readiness = %+v", readiness)
	}
	if got := appliedFilenames(t, metadataPath); len(got) != len(metadataEntries)-1 {
		t.Fatalf("readiness migrated metadata: %v", got)
	}
	if got := appliedFilenames(t, pdfPath); len(got) != len(pdfEntries)-1 {
		t.Fatalf("readiness migrated PDF: %v", got)
	}

	if _, err := bundle.Prepare(ctx, registry, bundle.Paths{Metadata: metadataPath, PDF: pdfPath}); err != nil {
		t.Fatal(err)
	}
	if _, err := bundle.VerifyReadiness(ctx, registry, bundle.Paths{Metadata: metadataPath, PDF: filepath.Join(root, "missing.pdf.db")}); err == nil || !strings.Contains(err.Error(), "pdf database does not exist") {
		t.Fatalf("missing PDF readiness error = %v", err)
	}
	if _, err := bundle.VerifyReadiness(ctx, registry, bundle.Paths{Metadata: filepath.Join(root, "missing.metadata.db"), PDF: pdfPath}); err == nil || !strings.Contains(err.Error(), "metadata database does not exist") {
		t.Fatalf("missing metadata readiness error = %v", err)
	}
	if err := os.Chmod(pdfPath, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(pdfPath, 0o644) })
	readiness, err = bundle.VerifyReadiness(ctx, registry, bundle.Paths{Metadata: metadataPath, PDF: pdfPath})
	if err != nil {
		t.Fatalf("read-only current PDF readiness: %v", err)
	}
	if readiness.Metadata != metadataEntries[len(metadataEntries)-1].Filename || readiness.PDF != pdfEntries[len(pdfEntries)-1].Filename {
		t.Fatalf("current readiness = %+v", readiness)
	}
}

// TestPrepareCreatesMissingBundle verifies preparation can initialize both
// missing stores through the explicit maintenance operation.
func TestPrepareCreatesMissingBundle(t *testing.T) {
	root, registry := setupBundle(t)
	metadataPath := filepath.Join(root, "new.metadata.db")
	pdfPath := filepath.Join(root, "new.pdf.db")
	report, err := bundle.Prepare(context.Background(), registry, bundle.Paths{Metadata: metadataPath, PDF: pdfPath})
	if err != nil {
		t.Fatal(err)
	}
	metadataEntries := productionEntries(t, "database.corpus.metadata.something")
	pdfEntries := productionEntries(t, "database.corpus.pdf.something")
	if report.Metadata.Target != metadataEntries[len(metadataEntries)-1].Filename || report.PDF.Target != pdfEntries[len(pdfEntries)-1].Filename {
		t.Fatalf("new bundle report = %+v", report)
	}
	if got := appliedFilenames(t, metadataPath); len(got) != len(metadataEntries) {
		t.Fatalf("new metadata history = %v", got)
	}
	if got := appliedFilenames(t, pdfPath); len(got) != len(pdfEntries) {
		t.Fatalf("new PDF history = %v", got)
	}
}

// TestPrepareRejectsDuplicateAndEmptyPaths verifies the coordinator refuses to
// prepare one file as both stores.
func TestPrepareRejectsDuplicateAndEmptyPaths(t *testing.T) {
	_, registry := setupBundle(t)
	path := filepath.Join(t.TempDir(), "bundle.db")
	if _, err := bundle.Prepare(context.Background(), registry, bundle.Paths{Metadata: path, PDF: path}); err == nil || !strings.Contains(err.Error(), "must differ") {
		t.Fatalf("duplicate path error = %v", err)
	}
	if _, err := bundle.Prepare(context.Background(), registry, bundle.Paths{Metadata: path}); err == nil || !strings.Contains(err.Error(), "PDF store path is required") {
		t.Fatalf("empty PDF path error = %v", err)
	}
	if _, err := bundle.Prepare(context.Background(), registry, bundle.Paths{PDF: path}); err == nil || !strings.Contains(err.Error(), "metadata database path is required") {
		t.Fatalf("empty metadata path error = %v", err)
	}
	realDir := t.TempDir()
	linkDir := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := bundle.Prepare(context.Background(), registry, bundle.Paths{Metadata: filepath.Join(realDir, "bundle.db"), PDF: filepath.Join(linkDir, "bundle.db")}); err == nil || !strings.Contains(err.Error(), "must differ") {
		t.Fatalf("symlinked duplicate path error = %v", err)
	}
}

// TestMaintenanceOwnershipAllowsSharedOwnersAndExcludesMaintenance verifies
// shared viewers coexist while exclusive preparation waits and releases cleanly.
func TestMaintenanceOwnershipAllowsSharedOwnersAndExcludesMaintenance(t *testing.T) {
	root, registry := setupBundle(t)
	metadataPath := filepath.Join(root, "corpus.metadata.db")
	pdfPath := filepath.Join(root, "corpus.pdf.db")
	first, err := database.AcquireMaintenanceLock(context.Background(), metadataPath, database.MaintenanceShared)
	if err != nil {
		t.Fatal(err)
	}
	second, err := database.AcquireMaintenanceLock(context.Background(), metadataPath, database.MaintenanceShared)
	if err != nil {
		first.Close()
		t.Fatal(err)
	}
	blocked, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := database.AcquireMaintenanceLock(blocked, metadataPath, database.MaintenanceExclusive); err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("exclusive ownership error = %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	exclusive, err := database.AcquireMaintenanceLock(context.Background(), metadataPath, database.MaintenanceExclusive)
	if err != nil {
		t.Fatal(err)
	}
	prepared := make(chan error, 1)
	go func() {
		_, err := bundle.Prepare(context.Background(), registry, bundle.Paths{Metadata: metadataPath, PDF: pdfPath})
		prepared <- err
	}()
	select {
	case err := <-prepared:
		t.Fatalf("preparation completed beneath exclusive maintenance ownership: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := exclusive.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-prepared:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("preparation did not resume after ownership release")
	}
	released, err := database.AcquireMaintenanceLock(context.Background(), metadataPath, database.MaintenanceExclusive)
	if err != nil {
		t.Fatalf("maintenance ownership leaked after preparation: %v", err)
	}
	if err := released.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestPrepareSerializesConcurrentPreparation verifies concurrent maintenance
// operations apply each migration exactly once.
func TestPrepareSerializesConcurrentPreparation(t *testing.T) {
	root, registry := setupBundle(t)
	metadataPath := filepath.Join(root, "corpus.metadata.db")
	pdfPath := filepath.Join(root, "corpus.pdf.db")
	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for index := 0; index < 2; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := bundle.Prepare(context.Background(), registry, bundle.Paths{Metadata: metadataPath, PDF: pdfPath})
			results <- err
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	metadataEntries := productionEntries(t, "database.corpus.metadata.something")
	pdfEntries := productionEntries(t, "database.corpus.pdf.something")
	if got := appliedFilenames(t, metadataPath); len(got) != len(metadataEntries) {
		t.Fatalf("concurrent metadata history = %v", got)
	}
	if got := appliedFilenames(t, pdfPath); len(got) != len(pdfEntries) {
		t.Fatalf("concurrent PDF history = %v", got)
	}
}

// TestPrepareCancellationReleasesOwnership verifies a canceled preparation does
// not leave maintenance ownership held or change either store.
func TestPrepareCancellationReleasesOwnership(t *testing.T) {
	root, registry := setupBundle(t)
	metadataPath := filepath.Join(root, "corpus.metadata.db")
	pdfPath := filepath.Join(root, "corpus.pdf.db")
	exclusive, err := database.AcquireMaintenanceLock(context.Background(), metadataPath, database.MaintenanceExclusive)
	if err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := bundle.Prepare(canceled, registry, bundle.Paths{Metadata: metadataPath, PDF: pdfPath}); err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("canceled preparation error = %v", err)
	}
	if err := exclusive.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(metadataPath); !os.IsNotExist(err) {
		t.Fatalf("canceled preparation created metadata: %v", err)
	}
	if _, err := os.Stat(pdfPath); !os.IsNotExist(err) {
		t.Fatalf("canceled preparation created PDF store: %v", err)
	}
	released, err := database.AcquireMaintenanceLock(context.Background(), metadataPath, database.MaintenanceExclusive)
	if err != nil {
		t.Fatalf("canceled preparation leaked ownership: %v", err)
	}
	if err := released.Close(); err != nil {
		t.Fatal(err)
	}
}
