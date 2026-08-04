// Integration tests for pipeline run metrics and audit events.
//go:build integration

package database

import (
	"testing"

	"analysis/manifest"
)

// TestMetricsSetAndGet verifies that a metric can be set and retrieved.
func TestMetricsSetAndGet(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	runID, err := db.PipelineRuns.StartRun("metrics_test", "")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	err = db.Metrics.Set(runID, "input_records", "", 100)
	if err != nil {
		t.Fatalf("Metrics.Set: %v", err)
	}

	m, err := db.Metrics.Get(runID, "input_records", "")
	if err != nil {
		t.Fatalf("Metrics.Get: %v", err)
	}
	if m == nil {
		t.Fatal("expected metric, got nil")
	}
	if m.Value != 100 {
		t.Errorf("expected value 100, got %d", m.Value)
	}
	if m.Metric != "input_records" {
		t.Errorf("expected metric 'input_records', got %q", m.Metric)
	}
	if m.Source != "" {
		t.Errorf("expected empty source, got %q", m.Source)
	}
}

// TestMetricsGetNotFound verifies that Get returns nil for a missing metric.
func TestMetricsGetNotFound(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	runID, _ := db.PipelineRuns.StartRun("metrics_notfound", "")

	m, err := db.Metrics.Get(runID, "nonexistent", "")
	if err != nil {
		t.Fatalf("Metrics.Get: %v", err)
	}
	if m != nil {
		t.Fatal("expected nil for missing metric")
	}
}

// TestMetricsSetReplacesValue verifies that Set replaces an existing value.
func TestMetricsSetReplacesValue(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	runID, _ := db.PipelineRuns.StartRun("metrics_replace", "")

	_ = db.Metrics.Set(runID, "input_records", "", 100)
	_ = db.Metrics.Set(runID, "input_records", "", 200)

	m, _ := db.Metrics.Get(runID, "input_records", "")
	if m == nil {
		t.Fatal("expected metric after replace")
	}
	if m.Value != 200 {
		t.Errorf("expected value 200 after replace, got %d", m.Value)
	}
}

// TestMetricsListByRun verifies ListByRun returns all metrics for a run.
func TestMetricsListByRun(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	runID, _ := db.PipelineRuns.StartRun("metrics_list", "")
	_ = db.Metrics.Set(runID, "input_records", "", 100)
	_ = db.Metrics.Set(runID, "parsed_articles", "", 90)
	_ = db.Metrics.Set(runID, "input_records", "scopus", 60)

	metrics, err := db.Metrics.ListByRun(runID)
	if err != nil {
		t.Fatalf("Metrics.ListByRun: %v", err)
	}
	if len(metrics) != 3 {
		t.Fatalf("expected 3 metrics, got %d", len(metrics))
	}
}

// TestMetricsListByRunAndSource verifies source-filtered listing.
func TestMetricsListByRunAndSource(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	runID, _ := db.PipelineRuns.StartRun("metrics_source", "")
	_ = db.Metrics.Set(runID, "input_records", "scopus", 60)
	_ = db.Metrics.Set(runID, "input_records", "ieee", 45)
	_ = db.Metrics.Set(runID, "input_records", "", 105)

	metrics, err := db.Metrics.ListByRunAndSource(runID, "scopus")
	if err != nil {
		t.Fatalf("Metrics.ListByRunAndSource: %v", err)
	}
	if len(metrics) != 1 {
		t.Fatalf("expected 1 scopus metric, got %d", len(metrics))
	}
	if metrics[0].Value != 60 {
		t.Errorf("expected value 60, got %d", metrics[0].Value)
	}
}

