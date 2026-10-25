// helpers_test.go provides shared test fixtures and request helpers
// used by the viewer integration tests.

package testhelpers

import "analysis/tests/support/go/testenv"

import subject "analysis/backend/server"

import (
	"analysis/tests/support/go/pdffixture"

	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"

	"testing"

	"analysis/backend/database"
	"analysis/backend/database/run"
	"analysis/backend/database/search"
	"analysis/backend/database/work"
	"analysis/backend/pdfstore"
	"analysis/internal/sqliteread"
	"analysis/tests/support/go/testfixture"
)

// PdfViewerFixture is a fixture type used by the package test suite.
type PdfViewerFixture struct {
	Server                 *Viewer
	MetadataPath           string
	RunID                  int64
	AvailableID            int64
	NotAvailableID         int64
	UnavailableID          int64
	RevisionID             int64
	NotAvailableRevisionID int64
}

// ReferenceResolutionFixture is a fixture type used by the package test suite.
type ReferenceResolutionFixture struct {
	Path                  string
	RunID                 int64
	CitingRevisionID      int64
	ExternalMentionID     int64
	ResolvedMentionID     int64
	NormalizedTargetID    int64
	NormalizedTargetTitle string
}

// ArticleActivityFixture is a fixture type used by the package test suite.
type ArticleActivityFixture struct {
	Path                 string
	RunID                int64
	NormalizedRevisionID int64
	DiscardedRevisionID  int64
	DiscardedReason      string
}

