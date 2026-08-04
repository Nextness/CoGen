// helpers_test.go provides shared test fixtures and request helpers
// used by the viewer integration tests.
package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"analysis/database"
	"analysis/pdfstore"
)

// pdfViewerFixture is a fixture type used by the package test suite.
type pdfViewerFixture struct {
	server         *Server
	runID          int64
	availableID    int64
	notAvailableID int64
	unavailableID  int64
	revisionID     int64
}

// referenceResolutionFixture is a fixture type used by the package test suite.
type referenceResolutionFixture struct {
	path                  string
	citingRevisionID      int64
	externalMentionID     int64
	resolvedMentionID     int64
	normalizedTargetID    int64
	normalizedTargetTitle string
}

// articleActivityFixture is a fixture type used by the package test suite.
type articleActivityFixture struct {
	path                 string
	normalizedRevisionID int64
	discardedRevisionID  int64
	discardedReason      string
}

// viewerFixture supports the package test suite's viewer fixture setup or assertions.
func viewerFixture(t *testing.T) (string, int64, int64, int64) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "workspace.db")
	db, err := database.Open(path, filepath.Join("..", "..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	exec := func(query string, args ...any) sql.Result {
		result, err := db.DB.Exec(query, args...)
		if err != nil {
			t.Fatalf("fixture exec %q: %v", query, err)
		}
		return result
	}
	search := exec("INSERT INTO searches (search_id) VALUES ('research')")
	searchID, _ := search.LastInsertId()
	revision := exec("INSERT INTO search_revisions (search_id, revision_label, config_artifact_hash, resolved_manifest_hash) VALUES (?, 'r1', 'config', 'manifest')", searchID)
	revisionID, _ := revision.LastInsertId()
	plan := exec("INSERT INTO execution_plans (search_revision_id, execution_fingerprint, resolved_manifest_hash, input_manifest_hash, enrichment_enabled) VALUES (?, 'fingerprint', 'manifest', 'input', 1)", revisionID)
	planID, _ := plan.LastInsertId()
	run := exec("INSERT INTO pipeline_runs (step, started_at, status, execution_plan_id, attempt_number) VALUES ('workspace', datetime('now'), 'completed', ?, 1)", planID)
	runID, _ := run.LastInsertId()
	configArtifact := exec("INSERT INTO artifacts (content_hash, byte_size, content_type) VALUES ('config-snapshot', 12, 'application/x-something-config')")
	configArtifactID, _ := configArtifact.LastInsertId()
	resolvedArtifact := exec("INSERT INTO artifacts (content_hash, byte_size, content_type) VALUES ('manifest', 16, 'application/json')")
	resolvedArtifactID, _ := resolvedArtifact.LastInsertId()
	inputArtifact := exec("INSERT INTO artifacts (content_hash, byte_size, content_type) VALUES ('input', 12, 'application/json')")
	inputArtifactID, _ := inputArtifact.LastInsertId()
	binaryArtifact := exec("INSERT INTO artifacts (content_hash, byte_size, content_type) VALUES ('binary', 3, 'application/octet-stream')")
	binaryArtifactID, _ := binaryArtifact.LastInsertId()
	exec("INSERT INTO artifacts (content_hash, byte_size, content_type) VALUES ('no-blob', 0, 'text/plain')")
	exec("INSERT INTO artifact_blobs (artifact_id, pipeline_run_id, data) VALUES (?, ?, 'workspace = {}')", configArtifactID, runID)
	exec("INSERT INTO artifact_blobs (artifact_id, pipeline_run_id, data) VALUES (?, ?, '{\"manifest\":true}')", resolvedArtifactID, runID)
	exec("INSERT INTO artifact_blobs (artifact_id, pipeline_run_id, data) VALUES (?, ?, '{\"input\":true}')", inputArtifactID, runID)
	exec("INSERT INTO artifact_blobs (artifact_id, pipeline_run_id, data) VALUES (?, ?, ?)", binaryArtifactID, runID, []byte{0, 1, 2})
	exec("INSERT INTO run_artifacts (pipeline_run_id, artifact_id, artifact_role) VALUES (?, ?, 'workspace_config'), (?, ?, 'resolved_manifest'), (?, ?, 'input_manifest')", runID, configArtifactID, runID, resolvedArtifactID, runID, inputArtifactID)
	exec("INSERT INTO run_steps (pipeline_run_id, step_name, step_status, input_artifact_id, output_artifact_id, started_at, finished_at) VALUES (?, 'preflight', 'completed', ?, ?, '2026-01-01T00:00:00Z', '2026-01-01T00:00:05Z')", runID, configArtifactID, inputArtifactID)
	trashedRun := exec("INSERT INTO pipeline_runs (step, started_at, status, execution_plan_id, attempt_number, visibility_state, trashed_at, trash_reason) VALUES ('workspace', datetime('now'), 'completed', ?, 2, 'trashed', datetime('now'), 'fixture trash')", planID)
	trashedRunID, _ := trashedRun.LastInsertId()
	runSource := exec("INSERT INTO run_sources (pipeline_run_id, source_name, source_type, expected_file, expected_result_count, observed_result_count, result_count_comparison) VALUES (?, 'fixture', 'csv', 'fixture.csv', 4, 1, 'below')", runID)
	runSourceID, _ := runSource.LastInsertId()
	exec("INSERT INTO source_records (run_source_id, record_index, raw_payload, content_hash, parse_status) VALUES (?, 1, '{}', 'source-fixture', 'parsed')", runSourceID)
	work1 := exec("INSERT INTO works (doi) VALUES ('10.1/one')")
	work1ID, _ := work1.LastInsertId()
	work2 := exec("INSERT INTO works (doi) VALUES ('10.1/two')")
	work2ID, _ := work2.LastInsertId()
	wr1 := exec(`INSERT INTO work_revisions (work_id, pipeline_run_id, payload_hash, title, year, source, citation_count, reference_count, producer_stage) VALUES (?, ?, 'one', 'Article One', 2024, 'scopus', 5, 1, 'normalize')`, work1ID, runID)
	revisionOne, _ := wr1.LastInsertId()
	exec(`INSERT INTO work_revisions (work_id, pipeline_run_id, payload_hash, title, year, source, citation_count, reference_count, producer_stage) VALUES (?, ?, 'one-parse', 'Article One raw import', 2024, 'scopus', 5, 1, 'parse')`, work1ID, runID)
	exec(`INSERT INTO work_revisions (work_id, pipeline_run_id, payload_hash, title, year, source, citation_count, reference_count, producer_stage) VALUES (?, ?, 'two', 'Article Two', 2023, 'scopus', 2, 0, 'normalize')`, work2ID, runID)
	otherWork := exec("INSERT INTO works (doi) VALUES ('10.1/other-run')")
	otherWorkID, _ := otherWork.LastInsertId()
	exec(`INSERT INTO work_revisions (work_id, pipeline_run_id, payload_hash, title, producer_stage) VALUES (?, ?, 'other-run', 'Other Run Article', 'normalize')`, otherWorkID, trashedRunID)
	person := exec("INSERT INTO people (orcid) VALUES ('0000-0002-1825-0097')")
	personID, _ := person.LastInsertId()
	author := exec("INSERT INTO author_occurrences (person_id, citation_name, orcid) VALUES (?, 'Ada Lovelace', '0000-0002-1825-0097')", personID)
	authorID, _ := author.LastInsertId()
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (?, ?, 1, 'Analytical Society')", revisionOne, authorID)
	uncertainAuthor := exec("INSERT INTO author_occurrences (citation_name, first_name, last_name) VALUES ('Charles Babbage', 'Charles', 'Babbage')")
	uncertainAuthorID, _ := uncertainAuthor.LastInsertId()
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (?, ?, 2, 'Babbage Institute')", revisionOne, uncertainAuthorID)
	resolution := exec("INSERT INTO author_identity_resolutions (pipeline_run_id, author_occurrence_id, status, provider, queried_citation_name, resolved_at) VALUES (?, ?, 'orcid_is_unclear', 'orcid', 'Charles Babbage', datetime('now'))", runID, uncertainAuthorID)
	resolutionID, _ := resolution.LastInsertId()
	exec("INSERT INTO author_identity_candidates (identity_resolution_id, candidate_orcid, query_url, payload_artifact_id, provider_rank) VALUES (?, '0000-0001-2345-6789', 'https://orcid.example/search?q=Charles+Babbage', ?, 1), (?, '0000-0002-1825-0097', 'https://orcid.example/search?q=Charles+Babbage', ?, 2)", resolutionID, configArtifactID, resolutionID, configArtifactID)
	failedAuthor := exec("INSERT INTO author_occurrences (citation_name) VALUES ('Provider Failure')")
	failedAuthorID, _ := failedAuthor.LastInsertId()
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order) VALUES (?, ?, 3)", revisionOne, failedAuthorID)
	exec("INSERT INTO author_identity_resolutions (pipeline_run_id, author_occurrence_id, status, provider, queried_citation_name, error_message, resolved_at) VALUES (?, ?, 'provider_failed', 'orcid', 'Provider Failure', 'invalid provider payload', datetime('now'))", runID, failedAuthorID)
	mention := exec("INSERT INTO reference_mentions (work_revision_id, resolved_work_id, mention_order, doi, title, author) VALUES (?, ?, 1, '10.1/two', 'Article Two', 'Ada Lovelace')", revisionOne, work2ID)
	mentionID, _ := mention.LastInsertId()
	exec("INSERT INTO reference_mentions (work_revision_id, mention_order, doi, title, author) VALUES (?, 2, '10.external/x', 'External reference', 'External Author')", revisionOne)
	exec("INSERT INTO run_work_stages (pipeline_run_id, work_id, stage_name, outcome) VALUES (?, ?, 'validate', 'valid')", runID, work1ID)
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (?, 'input_records', '', 4), (?, 'deduplicated_articles', '', 2), (?, 'valid_articles', '', 1), (?, 'normalized_articles_processed', '', 1), (?, 'normalization_fields_processed', '', 4), (?, 'normalization_fields_changed', '', 1), (?, 'normalization_fields_already_canonical', '', 2), (?, 'normalization_fields_unavailable', '', 1), (?, 'normalization_fields_processed', 'journal', 1), (?, 'normalization_fields_changed', 'journal', 1), (?, 'cache_hits', 'crossref', 3)", runID, runID, runID, runID, runID, runID, runID, runID, runID, runID, runID)
	exec("INSERT INTO audit_events (occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, metadata_json) VALUES (datetime('now'), 'crossref', ?, 'work_revision', ?, 'field_enriched', '{\"field\":\"title\",\"provider\":\"crossref\"}')", runID, revisionOne)
	exec("INSERT INTO audit_events (occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, metadata_json) VALUES (datetime('now'), 'crossref', ?, 'work_revision', ?, 'field_enriched', '{\"field\":\"abstract\",\"provider\":\"crossref\"}')", runID, revisionOne)
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (?, 'enriched_fields_total', '', 2)", runID)
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (?, 'enriched_fields_title', '', 1)", runID)
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (?, 'enriched_fields_abstract', '', 1)", runID)
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (?, 'enriched_fields', 'crossref', 2)", runID)
	for index := int64(1); index <= 21; index++ {
		cacheEntry := exec("INSERT INTO cache_entries (provider, namespace, request_fingerprint, response_status, payload_artifact_id, fetched_at, extractor_version) VALUES ('crossref', 'doi', ?, 200, ?, datetime('now'), 'v1')", "request-"+stringID(index), configArtifactID)
		cacheEntryID, _ := cacheEntry.LastInsertId()
		exec("INSERT INTO run_cache_uses (pipeline_run_id, cache_entry_id, cache_layer, outcome) VALUES (?, ?, 'global', 'hit')", runID, cacheEntryID)
	}
	exec("INSERT INTO audit_events (occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, metadata_json) VALUES (datetime('now', '-1 minute'), 'pipeline', ?, 'work', ?, 'validation_changed', '{\"stage\":\"validate\",\"outcome\":\"valid\"}')", runID, work1ID)
	exec("INSERT INTO audit_events (occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, metadata_json) VALUES (datetime('now', '-2 minutes'), 'pipeline', ?, 'pipeline_run', ?, 'run_completed', '{\"status\":\"completed\"}')", runID, runID)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return path, runID, revisionOne, mentionID
}

