// parity_integration_test.go verifies the legacy flat repositories and the run
// family store return and persist identical lifecycle, step, metric, reviewer,
// purge, and recovery evidence on separate copies of one migrated fixture.
//go:build integration

package run_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"

	"analysis/database"
	"analysis/database/run"
)

// secondTimestampPattern matches the second-precision pipeline_runs persisted format.
var secondTimestampPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$`)

// TestRunFamilyParityWithLegacyRepositories verifies the adapters and family store agree on reads, writes, errors, and recovery evidence.
func TestRunFamilyParityWithLegacyRepositories(t *testing.T) {
	ctx := context.Background()
	fixturePath := filepath.Join(t.TempDir(), "fixture.db")
	fixture, err := database.Open(fixturePath, familyConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	planID := createTestPlan(t, fixture, "parity")

	// Seed one completed attempt with a step, metric, and reviewer identity.
	priorRun, _, err := fixture.PipelineRuns.StartAttempt(planID, "prior", "query")
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.PipelineRuns.FinishRun(priorRun, "completed", "done"); err != nil {
		t.Fatal(err)
	}
	priorStep, err := fixture.RunSteps.Create(priorRun, "parse")
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.RunSteps.SetFingerprints(priorStep, "input-fingerprint", "output-fingerprint"); err != nil {
		t.Fatal(err)
	}
	if err := fixture.Metrics.Set(priorRun, "records", "scopus", 3); err != nil {
		t.Fatal(err)
	}
	if err := fixture.PipelineRunReviewers.Insert(priorRun, "Researcher", "researcher@example.test"); err != nil {
		t.Fatal(err)
	}

	// Seed one running attempt with a running step for recovery parity.
	recoverRun, _, err := fixture.PipelineRuns.StartAttempt(planID, "recover", "")
	if err != nil {
		t.Fatal(err)
	}
	recoverStep, err := fixture.RunSteps.Create(recoverRun, "running-step")
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.RunSteps.UpdateStatus(recoverStep, "running"); err != nil {
		t.Fatal(err)
	}

	if _, err := fixture.DB.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		t.Fatal(err)
	}
	if err := fixture.Close(); err != nil {
		t.Fatal(err)
	}

	legacyPath := filepath.Join(t.TempDir(), "legacy.db")
	familyPath := filepath.Join(t.TempDir(), "family.db")
	copyDatabaseFile(t, fixturePath, legacyPath)
	copyDatabaseFile(t, fixturePath, familyPath)

	legacy, err := database.OpenExisting(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	defer legacy.Close()
	family, err := database.OpenExisting(familyPath)
	if err != nil {
		t.Fatal(err)
	}
	defer family.Close()
	store := run.New(family.DB)

	// Read parity for run projection, plan ordering, steps, metrics, reviewer, and purge overview.
	legacyRun, err := legacy.PipelineRuns.GetByID(priorRun)
	if err != nil {
		t.Fatal(err)
	}
	familyRun, err := store.GetByID(ctx, priorRun)
	if err != nil {
		t.Fatal(err)
	}
	assertRunEqual(t, "GetByID", legacyRun, familyRun)

	legacyRuns, err := legacy.PipelineRuns.ListByPlan(planID)
	if err != nil {
		t.Fatal(err)
	}
	familyRuns, err := store.ListByPlan(ctx, planID)
	if err != nil {
		t.Fatal(err)
	}
	assertRunsEqual(t, "ListByPlan", legacyRuns, familyRuns)

	legacyActive, err := legacy.PipelineRuns.ListByVisibility("active")
	if err != nil {
		t.Fatal(err)
	}
	familyActive, err := store.ListByVisibility(ctx, "active")
	if err != nil {
		t.Fatal(err)
	}
	assertRunsEqual(t, "ListByVisibility", legacyActive, familyActive)

	legacySteps, err := legacy.RunSteps.ListByRun(priorRun)
	if err != nil {
		t.Fatal(err)
	}
	familySteps, err := store.ListStepsByRun(ctx, priorRun)
	if err != nil {
		t.Fatal(err)
	}
	assertStepsEqual(t, "ListStepsByRun", legacySteps, familySteps)

	legacyMetric, err := legacy.Metrics.Get(priorRun, "records", "scopus")
	if err != nil {
		t.Fatal(err)
	}
	familyMetric, err := store.GetMetric(ctx, priorRun, "records", "scopus")
	if err != nil {
		t.Fatal(err)
	}
	if legacyMetric == nil || familyMetric == nil || legacyMetric.Value != familyMetric.Value || legacyMetric.Source != familyMetric.Source {
		t.Fatalf("metric parity: legacy=%+v family=%+v", legacyMetric, familyMetric)
	}

	legacyReviewer, err := legacy.PipelineRunReviewers.Get(priorRun)
	if err != nil {
		t.Fatal(err)
	}
	familyReviewer, err := store.GetReviewer(ctx, priorRun)
	if err != nil {
		t.Fatal(err)
	}
	if legacyReviewer == nil || familyReviewer == nil ||
		legacyReviewer.Username != familyReviewer.Username || legacyReviewer.Email != familyReviewer.Email {
		t.Fatalf("reviewer parity: legacy=%+v family=%+v", legacyReviewer, familyReviewer)
	}

	legacyEligibility, err := legacy.PipelineRuns.CheckPurgeEligibility(priorRun)
	if err != nil {
		t.Fatal(err)
	}
	familyEligibility, err := store.CheckPurgeEligibility(ctx, priorRun)
	if err != nil {
		t.Fatal(err)
	}
	if *legacyEligibility != (database.PurgeEligibility{
		Eligible:                    familyEligibility.Eligible,
		SharedArtifactCount:         familyEligibility.SharedArtifactCount,
		ReusedByCount:               familyEligibility.ReusedByCount,
		OwnedReviewContextCount:     familyEligibility.OwnedReviewContextCount,
		DependentReviewContextCount: familyEligibility.DependentReviewContextCount,
	}) {
		t.Fatalf("purge eligibility parity: legacy=%+v family=%+v", legacyEligibility, familyEligibility)
	}

	// Legacy run creation parity.
	legacyNewRun, err := legacy.PipelineRuns.StartRun("parity", "")
	if err != nil {
		t.Fatal(err)
	}
	familyNewRun, err := store.StartRun(ctx, "parity", "")
	if err != nil {
		t.Fatal(err)
	}
	if legacyNewRun != familyNewRun {
		t.Fatalf("StartRun ID parity: legacy=%d family=%d", legacyNewRun, familyNewRun)
	}
	assertStoredRunEqual(t, legacy.DB, family.DB, legacyNewRun)

	// Attempt allocation parity.
	legacyAttemptRun, legacyAttempt, err := legacy.PipelineRuns.StartAttempt(planID, "parity-attempt", "")
	if err != nil {
		t.Fatal(err)
	}
	familyAttemptRun, familyAttempt, err := store.StartAttempt(ctx, run.StartAttemptInput{ExecutionPlanID: planID, Step: "parity-attempt"})
	if err != nil {
		t.Fatal(err)
	}
	if legacyAttemptRun != familyAttemptRun || legacyAttempt != familyAttempt {
		t.Fatalf("StartAttempt parity: legacy=(%d,%d) family=(%d,%d)", legacyAttemptRun, legacyAttempt, familyAttemptRun, familyAttempt)
	}
	assertStoredRunEqual(t, legacy.DB, family.DB, legacyAttemptRun)

	// Completion parity.
	if err := legacy.PipelineRuns.FinishRun(legacyAttemptRun, "failed", "parity failure"); err != nil {
		t.Fatal(err)
	}
	if err := store.FinishRun(ctx, familyAttemptRun, "failed", "parity failure"); err != nil {
		t.Fatal(err)
	}
	assertStoredRunEqual(t, legacy.DB, family.DB, legacyAttemptRun)

	// Visibility parity.
	if err := legacy.PipelineRuns.Trash(legacyNewRun, "parity reason"); err != nil {
		t.Fatal(err)
	}
	if err := store.Trash(ctx, familyNewRun, "parity reason"); err != nil {
		t.Fatal(err)
	}
	assertStoredRunEqual(t, legacy.DB, family.DB, legacyNewRun)
	if err := legacy.PipelineRuns.Restore(legacyNewRun); err != nil {
		t.Fatal(err)
	}
	if err := store.Restore(ctx, familyNewRun); err != nil {
		t.Fatal(err)
	}
	assertStoredRunEqual(t, legacy.DB, family.DB, legacyNewRun)

	// Step lifecycle and link parity.
	legacyStep, err := legacy.RunSteps.Create(legacyNewRun, "step")
	if err != nil {
		t.Fatal(err)
	}
	familyStep, err := store.CreateStep(ctx, familyNewRun, "step")
	if err != nil {
		t.Fatal(err)
	}
	if legacyStep != familyStep {
		t.Fatalf("CreateStep ID parity: legacy=%d family=%d", legacyStep, familyStep)
	}
	if err := legacy.RunSteps.UpdateStatus(legacyStep, "completed"); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateStepStatus(ctx, familyStep, "completed"); err != nil {
		t.Fatal(err)
	}
	if err := legacy.RunSteps.SetFingerprints(legacyStep, "in", "out"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetStepFingerprints(ctx, run.StepFingerprintInput{StepID: familyStep, InputFingerprint: "in", OutputFingerprint: "out"}); err != nil {
		t.Fatal(err)
	}
	assertStoredStepEqual(t, legacy.DB, family.DB, legacyStep)

	// Metric write parity.
	if err := legacy.Metrics.Set(legacyNewRun, "new-metric", "source", 9); err != nil {
		t.Fatal(err)
	}
	if err := store.SetMetric(ctx, run.MetricInput{RunID: familyNewRun, Metric: "new-metric", Source: "source", Value: 9}); err != nil {
		t.Fatal(err)
	}
	assertQueryParity(t, legacy.DB, family.DB,
		"SELECT pipeline_run_id, metric, source, value FROM pipeline_run_metrics WHERE pipeline_run_id=? ORDER BY metric, source", legacyNewRun)

	// Reviewer write parity.
	if err := legacy.PipelineRunReviewers.Insert(legacyNewRun, "  Name  ", "  name@example.test  "); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertReviewer(ctx, run.ReviewerInput{RunID: familyNewRun, Username: "  Name  ", Email: "  name@example.test  "}); err != nil {
		t.Fatal(err)
	}
	assertQueryParity(t, legacy.DB, family.DB,
		"SELECT pipeline_run_id, username, email FROM pipeline_run_reviewers WHERE pipeline_run_id=?", legacyNewRun)

	// Recovery parity: terminal run and step rows plus non-timestamp audit evidence.
	if err := legacy.PipelineRuns.RecoverAbandoned(ctx, recoverRun); err != nil {
		t.Fatal(err)
	}
	if err := store.RecoverAbandoned(ctx, recoverRun); err != nil {
		t.Fatal(err)
	}
	assertQueryParity(t, legacy.DB, family.DB,
		"SELECT id, step, status, summary FROM pipeline_runs WHERE id=?", recoverRun)
	assertQueryParity(t, legacy.DB, family.DB,
		"SELECT id, pipeline_run_id, step_name, step_status FROM run_steps WHERE id=?", recoverStep)
	assertQueryParity(t, legacy.DB, family.DB,
		"SELECT actor, entity_type, entity_id, action, metadata_json, correlation_id FROM audit_events WHERE pipeline_run_id=? ORDER BY id", recoverRun)
	if err := legacy.PipelineRuns.RecoverAbandoned(ctx, recoverRun); err == nil {
		t.Fatal("legacy accepted a second recovery")
	}
	if err := store.RecoverAbandoned(ctx, recoverRun); err == nil {
		t.Fatal("family accepted a second recovery")
	}

	// Error parity for validation and missing-row contracts.
	assertErrorParity(t, "invalid attempt status",
		legacy.PipelineRuns.FinishRun(priorRun, "bogus", ""),
		store.FinishRun(ctx, priorRun, "bogus", ""))
	assertErrorParity(t, "invalid stage outcome",
		legacy.RunSteps.UpdateStatus(priorStep, "bogus"),
		store.UpdateStepStatus(ctx, priorStep, "bogus"))
	assertErrorParity(t, "invalid reviewer",
		legacy.PipelineRunReviewers.Insert(0, "name", ""),
		store.InsertReviewer(ctx, run.ReviewerInput{RunID: 0, Username: "name"}))
	legacyMissing, legacyErr := legacy.PipelineRuns.CheckPurgeEligibility(99999)
	familyMissing, familyErr := store.CheckPurgeEligibility(ctx, 99999)
	assertErrorParity(t, "missing purge run", legacyErr, familyErr)
	if legacyMissing != nil || familyMissing != nil {
		t.Fatalf("missing purge eligibility: legacy=%+v family=%+v", legacyMissing, familyMissing)
	}
	legacyAbsent, err := legacy.PipelineRuns.GetByID(99999)
	if err != nil || legacyAbsent != nil {
		t.Fatalf("legacy missing run = %+v err=%v", legacyAbsent, err)
	}
	familyAbsent, err := store.GetByID(ctx, 99999)
	if err != nil || familyAbsent != nil {
		t.Fatalf("family missing run = %+v err=%v", familyAbsent, err)
	}
}

// assertRunEqual compares one legacy and one family run projection without requiring identical timestamps.
func assertRunEqual(t *testing.T, operation string, legacy *database.PipelineRun, family *run.Run) {
	t.Helper()
	if (legacy == nil) != (family == nil) {
		t.Fatalf("%s nil mismatch: legacy=%+v family=%+v", operation, legacy, family)
	}
	if legacy == nil {
		return
	}
	if legacy.ID != family.ID ||
		legacy.Step != family.Step ||
		legacy.StartedAt != family.StartedAt ||
		legacy.Status != family.Status ||
		legacy.VisibilityState != family.VisibilityState ||
		(legacy.FinishedAt == nil) != (family.FinishedAt == nil) ||
		(legacy.Summary == nil) != (family.Summary == nil) ||
		(legacy.SearchQuery == nil) != (family.SearchQuery == nil) ||
		(legacy.ExecutionPlanID == nil) != (family.ExecutionPlanID == nil) ||
		(legacy.AttemptNumber == nil) != (family.AttemptNumber == nil) ||
		(legacy.TrashedAt == nil) != (family.TrashedAt == nil) ||
		(legacy.TrashReason == nil) != (family.TrashReason == nil) {
		t.Fatalf("%s mismatch: legacy=%+v family=%+v", operation, legacy, family)
	}
	if legacy.ExecutionPlanID != nil && *legacy.ExecutionPlanID != *family.ExecutionPlanID {
		t.Fatalf("%s plan ID mismatch: legacy=%d family=%d", operation, *legacy.ExecutionPlanID, *family.ExecutionPlanID)
	}
	if legacy.AttemptNumber != nil && *legacy.AttemptNumber != *family.AttemptNumber {
		t.Fatalf("%s attempt mismatch: legacy=%d family=%d", operation, *legacy.AttemptNumber, *family.AttemptNumber)
	}
	if legacy.TrashReason != nil && *legacy.TrashReason != *family.TrashReason {
		t.Fatalf("%s trash reason mismatch: legacy=%q family=%q", operation, *legacy.TrashReason, *family.TrashReason)
	}
	if legacy.Summary != nil && *legacy.Summary != *family.Summary {
		t.Fatalf("%s summary mismatch: legacy=%q family=%q", operation, *legacy.Summary, *family.Summary)
	}
	if legacy.SearchQuery != nil && *legacy.SearchQuery != *family.SearchQuery {
		t.Fatalf("%s query mismatch: legacy=%q family=%q", operation, *legacy.SearchQuery, *family.SearchQuery)
	}
}

// assertRunsEqual compares ordered legacy and family run projections.
func assertRunsEqual(t *testing.T, operation string, legacy []*database.PipelineRun, family []*run.Run) {
	t.Helper()
	if len(legacy) != len(family) {
		t.Fatalf("%s length mismatch: legacy=%d family=%d", operation, len(legacy), len(family))
	}
	for index := range legacy {
		assertRunEqual(t, fmt.Sprintf("%s item %d", operation, index), legacy[index], family[index])
	}
}

// assertStepsEqual compares ordered legacy and family step projections.
func assertStepsEqual(t *testing.T, operation string, legacy []*database.RunStep, family []*run.Step) {
	t.Helper()
	if len(legacy) != len(family) {
		t.Fatalf("%s length mismatch: legacy=%d family=%d", operation, len(legacy), len(family))
	}
	for index := range legacy {
		if legacy[index].ID != family[index].ID ||
			legacy[index].PipelineRunID != family[index].PipelineRunID ||
			legacy[index].StepName != family[index].StepName ||
			legacy[index].StepStatus != family[index].StepStatus ||
			(legacy[index].InputArtifactID == nil) != (family[index].InputArtifactID == nil) ||
			(legacy[index].OutputArtifactID == nil) != (family[index].OutputArtifactID == nil) ||
			(legacy[index].ReusedFromRunID == nil) != (family[index].ReusedFromRunID == nil) ||
			legacy[index].InputFingerprint != family[index].InputFingerprint ||
			legacy[index].OutputFingerprint != family[index].OutputFingerprint {
			t.Fatalf("%s item %d mismatch: legacy=%+v family=%+v", operation, index, legacy[index], family[index])
		}
	}
}

// assertStoredRunEqual verifies the persisted lifecycle fields excluding call-time timestamps.
func assertStoredRunEqual(t *testing.T, legacy, family *sql.DB, id int64) {
	t.Helper()
	assertQueryParity(t, legacy, family,
		"SELECT id, step, status, summary, search_query, execution_plan_id, attempt_number, visibility_state, trash_reason FROM pipeline_runs WHERE id=?", id)
	for _, db := range []*sql.DB{legacy, family} {
		var startedAt string
		if err := db.QueryRow("SELECT started_at FROM pipeline_runs WHERE id=?", id).Scan(&startedAt); err != nil {
			t.Fatal(err)
		}
		if !secondTimestampPattern.MatchString(startedAt) {
			t.Fatalf("started_at %q does not match the persisted second format", startedAt)
		}
	}
}

// assertStoredStepEqual verifies the persisted step fields excluding call-time timestamps.
func assertStoredStepEqual(t *testing.T, legacy, family *sql.DB, id int64) {
	t.Helper()
	assertQueryParity(t, legacy, family,
		"SELECT id, pipeline_run_id, step_name, step_status, input_artifact_id, output_artifact_id, reused_from_run_id, input_fingerprint, output_fingerprint FROM run_steps WHERE id=?", id)
	for _, db := range []*sql.DB{legacy, family} {
		var finishedAt string
		if err := db.QueryRow("SELECT finished_at FROM run_steps WHERE id=?", id).Scan(&finishedAt); err != nil {
			t.Fatal(err)
		}
		if !stepTimestampPattern.MatchString(finishedAt) {
			t.Fatalf("finished_at %q does not match the persisted microsecond format", finishedAt)
		}
	}
}

// assertQueryParity runs the same read on both databases and compares every returned row.
func assertQueryParity(t *testing.T, legacy, family *sql.DB, query string, args ...any) {
	t.Helper()
	legacyRows := queryRowValues(t, legacy, query, args...)
	familyRows := queryRowValues(t, family, query, args...)
	if !reflect.DeepEqual(legacyRows, familyRows) {
		t.Fatalf("query parity mismatch:\nlegacy=%v\nfamily=%v", legacyRows, familyRows)
	}
}

// queryRowValues renders every row of one query as ordered comparable text.
func queryRowValues(t *testing.T, db *sql.DB, query string, args ...any) [][]string {
	t.Helper()
	rows, err := db.Query(query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	names, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var result [][]string
	for rows.Next() {
		values := make([]any, len(names))
		pointers := make([]any, len(names))
		for index := range values {
			pointers[index] = &values[index]
		}
		if err := rows.Scan(pointers...); err != nil {
			t.Fatal(err)
		}
		rendered := make([]string, len(names))
		for index, value := range values {
			rendered[index] = formatDatabaseValue(value)
		}
		result = append(result, rendered)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

// copyDatabaseFile copies one checkpointed fixture into an independent working copy.
func copyDatabaseFile(t *testing.T, source, destination string) {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// formatDatabaseValue renders one scanned SQLite value as comparable text.
func formatDatabaseValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return "<null>"
	case []byte:
		return string(typed)
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

// assertErrorParity verifies legacy and family operations reject input with the same error text.
func assertErrorParity(t *testing.T, operation string, legacyErr, familyErr error) {
	t.Helper()
	if (legacyErr == nil) != (familyErr == nil) {
		t.Fatalf("%s error mismatch: legacy=%v family=%v", operation, legacyErr, familyErr)
	}
	if legacyErr != nil && legacyErr.Error() != familyErr.Error() {
		t.Fatalf("%s error text mismatch: legacy=%q family=%q", operation, legacyErr, familyErr)
	}
}
