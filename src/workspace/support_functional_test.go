// support_functional_test.go verifies source-loader failures through the workspace boundary.
//go:build functional

package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadBibEntriesRejectsMalformedInput verifies malformed BibTeX cannot enter workspace ingestion.
func TestLoadBibEntriesRejectsMalformedInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "malformed.bib")
	data := []byte(`@article{k, title = {unterminated`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write malformed BibTeX: %v", err)
	}

	_, err := loadBibEntries(path, "test-source")
	if err == nil {
		t.Fatal("expected malformed BibTeX error")
	}
	if !strings.Contains(err.Error(), "unterminated braced value") {
		t.Fatalf("loadBibEntries error = %q, want lexical error", err)
	}
}
