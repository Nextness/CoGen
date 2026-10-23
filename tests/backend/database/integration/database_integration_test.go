// Integration tests for database schema smoke tests.

package database_test

import testhelpers "analysis/tests/support/go/backend/database"

import (
	"context"
	"testing"

	"analysis/backend/database/author"
	"analysis/backend/database/run"
	"analysis/backend/database/search"
	"analysis/backend/database/source"
	"analysis/backend/database/work"
	"analysis/backend/manifest"
	"analysis/tests/support/go/testfixture"
)

// TestSchemaSmokePhase14 verifies Phase 1.4: open a fully migrated database,
// create one of every workspace entity, then verify FK integrity and expected
// indexes exist. This is the schema smoke test that validates the complete
// V00001-V00008 migration chain works together.
func TestSchemaSmokePhase14(t *testing.T) {
	db := testhelpers.OpenTestDB(t)
	defer db.Close()
	ctx := context.Background()

	// 1. Create one of every entity type (respecting FK order)
	// -- searches
	searchID, err := db.Search.FindOrCreateSearch(ctx, "bpmn-optimisation")
	if err != nil {
		t.Fatalf("Search.FindOrCreateSearch: %v", err)
	}
	if searchID == 0 {
		t.Fatal("expected non-zero search ID")
	}

	// -- search_revisions
	revID, _, err := db.Search.UpsertRevision(ctx, search.RevisionInput{
		SearchID: searchID, RevisionLabel: "2026-07-query-expansion", ConfigArtifactHash: "cfg-hash-a1b2", ResolvedManifestHash: "manifest-hash-c3d4",
	})
	if err != nil {
		t.Fatalf("Search.UpsertRevision: %v", err)
	}
	if revID == 0 {
		t.Fatal("expected non-zero revision ID")
	}

	// -- execution_plans
	planID, err := db.Search.FindOrCreatePlanWithInputManifest(ctx, search.PlanInput{InputManifestHash: "fixture-input",
		SearchRevisionID: revID, ExecutionFingerprint: "exec-fp-001", ResolvedManifestHash: "manifest-hash-c3d4",
	})
	if err != nil {
		t.Fatalf("Search.FindOrCreatePlanWithInputManifest: %v", err)
	}
	if planID == 0 {
		t.Fatal("expected non-zero plan ID")
	}

	// -- pipeline_runs (attempt via StartAttemptIfIdle)
	runID, attemptNum, err := db.Run.StartAttemptIfIdle(ctx, run.StartAttemptInput{
		ExecutionPlanID: planID, Step: "parse+enrich", SearchQuery: "TITLE-ABS-KEY(bpmn)",
	})
	if err != nil {
		t.Fatalf("Run.StartAttemptIfIdle: %v", err)
	}
	if runID == 0 {
		t.Fatal("expected non-zero run ID")
	}
	if attemptNum != 1 {
		t.Fatalf("expected attempt 1, got %d", attemptNum)
	}

	// Finish the run so it has a terminal status
	if err := db.Run.FinishRun(ctx, runID, "completed", "ok"); err != nil {
		t.Fatalf("Run.FinishRun: %v", err)
	}

	// -- run_sources
	sourceID, err := db.Source.CreateSource(ctx, source.CreateSourceInput{
		RunID: runID, Name: "scopus", Type: "csv", ExpectedFile: "corpus/scopus.csv",
		Query: "TITLE-ABS-KEY(bpmn)", RequestedFields: "title,doi,authors", ExpectedCount: 3,
	})
	if err != nil {
		t.Fatalf("Source.CreateSource: %v", err)
	}
	if sourceID == 0 {
		t.Fatal("expected non-zero source ID")
	}

	// -- source_records
	recID, err := db.Source.CreateRecord(ctx, source.CreateRecordInput{
		RunSourceID: sourceID, RecordIndex: 0, RawPayload: `{"title":"Test"}`, ContentHash: "abc123hash",
	})
	if err != nil {
		t.Fatalf("Source.CreateRecord: %v", err)
	}
	if recID == 0 {
		t.Fatal("expected non-zero source record ID")
	}

	// Update parse status to accepted
	if err := db.Source.UpdateRecordStatus(ctx, source.RecordStatusInput{RecordID: recID, Status: "accepted"}); err != nil {
		t.Fatalf("Source.UpdateRecordStatus: %v", err)
	}

	// -- artifacts
	artifactID, err := testfixture.Artifact(ctx, db.Conn, "sha256-content-hash", "application/json", 1024)
	if err != nil {
		t.Fatalf("Artifact.Create: %v", err)
	}
	if artifactID == 0 {
		t.Fatal("expected non-zero artifact ID")
	}

	// -- run_steps (with input artifact link)
	stepID, err := db.Run.CreateStep(ctx, runID, "parse")
	if err != nil {
		t.Fatalf("Run.CreateStep: %v", err)
	}
	if stepID == 0 {
		t.Fatal("expected non-zero step ID")
	}

	if err := db.Run.LinkStepInputArtifact(ctx, run.StepArtifactInput{StepID: stepID, ArtifactID: artifactID}); err != nil {
		t.Fatalf("Run.LinkStepInputArtifact: %v", err)
	}
	if err := db.Run.UpdateStepStatus(ctx, stepID, "completed"); err != nil {
		t.Fatalf("Run.UpdateStepStatus: %v", err)
	}

	// -- pipeline_run_metrics
	if err := db.Run.SetMetric(ctx, run.MetricInput{RunID: runID, Metric: "input_records", Source: "scopus", Value: 10}); err != nil {
		t.Fatalf("Run.SetMetric: %v", err)
	}
	m, err := db.Run.GetMetric(ctx, runID, "input_records", "scopus")
	if err != nil {
		t.Fatalf("Run.GetMetric: %v", err)
	}
	if m == nil || m.Value != 10 {
		t.Fatalf("expected metric value 10, got %+v", m)
	}

	// -- audit_events
	auditID, err := db.Audit.Insert(ctx, &manifest.AuditEvent{
		OccurredAt:    "2026-07-21T12:00:00Z",
		Actor:         "pipeline",
		PipelineRunID: runID,
		EntityType:    "run",
		EntityID:      "1",
		Action:        manifest.AuditRunCompleted,
		MetadataJSON:  `{"attempt":1}`,
	})
	if err != nil {
		t.Fatalf("Audit.Insert: %v", err)
	}
	if auditID == 0 {
		t.Fatal("expected non-zero audit event ID")
	}

	// 2. Trash and restore the run
	// Trash
	if _, err := db.Run.UpdateVisibility(ctx, run.VisibilityInput{RunID: runID, VisibilityState: run.VisibilityTrashed, Reason: "test-purge"}); err != nil {
		t.Fatalf("Run.Trash: %v", err)
	}
	trashedRun, err := db.Run.GetByID(ctx, runID)
	if err != nil {
		t.Fatalf("GetByID after trash: %v", err)
	}
	if trashedRun == nil {
		t.Fatal("run not found after trash")
	}
	if trashedRun.VisibilityState != "trashed" {
		t.Errorf("expected visibility_state 'trashed', got %q", trashedRun.VisibilityState)
	}
	if trashedRun.TrashedAt == nil || *trashedRun.TrashedAt == "" {
		t.Error("expected trashed_at to be set")
	}
	if trashedRun.TrashReason == nil || *trashedRun.TrashReason != "test-purge" {
		t.Errorf("expected trash_reason 'test-purge', got %v", trashedRun.TrashReason)
	}

	// Restore
	if _, err := db.Run.UpdateVisibility(ctx, run.VisibilityInput{RunID: runID, VisibilityState: run.VisibilityActive}); err != nil {
		t.Fatalf("Run.Restore: %v", err)
	}
	restoredRun, err := db.Run.GetByID(ctx, runID)
	if err != nil {
		t.Fatalf("GetByID after restore: %v", err)
	}
	if restoredRun == nil {
		t.Fatal("run not found after restore")
	}
	if restoredRun.VisibilityState != "active" {
		t.Errorf("expected visibility_state 'active' after restore, got %q", restoredRun.VisibilityState)
	}
	if restoredRun.TrashedAt != nil {
		t.Error("expected trashed_at to be NULL after restore")
	}
	if restoredRun.TrashReason != nil {
		t.Error("expected trash_reason to be NULL after restore")
	}

	// 3. Verify foreign keys with real SQLite
	fkViolations, err := db.Conn.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatalf("PRAGMA foreign_key_check query failed: %v", err)
	}
	defer fkViolations.Close()
	var fkFound bool
	for fkViolations.Next() {
		fkFound = true
		var table, parent string
		var rowID int64
		var child int
		if err := fkViolations.Scan(&table, &rowID, &parent, &child); err != nil {
			t.Fatalf("FK violation scan: %v", err)
		}
		t.Errorf("FK violation: table=%s rowid=%d parent=%s child=%d", table, rowID, parent, child)
	}
	if fkFound {
		t.Fatal("foreign key violations found (see above)")
	}

	// 4. Verify expected indexes exist
	expectedIndexes := []string{
		"idx_search_revisions_search_id",
		"idx_execution_plans_fingerprint",
		"idx_pipeline_runs_attempt",
		"idx_source_records_run_source",
		"idx_source_records_hash",
		"idx_run_steps_artifact_in",
		"idx_run_steps_artifact_out",
		"idx_run_steps_input_fingerprint",
		"idx_audit_events_run",
		"idx_audit_events_entity",
		"idx_audit_events_action",
		"idx_audit_events_correlation",
	}

	// Also verify the append-only triggers exist
	expectedTriggers := []string{
		"audit_events_abort_update",
		"audit_events_abort_delete",
	}

	for _, name := range expectedIndexes {
		var count int
		err := db.Conn.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?", name,
		).Scan(&count)
		if err != nil {
			t.Fatalf("index check for %q failed: %v", name, err)
		}
		if count == 0 {
			t.Errorf("expected index %q not found", name)
		}
	}

	for _, name := range expectedTriggers {
		var count int
		err := db.Conn.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type = 'trigger' AND name = ?", name,
		).Scan(&count)
		if err != nil {
			t.Fatalf("trigger check for %q failed: %v", name, err)
		}
		if count == 0 {
			t.Errorf("expected trigger %q not found", name)
		}
	}
}

