// audit_integration_test.go exercises the audit store against a database
// created by the production migration runner.
//go:build integration

package audit_test

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"analysis/database"
	"analysis/database/audit"
	"analysis/manifest"
)

// familyConfigPath points at the production database registry from the family test directory.
var familyConfigPath = filepath.Join("..", "..", "..", "config", "database.something")

// openFamilyStore returns an audit store over a database created by the production migration runner.
func openFamilyStore(t *testing.T) (*audit.Store, *database.Database) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "audit.db")
	db, err := database.Open(dbPath, familyConfigPath)
	if err != nil {
		t.Fatalf("open migrated database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return audit.New(db.DB), db
}

// createTestRun creates one pipeline run for audit linkage and returns its ID.
func createTestRun(t *testing.T, db *database.Database, step string) int64 {
	t.Helper()
	runID, err := db.Run.StartRun(context.Background(), step, "")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	return runID
}

// TestStoreInsertAndListPreservesOrderingAndNullableFields verifies append-only insertion and ordered reads.
func TestStoreInsertAndListPreservesOrderingAndNullableFields(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "audit-insert")

	firstID, err := store.Insert(ctx, &manifest.AuditEvent{
		OccurredAt: "2026-01-02T03:04:05Z", Actor: "pipeline", PipelineRunID: runID,
		EntityType: "pipeline_run", EntityID: "1", Action: manifest.AuditRunStarted,
		MetadataJSON: `{"fresh":true}`, CorrelationID: "run-start-1",
	})
	if err != nil {
		t.Fatalf("insert full event: %v", err)
	}
	secondID, err := store.Insert(ctx, &manifest.AuditEvent{
		OccurredAt: "2026-01-02T03:04:06Z", Actor: "pipeline",
		EntityType: "work", EntityID: "9", Action: manifest.AuditValidationChanged,
	})
	if err != nil {
		t.Fatalf("insert minimal event: %v", err)
	}
	if firstID == 0 || secondID == 0 || firstID == secondID {
		t.Fatalf("insert ids = %d, %d", firstID, secondID)
	}

	events, err := store.ListByRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].ID != firstID {
		t.Fatalf("run events = %+v", events)
	}
	if events[0].PipelineRunID == nil || *events[0].PipelineRunID != runID ||
		events[0].MetadataJSON != `{"fresh":true}` || events[0].CorrelationID != "run-start-1" {
		t.Fatalf("full event = %+v", events[0])
	}

	byEntity, err := store.ListByEntity(ctx, "work", "9")
	if err != nil {
		t.Fatal(err)
	}
	if len(byEntity) != 1 || byEntity[0].ID != secondID {
		t.Fatalf("entity events = %+v", byEntity)
	}
	if byEntity[0].PipelineRunID != nil || byEntity[0].BeforeJSON != "" || byEntity[0].AfterJSON != "" {
		t.Fatalf("minimal event nullable fields = %+v", byEntity[0])
	}

	emptyRun, err := store.ListByRun(ctx, 99999)
	if err != nil || emptyRun != nil {
		t.Fatalf("empty run events = %+v err=%v, want nil without error", emptyRun, err)
	}
	emptyEntity, err := store.ListByEntity(ctx, "absent", "absent")
	if err != nil || emptyEntity != nil {
		t.Fatalf("empty entity events = %+v err=%v, want nil without error", emptyEntity, err)
	}
}

// TestStoreInsertRejectsInvalidAction verifies the manifest lifecycle gate runs before any row is written.
func TestStoreInsertRejectsInvalidAction(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()
	if _, err := store.Insert(ctx, nil); err == nil {
		t.Fatal("expected nil event to fail")
	}
	if _, err := store.Insert(ctx, &manifest.AuditEvent{Action: "not_an_action"}); err == nil {
		t.Fatal("expected invalid action to fail")
	}
}

// TestStoreInsertPropagatesAppendOnlyFailure verifies a rejected insert reports
// the wrapped database failure and leaves no audit row behind.
func TestStoreInsertPropagatesAppendOnlyFailure(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	if _, err := db.DB.Exec(`CREATE TRIGGER reject_audit_insert BEFORE INSERT ON audit_events
		BEGIN SELECT RAISE(ABORT, 'audit insert rejected'); END`); err != nil {
		t.Fatal(err)
	}
	_, err := store.Insert(ctx, &manifest.AuditEvent{
		OccurredAt: "2026-01-02T03:04:05Z", Actor: "pipeline",
		EntityType: "work", EntityID: "1", Action: manifest.AuditValidationChanged,
	})
	if err == nil || !strings.Contains(err.Error(), "audit insert rejected") {
		t.Fatalf("insert error = %v, want injected append-only failure", err)
	}
	var count int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM audit_events").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("rejected insert left %d audit rows", count)
	}
}

