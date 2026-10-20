// identity_test.go exercises the uncertain-identity resolution store against a
// database created by the production migration runner.
//go:build integration

package author_test

import (
	"context"
	"testing"

	"analysis/database/author"
)

// TestAuthorIdentityResolutionCreate verifies creating an identity resolution record.
func TestAuthorIdentityResolutionCreate(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()

	runID, err := db.Run.StartRun(ctx, "identity-test", "")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	occurrenceID, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Test, Author"})
	if err != nil {
		t.Fatalf("CreateOccurrence: %v", err)
	}

	resolution := &author.IdentityResolution{
		PipelineRunID:       runID,
		AuthorOccurrenceID:  occurrenceID,
		Status:              author.IdentityStatusConfirmed,
		Provider:            "orcid",
		QueriedCitationName: "Test, Author",
		ResolvedAt:          "2026-07-21T12:00:00Z",
	}
	id, err := store.CreateIdentityResolution(ctx, resolution)
	if err != nil {
		t.Fatalf("CreateIdentityResolution: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero resolution ID")
	}
}

// TestAuthorIdentityResolutionRejectsMissingFields verifies validation.
func TestAuthorIdentityResolutionRejectsMissingFields(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()

	if _, err := store.CreateIdentityResolution(ctx, &author.IdentityResolution{}); err == nil {
		t.Fatal("expected error for missing required fields")
	}
	if _, err := store.CreateIdentityResolution(ctx, nil); err == nil {
		t.Fatal("expected error for nil resolution")
	}
}

// TestAuthorIdentityResolutionRejectsUnknownStatus verifies the status
// vocabulary gates new resolution evidence.
func TestAuthorIdentityResolutionRejectsUnknownStatus(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()

	runID, err := db.Run.StartRun(ctx, "identity-status", "")
	if err != nil {
		t.Fatal(err)
	}
	occurrenceID, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Status, Author"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateIdentityResolution(ctx, &author.IdentityResolution{
		PipelineRunID: runID, AuthorOccurrenceID: occurrenceID, Status: "bogus",
		Provider: "orcid", QueriedCitationName: "Status, Author", ResolvedAt: "2026-07-21T12:00:00Z",
	}); err == nil {
		t.Fatal("expected an unknown status to be rejected")
	}
	if _, err := store.CreateIdentityResolution(ctx, &author.IdentityResolution{
		PipelineRunID: runID, AuthorOccurrenceID: occurrenceID, Status: author.IdentityStatusProviderFailed,
		Provider: " ", QueriedCitationName: "Status, Author", ResolvedAt: "2026-07-21T12:00:00Z",
	}); err == nil {
		t.Fatal("expected a blank provider to be rejected")
	}
}
