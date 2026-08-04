// bibtex_unit_test.go tests the BibTeX parser, entry-type constants, and
// field extraction helpers in isolation (no I/O).
//go:build unit

package bibtex

import (
	"testing"
)

// TestEntryType verifies entry type.
func TestEntryType(t *testing.T) {
	t.Run("values", func(t *testing.T) {
		if EntryArticle.String() != "article" {
			t.Fatalf("expected 'article', got %q", EntryArticle.String())
		}
		if EntryBook.String() != "book" {
			t.Fatalf("expected 'book', got %q", EntryBook.String())
		}
		if EntryInProceedings.String() != "inproceedings" {
			t.Fatalf("expected 'inproceedings', got %q", EntryInProceedings.String())
		}
		if EntryMisc.String() != "misc" {
			t.Fatalf("expected 'misc', got %q", EntryMisc.String())
		}
		if EntryUnknown.String() != "unknown" {
			t.Fatalf("expected 'unknown', got %q", EntryUnknown.String())
		}
	})

	t.Run("parse_from_string", func(t *testing.T) {
		tests := []struct {
			input string
			want  EntryType
		}{
			{"article", EntryArticle},
			{"ARTICLE", EntryArticle},
			{"book", EntryBook},
			{"inproceedings", EntryInProceedings},
			{"misc", EntryMisc},
			{"foobar", EntryUnknown},
			{"", EntryUnknown},
		}
		for _, tc := range tests {
			got := entryTypeFromString(tc.input)
			if got != tc.want {
				t.Errorf("entryTypeFromString(%q) = %d, want %d", tc.input, got, tc.want)
			}
		}
	})
}

