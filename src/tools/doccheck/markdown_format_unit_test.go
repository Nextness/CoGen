//go:build unit

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestSingleLineMarkdownAcceptsOneLineContentAndMultilineFences verifies single line markdown accepts one line content and multiline fences.
func TestSingleLineMarkdownAcceptsOneLineContentAndMultilineFences(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/guide.md", "# Guide\n\nOne complete paragraph.\n\n- One complete item.\n- Another complete item.\n\n| A | B |\n|---|---|\n| one | two |\n\n```text\ncomponent\n  -> dependency\n```\n")
	failures := checkSingleLineMarkdown(root, []string{filepath.FromSlash("docs/guide.md")})
	if len(failures) != 0 {
		t.Fatalf("single-line check returned %v", failures)
	}
}

// TestSingleLineMarkdownReportsWrappedParagraphListAndTableContent verifies single line markdown reports wrapped paragraph list and table content.
func TestSingleLineMarkdownReportsWrappedParagraphListAndTableContent(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/guide.md", "First paragraph line\nsecond paragraph line.\n\n- List item\n  continuation.\n\n| A | B |\ncontinued table content\n")
	failures := checkSingleLineMarkdown(root, []string{filepath.FromSlash("docs/guide.md")})
	if len(failures) != 3 {
		t.Fatalf("single-line failures = %v", failures)
	}
	if !strings.Contains(strings.Join(failures, "\n"), "one physical line") {
		t.Fatalf("single-line failure message = %v", failures)
	}
}
