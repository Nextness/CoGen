// payload_validation_unit_test.go tests ValidateProviderPayload exhaustively.
//go:build unit

package enrich

import "testing"

// TestValidateProviderPayload verifies validate provider payload.
func TestValidateProviderPayload(t *testing.T) {
	tests := []struct {
		name      string
		provider  string
		namespace string
		body      string
		wantErr   bool
		wantMatch string
	}{
		// crossref
		{name: "crossref valid", provider: "crossref", namespace: "work_by_doi", body: `{"message":{}}`, wantErr: false},
		{name: "crossref bad namespace", provider: "crossref", namespace: "wrong", body: `{"message":{}}`, wantErr: true, wantMatch: "unsupported Crossref cache namespace"},
		{name: "crossref missing message", provider: "crossref", namespace: "work_by_doi", body: `{"other":{}}`, wantErr: true, wantMatch: `missing "message" envelope`},
		{name: "crossref message not object", provider: "crossref", namespace: "work_by_doi", body: `{"message":42}`, wantErr: true, wantMatch: `"message" envelope is not an object`},

		// openalex - work_by_doi and author_by_orcid
		{name: "openalex work valid", provider: "openalex", namespace: "work_by_doi", body: `{"id":"W123"}`, wantErr: false},
		{name: "openalex author valid", provider: "openalex", namespace: "author_by_orcid", body: `{"id":"A456"}`, wantErr: false},
		{name: "openalex references valid", provider: "openalex", namespace: "work_references", body: `{"results":[]}`, wantErr: false},
		{name: "openalex bad namespace", provider: "openalex", namespace: "unknown", body: `{"id":"x"}`, wantErr: true, wantMatch: "unsupported OpenAlex cache namespace"},
		{name: "openalex missing id", provider: "openalex", namespace: "work_by_doi", body: `{"other":"x"}`, wantErr: true, wantMatch: `missing "id" envelope`},
		{name: "openalex id not string", provider: "openalex", namespace: "work_by_doi", body: `{"id":123}`, wantErr: true, wantMatch: `"id" envelope is not a non-empty string`},
		{name: "openalex id empty string", provider: "openalex", namespace: "work_by_doi", body: `{"id":""}`, wantErr: true, wantMatch: `"id" envelope is not a non-empty string`},
		{name: "openalex missing results", provider: "openalex", namespace: "work_references", body: `{"other":"x"}`, wantErr: true, wantMatch: `missing "results" envelope`},
		{name: "openalex results not array", provider: "openalex", namespace: "work_references", body: `{"results":{}}`, wantErr: true, wantMatch: `"results" envelope is not an array`},

		// orcid
		{name: "orcid author valid", provider: "orcid", namespace: "author_by_orcid", body: `{"person":{}}`, wantErr: false},
		{name: "orcid name search valid array", provider: "orcid", namespace: "author_name_search", body: `{"result":[]}`, wantErr: false},
		{name: "orcid name search valid null", provider: "orcid", namespace: "author_name_search", body: `{"result":null}`, wantErr: false},
		{name: "orcid bad namespace", provider: "orcid", namespace: "unknown", body: `{"person":{}}`, wantErr: true, wantMatch: "unsupported ORCID cache namespace"},
		{name: "orcid missing person", provider: "orcid", namespace: "author_by_orcid", body: `{"other":"x"}`, wantErr: true, wantMatch: `missing "person" envelope`},
		{name: "orcid person not object", provider: "orcid", namespace: "author_by_orcid", body: `{"person":42}`, wantErr: true, wantMatch: `"person" envelope is not an object`},
		{name: "orcid missing result", provider: "orcid", namespace: "author_name_search", body: `{"other":"x"}`, wantErr: true, wantMatch: `missing "result" envelope`},
		{name: "orcid result not array or null", provider: "orcid", namespace: "author_name_search", body: `{"result":{}}`, wantErr: true, wantMatch: `"result" envelope is not an array or null`},

		// unknown provider
		{name: "unknown provider", provider: "pubmed", namespace: "work_by_doi", body: `{"some":"key"}`, wantErr: true, wantMatch: `unsupported provider "pubmed"`},

		// edge cases
		{name: "empty body", provider: "crossref", namespace: "work_by_doi", body: ``, wantErr: true, wantMatch: "invalid JSON"},
		{name: "non-json body", provider: "crossref", namespace: "work_by_doi", body: `not json`, wantErr: true, wantMatch: "invalid JSON"},
		{name: "empty json object", provider: "crossref", namespace: "work_by_doi", body: `{}`, wantErr: true, wantMatch: "missing provider response envelope"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProviderPayload(tt.provider, tt.namespace, []byte(tt.body))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantMatch != "" {
					if !errorContains(err, tt.wantMatch) {
						t.Fatalf("error %q does not contain %q", err.Error(), tt.wantMatch)
					}
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// errorContains supports the package test suite's error contains setup or assertions.
func errorContains(err error, substr string) bool {
	return len(err.Error()) >= len(substr) && err.Error()[:len(substr)] == substr
}
