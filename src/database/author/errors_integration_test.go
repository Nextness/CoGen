// errors_integration_test.go verifies the author family propagates connection
// failures as errors instead of panicking or returning partial results.
//go:build integration

package author_test

import (
	"context"
	"testing"

	"analysis/database/author"
)

// TestStoreOperationsPropagateConnectionErrors verifies every public store
// operation reports a closed connection as an error rather than a value.
func TestStoreOperationsPropagateConnectionErrors(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		call func() error
	}{
		{"CreatePersonByORCID", func() error {
			_, err := store.CreatePersonByORCID(ctx, "0000-0002-1694-233X")
			return err
		}},
		{"GetPersonByID", func() error { _, err := store.GetPersonByID(ctx, 1); return err }},
		{"GetPersonByORCID", func() error {
			_, err := store.GetPersonByORCID(ctx, "0000-0002-1694-233X")
			return err
		}},
		{"CreateOccurrence", func() error {
			_, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "closed"})
			return err
		}},
		{"GetOccurrenceByID", func() error { _, err := store.GetOccurrenceByID(ctx, 1); return err }},
		{"ListOccurrencesByPersonID", func() error { _, err := store.ListOccurrencesByPersonID(ctx, 1); return err }},
		{"CreateAuthorship", func() error {
			_, err := store.CreateAuthorship(ctx, &author.Authorship{WorkRevisionID: 1, AuthorOccurrenceID: 1, AuthorOrder: 1})
			return err
		}},
		{"ListAuthorshipsByRevisionID", func() error { _, err := store.ListAuthorshipsByRevisionID(ctx, 1); return err }},
		{"ListAuthorshipsByOccurrenceID", func() error { _, err := store.ListAuthorshipsByOccurrenceID(ctx, 1); return err }},
		{"CreateIdentityResolution", func() error {
			_, err := store.CreateIdentityResolution(ctx, &author.IdentityResolution{
				PipelineRunID: 1, AuthorOccurrenceID: 1, Status: author.IdentityStatusConfirmed,
				Provider: "orcid", QueriedCitationName: "closed", ResolvedAt: "2026-07-21T12:00:00Z",
			})
			return err
		}},
		{"CreateIdentityCandidate", func() error {
			_, err := store.CreateIdentityCandidate(ctx, &author.IdentityCandidate{
				IdentityResolutionID: 1, CandidateORCID: "0000-0002-1694-233X",
				QueryURL: "https://pub.orcid.org/v3.0/0000-0002-1694-233X", ProviderRank: 1,
			})
			return err
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil {
				t.Fatal("closed connection did not produce an error")
			}
		})
	}
}
