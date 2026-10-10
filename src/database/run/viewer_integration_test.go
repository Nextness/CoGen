// viewer_integration_test.go exercises the run family's viewer-facing ancestry,
// lifecycle, attempt, and step reads against a database created by the
// production migration runner.
//go:build integration

package run_test

import (
	"context"
	"testing"

	"analysis/database/run"
)

// TestStoreViewerRunReadsPreserveAncestryAndLifecycle verifies the canonical
// run context, artifact context, existence probes, and lifecycle projection.
func TestStoreViewerRunReadsPreserveAncestryAndLifecycle(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	planID := createTestPlan(t, db, "viewer-context")

	runID, attempt, err := store.StartAttempt(ctx, run.StartAttemptInput{ExecutionPlanID: planID, Step: "viewer", SearchQuery: "q"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FinishRun(ctx, runID, "completed", "done"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec("INSERT INTO review_contexts (pipeline_run_id) VALUES (?)", runID); err != nil {
		t.Fatal(err)
	}

	ancestry, err := store.LatestRunWithAncestry(ctx)
	if err != nil || ancestry == nil {
		t.Fatalf("latest ancestry = %+v err=%v", ancestry, err)
	}
	if ancestry.ID != runID || ancestry.Status != "completed" || ancestry.VisibilityState != "active" {
		t.Fatalf("latest ancestry lifecycle = %+v", ancestry)
	}
	if ancestry.AttemptNumber == nil || *ancestry.AttemptNumber != attempt {
		t.Fatalf("latest ancestry attempt = %v, want %d", ancestry.AttemptNumber, attempt)
	}
	if ancestry.SearchName != "run-family-viewer-context" || ancestry.RevisionLabel != "v1" ||
		ancestry.SearchID == nil || ancestry.SearchRevisionID == nil || ancestry.ExecutionPlanID == nil {
		t.Fatalf("latest ancestry lineage = %+v", ancestry)
	}

	artifactContext, err := store.RunArtifactContext(ctx, runID)
	if err != nil || artifactContext == nil {
		t.Fatalf("artifact context = %+v err=%v", artifactContext, err)
	}
	if artifactContext.RunID != runID || artifactContext.SearchID != "run-family-viewer-context" ||
		artifactContext.SearchRevisionLabel != "v1" || artifactContext.ExecutionPlanID != planID {
		t.Fatalf("artifact context = %+v", artifactContext)
	}

	context, err := store.RunContext(ctx, runID)
	if err != nil || context == nil {
		t.Fatalf("run context = %+v err=%v", context, err)
	}
	if context.Search.SearchID != "run-family-viewer-context" || context.Revision.Label != "v1" ||
		context.Plan.ID != planID || context.Run.ID != runID || context.Run.Status != "completed" {
		t.Fatalf("run context = %+v", context)
	}
	if context.Run.FinishedAt == nil || context.Run.Summary == nil || *context.Run.Summary != "done" {
		t.Fatalf("run context terminal fields = %+v", context.Run)
	}
	if context.ReviewContextID == nil {
		t.Fatal("run context did not report the initialized review context")
	}

	exists, err := store.Exists(ctx, runID)
	if err != nil || !exists {
		t.Fatalf("exists = %v err=%v, want true", exists, err)
	}
	hasPlan, err := store.HasExecutionPlan(ctx, runID)
	if err != nil || !hasPlan {
		t.Fatalf("has execution plan = %v err=%v, want true", hasPlan, err)
	}
	visibility, err := store.GetVisibility(ctx, runID)
	if err != nil || visibility == nil || visibility.Status != "completed" || visibility.VisibilityState != "active" {
		t.Fatalf("visibility = %+v err=%v", visibility, err)
	}
}

// TestStoreViewerRunReadsReturnNilForMissingIdentities verifies absent runs
// preserve the family's not-found contract.
func TestStoreViewerRunReadsReturnNilForMissingIdentities(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()

	if ancestry, err := store.LatestRunWithAncestry(ctx); err != nil || ancestry != nil {
		t.Fatalf("latest ancestry = %+v err=%v, want nil", ancestry, err)
	}
	if context, err := store.RunContext(ctx, 999); err != nil || context != nil {
		t.Fatalf("run context = %+v err=%v, want nil", context, err)
	}
	if artifactContext, err := store.RunArtifactContext(ctx, 999); err != nil || artifactContext != nil {
		t.Fatalf("artifact context = %+v err=%v, want nil", artifactContext, err)
	}
	if visibility, err := store.GetVisibility(ctx, 999); err != nil || visibility != nil {
		t.Fatalf("visibility = %+v err=%v, want nil", visibility, err)
	}
	if exists, err := store.Exists(ctx, 999); err != nil || exists {
		t.Fatalf("exists = %v err=%v, want false", exists, err)
	}
	if hasPlan, err := store.HasExecutionPlan(ctx, 999); err != nil || hasPlan {
		t.Fatalf("has execution plan = %v err=%v, want false", hasPlan, err)
	}
}

// TestStoreSelectedAttemptHonorsPlanAndVisibility verifies exact plan
// membership and trashed exclusion for one selected attempt.
func TestStoreSelectedAttemptHonorsPlanAndVisibility(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	planID := createTestPlan(t, db, "viewer-attempt")
	otherPlanID := createTestPlan(t, db, "viewer-attempt-other")

	runID, attempt, err := store.StartAttempt(ctx, run.StartAttemptInput{ExecutionPlanID: planID, Step: "viewer", SearchQuery: "q"})
	if err != nil {
		t.Fatal(err)
	}
	selected, err := store.SelectedAttempt(ctx, planID, runID)
	if err != nil || selected == nil {
		t.Fatalf("selected attempt = %+v err=%v", selected, err)
	}
	if selected.AttemptNumber == nil || *selected.AttemptNumber != attempt || selected.VisibilityState != "active" {
		t.Fatalf("selected attempt = %+v", selected)
	}
	if selected, err := store.SelectedAttempt(ctx, otherPlanID, runID); err != nil || selected != nil {
		t.Fatalf("cross-plan selected attempt = %+v err=%v, want nil", selected, err)
	}

	if err := store.Trash(ctx, runID, "fixture"); err != nil {
		t.Fatal(err)
	}
	if selected, err := store.SelectedAttempt(ctx, planID, runID); err != nil || selected != nil {
		t.Fatalf("trashed selected attempt = %+v err=%v, want nil", selected, err)
	}
}

// TestStoreListStepsForViewerDerivesDuration verifies the stage timeline keeps
// nullable timing and derives a duration only when both timestamps exist.
func TestStoreListStepsForViewerDerivesDuration(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	planID := createTestPlan(t, db, "viewer-steps")

	runID, _, err := store.StartAttempt(ctx, run.StartAttemptInput{ExecutionPlanID: planID, Step: "viewer", SearchQuery: "q"})
	if err != nil {
		t.Fatal(err)
	}
	completedStep, err := store.CreateStep(ctx, runID, "preflight")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateStepStatus(ctx, completedStep, "completed"); err != nil {
		t.Fatal(err)
	}
	pendingStep, err := store.CreateStep(ctx, runID, "pending")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetStepFingerprints(ctx, run.StepFingerprintInput{StepID: pendingStep, InputFingerprint: "in", OutputFingerprint: "out"}); err != nil {
		t.Fatal(err)
	}

	steps, err := store.ListStepsForViewer(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 {
		t.Fatalf("viewer steps = %d, want 2", len(steps))
	}
	if steps[0].StepName != "preflight" || steps[0].StartedAt == nil || steps[0].FinishedAt == nil || steps[0].DurationSeconds == nil {
		t.Fatalf("completed step = %+v", steps[0])
	}
	if *steps[0].DurationSeconds < 0 {
		t.Fatalf("completed step duration = %v", *steps[0].DurationSeconds)
	}
	if steps[1].StepName != "pending" || steps[1].StartedAt == nil || steps[1].FinishedAt != nil || steps[1].DurationSeconds != nil {
		t.Fatalf("pending step = %+v", steps[1])
	}
	if steps[1].InputFingerprint != "in" || steps[1].OutputFingerprint != "out" {
		t.Fatalf("pending step fingerprints = %+v", steps[1])
	}
}
