//go:build unit

package notes

import (
	"encoding/json"
	"os"
	"testing"
)

// TestParseSupportedBlocksAndLinks verifies bounded blocks, escaping, link normalization, and code suppression.
func TestParseSupportedBlocksAndLinks(t *testing.T) {
	body := "# Heading [[article:https://doi.org/10.1000/Example|paper]]\n\n- one\n- two\n\n> quote\n\n| A | B |\n| - | -- |\n| x\\|y | [[note:12]] |\n\n```\n[[anchor:ignored]]\n```"
	document := Parse(body)
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
	document := Parse("😀 [[ftp:bad]]")
	if len(document.Errors) != 1 {
		t.Fatalf("errors = %+v", document.Errors)
	}
	if document.Errors[0].Position != 3 {
		t.Fatalf("UTF-16 position = %d, want 3", document.Errors[0].Position)
	}
}

// TestParseRejectsUnsafeAndMalformedInput verifies save-blocking language errors remain recoverable.
func TestParseRejectsUnsafeAndMalformedInput(t *testing.T) {
	for _, body := range []string{"[[ext:javascript:alert(1)]]", "[[pdf:page=0]]", "[[unknown:value]]", "[[note:1\\q]]", "```\nunclosed", "| a | b |\n| wrong | shape |"} {
		if document := Parse(body); len(document.Errors) == 0 {
			t.Errorf("Parse(%q) accepted malformed input", body)
		}
	}
}

// TestConformanceFixtures verifies the authoritative normalized link and diagnostic corpus.
func TestConformanceFixtures(t *testing.T) {
	data, err := os.ReadFile("testdata/conformance.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name               string   `json:"name"`
		Body               string   `json:"body"`
		LinkTypes          []string `json:"link_types"`
		ErrorCount         int      `json:"error_count"`
		FirstErrorPosition *int     `json:"first_error_position"`
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			document := Parse(fixture.Body)
			if len(document.Errors) != fixture.ErrorCount || len(document.Links) != len(fixture.LinkTypes) {
				t.Fatalf("document=%+v", document)
			}
			for index, kind := range fixture.LinkTypes {
				if document.Links[index].TargetType != kind {
					t.Fatalf("link %d type=%q want=%q", index, document.Links[index].TargetType, kind)
				}
			}
			if fixture.FirstErrorPosition != nil && document.Errors[0].Position != *fixture.FirstErrorPosition {
				t.Fatalf("error position=%d want=%d", document.Errors[0].Position, *fixture.FirstErrorPosition)
			}
		})
	}
}
