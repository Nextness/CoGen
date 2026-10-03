// run_integration_test.go exercises the run family store against a database
// created by the production migration runner.
//go:build integration

package run_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"analysis/database"
	"analysis/database/run"
	"analysis/database/search"

	_ "modernc.org/sqlite"
)

// familyConfigPath points at the production database registry from the family test directory.
var familyConfigPath = filepath.Join("..", "..", "..", "config", "database.something")

// openFamilyStore returns a run store over a database created by the production migration runner.
func openFamilyStore(t *testing.T) (*run.Store, *database.Database) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "run.db")
	db, err := database.Open(dbPath, familyConfigPath)
	if err != nil {
		t.Fatalf("open migrated database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return run.New(db.DB), db
}

// createTestPlan creates one search, revision, and execution plan and returns the plan ID.
func createTestPlan(t *testing.T, db *database.Database, label string) int64 {
	t.Helper()
	ctx := context.Background()
	searchID, err := db.Search.FindOrCreateSearch(ctx, "run-family-"+label)
	if err != nil {
		t.Fatalf("create search: %v", err)
	}
	revisionID, _, err := db.Search.UpsertRevision(ctx, search.RevisionInput{
		SearchID:             searchID,
		RevisionLabel:        "v1",
		ConfigArtifactHash:   "config-" + label,
		ResolvedManifestHash: "manifest-" + label,
	})
	if err != nil {
		t.Fatalf("create revision: %v", err)
	}
	planID, err := db.Search.FindOrCreatePlan(ctx, search.PlanInput{
		SearchRevisionID:     revisionID,
		ExecutionFingerprint: "fingerprint-" + label,
		ResolvedManifestHash: "manifest-" + label,
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	return planID
}

// TestStoreStartRunAndReadsPreserveNullabilityAndOrdering verifies legacy run creation, optional fields, not-found identity, and declared list ordering.
func TestStoreStartRunAndReadsPreserveNullabilityAndOrdering(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	planID := createTestPlan(t, db, "start-run")

	legacyID, err := store.StartRun(ctx, "legacy_step", "")
	if err != nil {
		t.Fatalf("start legacy run: %v", err)
	}
	legacy, err := store.GetByID(ctx, legacyID)
	if err != nil || legacy == nil {
		t.Fatalf("get legacy run = %+v err=%v", legacy, err)
	}
	if legacy.Step != "legacy_step" || legacy.Status != "running" || legacy.VisibilityState != "active" {
		t.Fatalf("legacy run = %+v", legacy)
	}
	if legacy.SearchQuery != nil || legacy.ExecutionPlanID != nil || legacy.AttemptNumber != nil || legacy.FinishedAt != nil {
		t.Fatalf("legacy optional fields = %+v", legacy)
	}

	queriedID, err := store.StartRun(ctx, "queried_step", "search text")
	if err != nil {
		t.Fatalf("start queried run: %v", err)
	}
	queried, err := store.GetByID(ctx, queriedID)
	if err != nil || queried == nil {
		t.Fatalf("get queried run = %+v err=%v", queried, err)
	}
	if queried.SearchQuery == nil || *queried.SearchQuery != "search text" {
		t.Fatalf("queried search_query = %v", queried.SearchQuery)
	}

	missing, err := store.GetByID(ctx, 999)
	if err != nil || missing != nil {
		t.Fatalf("missing run = %+v err=%v, want nil without error", missing, err)
	}
	emptyPlan, err := store.ListByPlan(ctx, 999)
	if err != nil || emptyPlan != nil {
		t.Fatalf("empty plan list = %+v err=%v, want nil without error", emptyPlan, err)
	}
	emptyVisibility, err := store.ListByVisibility(ctx, "absent")
	if err != nil || emptyVisibility != nil {
		t.Fatalf("empty visibility list = %+v err=%v, want nil without error", emptyVisibility, err)
	}

	first, _, err := store.StartAttempt(ctx, run.StartAttemptInput{ExecutionPlanID: planID, Step: "attempt", SearchQuery: "q1"})
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := store.StartAttempt(ctx, run.StartAttemptInput{ExecutionPlanID: planID, Step: "attempt", SearchQuery: "q2"})
	if err != nil {
		t.Fatal(err)
	}
	third, _, err := store.StartAttempt(ctx, run.StartAttemptInput{ExecutionPlanID: planID, Step: "attempt", SearchQuery: "q3"})
	if err != nil {
		t.Fatal(err)
	}
	byPlan, err := store.ListByPlan(ctx, planID)
	if err != nil {
		t.Fatal(err)
	}
	if len(byPlan) != 3 || byPlan[0].ID != first || byPlan[1].ID != second || byPlan[2].ID != third {
		t.Fatalf("plan list = %+v, want ids [%d %d %d]", byPlan, first, second, third)
	}
	for index, item := range byPlan {
		if item.AttemptNumber == nil || *item.AttemptNumber != index+1 {
			t.Fatalf("run %d attempt number = %v, want %d", index, item.AttemptNumber, index+1)
		}
	}

	active, err := store.ListByVisibility(ctx, "active")
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 5 {
		t.Fatalf("active run count = %d, want 5", len(active))
	}
	for index := 1; index < len(active); index++ {
		if active[index-1].ID >= active[index].ID {
			t.Fatalf("active list is not ordered by id: %+v", active)
		}
	}
}

// TestStoreStartAttemptAllocatesPositiveNumbersAtomically verifies each plan receives its own positive sequence and other plans stay independent.
func TestStoreStartAttemptAllocatesPositiveNumbersAtomically(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	planID := createTestPlan(t, db, "numbering")
	otherPlanID := createTestPlan(t, db, "numbering-other")

	for want := 1; want <= 3; want++ {
		_, attempt, err := store.StartAttempt(ctx, run.StartAttemptInput{ExecutionPlanID: planID, Step: "step"})
		if err != nil {
			t.Fatalf("attempt %d: %v", want, err)
		}
		if attempt != want {
			t.Fatalf("attempt number = %d, want %d", attempt, want)
		}
	}
	_, otherAttempt, err := store.StartAttempt(ctx, run.StartAttemptInput{ExecutionPlanID: otherPlanID, Step: "step"})
	if err != nil {
		t.Fatal(err)
	}
	if otherAttempt != 1 {
		t.Fatalf("other plan attempt number = %d, want 1", otherAttempt)
	}
}

// TestStoreStartAttemptIfIdleRejectsRunningAttempt verifies the typed active-attempt conflict and retry after a terminal status.
func TestStoreStartAttemptIfIdleRejectsRunningAttempt(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	planID := createTestPlan(t, db, "idle")

	runID, attempt, err := store.StartAttemptIfIdle(ctx, run.StartAttemptInput{ExecutionPlanID: planID, Step: "parse"})
	if err != nil {
		t.Fatal(err)
	}
	if attempt != 1 {
		t.Fatalf("first attempt = %d, want 1", attempt)
	}

	_, _, err = store.StartAttemptIfIdle(ctx, run.StartAttemptInput{ExecutionPlanID: planID, Step: "parse"})
	runningErr, ok := err.(*run.AttemptAlreadyRunningError)
	if !ok {
		t.Fatalf("second start error = %v, want AttemptAlreadyRunningError", err)
	}
	if runningErr.ExecutionPlanID != planID || runningErr.PipelineRunID != runID {
		t.Fatalf("running-attempt error = %+v, want plan=%d run=%d", runningErr, planID, runID)
	}

	if err := store.FinishRun(ctx, runID, "failed", "fixture failure"); err != nil {
		t.Fatal(err)
	}
	_, retryAttempt, err := store.StartAttemptIfIdle(ctx, run.StartAttemptInput{ExecutionPlanID: planID, Step: "parse"})
	if err != nil {
		t.Fatal(err)
	}
	if retryAttempt != 2 {
		t.Fatalf("retry attempt = %d, want 2", retryAttempt)
	}
}

// TestStoreStartAttemptIfIdleSerializesConcurrentAttempts verifies only one racing writer wins and the rest receive the typed conflict.
func TestStoreStartAttemptIfIdleSerializesConcurrentAttempts(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	planID := createTestPlan(t, db, "concurrent-idle")

	type result struct{ err error }
	const callers = 5
	results := make(chan result, callers)
	for range callers {
		go func() {
			_, _, err := store.StartAttemptIfIdle(ctx, run.StartAttemptInput{ExecutionPlanID: planID, Step: "parse"})
			results <- result{err: err}
		}()
	}

	successes := 0
	for range callers {
		outcome := <-results
		if outcome.err == nil {
			successes++
			continue
		}
		if _, ok := outcome.err.(*run.AttemptAlreadyRunningError); !ok {
			t.Errorf("concurrent start error = %v, want AttemptAlreadyRunningError", outcome.err)
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent successes = %d, want 1", successes)
	}
}

// TestStoreConcurrentStartAttemptAllocatesUniqueNumbers verifies racing callers each receive a distinct positive attempt number.
func TestStoreConcurrentStartAttemptAllocatesUniqueNumbers(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	planID := createTestPlan(t, db, "concurrent")

	const callers = 10
	type result struct {
		attempt int
		err     error
	}
	results := make(chan result, callers)
	for range callers {
		go func() {
			_, attempt, err := store.StartAttempt(ctx, run.StartAttemptInput{ExecutionPlanID: planID, Step: "parse"})
			results <- result{attempt: attempt, err: err}
		}()
	}

	seen := make(map[int]bool, callers)
	for range callers {
		outcome := <-results
		if outcome.err != nil {
			t.Errorf("concurrent start failed: %v", outcome.err)
			continue
		}
		if outcome.attempt < 1 || outcome.attempt > callers {
			t.Errorf("attempt number %d out of range [1,%d]", outcome.attempt, callers)
		}
		if seen[outcome.attempt] {
			t.Errorf("duplicate attempt number %d", outcome.attempt)
		}
		seen[outcome.attempt] = true
	}
	if len(seen) != callers {
		t.Fatalf("unique attempt numbers = %d, want %d", len(seen), callers)
	}
}

// TestStoreFinishRunSetsTerminalFieldsConsistently verifies status validation and the finished_at rule for running versus terminal statuses.
func TestStoreFinishRunSetsTerminalFieldsConsistently(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()

	runID, err := store.StartRun(ctx, "finish", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FinishRun(ctx, runID, "invalid_status", "summary"); err == nil {
		t.Fatal("invalid status was accepted")
	}

	if err := store.FinishRun(ctx, runID, "running", ""); err != nil {
		t.Fatal(err)
	}
	stillRunning, err := store.GetByID(ctx, runID)
	if err != nil || stillRunning == nil {
		t.Fatalf("running run = %+v err=%v", stillRunning, err)
	}
	if stillRunning.FinishedAt != nil {
		t.Fatalf("running finished_at = %v, want nil", stillRunning.FinishedAt)
	}
	if stillRunning.Summary != nil {
		t.Fatalf("empty summary = %v, want nil", stillRunning.Summary)
	}

	if err := store.FinishRun(ctx, runID, "completed", `{"count":5}`); err != nil {
		t.Fatal(err)
	}
	completed, err := store.GetByID(ctx, runID)
	if err != nil || completed == nil {
		t.Fatalf("completed run = %+v err=%v", completed, err)
	}
	if completed.FinishedAt == nil || *completed.FinishedAt == "" {
		t.Fatal("terminal finished_at was not set")
	}
	if completed.Summary == nil || *completed.Summary != `{"count":5}` {
		t.Fatalf("terminal summary = %v", completed.Summary)
	}
}

// TestStoreTrashAndRestorePreservesVisibilityLifecycle verifies the reversible visibility transition and cleared trash fields.
func TestStoreTrashAndRestorePreservesVisibilityLifecycle(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()

	runID, err := store.StartRun(ctx, "visibility", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FinishRun(ctx, runID, "completed", "done"); err != nil {
		t.Fatal(err)
	}
	if err := store.Trash(ctx, runID, "test trashing"); err != nil {
		t.Fatal(err)
	}
	trashed, err := store.GetByID(ctx, runID)
	if err != nil || trashed == nil {
		t.Fatalf("trashed run = %+v err=%v", trashed, err)
	}
	if trashed.VisibilityState != "trashed" || trashed.TrashedAt == nil || *trashed.TrashedAt == "" {
		t.Fatalf("trashed run = %+v", trashed)
	}
	if trashed.TrashReason == nil || *trashed.TrashReason != "test trashing" {
		t.Fatalf("trash reason = %v", trashed.TrashReason)
	}

	if err := store.Restore(ctx, runID); err != nil {
		t.Fatal(err)
	}
	restored, err := store.GetByID(ctx, runID)
	if err != nil || restored == nil {
		t.Fatalf("restored run = %+v err=%v", restored, err)
	}
	if restored.VisibilityState != "active" || restored.TrashedAt != nil || restored.TrashReason != nil {
		t.Fatalf("restored run = %+v", restored)
	}
}

// TestStoreRecoverAbandonedFailsRunAndStepsWithAudit verifies the recovery transaction, affected-row guard, and append-only evidence.
func TestStoreRecoverAbandonedFailsRunAndStepsWithAudit(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	planID := createTestPlan(t, db, "recovery")

	runID, _, err := store.StartAttempt(ctx, run.StartAttemptInput{ExecutionPlanID: planID, Step: "parse"})
	if err != nil {
		t.Fatal(err)
	}
	pendingStep, err := store.CreateStep(ctx, runID, "pending-step")
	if err != nil {
		t.Fatal(err)
	}
	runningStep, err := store.CreateStep(ctx, runID, "running-step")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateStepStatus(ctx, runningStep, "running"); err != nil {
		t.Fatal(err)
	}

	if err := store.RecoverAbandoned(ctx, runID); err != nil {
		t.Fatalf("recover: %v", err)
	}
	recovered, err := store.GetByID(ctx, runID)
	if err != nil || recovered == nil {
		t.Fatalf("recovered run = %+v err=%v", recovered, err)
	}
	if recovered.Status != "failed" || recovered.FinishedAt == nil || *recovered.FinishedAt == "" {
		t.Fatalf("recovered run = %+v", recovered)
	}
	if recovered.Summary == nil || *recovered.Summary != "abandoned attempt recovered by operator" {
		t.Fatalf("recovery summary = %v", recovered.Summary)
	}
	steps, err := store.ListStepsByRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range steps {
		if step.StepStatus != "failed" || step.FinishedAt == "" {
			t.Fatalf("step %d after recovery = %+v", step.ID, step)
		}
	}
	if pendingStep == runningStep {
		t.Fatal("step IDs collided")
	}

	var actor, entityType, entityID, action, metadataJSON, correlationID string
	if err := db.DB.QueryRow(`SELECT actor, entity_type, entity_id, action, metadata_json, correlation_id
		FROM audit_events WHERE pipeline_run_id=? ORDER BY id DESC LIMIT 1`, runID).Scan(
		&actor, &entityType, &entityID, &action, &metadataJSON, &correlationID); err != nil {
		t.Fatal(err)
	}
	if actor != "operator" || entityType != "pipeline_run" || entityID != fmt.Sprint(runID) {
		t.Fatalf("recovery audit identity = %q %q %q", actor, entityType, entityID)
	}
	if action != "run_failed" || metadataJSON != `{"reason":"abandoned_attempt_recovery"}` || correlationID != "run-recovered-"+fmt.Sprint(runID) {
		t.Fatalf("recovery audit evidence = %q %q %q", action, metadataJSON, correlationID)
	}

	if err := store.RecoverAbandoned(ctx, runID); err == nil {
		t.Fatal("second recovery of a terminal attempt was accepted")
	}
	var auditCount int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM audit_events WHERE pipeline_run_id=?", runID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("recovery audit count = %d, want 1", auditCount)
	}
	if err := store.RecoverAbandoned(ctx, 0); err == nil {
		t.Fatal("non-positive run ID was accepted")
	}
}

// TestNewBindsConfiguredConnectionWithoutSideEffects verifies the constructor neither opens, closes, reconfigures, nor migrates the supplied connection.
func TestNewBindsConfiguredConnectionWithoutSideEffects(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unmigrated.db")
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	conn.SetMaxOpenConns(1)
	defer conn.Close()

	readPragmas := func() (string, int, int) {
		t.Helper()
		var journalMode string
		if err := conn.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
			t.Fatal(err)
		}
		var foreignKeys, busyTimeout int
		if err := conn.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
			t.Fatal(err)
		}
		return journalMode, foreignKeys, busyTimeout
	}
	modeBefore, foreignKeysBefore, busyTimeoutBefore := readPragmas()

	store := run.New(conn)
	if store == nil {
		t.Fatal("New returned nil")
	}

	modeAfter, foreignKeysAfter, busyTimeoutAfter := readPragmas()
	if modeBefore != modeAfter || foreignKeysBefore != foreignKeysAfter || busyTimeoutBefore != busyTimeoutAfter {
		t.Fatalf("constructor altered pragmas: before=(%s,%d,%d) after=(%s,%d,%d)",
			modeBefore, foreignKeysBefore, busyTimeoutBefore, modeAfter, foreignKeysAfter, busyTimeoutAfter)
	}

	var tables int
	if err := conn.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables != 0 {
		t.Fatalf("constructor created schema: tables=%d", tables)
	}

	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.StartRun(context.Background(), "closed", ""); err == nil {
		t.Fatal("store used a connection the constructor opened instead of the supplied one")
	}
}
