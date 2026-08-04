// Integration tests for database schema smoke tests.
//go:build integration

package database

import (
	"testing"

	"analysis/manifest"
)

// TestSchemaSmokePhase14 verifies Phase 1.4: open a fully migrated database,
// create one of every workspace entity, then verify FK integrity and expected
// indexes exist. This is the schema smoke test that validates the complete
// V00001-V00008 migration chain works together.
func TestSchemaSmokePhase14(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// 1. Create one of every entity type (respecting FK order)
	// -- searches
	searchID, err := db.Searches.Create("bpmn-optimisation")
	if err != nil {
		t.Fatalf("Searches.Create: %v", err)
	}
	if searchID == 0 {
		t.Fatal("expected non-zero search ID")
	}

	// -- search_revisions
	revID, _, err := db.Revisions.Create(searchID, "2026-07-query-expansion", "cfg-hash-a1b2", "manifest-hash-c3d4")
	if err != nil {
		t.Fatalf("Revisions.Create: %v", err)
	}
	if revID == 0 {
		t.Fatal("expected non-zero revision ID")
	}

	// -- execution_plans
	planID, err := db.Plans.Create(revID, "exec-fp-001", "manifest-hash-c3d4")
	if err != nil {
		t.Fatalf("Plans.Create: %v", err)
	}
	if planID == 0 {
		t.Fatal("expected non-zero plan ID")
	}

	// -- pipeline_runs (attempt via StartAttempt)
	runID, attemptNum, err := db.PipelineRuns.StartAttempt(planID, "parse+enrich", "TITLE-ABS-KEY(bpmn)")
	if err != nil {
		t.Fatalf("PipelineRuns.StartAttempt: %v", err)
	}
	if runID == 0 {
		t.Fatal("expected non-zero run ID")
	}
	if attemptNum != 1 {
		t.Fatalf("expected attempt 1, got %d", attemptNum)
	}

	// Finish the run so it has a terminal status
	if err := db.PipelineRuns.FinishRun(runID, "completed", "ok"); err != nil {
		t.Fatalf("PipelineRuns.FinishRun: %v", err)
	}

	// -- run_sources
	sourceID, err := db.RunSources.Create(runID, "scopus", "csv", "corpus/scopus.csv", "TITLE-ABS-KEY(bpmn)", "title,doi,authors", 3, "")
	if err != nil {
		t.Fatalf("RunSources.Create: %v", err)
	}
	if sourceID == 0 {
		t.Fatal("expected non-zero source ID")
	}

	// -- source_records
	recID, err := db.SourceRecords.Create(sourceID, 0, `{"title":"Test"}`, "abc123hash")
	if err != nil {
		t.Fatalf("SourceRecords.Create: %v", err)
	}
	if recID == 0 {
		t.Fatal("expected non-zero source record ID")
	}

	// Update parse status to accepted
	if err := db.SourceRecords.UpdateParseStatus(recID, "accepted", ""); err != nil {
		t.Fatalf("SourceRecords.UpdateParseStatus: %v", err)
	}

	// -- artifacts
	artifactID, err := db.Artifacts.Create("sha256-content-hash", "application/json", 1024)
	if err != nil {
		t.Fatalf("Artifacts.Create: %v", err)
	}
	if artifactID == 0 {
		t.Fatal("expected non-zero artifact ID")
	}

	// -- run_steps (with input artifact link)
	stepID, err := db.RunSteps.Create(runID, "parse")
	if err != nil {
		t.Fatalf("RunSteps.Create: %v", err)
	}
	if stepID == 0 {
		t.Fatal("expected non-zero step ID")
	}

	if err := db.RunSteps.LinkInputArtifact(stepID, artifactID); err != nil {
		t.Fatalf("RunSteps.LinkInputArtifact: %v", err)
	}
	if err := db.RunSteps.UpdateStatus(stepID, "completed"); err != nil {
		t.Fatalf("RunSteps.UpdateStatus: %v", err)
	}

	// -- pipeline_run_metrics
	if err := db.Metrics.Set(runID, "input_records", "scopus", 10); err != nil {
		t.Fatalf("Metrics.Set: %v", err)
	}
	m, err := db.Metrics.Get(runID, "input_records", "scopus")
	if err != nil {
		t.Fatalf("Metrics.Get: %v", err)
	}
	if m == nil || m.Value != 10 {
		t.Fatalf("expected metric value 10, got %+v", m)
	}

	// -- audit_events
	auditID, err := db.AuditEvents.Insert(&manifest.AuditEvent{
		OccurredAt:    "2026-07-21T12:00:00Z",
		Actor:         "pipeline",
		PipelineRunID: runID,
		EntityType:    "run",
		EntityID:      "1",
		Action:        manifest.AuditRunCompleted,
		MetadataJSON:  `{"attempt":1}`,
	})
	if err != nil {
		t.Fatalf("AuditEvents.Insert: %v", err)
	}
	if auditID == 0 {
		t.Fatal("expected non-zero audit event ID")
	}

	// 2. Trash and restore the run
	// Trash
	if err := db.PipelineRuns.Trash(runID, "test-purge"); err != nil {
		t.Fatalf("PipelineRuns.Trash: %v", err)
	}
	trashedRun, err := db.PipelineRuns.GetByID(runID)
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
	if err := db.PipelineRuns.Restore(runID); err != nil {
		t.Fatalf("PipelineRuns.Restore: %v", err)
	}
	restoredRun, err := db.PipelineRuns.GetByID(runID)
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
	fkViolations, err := db.DB.Query("PRAGMA foreign_key_check")
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
		err := db.DB.QueryRow(
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
		err := db.DB.QueryRow(
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
	db := openTestDB(t)
	defer db.Close()

	createRun := func(searchName, revisionLabel, fingerprint string) (int64, int64) {
		t.Helper()
		searchID, err := db.Searches.Create(searchName)
		if err != nil {
			t.Fatalf("create search %s: %v", searchName, err)
		}
		revisionID, _, err := db.Revisions.Create(searchID, revisionLabel, "config-"+searchName, "manifest-"+searchName)
		if err != nil {
			t.Fatalf("create revision %s: %v", searchName, err)
		}
		planID, err := db.Plans.Create(revisionID, fingerprint, "manifest-"+searchName)
		if err != nil {
			t.Fatalf("create plan %s: %v", searchName, err)
		}
		runID, _, err := db.PipelineRuns.StartAttempt(planID, "pipeline", searchName)
		if err != nil {
			t.Fatalf("start attempt %s: %v", searchName, err)
		}
		runSourceID, err := db.RunSources.Create(runID, "source-"+searchName, "csv", searchName+".csv", "query", "title,doi", 1, "")
		if err != nil {
			t.Fatalf("create source %s: %v", searchName, err)
		}
		if _, err := db.SourceRecords.Create(runSourceID, 0, `{"doi":"10.1000/shared"}`, "hash-"+searchName); err != nil {
			t.Fatalf("create source record %s: %v", searchName, err)
		}
		return runID, revisionID
	}

	runOne, _ := createRun("search-one", "r1", "plan-one")
	runTwo, _ := createRun("search-two", "r1", "plan-two")
	sharedWorkID, err := db.Works.CreateByDOI("10.1000/shared")
	if err != nil {
		t.Fatalf("create shared work: %v", err)
	}
	firstRevision, err := db.WorkRevisions.Create(&WorkRevision{WorkID: sharedWorkID, PipelineRunID: runOne, ProducerStage: ProducerStageParse, Title: "Search one title"})
	if err != nil {
		t.Fatalf("create first work revision: %v", err)
	}
	secondRevision, err := db.WorkRevisions.Create(&WorkRevision{WorkID: sharedWorkID, PipelineRunID: runTwo, ProducerStage: ProducerStageParse, Title: "Search two title"})
	if err != nil {
		t.Fatalf("create second work revision: %v", err)
	}
	if firstRevision == secondRevision {
		t.Fatal("each search run must retain its own revision")
	}

	firstAuthor, err := db.AuthorOccs.Create(&AuthorOccurrence{CitationName: "One, Author"})
	if err != nil {
		t.Fatalf("create first author: %v", err)
	}
	secondAuthor, err := db.AuthorOccs.Create(&AuthorOccurrence{CitationName: "Two, Author"})
	if err != nil {
		t.Fatalf("create second author: %v", err)
	}
	if _, err := db.Authorships.Create(&Authorship{WorkRevisionID: firstRevision, AuthorOccurrenceID: firstAuthor, AuthorOrder: 1}); err != nil {
		t.Fatalf("create first authorship: %v", err)
	}
	if _, err := db.Authorships.Create(&Authorship{WorkRevisionID: secondRevision, AuthorOccurrenceID: secondAuthor, AuthorOrder: 1}); err != nil {
		t.Fatalf("create second authorship: %v", err)
	}
	if _, err := db.ReferenceMentions.Create(&ReferenceMention{WorkRevisionID: firstRevision, MentionOrder: 1, DOI: "10.2000/one"}); err != nil {
		t.Fatalf("create first reference: %v", err)
	}
	if _, err := db.ReferenceMentions.Create(&ReferenceMention{WorkRevisionID: secondRevision, MentionOrder: 1, DOI: "10.2000/two"}); err != nil {
		t.Fatalf("create second reference: %v", err)
	}

	firstAuthorships, err := db.Authorships.GetByRevisionID(firstRevision)
	if err != nil || len(firstAuthorships) != 1 || firstAuthorships[0].AuthorOccurrenceID != firstAuthor {
		t.Fatalf("first authorship snapshot: %v, %+v", err, firstAuthorships)
	}
	secondAuthorships, err := db.Authorships.GetByRevisionID(secondRevision)
	if err != nil || len(secondAuthorships) != 1 || secondAuthorships[0].AuthorOccurrenceID != secondAuthor {
		t.Fatalf("second authorship snapshot: %v, %+v", err, secondAuthorships)
	}
	firstReferences, err := db.ReferenceMentions.GetByRevisionID(firstRevision)
	if err != nil || len(firstReferences) != 1 || firstReferences[0].DOI != "10.2000/one" {
		t.Fatalf("first reference snapshot: %v, %+v", err, firstReferences)
	}
	secondReferences, err := db.ReferenceMentions.GetByRevisionID(secondRevision)
	if err != nil || len(secondReferences) != 1 || secondReferences[0].DOI != "10.2000/two" {
		t.Fatalf("second reference snapshot: %v, %+v", err, secondReferences)
	}
}
