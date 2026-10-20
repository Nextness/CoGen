// main_unit_test.go provides unit tests for the command entry point that do
// not require a built binary or a migrated database.
package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestFrontendAssets verifies frontend asset directory validation.
func TestFrontendAssets(t *testing.T) {
	assetDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(assetDir, "index.html"), []byte("filesystem asset"), 0644); err != nil {
		t.Fatal(err)
	}
	assets, err := frontendAssets(assetDir)
	if err != nil {
		t.Fatalf("frontendAssets filesystem directory: %v", err)
	}
	contents, err := fs.ReadFile(assets, "index.html")
	if err != nil || string(contents) != "filesystem asset" {
		t.Fatalf("filesystem assets index = %q, %v", contents, err)
	}

	if _, err := frontendAssets(filepath.Join(assetDir, "missing")); err == nil {
		t.Fatal("frontendAssets accepted missing directory")
	}
	filePath := filepath.Join(assetDir, "not-a-directory")
	if err := os.WriteFile(filePath, []byte("file"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := frontendAssets(filePath); err == nil {
		t.Fatal("frontendAssets accepted regular file")
	}
}

// TestVersion verifies the version command output format and current value.
func TestVersion(t *testing.T) {
	got := version()
	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(got) {
		t.Fatalf("version() = %q, want MAJOR.MINOR.PATCH", got)
	}
	if !strings.HasPrefix(got, "1.0.0") {
		t.Errorf("version() = %q, want current version 1.0.0", got)
	}
}

// TestValidateLoopbackAddress verifies writable serving rejects names, wildcards, and remote IPs.
func TestValidateLoopbackAddress(t *testing.T) {
	for _, address := range []string{"127.0.0.1:8080", "[::1]:0"} {
		if err := validateLoopbackAddress(address); err != nil {
			t.Errorf("validate %q: %v", address, err)
		}
	}
	for _, address := range []string{"localhost:8080", "0.0.0.0:8080", "[::]:8080", "192.0.2.1:8080", "missing-port"} {
		if err := validateLoopbackAddress(address); err == nil {
			t.Errorf("expected %q to be rejected", address)
		}
	}
}