// TestAuditEventInsertAndListAll verifies inserting an audit event and listing all.
func TestAuditEventInsertAndListAll(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	runID, _ := db.PipelineRuns.StartRun("audit_test", "")

	event := &manifest.AuditEvent{
		OccurredAt:    "2026-07-21T12:00:00Z",
		Actor:         "pipeline",
		PipelineRunID: runID,
		EntityType:    "run",
		EntityID:      "1",
		Action:        manifest.AuditRunStarted,
		BeforeJSON:    "",
		AfterJSON:     `{"status":"running"}`,
		MetadataJSON:  `{"reason":"normal start"}`,
		CorrelationID: "corr-001",
	}

	id, err := db.AuditEvents.Insert(event)
	if err != nil {
		t.Fatalf("AuditEvents.Insert: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero ID")
	}

	events, err := db.AuditEvents.ListAll(0)
	if err != nil {
		t.Fatalf("AuditEvents.ListAll: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Action != string(manifest.AuditRunStarted) {
		t.Errorf("expected action %q, got %q", manifest.AuditRunStarted, events[0].Action)
	}
	if events[0].CorrelationID != "corr-001" {
		t.Errorf("expected correlation_id 'corr-001', got %q", events[0].CorrelationID)
	}
	if events[0].PipelineRunID == nil || *events[0].PipelineRunID != runID {
		t.Errorf("expected pipeline_run_id %d", runID)
	}
}

// TestAuditEventInsertRejectsInvalidAction verifies that an invalid action is rejected.
func TestAuditEventInsertRejectsInvalidAction(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	event := &manifest.AuditEvent{
		OccurredAt: "2026-07-21T12:00:00Z",
		Actor:      "pipeline",
		EntityType: "run",
		EntityID:   "1",
		Action:     manifest.AuditAction("invalid_action"),
	}

	_, err := db.AuditEvents.Insert(event)
	if err == nil {
		t.Fatal("expected error for invalid audit action")
	}
}

// TestAuditEventInsertWithZeroRunID verifies that a zero PipelineRunID is stored as null.
func TestAuditEventInsertWithZeroRunID(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	event := &manifest.AuditEvent{
		OccurredAt:    "2026-07-21T12:00:00Z",
		Actor:         "pipeline",
		PipelineRunID: 0,
		EntityType:    "plan",
		EntityID:      "1",
		Action:        manifest.AuditPlanCreated,
	}

	id, err := db.AuditEvents.Insert(event)
	if err != nil {
		t.Fatalf("AuditEvents.Insert: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero ID")
	}

	events, err := db.AuditEvents.ListAll(0)
	if err != nil {
		t.Fatalf("AuditEvents.ListAll: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].PipelineRunID != nil {
		t.Error("expected nil pipeline_run_id for zero input")
	}
}

// TestAuditEventListByRun verifies listing events by pipeline run.
func TestAuditEventListByRun(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	runID1, _ := db.PipelineRuns.StartRun("audit_run1", "")
	runID2, _ := db.PipelineRuns.StartRun("audit_run2", "")

	_, _ = db.AuditEvents.Insert(&manifest.AuditEvent{
		OccurredAt: "2026-07-21T12:00:00Z", Actor: "pipeline",
		PipelineRunID: runID1, EntityType: "run", EntityID: "1",
		Action: manifest.AuditRunStarted,
	})
	_, _ = db.AuditEvents.Insert(&manifest.AuditEvent{
		OccurredAt: "2026-07-21T12:00:01Z", Actor: "pipeline",
		PipelineRunID: runID1, EntityType: "run", EntityID: "1",
		Action: manifest.AuditRunCompleted,
	})
	_, _ = db.AuditEvents.Insert(&manifest.AuditEvent{
		OccurredAt: "2026-07-21T12:00:02Z", Actor: "pipeline",
		PipelineRunID: runID2, EntityType: "run", EntityID: "2",
		Action: manifest.AuditRunStarted,
	})

	events, err := db.AuditEvents.ListByRun(runID1)
	if err != nil {
		t.Fatalf("AuditEvents.ListByRun: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events for run %d, got %d", runID1, len(events))
	}
}

// TestAuditEventListByEntity verifies listing events by entity type and ID.
func TestAuditEventListByEntity(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	_, _ = db.AuditEvents.Insert(&manifest.AuditEvent{
		OccurredAt: "2026-07-21T12:00:00Z", Actor: "pipeline",
		EntityType: "article", EntityID: "10.1234/example",
		Action: manifest.AuditFieldEnriched,
	})
	_, _ = db.AuditEvents.Insert(&manifest.AuditEvent{
		OccurredAt: "2026-07-21T12:00:01Z", Actor: "pipeline",
		EntityType: "article", EntityID: "10.1234/example",
		Action: manifest.AuditFieldEnriched,
	})
	_, _ = db.AuditEvents.Insert(&manifest.AuditEvent{
		OccurredAt: "2026-07-21T12:00:02Z", Actor: "pipeline",
		EntityType: "author", EntityID: "0000-0001-2345-6789",
		Action: manifest.AuditFieldEnriched,
	})

	events, err := db.AuditEvents.ListByEntity("article", "10.1234/example")
	if err != nil {
		t.Fatalf("AuditEvents.ListByEntity: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events for article, got %d", len(events))
	}
}

// TestAuditEventListByAction verifies listing events by action.
func TestAuditEventListByAction(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	_, _ = db.AuditEvents.Insert(&manifest.AuditEvent{
		OccurredAt: "2026-07-21T12:00:00Z", Actor: "pipeline",
		EntityType: "run", EntityID: "1", Action: manifest.AuditRunStarted,
	})
	_, _ = db.AuditEvents.Insert(&manifest.AuditEvent{
		OccurredAt: "2026-07-21T12:00:01Z", Actor: "pipeline",
		EntityType: "run", EntityID: "1", Action: manifest.AuditRunCompleted,
	})
	_, _ = db.AuditEvents.Insert(&manifest.AuditEvent{
		OccurredAt: "2026-07-21T12:00:02Z", Actor: "pipeline",
		EntityType: "run", EntityID: "2", Action: manifest.AuditRunStarted,
	})

	events, err := db.AuditEvents.ListByAction(manifest.AuditRunStarted)
	if err != nil {
		t.Fatalf("AuditEvents.ListByAction: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 'run_started' events, got %d", len(events))
	}
}

// TestAuditEventListByActionRejectsInvalid verifies that an invalid action is rejected.
func TestAuditEventListByActionRejectsInvalid(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	_, err := db.AuditEvents.ListByAction(manifest.AuditAction("bogus"))
	if err == nil {
		t.Fatal("expected error for invalid audit action")
	}
}

// TestAuditEventListAllLimit verifies the limit parameter on ListAll.
func TestAuditEventListAllLimit(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	for i := 0; i < 5; i++ {
		_, _ = db.AuditEvents.Insert(&manifest.AuditEvent{
			OccurredAt: "2026-07-21T12:00:00Z", Actor: "pipeline",
			EntityType: "run", EntityID: "1", Action: manifest.AuditRunStarted,
		})
	}

	events, err := db.AuditEvents.ListAll(3)
	if err != nil {
		t.Fatalf("AuditEvents.ListAll: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events with limit=3, got %d", len(events))
	}
}

// TestAuditEventRejectsUpdate verifies that UPDATE on audit_events is rejected
// by the append-only trigger.
func TestAuditEventRejectsUpdate(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Insert an event
	id, _ := db.AuditEvents.Insert(&manifest.AuditEvent{
		OccurredAt: "2026-07-21T12:00:00Z", Actor: "pipeline",
		EntityType: "run", EntityID: "1", Action: manifest.AuditRunStarted,
	})

	// Attempt to update it (must fail)
	_, err := db.DB.Exec("UPDATE audit_events SET action = 'run_completed' WHERE id = ?", id)
	if err == nil {
		t.Fatal("expected error when updating audit_events")
	}
}

// TestAuditEventRejectsDelete verifies that DELETE on audit_events is rejected
// by the append-only trigger.
func TestAuditEventRejectsDelete(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Insert an event
	_, _ = db.AuditEvents.Insert(&manifest.AuditEvent{
		OccurredAt: "2026-07-21T12:00:00Z", Actor: "pipeline",
		EntityType: "run", EntityID: "1", Action: manifest.AuditRunStarted,
	})

	// Verify the event exists
	countBefore := 0
	_ = db.DB.QueryRow("SELECT COUNT(*) FROM audit_events").Scan(&countBefore)
	if countBefore != 1 {
		t.Fatalf("expected 1 event before delete attempt, got %d", countBefore)
	}

	// Attempt to delete it (must fail)
	_, err := db.DB.Exec("DELETE FROM audit_events")
	if err == nil {
		t.Fatal("expected error when deleting from audit_events")
	}

	// Verify the event still exists
	countAfter := 0
	_ = db.DB.QueryRow("SELECT COUNT(*) FROM audit_events").Scan(&countAfter)
	if countAfter != 1 {
		t.Errorf("expected 1 event after failed delete, got %d", countAfter)
	}
}
