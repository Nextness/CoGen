// Integration tests for pipeline run lifecycle and purge eligibility.
//go:build integration

package database

import (
	"testing"
)

// TestPipelineRuns verifies pipeline runs.
func TestPipelineRuns(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	runID, err := db.PipelineRuns.StartRun("test_step", "search query")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if runID == 0 {
		t.Fatal("expected non-zero run id")
	}

	err = db.PipelineRuns.FinishRun(runID, "completed", `{"count": 5}`)
	if err != nil {
		t.Fatalf("FinishRun: %v", err)
	}
}

// TestPipelineRunAttemptNumbering verifies that StartAttempt auto-increments
// the attempt_number per execution plan.
func TestPipelineRunAttemptNumbering(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	searchID, _ := db.Searches.Create("attempt-test")
	revID, _, _ := db.Revisions.Create(searchID, "v1", "cfg", "res")
	planID, _ := db.Plans.Create(revID, "attempt-fp", "manifest")

	// First attempt should be #1
	runID1, attempt1, err := db.PipelineRuns.StartAttempt(planID, "test_step", "q1")
	if err != nil {
		t.Fatalf("StartAttempt 1: %v", err)
	}
	if runID1 == 0 {
		t.Fatal("expected non-zero run id")
	}
	if attempt1 != 1 {
		t.Errorf("expected attempt 1, got %d", attempt1)
	}

	// Second attempt should be #2
	runID2, attempt2, err := db.PipelineRuns.StartAttempt(planID, "test_step", "q2")
	if err != nil {
		t.Fatalf("StartAttempt 2: %v", err)
	}
	if runID2 == runID1 {
		t.Error("expected different run IDs for distinct attempts")
	}
	if attempt2 != 2 {
		t.Errorf("expected attempt 2, got %d", attempt2)
	}

	// Third attempt should be #3
	_, attempt3, err := db.PipelineRuns.StartAttempt(planID, "test_step", "q3")
	if err != nil {
		t.Fatalf("StartAttempt 3: %v", err)
	}
	if attempt3 != 3 {
		t.Errorf("expected attempt 3, got %d", attempt3)
	}

	// Attempts for a different plan should start at #1
	revID2, _, _ := db.Revisions.Create(searchID, "v2", "cfg2", "res2")
	planID2, _ := db.Plans.Create(revID2, "attempt-fp2", "manifest2")
	_, attemptOther, err := db.PipelineRuns.StartAttempt(planID2, "test_step", "q-other")
	if err != nil {
		t.Fatalf("StartAttempt other: %v", err)
	}
	if attemptOther != 1 {
		t.Errorf("expected attempt 1 for different plan, got %d", attemptOther)
	}
}

// TestPipelineRunStartAttemptIfIdleRejectsRunningAttempt verifies pipeline run start attempt if idle rejects running attempt.
func TestPipelineRunStartAttemptIfIdleRejectsRunningAttempt(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	searchID, _ := db.Searches.Create("idle-attempt")
	revisionID, _, _ := db.Revisions.Create(searchID, "v1", "cfg", "res")
	planID, _ := db.Plans.Create(revisionID, "idle-fingerprint", "manifest")
	runID, _, err := db.PipelineRuns.StartAttemptIfIdle(planID, "parse", "")
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = db.PipelineRuns.StartAttemptIfIdle(planID, "parse", "")
	runningErr, ok := err.(*AttemptAlreadyRunningError)
	if !ok {
		t.Fatalf("StartAttemptIfIdle error = %v, want AttemptAlreadyRunningError", err)
	}
	if runningErr.ExecutionPlanID != planID || runningErr.PipelineRunID != runID {
		t.Fatalf("running-attempt error = %+v, want plan=%d run=%d", runningErr, planID, runID)
	}

	if err := db.PipelineRuns.FinishRun(runID, "failed", "fixture failure"); err != nil {
		t.Fatal(err)
	}
	if _, attemptNumber, err := db.PipelineRuns.StartAttemptIfIdle(planID, "parse", ""); err != nil || attemptNumber != 2 {
		t.Fatalf("retry after terminal attempt = (%d, %v), want (2, nil)", attemptNumber, err)
	}
}

