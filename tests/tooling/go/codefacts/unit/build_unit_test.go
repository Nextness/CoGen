package codefacts_test

import (
	"encoding/json"
	"io/fs"
	"reflect"
	"sort"
	"strings"
	"testing"

	"analysis/tooling/go/codefacts"
)

// fixtureFiles returns a small frozen repository snapshot for portable facts.
func fixtureFiles() map[string][]byte {
	return map[string][]byte{
		"go.mod": []byte("module example.com/fixture\n\ngo 1.25.0\n"),
		"backend/sample.go": []byte(`package sample

import "fmt"

// Record stores one value.
type Record struct{ Value string }

// Save persists one record.
func (record *Record) Save() error {
	fmt.Println(record.Value)
	return nil
}

func helper() {}

func query() {
	db.Query("select 1")
}
`),
		"tests/tooling/example/unit/example_test.go": []byte(`package example

import "testing"

func TestThing(t *testing.T) {}
`),
		"config/sqlc.yaml": []byte(`version: "2"
sql:
  - engine: "sqlite"
    schema: "../backend/schema/definitions.sql"
    queries:
      - "../backend/schema/queries.sql"
    gen:
      go:
        package: "sql"
        out: "../backend/internal/sql"
`),
		"docs/DOC-STATE.md": []byte(`# Documentation State

<!-- BEGIN DOCUMENT REVIEW DEPENDENCIES -->

| Source document | Review dependents |
|---|---|
| [docs/APP-USAGE.md](APP-USAGE.md) | None |

<!-- END DOCUMENT REVIEW DEPENDENCIES -->

<!-- BEGIN GENERATED DOCUMENT STATE -->

| Document | SHA-256 | Review dependents |
|---|---|---|
| [docs/APP-USAGE.md](APP-USAGE.md) | ` + "`aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa`" + ` | None |

<!-- END GENERATED DOCUMENT STATE -->
`),
	}
}

// TestBuildProducesDeterministicEnvelope verifies stable ordering, versions, and facts.
func TestBuildProducesDeterministicEnvelope(t *testing.T) {
	snapshot, err := codefacts.NewSnapshot(fixtureFiles())
	if err != nil {
		t.Fatal(err)
	}
	options := codefacts.Options{RepositoryID: "repo-1", WorktreeID: "worktree-1"}
	first, err := codefacts.Build(snapshot, options)
	if err != nil {
		t.Fatal(err)
	}
	second, err := codefacts.Build(snapshot, options)
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatal("Build is not deterministic for the same captured bytes")
	}
	if first.FormatVersion != 1 || first.Producer != "code-facts" {
		t.Fatalf("unexpected version metadata: %+v", first)
	}
	if first.Workspace.RepositoryID != "repo-1" || first.Workspace.WorktreeID != "worktree-1" {
		t.Fatalf("workspace identity not preserved: %+v", first.Workspace)
	}
	if !strings.HasPrefix(first.Workspace.InputFingerprint, "sha256:") || len(first.Workspace.InputFingerprint) != 71 {
		t.Fatalf("unexpected input fingerprint %q", first.Workspace.InputFingerprint)
	}
	if len(first.Declarations) == 0 {
		t.Fatal("expected declarations")
	}
	var found bool
	for _, declaration := range first.Declarations {
		if declaration.Name == "(*Record).Save" {
			found = true
			if declaration.Module != "example.com/fixture" {
				t.Fatalf("unexpected module %q", declaration.Module)
			}
			if declaration.Evidence != "syntactic" {
				t.Fatalf("unexpected evidence %q", declaration.Evidence)
			}
			if !strings.HasPrefix(declaration.SourceHash, "sha256:") {
				t.Fatalf("unexpected source hash %q", declaration.SourceHash)
			}
		}
	}
	if !found {
		t.Fatal("missing Save declaration")
	}
	if len(first.Imports) == 0 {
		t.Fatal("expected imports")
	}
	if len(first.SQLCalls) == 0 {
		t.Fatal("expected raw SQL calls")
	}
	if len(first.SQLGenerationMappings) != 1 {
		t.Fatalf("expected one SQL generation mapping, got %d", len(first.SQLGenerationMappings))
	}
	if len(first.Suites) == 0 {
		t.Fatal("expected suites")
	}
	if len(first.ReviewMetadata) != 1 {
		t.Fatalf("expected one review metadata row, got %d", len(first.ReviewMetadata))
	}
	if first.ReviewMetadata[0].Document != "docs/APP-USAGE.md" {
		t.Fatalf("unexpected review document %q", first.ReviewMetadata[0].Document)
	}
}

