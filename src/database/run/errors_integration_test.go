// errors_integration_test.go verifies the run family propagates connection
// failures as errors instead of panicking or returning partial results.
//go:build integration

package run_test

import (
	"context"
	"testing"

	"analysis/database/run"
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
		{"StartRun", func() error { _, err := store.StartRun(ctx, "closed", ""); return err }},
		{"GetByID", func() error { _, err := store.GetByID(ctx, 1); return err }},
		{"ListByPlan", func() error { _, err := store.ListByPlan(ctx, 1); return err }},
		{"ListByVisibility", func() error { _, err := store.ListByVisibility(ctx, "active"); return err }},
		{"StartAttempt", func() error {
			_, _, err := store.StartAttempt(ctx, run.StartAttemptInput{ExecutionPlanID: 1, Step: "closed"})
			return err
		}},
		{"StartAttemptIfIdle", func() error {
			_, _, err := store.StartAttemptIfIdle(ctx, run.StartAttemptInput{ExecutionPlanID: 1, Step: "closed"})
			return err
		}},
		{"FinishRun", func() error { return store.FinishRun(ctx, 1, "completed", "") }},
		{"Trash", func() error { return store.Trash(ctx, 1, "reason") }},
		{"Restore", func() error { return store.Restore(ctx, 1) }},
		{"RecoverAbandoned", func() error { return store.RecoverAbandoned(ctx, 1) }},
		{"InsertReviewer", func() error {
			return store.InsertReviewer(ctx, run.ReviewerInput{RunID: 1, Username: "name", Email: "name@example.test"})
		}},
		{"GetReviewer", func() error { _, err := store.GetReviewer(ctx, 1); return err }},
		{"CreateStep", func() error { _, err := store.CreateStep(ctx, 1, "step"); return err }},
		{"UpdateStepStatus", func() error { return store.UpdateStepStatus(ctx, 1, "completed") }},
		{"LinkStepReuse", func() error {
			return store.LinkStepReuse(ctx, run.StepReuseInput{StepID: 1, ReusedFromRunID: 2})
		}},
		{"LinkStepInputArtifact", func() error {
			return store.LinkStepInputArtifact(ctx, run.StepArtifactInput{StepID: 1, ArtifactID: 2})
		}},
		{"LinkStepOutputArtifact", func() error {
			return store.LinkStepOutputArtifact(ctx, run.StepArtifactInput{StepID: 1, ArtifactID: 2})
		}},
		{"SetStepFingerprints", func() error {
			return store.SetStepFingerprints(ctx, run.StepFingerprintInput{StepID: 1, InputFingerprint: "in", OutputFingerprint: "out"})
		}},
		{"ListStepsByRun", func() error { _, err := store.ListStepsByRun(ctx, 1); return err }},
		{"SetMetric", func() error {
			return store.SetMetric(ctx, run.MetricInput{RunID: 1, Metric: "metric", Source: "source", Value: 1})
		}},
		{"GetMetric", func() error { _, err := store.GetMetric(ctx, 1, "metric", "source"); return err }},
		{"ListMetricsByRun", func() error { _, err := store.ListMetricsByRun(ctx, 1); return err }},
		{"CheckPurgeEligibility", func() error { _, err := store.CheckPurgeEligibility(ctx, 1); return err }},
		{"LatestRunWithAncestry", func() error { _, err := store.LatestRunWithAncestry(ctx); return err }},
		{"SelectedAttempt", func() error { _, err := store.SelectedAttempt(ctx, 1, 1); return err }},
		{"RunArtifactContext", func() error { _, err := store.RunArtifactContext(ctx, 1); return err }},
		{"RunContext", func() error { _, err := store.RunContext(ctx, 1); return err }},
		{"GetVisibility", func() error { _, err := store.GetVisibility(ctx, 1); return err }},
		{"Exists", func() error { _, err := store.Exists(ctx, 1); return err }},
		{"HasExecutionPlan", func() error { _, err := store.HasExecutionPlan(ctx, 1); return err }},
		{"ListStepsForViewer", func() error { _, err := store.ListStepsForViewer(ctx, 1); return err }},
		{"ListHierarchyAttempts", func() error {
			_, _, err := store.ListHierarchyAttempts(ctx, run.HierarchyAttemptFilter{ExecutionPlanID: 1, Limit: 10})
			return err
		}},
		{"ListHierarchyRuns", func() error {
			_, _, err := store.ListHierarchyRuns(ctx, run.HierarchyRunFilter{Visibility: "all", Limit: 10})
			return err
		}},
		{"ListLegacyRuns", func() error {
			_, err := store.ListLegacyRuns(ctx, run.LegacyRunFilter{Limit: 10})
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
