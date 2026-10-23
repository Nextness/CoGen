// Package testselect_test verifies the supported selector against actual standard Go runner execution.
package testselect_test

import (
	"analysis/tooling/go/testselect"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunFocusedGoRejectsZeroMatchesAndPreservesFailure verifies actual execution rather than discovery alone.
func TestRunFocusedGoRejectsZeroMatchesAndPreservesFailure(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module selectionfixture\n\ngo 1.25.0\n",
		"tests/backend/example/unit/example_test.go": `package example_test
import "testing"
func TestSelected(t *testing.T) {}
func TestFailure(t *testing.T) { t.Fatal("expected fixture failure") }
`,
	}
	for name, contents := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(root)
	t.Setenv("GOWORK", "off")
	t.Setenv("GOTOOLCHAIN", "local")
	t.Setenv("CGO_ENABLED", "0")
	// There is no npm workspace, browser harness, or Makefile in this fixture.
	// A focused backend request must need only Go.
	for _, test := range []struct {
		expression string
		code       int
		diagnostic string
	}{
		{"^TestSelected$", 0, "PASS"},
		{"^TestMissing$", 1, "matched no tests"},
		{"^TestFailure$", 1, "expected fixture failure"},
	} {
		t.Run(test.expression, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := testselect.Run([]string{"--domain", "backend", "--runner", "go", "--test", test.expression}, &stdout, &stderr)
			if code != test.code || !strings.Contains(stdout.String()+stderr.String(), test.diagnostic) {
				t.Fatalf("code=%d stdout=%s stderr=%s", code, &stdout, &stderr)
			}
		})
	}
	for _, name := range []string{"./tests/backend", "./tests/backend/example"} {
		var stdout, stderr bytes.Buffer
		code := testselect.Run([]string{"--runner", "go", "--package", name, "--test", "^TestSelected$"}, &stdout, &stderr)
		if code != 2 || !strings.Contains(stderr.String(), "no runnable tests") {
			t.Fatalf("PACKAGE=%s code=%d stdout=%s stderr=%s", name, code, &stdout, &stderr)
		}
	}
}
