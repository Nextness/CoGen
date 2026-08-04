// article_unit_test.go tests the article model types, text sanitisation, and
// DOI deduplication logic.
//go:build unit

package article

import (
	"testing"
)

// TestSanitizeTextRemovesHTML verifies sanitize text removes html.
func TestSanitizeTextRemovesHTML(t *testing.T) {
	got := SanitizeText("hello <b>world</b>")
	if got != "hello world" {
		t.Errorf("expected 'hello world', got %q", got)
	}
}

// TestSanitizeTextSmartQuotes verifies sanitize text smart quotes.
func TestSanitizeTextSmartQuotes(t *testing.T) {
	got := SanitizeText("\u201cquoted\u201d")
	if got != `"quoted"` {
		t.Errorf("expected '\\\"quoted\\\"', got %q", got)
	}
}

// TestSanitizeTextBOM verifies sanitize text bom.
func TestSanitizeTextBOM(t *testing.T) {
	got := SanitizeText("\ufeffhello")
	if got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
}

// TestSanitizeTextDashes verifies sanitize text dashes.
func TestSanitizeTextDashes(t *testing.T) {
	got := SanitizeText("a\u2013b\u2014c")
	if got != "a--b---c" {
		t.Errorf("expected 'a--b---c', got %q", got)
	}
}

