// main_functional_test.go tests raw SQL ownership enforcement against the real
// repository tree.

package rawsqlcheck_test

import "analysis/tests/support/go/testenv"

import subject "analysis/tooling/go/rawsqlcheck"

import (
	"bytes"

	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRepositoryOwnershipCheckPasses verifies the committed ownership table classifies every maintained raw SQL call.
func TestRepositoryOwnershipCheckPasses(t *testing.T) {
	root := testenv.Root()
	var stdout, stderr bytes.Buffer
	if status := subject.Run([]string{"check", "--root", root}, &stdout, &stderr); status != 0 {
		t.Fatalf("status = %d, want 0; stderr=%s", status, stderr.String())
	}
	if !strings.Contains(stdout.String(), "raw SQL ownership check passed") {
		t.Fatalf("stdout = %q, want a passing check summary", stdout.String())
	}
}

// TestRepositoryOwnershipReportHasNoUnclassifiedCalls verifies the report covers every category without unclassified calls.
func TestRepositoryOwnershipReportHasNoUnclassifiedCalls(t *testing.T) {
	root := testenv.Root()
	var stdout, stderr bytes.Buffer
	if status := subject.Run([]string{"report", "--root", root}, &stdout, &stderr); status != 0 {
		t.Fatalf("status = %d, want 0; stderr=%s", status, stderr.String())
	}
	output := stdout.String()
	if strings.Contains(output, "Unclassified raw SQL calls") {
		t.Fatalf("report contains unclassified calls:\n%s", output)
	}
	for _, want := range []string{"Raw SQL ownership report", "platform", "plumbing", "introspection", "export", "outbox", "dynamic", "test", "generated"} {
		if !strings.Contains(output, want) {
			t.Fatalf("report does not mention %q:\n%s", want, output)
		}
	}
}

// TestCommandValidationAndDefaultDispatch verifies the public command preserves failures and both default check forms.
func TestCommandValidationAndDefaultDispatch(t *testing.T) {
	root := testenv.Root()
	t.Chdir(root)
	for _, args := range [][]string{nil, {"--root", root}} {
		var stdout, stderr bytes.Buffer
		if code := subject.Run(args, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "check passed") {
			t.Fatalf("args=%q code=%d stdout=%s stderr=%s", args, code, &stdout, &stderr)
		}
	}
	malformed := t.TempDir()
	if err := os.MkdirAll(filepath.Join(malformed, "backend"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(malformed, "backend/broken.go"), []byte("package broken\nfunc bad(\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"report", "--bad-flag"}, {"report", "--root", malformed}, {"check", "positional"}} {
		var stdout, stderr bytes.Buffer
		if code := subject.Run(args, &stdout, &stderr); code != 2 || stderr.Len() == 0 {
			t.Fatalf("args=%q code=%d stderr=%s", args, code, &stderr)
		}
	}
}