// viewerReferenceResolutionFixture supports the package test suite's viewer reference resolution fixture setup or assertions.
func viewerReferenceResolutionFixture(t *testing.T) referenceResolutionFixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "workspace.db")
	db, err := database.Open(path, filepath.Join("..", "..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	createWork := func(doi string) int64 {
		t.Helper()
		id, err := db.Works.CreateByDOI(doi)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	createRevision := func(workID, runID int64, stage, title string) int64 {
		t.Helper()
		id, err := db.WorkRevisions.Create(&database.WorkRevision{
			WorkID: workID, PipelineRunID: runID, ProducerStage: stage, Title: title,
		})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	runID, err := db.PipelineRuns.StartRun("viewer", "reference resolution")
	if err != nil {
		t.Fatal(err)
	}
	citingWorkID := createWork("10.1000/viewer-citing")
	citingRevisionID := createRevision(citingWorkID, runID, database.ProducerStageNormalize, "Citing normalized title")
	targetWorkID := createWork("10.1000/viewer-target")
	stages := []struct {
		stage string
		title string
	}{
		{database.ProducerStageParse, "Target parse title"},
		{database.ProducerStageDeduplicate, "Target deduplicate title"},
		{database.ProducerStageEnrich, "Target enrich title"},
		{database.ProducerStageEnrichMetadata, "Target enrich metadata title"},
		{database.ProducerStageEnrichIdentity, "Target enrich identity title"},
		{database.ProducerStageValidate, "Target validate title"},
		{database.ProducerStageNormalize, "Target normalized title"},
	}
	var normalizedTargetID int64
	for _, snapshot := range stages {
		id := createRevision(targetWorkID, runID, snapshot.stage, snapshot.title)
		if snapshot.stage == database.ProducerStageNormalize {
			normalizedTargetID = id
		}
	}
	externalMentionID, err := db.ReferenceMentions.Create(&database.ReferenceMention{
		WorkRevisionID: citingRevisionID,
		MentionOrder:   1,
		RawReference:   "External reference",
		DOI:            "10.1000/viewer-external",
		Title:          "External reference title",
		Author:         "External author",
	})
	if err != nil {
		t.Fatal(err)
	}
	resolvedMentionID, err := db.ReferenceMentions.Create(&database.ReferenceMention{
		WorkRevisionID: citingRevisionID,
		ResolvedWorkID: targetWorkID,
		MentionOrder:   2,
		RawReference:   "Resolved reference",
		DOI:            "10.1000/viewer-target",
		Title:          "Resolved reference title",
		Author:         "Resolved author",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return referenceResolutionFixture{
		path:                  path,
		citingRevisionID:      citingRevisionID,
		externalMentionID:     externalMentionID,
		resolvedMentionID:     resolvedMentionID,
		normalizedTargetID:    normalizedTargetID,
		normalizedTargetTitle: "Target normalized title",
	}
}

// viewerArticleActivityFixture supports the package test suite's viewer article activity fixture setup or assertions.
func viewerArticleActivityFixture(t *testing.T) articleActivityFixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "workspace.db")
	db, err := database.Open(path, filepath.Join("..", "..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	createWork := func(doi string) int64 {
		t.Helper()
		id, err := db.Works.CreateByDOI(doi)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	createRevision := func(workID, runID int64, stage, title string) int64 {
		t.Helper()
		id, err := db.WorkRevisions.Create(&database.WorkRevision{
			WorkID: workID, PipelineRunID: runID, ProducerStage: stage, Title: title,
		})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	setStage := func(runID, workID int64, stage, outcome, reason string) {
		t.Helper()
		if err := db.RunWorkStages.SetOutcome(runID, workID, stage, outcome, reason); err != nil {
			t.Fatal(err)
		}
	}
	insertAudit := func(runID any, entityType string, entityID int64, action, metadata, correlationID string) {
		t.Helper()
		if _, err := db.DB.Exec(`INSERT INTO audit_events
			(occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, metadata_json, correlation_id)
			VALUES (datetime('now'), 'fixture', ?, ?, ?, ?, ?, ?)`,
			runID, entityType, stringID(entityID), action, metadata, correlationID); err != nil {
			t.Fatal(err)
		}
	}
	previousRunID, err := db.PipelineRuns.StartRun("viewer", "prior article activity")
	if err != nil {
		t.Fatal(err)
	}
	runID, err := db.PipelineRuns.StartRun("viewer", "article activity")
	if err != nil {
		t.Fatal(err)
	}
	workID := createWork("10.1000/viewer-activity")
	snapshots := []struct {
		producerStage string
		stageName     string
		outcome       string
		title         string
	}{
		{database.ProducerStageParse, database.StageNameParse, database.OutcomeParsed, "Activity parse title"},
		{database.ProducerStageDeduplicate, database.StageNameDeduplicate, database.OutcomeDeduplicated, "Activity deduplicated title"},
		{database.ProducerStageEnrichMetadata, database.StageNameEnrichMetadata, database.OutcomeEnriched, "Activity metadata title"},
		{database.ProducerStageEnrichIdentity, database.StageNameEnrichIdentity, database.OutcomeEnriched, "Activity identity title"},
		{database.ProducerStageValidate, database.StageNameValidate, database.OutcomeValid, "Activity validated title"},
		{database.ProducerStageNormalize, database.StageNameNormalize, database.OutcomeNormalized, "Activity normalized title"},
	}
	var metadataRevisionID, identityRevisionID, normalizedRevisionID int64
	for _, snapshot := range snapshots {
		revisionID := createRevision(workID, runID, snapshot.producerStage, snapshot.title)
		setStage(runID, workID, snapshot.stageName, snapshot.outcome, "")
		switch snapshot.producerStage {
		case database.ProducerStageEnrichMetadata:
			metadataRevisionID = revisionID
		case database.ProducerStageEnrichIdentity:
			identityRevisionID = revisionID
		case database.ProducerStageNormalize:
			normalizedRevisionID = revisionID
		}
	}
	insertAudit(runID, "work_revision", metadataRevisionID, "field_enriched", `{"field":"title","provider":"crossref"}`, "current-metadata-enrichment")
	insertAudit(runID, "work_revision", identityRevisionID, "field_enriched", `{"field":"orcid","provider":"orcid"}`, "current-identity-enrichment")
	insertAudit(runID, "work", workID, "validation_changed", `{"stage":"validate","outcome":"valid"}`, "current-validation")
	insertAudit(previousRunID, "work", workID, "validation_changed", `{"stage":"validate","outcome":"discarded"}`, "previous-validation")
	insertAudit(nil, "work", workID, "pdf_document_inventoried", `{}`, "manual-pdf")

	discardedWorkID := createWork("10.1000/viewer-activity-discarded")
	createRevision(discardedWorkID, runID, database.ProducerStageParse, "Discarded parse title")
	setStage(runID, discardedWorkID, database.StageNameParse, database.OutcomeParsed, "")
	discardedRevisionID := createRevision(discardedWorkID, runID, database.ProducerStageValidate, "Discarded validation title")
	discardedReason := `["DOI is missing","Publication year is invalid"]`
	setStage(runID, discardedWorkID, database.StageNameValidate, database.OutcomeDiscarded, discardedReason)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return articleActivityFixture{
		path:                 path,
		normalizedRevisionID: normalizedRevisionID,
		discardedRevisionID:  discardedRevisionID,
		discardedReason:      discardedReason,
	}
}

// viewerRequest supports the package test suite's viewer request setup or assertions.
func viewerRequest(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	return response
}

// requestJSON supports the package test suite's request json setup or assertions.
func requestJSON(t *testing.T, handler http.Handler, path string) (int, map[string]any) {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %s response: %v\n%s", path, err, recorder.Body.String())
	}
	return recorder.Code, body
}

// newPDFViewerFixture supports the package test suite's new pdf viewer fixture setup or assertions.
func newPDFViewerFixture(t *testing.T) pdfViewerFixture {
	t.Helper()
	ctx := context.Background()
	tempDir := t.TempDir()
	registry := filepath.Join("..", "..", "config", "database.something")
	metadataPath := filepath.Join(tempDir, "corpus.metadata.db")
	metadata, err := database.Open(metadataPath, registry)
	if err != nil {
		t.Fatal(err)
	}
	create := func(doi string) int64 {
		id, err := metadata.Works.CreateByDOI(doi)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	availableID := create("10.1000/viewer-available")
	notAvailableID := create("10.1000/viewer-not-available")
	unavailableID, err := metadata.Works.CreateWithoutDOI()
	if err != nil {
		t.Fatal(err)
	}
	pipelineRunID, err := metadata.PipelineRuns.StartRun("viewer", "query")
	if err != nil {
		t.Fatal(err)
	}
	revisionID, err := metadata.WorkRevisions.Create(&database.WorkRevision{
		WorkID: availableID, PipelineRunID: pipelineRunID, ProducerStage: database.ProducerStageNormalize,
		Title: "Viewer PDF article",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := metadata.WorkRevisions.Create(&database.WorkRevision{
		WorkID: notAvailableID, PipelineRunID: pipelineRunID, ProducerStage: database.ProducerStageNormalize,
		Title: "Viewer PDF article without inventory content",
	}); err != nil {
		t.Fatal(err)
	}
	if err := pdfstore.BindStore(ctx, metadata.DB, "corpus.pdf.db"); err != nil {
		t.Fatal(err)
	}
	store, err := pdfstore.Open(filepath.Join(tempDir, "corpus.pdf.db"), registry)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		doi string
		id  int64
	}{
		{"10.1000/viewer-available", availableID},
		{"10.1000/viewer-not-available", notAvailableID},
	} {
		if _, err := store.Register(ctx, item.doi, item.id, pipelineRunID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.Add(ctx, "10.1000/viewer-available", availableID, []byte("%PDF-1.7\nviewer content")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.FlushAuditOutbox(ctx, metadata.DB); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := metadata.Close(); err != nil {
		t.Fatal(err)
	}
	viewer, err := Open(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = viewer.Close() })
	return pdfViewerFixture{
		server: viewer, runID: pipelineRunID, availableID: availableID,
		notAvailableID: notAvailableID, unavailableID: unavailableID, revisionID: revisionID,
	}
}
