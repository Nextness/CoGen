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

// TestStoreListIdentityEvidenceSearchesSortsAndPages verifies the run-scoped
// evidence page resolves the current normalized revision, binds the search
// predicate, follows its declared ordering, and reports a total.
func TestStoreListIdentityEvidenceSearchesSortsAndPages(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID, err := db.Run.StartRun(ctx, "author-evidence-list", "")
	if err != nil {
		t.Fatal(err)
	}
	workID, err := db.Work.CreateWorkByDOI(ctx, "10.1000/author-evidence-list")
	if err != nil {
		t.Fatal(err)
	}
	revisionID, err := db.Work.CreateRevision(ctx, &work.Revision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: work.ProducerStageNormalize, Title: "Evidence article",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.RunWorkStages.SetOutcome(runID, workID, work.StageNameValidate, work.OutcomeValid, ""); err != nil {
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
	for order, occurrenceID := range []int64{alphaID, betaID} {
		if _, err := store.CreateAuthorship(ctx, &author.Authorship{WorkRevisionID: revisionID, AuthorOccurrenceID: occurrenceID, AuthorOrder: order + 1}); err != nil {
			t.Fatal(err)
		}
	}
	alphaResolutionID, err := store.CreateIdentityResolution(ctx, &author.IdentityResolution{
		PipelineRunID: runID, AuthorOccurrenceID: alphaID, Status: author.IdentityStatusORCIDUnclear,
		Provider: "orcid", QueriedCitationName: "Alpha Evidence", ResolvedAt: "2026-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateIdentityResolution(ctx, &author.IdentityResolution{
		PipelineRunID: runID, AuthorOccurrenceID: betaID, Status: author.IdentityStatusNoORCIDCandidate,
		Provider: "orcid", QueriedCitationName: "Beta Evidence", ResolvedAt: "2026-01-02T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}

	page, err := store.ListIdentityEvidence(ctx, author.IdentityEvidenceFilter{RunID: runID, Sort: "id", Order: "ASC", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || len(page.Items) != 2 {
		t.Fatalf("identity evidence = %d items total=%d, want 2", len(page.Items), page.Total)
	}
	if page.Items[0].WorkRevisionID == nil || *page.Items[0].WorkRevisionID != revisionID || page.Items[0].ArticleTitle == nil || *page.Items[0].ArticleTitle != "Evidence article" {
		t.Fatalf("resolved evidence = %+v, want revision %d", page.Items[0], revisionID)
	}

	filtered, err := store.ListIdentityEvidence(ctx, author.IdentityEvidenceFilter{RunID: runID, Query: "beta", Sort: "citation_name", Order: "DESC", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatal(err)
	}
	if filtered.Total != 1 || len(filtered.Items) != 1 || filtered.Items[0].QueriedCitationName != "Beta Evidence" {
		t.Fatalf("filtered identity evidence = %+v", filtered.Items)
	}
	if _, err := store.ListIdentityEvidence(ctx, author.IdentityEvidenceFilter{RunID: runID, Sort: "unknown", Order: "ASC", Page: 1, PerPage: 20}); err == nil {
		t.Fatal("expected an unsupported sort error")
	}

	previews, err := store.ListCandidatePreviews(ctx, []int64{alphaResolutionID}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(previews) != 0 {
		t.Fatalf("previews without candidates = %+v, want none", previews)
	}
}

// TestStoreListIdentityCandidatesCursorsAndPreviews verifies ranked candidate
// ordering, cursor traversal, lookahead, and bounded per-resolution previews.
func TestStoreListIdentityCandidatesCursorsAndPreviews(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID, err := db.Run.StartRun(ctx, "author-candidates", "")
	if err != nil {
		t.Fatal(err)
	}
	occurrenceID, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Candidate Author"})
	if err != nil {
		t.Fatal(err)
	}
	resolutionID, err := store.CreateIdentityResolution(ctx, &author.IdentityResolution{
		PipelineRunID: runID, AuthorOccurrenceID: occurrenceID, Status: author.IdentityStatusORCIDUnclear,
		Provider: "orcid", QueriedCitationName: "Candidate Author", ResolvedAt: "2026-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	otherOccurrenceID, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Other Candidate Author"})
	if err != nil {
		t.Fatal(err)
	}
	otherResolutionID, err := store.CreateIdentityResolution(ctx, &author.IdentityResolution{
		PipelineRunID: runID, AuthorOccurrenceID: otherOccurrenceID, Status: author.IdentityStatusORCIDUnclear,
		Provider: "orcid", QueriedCitationName: "Other Candidate Author", ResolvedAt: "2026-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	for rank := 1; rank <= 5; rank++ {
		if _, err := store.CreateIdentityCandidate(ctx, &author.IdentityCandidate{
			IdentityResolutionID: resolutionID, CandidateORCID: "0000-0000-0000-" + string(rune('0'+rank)),
			QueryURL: "https://orcid.example/search", ProviderRank: rank,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.CreateIdentityCandidate(ctx, &author.IdentityCandidate{
		IdentityResolutionID: otherResolutionID, CandidateORCID: "0000-0000-0000-9000",
		QueryURL: "https://orcid.example/search", ProviderRank: 1,
	}); err != nil {
		t.Fatal(err)
	}

	first, err := store.ListIdentityCandidates(ctx, author.IdentityCandidateFilter{ResolutionID: resolutionID, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 3 || first[0].ProviderRank == nil || *first[0].ProviderRank != 1 || first[2].ProviderRank == nil || *first[2].ProviderRank != 3 {
		t.Fatalf("first candidate page = %+v, want 3 lookahead rows ranked 1..3", first)
	}
	next, err := store.ListIdentityCandidates(ctx, author.IdentityCandidateFilter{
		ResolutionID: resolutionID, CursorRank: *first[1].ProviderRank, CursorID: first[1].ID, Limit: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(next) != 3 || next[0].ProviderRank == nil || *next[0].ProviderRank != 3 {
		t.Fatalf("cursor candidate page = %+v, want ranks 3..5", next)
	}

	previews, err := store.ListCandidatePreviews(ctx, []int64{resolutionID, otherResolutionID}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(previews) != 4 {
		t.Fatalf("candidate previews = %d, want 3 plus 1", len(previews))
	}
	if previews[0].ResolutionID != resolutionID || previews[3].ResolutionID != otherResolutionID {
		t.Fatalf("candidate preview order = %+v", previews)
	}
	if _, err := store.ListIdentityCandidates(ctx, author.IdentityCandidateFilter{ResolutionID: resolutionID, Limit: 0}); err == nil {
		t.Fatal("expected a limit validation error")
	}
}

// TestStoreListAuthorIdentityEvidenceCarriesSameNameEvidence verifies the
// author detail evidence page applies the same-name revision membership rule,
// orders by descending resolution ID, and reports exact totals and cursors.
func TestStoreListAuthorIdentityEvidenceCarriesSameNameEvidence(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID, err := db.Run.StartRun(ctx, "author-detail-evidence", "")
	if err != nil {
		t.Fatal(err)
	}
	workID, err := db.Work.CreateWorkByDOI(ctx, "10.1000/author-detail-evidence")
	if err != nil {
		t.Fatal(err)
	}
	firstRevisionID, err := db.Work.CreateRevision(ctx, &work.Revision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: work.ProducerStageNormalize, Title: "First revision",
	})
	if err != nil {
		t.Fatal(err)
	}
	secondRevisionID, err := db.Work.CreateRevision(ctx, &work.Revision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: work.ProducerStageNormalize, Title: "Second revision",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.RunWorkStages.SetOutcome(runID, workID, work.StageNameValidate, work.OutcomeValid, ""); err != nil {
		t.Fatal(err)
	}
	firstOccurrenceID, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Shared Evidence"})
	if err != nil {
		t.Fatal(err)
	}
	secondOccurrenceID, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Shared Evidence"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateAuthorship(ctx, &author.Authorship{WorkRevisionID: firstRevisionID, AuthorOccurrenceID: firstOccurrenceID, AuthorOrder: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateAuthorship(ctx, &author.Authorship{WorkRevisionID: secondRevisionID, AuthorOccurrenceID: secondOccurrenceID, AuthorOrder: 1}); err != nil {
		t.Fatal(err)
	}
	firstResolutionID, err := store.CreateIdentityResolution(ctx, &author.IdentityResolution{
		PipelineRunID: runID, AuthorOccurrenceID: firstOccurrenceID, Status: author.IdentityStatusORCIDUnclear,
		Provider: "orcid", QueriedCitationName: "Shared Evidence", ResolvedAt: "2026-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	secondResolutionID, err := store.CreateIdentityResolution(ctx, &author.IdentityResolution{
		PipelineRunID: runID, AuthorOccurrenceID: secondOccurrenceID, Status: author.IdentityStatusORCIDUnclear,
		Provider: "orcid", QueriedCitationName: "Shared Evidence", ResolvedAt: "2026-01-02T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}

	page, err := store.ListAuthorIdentityEvidence(ctx, author.AuthorIdentityEvidenceFilter{
		RunID: runID, AuthorOccurrenceID: secondOccurrenceID, Limit: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || len(page.Items) != 2 || page.Items[0].ResolutionID != secondResolutionID || page.Items[1].ResolutionID != firstResolutionID {
		t.Fatalf("author identity evidence = %+v total=%d, want resolutions %d then %d", page.Items, page.Total, secondResolutionID, firstResolutionID)
	}

	firstPage, err := store.ListAuthorIdentityEvidence(ctx, author.AuthorIdentityEvidenceFilter{
		RunID: runID, AuthorOccurrenceID: secondOccurrenceID, Limit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !firstPage.HasMore || firstPage.NextCursorID != secondResolutionID {
		t.Fatalf("author identity evidence first page = %+v", firstPage)
	}
	nextPage, err := store.ListAuthorIdentityEvidence(ctx, author.AuthorIdentityEvidenceFilter{
		RunID: runID, AuthorOccurrenceID: secondOccurrenceID, CursorID: firstPage.NextCursorID, Limit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if nextPage.HasMore || len(nextPage.Items) != 1 || nextPage.Items[0].ResolutionID != firstResolutionID {
		t.Fatalf("author identity evidence next page = %+v", nextPage.Items)
	}

	earlier, err := store.ListAuthorIdentityEvidence(ctx, author.AuthorIdentityEvidenceFilter{
		RunID: runID, AuthorOccurrenceID: firstOccurrenceID, Limit: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if earlier.Total != 1 || len(earlier.Items) != 1 || earlier.Items[0].ResolutionID != firstResolutionID {
		t.Fatalf("earlier occurrence evidence = %+v total=%d, want only its own resolution", earlier.Items, earlier.Total)
	}
}
