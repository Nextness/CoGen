// visibility_integration_test.go exercises the transactional run visibility
// lifecycle against a database created by the production migration runner.
//go:build integration

package run_test

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"analysis/database/run"
)

// nanoTimestampPattern matches RFC3339Nano text with an optional fractional second.
var nanoTimestampPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?Z$`)

// TestStoreUpdateVisibilityTrashesAndRestoresWithAudit verifies the reversible transition, no-op guard, and matching atomic audit evidence.
func TestStoreUpdateVisibilityTrashesAndRestoresWithAudit(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID, err := store.StartRun(ctx, "visibility", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FinishRun(ctx, runID, "completed", "done"); err != nil {
		t.Fatal(err)
	}

	trashed, err := store.UpdateVisibility(ctx, run.VisibilityInput{
		RunID: runID, VisibilityState: run.VisibilityTrashed, Reason: "Superseded attempt",
	})
	if err != nil {
		t.Fatalf("trash run: %v", err)
	}
	if trashed == nil || !trashed.Changed || trashed.VisibilityState != run.VisibilityTrashed {
		t.Fatalf("trash result = %+v", trashed)
	}
	stored, err := store.GetByID(ctx, runID)
	if err != nil || stored == nil {
		t.Fatalf("get trashed run = %+v err=%v", stored, err)
	}
	if stored.VisibilityState != run.VisibilityTrashed || stored.TrashedAt == nil || *stored.TrashedAt == "" {
		t.Fatalf("trashed run = %+v", stored)
	}
	if stored.TrashReason == nil || *stored.TrashReason != "Superseded attempt" {
		t.Fatalf("trash reason = %v", stored.TrashReason)
	}
	if !nanoTimestampPattern.MatchString(*stored.TrashedAt) {
		t.Fatalf("trashed_at = %q, want RFC3339Nano precision", *stored.TrashedAt)
	}

	trashAudit := latestVisibilityAudit(t, db.DB, runID)
	if trashAudit.actor != "local_user" || trashAudit.entityType != "pipeline_run" ||
		trashAudit.entityID != strconv.FormatInt(runID, 10) || trashAudit.action != "run_trashed" {
		t.Fatalf("trash audit identity = %+v", trashAudit)
	}
	if trashAudit.beforeJSON != `{"visibility_state":"active"}` || trashAudit.afterJSON != `{"visibility_state":"trashed"}` {
		t.Fatalf("trash audit state = %q -> %q", trashAudit.beforeJSON, trashAudit.afterJSON)
	}
	if trashAudit.metadataJSON != `{"source":"local_viewer"}` || trashAudit.correlationID.Valid {
		t.Fatalf("trash audit metadata = %q correlation=%v", trashAudit.metadataJSON, trashAudit.correlationID)
	}
	if trashAudit.occurredAt != *stored.TrashedAt {
		t.Fatalf("audit occurred_at %q != trashed_at %q", trashAudit.occurredAt, *stored.TrashedAt)
	}
	if !nanoTimestampPattern.MatchString(trashAudit.occurredAt) {
		t.Fatalf("audit occurred_at = %q, want RFC3339Nano precision", trashAudit.occurredAt)
	}

	unchanged, err := store.UpdateVisibility(ctx, run.VisibilityInput{
		RunID: runID, VisibilityState: run.VisibilityTrashed, Reason: "Ignored no-op",
	})
	if err != nil {
		t.Fatalf("no-op trash: %v", err)
	}
	if unchanged == nil || unchanged.Changed || unchanged.VisibilityState != run.VisibilityTrashed {
		t.Fatalf("no-op result = %+v", unchanged)
	}
	if count := visibilityAuditCount(t, db.DB, runID); count != 1 {
		t.Fatalf("audit count after no-op = %d, want 1", count)
	}

	restored, err := store.UpdateVisibility(ctx, run.VisibilityInput{
		RunID: runID, VisibilityState: run.VisibilityActive,
	})
	if err != nil {
		t.Fatalf("restore run: %v", err)
	}
	if restored == nil || !restored.Changed || restored.VisibilityState != run.VisibilityActive {
		t.Fatalf("restore result = %+v", restored)
	}
	stored, err = store.GetByID(ctx, runID)
	if err != nil || stored == nil {
		t.Fatalf("get restored run = %+v err=%v", stored, err)
	}
	if stored.VisibilityState != run.VisibilityActive || stored.TrashedAt != nil || stored.TrashReason != nil {
		t.Fatalf("restored run = %+v", stored)
	}
	restoreAudit := latestVisibilityAudit(t, db.DB, runID)
	if restoreAudit.action != "run_restored" ||
		restoreAudit.beforeJSON != `{"visibility_state":"trashed"}` ||
		restoreAudit.afterJSON != `{"visibility_state":"active"}` {
		t.Fatalf("restore audit = %+v", restoreAudit)
	}
}

// TestStoreUpdateVisibilityRejectsRunningAttempt verifies the active-attempt guard and no-op precedence without writing evidence.
func TestStoreUpdateVisibilityRejectsRunningAttempt(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID, err := store.StartRun(ctx, "running-visibility", "")
	if err != nil {
		t.Fatal(err)
	}

	unchanged, err := store.UpdateVisibility(ctx, run.VisibilityInput{
		RunID: runID, VisibilityState: run.VisibilityActive,
	})
	if err != nil || unchanged == nil || unchanged.Changed {
		t.Fatalf("running no-op = %+v err=%v", unchanged, err)
	}

	_, err = store.UpdateVisibility(ctx, run.VisibilityInput{
		RunID: runID, VisibilityState: run.VisibilityTrashed, Reason: "test",
	})
	if !errors.Is(err, run.ErrRunActive) {
		t.Fatalf("running trash error = %v, want ErrRunActive", err)
	}
	stored, err := store.GetByID(ctx, runID)
	if err != nil || stored == nil {
		t.Fatalf("get running run = %+v err=%v", stored, err)
	}
	if stored.VisibilityState != run.VisibilityActive || stored.TrashedAt != nil || stored.TrashReason != nil {
		t.Fatalf("running run mutated = %+v", stored)
	}
	if count := visibilityAuditCount(t, db.DB, runID); count != 0 {
		t.Fatalf("running guard wrote %d audit rows, want 0", count)
	}
}

// TestStoreUpdateVisibilityMissingRunReturnsNil verifies an absent run reports no result and no error.
func TestStoreUpdateVisibilityMissingRunReturnsNil(t *testing.T) {
	store, _ := openFamilyStore(t)
	result, err := store.UpdateVisibility(context.Background(), run.VisibilityInput{
		RunID: 99999, VisibilityState: run.VisibilityTrashed, Reason: "missing",
	})
	if err != nil || result != nil {
		t.Fatalf("missing run = %+v err=%v, want nil without error", result, err)
	}
}

// TestStoreUpdateVisibilityValidatesInput verifies state and reason bounds before any transaction.
func TestStoreUpdateVisibilityValidatesInput(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()
	runID, err := store.StartRun(ctx, "visibility-validation", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FinishRun(ctx, runID, "completed", ""); err != nil {
		t.Fatal(err)
	}

	if _, err := store.UpdateVisibility(ctx, run.VisibilityInput{RunID: runID, VisibilityState: "deleted"}); err == nil ||
		err.Error() != "visibility_state must be active or trashed" {
		t.Fatalf("invalid state error = %v", err)
	}
	if _, err := store.UpdateVisibility(ctx, run.VisibilityInput{
		RunID: runID, VisibilityState: run.VisibilityTrashed, Reason: strings.Repeat("a", 1001),
	}); err == nil || err.Error() != "reason must not exceed 1000 UTF-8 bytes" {
		t.Fatalf("over-limit reason error = %v", err)
	}
	if _, err := store.UpdateVisibility(ctx, run.VisibilityInput{
		RunID: runID, VisibilityState: run.VisibilityTrashed, Reason: strings.Repeat("a", 1000),
	}); err != nil {
		t.Fatalf("limit reason rejected: %v", err)
	}
}

// TestStoreUpdateVisibilityRollsBackWhenAuditFails verifies the run update and audit insert share one transaction.
func TestStoreUpdateVisibilityRollsBackWhenAuditFails(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID, err := store.StartRun(ctx, "visibility-rollback", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FinishRun(ctx, runID, "completed", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec(`CREATE TRIGGER visibility_audit_abort BEFORE INSERT ON audit_events
		WHEN NEW.action='run_trashed' BEGIN SELECT RAISE(ABORT, 'audit unavailable'); END`); err != nil {
		t.Fatal(err)
	}

	if _, err := store.UpdateVisibility(ctx, run.VisibilityInput{
		RunID: runID, VisibilityState: run.VisibilityTrashed, Reason: "rollback",
	}); err == nil {
		t.Fatal("audit failure was not propagated")
	}
	stored, err := store.GetByID(ctx, runID)
	if err != nil || stored == nil {
		t.Fatalf("get rolled-back run = %+v err=%v", stored, err)
	}
	if stored.VisibilityState != run.VisibilityActive || stored.TrashedAt != nil || stored.TrashReason != nil {
		t.Fatalf("run mutated despite audit failure: %+v", stored)
	}
}

// visibilityAudit captures one run visibility audit row for assertions.
type visibilityAudit struct {
	occurredAt    string
	actor         string
	entityType    string
	entityID      string
	action        string
	beforeJSON    string
	afterJSON     string
	metadataJSON  string
	correlationID sql.NullString
}

// latestVisibilityAudit reads the newest audit row for a run.
func latestVisibilityAudit(t *testing.T, db *sql.DB, runID int64) visibilityAudit {
	t.Helper()
	var audit visibilityAudit
	if err := db.QueryRow(`SELECT occurred_at, actor, entity_type, entity_id, action,
		before_json, after_json, metadata_json, correlation_id
		FROM audit_events WHERE pipeline_run_id=? ORDER BY id DESC LIMIT 1`, runID).Scan(
		&audit.occurredAt, &audit.actor, &audit.entityType, &audit.entityID, &audit.action,
		&audit.beforeJSON, &audit.afterJSON, &audit.metadataJSON, &audit.correlationID); err != nil {
		t.Fatalf("read visibility audit: %v", err)
	}
	return audit
}

// visibilityAuditCount counts audit rows recorded for a run.
func visibilityAuditCount(t *testing.T, db *sql.DB, runID int64) int {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM audit_events WHERE pipeline_run_id=?", runID).Scan(&count); err != nil {
		t.Fatalf("count visibility audit: %v", err)
	}
	return count
}
