// main_unit_test.go tests the something-json CLI tool in isolation.
//go:build unit

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"analysis/something"
)

// TestLoadSimpleConfig verifies load simple config.
func TestLoadSimpleConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.something")
	content := `x := "hello";`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := something.LoadSomethingFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["x"] != "hello" {
		t.Errorf("x = %v, want hello", result["x"])
	}
}

// TestLoadConfigWithValues verifies load config with values.
func TestLoadConfigWithValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "full.something")
	content := `search_id := "example";
search_revision := "v1";`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := something.LoadSomethingFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if result["search_id"] != "example" {
		t.Errorf("search_id = %v, want example", result["search_id"])
	}
	if result["search_revision"] != "v1" {
		t.Errorf("search_revision = %v, want v1", result["search_revision"])
	}
}

// TestLoadMissingFile verifies load missing file.
func TestLoadMissingFile(t *testing.T) {
	_, err := something.LoadSomethingFile("/nonexistent/file.something")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "ERROR:") && !strings.Contains(err.Error(), "no such file") {
		t.Fatalf("unexpected error: %v", err)
	}
}
