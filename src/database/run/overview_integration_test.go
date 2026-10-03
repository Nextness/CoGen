// overview_integration_test.go exercises the run purge overview against a
// database created by the production migration runner.
//go:build integration

package run_test

import (
	"context"
	"testing"

	"analysis/database/run"
)

// TestStoreCheckPurgeEligibilityNoSharedData verifies a run with no shared evidence is eligible.
func TestStoreCheckPurgeEligibilityNoSharedData(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()
	runID, err := store.StartRun(ctx, "purge_eligible", "")
	if err != nil {
		t.Fatal(err)
	}

	eligibility, err := store.CheckPurgeEligibility(ctx, runID)
	if err != nil {
		t.Fatalf("check purge eligibility: %v", err)
	}
	if !eligibility.Eligible || eligibility.SharedArtifactCount != 0 || eligibility.ReusedByCount != 0 {
		t.Fatalf("eligibility = %+v, want eligible with no sharing", eligibility)
	}
}

// TestStoreCheckPurgeEligibilitySharedArtifact verifies an artifact referenced by another run blocks purge.
func TestStoreCheckPurgeEligibilitySharedArtifact(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runA, err := store.StartRun(ctx, "purge_shared_a", "")
	if err != nil {
		t.Fatal(err)
	}
	artifactID, err := db.Artifacts.Create("purge-shared-hash", "application/json", 100)
	if err != nil {
		t.Fatal(err)
	}
	stepA, err := store.CreateStep(ctx, runA, "parse_a")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.LinkStepOutputArtifact(ctx, run.StepArtifactInput{StepID: stepA, ArtifactID: artifactID}); err != nil {
		t.Fatal(err)
	}

	runB, err := store.StartRun(ctx, "purge_shared_b", "")
	if err != nil {
		t.Fatal(err)
	}
	stepB, err := store.CreateStep(ctx, runB, "parse_b")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.LinkStepInputArtifact(ctx, run.StepArtifactInput{StepID: stepB, ArtifactID: artifactID}); err != nil {
		t.Fatal(err)
	}

	eligibility, err := store.CheckPurgeEligibility(ctx, runA)
	if err != nil {
		t.Fatalf("check purge eligibility: %v", err)
	}
	if eligibility.Eligible || eligibility.SharedArtifactCount == 0 {
		t.Fatalf("eligibility = %+v, want ineligible shared artifact", eligibility)
	}
}

// TestStoreCheckPurgeEligibilityReusedBy verifies another run reusing a stage blocks purge while self-reuse does not.
func TestStoreCheckPurgeEligibilityReusedBy(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()
	runA, err := store.StartRun(ctx, "purge_reused_a", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateStep(ctx, runA, "parse"); err != nil {
		t.Fatal(err)
	}

	runB, err := store.StartRun(ctx, "purge_reused_b", "")
	if err != nil {
		t.Fatal(err)
	}
	stepB, err := store.CreateStep(ctx, runB, "parse")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.LinkStepReuse(ctx, run.StepReuseInput{StepID: stepB, ReusedFromRunID: runA}); err != nil {
		t.Fatal(err)
	}

	eligibility, err := store.CheckPurgeEligibility(ctx, runA)
	if err != nil {
		t.Fatalf("check purge eligibility: %v", err)
	}
	if eligibility.Eligible || eligibility.ReusedByCount == 0 {
		t.Fatalf("eligibility = %+v, want ineligible reused by another run", eligibility)
	}

	selfRun, err := store.StartRun(ctx, "purge_self_reuse", "")
	if err != nil {
		t.Fatal(err)
	}
	selfStep, err := store.CreateStep(ctx, selfRun, "parse")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.LinkStepReuse(ctx, run.StepReuseInput{StepID: selfStep, ReusedFromRunID: selfRun}); err != nil {
		t.Fatal(err)
	}
	selfEligibility, err := store.CheckPurgeEligibility(ctx, selfRun)
	if err != nil {
		t.Fatalf("check self purge eligibility: %v", err)
	}
	if !selfEligibility.Eligible || selfEligibility.ReusedByCount != 0 {
		t.Fatalf("self-reuse eligibility = %+v, want eligible", selfEligibility)
	}
}

// TestStoreCheckPurgeEligibilityReviewContexts verifies owned and descendant review contexts block purge.
func TestStoreCheckPurgeEligibilityReviewContexts(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runA, err := store.StartRun(ctx, "purge_context_a", "")
	if err != nil {
		t.Fatal(err)
	}
	runB, err := store.StartRun(ctx, "purge_context_b", "")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := db.DB.Exec("INSERT INTO review_contexts (pipeline_run_id) VALUES (?)", runA); err != nil {
		t.Fatal(err)
	}
	eligibility, err := store.CheckPurgeEligibility(ctx, runA)
	if err != nil {
		t.Fatalf("check owned context eligibility: %v", err)
	}
	if eligibility.Eligible || eligibility.OwnedReviewContextCount != 1 {
		t.Fatalf("owned context eligibility = %+v, want ineligible", eligibility)
	}

	var parentContextID int64
	if err := db.DB.QueryRow("SELECT id FROM review_contexts WHERE pipeline_run_id=?", runA).Scan(&parentContextID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec("INSERT INTO review_contexts (pipeline_run_id, parent_context_id) VALUES (?, ?)", runB, parentContextID); err != nil {
		t.Fatal(err)
	}
	eligibility, err = store.CheckPurgeEligibility(ctx, runA)
	if err != nil {
		t.Fatalf("check dependent context eligibility: %v", err)
	}
	if eligibility.Eligible || eligibility.DependentReviewContextCount != 1 {
		t.Fatalf("dependent context eligibility = %+v, want ineligible descendant", eligibility)
	}
}

// TestStoreCheckPurgeEligibilityMissingRun verifies an absent run returns an error and no eligibility value.
func TestStoreCheckPurgeEligibilityMissingRun(t *testing.T) {
	store, _ := openFamilyStore(t)
	eligibility, err := store.CheckPurgeEligibility(context.Background(), 99999)
	if err == nil {
		t.Fatal("expected an error for a non-existent run")
	}
	if eligibility != nil {
		t.Fatalf("eligibility = %+v, want nil", eligibility)
	}
}
