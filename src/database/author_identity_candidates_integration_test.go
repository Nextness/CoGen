// Integration tests for author identity resolution and candidate repositories.
//go:build integration

package database

import (
	"testing"
)

// TestAuthorIdentityResolutionCreate verifies creating an identity resolution record.
func TestAuthorIdentityResolutionCreate(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	runID, err := db.PipelineRuns.StartRun("identity-test", "")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	aoID, err := db.AuthorOccs.Create(&AuthorOccurrence{CitationName: "Test, Author"})
	if err != nil {
		t.Fatalf("Create occurrence: %v", err)
	}

	res := &AuthorIdentityResolution{
		PipelineRunID:       runID,
		AuthorOccurrenceID:  aoID,
		Status:              AuthorIdentityStatusConfirmed,
		Provider:            "orcid",
		QueriedCitationName: "Test, Author",
		ResolvedAt:          "2026-07-21T12:00:00Z",
	}
	id, err := db.IdentityResolutions.Create(res)
	if err != nil {
		t.Fatalf("Create resolution: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero resolution ID")
	}
}

// TestAuthorIdentityResolutionRejectsMissingFields verifies validation.
func TestAuthorIdentityResolutionRejectsMissingFields(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	_, err := db.IdentityResolutions.Create(&AuthorIdentityResolution{})
	if err == nil {
		t.Fatal("expected error for missing required fields")
	}
	_, err = db.IdentityResolutions.Create(nil)
	if err == nil {
		t.Fatal("expected error for nil resolution")
	}
}

// TestAuthorIdentityCandidateCreate verifies creating an identity candidate.
func TestAuthorIdentityCandidateCreate(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	runID, _ := db.PipelineRuns.StartRun("candidate-test", "")
	aoID, _ := db.AuthorOccs.Create(&AuthorOccurrence{CitationName: "Candidate, Test"})

	res := &AuthorIdentityResolution{
		PipelineRunID:       runID,
		AuthorOccurrenceID:  aoID,
		Status:              AuthorIdentityStatusORCIDUnclear,
		Provider:            "orcid",
		QueriedCitationName: "Candidate, Test",
		ResolvedAt:          "2026-07-21T12:00:00Z",
	}
	resID, err := db.IdentityResolutions.Create(res)
	if err != nil {
		t.Fatalf("Create resolution: %v", err)
	}

	cand := &AuthorIdentityCandidate{
		IdentityResolutionID: resID,
		CandidateORCID:       "0000-0002-1694-233X",
		ProviderDisplayName:  "ORCID",
		QueryURL:             "https://pub.orcid.org/v3.0/0000-0002-1694-233X",
		ProviderRank:         1,
	}
	candID, err := db.IdentityCandidates.Create(cand)
	if err != nil {
		t.Fatalf("Create candidate: %v", err)
	}
	if candID == 0 {
		t.Fatal("expected non-zero candidate ID")
	}
}

// TestAuthorIdentityStatusConstantsAreValid — extracted to author_identity_candidates_unit_test.go
// (deliberately empty — no integration tests remain)
