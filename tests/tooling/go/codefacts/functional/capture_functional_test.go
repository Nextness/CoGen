package codefacts_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"analysis/tooling/go/codefacts"
)

// writeFile creates one fixture file beneath root.
func writeFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// captureFixture writes a repository with generated, testdata, and docs/ref exclusions.
func captureFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/capture\n\ngo 1.25.0\n")
	writeFile(t, root, "backend/real.go", "package backend\n\n// Real is maintained.\nfunc Real() {}\n")
	writeFile(t, root, "backend/internal/sql/generated.go", "package sql\n\nfunc Generated() {}\n")
	writeFile(t, root, "tests/tooling/example/unit/example_test.go", "package example\n\nimport \"testing\"\n\nfunc TestThing(t *testing.T) {}\n")
	writeFile(t, root, "tests/tooling/example/unit/testdata/fixture.go", "package fixture\n")
	writeFile(t, root, "docs/APP-USAGE.md", "# App usage\n")
	writeFile(t, root, "docs/ref/reference.md", "# Reference\n")
	writeFile(t, root, "config/sqlc.yaml", "version: \"2\"\nsql: []\n")
	return root
}

// TestCaptureExcludesGeneratedTestdataAndReference verifies the default policy boundaries.
func TestCaptureExcludesGeneratedTestdataAndReference(t *testing.T) {
	root := captureFixture(t)
	snapshot, err := codefacts.Capture(root, codefacts.DefaultPolicy)
	if err != nil {
		t.Fatal(err)
	}
	paths := make(map[string]bool)
	for _, fact := range snapshot.Facts() {
		paths[fact.Path] = true
	}
	for _, expected := range []string{"go.mod", "backend/real.go", "tests/tooling/example/unit/example_test.go", "docs/APP-USAGE.md", "config/sqlc.yaml"} {
		if !paths[expected] {
			t.Errorf("expected captured path %q", expected)
		}
	}
	for _, excluded := range []string{"backend/internal/sql/generated.go", "tests/tooling/example/unit/testdata/fixture.go", "docs/ref/reference.md"} {
		if paths[excluded] {
			t.Errorf("excluded path was captured: %q", excluded)
		}
	}
}

// TestCaptureStableOutput verifies repeated captures of unchanged bytes are identical.
func TestCaptureStableOutput(t *testing.T) {
	root := captureFixture(t)
	first, err := codefacts.Capture(root, codefacts.DefaultPolicy)
	if err != nil {
		t.Fatal(err)
	}
	second, err := codefacts.Capture(root, codefacts.DefaultPolicy)
	if err != nil {
		t.Fatal(err)
	}
	if first.InputFingerprint() != second.InputFingerprint() {
		t.Fatal("capture fingerprints differ for unchanged bytes")
	}
}

// TestBuildUsesCapturedBytesAfterLiveEdit verifies owner scans never reread the live tree.
func TestBuildUsesCapturedBytesAfterLiveEdit(t *testing.T) {
	root := captureFixture(t)
	snapshot, err := codefacts.Capture(root, codefacts.DefaultPolicy)
	if err != nil {
		t.Fatal(err)
	}
	before, err := codefacts.Build(snapshot, codefacts.Options{RepositoryID: "repo", WorktreeID: "worktree"})
	if err != nil {
		t.Fatal(err)
	}
	livePath := filepath.Join(root, "backend", "real.go")
	if err := os.WriteFile(livePath, []byte("package backend\n\n// Real changed.\nfunc Real() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	after, err := codefacts.Build(snapshot, codefacts.Options{RepositoryID: "repo", WorktreeID: "worktree"})
	if err != nil {
		t.Fatal(err)
	}
	if before.Workspace.InputFingerprint != after.Workspace.InputFingerprint {
		t.Fatal("captured facts changed after the live tree changed")
	}
	var beforeHash, afterHash string
	for _, declaration := range before.Declarations {
		if declaration.Name == "Real" {
			beforeHash = declaration.SourceHash
		}
	}
	for _, declaration := range after.Declarations {
		if declaration.Name == "Real" {
			afterHash = declaration.SourceHash
		}
	}
	if beforeHash == "" || beforeHash != afterHash {
		t.Fatalf("captured declaration hash changed: %q vs %q", beforeHash, afterHash)
	}
	fresh, err := codefacts.Capture(root, codefacts.DefaultPolicy)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.InputFingerprint() == before.Workspace.InputFingerprint {
		t.Fatal("a fresh capture did not observe the live edit")
	}
}

// TestRunWritesPureJSONToStdout verifies the command contract.
func TestRunWritesPureJSONToStdout(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/run\n\ngo 1.25.0\n")
	writeFile(t, root, "backend/sample.go", "package sample\n\n// Sample is maintained.\nfunc Sample() {}\n")
	var stdout, stderr bytes.Buffer
	if status := codefacts.Run([]string{"--root", root}, &stdout, &stderr); status != 0 {
		t.Fatalf("unexpected exit status %d: %s", status, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected diagnostics: %s", stderr.String())
	}
	var envelope codefacts.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not pure JSON: %v\n%s", err, stdout.String())
	}
	if envelope.FormatVersion != 1 || envelope.Producer != "code-facts" {
		t.Fatalf("unexpected envelope metadata: %+v", envelope)
	}
	if envelope.Workspace.RepositoryID == "" || envelope.Workspace.WorktreeID == "" {
		t.Fatalf("workspace identity was not populated: %+v", envelope.Workspace)
	}
	if !strings.HasPrefix(envelope.Workspace.InputFingerprint, "sha256:") {
		t.Fatalf("unexpected input fingerprint %q", envelope.Workspace.InputFingerprint)
	}
}

// TestRunReportsDiagnosticsToStderr verifies failures stay off stdout.
func TestRunReportsDiagnosticsToStderr(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if status := codefacts.Run([]string{"--bogus"}, &stdout, &stderr); status != 2 {
		t.Fatalf("expected exit status 2, got %d", status)
	}
	if stdout.Len() != 0 {
		t.Fatalf("unexpected stdout on failure: %s", stdout.String())
	}
	if stderr.Len() == 0 {
		t.Fatal("expected diagnostics on stderr")
	}
}
