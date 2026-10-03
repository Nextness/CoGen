// step_integration_test.go exercises the run-step store against a database
// created by the production migration runner.
//go:build integration

package run_test

import (
	"context"
	"regexp"
	"testing"

	"analysis/database/run"
)

// stepTimestampPattern matches the microsecond-precision persisted stage format.
var stepTimestampPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}Z$`)

// TestStoreStepLifecyclePreservesStatusAndTimestamps verifies stage validation, terminal finish times, ordering, and empty-list identity.
func TestStoreStepLifecyclePreservesStatusAndTimestamps(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()
	runID, err := store.StartRun(ctx, "steps", "")
	if err != nil {
		t.Fatal(err)
	}

	first, err := store.CreateStep(ctx, runID, "first")
	if err != nil {
		t.Fatalf("create first step: %v", err)
	}
	second, err := store.CreateStep(ctx, runID, "second")
	if err != nil {
		t.Fatalf("create second step: %v", err)
	}

	if err := store.UpdateStepStatus(ctx, first, "running"); err != nil {
		t.Fatal(err)
	}
	running, err := store.ListStepsByRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(running) != 2 || running[0].ID != first || running[1].ID != second {
		t.Fatalf("step list = %+v, want ids [%d %d]", running, first, second)
	}
	if running[0].StepStatus != "running" || running[0].FinishedAt != "" {
		t.Fatalf("running step = %+v, want no finished_at", running[0])
	}
	if !stepTimestampPattern.MatchString(running[0].StartedAt) {
		t.Fatalf("started_at = %q, want microsecond precision", running[0].StartedAt)
	}

	if err := store.UpdateStepStatus(ctx, first, "bogus"); err == nil {
		t.Fatal("invalid stage outcome was accepted")
	}
	if err := store.UpdateStepStatus(ctx, first, "completed"); err != nil {
		t.Fatal(err)
	}
	completed, err := store.ListStepsByRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if completed[0].StepStatus != "completed" || !stepTimestampPattern.MatchString(completed[0].FinishedAt) {
		t.Fatalf("completed step = %+v", completed[0])
	}

	empty, err := store.ListStepsByRun(ctx, 999)
	if err != nil || empty != nil {
		t.Fatalf("empty step list = %+v err=%v, want nil without error", empty, err)
	}
}

// TestStoreStepLinksAndFingerprintsPreserveReuseEvidence verifies input/output artifact links, reuse transition, and immutable fingerprints.
func TestStoreStepLinksAndFingerprintsPreserveReuseEvidence(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID, err := store.StartRun(ctx, "step-links", "")
	if err != nil {
		t.Fatal(err)
	}
	priorRunID, err := store.StartRun(ctx, "step-reuse-source", "")
	if err != nil {
		t.Fatal(err)
	}
	inputArtifactID, err := db.Artifacts.Create("step-input-hash", "application/json", 10)
	if err != nil {
		t.Fatal(err)
	}
	outputArtifactID, err := db.Artifacts.Create("step-output-hash", "application/json", 20)
	if err != nil {
		t.Fatal(err)
	}

	stepID, err := store.CreateStep(ctx, runID, "parse")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.LinkStepInputArtifact(ctx, run.StepArtifactInput{StepID: stepID, ArtifactID: inputArtifactID}); err != nil {
		t.Fatal(err)
	}
	if err := store.LinkStepOutputArtifact(ctx, run.StepArtifactInput{StepID: stepID, ArtifactID: outputArtifactID}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetStepFingerprints(ctx, run.StepFingerprintInput{
		StepID:            stepID,
		InputFingerprint:  "input-fingerprint",
		OutputFingerprint: "output-fingerprint",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.LinkStepReuse(ctx, run.StepReuseInput{StepID: stepID, ReusedFromRunID: priorRunID}); err != nil {
		t.Fatal(err)
	}

	steps, err := store.ListStepsByRun(ctx, runID)
	if err != nil || len(steps) != 1 {
		t.Fatalf("step list = %+v err=%v", steps, err)
	}
	step := steps[0]
	if step.InputArtifactID == nil || *step.InputArtifactID != inputArtifactID {
		t.Fatalf("input artifact = %v, want %d", step.InputArtifactID, inputArtifactID)
	}
	if step.OutputArtifactID == nil || *step.OutputArtifactID != outputArtifactID {
		t.Fatalf("output artifact = %v, want %d", step.OutputArtifactID, outputArtifactID)
	}
	if step.ReusedFromRunID == nil || *step.ReusedFromRunID != priorRunID {
		t.Fatalf("reused from run = %v, want %d", step.ReusedFromRunID, priorRunID)
	}
	if step.InputFingerprint != "input-fingerprint" || step.OutputFingerprint != "output-fingerprint" {
		t.Fatalf("fingerprints = %q %q", step.InputFingerprint, step.OutputFingerprint)
	}
	if step.StepStatus != "reused" || !stepTimestampPattern.MatchString(step.FinishedAt) {
		t.Fatalf("reused step = %+v", step)
	}
}