// TestCorpusModelSmokePhase25 proves that two search workspaces can retain
// independent provenance around one globally identified DOI.
func TestCorpusModelSmokePhase25(t *testing.T) {
	db := testhelpers.OpenTestDB(t)
	defer db.Close()
	ctx := context.Background()

	createRun := func(searchName, revisionLabel, fingerprint string) (int64, int64) {
		t.Helper()
		searchID, err := db.Search.FindOrCreateSearch(ctx, searchName)
		if err != nil {
			t.Fatalf("create search %s: %v", searchName, err)
		}
		revisionID, _, err := db.Search.UpsertRevision(ctx, search.RevisionInput{
			SearchID: searchID, RevisionLabel: revisionLabel, ConfigArtifactHash: "config-" + searchName, ResolvedManifestHash: "manifest-" + searchName,
		})
		if err != nil {
			t.Fatalf("create revision %s: %v", searchName, err)
		}
		planID, err := db.Search.FindOrCreatePlanWithInputManifest(ctx, search.PlanInput{InputManifestHash: "fixture-input",
			SearchRevisionID: revisionID, ExecutionFingerprint: fingerprint, ResolvedManifestHash: "manifest-" + searchName,
		})
		if err != nil {
			t.Fatalf("create plan %s: %v", searchName, err)
		}
		runID, _, err := db.Run.StartAttemptIfIdle(ctx, run.StartAttemptInput{ExecutionPlanID: planID, Step: "pipeline", SearchQuery: searchName})
		if err != nil {
			t.Fatalf("start attempt %s: %v", searchName, err)
		}
		runSourceID, err := db.Source.CreateSource(ctx, source.CreateSourceInput{
			RunID: runID, Name: "source-" + searchName, Type: "csv", ExpectedFile: searchName + ".csv",
			Query: "query", RequestedFields: "title,doi", ExpectedCount: 1,
		})
		if err != nil {
			t.Fatalf("create source %s: %v", searchName, err)
		}
		if _, err := db.Source.CreateRecord(ctx, source.CreateRecordInput{
			RunSourceID: runSourceID, RecordIndex: 0, RawPayload: `{"doi":"10.1000/shared"}`, ContentHash: "hash-" + searchName,
		}); err != nil {
			t.Fatalf("create source record %s: %v", searchName, err)
		}
		return runID, revisionID
	}

	runOne, _ := createRun("search-one", "r1", "plan-one")
	runTwo, _ := createRun("search-two", "r1", "plan-two")
	sharedWorkID, err := db.Work.CreateWorkByDOI(ctx, "10.1000/shared")
	if err != nil {
		t.Fatalf("create shared work: %v", err)
	}
	firstRevision, err := db.Work.CreateRevision(ctx, &work.Revision{WorkID: sharedWorkID, PipelineRunID: runOne, ProducerStage: work.ProducerStageParse, Title: "Search one title"})
	if err != nil {
		t.Fatalf("create first work revision: %v", err)
	}
	secondRevision, err := db.Work.CreateRevision(ctx, &work.Revision{WorkID: sharedWorkID, PipelineRunID: runTwo, ProducerStage: work.ProducerStageParse, Title: "Search two title"})
	if err != nil {
		t.Fatalf("create second work revision: %v", err)
	}
	if firstRevision == secondRevision {
		t.Fatal("each search run must retain its own revision")
	}

	firstAuthor, err := db.Author.CreateOccurrence(ctx, &author.Occurrence{CitationName: "One, Author"})
	if err != nil {
		t.Fatalf("create first author: %v", err)
	}
	secondAuthor, err := db.Author.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Two, Author"})
	if err != nil {
		t.Fatalf("create second author: %v", err)
	}
	if _, err := db.Author.CreateAuthorship(ctx, &author.Authorship{WorkRevisionID: firstRevision, AuthorOccurrenceID: firstAuthor, AuthorOrder: 1}); err != nil {
		t.Fatalf("create first authorship: %v", err)
	}
	if _, err := db.Author.CreateAuthorship(ctx, &author.Authorship{WorkRevisionID: secondRevision, AuthorOccurrenceID: secondAuthor, AuthorOrder: 1}); err != nil {
		t.Fatalf("create second authorship: %v", err)
	}
	if _, err := db.Work.CreateReference(ctx, &work.Reference{WorkRevisionID: firstRevision, MentionOrder: 1, DOI: "10.2000/one"}); err != nil {
		t.Fatalf("create first reference: %v", err)
	}
	if _, err := db.Work.CreateReference(ctx, &work.Reference{WorkRevisionID: secondRevision, MentionOrder: 1, DOI: "10.2000/two"}); err != nil {
		t.Fatalf("create second reference: %v", err)
	}

	firstAuthorships, err := db.Author.ListAuthorshipsByRevisionID(ctx, firstRevision)
	if err != nil || len(firstAuthorships) != 1 || firstAuthorships[0].AuthorOccurrenceID != firstAuthor {
		t.Fatalf("first authorship snapshot: %v, %+v", err, firstAuthorships)
	}
	secondAuthorships, err := db.Author.ListAuthorshipsByRevisionID(ctx, secondRevision)
	if err != nil || len(secondAuthorships) != 1 || secondAuthorships[0].AuthorOccurrenceID != secondAuthor {
		t.Fatalf("second authorship snapshot: %v, %+v", err, secondAuthorships)
	}
	firstReferences, err := db.Work.ListReferencesByRevisionID(ctx, firstRevision)
	if err != nil || len(firstReferences) != 1 || firstReferences[0].DOI != "10.2000/one" {
		t.Fatalf("first reference snapshot: %v, %+v", err, firstReferences)
	}
	secondReferences, err := db.Work.ListReferencesByRevisionID(ctx, secondRevision)
	if err != nil || len(secondReferences) != 1 || secondReferences[0].DOI != "10.2000/two" {
		t.Fatalf("second reference snapshot: %v, %+v", err, secondReferences)
	}
}
