// bibtex_functional_test.go tests BibTeX file loading through the full
// parser pipeline with real temporary files.

package bibtex_test

import testhelpers "analysis/tests/support/go/backend/bibtex"

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadFile verifies load file.
func TestLoadFile(t *testing.T) {
	p := testhelpers.Parser()

	t.Run("loads_file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.bib")
		if err := os.WriteFile(path, []byte(testhelpers.SimpleBib), 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		data, err := p.LoadFile(path)
		if err != nil {
			t.Fatalf("LoadFile failed: %v", err)
		}
		if len(data) == 0 {
			t.Fatal("expected non-empty data")
		}
	})

	t.Run("missing_file", func(t *testing.T) {
		_, err := p.LoadFile("/nonexistent/path.bib")
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})
}
