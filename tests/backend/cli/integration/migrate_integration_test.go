// migrate_integration_test.go verifies the explicit migrate command prepares
// both bundle stores and reconciles normalized inventory without a research attempt.

package cli_test

import (
	"analysis/backend/database"
	"analysis/backend/database/pdfbinding"
	"analysis/backend/database/run"
	"analysis/backend/database/search"
	"analysis/backend/database/work"
	"analysis/backend/pdfstore"
	"analysis/internal/migrations"
	"analysis/internal/sqliteread"
	testhelpers "analysis/tests/support/go/backend/database"
	"analysis/tests/support/go/testenv"
	"analysis/tests/support/go/testfixture"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildAnalysisBinary builds the supported application binary for CLI integration tests.
func buildAnalysisBinary(t *testing.T) string {
	t.Helper()
	binaryPath := filepath.Join(t.TempDir(), "analysis")
	command := exec.Command("go", "build", "-o", binaryPath, "./backend/cmd/analysis")
	command.Dir = testenv.Root()
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build analysis: %v\n%s", err, output)
	}
	return binaryPath
}

// runMigrate invokes the explicit bundle preparation command.
func runMigrate(t *testing.T, binaryPath, dbPath string) ([]byte, error) {
	t.Helper()
	command := exec.Command(binaryPath, "migrate", "--db", dbPath)
	command.Dir = testenv.Root()
	return command.CombinedOutput()
}

// appliedHistory reads one store's recorded migration history.
func appliedHistory(t *testing.T, path string) []string {
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

// TestMigratePreparesBundleAndReconcilesInventory verifies both schemas reach their targets and normalized DOI inventory is registered once.
func TestMigratePreparesBundleAndReconcilesInventory(t *testing.T) {
	ctx := context.Background()
	registry := testenv.Path("config", "database.something")
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "corpus.metadata.db")
	metadataEntries := testhelpers.MigrationEntries(t, "database.corpus.metadata.something")
	pdfEntries := testhelpers.MigrationEntries(t, "database.corpus.pdf.something")
	testhelpers.ApplyMigrationPrefix(t, dbPath, testenv.Path("migrations", "corpus.metadata"), metadataEntries[:len(metadataEntries)-1])

	db, err := database.OpenExisting(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	searchID, err := db.Search.FindOrCreateSearch(ctx, "migrate-search")
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	revisionID, _, err := db.Search.UpsertRevision(ctx, search.RevisionInput{
		SearchID: searchID, RevisionLabel: "r1", ConfigArtifactHash: "config", ResolvedManifestHash: "manifest",
	})
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	planID, err := db.Search.FindOrCreatePlanWithInputManifest(ctx, search.PlanInput{
		SearchRevisionID: revisionID, ExecutionFingerprint: "migrate-fingerprint",
		ResolvedManifestHash: "manifest", InputManifestHash: "input",
	})
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	runID, _, err := db.Run.StartAttemptIfIdle(ctx, run.StartAttemptInput{ExecutionPlanID: planID, Step: "migrate"})
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Run.FinishRun(ctx, runID, "completed", "fixture complete"); err != nil {
		db.Close()
		t.Fatal(err)
	}
	workID, err := db.Work.CreateWorkByDOI(ctx, "10.1000/migrate-inventory")
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	if _, err := db.Work.CreateRevision(ctx, &work.Revision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: work.ProducerStageNormalize, Title: "Migrated inventory article",
	}); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	binaryPath := buildAnalysisBinary(t)
	for attempt := 0; attempt < 2; attempt++ {
		output, err := runMigrate(t, binaryPath, dbPath)
		if err != nil {
			t.Fatalf("migrate attempt %d: %v\n%s", attempt+1, err, output)
		}
	}

	if got := appliedHistory(t, dbPath); len(got) != len(metadataEntries) || got[len(got)-1] != metadataEntries[len(metadataEntries)-1].Filename {
		t.Fatalf("metadata history = %v", got)
	}
	pdfPath := filepath.Join(tempDir, pdfbinding.DefaultStoreFilename)
	if got := appliedHistory(t, pdfPath); len(got) != len(pdfEntries) || got[len(got)-1] != pdfEntries[len(pdfEntries)-1].Filename {
		t.Fatalf("PDF history = %v", got)
	}
	metadata, err := testfixture.OpenExistingPlatform(t, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer metadata.Close()
	if relativePath, found, err := metadata.PDFBindings.Binding(ctx); err != nil || !found || relativePath != pdfbinding.DefaultStoreFilename {
		t.Fatalf("binding = %q found=%v err=%v", relativePath, found, err)
	}
	var runCount int
	if err := metadata.Conn.QueryRow("SELECT COUNT(*) FROM pipeline_runs").Scan(&runCount); err != nil {
		t.Fatal(err)
	}
	if runCount != 1 {
		t.Fatalf("migrate created research attempts: %d", runCount)
	}
	pdf, err := pdfstore.Open(pdfPath, registry)
	if err != nil {
		t.Fatal(err)
	}
	defer pdf.Close()
	var documents, delivered int
	if err := pdf.DB.QueryRow("SELECT COUNT(*) FROM pdf_documents WHERE doi = '10.1000/migrate-inventory' AND status = 'not_available'").Scan(&documents); err != nil {
		t.Fatal(err)
	}
	if err := pdf.DB.QueryRow("SELECT COUNT(*) FROM pdf_audit_outbox WHERE delivered_at IS NOT NULL").Scan(&delivered); err != nil {
		t.Fatal(err)
	}
	if documents != 1 || delivered != 1 {
		t.Fatalf("inventory documents=%d delivered audit=%d, want 1 and 1", documents, delivered)
	}
}

// TestMigrateRejectsMissingBoundCompanion verifies a lost companion is not recreated by preparation.
func TestMigrateRejectsMissingBoundCompanion(t *testing.T) {
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
	output, err := runMigrate(t, buildAnalysisBinary(t), dbPath)
	if err == nil || !strings.Contains(string(output), "missing") || !strings.Contains(string(output), "restore") {
		t.Fatalf("migrate missing companion: err=%v output=%s", err, output)
	}
	if _, statErr := os.Stat(filepath.Join(tempDir, pdfbinding.DefaultStoreFilename)); !os.IsNotExist(statErr) {
		t.Fatalf("missing bound companion was recreated: %v", statErr)
	}
}

// TestMigrateRejectsMissingMetadata verifies preparation never creates an absent metadata database.
func TestMigrateRejectsMissingMetadata(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "missing.metadata.db")
	output, err := runMigrate(t, buildAnalysisBinary(t), dbPath)
	if err == nil {
		t.Fatalf("migrate created a missing metadata database: %s", output)
	}
	if _, statErr := os.Stat(dbPath); !os.IsNotExist(statErr) {
		t.Fatalf("missing metadata database was created: %v", statErr)
	}
}
