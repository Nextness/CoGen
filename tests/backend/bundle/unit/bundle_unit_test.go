// Package bundle_test verifies the supported schema-pair consumer in isolation.
package bundle_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"analysis/backend/bundle"
)

// chainEntry is one generated migration link used by the synthetic configs.
type chainEntry struct {
	filename   string
	previous   string
	upgrade    string
	supersedes string
}

// writeChain writes one synthetic migration configuration.
func writeChain(t *testing.T, path string, entries []chainEntry) {
	t.Helper()
	var builder strings.Builder
	builder.WriteString(`database_migration_config: setup = {
    migrations_dir: string;
}

db_migration: setup = {
    filename: string;
    description: string;
    previous: string;
    upgrade: string;
    supersedes?: []string = []string{};
}

#assert db_migration {
    #if #not #match(filename, "^V[0-9]{{5}}_.*\.sql$") {
        #error("Expected the format V00001_<something>.sql, but got {filename}");
    }
    #if #not (#match(previous, "^V[0-9]{{5}}_.*\.sql$") #or previous == "ROOT") {
        #error("Expected the format V00001_<something>.sql, but got {previous}");
    }
    #if #not (#match(upgrade, "^V[0-9]{{5}}_.*\.sql$") #or upgrade == "NONE") {
        #error("Expected the format V00001_<something>.sql, but got {upgrade}");
    }
}

database_migrations: database_migration_config = {
    migrations_dir = ".",
};
`)
	for _, entry := range entries {
		builder.WriteString(fmt.Sprintf(`
#iteration("_db_migration"): db_migration = {
    filename = %q,
    description = "Synthetic migration.",
    previous = %q,
    upgrade = %q,
`, entry.filename, entry.previous, entry.upgrade))
		if entry.supersedes != "" {
			builder.WriteString(fmt.Sprintf("    supersedes = []string { %q },\n", entry.supersedes))
		}
		builder.WriteString("};\n")
	}
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatal(err)
	}
}

// linearChain builds a complete linear chain ending at endpoint.
func linearChain(prefix string, count int, endpoint string) []chainEntry {
	entries := make([]chainEntry, count)
	for index := range entries {
		entries[index].filename = fmt.Sprintf("V%05d_%s.sql", index+1, prefix)
		if index == count-1 {
			entries[index].filename = endpoint
		}
		if index == 0 {
			entries[index].previous = "ROOT"
		} else {
			entries[index].previous = entries[index-1].filename
		}
		if index == count-1 {
			entries[index].upgrade = "NONE"
		} else {
			entries[index].upgrade = fmt.Sprintf("V%05d_%s.sql", index+2, prefix)
			if index+2 == count {
				entries[index].upgrade = endpoint
			}
		}
	}
	return entries
}

// writePair writes metadata and PDF configs and returns their paths.
func writePair(t *testing.T, metadata, pdf []chainEntry) (string, string) {
	t.Helper()
	dir := t.TempDir()
	metadataPath := filepath.Join(dir, "metadata.something")
	pdfPath := filepath.Join(dir, "pdf.something")
	writeChain(t, metadataPath, metadata)
	writeChain(t, pdfPath, pdf)
	return metadataPath, pdfPath
}

// TestVerifyPairAcceptsSupportedChain verifies a complete supported pair passes.
func TestVerifyPairAcceptsSupportedChain(t *testing.T) {
	metadataPath, pdfPath := writePair(t, linearChain("metadata", 28, "V00028_immutable_cache_responses.sql"), linearChain("pdf", 3, "V00003_pdf_document_management.sql"))
	if err := bundle.VerifyPair(metadataPath, pdfPath, bundle.SupportedPair); err != nil {
		t.Fatal(err)
	}
}

// TestVerifyPairRejectsOlderMetadata verifies an older metadata endpoint is rejected.
func TestVerifyPairRejectsOlderMetadata(t *testing.T) {
	metadataPath, pdfPath := writePair(t, linearChain("metadata", 27, "V00027_metadata.sql"), linearChain("pdf", 3, "V00003_pdf_document_management.sql"))
	err := bundle.VerifyPair(metadataPath, pdfPath, bundle.SupportedPair)
	if err == nil || !strings.Contains(err.Error(), "metadata endpoint") {
		t.Fatalf("error = %v", err)
	}
}

