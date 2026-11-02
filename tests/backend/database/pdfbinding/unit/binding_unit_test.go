// binding_unit_test.go tests PDF binding path validation with no database required.

package pdfbinding_test

import subject "analysis/backend/database/pdfbinding"
import "analysis/tests/support/go/sqlprobe"
import "database/sql/driver"
import "context"

import (
	"os"
	"path/filepath"
	"testing"
)

// TestValidateRelativeStorePathAcceptsValidPath verifies validate relative store path accepts valid path.
func TestValidateRelativeStorePathAcceptsValidPath(t *testing.T) {
	got, err := validateBinding(t, "data/corpus.pdf.db")
	if err != nil {
		t.Fatalf("valid relative path rejected: %v", err)
	}
	if got != filepath.Clean("data/corpus.pdf.db") {
		t.Fatalf("validateRelativeStorePath returned %q, want %q", got, filepath.Clean("data/corpus.pdf.db"))
	}
}

// TestValidateRelativeStorePathRejectsUnsafePaths verifies validate relative store path rejects unsafe paths.
func TestValidateRelativeStorePathRejectsUnsafePaths(t *testing.T) {
	for _, path := range []string{"", ".", "..", "../outside.pdf.db", "../../etc/passwd"} {
		if _, err := validateBinding(t, path); err == nil {
			t.Fatalf("unsafe relative path %q was accepted", path)
		}
	}
}

// TestValidateRelativeStorePathRejectsAbsolutePath verifies validate relative store path rejects absolute path.
func TestValidateRelativeStorePathRejectsAbsolutePath(t *testing.T) {
	if _, err := validateBinding(t, "/absolute/path.pdf.db"); err == nil {
		t.Fatal("absolute path was accepted")
	}
}

// TestStorePathValidationRejectsUnsafePaths verifies store path validation rejects unsafe paths.
func TestStorePathValidationRejectsUnsafePaths(t *testing.T) {
	metadataPath := filepath.Join(t.TempDir(), "corpus.metadata.db")
	if _, err := resolveBinding(t, "", subject.DefaultStoreFilename); err == nil {
		t.Fatal("empty metadata path was accepted")
	}
	for _, relativePath := range []string{"../outside.pdf.db", metadataPath} {
		if _, err := resolveBinding(t, metadataPath, relativePath); err == nil {
			t.Fatalf("unsafe PDF store path %q was accepted", relativePath)
		}
	}
	if _, err := resolveBinding(t, metadataPath, filepath.Base(metadataPath)); err == nil {
		t.Fatal("metadata database was accepted as its own PDF store")
	}
}

// TestResolveStorePathResolvesRelativePath verifies resolve store path resolves relative path.
func TestResolveStorePathResolvesRelativePath(t *testing.T) {
	metadataDir := t.TempDir()
	metadataPath := filepath.Join(metadataDir, "corpus.metadata.db")
	got, err := resolveBinding(t, metadataPath, "corpus.pdf.db")
	if err != nil {
		t.Fatalf("resolveStorePath returned error: %v", err)
	}
	want := filepath.Join(metadataDir, "corpus.pdf.db")
	if got != want {
		t.Fatalf("resolveStorePath(%q, %q) = %q, want %q", metadataPath, "corpus.pdf.db", got, want)
	}
}

// TestResolveStorePathRejectsMetadataPathEqualToStorePath verifies resolve store path rejects metadata path equal to store path.
func TestResolveStorePathRejectsMetadataPathEqualToStorePath(t *testing.T) {
	metadataPath := filepath.Join(t.TempDir(), "same.db")
	if _, err := resolveBinding(t, metadataPath, "same.db"); err == nil {
		t.Fatal("metadata path equal to resolved store path was accepted")
	}
}

// TestResolveStorePathRejectsSymlinkEscape verifies existing path components cannot escape the bundle.
func TestResolveStorePathRejectsSymlinkEscape(t *testing.T) {
	metadataDir := t.TempDir()
	outsideDir := t.TempDir()
	if err := os.Symlink(outsideDir, filepath.Join(metadataDir, "escape")); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	if _, err := resolveBinding(t, filepath.Join(metadataDir, "corpus.metadata.db"), "escape/corpus.pdf.db"); err == nil {
		t.Fatal("symlink escape was accepted")
	}
}

// TestResolveStorePathAllowsInBundleSymlink verifies in-bundle targets remain valid.
func TestResolveStorePathAllowsInBundleSymlink(t *testing.T) {
	metadataDir := t.TempDir()
	targetDir := filepath.Join(metadataDir, "stores")
	if err := os.Mkdir(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(targetDir, filepath.Join(metadataDir, "current")); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	path, err := resolveBinding(t, filepath.Join(metadataDir, "corpus.metadata.db"), "current/corpus.pdf.db")
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(targetDir, "corpus.pdf.db") {
		t.Fatalf("resolved in-bundle symlink path = %q", path)
	}
}

// validateBinding observes relative path cleaning through the public binding insertion.
func validateBinding(t *testing.T, path string) (string, error) {
	t.Helper()
	var clean string
	d := &sqlprobe.Driver{}
	d.Query = func(sqlprobe.Call) (driver.Rows, error) { return &sqlprobe.Rows{Names: []string{"relative_path"}}, nil }
	d.Exec = func(call sqlprobe.Call) (driver.Result, error) {
		clean = call.Args[0].Value.(string)
		return sqlprobe.Result{Affected: 1}, nil
	}
	db := d.OpenDB()
	defer db.Close()
	err := subject.New(db).EnsureBinding(context.Background(), path)
	return clean, err
}

// resolveBinding supplies a persisted binding to the public resolver without exposing source internals.
func resolveBinding(t *testing.T, metadata, relative string) (string, error) {
	t.Helper()
	d := &sqlprobe.Driver{Query: func(sqlprobe.Call) (driver.Rows, error) {
		return &sqlprobe.Rows{Names: []string{"relative_path"}, Values: [][]driver.Value{{relative}}}, nil
	}}
	db := d.OpenDB()
	defer db.Close()
	return subject.New(db).ResolveBinding(context.Background(), metadata)
}

// TestResolveBoundPath verifies an existing binding is resolved without creating one.
func TestResolveBoundPath(t *testing.T) {
	metadataDir := t.TempDir()
	metadataPath := filepath.Join(metadataDir, "corpus.metadata.db")
	path, found, err := resolveBoundPath(t, metadataPath, "corpus.pdf.db")
	if err != nil || !found || path != filepath.Join(metadataDir, "corpus.pdf.db") {
		t.Fatalf("bound path = %q found=%v err=%v", path, found, err)
	}
	path, found, err = resolveBoundPath(t, metadataPath, "")
	if err != nil || found || path != "" {
		t.Fatalf("missing binding = %q found=%v err=%v", path, found, err)
	}
	if _, found, err := resolveBoundPath(t, metadataPath, "../escape.pdf.db"); err == nil || !found {
		t.Fatalf("unsafe binding found=%v err=%v", found, err)
	}
}

// resolveBoundPath supplies a controlled binding result to the public existing-only resolver.
func resolveBoundPath(t *testing.T, metadata, relative string) (string, bool, error) {
	t.Helper()
	rows := &sqlprobe.Rows{Names: []string{"relative_path"}}
	if relative != "" {
		rows.Values = [][]driver.Value{{relative}}
	}
	d := &sqlprobe.Driver{Query: func(sqlprobe.Call) (driver.Rows, error) { return rows, nil }}
	db := d.OpenDB()
	defer db.Close()
	return subject.New(db).ResolveBoundPath(context.Background(), metadata)
}
