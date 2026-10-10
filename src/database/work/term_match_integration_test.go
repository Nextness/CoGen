// term_match_integration_test.go exercises the run term inventory and
// revision-match store against a database created by the production migration
// runner.
//go:build integration

package work_test

import (
	"context"
	"reflect"
	"testing"

	"analysis/database/work"
)

// TestStoreReplaceRunTermDataOrdersTermsDeterministically verifies the
// inventory replacement is idempotent and ordered by ID.
func TestStoreReplaceRunTermDataOrdersTermsDeterministically(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "term-order")

	input := work.ReplaceTermDataInput{
		RunID: runID,
		TermsBySource: map[string][]string{
			"scopus": {"beta", "alpha"},
			"ieee":   {"gamma"},
		},
	}
	if err := store.ReplaceRunTermData(ctx, input); err != nil {
		t.Fatal(err)
	}
	first, err := store.ListRunTerms(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 3 {
		t.Fatalf("run terms = %+v, want three", first)
	}
	if first[0].SourceName != "ieee" || first[1].SourceName != "scopus" || first[2].SourceName != "scopus" {
		t.Fatalf("run term order = %+v, want sorted sources", first)
	}
	if first[1].Term != "beta" || first[2].Term != "alpha" {
		t.Fatalf("run term order = %+v, want caller order within each source", first)
	}

	if err := store.ReplaceRunTermData(ctx, input); err != nil {
		t.Fatalf("idempotent replace: %v", err)
	}
	second, err := store.ListRunTerms(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 3 {
		t.Fatalf("idempotent run terms = %+v, want three", second)
	}

	if absent, err := store.ListRunTerms(ctx, 99999); err != nil || absent != nil {
		t.Fatalf("absent run terms = %+v err=%v, want nil without error", absent, err)
	}
}

// TestStoreReplaceRunTermDataDistinguishesAnEmptyCompletedResult verifies the
// reconciliation marker is written even when the valid result set is empty.
func TestStoreReplaceRunTermDataDistinguishesAnEmptyCompletedResult(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "term-empty")

	reconciled, err := store.HasRunTermData(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if reconciled {
		t.Fatal("unreconciled run reported term data")
	}
	if err := store.ReplaceRunTermData(ctx, work.ReplaceTermDataInput{RunID: runID}); err != nil {
		t.Fatal(err)
	}
	reconciled, err = store.HasRunTermData(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if !reconciled {
		t.Fatal("empty completed reconciliation was not recorded")
	}
	terms, err := store.ListRunTerms(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if terms != nil {
		t.Fatalf("empty run terms = %+v, want nil", terms)
	}
}

// TestStoreGetRevisionMatchesAndBulkGroupByField verifies single and bulk
// match reads group terms by field and order.
func TestStoreGetRevisionMatchesAndBulkGroupByField(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	workID, err := store.CreateWorkByDOI(ctx, "10.1000/term-matches")
	if err != nil {
		t.Fatal(err)
	}
	runID := createTestRun(t, db, "term-matches")
	firstRevision, err := store.CreateRevision(ctx, &work.Revision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: work.ProducerStageNormalize, Title: "first",
	})
	if err != nil {
		t.Fatal(err)
	}
	secondRevision, err := store.CreateRevision(ctx, &work.Revision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: work.ProducerStageNormalize, Title: "second",
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := store.ReplaceRunTermData(ctx, work.ReplaceTermDataInput{
		RunID:         runID,
		TermsBySource: map[string][]string{"scopus": {"alpha"}},
		Matches: map[int64]map[string][]string{
			firstRevision: {"title": {"beta", "alpha"}, "abstract": {"gamma"}},
		},
	}); err != nil {
		t.Fatal(err)
	}

	matches, err := store.GetRevisionMatches(ctx, runID, firstRevision)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{"title": {"alpha", "beta"}, "abstract": {"gamma"}}
	if !reflect.DeepEqual(matches, want) {
		t.Fatalf("revision matches = %v, want %v", matches, want)
	}
	empty, err := store.GetRevisionMatches(ctx, runID, secondRevision)
	if err != nil {
		t.Fatal(err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("unmatched revision = %v, want empty non-nil map", empty)
	}

	bulk, err := store.GetRevisionMatchesBulk(ctx, runID, []int64{firstRevision, secondRevision})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(bulk[firstRevision], want) {
		t.Fatalf("bulk matches = %v, want %v", bulk[firstRevision], want)
	}
	if second, ok := bulk[secondRevision]; ok && len(second) != 0 {
		t.Fatalf("bulk unmatched revision = %v, want absent or empty", second)
	}
	emptyBulk, err := store.GetRevisionMatchesBulk(ctx, runID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if emptyBulk == nil || len(emptyBulk) != 0 {
		t.Fatalf("empty bulk result = %v, want empty non-nil map", emptyBulk)
	}
}

// TestStoreReplaceRunTermDataRollsBackOnInvalidMatch verifies a mid-transaction
// relationship failure leaves the prior inventory and matches intact.
func TestStoreReplaceRunTermDataRollsBackOnInvalidMatch(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	workID, err := store.CreateWorkByDOI(ctx, "10.1000/term-rollback")
	if err != nil {
		t.Fatal(err)
	}
	runID := createTestRun(t, db, "term-rollback")
	revisionID, err := store.CreateRevision(ctx, &work.Revision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: work.ProducerStageNormalize, Title: "rollback",
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := store.ReplaceRunTermData(ctx, work.ReplaceTermDataInput{
		RunID:         runID,
		TermsBySource: map[string][]string{"scopus": {"alpha"}},
		Matches:       map[int64]map[string][]string{revisionID: {"title": {"alpha"}}},
	}); err != nil {
		t.Fatal(err)
	}
	// A missing work revision is a foreign-key failure that INSERT OR IGNORE
	// does not neutralize, so the whole replacement must roll back.
	if err := store.ReplaceRunTermData(ctx, work.ReplaceTermDataInput{
		RunID:         runID,
		TermsBySource: map[string][]string{"scopus": {"beta"}},
		Matches:       map[int64]map[string][]string{99999: {"title": {"beta"}}},
	}); err == nil {
		t.Fatal("expected the missing revision to fail")
	}

	terms, err := store.ListRunTerms(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(terms) != 1 || terms[0].Term != "alpha" {
		t.Fatalf("rolled-back run terms = %+v, want the prior inventory", terms)
	}
	matches, err := store.GetRevisionMatches(ctx, runID, revisionID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(matches, map[string][]string{"title": {"alpha"}}) {
		t.Fatalf("rolled-back matches = %v, want the prior matches", matches)
	}
	reconciled, err := store.HasRunTermData(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if !reconciled {
		t.Fatal("rollback removed the prior reconciliation marker")
	}
}
