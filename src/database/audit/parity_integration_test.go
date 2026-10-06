// parity_integration_test.go verifies the legacy flat audit repository and the
// audit family store return and persist identical evidence on separate copies
// of one migrated fixture.
//go:build integration

package audit_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"analysis/database"
	"analysis/database/audit"
	"analysis/manifest"
)

// TestAuditFamilyParityWithLegacyRepository verifies the adapter and family store agree on reads, writes, and errors.
func TestAuditFamilyParityWithLegacyRepository(t *testing.T) {
	ctx := context.Background()
	fixturePath := filepath.Join(t.TempDir(), "fixture.db")
	fixture, err := database.Open(fixturePath, familyConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	runID, err := fixture.Run.StartRun(ctx, "audit-parity", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.AuditEvents.Insert(&manifest.AuditEvent{
		OccurredAt: "2026-01-02T03:04:05Z", Actor: "pipeline", PipelineRunID: runID,
		EntityType: "pipeline_run", EntityID: "1", Action: manifest.AuditRunStarted,
		MetadataJSON: `{"fresh":true}`, CorrelationID: "run-start-1",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.AuditEvents.Insert(&manifest.AuditEvent{
		OccurredAt: "2026-01-02T03:04:06Z", Actor: "pipeline",
		EntityType: "work", EntityID: "9", Action: manifest.AuditValidationChanged,
	}); err != nil {
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
	store := audit.New(family.DB)

	// Read parity for run and entity projections.
	legacyRun, err := legacy.AuditEvents.ListByRun(runID)
	if err != nil {
		t.Fatal(err)
	}
	familyRun, err := store.ListByRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	assertEventsEqual(t, "ListByRun", legacyRun, familyRun)

	legacyEntity, err := legacy.AuditEvents.ListByEntity("work", "9")
	if err != nil {
		t.Fatal(err)
	}
	familyEntity, err := store.ListByEntity(ctx, "work", "9")
	if err != nil {
		t.Fatal(err)
	}
	assertEventsEqual(t, "ListByEntity", legacyEntity, familyEntity)

	// Missing-read identity parity.
	legacyMissing, err := legacy.AuditEvents.ListByRun(99999)
	if err != nil || legacyMissing != nil {
		t.Fatalf("legacy missing events = %+v err=%v", legacyMissing, err)
	}
	familyMissing, err := store.ListByRun(ctx, 99999)
	if err != nil || familyMissing != nil {
		t.Fatalf("family missing events = %+v err=%v", familyMissing, err)
	}

	// Write parity.
	legacyID, err := legacy.AuditEvents.Insert(&manifest.AuditEvent{
		OccurredAt: "2026-01-02T03:04:07Z", Actor: "viewer", PipelineRunID: runID,
		EntityType: "work", EntityID: "9", Action: manifest.AuditValidationChanged,
		AfterJSON: `{"status":"valid"}`, CorrelationID: "validation-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	familyID, err := store.Insert(ctx, &manifest.AuditEvent{
		OccurredAt: "2026-01-02T03:04:07Z", Actor: "viewer", PipelineRunID: runID,
		EntityType: "work", EntityID: "9", Action: manifest.AuditValidationChanged,
		AfterJSON: `{"status":"valid"}`, CorrelationID: "validation-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if legacyID != familyID {
		t.Fatalf("Insert ID parity: legacy=%d family=%d", legacyID, familyID)
	}
	assertQueryParity(t, legacy.DB, family.DB,
		"SELECT id, occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, before_json, after_json, metadata_json, correlation_id FROM audit_events WHERE id=?", legacyID)

	// Error parity for invalid actions.
	assertErrorParity(t, "invalid action",
		legacyInsertError(legacy, &manifest.AuditEvent{Action: "not_an_action"}),
		familyInsertError(ctx, store, &manifest.AuditEvent{Action: "not_an_action"}))
	assertErrorParity(t, "nil event",
		legacyInsertError(legacy, nil),
		familyInsertError(ctx, store, nil))
}

// legacyInsertError attempts one audit insert and returns only its error.
func legacyInsertError(db *database.Database, event *manifest.AuditEvent) error {
	_, err := db.AuditEvents.Insert(event)
	return err
}

// familyInsertError attempts one audit insert and returns only its error.
func familyInsertError(ctx context.Context, store *audit.Store, event *manifest.AuditEvent) error {
	_, err := store.Insert(ctx, event)
	return err
}

// assertEventsEqual compares ordered legacy and family audit projections.
func assertEventsEqual(t *testing.T, operation string, legacy []*database.AuditEventRecord, family []*audit.Event) {
	t.Helper()
	if len(legacy) != len(family) {
		t.Fatalf("%s length mismatch: legacy=%d family=%d", operation, len(legacy), len(family))
	}
	for index := range legacy {
		left, right := legacy[index], family[index]
		if left.ID != right.ID ||
			left.OccurredAt != right.OccurredAt ||
			left.Actor != right.Actor ||
			(left.PipelineRunID == nil) != (right.PipelineRunID == nil) ||
			left.EntityType != right.EntityType ||
			left.EntityID != right.EntityID ||
			left.Action != right.Action ||
			left.BeforeJSON != right.BeforeJSON ||
			left.AfterJSON != right.AfterJSON ||
			left.MetadataJSON != right.MetadataJSON ||
			left.CorrelationID != right.CorrelationID {
			t.Fatalf("%s item %d mismatch: legacy=%+v family=%+v", operation, index, left, right)
		}
		if left.PipelineRunID != nil && *left.PipelineRunID != *right.PipelineRunID {
			t.Fatalf("%s item %d run ID mismatch: legacy=%d family=%d", operation, index, *left.PipelineRunID, *right.PipelineRunID)
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