// TestBuildPreservesMissingCommentFallback verifies declarations without comments do not fail.
func TestBuildPreservesMissingCommentFallback(t *testing.T) {
	files := fixtureFiles()
	files["backend/sample.go"] = []byte("package sample\n\nfunc helper() {}\n")
	snapshot, err := codefacts.NewSnapshot(files)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := codefacts.Build(snapshot, codefacts.Options{RepositoryID: "repo", WorktreeID: "worktree"})
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, declaration := range envelope.Declarations {
		if declaration.Name == "helper" {
			found = true
			if declaration.Description != "No source description" {
				t.Fatalf("unexpected missing-comment fallback %q", declaration.Description)
			}
		}
	}
	if !found {
		t.Fatal("missing helper declaration")
	}
}

// TestBuildChangesInputFingerprintWhenCapturedBytesChange verifies the fingerprint tracks source bytes.
func TestBuildChangesInputFingerprintWhenCapturedBytesChange(t *testing.T) {
	firstFiles := fixtureFiles()
	firstSnapshot, err := codefacts.NewSnapshot(firstFiles)
	if err != nil {
		t.Fatal(err)
	}
	first, err := codefacts.Build(firstSnapshot, codefacts.Options{RepositoryID: "repo", WorktreeID: "worktree"})
	if err != nil {
		t.Fatal(err)
	}
	secondFiles := fixtureFiles()
	secondFiles["backend/sample.go"] = append([]byte("// changed\n"), secondFiles["backend/sample.go"]...)
	secondSnapshot, err := codefacts.NewSnapshot(secondFiles)
	if err != nil {
		t.Fatal(err)
	}
	second, err := codefacts.Build(secondSnapshot, codefacts.Options{RepositoryID: "repo", WorktreeID: "worktree"})
	if err != nil {
		t.Fatal(err)
	}
	if first.Workspace.InputFingerprint == second.Workspace.InputFingerprint {
		t.Fatal("input fingerprint did not change with captured bytes")
	}
	if reflect.DeepEqual(first.Files, second.Files) {
		t.Fatal("file facts did not change with captured bytes")
	}
}

// TestNewSnapshotRejectsEscapedPaths verifies invalid captured paths are rejected.
func TestNewSnapshotRejectsEscapedPaths(t *testing.T) {
	for _, escaped := range []string{"../escape.go", "/absolute.go", "backend/../escape.go", "backend//empty.go"} {
		if _, err := codefacts.NewSnapshot(map[string][]byte{escaped: []byte("package escape\n")}); err == nil {
			t.Fatalf("expected error for escaped path %q", escaped)
		}
	}
}

// TestSnapshotReadFileReturnsCapturedBytes verifies the frozen view does not alias caller data.
func TestSnapshotReadFileReturnsCapturedBytes(t *testing.T) {
	files := map[string][]byte{"backend/sample.go": []byte("package sample\n")}
	snapshot, err := codefacts.NewSnapshot(files)
	if err != nil {
		t.Fatal(err)
	}
	files["backend/sample.go"][0] = 'X'
	data, err := snapshot.ReadFile("backend/sample.go")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "package sample\n" {
		t.Fatalf("snapshot aliased caller bytes: %q", data)
	}
}

// TestNewSnapshotPreservesSiblingDirectories verifies directory children are not collapsed by shared keys.
func TestNewSnapshotPreservesSiblingDirectories(t *testing.T) {
	files := map[string][]byte{
		"backend/a.go":   []byte("package backend\n"),
		"backend/b.go":   []byte("package backend\n"),
		"backend/x/c.go": []byte("package x\n"),
		"backend/x/d.go": []byte("package x\n"),
		"backend/y/e.go": []byte("package y\n"),
	}
	snapshot, err := codefacts.NewSnapshot(files)
	if err != nil {
		t.Fatal(err)
	}
	var visited []string
	err = fs.WalkDir(snapshot, "backend", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		visited = append(visited, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(visited)
	want := []string{"backend", "backend/a.go", "backend/b.go", "backend/x", "backend/x/c.go", "backend/x/d.go", "backend/y", "backend/y/e.go"}
	if !reflect.DeepEqual(visited, want) {
		t.Fatalf("unexpected walk paths:\nwant %v\ngot  %v", want, visited)
	}
}
