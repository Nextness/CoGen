// viewer_integration_test.go exercises the author family's viewer-facing
// run-scoped occurrence and identity evidence reads against a database created
// by the production migration runner.
//go:build integration

package author_test

import (
	"context"
	"testing"

	"analysis/database/author"
	"analysis/database/work"
)

// TestStoreGetOccurrenceForRunPreservesMembershipAndNullableIdentity verifies
// run-scoped membership and the optional person linkage projection.
func TestStoreGetOccurrenceForRunPreservesMembershipAndNullableIdentity(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID, err := db.Run.StartRun(ctx, "author-viewer", "")
	if err != nil {
		t.Fatal(err)
	}
	workID, err := db.Work.CreateWorkByDOI(ctx, "10.1000/author-viewer")
	if err != nil {
		t.Fatal(err)
	}
	revisionID, err := db.Work.CreateRevision(ctx, &work.Revision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: work.ProducerStageNormalize, Title: "Viewer article",
	})
	if err != nil {
		t.Fatal(err)
	}
	linkedID, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Ada Lovelace", ORCID: "0000-0002-1825-0097"})
	if err != nil {
		t.Fatal(err)
	}
	unlinkedID, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Charles Babbage"})
	if err != nil {
		t.Fatal(err)
	}
	for order, occurrenceID := range []int64{linkedID, unlinkedID} {
		if _, err := store.CreateAuthorship(ctx, &author.Authorship{
			WorkRevisionID: revisionID, AuthorOccurrenceID: occurrenceID, AuthorOrder: order + 1,
		}); err != nil {
			t.Fatal(err)
		}
	}

	occurrence, err := store.GetOccurrenceForRun(ctx, linkedID, runID)
	if err != nil || occurrence == nil {
		t.Fatalf("linked occurrence = %+v err=%v", occurrence, err)
	}
	if occurrence.CitationName != "Ada Lovelace" || occurrence.PersonID == nil || occurrence.PersonORCID == nil ||
		*occurrence.PersonORCID != "0000-0002-1825-0097" {
		t.Fatalf("linked occurrence = %+v", occurrence)
	}

	occurrence, err = store.GetOccurrenceForRun(ctx, unlinkedID, runID)
	if err != nil || occurrence == nil {
		t.Fatalf("unlinked occurrence = %+v err=%v", occurrence, err)
	}
	if occurrence.PersonID != nil || occurrence.PersonORCID != nil || occurrence.FirstName != nil || occurrence.LastName != nil || occurrence.ORCID != nil {
		t.Fatalf("unlinked occurrence optional identity = %+v", occurrence)
	}

	if occurrence, err := store.GetOccurrenceForRun(ctx, linkedID, runID+1); err != nil || occurrence != nil {
		t.Fatalf("cross-run occurrence = %+v err=%v, want nil", occurrence, err)
	}
}

// TestStoreIdentityEvidenceStatsCountsRunOutcomes verifies resolution status
// and candidate totals stay scoped to one run.
func TestStoreIdentityEvidenceStatsCountsRunOutcomes(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID, err := db.Run.StartRun(ctx, "author-viewer", "")
	if err != nil {
		t.Fatal(err)
	}
	unclearID, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Unclear Author"})
	if err != nil {
		t.Fatal(err)
	}
	failedID, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Failed Author"})
	if err != nil {
		t.Fatal(err)
	}
	unclearResolutionID, err := store.CreateIdentityResolution(ctx, &author.IdentityResolution{
		PipelineRunID: runID, AuthorOccurrenceID: unclearID,
		Status: author.IdentityStatusORCIDUnclear, Provider: "orcid",
		QueriedCitationName: "Unclear Author", ResolvedAt: "2026-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateIdentityCandidate(ctx, &author.IdentityCandidate{
		IdentityResolutionID: unclearResolutionID, CandidateORCID: "0000-0001-2345-6789",
		QueryURL: "https://orcid.example/search", ProviderRank: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateIdentityResolution(ctx, &author.IdentityResolution{
		PipelineRunID: runID, AuthorOccurrenceID: failedID,
		Status: author.IdentityStatusProviderFailed, Provider: "orcid",
		QueriedCitationName: "Failed Author", ResolvedAt: "2026-01-01T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}

	stats, err := store.IdentityEvidenceStats(ctx, runID)
	if err != nil || stats == nil {
		t.Fatalf("identity evidence stats = %+v err=%v", stats, err)
	}
	if stats.Resolutions != 2 || stats.Unclear != 1 || stats.NoCandidate != 0 || stats.ProviderFailed != 1 || stats.Candidates != 1 {
		t.Fatalf("identity evidence stats = %+v", stats)
	}

	otherStats, err := store.IdentityEvidenceStats(ctx, runID+1)
	if err != nil || otherStats == nil || otherStats.Resolutions != 0 || otherStats.Candidates != 0 {
		t.Fatalf("other-run stats = %+v err=%v", otherStats, err)
	}

	exists, err := store.IdentityResolutionExists(ctx, unclearResolutionID, runID)
	if err != nil || !exists {
		t.Fatalf("identity resolution exists = %v err=%v, want true", exists, err)
	}
	exists, err = store.IdentityResolutionExists(ctx, unclearResolutionID, runID+1)
	if err != nil || exists {
		t.Fatalf("cross-run identity resolution exists = %v err=%v, want false", exists, err)
	}
}
