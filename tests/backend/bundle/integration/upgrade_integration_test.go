// upgrade_integration_test.go proves production-chain upgrades adopt supersedes
// aliases, reject missing or future steps before changing either store, and
// treat a current populated pair as a no-op.

package bundle_test

import (
	"analysis/backend/bundle"
	"analysis/backend/database"
	"analysis/tests/support/go/testenv"
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestPrepareAdoptsSupersededAnchorMigration verifies a database that recorded
// the earlier anchor-label filename adopts the canonical migration without
// rerunning its SQL, then finishes both chains forward.
func TestPrepareAdoptsSupersededAnchorMigration(t *testing.T) {
	ctx := context.Background()
	root, registry := setupBundle(t)
	metadataPath := filepath.Join(root, "corpus.metadata.db")
	pdfPath := filepath.Join(root, "corpus.pdf.db")
	metadataEntries := productionEntries(t, "database.corpus.metadata.something")
	pdfEntries := productionEntries(t, "database.corpus.pdf.something")
	applyPrefix(t, metadataPath, testenv.Path("migrations", "corpus.metadata"), metadataEntries[:26])
	applyPrefix(t, pdfPath, testenv.Path("migrations", "corpus.pdf"), pdfEntries[:2])
	seedMetadata(t, metadataPath)
	seedPDF(t, pdfPath)

	conn, err := database.OpenWritable(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`DELETE FROM schema_migrations WHERE filename = 'V00026_review_anchor_labels.sql';
		INSERT INTO schema_migrations (filename, checksum) VALUES ('V00025_review_anchor_labels.sql', 'legacy-anchor-label-migration')`); err != nil {
		conn.Close()
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}

	report, err := bundle.Prepare(ctx, registry, bundle.Paths{Metadata: metadataPath, PDF: pdfPath})
	if err != nil {
		t.Fatal(err)
	}
	if report.Metadata.Applied != 2 || report.Metadata.Adopted != 1 || report.Metadata.Skipped != 25 {
		t.Fatalf("metadata report = %+v, want 2 applied, 1 adopted, 25 skipped", report.Metadata)
	}
	if report.PDF.Applied != 1 || report.PDF.Skipped != 2 {
		t.Fatalf("PDF report = %+v, want 1 applied, 2 skipped", report.PDF)
	}
	history := appliedFilenames(t, metadataPath)
	if len(history) != len(metadataEntries)+1 {
		t.Fatalf("metadata history = %v, want the alias and canonical rows plus the full chain", history)
	}
	if !slices.Contains(history, "V00025_review_anchor_labels.sql") || !slices.Contains(history, "V00026_review_anchor_labels.sql") {
		t.Fatalf("metadata history did not retain the alias and canonical rows: %v", history)
	}
	if history[len(history)-1] != metadataEntries[len(metadataEntries)-1].Filename {
		t.Fatalf("metadata target = %q, want %q", history[len(history)-1], metadataEntries[len(metadataEntries)-1].Filename)
	}
	if got := queryOne[int](t, metadataPath, "SELECT COUNT(*) FROM searches WHERE search_id = 'preserved-search'"); got != 1 {
		t.Fatalf("preserved search count = %d", got)
	}
	if got := queryOne[int](t, metadataPath, "SELECT COUNT(*) FROM pipeline_runs"); got != 0 {
		t.Fatalf("preparation created %d research attempts", got)
	}
	hash := queryOne[string](t, pdfPath, "SELECT content_hash FROM pdf_documents WHERE doi = '10.1000/available'")
	if data := queryOne[[]byte](t, pdfPath, "SELECT data FROM pdf_blobs WHERE content_hash = ?", hash); string(data) != "%PDF-1.7\npreserved bytes" {
		t.Fatalf("preserved PDF bytes = %q", data)
	}
	if _, err := bundle.VerifyReadiness(ctx, registry, bundle.Paths{Metadata: metadataPath, PDF: pdfPath}); err != nil {
		t.Fatal(err)
	}
}

// TestPrepareRejectsMissingAppliedStepBeforeChangingEitherStore verifies a
// ledger with a gap is rejected before the pending metadata step is applied.
func TestPrepareRejectsMissingAppliedStepBeforeChangingEitherStore(t *testing.T) {
	root, registry := setupBundle(t)
	metadataPath := filepath.Join(root, "corpus.metadata.db")
	pdfPath := filepath.Join(root, "corpus.pdf.db")
	metadataEntries := productionEntries(t, "database.corpus.metadata.something")
	pdfEntries := productionEntries(t, "database.corpus.pdf.something")
	applyPrefix(t, metadataPath, testenv.Path("migrations", "corpus.metadata"), metadataEntries[:27])
	applyPrefix(t, pdfPath, testenv.Path("migrations", "corpus.pdf"), pdfEntries[:2])

	conn, err := database.OpenWritable(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec("DELETE FROM schema_migrations WHERE filename = 'V00010_global_cache.sql'"); err != nil {
		conn.Close()
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}

	paths := bundle.Paths{Metadata: metadataPath, PDF: pdfPath}
	_, err = bundle.Prepare(context.Background(), registry, paths)
	if err == nil || !strings.Contains(err.Error(), "missing applied migration") || !strings.Contains(err.Error(), "V00010_global_cache.sql") {
		t.Fatalf("error = %v", err)
	}
	if _, err := bundle.VerifyReadiness(context.Background(), registry, paths); err == nil || !strings.Contains(err.Error(), "missing applied migration") {
		t.Fatalf("readiness error = %v", err)
	}
	metadataHistory := appliedFilenames(t, metadataPath)
	if slices.Contains(metadataHistory, metadataEntries[len(metadataEntries)-1].Filename) {
		t.Fatalf("metadata pending step was applied despite missing history: %v", metadataHistory)
	}
	if got := appliedFilenames(t, pdfPath); len(got) != len(pdfEntries)-1 {
		t.Fatalf("PDF history changed after missing metadata history: %v", got)
	}
}

// TestPrepareRejectsFuturePDFStepBeforeChangingMetadata verifies future
// companion history is rejected before the metadata store advances.
func TestPrepareRejectsFuturePDFStepBeforeChangingMetadata(t *testing.T) {
	root, registry := setupBundle(t)
	metadataPath := filepath.Join(root, "corpus.metadata.db")
	pdfPath := filepath.Join(root, "corpus.pdf.db")
	metadataEntries := productionEntries(t, "database.corpus.metadata.something")
	pdfEntries := productionEntries(t, "database.corpus.pdf.something")
	applyPrefix(t, metadataPath, testenv.Path("migrations", "corpus.metadata"), metadataEntries[:27])
	applyPrefix(t, pdfPath, testenv.Path("migrations", "corpus.pdf"), pdfEntries[:2])

	conn, err := database.OpenWritable(pdfPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec("INSERT INTO schema_migrations (filename, checksum) VALUES ('V00099_future.sql', 'future')"); err != nil {
		conn.Close()
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}

	paths := bundle.Paths{Metadata: metadataPath, PDF: pdfPath}
	_, err = bundle.Prepare(context.Background(), registry, paths)
	if err == nil || !strings.Contains(err.Error(), "unknown applied migration") {
		t.Fatalf("error = %v", err)
	}
	if _, err := bundle.VerifyReadiness(context.Background(), registry, paths); err == nil || !strings.Contains(err.Error(), "unknown applied migration") {
		t.Fatalf("readiness error = %v", err)
	}
	metadataHistory := appliedFilenames(t, metadataPath)
	if slices.Contains(metadataHistory, metadataEntries[len(metadataEntries)-1].Filename) {
		t.Fatalf("metadata advanced despite future PDF history: %v", metadataHistory)
	}
	pdfHistory := appliedFilenames(t, pdfPath)
	if len(pdfHistory) != len(pdfEntries) || slices.Contains(pdfHistory, pdfEntries[len(pdfEntries)-1].Filename) {
		t.Fatalf("PDF history changed after future history rejection: %v", pdfHistory)
	}
}

// TestPrepareCurrentPopulatedPairIsNoOp verifies a current pair with preserved
// evidence reaches no pending steps, creates no research attempt, and remains readable.
func TestPrepareCurrentPopulatedPairIsNoOp(t *testing.T) {
	ctx := context.Background()
	root, registry := setupBundle(t)
	metadataPath := filepath.Join(root, "corpus.metadata.db")
	pdfPath := filepath.Join(root, "corpus.pdf.db")
	metadataEntries := productionEntries(t, "database.corpus.metadata.something")
	pdfEntries := productionEntries(t, "database.corpus.pdf.something")
	applyPrefix(t, metadataPath, testenv.Path("migrations", "corpus.metadata"), metadataEntries)
	applyPrefix(t, pdfPath, testenv.Path("migrations", "corpus.pdf"), pdfEntries)
	seedMetadata(t, metadataPath)
	seedPDF(t, pdfPath)

	report, err := bundle.Prepare(ctx, registry, bundle.Paths{Metadata: metadataPath, PDF: pdfPath})
	if err != nil {
		t.Fatal(err)
	}
	if report.Metadata.Applied != 0 || report.Metadata.Adopted != 0 || report.Metadata.Skipped != len(metadataEntries) {
		t.Fatalf("metadata report = %+v", report.Metadata)
	}
	if report.PDF.Applied != 0 || report.PDF.Adopted != 0 || report.PDF.Skipped != len(pdfEntries) {
		t.Fatalf("PDF report = %+v", report.PDF)
	}
	if got := queryOne[int](t, metadataPath, "SELECT COUNT(*) FROM searches WHERE search_id = 'preserved-search'"); got != 1 {
		t.Fatalf("preserved search count = %d", got)
	}
	if got := queryOne[int](t, metadataPath, "SELECT COUNT(*) FROM pipeline_runs"); got != 0 {
		t.Fatalf("preparation created %d research attempts", got)
	}
	if got := queryOne[int](t, pdfPath, "SELECT change_counter FROM pdf_documents WHERE doi = '10.1000/available'"); got != 1 {
		t.Fatalf("preserved change_counter = %d, want 1", got)
	}
	readiness, err := bundle.VerifyReadiness(ctx, registry, bundle.Paths{Metadata: metadataPath, PDF: pdfPath})
	if err != nil {
		t.Fatal(err)
	}
	if readiness.Metadata != metadataEntries[len(metadataEntries)-1].Filename || readiness.PDF != pdfEntries[len(pdfEntries)-1].Filename {
		t.Fatalf("readiness = %+v", readiness)
	}
}
