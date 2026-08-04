//go:build unit

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stateFixture supports the package test suite's state fixture setup or assertions.
func stateFixture() string {
	return "# Documentation State\n\n" + dependencyBegin + "\n\n| Source document | Review dependents |\n|---|---|\n| [docs/A.md](A.md) | [docs/B.md](B.md) |\n| [docs/B.md](B.md) | None |\n\n" + dependencyEnd + "\n\n" + stateBegin + "\n\n| Document | SHA-256 | Review dependents |\n|---|---|---|\n\n" + stateEnd + "\n"
}

// TestDocumentationFilesExcludeStateAndReferenceTree verifies documentation files exclude state and reference tree.
func TestDocumentationFilesExcludeStateAndReferenceTree(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/A.md", "A\n")
	writeTestFile(t, root, stateDocument, stateFixture())
	writeTestFile(t, root, "docs/ref/vendor.md", "Vendor\n")
	paths, err := documentationFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "docs/A.md" {
		t.Fatalf("documentationFiles = %v", paths)
	}
}

// TestDocumentStateReportsChangedAddedAndRemovedFilesWithDependents verifies document state reports changed added and removed files with dependents.
func TestDocumentStateReportsChangedAddedAndRemovedFilesWithDependents(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/A.md", "A\n")
	writeTestFile(t, root, "docs/B.md", "B\n")
	writeTestFile(t, root, stateDocument, stateFixture())
	if status := runStateCommand([]string{"update", "--root", root}, &strings.Builder{}, &strings.Builder{}); status != 0 {
		t.Fatalf("initial state update status = %d", status)
	}
	writeTestFile(t, root, "docs/A.md", "Changed\n")
	writeTestFile(t, root, "docs/C.md", "Added\n")
	if err := os.Remove(filepath.Join(root, "docs", "B.md")); err != nil {
		t.Fatal(err)
	}
	failures, err := checkDocumentState(root)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(failures, "\n")
	for _, expected := range []string{"docs/A.md: documentation changed", "review docs/B.md", "docs/B.md: documented file is missing", "docs/C.md: documentation was added"} {
		if !strings.Contains(joined, expected) {
			t.Errorf("state failures missing %q:\n%s", expected, joined)
		}
	}
}

// TestDocumentStateRejectsMalformedDuplicateEscapingAndExcludedRows verifies document state rejects malformed duplicate escaping and excluded rows.
func TestDocumentStateRejectsMalformedDuplicateEscapingAndExcludedRows(t *testing.T) {
	tests := []struct {
		name string
		row  string
		want string
	}{
		{name: "malformed", row: "not a table row", want: "malformed"},
		{name: "bare-hash", row: "| [docs/A.md](A.md) | aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa | None |", want: "malformed SHA-256"},
		{name: "escaping", row: "| [docs/../secret](../secret) | `aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa` | None |", want: "escaping"},
		{name: "excluded", row: "| [docs/ref/vendor.md](ref/vendor.md) | `aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa` | None |", want: "excluded"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document := stateFixture()
			document = strings.Replace(document, "| Document | SHA-256 | Review dependents |\n|---|---|---|", "| Document | SHA-256 | Review dependents |\n|---|---|---|\n"+test.row, 1)
			_, failures := parseStoredState(document)
			if !strings.Contains(strings.Join(failures, "\n"), test.want) {
				t.Fatalf("parseStoredState failures = %v", failures)
			}
		})
	}
	document := stateFixture()
	row := "| [docs/A.md](A.md) | `aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa` | [docs/B.md](B.md) |"
	document = strings.Replace(document, "| Document | SHA-256 | Review dependents |\n|---|---|---|", "| Document | SHA-256 | Review dependents |\n|---|---|---|\n"+row+"\n"+row, 1)
	_, failures := parseStoredState(document)
	if !strings.Contains(strings.Join(failures, "\n"), "duplicated") {
		t.Fatalf("duplicate state failures = %v", failures)
	}
}

// TestStateCheckIsNonMutatingAndUpdatePreservesDependencies verifies state check is non mutating and update preserves dependencies.
func TestStateCheckIsNonMutatingAndUpdatePreservesDependencies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/A.md", "A\n")
	writeTestFile(t, root, "docs/B.md", "B\n")
	document := stateFixture()
	writeTestFile(t, root, stateDocument, document)
	if _, err := checkDocumentState(root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(stateDocument)))
	if err != nil || string(data) != document {
		t.Fatalf("state check mutated the document: %v", err)
	}
	if status := runStateCommand([]string{"update", "--root", root}, &strings.Builder{}, &strings.Builder{}); status != 0 {
		t.Fatalf("state update status = %d", status)
	}
	updated, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(stateDocument)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), "[docs/A.md](A.md) | [docs/B.md](B.md)") || !strings.Contains(string(updated), "[docs/A.md](A.md) | `") {
		t.Fatalf("state update lost maintained or generated content:\n%s", updated)
	}
}

// TestStateCheckDetectsStaleGeneratedDependentsAndUpdateRepairsGeneratedRows verifies state check detects stale generated dependents and update repairs generated rows.
func TestStateCheckDetectsStaleGeneratedDependentsAndUpdateRepairsGeneratedRows(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/A.md", "A\n")
	writeTestFile(t, root, "docs/B.md", "B\n")
	writeTestFile(t, root, stateDocument, stateFixture())
	if status := runStateCommand([]string{"update", "--root", root}, &strings.Builder{}, &strings.Builder{}); status != 0 {
		t.Fatalf("initial state update status = %d", status)
	}
	path := filepath.Join(root, filepath.FromSlash(stateDocument))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stale := strings.Replace(string(data), "[docs/B.md](B.md) |", "None |", 1)
	if err := os.WriteFile(path, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	failures, err := checkDocumentState(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(failures, "\n"), "review dependents are stale") {
		t.Fatalf("stale dependent failures = %v", failures)
	}
	malformed := strings.Replace(stale, "| [docs/A.md](A.md) | `", "malformed row\n| [docs/A.md](A.md) | `", 1)
	if err := os.WriteFile(path, []byte(malformed), 0o644); err != nil {
		t.Fatal(err)
	}
	if status := runStateCommand([]string{"update", "--root", root}, &strings.Builder{}, &strings.Builder{}); status != 0 {
		t.Fatalf("state repair status = %d", status)
	}
	if failures, err := checkDocumentState(root); err != nil || len(failures) != 0 {
		t.Fatalf("repaired state failures = %v, err = %v", failures, err)
	}
}