// TestSplitToListBySemicolon verifies split to list by semicolon.
func TestSplitToListBySemicolon(t *testing.T) {
	got := SplitToList("a; b; c", ";")
	want := []string{"a", "b", "c"}
	if !stringSliceEqual(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

// TestSplitToListByNewline verifies split to list by newline.
func TestSplitToListByNewline(t *testing.T) {
	got := SplitToList("a\nb\nc", "\n")
	want := []string{"a", "b", "c"}
	if !stringSliceEqual(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

// TestSplitToListEmpty verifies split to list empty.
func TestSplitToListEmpty(t *testing.T) {
	got := SplitToList("", ";")
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

// TestSplitToListNoNewlineCollapse verifies split to list no newline collapse.
func TestSplitToListNoNewlineCollapse(t *testing.T) {
	// When separator is not "\n", newlines are stripped first
	got := SplitToList("a\n; b; c", ";")
	want := []string{"a", "b", "c"}
	if !stringSliceEqual(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

// TestParseInt verifies parse int.
func TestParseInt(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"2024", 2024},
		{"", 0},
		{"abc", 0},
		{"  2024  ", 2024},
	}
	for _, tc := range tests {
		got := ParseInt(tc.input)
		if got != tc.want {
			t.Errorf("ParseInt(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

// TestParseOptionalInt verifies parse optional int.
func TestParseOptionalInt(t *testing.T) {
	if v := ParseOptionalInt("42"); v == nil || *v != 42 {
		t.Errorf("expected 42, got %v", v)
	}
	if v := ParseOptionalInt(""); v != nil {
		t.Errorf("expected nil, got %v", *v)
	}
}

// TestExtractDOIBare verifies extract doi bare.
func TestExtractDOIBare(t *testing.T) {
	got := ExtractDOI("10.1234/abc.def")
	if got != "10.1234/abc.def" {
		t.Errorf("expected '10.1234/abc.def', got %q", got)
	}
}

// TestExtractDOIFromURL verifies extract doi from url.
func TestExtractDOIFromURL(t *testing.T) {
	got := ExtractDOI("https://doi.org/10.1234/abc.def")
	if got != "10.1234/abc.def" {
		t.Errorf("expected '10.1234/abc.def', got %q", got)
	}
}

// TestExtractDOITrailingPunct verifies extract doi trailing punct.
func TestExtractDOITrailingPunct(t *testing.T) {
	got := ExtractDOI("10.1234/abc.def.")
	if got != "10.1234/abc.def" {
		t.Errorf("expected '10.1234/abc.def', got %q", got)
	}
}

// TestExtractDOINone verifies extract doi none.
func TestExtractDOINone(t *testing.T) {
	got := ExtractDOI("no doi here")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// TestParseAuthorsSemicolon verifies parse authors semicolon.
func TestParseAuthorsSemicolon(t *testing.T) {
	authors := ParseAuthors("Doe J.; Smith A.", "Univ A\nUniv B")
	if len(authors) != 2 {
		t.Fatalf("expected 2 authors, got %d", len(authors))
	}
	if authors[0].CitationName != "Doe J." || authors[0].Affiliation != "Univ A" {
		t.Errorf("unexpected author 0: %+v", authors[0])
	}
	if authors[1].CitationName != "Smith A." || authors[1].Affiliation != "Univ B" {
		t.Errorf("unexpected author 1: %+v", authors[1])
	}
}

// TestParseAuthorsSemicolonAffiliations verifies parse authors semicolon affiliations.
func TestParseAuthorsSemicolonAffiliations(t *testing.T) {
	authors := ParseAuthors("Doe J.; Smith A.", "Univ A; Univ B")
	if len(authors) != 2 || authors[0].Affiliation != "Univ A" || authors[1].Affiliation != "Univ B" {
		t.Fatalf("unexpected authors: %+v", authors)
	}
}

// TestParseAuthorsAndFallback verifies parse authors and fallback.
func TestParseAuthorsAndFallback(t *testing.T) {
	authors := ParseAuthors("Doe, J. and Smith, A.", "")
	if len(authors) != 2 {
		t.Fatalf("expected 2 authors, got %d", len(authors))
	}
	if authors[0].CitationName != "Doe, J." {
		t.Errorf("expected 'Doe, J.', got %q", authors[0].CitationName)
	}
}

// TestParseAuthorsEmpty verifies parse authors empty.
func TestParseAuthorsEmpty(t *testing.T) {
	authors := ParseAuthors("", "")
	if authors != nil {
		t.Errorf("expected nil, got %v", authors)
	}
}

// TestParseReferences verifies parse references.
func TestParseReferences(t *testing.T) {
	refs := ParseReferences("Some ref 10.1234/one\nAnother ref 10.5678/two")
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(refs))
	}
	if refs[0].DOI != "10.1234/one" {
		t.Errorf("expected DOI '10.1234/one', got %q", refs[0].DOI)
	}
	if refs[1].DOI != "10.5678/two" {
		t.Errorf("expected DOI '10.5678/two', got %q", refs[1].DOI)
	}
}

// TestParseReferencesSemicolonSeparated verifies parse references semicolon separated.
func TestParseReferencesSemicolonSeparated(t *testing.T) {
	refs := ParseReferences("Some ref 10.1234/one; Another ref 10.5678/two")
	if len(refs) != 2 || refs[0].DOI != "10.1234/one" || refs[1].DOI != "10.5678/two" {
		t.Fatalf("unexpected references: %+v", refs)
	}
}

// TestRenameFields verifies rename fields.
func TestRenameFields(t *testing.T) {
	m := map[string]string{"old": "val", "keep": "stay"}
	RenameFields(m, map[string]string{"old": "new"})
	if m["new"] != "val" {
		t.Errorf("expected 'val' at 'new', got %q", m["new"])
	}
	if _, ok := m["old"]; ok {
		t.Error("'old' should be deleted after rename")
	}
	if m["keep"] != "stay" {
		t.Errorf("'keep' should remain, got %q", m["keep"])
	}
}

// TestRenameFieldsChainedRenamesAreDeterministic verifies rename fields chained renames are deterministic.
func TestRenameFieldsChainedRenamesAreDeterministic(t *testing.T) {
	m := map[string]string{"a": "from-a", "b": "from-b"}
	RenameFields(m, map[string]string{"a": "b", "b": "c"})
	if m["b"] != "from-a" || m["c"] != "from-b" {
		t.Fatalf("unexpected chained rename result: %v", m)
	}
}

// TestKeepFields verifies keep fields.
func TestKeepFields(t *testing.T) {
	m := map[string]string{"a": "1", "b": "2", "c": "3"}
	KeepFields(m, []string{"a", "c"})
	if len(m) != 2 {
		t.Errorf("expected 2 keys, got %d", len(m))
	}
	if m["a"] != "1" || m["c"] != "3" {
		t.Errorf("unexpected map: %v", m)
	}
	if _, ok := m["b"]; ok {
		t.Error("'b' should be removed")
	}
}

// TestNewFromMapMinimal verifies new from map minimal.
func TestNewFromMapMinimal(t *testing.T) {
	entry := map[string]string{
		"doi":     "10.1234/test",
		"title":   "Test Article",
		"year":    "2024",
		"authors": "Doe J.",
	}
	a, err := NewFromMap(entry, "test_source")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.DOI != "10.1234/test" {
		t.Errorf("DOI = %q", a.DOI)
	}
	if a.Title != "Test Article" {
		t.Errorf("Title = %q", a.Title)
	}
	if a.Year != 2024 {
		t.Errorf("Year = %d", a.Year)
	}
	if a.Source != "test_source" {
		t.Errorf("Source = %q", a.Source)
	}
	if len(a.Authors) != 1 {
		t.Errorf("expected 1 author, got %d", len(a.Authors))
	}
}

// TestNewFromMapNormalizesDOIURL verifies article construction normalizes a DOI URL.
func TestNewFromMapNormalizesDOIURL(t *testing.T) {
	a, err := NewFromMap(map[string]string{
		"doi":   " HTTPS://DOI.ORG/10.1234/ABC.DEF ",
		"title": "Test Article",
		"year":  "2024",
	}, "test")
	if err != nil {
		t.Fatal(err)
	}
	if a.DOI != "10.1234/abc.def" {
		t.Fatalf("unexpected normalized DOI: %q", a.DOI)
	}
}

// TestNewFromMapMissingRequired verifies new from map missing required.
func TestNewFromMapMissingRequired(t *testing.T) {
	_, err := NewFromMap(map[string]string{}, "src")
	if err == nil {
		t.Fatal("expected RequiredFieldError")
	}
	var rfe *RequiredFieldError
	if !as(err, &rfe) {
		t.Fatalf("expected *RequiredFieldError, got %T", err)
	}
}

// TestCheckRequired verifies check required.
func TestCheckRequired(t *testing.T) {
	a := &Article{}
	missing := CheckRequired(a)
	if len(missing) != 3 {
		t.Errorf("expected 3 missing, got %v", missing)
	}
	a.DOI = "10.1234/x"
	a.Title = "x"
	a.Year = 2024
	missing = CheckRequired(a)
	if len(missing) != 0 {
		t.Errorf("expected 0 missing, got %v", missing)
	}
}

// TestArticleToMap verifies article to map.
func TestArticleToMap(t *testing.T) {
	a := &Article{
		DOI:             "10.1234/x",
		Title:           "Test",
		Year:            2024,
		Source:          "src",
		Authors:         []Author{{CitationName: "Doe J."}},
		CitedReferences: []Reference{{Raw: "ref1", DOI: "10.9999/y"}},
		Keywords:        []string{"kw1"},
	}
	d := ArticleToMap(a)
	if d["doi"] != "10.1234/x" {
		t.Errorf("doi = %v", d["doi"])
	}
	authors, ok := d["authors"].([]map[string]any)
	if !ok {
		t.Fatalf("authors type = %T", d["authors"])
	}
	if len(authors) != 1 || authors[0]["citation_name"] != "Doe J." {
		t.Errorf("unexpected authors: %v", authors)
	}
}

// TestMergeBySource verifies merge by source.
func TestMergeBySource(t *testing.T) {
	bySource := map[string][]*Article{
		"src1": {
			{DOI: "10.1/a", Title: "A", Year: 2020},
			{DOI: "10.2/b", Title: "B", Year: 2021},
		},
		"src2": {
			{DOI: "10.1/a", Title: "A dup", Year: 2020}, // duplicate
			{DOI: "10.3/c", Title: "C", Year: 2022},
		},
	}
	unique, dups := MergeBySource(bySource)
	if len(unique) != 3 {
		t.Errorf("expected 3 unique, got %d", len(unique))
	}
	if len(dups) != 1 {
		t.Errorf("expected 1 duplicate, got %d", len(dups))
	}
}

// TestMergeBySourceNoDOIIsUnique verifies merge by source no doi is unique.
func TestMergeBySourceNoDOIIsUnique(t *testing.T) {
	bySource := map[string][]*Article{
		"src1": {{Title: "No DOI", Year: 2020}},
		"src2": {{Title: "No DOI", Year: 2020}},
	}
	unique, dups := MergeBySource(bySource)
	// Both kept because they have no DOI
	if len(unique) != 2 {
		t.Errorf("expected 2 unique (no DOI), got %d", len(unique))
	}
	if len(dups) != 0 {
		t.Errorf("expected 0 duplicates, got %d", len(dups))
	}
}
