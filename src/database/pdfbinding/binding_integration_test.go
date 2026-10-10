// binding_integration_test.go tests the metadata PDF binding against real
// temporary SQLite databases created by the production migration runner.
//go:build integration

package pdfbinding

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"analysis/database"
)

// openTestStore opens a migrated metadata database and returns a binding store.
func openTestStore(t *testing.T) (*Store, *database.Database, string) {
	t.Helper()
	metadataPath := filepath.Join(t.TempDir(), "corpus.metadata.db")
	metadata, err := database.Open(metadataPath, filepath.Join("..", "..", "..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = metadata.Close() })
	return New(metadata.DB), metadata, metadataPath
}

// TestBindingReportsMissingRow verifies a fresh metadata database has no binding.
func TestBindingReportsMissingRow(t *testing.T) {
	ctx := context.Background()
	store, _, _ := openTestStore(t)
	relativePath, found, err := store.Binding(ctx)
	if err != nil || found || relativePath != "" {
		t.Fatalf("fresh binding = %q found=%v err=%v, want missing", relativePath, found, err)
	}
}

// TestResolveBindingUsesDefaultAndPreservesExistingBinding verifies resolve binding uses default and preserves existing binding.
func TestResolveBindingUsesDefaultAndPreservesExistingBinding(t *testing.T) {
	ctx := context.Background()
	store, _, metadataPath := openTestStore(t)
	path, err := store.ResolveBinding(ctx, metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(filepath.Dir(metadataPath), DefaultStoreFilename) {
		t.Fatalf("default PDF store path = %q", path)
	}
	relativePath, found, err := store.Binding(ctx)
	if err != nil || !found || relativePath != DefaultStoreFilename {
		t.Fatalf("resolved binding = %q found=%v err=%v", relativePath, found, err)
	}
	if err := store.EnsureBinding(ctx, "other.pdf.db"); err == nil || !strings.Contains(err.Error(), "already bound") {
		t.Fatalf("conflicting binding error = %v", err)
	}
}

// TestEnsureBindingRejectsUnsafePaths verifies absolute and escaping paths are rejected before any row is written.
func TestEnsureBindingRejectsUnsafePaths(t *testing.T) {
	ctx := context.Background()
	store, metadata, _ := openTestStore(t)
	for _, relativePath := range []string{"", ".", "..", "../outside.pdf.db", "/absolute/path.pdf.db"} {
		if err := store.EnsureBinding(ctx, relativePath); err == nil {
			t.Fatalf("unsafe binding path %q was accepted", relativePath)
		}
	}
	var count int
	if err := metadata.DB.QueryRow("SELECT COUNT(*) FROM pdf_store_binding").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("rejected bindings wrote %d rows", count)
	}
}

// TestEnsureBindingPreservesExistingPath verifies an identical re-bind is a no-op.
func TestEnsureBindingPreservesExistingPath(t *testing.T) {
	ctx := context.Background()
	store, metadata, _ := openTestStore(t)
	if err := store.EnsureBinding(ctx, "corpus.pdf.db"); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureBinding(ctx, "corpus.pdf.db"); err != nil {
		t.Fatalf("identical re-bind error = %v", err)
	}
	var count int
	if err := metadata.DB.QueryRow("SELECT COUNT(*) FROM pdf_store_binding").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("binding rows = %d, want one", count)
	}
}

// TestBindingOperationsSurfaceConnectionFailures verifies binding reads and writes propagate connection errors.
func TestBindingOperationsSurfaceConnectionFailures(t *testing.T) {
	ctx := context.Background()
	store, metadata, metadataPath := openTestStore(t)
	if err := metadata.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Binding(ctx); err == nil {
		t.Fatal("Binding on a closed connection returned no error")
	}
	if err := store.EnsureBinding(ctx, "corpus.pdf.db"); err == nil {
		t.Fatal("EnsureBinding on a closed connection returned no error")
	}
	if _, err := store.ResolveBinding(ctx, metadataPath); err == nil {
		t.Fatal("ResolveBinding on a closed connection returned no error")
	}
}