// ViewerFixture supports the package test suite's viewer fixture setup or assertions.
func ViewerFixture(t *testing.T) (string, int64, int64, int64) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "workspace.db")
	db, err := testfixture.Open(path, testenv.Path("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	exec := func(query string, args ...any) sql.Result {
		result, err := db.Conn.Exec(query, args...)
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
		cacheEntry := exec("INSERT INTO cache_entries (provider, namespace, request_fingerprint, response_status, payload_artifact_id, fetched_at, extractor_version) VALUES ('crossref', 'doi', ?, 200, ?, datetime('now'), 'v1')", "request-"+fmt.Sprint(index), configArtifactID)
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

// ViewerReferenceResolutionFixture supports the package test suite's viewer reference resolution fixture setup or assertions.
func ViewerReferenceResolutionFixture(t *testing.T) ReferenceResolutionFixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "workspace.db")
	db, err := testfixture.Open(path, testenv.Path("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	createWork := func(doi string) int64 {
		t.Helper()
		id, err := db.Work.CreateWorkByDOI(context.Background(), doi)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	createRevision := func(workID, runID int64, stage, title string) int64 {
		t.Helper()
		id, err := db.Work.CreateRevision(context.Background(), &work.Revision{
			WorkID: workID, PipelineRunID: runID, ProducerStage: stage, Title: title,
		})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	runID, err := testfixture.Run(context.Background(), db.Conn, "viewer", "reference resolution")
	if err != nil {
		t.Fatal(err)
	}
	citingWorkID := createWork("10.1000/viewer-citing")
	citingRevisionID := createRevision(citingWorkID, runID, work.ProducerStageNormalize, "Citing normalized title")
	targetWorkID := createWork("10.1000/viewer-target")
	for _, workID := range []int64{citingWorkID, targetWorkID} {
		if err := db.Work.SetStageOutcome(context.Background(), work.StageOutcomeInput{
			RunID: runID, WorkID: workID, StageName: work.StageNameValidate, Outcome: work.OutcomeValid,
		}); err != nil {
			t.Fatal(err)
		}
	}
	stages := []struct {
		Stage string
		Title string
	}{
		{work.ProducerStageParse, "Target parse title"},
		{work.ProducerStageDeduplicate, "Target deduplicate title"},
		{work.ProducerStageEnrich, "Target enrich title"},
		{work.ProducerStageEnrichMetadata, "Target enrich metadata title"},
		{work.ProducerStageEnrichIdentity, "Target enrich identity title"},
		{work.ProducerStageValidate, "Target validate title"},
		{work.ProducerStageNormalize, "Target normalized title"},
	}
	var normalizedTargetID int64
	for _, snapshot := range stages {
		id := createRevision(targetWorkID, runID, snapshot.Stage, snapshot.Title)
		if snapshot.Stage == work.ProducerStageNormalize {
			normalizedTargetID = id
		}
	}
	externalMentionID, err := db.Work.CreateReference(context.Background(), &work.Reference{
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
	resolvedMentionID, err := db.Work.CreateReference(context.Background(), &work.Reference{
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
	return ReferenceResolutionFixture{
		Path:                  path,
		RunID:                 runID,
		CitingRevisionID:      citingRevisionID,
		ExternalMentionID:     externalMentionID,
		ResolvedMentionID:     resolvedMentionID,
		NormalizedTargetID:    normalizedTargetID,
		NormalizedTargetTitle: "Target normalized title",
	}
}

// ViewerArticleActivityFixture supports the package test suite's viewer article activity fixture setup or assertions.
func ViewerArticleActivityFixture(t *testing.T) ArticleActivityFixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "workspace.db")
	db, err := testfixture.Open(path, testenv.Path("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	createWork := func(doi string) int64 {
		t.Helper()
		id, err := db.Work.CreateWorkByDOI(context.Background(), doi)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	createRevision := func(workID, runID int64, stage, title string) int64 {
		t.Helper()
		id, err := db.Work.CreateRevision(context.Background(), &work.Revision{
			WorkID: workID, PipelineRunID: runID, ProducerStage: stage, Title: title,
		})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	setStage := func(runID, workID int64, stage, outcome, reason string) {
		t.Helper()
		if err := db.Work.SetStageOutcome(context.Background(), work.StageOutcomeInput{
			RunID: runID, WorkID: workID, StageName: stage, Outcome: outcome, Reason: reason,
		}); err != nil {
			t.Fatal(err)
		}
	}
	insertAudit := func(runID any, entityType string, entityID int64, action, metadata, correlationID string) {
		t.Helper()
		if _, err := db.Conn.Exec(`INSERT INTO audit_events
			(occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, metadata_json, correlation_id)
			VALUES (datetime('now'), 'fixture', ?, ?, ?, ?, ?, ?)`,
			runID, entityType, fmt.Sprint(entityID), action, metadata, correlationID); err != nil {
			t.Fatal(err)
		}
	}
	previousRunID, err := testfixture.Run(context.Background(), db.Conn, "viewer", "prior article activity")
	if err != nil {
		t.Fatal(err)
	}
	runID, err := testfixture.Run(context.Background(), db.Conn, "viewer", "article activity")
	if err != nil {
		t.Fatal(err)
	}
	workID := createWork("10.1000/viewer-activity")
	snapshots := []struct {
		ProducerStage string
		StageName     string
		Outcome       string
		Title         string
	}{
		{work.ProducerStageParse, work.StageNameParse, work.OutcomeParsed, "Activity parse title"},
		{work.ProducerStageDeduplicate, work.StageNameDeduplicate, work.OutcomeDeduplicated, "Activity deduplicated title"},
		{work.ProducerStageEnrichMetadata, work.StageNameEnrichMetadata, work.OutcomeEnriched, "Activity metadata title"},
		{work.ProducerStageEnrichIdentity, work.StageNameEnrichIdentity, work.OutcomeEnriched, "Activity identity title"},
		{work.ProducerStageValidate, work.StageNameValidate, work.OutcomeValid, "Activity validated title"},
		{work.ProducerStageNormalize, work.StageNameNormalize, work.OutcomeNormalized, "Activity normalized title"},
	}
	var metadataRevisionID, identityRevisionID, normalizedRevisionID int64
	for _, snapshot := range snapshots {
		revisionID := createRevision(workID, runID, snapshot.ProducerStage, snapshot.Title)
		setStage(runID, workID, snapshot.StageName, snapshot.Outcome, "")
		switch snapshot.ProducerStage {
		case work.ProducerStageEnrichMetadata:
			metadataRevisionID = revisionID
		case work.ProducerStageEnrichIdentity:
			identityRevisionID = revisionID
		case work.ProducerStageNormalize:
			normalizedRevisionID = revisionID
		}
	}
	insertAudit(runID, "work_revision", metadataRevisionID, "field_enriched", `{"field":"title","provider":"crossref"}`, "current-metadata-enrichment")
	insertAudit(runID, "work_revision", identityRevisionID, "field_enriched", `{"field":"orcid","provider":"orcid"}`, "current-identity-enrichment")
	insertAudit(runID, "work", workID, "validation_changed", `{"stage":"validate","outcome":"valid"}`, "current-validation")
	insertAudit(previousRunID, "work", workID, "validation_changed", `{"stage":"validate","outcome":"discarded"}`, "previous-validation")
	insertAudit(nil, "work", workID, "pdf_document_inventoried", `{}`, "manual-pdf")

	discardedWorkID := createWork("10.1000/viewer-activity-discarded")
	createRevision(discardedWorkID, runID, work.ProducerStageParse, "Discarded parse title")
	setStage(runID, discardedWorkID, work.StageNameParse, work.OutcomeParsed, "")
	discardedRevisionID := createRevision(discardedWorkID, runID, work.ProducerStageValidate, "Discarded validation title")
	discardedReason := `["DOI is missing","Publication year is invalid"]`
	setStage(runID, discardedWorkID, work.StageNameValidate, work.OutcomeDiscarded, discardedReason)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return ArticleActivityFixture{
		Path:                 path,
		RunID:                runID,
		NormalizedRevisionID: normalizedRevisionID,
		DiscardedRevisionID:  discardedRevisionID,
		DiscardedReason:      discardedReason,
	}
}

// ViewerRequest supports the package test suite's viewer request setup or assertions.
func ViewerRequest(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	return response
}

// RequestJSON supports the package test suite's request json setup or assertions.
func RequestJSON(t *testing.T, handler http.Handler, path string) (int, map[string]any) {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %s response: %v\n%s", path, err, recorder.Body.String())
	}
	return recorder.Code, body
}

// NewPDFViewerFixture supports the package test suite's new pdf viewer fixture setup or assertions.
func NewPDFViewerFixture(t *testing.T) PdfViewerFixture {
	t.Helper()
	ctx := context.Background()
	tempDir := t.TempDir()
	registry := testenv.Path("config", "database.something")
	metadataPath := filepath.Join(tempDir, "corpus.metadata.db")
	metadata, err := testfixture.Open(metadataPath, registry)
	if err != nil {
		t.Fatal(err)
	}
	create := func(doi string) int64 {
		id, err := metadata.Work.CreateWorkByDOI(ctx, doi)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	availableID := create("10.1000/viewer-available")
	notAvailableID := create("10.1000/viewer-not-available")
	unavailableID, err := metadata.Work.CreateWorkWithoutDOI(ctx)
	if err != nil {
		t.Fatal(err)
	}
	searchID, err := metadata.Search.FindOrCreateSearch(ctx, "viewer-pdf-search")
	if err != nil {
		t.Fatal(err)
	}
	searchRevisionID, _, err := metadata.Search.UpsertRevision(ctx, search.RevisionInput{
		SearchID: searchID, RevisionLabel: "r1", ConfigArtifactHash: "config", ResolvedManifestHash: "manifest",
	})
	if err != nil {
		t.Fatal(err)
	}
	planID, err := metadata.Search.FindOrCreatePlanWithInputManifest(ctx, search.PlanInput{
		SearchRevisionID: searchRevisionID, ExecutionFingerprint: "viewer-pdf-fingerprint",
		ResolvedManifestHash: "manifest", InputManifestHash: "input",
	})
	if err != nil {
		t.Fatal(err)
	}
	pipelineRunID, _, err := metadata.Run.StartAttemptIfIdle(ctx, run.StartAttemptInput{ExecutionPlanID: planID, Step: "viewer", SearchQuery: "query"})
	if err != nil {
		t.Fatal(err)
	}
	if err := metadata.Run.InsertReviewer(ctx, run.ReviewerInput{RunID: pipelineRunID, Username: "Fixture Reviewer", Email: "fixture@example.test"}); err != nil {
		t.Fatal(err)
	}
	revisionID, err := metadata.Work.CreateRevision(ctx, &work.Revision{
		WorkID: availableID, PipelineRunID: pipelineRunID, ProducerStage: work.ProducerStageNormalize,
		Title: "Viewer PDF article",
	})
	if err != nil {
		t.Fatal(err)
	}
	notAvailableRevisionID, err := metadata.Work.CreateRevision(ctx, &work.Revision{
		WorkID: notAvailableID, PipelineRunID: pipelineRunID, ProducerStage: work.ProducerStageNormalize,
		Title: "Viewer PDF article without inventory content",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, workID := range []int64{availableID, notAvailableID} {
		if err := metadata.Work.SetStageOutcome(ctx, work.StageOutcomeInput{
			RunID: pipelineRunID, WorkID: workID, StageName: work.StageNameValidate, Outcome: work.OutcomeValid,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := metadata.Run.FinishRun(ctx, pipelineRunID, "completed", "fixture complete"); err != nil {
		t.Fatal(err)
	}
	bindings := metadata.PDFBindings
	if err := bindings.EnsureBinding(ctx, "corpus.pdf.db"); err != nil {
		t.Fatal(err)
	}
	store, err := pdfstore.Open(filepath.Join(tempDir, "corpus.pdf.db"), registry)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		Doi string
		Id  int64
	}{
		{"10.1000/viewer-available", availableID},
		{"10.1000/viewer-not-available", notAvailableID},
	} {
		if _, err := store.Register(ctx, item.Doi, item.Id, pipelineRunID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.Save(ctx, pdfstore.SaveInput{DOI: "10.1000/viewer-available", WorkID: availableID, ExpectedChangeCounter: 1, Data: pdffixture.Bytes("Selectable viewer methods on page one", "Selectable viewer results on page two")}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.FlushAuditOutbox(ctx, bindings); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := metadata.Close(); err != nil {
		t.Fatal(err)
	}
	viewer, err := OpenViewer(t, metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = viewer.Close() })
	return PdfViewerFixture{
		Server: viewer, MetadataPath: metadataPath, RunID: pipelineRunID, AvailableID: availableID,
		NotAvailableID: notAvailableID, UnavailableID: unavailableID, RevisionID: revisionID,
		NotAvailableRevisionID: notAvailableRevisionID,
	}
}

// Viewer keeps the real server separate from test-owned fixture and committed-state inspection connections.
type Viewer struct {
	*subject.Server
	Metadata   *testfixture.Database
	Read       *sql.DB
	ReadStores *database.Database
	PDF        *sql.DB
}

// OpenViewer exercises the actual existing-only server opener before allocating independent test-owned inspection handles.
func OpenViewer(t *testing.T, path string) (*Viewer, error) {
	t.Helper()
	actual, err := subject.Open(path, testenv.Path("config", "database.something"))
	if err != nil {
		return nil, err
	}
	metadata, err := testfixture.ConnectExisting(path, "sqlite")
	if err != nil {
		_ = actual.Close()
		return nil, err
	}
	read, err := sqliteread.Open(path, "analysis-viewer-budgeted-sqlite")
	if err != nil {
		_ = actual.Close()
		_ = metadata.Close()
		return nil, err
	}
	viewer := &Viewer{Server: actual, Metadata: metadata, Read: read, ReadStores: database.New(read)}
	if actual.PDFStoreBound() {
		relative, _, err := metadata.PDFBindings.Binding(t.Context())
		if err != nil {
			_ = viewer.Close()
			return nil, err
		}
		viewer.PDF, err = sqliteread.Open(filepath.Join(filepath.Dir(path), relative), "analysis-viewer-budgeted-sqlite")
		if err != nil {
			_ = viewer.Close()
			return nil, err
		}
	}
	t.Cleanup(func() { _ = viewer.Close() })
	return viewer, nil
}

// Close releases test-owned handles and the actual server's independent connections.
func (v *Viewer) Close() error {
	if v.PDF != nil {
		_ = v.PDF.Close()
	}
	_ = v.Read.Close()
	_ = v.Metadata.Close()
	return v.Server.Close()
}
