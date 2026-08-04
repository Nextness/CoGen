//go:build unit

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTestFile creates one documentation-checker fixture file.
func writeTestFile(t *testing.T, root, relativePath, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeMigrationRepository creates matching metadata, PDF, and documentation fixtures.
func writeMigrationRepository(t *testing.T, root string, metadataLast, pdfLast int) {
	t.Helper()
	writeTestFile(t, root, "config/database.something", "corpus_metadata_config = \"database.corpus.metadata.something\",\ncorpus_pdf_config = \"database.corpus.pdf.something\",\n")
	chains := []struct {
		label     string
		filename  string
		directory string
		last      int
	}{
		{label: "metadata", filename: "database.corpus.metadata.something", directory: "corpus.metadata", last: metadataLast},
		{label: "pdf", filename: "database.corpus.pdf.something", directory: "corpus.pdf", last: pdfLast},
	}
	for _, chain := range chains {
		var declarations strings.Builder
		fmt.Fprintf(&declarations, "migrations_dir = \"../migrations/%s\",\n", chain.directory)
		for version := 1; version <= chain.last; version++ {
			migration := fmt.Sprintf("V%05d_%s_%d.sql", version, chain.label, version)
			fmt.Fprintf(&declarations, "filename = \"%s\",\n", migration)
			writeTestFile(t, root, filepath.ToSlash(filepath.Join("migrations", chain.directory, migration)), "-- ==UP==\nSELECT 1;\n")
		}
		writeTestFile(t, root, filepath.ToSlash(filepath.Join("config", chain.filename)), declarations.String())
	}
	documentation := fmt.Sprintf("Metadata V00001-V%05d; PDF V00001-V%05d.\n", metadataLast, pdfLast)
	for _, document := range migrationDocs {
		writeTestFile(t, root, document, documentation)
	}
}

// TestDestinationPathIgnoresRemoteAndAnchorLinks verifies destination path ignores remote and anchor links.
func TestDestinationPathIgnoresRemoteAndAnchorLinks(t *testing.T) {
	if _, local := destinationPath("https://example.com/file.md"); local {
		t.Fatal("remote link was treated as local")
	}
	if _, local := destinationPath("#local-heading"); local {
		t.Fatal("anchor link was treated as local")
	}
	if path, local := destinationPath("<../config/file%20name.something#part>"); !local || path != "../config/file name.something" {
		t.Fatalf("destinationPath returned %q, %t", path, local)
	}
}

// TestMarkdownLinkCheckAcceptsExistingFilesAndIgnoresCodeExamples verifies markdown link check accepts existing files and ignores code examples.
func TestMarkdownLinkCheckAcceptsExistingFilesAndIgnoresCodeExamples(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/target.md", "# Target\n")
	writeTestFile(t, root, "docs/guide.md", "[target](target.md#section)\n`func as[T error](err error)`\n```md\n[example](missing.md)\n```\n")
	if failures := checkMarkdownLinks(root, []string{filepath.FromSlash("docs/guide.md")}); len(failures) != 0 {
		t.Fatalf("checkMarkdownLinks returned %v", failures)
	}
}

// TestMarkdownLinkCheckReportsMissingAndEscapingTargets verifies markdown link check reports missing and escaping targets.
func TestMarkdownLinkCheckReportsMissingAndEscapingTargets(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/guide.md", "[missing](missing.md)\n[outside](../../outside.md)\n")
	failures := checkMarkdownLinks(root, []string{filepath.FromSlash("docs/guide.md")})
	if len(failures) != 2 || !strings.Contains(failures[0], "does not exist") || !strings.Contains(failures[1], "escapes the repository") {
		t.Fatalf("checkMarkdownLinks returned %v", failures)
	}
}

// TestMarkdownLinkCheckReportsSymbolicLinkEscape verifies markdown link check reports symbolic link escape.
func TestMarkdownLinkCheckReportsSymbolicLinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeTestFile(t, outside, "target.md", "# Outside\n")
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "target.md"), filepath.Join(root, "docs", "target.md")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, root, "docs/guide.md", "[outside](target.md)\n")
	failures := checkMarkdownLinks(root, []string{filepath.FromSlash("docs/guide.md")})
	if len(failures) != 1 || !strings.Contains(failures[0], "symbolic link") {
		t.Fatalf("checkMarkdownLinks returned %v", failures)
	}
}

// TestObsoleteReferenceCheckReportsExactHistoricalNames verifies obsolete reference check reports exact historical names.
func TestObsoleteReferenceCheckReportsExactHistoricalNames(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/guide.md", "See TODO.md Phase B, docs/viewer-design.md, and workspace_store.go.\n")
	failures := checkObsoleteReferences(root, []string{filepath.FromSlash("docs/guide.md")})
	if len(failures) != 3 {
		t.Fatalf("checkObsoleteReferences returned %v", failures)
	}
}

// TestMigrationChecksAcceptMatchingFilesAndDocumentation verifies migration checks accept matching files and documentation.
func TestMigrationChecksAcceptMatchingFilesAndDocumentation(t *testing.T) {
	root := t.TempDir()
	writeMigrationRepository(t, root, 2, 1)
	failures, boundaries := checkMigrationChains(root)
	failures = append(failures, checkDocumentedMigrationBoundaries(root, boundaries)...)
	if len(failures) != 0 {
		t.Fatalf("migration checks returned %v", failures)
	}
	if boundaries["metadata"] != (migrationBoundary{first: 1, last: 2}) || boundaries["PDF"] != (migrationBoundary{first: 1, last: 1}) {
		t.Fatalf("migration boundaries = %v", boundaries)
	}
}

// TestMigrationChecksReportMissingUnconfiguredAndStaleDocumentation verifies migration checks report missing unconfigured and stale documentation.
func TestMigrationChecksReportMissingUnconfiguredAndStaleDocumentation(t *testing.T) {
	root := t.TempDir()
	writeMigrationRepository(t, root, 2, 1)
	if err := os.Remove(filepath.Join(root, "migrations", "corpus.metadata", "V00002_metadata_2.sql")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, root, "migrations/corpus.metadata/V00003_extra.sql", "-- ==UP==\nSELECT 1;\n")
	writeTestFile(t, root, "AGENTS.md", "Metadata V00001-V00001; PDF V00001-V00001.\n")
	failures, boundaries := checkMigrationChains(root)
	failures = append(failures, checkDocumentedMigrationBoundaries(root, boundaries)...)
	joined := strings.Join(failures, "\n")
	for _, expected := range []string{"configured migration file does not exist", "migration file is not configured", "missing current metadata migration boundary"} {
		if !strings.Contains(joined, expected) {
			t.Errorf("migration failures missing %q: %v", expected, failures)
		}
	}
}
