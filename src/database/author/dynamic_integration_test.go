// dynamic_integration_test.go verifies the retained identity-evidence builder
// rejects injection attempts, follows its declared ordering, and caps its
// search and page inputs.
//go:build integration

package author_test

import (
	"context"
	"strings"
	"testing"

	"analysis/database/author"
)

// TestStoreListIdentityEvidenceConstrainsDynamicInputs verifies the retained
// identity-evidence builder binds search values, rejects unknown sort and
// order values, follows its declared ordering, and caps oversized inputs.
func TestStoreListIdentityEvidenceConstrainsDynamicInputs(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID, err := db.Run.StartRun(ctx, "author-evidence-dynamic", "")
	if err != nil {
		t.Fatal(err)
	}
	alphaID, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Alpha Evidence"})
	if err != nil {
		t.Fatal(err)
	}
	betaID, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Beta Evidence"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateIdentityResolution(ctx, &author.IdentityResolution{
		PipelineRunID: runID, AuthorOccurrenceID: alphaID, Status: author.IdentityStatusORCIDUnclear,
		Provider: "orcid", QueriedCitationName: "Alpha Evidence", ResolvedAt: "2026-01-01T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateIdentityResolution(ctx, &author.IdentityResolution{
		PipelineRunID: runID, AuthorOccurrenceID: betaID, Status: author.IdentityStatusNoORCIDCandidate,
		Provider: "orcid", QueriedCitationName: `x' OR '1'='1`, ResolvedAt: "2026-01-02T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}

	ascending, err := store.ListIdentityEvidence(ctx, author.IdentityEvidenceFilter{RunID: runID, Sort: "citation_name", Order: "ASC", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(ascending.Items) != 2 || ascending.Items[0].QueriedCitationName != "Alpha Evidence" || ascending.Items[1].QueriedCitationName != `x' OR '1'='1` {
		t.Fatalf("ascending citation order = %+v", ascending.Items)
	}
	descending, err := store.ListIdentityEvidence(ctx, author.IdentityEvidenceFilter{RunID: runID, Sort: "citation_name", Order: "DESC", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(descending.Items) != 2 || descending.Items[0].QueriedCitationName != `x' OR '1'='1` || descending.Items[1].QueriedCitationName != "Alpha Evidence" {
		t.Fatalf("descending citation order = %+v", descending.Items)
	}

	// A malicious search value is bound as a literal LIKE pattern and matches
	// only the resolution that stores the same text.
	literal, err := store.ListIdentityEvidence(ctx, author.IdentityEvidenceFilter{RunID: runID, Query: `' OR '1'='1`, Sort: "id", Order: "ASC", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatal(err)
	}
	if literal.Total != 1 || len(literal.Items) != 1 || literal.Items[0].QueriedCitationName != `x' OR '1'='1` {
		t.Fatalf("literal search = %+v, want only the stored malicious text", literal.Items)
	}

	for _, test := range []struct {
		name   string
		filter author.IdentityEvidenceFilter
	}{
		{"injected sort", author.IdentityEvidenceFilter{RunID: runID, Sort: "id; DROP TABLE author_identity_resolutions; --", Order: "ASC", Page: 1, PerPage: 20}},
		{"injected order", author.IdentityEvidenceFilter{RunID: runID, Sort: "id", Order: "ASC; DROP TABLE author_identity_resolutions; --", Page: 1, PerPage: 20}},
		{"oversized query", author.IdentityEvidenceFilter{RunID: runID, Query: strings.Repeat("x", 201), Sort: "id", Order: "ASC", Page: 1, PerPage: 20}},
		{"oversized page size", author.IdentityEvidenceFilter{RunID: runID, Sort: "id", Order: "ASC", Page: 1, PerPage: 1001}},
		{"zero page", author.IdentityEvidenceFilter{RunID: runID, Sort: "id", Order: "ASC", Page: 0, PerPage: 20}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := store.ListIdentityEvidence(ctx, test.filter); err == nil {
				t.Fatal("expected the retained builder to reject the input")
			}
		})
	}
}
