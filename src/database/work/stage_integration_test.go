// stage_integration_test.go exercises the run-work-stage outcome store against
// a database created by the production migration runner.
//go:build integration

package work_test

import (
	"context"
	"testing"

	"analysis/database/work"
)

// TestStoreSetStageOutcomeUpsertsAndPreservesIdentity verifies the unique
// (run, work, stage) row identity survives progressive outcome updates.
func TestStoreSetStageOutcomeUpsertsAndPreservesIdentity(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	workID, err := store.CreateWorkByDOI(ctx, "10.1000/stage-upsert")
	if err != nil {
		t.Fatal(err)
	}
	runID := createTestRun(t, db, "stage-upsert")

	if err := store.SetStageOutcome(ctx, work.StageOutcomeInput{
		RunID: runID, WorkID: workID, StageName: work.StageNameParse, Outcome: work.OutcomePending,
	}); err != nil {
		t.Fatal(err)
	}
	first, err := store.GetStageOutcome(ctx, runID, workID, work.StageNameParse)
	if err != nil {
		t.Fatal(err)
	}
	if first == nil || first.Outcome != work.OutcomePending || first.Reason != "" {
		t.Fatalf("first stage outcome = %+v", first)
	}

	if err := store.SetStageOutcome(ctx, work.StageOutcomeInput{
		RunID: runID, WorkID: workID, StageName: work.StageNameParse, Outcome: work.OutcomeParsed, Reason: "done",
	}); err != nil {
		t.Fatal(err)
	}
	second, err := store.GetStageOutcome(ctx, runID, workID, work.StageNameParse)
	if err != nil {
		t.Fatal(err)
	}
	if second == nil {
		t.Fatal("stage outcome missing after update")
	}
	if second.ID != first.ID || second.CreatedAt != first.CreatedAt {
		t.Fatalf("upsert changed row identity: first=%+v second=%+v", first, second)
	}
	if second.Outcome != work.OutcomeParsed || second.Reason != "done" {
		t.Fatalf("updated stage outcome = %+v", second)
	}
	if missing, err := store.GetStageOutcome(ctx, runID, workID, work.StageNameValidate); err != nil || missing != nil {
		t.Fatalf("missing stage outcome = %+v err=%v, want nil without error", missing, err)
	}
}

// TestStoreStageOutcomesScopeAndOrder verifies run and work scoping plus
// declared ordering.
func TestStoreStageOutcomesScopeAndOrder(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	firstWork, err := store.CreateWorkByDOI(ctx, "10.1000/stage-scope-first")
	if err != nil {
		t.Fatal(err)
	}
	secondWork, err := store.CreateWorkByDOI(ctx, "10.1000/stage-scope-second")
	if err != nil {
		t.Fatal(err)
	}
	firstRun := createTestRun(t, db, "stage-scope-first")
	secondRun := createTestRun(t, db, "stage-scope-second")

	if err := store.SetStageOutcome(ctx, work.StageOutcomeInput{
		RunID: firstRun, WorkID: firstWork, StageName: work.StageNameParse, Outcome: work.OutcomeParsed,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetStageOutcome(ctx, work.StageOutcomeInput{
		RunID: firstRun, WorkID: firstWork, StageName: work.StageNameValidate, Outcome: work.OutcomeValid,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetStageOutcome(ctx, work.StageOutcomeInput{
		RunID: secondRun, WorkID: secondWork, StageName: work.StageNameParse, Outcome: work.OutcomeParsed,
	}); err != nil {
		t.Fatal(err)
	}

	byRun, err := store.ListStageOutcomesByRun(ctx, firstRun)
	if err != nil {
		t.Fatal(err)
	}
	if len(byRun) != 2 || byRun[0].StageName != work.StageNameParse || byRun[1].StageName != work.StageNameValidate {
		t.Fatalf("stage outcomes by run = %+v, want ID order", byRun)
	}
	byWork, err := store.ListStageOutcomesByWork(ctx, firstWork)
	if err != nil {
		t.Fatal(err)
	}
	if len(byWork) != 2 {
		t.Fatalf("stage outcomes by work = %+v, want two", byWork)
	}
	if absent, err := store.ListStageOutcomesByRun(ctx, 99999); err != nil || absent != nil {
		t.Fatalf("absent stage list = %+v err=%v, want nil without error", absent, err)
	}

	count, err := store.CountStageOutcomes(ctx, firstRun, work.StageNameParse, work.OutcomeParsed)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("parsed count = %d, want 1", count)
	}
	count, err = store.CountStageOutcomes(ctx, firstRun, work.StageNameValidate, work.OutcomeValid)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("valid count = %d, want 1", count)
	}
}

// TestStoreSetStageOutcomeValidatesVocabulary verifies invalid stage and
// outcome pairs are rejected before any write.
func TestStoreSetStageOutcomeValidatesVocabulary(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	workID, err := store.CreateWorkByDOI(ctx, "10.1000/stage-validation")
	if err != nil {
		t.Fatal(err)
	}
	runID := createTestRun(t, db, "stage-validation")

	cases := []struct {
		name  string
		input work.StageOutcomeInput
	}{
		{"empty stage", work.StageOutcomeInput{RunID: runID, WorkID: workID, Outcome: work.OutcomeParsed}},
		{"empty outcome", work.StageOutcomeInput{RunID: runID, WorkID: workID, StageName: work.StageNameParse}},
		{"unknown stage", work.StageOutcomeInput{RunID: runID, WorkID: workID, StageName: "bogus", Outcome: work.OutcomeParsed}},
		{"invalid combination", work.StageOutcomeInput{RunID: runID, WorkID: workID, StageName: work.StageNameParse, Outcome: work.OutcomeValid}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if err := store.SetStageOutcome(ctx, test.input); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	outcomes, err := store.ListStageOutcomesByRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if outcomes != nil {
		t.Fatalf("invalid inputs persisted rows: %+v", outcomes)
	}
}