// TestPipelineRunStartAttemptIfIdleRejectsConcurrentAttempts verifies pipeline run start attempt if idle rejects concurrent attempts.
func TestPipelineRunStartAttemptIfIdleRejectsConcurrentAttempts(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	searchID, _ := db.Searches.Create("concurrent-idle-attempt")
	revisionID, _, _ := db.Revisions.Create(searchID, "v1", "cfg", "res")
	planID, _ := db.Plans.Create(revisionID, "concurrent-idle-fingerprint", "manifest")

	type result struct{ err error }
	const callers = 5
	results := make(chan result, callers)
	for range callers {
		go func() {
			_, _, err := db.PipelineRuns.StartAttemptIfIdle(planID, "parse", "")
			results <- result{err: err}
		}()
	}

	successes := 0
	for range callers {
		result := <-results
		if result.err == nil {
			successes++
			continue
		}
		if _, ok := result.err.(*AttemptAlreadyRunningError); !ok {
			t.Errorf("concurrent StartAttemptIfIdle error = %v, want AttemptAlreadyRunningError", result.err)
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent StartAttemptIfIdle successes = %d, want 1", successes)
	}
}

// TestPipelineRunListByPlan verifies that ListByPlan returns all attempts for a plan.
func TestPipelineRunListByPlan(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	searchID, _ := db.Searches.Create("list-by-plan")
	revID, _, _ := db.Revisions.Create(searchID, "v1", "cfg", "res")
	planID, _ := db.Plans.Create(revID, "list-fp", "manifest")

	db.PipelineRuns.StartAttempt(planID, "step", "q1")
	db.PipelineRuns.StartAttempt(planID, "step", "q2")
	db.PipelineRuns.StartAttempt(planID, "step", "q3")

	runs, err := db.PipelineRuns.ListByPlan(planID)
	if err != nil {
		t.Fatalf("ListByPlan: %v", err)
	}
	if len(runs) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(runs))
	}

	// Order should be by attempt_number
	for i, run := range runs {
		if run.AttemptNumber == nil || *run.AttemptNumber != i+1 {
			t.Errorf("run %d: expected attempt %d, got %v", i, i+1, run.AttemptNumber)
		}
		if run.ExecutionPlanID == nil || *run.ExecutionPlanID != planID {
			t.Errorf("run %d: expected execution_plan_id %d, got %v", i, planID, run.ExecutionPlanID)
		}
	}
}

