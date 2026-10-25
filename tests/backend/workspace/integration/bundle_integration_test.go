// bundle_integration_test.go verifies the workspace pipeline prepares both
// bundle schemas before creating or reusing a research attempt.

package workspace_test

import (
	"analysis/backend/database/pdfbinding"
	"analysis/backend/pdfstore"
	"analysis/backend/workspace"
	"analysis/tests/support/go/testenv"
	"analysis/tests/support/go/testfixture"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunPipelineRejectsMissingBoundCompanionBeforeAttempt verifies preparation
// fails before a research attempt is created when a bound companion is missing.
func TestRunPipelineRejectsMissingBoundCompanionBeforeAttempt(t *testing.T) {
	t.Chdir(testenv.Root())
	ctx := context.Background()
	registry := testenv.Path("config", "database.something")
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "corpus.metadata.db")
	sourcePath := filepath.Join(tempDir, "source.csv")
	if err := os.WriteFile(sourcePath, []byte("doi,title,year\n10.1000/missing-companion,Preparation article,2024\n"), 0o644); err != nil {
		t.Fatal(err)
	}
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

	err = workspace.RunPipeline(dbPath, []byte("workspace-config"), testWorkspaceRun(sourcePath), false)
	if err == nil || !strings.Contains(err.Error(), "prepare database bundle") || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("error = %v", err)
	}
	metadata, err := testfixture.OpenExistingPlatform(t, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer metadata.Close()
	var runs int
	if err := metadata.Conn.QueryRow("SELECT COUNT(*) FROM pipeline_runs").Scan(&runs); err != nil {
		t.Fatal(err)
	}
	if runs != 0 {
		t.Fatalf("failed preparation created %d research attempts", runs)
	}
	if _, statErr := os.Stat(filepath.Join(tempDir, pdfbinding.DefaultStoreFilename)); !os.IsNotExist(statErr) {
		t.Fatalf("missing bound companion was recreated: %v", statErr)
	}
}

// TestRunPipelinePreparesDefaultCompanion verifies a fresh pipeline creates and
// binds the default companion before completing its research attempt.
func TestRunPipelinePreparesDefaultCompanion(t *testing.T) {
	t.Chdir(testenv.Root())
	registry := testenv.Path("config", "database.something")
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "corpus.metadata.db")
	sourcePath := filepath.Join(tempDir, "source.csv")
	if err := os.WriteFile(sourcePath, []byte("doi,title,year,authors,publisher,cited_references\n10.1000/prepared-companion,Prepared article,2024,Alice Smith,Example Publisher,10.5555/reference\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := testWorkspaceRun(sourcePath)
	run.Manifest.SearchID = "prepared-companion"
	run.Manifest.Sources[0].KeepFields = []string{"doi", "title", "year", "authors", "publisher", "cited_references"}
	if err := workspace.RunPipeline(dbPath, []byte("workspace-config"), run, false); err != nil {
		t.Fatal(err)
	}
	metadata, err := testfixture.OpenExistingPlatform(t, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer metadata.Close()
	var runs int
	if err := metadata.Conn.QueryRow("SELECT COUNT(*) FROM pipeline_runs").Scan(&runs); err != nil {
		t.Fatal(err)
	}
	if runs != 1 {
		t.Fatalf("pipeline attempts = %d, want 1", runs)
	}
	if relativePath, found, err := metadata.PDFBindings.Binding(context.Background()); err != nil || !found || relativePath != pdfbinding.DefaultStoreFilename {
		t.Fatalf("binding = %q found=%v err=%v", relativePath, found, err)
	}
	pdf, err := pdfstore.Open(filepath.Join(tempDir, pdfbinding.DefaultStoreFilename), registry)
	if err != nil {
		t.Fatal(err)
	}
	defer pdf.Close()
	var migrations, documents int
	if err := pdf.DB.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&migrations); err != nil {
		t.Fatal(err)
	}
	if err := pdf.DB.QueryRow("SELECT COUNT(*) FROM pdf_documents WHERE doi = '10.1000/prepared-companion' AND status = 'not_available'").Scan(&documents); err != nil {
		t.Fatal(err)
	}
	if migrations != 3 || documents != 1 {
		t.Fatalf("PDF migrations=%d documents=%d, want 3 and 1", migrations, documents)
	}
}
