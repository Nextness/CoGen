// main_unit_test.go tests the something-printer CLI tool in isolation.
//go:build unit

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeConfig writes content into a temporary .something file and returns its path.
func writeConfig(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestRunJSONStyle verifies json style output.
func TestRunJSONStyle(t *testing.T) {
	path := writeConfig(t, "test.something", `x := "hello";`)
	var stdout, stderr bytes.Buffer
	code := run("something-printer", []string{"--json", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"x": "hello"`) {
		t.Errorf("unexpected JSON output: %s", stdout.String())
	}
}

// TestRunSomethingStyle verifies something style output.
func TestRunSomethingStyle(t *testing.T) {
	path := writeConfig(t, "full.something", `search_id := "example";
count := 3;`)
	var stdout, stderr bytes.Buffer
	code := run("something-printer", []string{"--something", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, `search_id = "example",`) {
		t.Errorf("missing search_id assignment: %s", out)
	}
	if !strings.Contains(out, "count = 3,") {
		t.Errorf("missing count assignment: %s", out)
	}
}

// TestRunYAMLStyle verifies yaml style output.
func TestRunYAMLStyle(t *testing.T) {
	path := writeConfig(t, "full.something", `x := "hello";
y := true;`)
	var stdout, stderr bytes.Buffer
	code := run("something-printer", []string{"--yaml", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "x: hello") {
		t.Errorf("missing x in YAML output: %s", out)
	}
	if !strings.Contains(out, "true") {
		t.Errorf("missing y value in YAML output: %s", out)
	}
}

// TestRunDefaultIsSomething verifies default style is something.
func TestRunDefaultIsSomething(t *testing.T) {
	path := writeConfig(t, "default.something", `x := "hello";`)
	var stdout, stderr bytes.Buffer
	code := run("something-printer", []string{path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `x = "hello",`) {
		t.Errorf("expected SOMETHING default output, got: %s", stdout.String())
	}
}

// TestRunConflictingStyles verifies mutually exclusive styles error.
func TestRunConflictingStyles(t *testing.T) {
	path := writeConfig(t, "conflict.something", `x := "hello";`)
	var stdout, stderr bytes.Buffer
	code := run("something-printer", []string{"--json", "--yaml", path}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code for conflicting styles")
	}
	if !strings.Contains(stderr.String(), "mutually exclusive") {
		t.Errorf("unexpected stderr: %s", stderr.String())
	}
}

// TestRunMissingFile verifies missing file reports an error.
func TestRunMissingFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run("something-printer", []string{"/nonexistent/file.something"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code for missing file")
	}
	if !strings.Contains(stderr.String(), "error:") {
		t.Errorf("unexpected stderr: %s", stderr.String())
	}
}

// TestRunNoArguments verifies no arguments prints usage and fails.
func TestRunNoArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run("something-printer", nil, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code without arguments")
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Errorf("expected usage text in stderr: %s", stderr.String())
	}
}

// TestQuoteString verifies quote handling for strings that need escaping.
func TestQuoteString(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{`hello`, `"hello"`},
		{`a"b`, `'a"b'`},
		{`a'b`, `"a'b"`},
		{`both"and'`, `"both\"and'"`},
		{`line1` + "\n" + `line2`, `"line1\nline2"`},
		{`tab\there`, `"tab\\there"`},
		{`open{close}`, `"open{{close}}"`},
		{`back\slash`, `"back\\slash"`},
	}
	for _, c := range cases {
		got := quoteString(c.input)
		if got != c.want {
			t.Errorf("quoteString(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// TestPrintSomethingNested verifies nested object and list rendering.
func TestPrintSomethingNested(t *testing.T) {
	input := map[string]any{
		"items": []any{"a", 1, true},
		"inner": map[string]any{"x": 2},
	}
	out := printSomething(input)
	if !strings.Contains(out, "inner = {") {
		t.Errorf("missing inner object: %s", out)
	}
	if !strings.Contains(out, `items = [`) {
		t.Errorf("missing items list: %s", out)
	}
}
