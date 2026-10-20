// helpers_test.go contains shared test infrastructure for pdfstore tests.
// It has no build tag and is always compiled.

package testhelpers

import "analysis/tests/support/go/testenv"

import subject "analysis/backend/pdfstore"

import (
	"path/filepath"
	"testing"

	"analysis/backend/database/pdfbinding"
)

// OpenTestStore supports the package test suite's open test store setup or assertions.
func OpenTestStore(t *testing.T) *subject.Store {
	t.Helper()
	store, err := subject.Open(filepath.Join(t.TempDir(), pdfbinding.DefaultStoreFilename), testenv.Path("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}