// TestStoreListSummaryAndFacetsApplyFilters verifies the dynamic filter builder
// and the bounded summary and facet reads.
func TestStoreListSummaryAndFacetsApplyFilters(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "audit-filter")
	for index, event := range []*manifest.AuditEvent{
		{OccurredAt: "2026-01-02T03:04:05Z", Actor: "pipeline", PipelineRunID: runID, EntityType: "work_revision", EntityID: "1", Action: manifest.AuditFieldEnriched, MetadataJSON: `{"provider":"crossref","field":"title"}`},
		{OccurredAt: "2026-01-02T03:04:06Z", Actor: "pipeline", PipelineRunID: runID, EntityType: "work_revision", EntityID: "1", Action: manifest.AuditFieldEnriched, MetadataJSON: `{"provider":"openalex","field":"abstract"}`},
		{OccurredAt: "2026-01-02T03:04:07Z", Actor: "viewer", PipelineRunID: runID, EntityType: "work", EntityID: "1", Action: manifest.AuditValidationChanged, AfterJSON: `{"status":"valid"}`},
	} {
		if _, err := store.Insert(ctx, event); err != nil {
			t.Fatalf("insert event %d: %v", index, err)
		}
	}

	events, err := store.List(ctx, audit.Filter{RunID: runID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[0].Action != string(manifest.AuditValidationChanged) {
		t.Fatalf("filtered events = %+v", events)
	}
	enriched, err := store.List(ctx, audit.Filter{RunID: runID, Categories: []string{"enrichment"}, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(enriched) != 2 {
		t.Fatalf("enrichment events = %+v", enriched)
	}
	searched, err := store.List(ctx, audit.Filter{RunID: runID, Query: "viewer", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(searched) != 1 || searched[0].Actor != "viewer" {
		t.Fatalf("searched events = %+v", searched)
	}

	summary, err := store.Summary(ctx, audit.Filter{RunID: runID})
	if err != nil {
		t.Fatal(err)
	}
	if summary.TotalEvents != 3 || len(summary.Actions) != 2 || summary.Actions[0].Action != string(manifest.AuditFieldEnriched) || summary.Actions[0].Count != 2 {
		t.Fatalf("summary = %+v", summary)
	}
	facets, err := store.Facets(ctx, audit.Filter{RunID: runID})
	if err != nil {
		t.Fatal(err)
	}
	if len(facets.Actors) != 2 || len(facets.Actions) != 2 || len(facets.EntityTypes) != 2 {
		t.Fatalf("facets = %+v", facets)
	}
}

// TestStoreListCursorPaginatesAndRejectsUnknownCursor verifies keyset pagination and the cursor contract.
func TestStoreListCursorPaginatesAndRejectsUnknownCursor(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "audit-cursor")
	for index := 0; index < 3; index++ {
		if _, err := store.Insert(ctx, &manifest.AuditEvent{
			OccurredAt: "2026-01-02T03:04:0" + string(rune('5'+index)) + "Z", Actor: "pipeline", PipelineRunID: runID,
			EntityType: "pipeline_run", EntityID: "1", Action: manifest.AuditRunStarted,
		}); err != nil {
			t.Fatal(err)
		}
	}
	first, err := store.List(ctx, audit.Filter{RunID: runID, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 3 {
		t.Fatalf("first page = %+v, want limit+1 lookahead", first)
	}
	second, err := store.List(ctx, audit.Filter{RunID: runID, Limit: 2, Cursor: first[1].ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || second[0].ID != first[2].ID {
		t.Fatalf("second page = %+v, want only the oldest event", second)
	}
	if _, err := store.List(ctx, audit.Filter{RunID: runID, Limit: 2, Cursor: 99999}); err != audit.ErrCursorNotFound {
		t.Fatalf("unknown cursor error = %v, want ErrCursorNotFound", err)
	}
}

// TestStoreRecordedDataAndEnrichmentSummary verifies the evidence reads.
func TestStoreRecordedDataAndEnrichmentSummary(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "audit-evidence")
	eventID, err := store.Insert(ctx, &manifest.AuditEvent{
		OccurredAt: "2026-01-02T03:04:05Z", Actor: "pipeline", PipelineRunID: runID,
		EntityType: "work_revision", EntityID: "1", Action: manifest.AuditFieldEnriched,
		BeforeJSON: `{"title":"old"}`, AfterJSON: `{"title":"new"}`, MetadataJSON: `{"provider":"crossref","field":"title"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := store.RecordedData(ctx, eventID, runID)
	if err != nil {
		t.Fatal(err)
	}
	if data == nil || data.BeforeJSON != `{"title":"old"}` || data.AfterJSON != `{"title":"new"}` || data.MetadataJSON != `{"provider":"crossref","field":"title"}` {
		t.Fatalf("recorded data = %+v", data)
	}
	missing, err := store.RecordedData(ctx, eventID, runID+999)
	if err != nil || missing != nil {
		t.Fatalf("out-of-scope recorded data = %+v err=%v, want nil without error", missing, err)
	}

	workID, err := db.Works.CreateByDOI("10.1000/audit-evidence")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.WorkRevisions.Create(&database.WorkRevision{
		WorkID: workID, PipelineRunID: runID, PayloadHash: "hash", ProducerStage: "enrich",
	}); err != nil {
		t.Fatal(err)
	}
	summary, err := store.EnrichmentSummary(ctx, workID, runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Providers) != 1 || summary.Providers[0] != "crossref" || len(summary.Fields) != 1 || summary.Fields[0] != "title" || summary.Truncated {
		t.Fatalf("enrichment summary = %+v", summary)
	}
}

// TestStoreDetailEventPagesFilterPageAndScope verifies the article and author
// detail event pages apply their logical-work and author scope, order by
// descending ID, report exact totals, and honor a continuation cursor.
func TestStoreDetailEventPagesFilterPageAndScope(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "audit-detail-events")
	workID, err := db.Works.CreateByDOI("10.1000/audit-detail-events")
	if err != nil {
		t.Fatal(err)
	}
	revisionID, err := db.WorkRevisions.Create(&database.WorkRevision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: "normalize",
	})
	if err != nil {
		t.Fatal(err)
	}
	insert := func(runID int64, entityType, entityID, action string) int64 {
		t.Helper()
		result, err := db.DB.Exec(`INSERT INTO audit_events
			(occurred_at, actor, pipeline_run_id, entity_type, entity_id, action)
			VALUES (datetime('now'), 'pipeline', ?, ?, ?, ?)`, runID, entityType, entityID, action)
		if err != nil {
			t.Fatal(err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	insert(runID, "work_revision", strconv.FormatInt(revisionID, 10), "field_enriched")
	workEventID := insert(runID, "work", strconv.FormatInt(workID, 10), "validation_changed")
	contextEventID := insert(runID, "review_context", "1", "review_context_created")
	otherRunID := createTestRun(t, db, "audit-detail-events-other")
	otherWorkID, err := db.Works.CreateByDOI("10.1000/audit-detail-events-other")
	if err != nil {
		t.Fatal(err)
	}
	otherRevisionID, err := db.WorkRevisions.Create(&database.WorkRevision{
		WorkID: otherWorkID, PipelineRunID: otherRunID, ProducerStage: "normalize",
	})
	if err != nil {
		t.Fatal(err)
	}
	insert(otherRunID, "work_revision", strconv.FormatInt(otherRevisionID, 10), "field_enriched")

	page, err := store.ListArticleDetailEvents(ctx, audit.ArticleDetailEventFilter{WorkID: workID, RunID: runID, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 || len(page.Items) != 2 || !page.HasMore {
		t.Fatalf("article detail events = %d items total=%d hasMore=%v, want 2 of 3", len(page.Items), page.Total, page.HasMore)
	}
	if page.Items[0].ID != contextEventID || page.Items[1].ID != workEventID || page.NextCursorID != workEventID {
		t.Fatalf("article detail order = %+v next=%d", page.Items, page.NextCursorID)
	}
	next, err := store.ListArticleDetailEvents(ctx, audit.ArticleDetailEventFilter{WorkID: workID, RunID: runID, CursorID: page.NextCursorID, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Items) != 1 || next.Items[0].EntityType != "work_revision" || next.HasMore {
		ids := make([]int64, 0, len(next.Items))
		for _, item := range next.Items {
			ids = append(ids, item.ID)
		}
		t.Fatalf("article detail cursor page ids=%v cursor=%d events=%+v", ids, page.NextCursorID, next.Items)
	}

	authorEventID := insert(runID, "author_occurrence", "7", "author_identity_resolved")
	insert(runID, "author_occurrence", "7", "author_identity_resolved")
	insert(runID, "author_occurrence", "8", "author_identity_resolved")
	authorPage, err := store.ListAuthorDetailEvents(ctx, audit.AuthorDetailEventFilter{AuthorOccurrenceID: 7, RunID: runID, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if authorPage.Total != 2 || len(authorPage.Items) != 1 || !authorPage.HasMore {
		t.Fatalf("author detail events = %d items total=%d hasMore=%v, want 1 of 2", len(authorPage.Items), authorPage.Total, authorPage.HasMore)
	}
	if authorPage.Items[0].EntityID != "7" || authorPage.Items[0].ID <= authorEventID {
		t.Fatalf("author detail newest event = %+v", authorPage.Items[0])
	}
	if _, err := store.ListArticleDetailEvents(ctx, audit.ArticleDetailEventFilter{WorkID: workID, RunID: runID, Limit: 0}); err == nil {
		t.Fatal("expected a limit validation error")
	}
}
