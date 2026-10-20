// evidence_test.go exercises the uncertain-identity candidate store against a
// database created by the production migration runner.
//go:build integration

package author_test

import (
	"context"
	"testing"

	"analysis/database/author"
)

// TestAuthorIdentityCandidateCreate verifies creating an identity candidate.
func TestAuthorIdentityCandidateCreate(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()

	runID, err := db.Run.StartRun(ctx, "candidate-test", "")
	if err != nil {
		t.Fatal(err)
	}
	occurrenceID, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Candidate, Test"})
	if err != nil {
		t.Fatal(err)
	}
	resolutionID, err := store.CreateIdentityResolution(ctx, &author.IdentityResolution{
		PipelineRunID:       runID,
		AuthorOccurrenceID:  occurrenceID,
		Status:              author.IdentityStatusORCIDUnclear,
		Provider:            "orcid",
		QueriedCitationName: "Candidate, Test",
		ResolvedAt:          "2026-07-21T12:00:00Z",
	})
	if err != nil {
		t.Fatalf("CreateIdentityResolution: %v", err)
	}

	candidate := &author.IdentityCandidate{
		IdentityResolutionID: resolutionID,
		CandidateORCID:       "0000-0002-1694-233X",
		ProviderDisplayName:  "ORCID",
		QueryURL:             "https://pub.orcid.org/v3.0/0000-0002-1694-233X",
		ProviderRank:         1,
	}
	candidateID, err := store.CreateIdentityCandidate(ctx, candidate)
	if err != nil {
		t.Fatalf("CreateIdentityCandidate: %v", err)
	}
	if candidateID == 0 {
		t.Fatal("expected non-zero candidate ID")
	}
}

// TestAuthorIdentityCandidateRejectsMissingFields verifies candidate validation.
func TestAuthorIdentityCandidateRejectsMissingFields(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()

	if _, err := store.CreateIdentityCandidate(ctx, nil); err == nil {
		t.Fatal("expected error for nil candidate")
	}
	if _, err := store.CreateIdentityCandidate(ctx, &author.IdentityCandidate{}); err == nil {
		t.Fatal("expected error for missing required fields")
	}
	if _, err := store.CreateIdentityCandidate(ctx, &author.IdentityCandidate{
		IdentityResolutionID: 1, CandidateORCID: "0000-0002-1694-233X",
		QueryURL: "https://pub.orcid.org/v3.0/0000-0002-1694-233X", ProviderRank: 0,
	}); err == nil {
		t.Fatal("expected error for a non-positive provider rank")
	}
}

// TestAuthorIdentityCandidateUniqueRankAndORCID verifies the candidate
// uniqueness constraints keep ranked evidence unambiguous.
func TestAuthorIdentityCandidateUniqueRankAndORCID(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()

	runID, err := db.Run.StartRun(ctx, "candidate-unique", "")
	if err != nil {
		t.Fatal(err)
	}
	occurrenceID, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Unique, Candidate"})
	if err != nil {
		t.Fatal(err)
	}
	resolutionID, err := store.CreateIdentityResolution(ctx, &author.IdentityResolution{
		PipelineRunID:       runID,
		AuthorOccurrenceID:  occurrenceID,
		Status:              author.IdentityStatusORCIDUnclear,
		Provider:            "orcid",
		QueriedCitationName: "Unique, Candidate",
		ResolvedAt:          "2026-07-21T12:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateIdentityCandidate(ctx, &author.IdentityCandidate{
		IdentityResolutionID: resolutionID, CandidateORCID: "0000-0002-1694-233X",
		QueryURL: "https://pub.orcid.org/v3.0/0000-0002-1694-233X", ProviderRank: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateIdentityCandidate(ctx, &author.IdentityCandidate{
		IdentityResolutionID: resolutionID, CandidateORCID: "0000-0002-1694-233X",
		QueryURL: "https://pub.orcid.org/v3.0/0000-0002-1694-233X", ProviderRank: 2,
	}); err == nil {
		t.Fatal("expected a duplicate candidate ORCID to fail")
	}
	if _, err := store.CreateIdentityCandidate(ctx, &author.IdentityCandidate{
		IdentityResolutionID: resolutionID, CandidateORCID: "0000-0001-5109-3700",
		QueryURL: "https://pub.orcid.org/v3.0/0000-0001-5109-3700", ProviderRank: 1,
	}); err == nil {
		t.Fatal("expected a duplicate provider rank to fail")
	}
}