// TestParse verifies parse.
func TestParse(t *testing.T) {
	p := parser()

	t.Run("simple_entry", func(t *testing.T) {
		lib, err := p.Parse(simpleBib, "test", true)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		if len(lib) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(lib))
		}
		for _, entry := range lib {
			if entry["author"] != "Alice, Bob" {
				t.Fatalf("expected author 'Alice, Bob', got %q", entry["author"])
			}
			if entry["title"] != "Test" {
				t.Fatalf("expected title 'Test', got %q", entry["title"])
			}
			if entry["year"] != "2024" {
				t.Fatalf("expected year '2024', got %q", entry["year"])
			}
			if entry["doi"] != "10.1234/test" {
				t.Fatalf("expected doi '10.1234/test', got %q", entry["doi"])
			}
			if entry["article_source"] != "test" {
				t.Fatalf("expected article_source 'test', got %q", entry["article_source"])
			}
		}
	})

	t.Run("strip_braces", func(t *testing.T) {
		bib := `@article{k, title = {The Title}, doi = {10.1/a}}`
		lib, err := p.Parse(bib, "", true)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		if lib["k"]["title"] != "The Title" {
			t.Fatalf("expected 'The Title', got %q", lib["k"]["title"])
		}
	})

	t.Run("strip_braces_nested", func(t *testing.T) {
		bib := `@article{k, title = {{The Title}}, doi = {10.1/a}}`
		lib, err := p.Parse(bib, "", true)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		if lib["k"]["title"] != "{The Title}" {
			t.Fatalf("expected '{The Title}', got %q", lib["k"]["title"])
		}
	})

	t.Run("skips_non_article", func(t *testing.T) {
		bib := `@book{k, title = {A Book}}`
		lib, err := p.Parse(bib, "", true)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		if len(lib) != 0 {
			t.Fatalf("expected 0 entries for non-article, got %d", len(lib))
		}
	})

	t.Run("handles_duplicate_keys", func(t *testing.T) {
		bib := `@article{k, title = {A}}` + "\n" + `@article{k, title = {B}}`
		lib, err := p.Parse(bib, "", true)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		if len(lib) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(lib))
		}
		if lib["k"]["title"] != "A" {
			t.Fatalf("expected first entry title 'A', got %q", lib["k"]["title"])
		}
		if lib["k_0"]["title"] != "B" {
			t.Fatalf("expected duplicate entry title 'B', got %q", lib["k_0"]["title"])
		}
	})

	t.Run("empty_input", func(t *testing.T) {
		lib, err := p.Parse("", "empty.bib", true)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		if len(lib) != 0 {
			t.Fatalf("expected empty library, got %d entries", len(lib))
		}
	})

	t.Run("no_strip_braces_outer_kept_nested", func(t *testing.T) {
		bib := `@article{k, title = {{Nested}}}` + "\n" +
			`@article{m, title = {{Deep {Nested}}}}`
		lib, err := p.Parse(bib, "", false)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		if lib["k"]["title"] != "{Nested}" {
			t.Fatalf("expected '{Nested}', got %q", lib["k"]["title"])
		}
		if lib["m"]["title"] != "{Deep {Nested}}" {
			t.Fatalf("expected '{Deep {Nested}}', got %q", lib["m"]["title"])
		}
	})

	t.Run("identifier_value", func(t *testing.T) {
		bib := `@article{k, crossref = some_ref}`
		lib, err := p.Parse(bib, "", true)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		if lib["k"]["crossref"] != "some_ref" {
			t.Fatalf("expected 'some_ref', got %q", lib["k"]["crossref"])
		}
	})

	t.Run("quoted_value", func(t *testing.T) {
		bib := `@article{k, title = "A quoted, title", year = "2024"}`
		lib, err := p.Parse(bib, "", true)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		if lib["k"]["title"] != "A quoted, title" || lib["k"]["year"] != "2024" {
			t.Fatalf("unexpected quoted fields: %v", lib["k"])
		}
	})

	t.Run("concatenated_value", func(t *testing.T) {
		bib := `@article{k, title = "Business " # {Process} # " Management"}`
		lib, err := p.Parse(bib, "", true)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		if lib["k"]["title"] != "Business Process Management" {
			t.Fatalf("unexpected concatenated title: %q", lib["k"]["title"])
		}
	})

	t.Run("citation_key_with_colon", func(t *testing.T) {
		bib := `@article{WOS:000123, title = {Title}}`
		lib, err := p.Parse(bib, "", true)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		if lib["wos:000123"]["title"] != "Title" {
			t.Fatalf("citation key was not preserved: %v", lib)
		}
	})

	t.Run("case_insensitive_entry_type", func(t *testing.T) {
		bib := `@Article{k, title = {Title}}`
		lib, err := p.Parse(bib, "", true)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		if len(lib) != 1 {
			t.Fatalf("expected 1 entry for @Article, got %d", len(lib))
		}
	})

	t.Run("case_insensitive_field_names", func(t *testing.T) {
		bib := `@article{k, TITLE = {Title}}`
		lib, err := p.Parse(bib, "", true)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		if lib["k"]["title"] != "Title" {
			t.Fatalf("expected 'Title', got %q", lib["k"]["title"])
		}
	})

	t.Run("escaped_braces", func(t *testing.T) {
		bib := `@article{k, title = {Escaped \{ brace}}`
		lib, err := p.Parse(bib, "", true)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		if lib["k"]["title"] != `Escaped \{ brace` {
			t.Fatalf("expected 'Escaped \\{ brace', got %q", lib["k"]["title"])
		}
	})

	t.Run("multiple_duplicates", func(t *testing.T) {
		bib := `@article{k, title = {A}}` + "\n" +
			`@article{k, title = {B}}` + "\n" +
			`@article{k, title = {C}}`
		lib, err := p.Parse(bib, "", true)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		if len(lib) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(lib))
		}
		if lib["k"]["title"] != "A" {
			t.Fatalf("expected 'A', got %q", lib["k"]["title"])
		}
		if lib["k_0"]["title"] != "B" {
			t.Fatalf("expected 'B', got %q", lib["k_0"]["title"])
		}
		if lib["k_1"]["title"] != "C" {
			t.Fatalf("expected 'C', got %q", lib["k_1"]["title"])
		}
	})

	t.Run("entry_type_field", func(t *testing.T) {
		bib := `@article{k, title = {T}}`
		lib, err := p.Parse(bib, "", true)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		if lib["k"]["entry_type"] != "article" {
			t.Fatalf("expected entry_type 'article', got %q", lib["k"]["entry_type"])
		}
	})

	t.Run("empty_field_value", func(t *testing.T) {
		bib := `@article{k, title}`
		lib, err := p.Parse(bib, "", true)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		if lib["k"]["title"] != "" {
			t.Fatalf("expected empty title, got %q", lib["k"]["title"])
		}
	})
}

// TestNewParserNilLogger verifies new parser nil logger.
func TestNewParserNilLogger(t *testing.T) {
	p := NewParser(nil)
	lib, err := p.Parse("@article{k, title = {T}}", "", true)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if lib["k"]["title"] != "T" {
		t.Fatalf("expected 'T', got %q", lib["k"]["title"])
	}
}
