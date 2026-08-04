// bibtex_functional_test.go tests BibTeX file loading through the full
// parser pipeline with real temporary files.
//go:build functional

package bibtex

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadFile verifies load file.
func TestLoadFile(t *testing.T) {
	p := parser()

	t.Run("loads_file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.bib")
		if err := os.WriteFile(path, []byte(simpleBib), 0644); err != nil {
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