// TestVerifyPairRejectsOlderPDF verifies an older PDF endpoint is rejected.
func TestVerifyPairRejectsOlderPDF(t *testing.T) {
	metadataPath, pdfPath := writePair(t, linearChain("metadata", 28, "V00028_immutable_cache_responses.sql"), linearChain("pdf", 2, "V00002_normalized_inventory.sql"))
	err := bundle.VerifyPair(metadataPath, pdfPath, bundle.SupportedPair)
	if err == nil || !strings.Contains(err.Error(), "pdf endpoint") {
		t.Fatalf("error = %v", err)
	}
}

// TestVerifyPairRejectsNewerMetadata verifies a newer metadata endpoint is rejected.
func TestVerifyPairRejectsNewerMetadata(t *testing.T) {
	metadataPath, pdfPath := writePair(t, linearChain("metadata", 29, "V00029_metadata.sql"), linearChain("pdf", 3, "V00003_pdf_document_management.sql"))
	err := bundle.VerifyPair(metadataPath, pdfPath, bundle.SupportedPair)
	if err == nil || !strings.Contains(err.Error(), "metadata endpoint") {
		t.Fatalf("error = %v", err)
	}
}

// TestVerifyPairRejectsBrokenPrevious verifies a broken prior-history link is rejected.
func TestVerifyPairRejectsBrokenPrevious(t *testing.T) {
	metadata := linearChain("metadata", 28, "V00028_immutable_cache_responses.sql")
	metadata[len(metadata)-1].previous = "V00027_wrong.sql"
	metadataPath, pdfPath := writePair(t, metadata, linearChain("pdf", 3, "V00003_pdf_document_management.sql"))
	err := bundle.VerifyPair(metadataPath, pdfPath, bundle.SupportedPair)
	if err == nil || !strings.Contains(err.Error(), "metadata migration") {
		t.Fatalf("error = %v", err)
	}
}

// TestVerifyPairRejectsInvalidSupersedes verifies an invalid supersedes alias is rejected.
func TestVerifyPairRejectsInvalidSupersedes(t *testing.T) {
	metadata := linearChain("metadata", 28, "V00028_immutable_cache_responses.sql")
	metadata[len(metadata)-1].supersedes = "not-a-migration"
	metadataPath, pdfPath := writePair(t, metadata, linearChain("pdf", 3, "V00003_pdf_document_management.sql"))
	err := bundle.VerifyPair(metadataPath, pdfPath, bundle.SupportedPair)
	if err == nil || !strings.Contains(err.Error(), "supersedes") {
		t.Fatalf("error = %v", err)
	}
}

// TestVerifyPairAcceptsSupersedesAlias verifies a valid supersedes alias passes.
func TestVerifyPairAcceptsSupersedesAlias(t *testing.T) {
	metadata := linearChain("metadata", 28, "V00028_immutable_cache_responses.sql")
	metadata[len(metadata)-1].supersedes = "V00027_old_name.sql"
	metadataPath, pdfPath := writePair(t, metadata, linearChain("pdf", 3, "V00003_pdf_document_management.sql"))
	if err := bundle.VerifyPair(metadataPath, pdfPath, bundle.SupportedPair); err != nil {
		t.Fatal(err)
	}
}

// TestVerifyPairRejectsMismatchedSupportedPair verifies a code/config mismatch is rejected.
func TestVerifyPairRejectsMismatchedSupportedPair(t *testing.T) {
	metadataPath, pdfPath := writePair(t, linearChain("metadata", 28, "V00028_immutable_cache_responses.sql"), linearChain("pdf", 3, "V00003_pdf_document_management.sql"))
	err := bundle.VerifyPair(metadataPath, pdfPath, bundle.Pair{
		Metadata: "V00027_metadata.sql",
		PDF:      "V00003_pdf.sql",
	})
	if err == nil || !strings.Contains(err.Error(), "metadata endpoint") {
		t.Fatalf("error = %v", err)
	}
}