// TestPipelineRunTrashAndRestore verifies the trash/restore lifecycle.
func TestPipelineRunTrashAndRestore(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	searchID, _ := db.Searches.Create("trash-test")
	revID, _, _ := db.Revisions.Create(searchID, "v1", "cfg", "res")
	planID, _ := db.Plans.Create(revID, "trash-fp", "manifest")

	runID, _, _ := db.PipelineRuns.StartAttempt(planID, "step", "trash-query")
	db.PipelineRuns.FinishRun(runID, "completed", "done")

	// Trash the run
	if err := db.PipelineRuns.Trash(runID, "test trashing"); err != nil {
		t.Fatalf("Trash: %v", err)
	}

	// Verify it's trashed
	run, err := db.PipelineRuns.GetByID(runID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if run == nil {
		t.Fatal("run not found")
	}
	if run.VisibilityState != "trashed" {
		t.Errorf("expected visibility_state 'trashed', got %q", run.VisibilityState)
	}
	if run.TrashedAt == nil || *run.TrashedAt == "" {
		t.Error("expected non-empty trashed_at")
	}
	if run.TrashReason == nil || *run.TrashReason != "test trashing" {
		t.Errorf("expected trash_reason 'test trashing', got %v", run.TrashReason)
	}

	// Restore the run
	if err := db.PipelineRuns.Restore(runID); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	run, _ = db.PipelineRuns.GetByID(runID)
	if run.VisibilityState != "active" {
		t.Errorf("expected visibility_state 'active' after restore, got %q", run.VisibilityState)
	}
	if run.TrashedAt != nil {
		t.Error("expected trashed_at to be NULL after restore")
	}
	if run.TrashReason != nil {
		t.Error("expected trash_reason to be NULL after restore")
	}
}

// TestPipelineRunListByVisibility verifies filtering by visibility state.
func TestPipelineRunListByVisibility(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	searchID, _ := db.Searches.Create("visibility-test")
	revID, _, _ := db.Revisions.Create(searchID, "v1", "cfg", "res")
	planID, _ := db.Plans.Create(revID, "vis-fp", "manifest")

	runID1, _, _ := db.PipelineRuns.StartAttempt(planID, "step", "q1")
	runID2, _, _ := db.PipelineRuns.StartAttempt(planID, "step", "q2")
	runID3, _, _ := db.PipelineRuns.StartAttempt(planID, "step", "q3")

	// Trash the middle run
	db.PipelineRuns.Trash(runID2, "removed")

	active, err := db.PipelineRuns.ListByVisibility("active")
	if err != nil {
		t.Fatalf("ListByVisibility active: %v", err)
	}
	if len(active) != 2 {
		t.Fatalf("expected 2 active runs, got %d", len(active))
	}
	activeIDs := map[int64]bool{active[0].ID: true, active[1].ID: true}
	if !activeIDs[runID1] || !activeIDs[runID3] {
		t.Error("active list should contain runID1 and runID3")
	}

	trashed, err := db.PipelineRuns.ListByVisibility("trashed")
	if err != nil {
		t.Fatalf("ListByVisibility trashed: %v", err)
	}
	if len(trashed) != 1 {
		t.Fatalf("expected 1 trashed run, got %d", len(trashed))
	}
	if trashed[0].ID != runID2 {
		t.Errorf("expected trashed run ID %d, got %d", runID2, trashed[0].ID)
	}
}

// TestPipelineRunAttemptFKReachability verifies that a run source referencing
// a non-existent pipeline run is rejected by the foreign key constraint.
func TestPipelineRunAttemptFKReachability(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Try to create a run source for a non-existent run
	_, err := db.RunSources.Create(99999, "scopus", "csv", "corpus/scopus.csv", "", "", 0, "")
	if err == nil {
		t.Fatal("expected FK violation error for non-existent pipeline_run_id")
	}
}

// TestLegacyStartRunBackwardCompat verifies that the legacy StartRun/FinishRun
// methods still work and leave the new columns as NULL.
func TestLegacyStartRunBackwardCompat(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	runID, err := db.PipelineRuns.StartRun("legacy_step", "legacy_query")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if runID == 0 {
		t.Fatal("expected non-zero run id")
	}

	if err := db.PipelineRuns.FinishRun(runID, "completed", "legacy done"); err != nil {
		t.Fatalf("FinishRun: %v", err)
	}

	// Verify the run exists and new columns are NULL
	run, err := db.PipelineRuns.GetByID(runID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if run == nil {
		t.Fatal("run not found")
	}
	if run.Step != "legacy_step" {
		t.Errorf("expected step 'legacy_step', got %q", run.Step)
	}
	if run.Status != "completed" {
		t.Errorf("expected status 'completed', got %q", run.Status)
	}
	if run.ExecutionPlanID != nil {
		t.Error("expected execution_plan_id to be NULL for legacy run")
	}
	if run.AttemptNumber != nil {
		t.Error("expected attempt_number to be NULL for legacy run")
	}
	if run.VisibilityState != "active" {
		t.Errorf("expected visibility_state 'active' for legacy run, got %q", run.VisibilityState)
	}
}

// TestPipelineRunConcurrentStartAttempt verifies that concurrent callers to
// StartAttempt each receive a unique attempt_number, enforced by the UNIQUE
// constraint and the retry loop.
func TestPipelineRunConcurrentStartAttempt(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	searchID, _ := db.Searches.Create("concurrent-attempt")
	revID, _, _ := db.Revisions.Create(searchID, "v1", "cfg", "res")
	planID, _ := db.Plans.Create(revID, "concurrent-fp", "manifest")

	const goroutines = 10
	type result struct {
		runID      int64
		attemptNum int
		err        error
	}

	results := make(chan result, goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			runID, attemptNum, err := db.PipelineRuns.StartAttempt(planID, "concurrent", "q")
			results <- result{runID, attemptNum, err}
		}()
	}

	seen := make(map[int]bool)
	for i := 0; i < goroutines; i++ {
		r := <-results
		if r.err != nil {
			t.Errorf("StartAttempt failed: %v", r.err)
			continue
		}
		if r.attemptNum < 1 || r.attemptNum > goroutines {
			t.Errorf("attempt number %d out of range [1,%d]", r.attemptNum, goroutines)
		}
		if seen[r.attemptNum] {
			t.Errorf("duplicate attempt number %d", r.attemptNum)
		}
		seen[r.attemptNum] = true
	}

	if len(seen) != goroutines {
		t.Errorf("expected %d unique attempt numbers, got %d", goroutines, len(seen))
	}
}

// TestPipelineRunFinishRunInvalidStatus verifies that FinishRun rejects invalid
// attempt statuses.
func TestPipelineRunFinishRunInvalidStatus(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	runID, err := db.PipelineRuns.StartRun("test_invalid_status", "")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Invalid status should be rejected
	err = db.PipelineRuns.FinishRun(runID, "invalid_status", "summary")
	if err == nil {
		t.Fatal("expected error for invalid attempt status")
	}

	// Valid terminal status should succeed
	err = db.PipelineRuns.FinishRun(runID, "completed", "summary")
	if err != nil {
		t.Fatalf("FinishRun with valid status: %v", err)
	}

	// Verify finished_at is set
	run, err := db.PipelineRuns.GetByID(runID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if run.FinishedAt == nil || *run.FinishedAt == "" {
		t.Error("expected non-empty finished_at for terminal status")
	}
}

// TestPurgeEligibilityNoSharedData verifies that a run with no shared data
// is eligible for purge.
func TestPurgeEligibilityNoSharedData(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	runID, _ := db.PipelineRuns.StartRun("purge_eligible", "")

	pe, err := db.PipelineRuns.CheckPurgeEligibility(runID)
	if err != nil {
		t.Fatalf("CheckPurgeEligibility: %v", err)
	}
	if !pe.Eligible {
		t.Errorf("expected eligible=true for run with no artifacts, got eligible=%v", pe.Eligible)
	}
	if pe.SharedArtifactCount != 0 {
		t.Errorf("expected 0 shared artifacts, got %d", pe.SharedArtifactCount)
	}
	if pe.ReusedByCount != 0 {
		t.Errorf("expected 0 reused by, got %d", pe.ReusedByCount)
	}
}

// TestPurgeEligibilitySharedArtifact verifies that sharing an artifact makes
// the source run ineligible for purge.
func TestPurgeEligibilitySharedArtifact(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Create run A
	runA, _ := db.PipelineRuns.StartRun("purge_shared_a", "")

	// Create an artifact
	artifactID, _ := db.Artifacts.Create("hash-abc", "application/json", 100)

	// Create a run step for run A with the artifact as output
	stepA, _ := db.RunSteps.Create(runA, "parse_a")
	_ = db.RunSteps.LinkOutputArtifact(stepA, artifactID)

	// Create run B
	runB, _ := db.PipelineRuns.StartRun("purge_shared_b", "")

	// Create a run step for run B that references the artifact as input
	stepB, _ := db.RunSteps.Create(runB, "parse")
	_ = db.RunSteps.LinkInputArtifact(stepB, artifactID)

	// Run A should be ineligible because its artifact is referenced by run B
	pe, err := db.PipelineRuns.CheckPurgeEligibility(runA)
	if err != nil {
		t.Fatalf("CheckPurgeEligibility: %v", err)
	}
	if pe.Eligible {
		t.Error("expected eligible=false when artifact is shared with another run")
	}
	if pe.SharedArtifactCount == 0 {
		t.Error("expected non-zero shared artifact count")
	}
}

// TestPurgeEligibilityReusedBy verifies that another run reusing a stage from
// this run makes the source run ineligible for purge.
func TestPurgeEligibilityReusedBy(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Create run A
	runA, _ := db.PipelineRuns.StartRun("purge_reused_a", "")

	// Create a run step for run A
	_, _ = db.RunSteps.Create(runA, "parse")

	// Create run B that reuses from run A
	runB, _ := db.PipelineRuns.StartRun("purge_reused_b", "")
	stepB, _ := db.RunSteps.Create(runB, "parse")
	_ = db.RunSteps.LinkReuse(stepB, runA)

	// Run A should be ineligible because run B reuses its stage
	pe, err := db.PipelineRuns.CheckPurgeEligibility(runA)
	if err != nil {
		t.Fatalf("CheckPurgeEligibility: %v", err)
	}
	if pe.Eligible {
		t.Error("expected eligible=false when another run reuses a stage")
	}
	if pe.ReusedByCount == 0 {
		t.Error("expected non-zero reused by count")
	}
}

// TestPurgeEligibilitySharedOutputArtifact verifies that sharing an artifact as
// another run's output makes the source run ineligible for purge.
func TestPurgeEligibilitySharedOutputArtifact(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	runA, _ := db.PipelineRuns.StartRun("purge_shared_output_a", "")
	runB, _ := db.PipelineRuns.StartRun("purge_shared_output_b", "")

	// Create an artifact (content-addressed, same hash = same ID)
	artifactID, _ := db.Artifacts.Create("hash-output-shared", "application/json", 100)

	// Run A produces the artifact as output
	stepA, _ := db.RunSteps.Create(runA, "parse_a")
	_ = db.RunSteps.LinkOutputArtifact(stepA, artifactID)

	// Run B also produces the same artifact as output
	stepB, _ := db.RunSteps.Create(runB, "parse_b")
	_ = db.RunSteps.LinkOutputArtifact(stepB, artifactID)

	// Run A should be ineligible because run B references the same artifact as output
	pe, err := db.PipelineRuns.CheckPurgeEligibility(runA)
	if err != nil {
		t.Fatalf("CheckPurgeEligibility: %v", err)
	}
	if pe.Eligible {
		t.Error("expected eligible=false when artifact is shared as output with another run")
	}
	if pe.SharedArtifactCount == 0 {
		t.Error("expected non-zero shared artifact count for output sharing")
	}
}

// TestPurgeEligibilityNonexistentRun verifies that checking eligibility for
// a non-existent run returns an error.
func TestPurgeEligibilityNonexistentRun(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	pe, err := db.PipelineRuns.CheckPurgeEligibility(99999)
	if err == nil {
		t.Fatal("expected error for non-existent run")
	}
	if pe != nil {
		t.Fatal("expected nil PurgeEligibility for non-existent run")
	}
}

// TestPurgeEligibilitySelfReuseDoesNotBlock verifies that a self-reuse record
// (a step reusing from the same run) does not falsely make the run ineligible.
func TestPurgeEligibilitySelfReuseDoesNotBlock(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	runID, _ := db.PipelineRuns.StartRun("purge_self_reuse", "")

	// Create a step that reuses from the same run — a self-reuse link.
	stepID, _ := db.RunSteps.Create(runID, "parse")
	_ = db.RunSteps.LinkReuse(stepID, runID)

	pe, err := db.PipelineRuns.CheckPurgeEligibility(runID)
	if err != nil {
		t.Fatalf("CheckPurgeEligibility: %v", err)
	}
	if !pe.Eligible {
		t.Errorf("expected eligible=true despite self-reuse, got eligible=%v", pe.Eligible)
	}
	if pe.ReusedByCount != 0 {
		t.Errorf("expected ReusedByCount=0 for self-reuse, got %d", pe.ReusedByCount)
	}
}
