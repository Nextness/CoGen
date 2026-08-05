//go:build integration

package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"analysis/database"
	"analysis/pdfstore"
)

// TestPrepareCopiesAndSanitizesWithoutMutatingSources verifies the atomic copy-only export boundary.
func TestPrepareCopiesAndSanitizesWithoutMutatingSources(t *testing.T) {
	directory := t.TempDir()
	metadataPath := filepath.Join(directory, "corpus.metadata.db")
	pdfPath := filepath.Join(directory, "corpus.pdf.db")
	registry := filepath.Join("..", "..", "..", "config", "database.something")
	metadata, err := database.Open(metadataPath, registry)
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := pdfstore.Open(pdfPath, registry)
	if err != nil {
		t.Fatal(err)
	}
	if err := pdfstore.BindStore(context.Background(), metadata.DB, filepath.Base(pdfPath)); err != nil {
		t.Fatal(err)
	}
	rawConfig := []byte(`workspace = { reviewer = reviewer_config { username = "Sensitive Researcher", email = "sensitive@example.test", }, };`)
	digest := sha256.Sum256(rawConfig)
	rawHash := hex.EncodeToString(digest[:])
	searchResult := mustExec(t, metadata.DB, "INSERT INTO searches (search_id) VALUES ('export-search')")
	searchID, _ := searchResult.LastInsertId()
	revisionResult := mustExec(t, metadata.DB, "INSERT INTO search_revisions (search_id, revision_label, config_artifact_hash, resolved_manifest_hash) VALUES (?, 'r1', ?, 'manifest')", searchID, rawHash)
	revisionID, _ := revisionResult.LastInsertId()
	planResult := mustExec(t, metadata.DB, "INSERT INTO execution_plans (search_revision_id, execution_fingerprint, resolved_manifest_hash, input_manifest_hash) VALUES (?, 'fingerprint', 'manifest', 'input')", revisionID)
	planID, _ := planResult.LastInsertId()
	runResult := mustExec(t, metadata.DB, "INSERT INTO pipeline_runs (step, started_at, finished_at, status, execution_plan_id, attempt_number) VALUES ('export', '2026-01-01', '2026-01-01', 'completed', ?, 1)", planID)
	runID, _ := runResult.LastInsertId()
	if err := metadata.PipelineRunReviewers.Insert(runID, "Sensitive Researcher", "sensitive@example.test"); err != nil {
		t.Fatal(err)
	}
	artifactResult := mustExec(t, metadata.DB, "INSERT INTO artifacts (content_hash, byte_size, content_type) VALUES (?, ?, 'text/plain')", rawHash, len(rawConfig))
	artifactID, _ := artifactResult.LastInsertId()
	mustExec(t, metadata.DB, "INSERT INTO artifact_blobs (artifact_id, pipeline_run_id, data) VALUES (?, ?, ?)", artifactID, runID, rawConfig)
	mustExec(t, metadata.DB, "INSERT INTO run_artifacts (pipeline_run_id, artifact_id, artifact_role) VALUES (?, ?, 'workspace_config')", runID, artifactID)
	var sourceCorpusID string
	if err := metadata.DB.QueryRow("SELECT corpus_id FROM review_settings WHERE id=1").Scan(&sourceCorpusID); err != nil {
		t.Fatal(err)
	}
	if err := pdf.Close(); err != nil {
		t.Fatal(err)
	}
	if err := metadata.Close(); err != nil {
		t.Fatal(err)
	}
	metadataBefore, _ := fileHash(metadataPath)
	pdfBefore, _ := fileHash(pdfPath)
	out := filepath.Join(directory, "export")
	if err := prepare(context.Background(), options{DB: metadataPath, Out: out}); err != nil {
		t.Fatal(err)
	}
	metadataAfter, _ := fileHash(metadataPath)
	pdfAfter, _ := fileHash(pdfPath)
	if metadataBefore != metadataAfter || pdfBefore != pdfAfter {
		t.Fatal("prepare changed a source database")
	}
	copy, err := sql.Open("sqlite", filepath.Join(out, filepath.Base(metadataPath)))
	if err != nil {
		t.Fatal(err)
	}
	defer copy.Close()
	var corpusID string
	if err := copy.QueryRow("SELECT corpus_id FROM review_settings WHERE id=1").Scan(&corpusID); err != nil || len(corpusID) != 32 || corpusID == sourceCorpusID {
		t.Fatalf("export corpus ID: %q, %v", corpusID, err)
	}
	var username, email, sanitizedHash string
	if err := copy.QueryRow("SELECT username, email FROM pipeline_run_reviewers WHERE pipeline_run_id=?", runID).Scan(&username, &email); err != nil || username != "" || email != "" {
		t.Fatalf("redacted reviewer: %q %q %v", username, email, err)
	}
	if err := copy.QueryRow("SELECT config_artifact_hash FROM search_revisions WHERE id=?", revisionID).Scan(&sanitizedHash); err != nil || sanitizedHash == rawHash {
		t.Fatalf("sanitized config hash: %q %v", sanitizedHash, err)
	}
	metadataBytes, err := os.ReadFile(filepath.Join(out, filepath.Base(metadataPath)))
	if err != nil || strings.Contains(string(metadataBytes), "Sensitive Researcher") || strings.Contains(string(metadataBytes), "sensitive@example.test") {
		t.Fatalf("export metadata retained reviewer bytes: %v", err)
	}
	manifestBytes, err := os.ReadFile(filepath.Join(out, "export-manifest.json"))
	var manifest exportManifest
	if err == nil {
		err = json.Unmarshal(manifestBytes, &manifest)
	}
	if err != nil || !strings.Contains(manifest.BrowserDraftDisclaimer, "Browser-local drafts") {
		t.Fatalf("export manifest: %v\n%s", err, manifestBytes)
	}
	if manifest.SourceSchemaVersions["metadata"] == "" || manifest.SourceSchemaVersions["pdf"] == "" || manifest.SourceSchemaVersions["metadata"] != manifest.SanitizedSchemaVersions["metadata"] || manifest.SourceSchemaVersions["pdf"] != manifest.SanitizedSchemaVersions["pdf"] {
		t.Fatalf("export schema versions: %#v %#v", manifest.SourceSchemaVersions, manifest.SanitizedSchemaVersions)
	}
	if manifest.Files[filepath.Base(metadataPath)] == "" || manifest.Files[filepath.Base(pdfPath)] == "" {
		t.Fatalf("export file hashes: %#v", manifest.Files)
	}
	if err := prepare(context.Background(), options{DB: metadataPath, Out: out}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("existing output error=%v", err)
	}
}

