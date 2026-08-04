// orcid_unit_test.go tests ORCID response decoding and search URL generation in isolation.
//go:build unit

package enrich

import (
	"strings"
	"testing"
)

// TestDecodeORCIDResponsesAndSearchCandidates verifies decode orcid responses and search candidates.
func TestDecodeORCIDResponsesAndSearchCandidates(t *testing.T) {
	profile := DecodeORCIDRecordResponse([]byte(`{
		"person":{"name":{"given-names":{"value":"Ada"},"family-name":{"value":"Lovelace"},"credit-name":{"value":"Ada Lovelace"}}},
		"orcid":{"path":"0000-0000-0000-0001"}
	}`), "0000-0000-0000-0001")
	if profile == nil || profile.CitationName != "Lovelace, Ada" {
		t.Fatalf("decoded ORCID profile = %+v", profile)
	}
	candidates := DecodeORCIDNameSearchCandidates([]byte(`{"result":[{"orcid-identifier":{"path":"0000-0000-0000-0001"}},{"orcid-identifier":{"path":"0000-0000-0000-0002"}}]}`))
	if len(candidates) != 2 || candidates[0].ORCID != "0000-0000-0000-0001" {
		t.Fatalf("decoded candidates = %+v", candidates)
	}
	urls := ORCIDNameSearchURLs(SourceConfig{ExtraURLs: map[string]string{"search": "https://orcid.example/search"}}, "Lovelace, Ada")
	if len(urls) != 3 || !strings.Contains(urls[0], "given-names") {
		t.Fatalf("search URLs = %v", urls)
	}
}
