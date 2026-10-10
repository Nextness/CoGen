// binding_unit_test.go tests PDF binding path validation with no database required.
//go:build unit

package pdfbinding

import (
	"os"
	"path/filepath"
	"testing"
)

// TestValidateRelativeStorePathAcceptsValidPath verifies validate relative store path accepts valid path.
func TestValidateRelativeStorePathAcceptsValidPath(t *testing.T) {
	got, err := validateRelativeStorePath("data/corpus.pdf.db")
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
		if _, err := validateRelativeStorePath(path); err == nil {
			t.Fatalf("unsafe relative path %q was accepted", path)
		}
	}
}

// TestValidateRelativeStorePathRejectsAbsolutePath verifies validate relative store path rejects absolute path.
func TestValidateRelativeStorePathRejectsAbsolutePath(t *testing.T) {
	if _, err := validateRelativeStorePath("/absolute/path.pdf.db"); err == nil {
		t.Fatal("absolute path was accepted")
	}
}

// TestStorePathValidationRejectsUnsafePaths verifies store path validation rejects unsafe paths.
func TestStorePathValidationRejectsUnsafePaths(t *testing.T) {
	metadataPath := filepath.Join(t.TempDir(), "corpus.metadata.db")
	if _, err := resolveStorePath("", DefaultStoreFilename); err == nil {
		t.Fatal("empty metadata path was accepted")
	}
	for _, relativePath := range []string{"../outside.pdf.db", metadataPath} {
		if _, err := resolveStorePath(metadataPath, relativePath); err == nil {
			t.Fatalf("unsafe PDF store path %q was accepted", relativePath)
		}
	}
	if _, err := resolveStorePath(metadataPath, filepath.Base(metadataPath)); err == nil {
		t.Fatal("metadata database was accepted as its own PDF store")
	}
}

// TestResolveStorePathResolvesRelativePath verifies resolve store path resolves relative path.
func TestResolveStorePathResolvesRelativePath(t *testing.T) {
	metadataDir := t.TempDir()
	metadataPath := filepath.Join(metadataDir, "corpus.metadata.db")
	got, err := resolveStorePath(metadataPath, "corpus.pdf.db")
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
	if _, err := resolveStorePath(metadataPath, "same.db"); err == nil {
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
	if _, err := resolveStorePath(filepath.Join(metadataDir, "corpus.metadata.db"), "escape/corpus.pdf.db"); err == nil {
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
	path, err := resolveStorePath(filepath.Join(metadataDir, "corpus.metadata.db"), "current/corpus.pdf.db")
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(targetDir, "corpus.pdf.db") {
		t.Fatalf("resolved in-bundle symlink path = %q", path)
	}
}