// mustExec runs one fixture statement and fails the current test on error.
func mustExec(t *testing.T, db *sql.DB, query string, args ...any) sql.Result {
	t.Helper()
	result, err := db.Exec(query, args...)
	if err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
	return result
}

// TestSanitizeReviewerAssignments verifies inline redaction and fail-closed references.
func TestSanitizeReviewerAssignments(t *testing.T) {
	source := []byte("workspace = { reviewer = reviewer_config { username = \"Researcher\", email = \"person@example.test\", }, title = \"reviewer = untouched\", query = #multiline RAW\nreviewer = reviewer_config { username = \"research text\" }\nRAW, };")
	redacted, changed, err := sanitizeReviewerAssignments(source)
	if err != nil || !changed {
		t.Fatalf("sanitize: changed=%v err=%v", changed, err)
	}
	if strings.Contains(string(redacted), "Researcher") || strings.Contains(string(redacted), "person@example.test") || !strings.Contains(string(redacted), `title = "reviewer = untouched"`) || !strings.Contains(string(redacted), `username = "research text"`) {
		t.Fatalf("unexpected sanitized source: %s", redacted)
	}
	if _, _, err := sanitizeReviewerAssignments([]byte(`workspace = { reviewer = make_reviewer!(), };`)); err == nil {
		t.Fatal("expected macro-backed reviewer to fail closed")
	}
}

// TestConfigurationCopyRejectsSymlinkEscape verifies that configuration includes cannot follow links outside the copied root.
func TestConfigurationCopyRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.something")
	if err := os.WriteFile(outside, []byte(`workspace = { reviewer = reviewer_config { username = "Sensitive", email = "", }, };`), 0o644); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(root, "outside.something")
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}
	mainPath := filepath.Join(root, "main.something")
	if err := os.WriteFile(mainPath, []byte(`#include("outside.something");`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := copyAndSanitizeConfiguration(mainPath, t.TempDir()); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("expected symlink escape rejection, got %v", err)
	}
}
