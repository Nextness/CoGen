package notes_test

import "analysis/tests/support/go/testenv"

import subject "analysis/backend/notes"

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

// TestParseSupportedBlocksAndLinks verifies bounded blocks, escaping, link normalization, and code suppression.
func TestParseSupportedBlocksAndLinks(t *testing.T) {
	body := "# Heading [[article:https://doi.org/10.1000/Example|paper]]\n\n- one\n- two\n\n> quote\n\n| A | B |\n| --- | --- |\n| x\\|y | [[note:12]] |\n\n```\n[[anchor:ignored]]\n```"
	document := subject.Parse(body)
	if len(document.Errors) != 0 {
		t.Fatalf("Parse errors = %+v", document.Errors)
	}
	if len(document.Blocks) != 5 {
		t.Fatalf("blocks = %d, want 5: %+v", len(document.Blocks), document.Blocks)
	}
	if len(document.Links) != 2 || document.Links[0].TargetType != "article" || document.Links[0].RawTarget != "10.1000/example" || document.Links[1].TargetType != "note" {
		t.Fatalf("links = %+v", document.Links)
	}
}

// TestParseReportsUTF16Diagnostics verifies non-BMP characters count as two UTF-16 code units.
func TestParseReportsUTF16Diagnostics(t *testing.T) {
	document := subject.Parse("😀 [[ftp:bad]]")
	if len(document.Errors) != 1 {
		t.Fatalf("errors = %+v", document.Errors)
	}
	if document.Errors[0].Position != 3 {
		t.Fatalf("UTF-16 position = %d, want 3", document.Errors[0].Position)
	}
}

// TestParseRejectsUnsafeAndMalformedInput verifies save-blocking language errors remain recoverable.
func TestParseRejectsUnsafeAndMalformedInput(t *testing.T) {
	for _, body := range []string{"[[ext:javascript:alert(1)]]", "[[pdf:page=0]]", "[[unknown:value]]", "[[note:1\\q]]", "```\nunclosed", "| a | b |\n| --- | --- |\n| missing |"} {
		if document := subject.Parse(body); len(document.Errors) == 0 {
			t.Errorf("Parse(%q) accepted malformed input", body)
		}
	}
}

// TestConformanceFixtures verifies the authoritative normalized link and diagnostic corpus.
func TestConformanceFixtures(t *testing.T) {
	data, err := os.ReadFile(testenv.Path("tests", "contracts", "notes", "testdata", "conformance.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name string `json:"name"`
		Body string `json:"body"`
		subject.Document
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			document := subject.Parse(fixture.Body)
			if !reflect.DeepEqual(document, fixture.Document) {
				got, _ := json.MarshalIndent(document, "", "  ")
				want, _ := json.MarshalIndent(fixture.Document, "", "  ")
				t.Fatalf("document mismatch\ngot: %s\nwant: %s", got, want)
			}
		})
	}
}

// TestPlainTextRemovesApplicationLinkSyntax verifies PDF annotation text is safe plain prose.
func TestPlainTextRemovesApplicationLinkSyntax(t *testing.T) {
	body := "# Methods\n\nSee [[anchor:methods-1|the methods anchor]] and [[ext:https://example.test|source]].\n\n- item one\n\n```\ncode block\n```\n\n| A | B |\n| --- | --- |\n| 1 | 2 |"
	text := subject.PlainText(body)
	for _, forbidden := range []string{"[[", "]]", "anchor:methods-1"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("plain text retained %q: %s", forbidden, text)
		}
	}
	for _, expected := range []string{"Methods", "the methods anchor", "source", "item one", "code block", "A | B", "1 | 2"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("plain text missing %q: %s", expected, text)
		}
	}
	if got := subject.PlainText("[[anchor:invalid id]]"); got != "" {
		t.Fatalf("invalid note plain text=%q", got)
	}
}
