// api_functional_test.go contains functional tests for api.go that exercise
// LoadSomethingFile and LoadSomethingBytes with temporary files and the
// full SOMETHING pipeline.
//go:build functional

package something

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParseIncludeValue_Functional verifies parse include value functional.
func TestParseIncludeValue_Functional(t *testing.T) {
	// #include(...) as an expression value in a variable declaration
	// The included file's vars are returned as the namespace value
	dir := t.TempDir()
	incPath := filepath.Join(dir, "inc.something")
	os.WriteFile(incPath, []byte(`x := "from-inc";`), 0644)
	mainPath := filepath.Join(dir, "main.something")
	os.WriteFile(mainPath, []byte(`ns := #include("inc.something");`), 0644)
	result, err := LoadSomethingFile(mainPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// ns contains the included file's vars
	ns, ok := result["ns"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'ns' to be a map, got %T", result["ns"])
	}
	if ns["x"] != "from-inc" {
		t.Errorf("expected 'from-inc', got %v", ns["x"])
	}
}

// TestLoadSomethingFileError_Functional verifies load something file error functional.
func TestLoadSomethingFileError_Functional(t *testing.T) {
	// Test LoadSomethingFile with a file that exists but has parse errors
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.something")
	os.WriteFile(badPath, []byte(`x := `), 0644)
	_, err := LoadSomethingFile(badPath)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Unexpected value token") {
		t.Errorf("expected parse error, got %v", err)
	}
}

// TestEvalIncludeFromFile_Functional verifies eval include from file functional.
func TestEvalIncludeFromFile_Functional(t *testing.T) {
	dir := t.TempDir()
	incPath := filepath.Join(dir, "included.something")
	os.WriteFile(incPath, []byte(`x := "from-include";`), 0644)
	mainPath := filepath.Join(dir, "main.something")
	os.WriteFile(mainPath, []byte(`#include("included.something");`), 0644)
	result, err := LoadSomethingFile(mainPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["x"] != "from-include" {
		t.Errorf("expected 'from-include', got %v", result["x"])
	}
}

// TestEvalIncludeDedup_Functional verifies eval include dedup functional.
func TestEvalIncludeDedup_Functional(t *testing.T) {
	dir := t.TempDir()
	incPath := filepath.Join(dir, "included.something")
	os.WriteFile(incPath, []byte(`x := "once";`), 0644)
	mainPath := filepath.Join(dir, "main.something")
	os.WriteFile(mainPath, []byte(`#include("included.something"); #include("included.something");`), 0644)
	result, err := LoadSomethingFile(mainPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["x"] != "once" {
		t.Errorf("expected 'once', got %v", result["x"])
	}
}

// TestEvalIncludeNotFound_Functional verifies eval include not found functional.
func TestEvalIncludeNotFound_Functional(t *testing.T) {
	_, err := LoadSomethingFile("/nonexistent/file.something")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Could not read file") {
		t.Errorf("expected 'Could not read file', got %v", err)
	}
}

// TestEvalIncludeInScope_Functional verifies eval include in scope functional.
func TestEvalIncludeInScope_Functional(t *testing.T) {
	dir := t.TempDir()
	incPath := filepath.Join(dir, "inc_scope.something")
	os.WriteFile(incPath, []byte(`x := "in-scope";`), 0644)
	mainPath := filepath.Join(dir, "main.something")
	// Include inside scope, then use the included var
	os.WriteFile(mainPath, []byte(`#include("inc_scope.something");
s: scope = { y := x; }`), 0644)
	result, err := LoadSomethingFile(mainPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := result["s"].(map[string]any)
	if !ok {
		t.Fatalf("expected 's' to be a map, got %T", result["s"])
	}
	if s["y"] != "in-scope" {
		t.Errorf("expected 'in-scope', got %v", s["y"])
	}
}

// TestEvalScopeBodyIncludeNamespace_Functional verifies eval scope body include namespace functional.
func TestEvalScopeBodyIncludeNamespace_Functional(t *testing.T) {
	// Include with namespace in a scope body
	dir := t.TempDir()
	incPath := filepath.Join(dir, "ns_inc.something")
	os.WriteFile(incPath, []byte(`x := 42;`), 0644)
	mainPath := filepath.Join(dir, "main.something")
	os.WriteFile(mainPath, []byte(`ns: namespace = #include("ns_inc.something"); a := ns.x;`), 0644)
	result, err := LoadSomethingFile(mainPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["a"] != 42 {
		t.Errorf("expected 42, got %v", result["a"])
	}
}
