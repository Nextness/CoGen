// crossref_unit_test.go tests Crossref response decoding in isolation.
//go:build unit

package enrich

import "testing"

// TestDecodeCrossrefResponse verifies decode crossref response.
func TestDecodeCrossrefResponse(t *testing.T) {
	article := DecodeCrossrefResponse([]byte(`{
		"message": {
			"title":["Crossref title"], "publisher":"Publisher",
			"author":[{"given":"Ada","family":"Lovelace","ORCID":"https://orcid.org/0000-0000-0000-0001"}],
			"reference":[{"DOI":"10.1000/REFERENCE","year":"2024"}]
		}
	}`), "10.1000/work")
	if article == nil || article.Title != "Crossref title" || article.Publisher != "Publisher" {
		t.Fatalf("decoded article = %+v", article)
	}
	if len(article.Authors) != 1 || article.Authors[0].ORCID != "0000-0000-0000-0001" {
		t.Fatalf("decoded authors = %+v", article.Authors)
	}
	if len(article.References) != 1 || article.References[0].DOI != "10.1000/reference" || article.References[0].Year != 2024 {
		t.Fatalf("decoded references = %+v", article.References)
	}
}
