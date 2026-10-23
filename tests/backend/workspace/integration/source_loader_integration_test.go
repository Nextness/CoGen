// support_functional_test.go verifies source-loader failures through the workspace boundary.

package workspace_test

import subject "analysis/backend/workspace"

import (
	"analysis/backend/manifest"
	"analysis/tests/support/go/testenv"
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

	t.Chdir(testenv.Root())
	run := &subject.Run{Manifest: &manifest.ResolvedManifest{FormatVersion: 2, SearchID: "malformed-bib", SearchRevision: "r1", ReusePolicy: "reuse_completed", Sources: []manifest.SourceManifest{{Name: "test-source", FileType: "bib", ExpectedFile: path, ExpectedResultCount: 1}}}}
	err := subject.RunPipeline(filepath.Join(t.TempDir(), "metadata.db"), []byte("// malformed source fixture"), run, false)
	if err == nil {
		t.Fatal("expected malformed BibTeX error")
	}
	if !strings.Contains(err.Error(), "unterminated braced value") {
		t.Fatalf("loadBibEntries error = %q, want lexical error", err)
	}
}
