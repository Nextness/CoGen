// helpers_test.go contains shared test infrastructure for pdfstore tests.
// It has no build tag and is always compiled.

package pdfstore

import (
	"path/filepath"
	"testing"

	"analysis/database/pdfbinding"
)

// openTestStore supports the package test suite's open test store setup or assertions.
func openTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), pdfbinding.DefaultStoreFilename), filepath.Join("..", "..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}
