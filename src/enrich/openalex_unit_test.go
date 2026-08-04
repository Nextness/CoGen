// openalex_unit_test.go tests OpenAlex response decoding in isolation.
//go:build unit

package enrich

import "testing"

// TestDecodeOpenAlexResponses verifies decode open alex responses.
func TestDecodeOpenAlexResponses(t *testing.T) {
	article, ids := DecodeOpenAlexResponse([]byte(`{
		"display_name":"OpenAlex title", "cited_by_count":4,
		"abstract_inverted_index":{"Hello":[0],"world":[1]},
		"authorships":[{"author":{"display_name":"Ada Lovelace","orcid":"https://orcid.org/0000-0000-0000-0001"}}],
		"referenced_works":["https://openalex.org/W123"]
	}`), "10.1000/work")
	if article == nil || article.Title != "OpenAlex title" || article.Abstract != "Hello world" || article.CitationCount != 4 {
		t.Fatalf("decoded article = %+v", article)
	}
	if len(ids) != 1 || ids[0] != "W123" || len(article.Authors) != 1 {
		t.Fatalf("reference IDs = %v, authors = %+v", ids, article.Authors)
	}
	references := DecodeOpenAlexReferenceResponse([]byte(`{"results":[{"id":"https://openalex.org/W123","doi":"https://doi.org/10.1000/reference","title":"Reference","publication_year":2023}]}`))
	if reference, ok := references["W123"]; !ok || reference.DOI != "10.1000/reference" || reference.Year != 2023 {
		t.Fatalf("decoded references = %+v", references)
	}
}

// TestDecodeOpenAlexAuthorResponse verifies decode open alex author response.
func TestDecodeOpenAlexAuthorResponse(t *testing.T) {
	t.Run("valid response", func(t *testing.T) {
		body := []byte(`{
			"display_name": "Ada Lovelace",
			"orcid": "https://orcid.org/0000-0000-0000-0001",
			"works_count": 42,
			"cited_by_count": 100,
			"summary_stats": {"h_index": 10, "i10_index": 15},
			"last_known_institutions": [{"display_name": "University of London"}]
		}`)
		author := DecodeOpenAlexAuthorResponse(body, "0000-0000-0000-0001")
		if author == nil {
			t.Fatal("expected non-nil author")
		}
		if author.ORCID != "0000-0000-0000-0001" {
			t.Fatalf("orcid = %q", author.ORCID)
		}
		if author.DisplayName != "Ada Lovelace" {
			t.Fatalf("display_name = %q", author.DisplayName)
		}
		if author.FirstName != "Ada" || author.LastName != "Lovelace" {
			t.Fatalf("name split = %q %q", author.FirstName, author.LastName)
		}
		if author.WorksCount != 42 {
			t.Fatalf("works_count = %d", author.WorksCount)
		}
		if author.CitedByCnt != 100 {
			t.Fatalf("cited_by_count = %d", author.CitedByCnt)
		}
		if author.HIndex != 10 || author.I10Index != 15 {
			t.Fatalf("h_index = %d, i10_index = %d", author.HIndex, author.I10Index)
		}
		if author.Institution != "University of London" {
			t.Fatalf("institution = %q", author.Institution)
		}
		if author.Source != "openalex" {
			t.Fatalf("source = %q", author.Source)
		}
	})

	t.Run("missing fields", func(t *testing.T) {
		body := []byte(`{"display_name": "Ada Lovelace"}`)
		author := DecodeOpenAlexAuthorResponse(body, "0000-0000-0000-0002")
		if author == nil {
			t.Fatal("expected non-nil author")
		}
		if author.DisplayName != "Ada Lovelace" {
			t.Fatalf("display_name = %q", author.DisplayName)
		}
		if author.FirstName != "Ada" || author.LastName != "Lovelace" {
			t.Fatalf("name split = %q %q", author.FirstName, author.LastName)
		}
		if author.WorksCount != 0 {
			t.Fatalf("works_count = %d", author.WorksCount)
		}
		if author.CitedByCnt != 0 {
			t.Fatalf("cited_by_count = %d", author.CitedByCnt)
		}
		if author.HIndex != 0 || author.I10Index != 0 {
			t.Fatalf("h_index = %d, i10_index = %d", author.HIndex, author.I10Index)
		}
		if author.Institution != "" {
			t.Fatalf("institution = %q", author.Institution)
		}
	})

	t.Run("empty orcid", func(t *testing.T) {
		body := []byte(`{"display_name": "Ada Lovelace", "works_count": 5}`)
		author := DecodeOpenAlexAuthorResponse(body, "")
		if author == nil {
			t.Fatal("expected non-nil author")
		}
		if author.ORCID != "" {
			t.Fatalf("orcid = %q", author.ORCID)
		}
		if author.DisplayName != "Ada Lovelace" {
			t.Fatalf("display_name = %q", author.DisplayName)
		}
		if author.WorksCount != 5 {
			t.Fatalf("works_count = %d", author.WorksCount)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		body := []byte(`{invalid json}`)
		author := DecodeOpenAlexAuthorResponse(body, "0000-0000-0000-0003")
		if author != nil {
			t.Fatal("expected nil author for invalid JSON")
		}
	})
}
