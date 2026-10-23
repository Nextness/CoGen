package doccheck_test

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
	for _, document := range []string{"AGENTS.md", "docs/ARCHITECTURE.md"} {
		writeTestFile(t, root, document, documentation)
	}
}

// TestDestinationPathIgnoresRemoteAndAnchorLinks verifies decoded paths and ignored remote and anchor links through the public check.
func TestDestinationPathIgnoresRemoteAndAnchorLinks(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "config/file name.something", "fixture")
	writeTestFile(t, root, "docs/guide.md", "[remote](https://example.com/file.md) [anchor](#local-heading) [encoded](<../config/file%20name.something#part>)\n")
	if failures := checkDocuments(t, root); len(failures) != 0 {
		t.Fatalf("link failures=%v", failures)
	}
}

// TestMarkdownLinkCheckAcceptsExistingFilesAndIgnoresCodeExamples verifies markdown link check accepts existing files and ignores code examples.
func TestMarkdownLinkCheckAcceptsExistingFilesAndIgnoresCodeExamples(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/target.md", "# Target\n")
	writeTestFile(t, root, "docs/guide.md", "[target](target.md#section)\n\n`func as[T error](err error)`\n```md\n[example](missing.md)\n```\n")
	if failures := checkDocuments(t, root); len(failures) != 0 {
		t.Fatalf("checkMarkdownLinks returned %v", failures)
	}
}

// TestMarkdownLinkCheckReportsMissingAndEscapingTargets verifies markdown link check reports missing and escaping targets.
func TestMarkdownLinkCheckReportsMissingAndEscapingTargets(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/guide.md", "[missing](missing.md)\n\n[outside](../../outside.md)\n")
	failures := checkDocuments(t, root)
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
	failures := checkDocuments(t, root)
	if len(failures) != 1 || !strings.Contains(failures[0], "symbolic link") {
		t.Fatalf("checkMarkdownLinks returned %v", failures)
	}
}

// TestObsoleteReferenceCheckReportsExactHistoricalNames verifies obsolete reference check reports exact historical names.
func TestObsoleteReferenceCheckReportsExactHistoricalNames(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/guide.md", "See TODO.md Phase B, docs/viewer-design.md, and workspace_store.go.\n")
	failures := checkDocuments(t, root)
	if len(failures) != 3 {
		t.Fatalf("checkObsoleteReferences returned %v", failures)
	}
}

// TestMigrationChecksAcceptMatchingFilesAndDocumentation verifies migration checks accept matching files and documentation.
func TestMigrationChecksAcceptMatchingFilesAndDocumentation(t *testing.T) {
	root := t.TempDir()
	writeMigrationRepository(t, root, 2, 1)
	failures := checkDocuments(t, root)
	if len(failures) != 0 {
		t.Fatalf("migration checks returned %v", failures)
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
	failures := checkDocuments(t, root)
	joined := strings.Join(failures, "\n")
	for _, expected := range []string{"configured migration file does not exist", "migration file is not configured", "missing current metadata migration boundary"} {
		if !strings.Contains(joined, expected) {
			t.Errorf("migration failures missing %q: %v", expected, failures)
		}
	}
}
