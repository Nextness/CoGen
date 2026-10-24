// Package pdfexport_test verifies DOI exports against temporary databases and production PDF inventory APIs.
package pdfexport_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"analysis/backend/database/pdfbinding"
	"analysis/backend/database/work"
	"analysis/backend/pdfstore"
	"analysis/tests/support/go/testenv"
	"analysis/tests/support/go/testfixture"
	subject "analysis/tooling/go/pdf-export"
)

// exportCorpus creates a migrated corpus with safe-title, long-title, and unavailable PDF cases.
func exportCorpus(t *testing.T) (string, string, []byte) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	metadataPath := filepath.Join(dir, "corpus.metadata.db")
	registry := testenv.Path("config", "database.something")
	metadata, err := testfixture.Open(metadataPath, registry)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { metadata.Close() })
	if err := metadata.PDFBindings.EnsureBinding(ctx, pdfbinding.DefaultStoreFilename); err != nil {
		t.Fatal(err)
	}
	companionPath := filepath.Join(dir, pdfbinding.DefaultStoreFilename)
	companion, err := pdfstore.Open(companionPath, registry)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { companion.Close() })
	runID, err := testfixture.Run(ctx, metadata.Conn, "pdf-export", "")
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("%PDF-1.7\nsynthetic export fixture\n%%EOF\n")
	for _, paper := range []struct {
		doi       string
		title     string
		available bool
	}{
		{"10.1000/one", "A study: paths / Unicode α?", true},
		{"10.1000/two", strings.Repeat("é", 160), true},
		{"10.1000/missing", "Missing PDF", false},
	} {
		id, err := metadata.Work.CreateWorkByDOI(ctx, paper.doi)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := metadata.Work.CreateRevision(ctx, &work.Revision{
			WorkID: id, PipelineRunID: runID, ProducerStage: work.ProducerStageNormalize, Title: paper.title,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := metadata.Work.CreateRevision(ctx, &work.Revision{
			WorkID: id, PipelineRunID: runID, ProducerStage: work.ProducerStageEnrich, Title: "Later unnormalized title",
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := companion.Register(ctx, paper.doi, id, runID); err != nil {
			t.Fatal(err)
		}
		if paper.available {
			if _, err := companion.Add(ctx, paper.doi, id, data); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := companion.Close(); err != nil {
		t.Fatal(err)
	}
	if err := metadata.Close(); err != nil {
		t.Fatal(err)
	}
	return metadataPath, companionPath, data
}

// TestExportPreservesDatabasesAndContent verifies title selection, DOI deduplication, safe filenames, and repeat exports.
func TestExportPreservesDatabasesAndContent(t *testing.T) {
	metadata, companion, data := exportCorpus(t)
	before := make(map[string][32]byte)
	for _, path := range []string{metadata, companion} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		before[path] = sha256.Sum256(content)
	}
	out := filepath.Join(t.TempDir(), "papers")
	dois := []string{"https://doi.org/10.1000/ONE", "10.1000/one", "10.1000/two"}
	for attempt := 0; attempt < 2; attempt++ {
		results, err := subject.Export(context.Background(), metadata, dois, out)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 2 {
			t.Fatalf("exported %d results, want 2", len(results))
		}
		if filepath.Base(results[0].Path) != "A study- paths - Unicode α-.pdf" {
			t.Fatalf("unexpected title filename %q", filepath.Base(results[0].Path))
		}
		for _, result := range results {
			content, err := os.ReadFile(result.Path)
			if err != nil || !bytes.Equal(content, data) {
				t.Fatalf("exported bytes differ: %v", err)
			}
			if filepath.Dir(result.Path) != out || len(filepath.Base(result.Path)) > 240 || !utf8.ValidString(result.Path) {
				t.Fatalf("unsafe or unbounded output filename %q", result.Path)
			}
			if attempt == 1 && result.Status != "unchanged" {
				t.Fatalf("repeat export status = %q", result.Status)
			}
		}
	}
	for path, digest := range before {
		content, err := os.ReadFile(path)
		if err != nil || sha256.Sum256(content) != digest {
			t.Fatalf("source database changed: %v", err)
		}
	}
}

// TestExportReportsPartialFailuresAndRefusesReplacement verifies missing PDFs, filename conflicts, and symlink protection.
func TestExportReportsPartialFailuresAndRefusesReplacement(t *testing.T) {
	metadata, _, _ := exportCorpus(t)
	out := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := subject.Run([]string{"--db", metadata, "--doi", "10.1000/missing", "--doi", "10.1000/unknown", "--doi", "10.1000/one", "--out", out}, &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "not available") || !strings.Contains(stderr.String(), "does not belong") {
		t.Fatalf("partial failure: exit=%d, stderr=%s", code, &stderr)
	}
	var results []subject.Result
	if err := json.Unmarshal(stdout.Bytes(), &results); err != nil || len(results) != 1 {
		t.Fatalf("partial result count = %d: %v", len(results), err)
	}
	path := results[0].Path
	foreign := []byte("existing user-owned file")
	if err := os.WriteFile(path, foreign, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := subject.Export(context.Background(), metadata, []string{"10.1000/one"}, out); err == nil {
		t.Fatal("conflicting output was accepted")
	}
	if content, err := os.ReadFile(path); err != nil || !bytes.Equal(content, foreign) {
		t.Fatal("existing output was overwritten")
	}
	linkDir := t.TempDir()
	if err := os.Symlink(path, filepath.Join(linkDir, filepath.Base(path))); err != nil {
		t.Fatal(err)
	}
	if _, err := subject.Export(context.Background(), metadata, []string{"10.1000/one"}, linkDir); err == nil {
		t.Fatal("symlink output was accepted")
	}
}

// TestExportRejectsCorruptContent verifies stored content hashes before creating any PDF file.
func TestExportRejectsCorruptContent(t *testing.T) {
	metadata, companion, data := exportCorpus(t)
	conn, err := sql.Open("sqlite", companion)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	hash := strings.Repeat("0", 64)
	if _, err := conn.Exec("INSERT INTO pdf_blobs(content_hash,byte_size,data,created_at) VALUES (?,?,?,datetime('now'))", hash, len(data), data); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec("UPDATE pdf_documents SET content_hash=? WHERE doi='10.1000/one'", hash); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if _, err := subject.Export(context.Background(), metadata, []string{"10.1000/one"}, out); err == nil || !strings.Contains(err.Error(), "content-hash") {
		t.Fatalf("corrupt content error = %v", err)
	}
	files, err := os.ReadDir(out)
	if err != nil || len(files) != 0 {
		t.Fatalf("corrupt PDF created output: %v", err)
	}
}

// TestRunUsageAndMissingDatabase verifies flag validation and existing-only opening behavior.
func TestRunUsageAndMissingDatabase(t *testing.T) {
	for _, args := range [][]string{nil, {"--unknown"}, {"--db", "unused", "--out", "unused"}} {
		var stdout, stderr bytes.Buffer
		if code := subject.Run(args, &stdout, &stderr); code != 2 {
			t.Fatalf("usage exit = %d", code)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := subject.Run([]string{"--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("help exit = %d", code)
	}
	missing := filepath.Join(t.TempDir(), "absent.db")
	if code := subject.Run([]string{"--db", missing, "--doi", "10.1000/one", "--out", t.TempDir()}, &stdout, &stderr); code != 1 {
		t.Fatalf("missing database exit = %d", code)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("missing database was created: %v", err)
	}
}
