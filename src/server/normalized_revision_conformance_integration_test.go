// normalized_revision_conformance_integration_test.go verifies every
// normalized-revision predicate copy selects the same analysis-ready revisions
// for accepted, discarded, superseded, and missing-normalization cases.
//go:build integration

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"testing"

	"analysis/database"
	"analysis/database/author"
	"analysis/database/work"
)

// normalizedRevisionConformanceFixture is one migrated run containing an
// accepted work, a discarded work, a superseded work, and a work without a
// normalize revision.
type normalizedRevisionConformanceFixture struct {
	server                     *Server
	db                         *database.Database
	runID                      int64
	acceptedWorkID             int64
	acceptedRevisionID         int64
	discardedWorkID            int64
	discardedRevisionID        int64
	supersededWorkID           int64
	supersededRevisionID       int64
	supersededLatestRevisionID int64
	missingWorkID              int64
	missingRevisionID          int64
	supersededReferenceID      int64
	discardedReferenceID       int64
}

// newNormalizedRevisionConformanceFixture seeds the four predicate cases and
// opens both the viewer and a writable metadata connection over the same file.
func newNormalizedRevisionConformanceFixture(t *testing.T) *normalizedRevisionConformanceFixture {
	t.Helper()
	registry := filepath.Join("..", "..", "config", "database.something")
	metadataPath := filepath.Join(t.TempDir(), "corpus.metadata.db")
	metadata, err := database.Open(metadataPath, registry)
	if err != nil {
		t.Fatal(err)
	}
	exec := func(query string, args ...any) int64 {
		t.Helper()
		result, err := metadata.DB.Exec(query, args...)
		if err != nil {
			t.Fatalf("fixture query %q: %v", query, err)
		}
		id, _ := result.LastInsertId()
		return id
	}
	searchID, err := metadata.Searches.Create("normalized-conformance-search")
	if err != nil {
		t.Fatal(err)
	}
	searchRevisionID, _, err := metadata.Revisions.Create(searchID, "r1", "config", "manifest")
	if err != nil {
		t.Fatal(err)
	}
	planID, err := metadata.Plans.CreateWithInputManifest(searchRevisionID, "normalized-conformance-fingerprint", "manifest", "input", false)
	if err != nil {
		t.Fatal(err)
	}
	runID, _, err := metadata.PipelineRuns.StartAttempt(planID, "normalized-conformance", "query")
	if err != nil {
		t.Fatal(err)
	}
	if err := metadata.PipelineRuns.FinishRun(runID, "completed", "fixture complete"); err != nil {
		t.Fatal(err)
	}
	createWork := func(doi string) int64 {
		t.Helper()
		id, err := metadata.Works.CreateByDOI(doi)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	createRevision := func(workID int64, stage, title string) int64 {
		t.Helper()
		id, err := metadata.WorkRevisions.Create(&database.WorkRevision{
			WorkID: workID, PipelineRunID: runID, ProducerStage: stage, Title: title,
		})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	setStage := func(workID int64, stage, outcome string) {
		t.Helper()
		if err := metadata.RunWorkStages.SetOutcome(runID, workID, stage, outcome, ""); err != nil {
			t.Fatal(err)
		}
	}

	acceptedWorkID := createWork("10.1000/conformance-accepted")
	acceptedRevisionID := createRevision(acceptedWorkID, database.ProducerStageNormalize, "Accepted article")
	setStage(acceptedWorkID, database.StageNameValidate, database.OutcomeValid)

	discardedWorkID := createWork("10.1000/conformance-discarded")
	discardedRevisionID := createRevision(discardedWorkID, database.ProducerStageNormalize, "Discarded article")
	setStage(discardedWorkID, database.StageNameValidate, database.OutcomeDiscarded)

	supersededWorkID := createWork("10.1000/conformance-superseded")
	supersededRevisionID := createRevision(supersededWorkID, database.ProducerStageNormalize, "Superseded article")
	supersededLatestRevisionID := createRevision(supersededWorkID, database.ProducerStageNormalize, "Superseding article")
	setStage(supersededWorkID, database.StageNameValidate, database.OutcomeValid)

	missingWorkID := createWork("10.1000/conformance-missing")
	missingRevisionID := createRevision(missingWorkID, database.ProducerStageParse, "Missing normalization article")
	setStage(missingWorkID, database.StageNameValidate, database.OutcomeValid)

	authorID := exec("INSERT INTO author_occurrences (citation_name) VALUES ('Conformance, Author')")
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order) VALUES (?, ?, 1)", acceptedRevisionID, authorID)

	supersededReferenceID, err := metadata.ReferenceMentions.Create(&database.ReferenceMention{
		WorkRevisionID: acceptedRevisionID, MentionOrder: 1, RawReference: "superseded reference",
		DOI: "10.1000/conformance-superseded",
	})
	if err != nil {
		t.Fatal(err)
	}
	discardedReferenceID, err := metadata.ReferenceMentions.Create(&database.ReferenceMention{
		WorkRevisionID: acceptedRevisionID, MentionOrder: 2, RawReference: "discarded reference",
		DOI: "10.1000/conformance-discarded",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := metadata.ReferenceMentions.Create(&database.ReferenceMention{
		WorkRevisionID: supersededLatestRevisionID, MentionOrder: 1, RawReference: "superseding reference",
		DOI: "10.1000/conformance-accepted",
	}); err != nil {
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
	db, err := database.Open(metadataPath, registry)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	return &normalizedRevisionConformanceFixture{
		server: viewer, db: db, runID: runID,
		acceptedWorkID: acceptedWorkID, acceptedRevisionID: acceptedRevisionID,
		discardedWorkID: discardedWorkID, discardedRevisionID: discardedRevisionID,
		supersededWorkID: supersededWorkID, supersededRevisionID: supersededRevisionID,
		supersededLatestRevisionID: supersededLatestRevisionID,
		missingWorkID:              missingWorkID, missingRevisionID: missingRevisionID,
		supersededReferenceID: supersededReferenceID, discardedReferenceID: discardedReferenceID,
	}
}

// TestNormalizedRevisionPredicateConformance compares every predicate copy
// against the same accepted, discarded, superseded, and missing-normalization
// fixture so duplicated SQL cannot silently change semantics.
func TestNormalizedRevisionPredicateConformance(t *testing.T) {
	fixture := newNormalizedRevisionConformanceFixture(t)
	expected := revisionSet(fixture.acceptedRevisionID, fixture.supersededLatestRevisionID)

	t.Run("server corpus articles", func(t *testing.T) {
		assertRevisionSet(t, "server corpus articles", fixture.corpusArticleRevisionIDs(t), expected)
	})
	t.Run("server corpus references", func(t *testing.T) {
		assertRevisionSet(t, "server corpus references", fixture.corpusReferenceRevisionIDs(t), expected)
	})
	t.Run("server evaluation", func(t *testing.T) {
		assertRevisionSet(t, "server evaluation", fixture.evaluationRevisionIDs(t), expected)
	})
	t.Run("server graph", func(t *testing.T) {
		assertRevisionSet(t, "server graph", fixture.graphRevisionIDs(t), expected)
	})
	t.Run("server overview totals", func(t *testing.T) {
		status, body := requestJSON(t, fixture.server.Handler(), fmt.Sprintf("/api/overview?run_id=%d", fixture.runID))
		if status != http.StatusOK {
			t.Fatalf("overview status=%d body=%v", status, body)
		}
		totals, ok := body["relationship_totals"].(map[string]any)
		if !ok {
			t.Fatalf("overview relationship totals = %v", body["relationship_totals"])
		}
		if value := totals["analysis_ready_articles"].(map[string]any)["value"]; value != float64(len(expected)) {
			t.Fatalf("analysis_ready_articles=%v, want %d", value, len(expected))
		}
		if value := totals["authorships"].(map[string]any)["value"]; value != float64(1) {
			t.Fatalf("authorships=%v, want 1", value)
		}
		if value := totals["reference_mentions"].(map[string]any)["value"]; value != float64(3) {
			t.Fatalf("reference_mentions=%v, want 3", value)
		}
	})
	t.Run("server article detail", func(t *testing.T) {
		if status := fixture.articleDetailStatus(t, fixture.acceptedRevisionID); status != http.StatusOK {
			t.Fatalf("accepted article detail status=%d, want 200", status)
		}
		if status := fixture.articleDetailStatus(t, fixture.supersededLatestRevisionID); status != http.StatusOK {
			t.Fatalf("superseding article detail status=%d, want 200", status)
		}
		if status := fixture.articleDetailStatus(t, fixture.supersededRevisionID); status != http.StatusNotFound {
			t.Fatalf("superseded article detail status=%d, want 404", status)
		}
		if status := fixture.articleDetailStatus(t, fixture.discardedRevisionID); status != http.StatusNotFound {
			t.Fatalf("discarded article detail status=%d, want 404", status)
		}
		if status := fixture.articleDetailStatus(t, fixture.missingRevisionID); status != http.StatusOK {
			t.Fatalf("non-normalize article detail status=%d, want 200", status)
		}
	})
	t.Run("server reference detail", func(t *testing.T) {
		resolved := fixture.referenceResolvedRevisionID(t, fixture.supersededReferenceID)
		if resolved == nil || *resolved != fixture.supersededLatestRevisionID {
			t.Fatalf("superseded reference resolved revision=%v, want %d", resolved, fixture.supersededLatestRevisionID)
		}
		if resolved := fixture.referenceResolvedRevisionID(t, fixture.discardedReferenceID); resolved != nil {
			t.Fatalf("discarded reference resolved revision=%v, want nil", *resolved)
		}
	})
	t.Run("server review article", func(t *testing.T) {
		if status := fixture.reviewArticleStatus(t, fixture.acceptedRevisionID); status != http.StatusOK {
			t.Fatalf("accepted review article status=%d, want 200", status)
		}
		if status := fixture.reviewArticleStatus(t, fixture.supersededLatestRevisionID); status != http.StatusOK {
			t.Fatalf("superseding review article status=%d, want 200", status)
		}
		if status := fixture.reviewArticleStatus(t, fixture.supersededRevisionID); status != http.StatusNotFound {
			t.Fatalf("superseded review article status=%d, want 404", status)
		}
		if status := fixture.reviewArticleStatus(t, fixture.discardedRevisionID); status != http.StatusNotFound {
			t.Fatalf("discarded review article status=%d, want 404", status)
		}
	})
	t.Run("work evaluation", func(t *testing.T) {
		assertRevisionSet(t, "work evaluation", fixture.workEvaluationRevisionIDs(t), expected)
	})
	t.Run("work graph", func(t *testing.T) {
		assertRevisionSet(t, "work graph", fixture.workGraphRevisionIDs(t), expected)
	})
	t.Run("work corpus references", func(t *testing.T) {
		assertRevisionSet(t, "work corpus references", fixture.workCorpusReferenceRevisionIDs(t), expected)
	})
	t.Run("work article revision", func(t *testing.T) {
		if !fixture.workArticleRevisionVisible(t, fixture.acceptedRevisionID) {
			t.Fatal("accepted revision was not visible")
		}
		if !fixture.workArticleRevisionVisible(t, fixture.supersededLatestRevisionID) {
			t.Fatal("superseding revision was not visible")
		}
		if fixture.workArticleRevisionVisible(t, fixture.supersededRevisionID) {
			t.Fatal("superseded revision was visible")
		}
		if fixture.workArticleRevisionVisible(t, fixture.discardedRevisionID) {
			t.Fatal("discarded revision was visible")
		}
		if !fixture.workArticleRevisionVisible(t, fixture.missingRevisionID) {
			t.Fatal("non-normalize revision was not visible")
		}
	})
	t.Run("work article detail work id", func(t *testing.T) {
		if workID, err := fixture.db.Work.GetArticleDetailWorkID(context.Background(), fixture.acceptedRevisionID, fixture.runID); err != nil || workID != fixture.acceptedWorkID {
			t.Fatalf("accepted detail work id=%d err=%v, want %d", workID, err, fixture.acceptedWorkID)
		}
		if _, err := fixture.db.Work.GetArticleDetailWorkID(context.Background(), fixture.supersededRevisionID, fixture.runID); err == nil {
			t.Fatal("superseded revision was accepted by the detail guard")
		}
		if _, err := fixture.db.Work.GetArticleDetailWorkID(context.Background(), fixture.discardedRevisionID, fixture.runID); err == nil {
			t.Fatal("discarded revision was accepted by the detail guard")
		}
	})
	t.Run("work evaluation summary", func(t *testing.T) {
		summary, err := fixture.db.Work.EvaluationReviewSummary(context.Background(), work.EvaluationFilter{
			RunID: fixture.runID, AvailableDOIsJSON: "[]",
		})
		if err != nil {
			t.Fatal(err)
		}
		if summary.Total != int64(len(expected)) {
			t.Fatalf("evaluation summary total=%d, want %d", summary.Total, len(expected))
		}
	})
	t.Run("work relationship totals", func(t *testing.T) {
		totals, err := fixture.db.Work.RelationshipTotals(context.Background(), fixture.runID)
		if err != nil {
			t.Fatal(err)
		}
		if totals.AnalysisReadyArticles != int64(len(expected)) {
			t.Fatalf("analysis-ready articles=%d, want %d", totals.AnalysisReadyArticles, len(expected))
		}
		if totals.Authorships != 1 {
			t.Fatalf("authorships=%d, want 1", totals.Authorships)
		}
		if totals.ReferenceMentions != 3 {
			t.Fatalf("reference mentions=%d, want 3", totals.ReferenceMentions)
		}
	})
	t.Run("work queue navigation", func(t *testing.T) {
		if _, err := fixture.db.Work.EvaluationQueueNavigation(context.Background(), work.EvaluationFilter{
			RunID: fixture.runID, SortField: "id", Order: "ASC", CurrentRevisionID: fixture.acceptedRevisionID,
		}); err != nil {
			t.Fatalf("accepted queue navigation: %v", err)
		}
		if _, err := fixture.db.Work.EvaluationQueueNavigation(context.Background(), work.EvaluationFilter{
			RunID: fixture.runID, SortField: "id", Order: "ASC", CurrentRevisionID: fixture.discardedRevisionID,
		}); err == nil {
			t.Fatal("discarded revision was accepted by queue navigation")
		}
	})
	t.Run("work reference detail", func(t *testing.T) {
		detail, err := fixture.db.Work.GetReferenceDetail(context.Background(), fixture.supersededReferenceID, fixture.runID)
		if err != nil {
			t.Fatal(err)
		}
		if detail == nil || detail.ResolvedRevisionID == nil || *detail.ResolvedRevisionID != fixture.supersededLatestRevisionID {
			t.Fatalf("superseded reference detail=%+v, want resolved revision %d", detail, fixture.supersededLatestRevisionID)
		}
		discarded, err := fixture.db.Work.GetReferenceDetail(context.Background(), fixture.discardedReferenceID, fixture.runID)
		if err != nil {
			t.Fatal(err)
		}
		if discarded == nil || discarded.ResolvedRevisionID != nil {
			t.Fatalf("discarded reference detail=%+v, want no resolved revision", discarded)
		}
	})
	t.Run("work article references", func(t *testing.T) {
		page, err := fixture.db.Work.ListArticleReferences(context.Background(), work.ArticleReferencePageInput{
			RevisionID: fixture.acceptedRevisionID, Limit: 20,
		})
		if err != nil {
			t.Fatal(err)
		}
		if page.Total != 2 || len(page.Items) != 2 {
			t.Fatalf("article references total=%d items=%d, want 2", page.Total, len(page.Items))
		}
		for _, reference := range page.Items {
			switch reference.ID {
			case fixture.supersededReferenceID:
				if reference.ResolvedRevisionID == nil || *reference.ResolvedRevisionID != fixture.supersededLatestRevisionID {
					t.Fatalf("superseded reference resolved revision=%v, want %d", reference.ResolvedRevisionID, fixture.supersededLatestRevisionID)
				}
			case fixture.discardedReferenceID:
				if reference.ResolvedRevisionID != nil {
					t.Fatalf("discarded reference resolved revision=%v, want nil", *reference.ResolvedRevisionID)
				}
			default:
				t.Fatalf("unexpected reference %d", reference.ID)
			}
		}
	})
	t.Run("review context heads", func(t *testing.T) {
		contextRecord, created, err := fixture.db.Reviews.CreateContext(context.Background(), fixture.runID, nil)
		if err != nil || !created {
			t.Fatalf("create review context: context=%+v created=%v err=%v", contextRecord, created, err)
		}
		heads := map[int64]int64{}
		rows, err := fixture.db.DB.Query(`SELECT work_id, work_revision_id FROM review_context_work_heads
			WHERE review_context_id=? ORDER BY work_id`, contextRecord.ID)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var workID, revisionID int64
			if err := rows.Scan(&workID, &revisionID); err != nil {
				t.Fatal(err)
			}
			heads[workID] = revisionID
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		want := map[int64]int64{
			fixture.acceptedWorkID:   fixture.acceptedRevisionID,
			fixture.supersededWorkID: fixture.supersededLatestRevisionID,
		}
		if len(heads) != len(want) {
			t.Fatalf("review context heads=%v, want %v", heads, want)
		}
		for workID, revisionID := range want {
			if heads[workID] != revisionID {
				t.Fatalf("review context head for work %d=%d, want %d", workID, heads[workID], revisionID)
			}
		}
	})
	t.Run("author identity evidence revisions", func(t *testing.T) {
		fixture.assertAuthorIdentityEvidenceRevisions(t)
	})
}

// assertAuthorIdentityEvidenceRevisions seeds one resolution per predicate case
// and verifies the author family evidence read selects the same analysis-ready
// revision rule: the current normalized revision when the predicate matches,
// and the captured evidence revision otherwise.
func (f *normalizedRevisionConformanceFixture) assertAuthorIdentityEvidenceRevisions(t *testing.T) {
	t.Helper()
	seed := func(name string, revisionID int64, authorOrder int) int64 {
		t.Helper()
		occurrence, err := f.db.DB.Exec("INSERT INTO author_occurrences (citation_name) VALUES (?)", name)
		if err != nil {
			t.Fatal(err)
		}
		occurrenceID, err := occurrence.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.db.DB.Exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order) VALUES (?, ?, ?)", revisionID, occurrenceID, authorOrder); err != nil {
			t.Fatal(err)
		}
		resolution, err := f.db.DB.Exec(`INSERT INTO author_identity_resolutions
			(pipeline_run_id, author_occurrence_id, status, provider, queried_citation_name, resolved_at)
			VALUES (?, ?, 'orcid_is_unclear', 'orcid', ?, '2026-01-01T00:00:00Z')`, f.runID, occurrenceID, name)
		if err != nil {
			t.Fatal(err)
		}
		resolutionID, err := resolution.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		return resolutionID
	}

	acceptedResolution := seed("Accepted Evidence", f.acceptedRevisionID, 2)
	supersededResolution := seed("Superseded Evidence", f.supersededRevisionID, 1)
	discardedResolution := seed("Discarded Evidence", f.discardedRevisionID, 1)
	missingResolution := seed("Missing Evidence", f.missingRevisionID, 1)

	page, err := f.server.authorStore.ListIdentityEvidence(context.Background(), author.IdentityEvidenceFilter{
		RunID: f.runID, Sort: "id", Order: "ASC", Page: 1, PerPage: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	byResolution := map[int64]*author.IdentityEvidenceRow{}
	for _, row := range page.Items {
		byResolution[row.ResolutionID] = row
	}
	want := map[int64]int64{
		acceptedResolution:   f.acceptedRevisionID,
		supersededResolution: f.supersededLatestRevisionID,
		discardedResolution:  f.discardedRevisionID,
		missingResolution:    f.missingRevisionID,
	}
	if len(byResolution) != len(want) {
		t.Fatalf("identity evidence rows = %d, want %d", len(byResolution), len(want))
	}
	for resolutionID, revisionID := range want {
		row := byResolution[resolutionID]
		if row == nil || row.WorkRevisionID == nil || *row.WorkRevisionID != revisionID {
			t.Fatalf("resolution %d work revision = %+v, want %d", resolutionID, row, revisionID)
		}
	}
}

// corpusArticleRevisionIDs collects the citing revisions returned by the corpus article endpoint.
func (f *normalizedRevisionConformanceFixture) corpusArticleRevisionIDs(t *testing.T) map[int64]bool {
	t.Helper()
	status, body := requestJSON(t, f.server.Handler(), fmt.Sprintf("/api/runs/%d/corpus/articles?per_page=20", f.runID))
	if status != http.StatusOK {
		t.Fatalf("corpus articles status=%d body=%v", status, body)
	}
	return revisionIDsFromRows(t, body)
}

// corpusReferenceRevisionIDs collects the citing revisions returned by the corpus reference endpoint.
func (f *normalizedRevisionConformanceFixture) corpusReferenceRevisionIDs(t *testing.T) map[int64]bool {
	t.Helper()
	status, body := requestJSON(t, f.server.Handler(), fmt.Sprintf("/api/runs/%d/corpus/references?per_page=20", f.runID))
	if status != http.StatusOK {
		t.Fatalf("corpus references status=%d body=%v", status, body)
	}
	return revisionIDsFromRows(t, body)
}

// evaluationRevisionIDs collects the revisions returned by the evaluation endpoint.
func (f *normalizedRevisionConformanceFixture) evaluationRevisionIDs(t *testing.T) map[int64]bool {
	t.Helper()
	status, body := requestJSON(t, f.server.Handler(), fmt.Sprintf("/api/runs/%d/evaluation?per_page=20", f.runID))
	if status != http.StatusOK {
		t.Fatalf("evaluation status=%d body=%v", status, body)
	}
	return revisionIDsFromRows(t, body)
}

// graphRevisionIDs collects the article node revisions returned by the graph endpoint.
func (f *normalizedRevisionConformanceFixture) graphRevisionIDs(t *testing.T) map[int64]bool {
	t.Helper()
	response := viewerRequest(t, f.server.Handler(), fmt.Sprintf("/api/graph?run_id=%d&mode=article_author", f.runID))
	if response.Code != http.StatusOK {
		t.Fatalf("graph status=%d body=%s", response.Code, response.Body.String())
	}
	var graph struct {
		Nodes []map[string]any `json:"nodes"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &graph); err != nil {
		t.Fatal(err)
	}
	result := map[int64]bool{}
	for _, node := range graph.Nodes {
		if node["type"] != "article" {
			continue
		}
		result[int64(node["revision_id"].(float64))] = true
	}
	return result
}

// articleDetailStatus returns the article detail status for one revision.
func (f *normalizedRevisionConformanceFixture) articleDetailStatus(t *testing.T, revisionID int64) int {
	t.Helper()
	return viewerRequest(t, f.server.Handler(), fmt.Sprintf("/api/articles/%d?run_id=%d", revisionID, f.runID)).Code
}

// referenceResolvedRevisionID returns the resolved revision of one reference detail.
func (f *normalizedRevisionConformanceFixture) referenceResolvedRevisionID(t *testing.T, referenceID int64) *int64 {
	t.Helper()
	status, body := requestJSON(t, f.server.Handler(), fmt.Sprintf("/api/references/%d?run_id=%d", referenceID, f.runID))
	if status != http.StatusOK {
		t.Fatalf("reference detail status=%d body=%v", status, body)
	}
	reference, ok := body["reference"].(map[string]any)
	if !ok {
		t.Fatalf("reference detail body=%v", body)
	}
	if reference["resolved_revision_id"] == nil {
		return nil
	}
	resolved := int64(reference["resolved_revision_id"].(float64))
	return &resolved
}

// reviewArticleStatus returns the review article status for one revision.
func (f *normalizedRevisionConformanceFixture) reviewArticleStatus(t *testing.T, revisionID int64) int {
	t.Helper()
	return viewerRequest(t, f.server.Handler(), fmt.Sprintf("/api/runs/%d/articles/%d/review", f.runID, revisionID)).Code
}

// workEvaluationRevisionIDs collects the revisions returned by the work evaluation read.
func (f *normalizedRevisionConformanceFixture) workEvaluationRevisionIDs(t *testing.T) map[int64]bool {
	t.Helper()
	page, err := f.db.Work.ListEvaluation(context.Background(), work.EvaluationFilter{
		RunID: f.runID, SortField: "id", Order: "ASC", Page: 1, PerPage: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	result := map[int64]bool{}
	for _, item := range page.Items {
		result[item.WorkRevisionID] = true
	}
	return result
}

// workGraphRevisionIDs collects the article revisions returned by the work graph read.
func (f *normalizedRevisionConformanceFixture) workGraphRevisionIDs(t *testing.T) map[int64]bool {
	t.Helper()
	articles, _, err := f.db.Work.ListGraphArticles(context.Background(), work.GraphFilter{RunID: f.runID, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	result := map[int64]bool{}
	for _, article := range articles {
		result[article.ID] = true
	}
	return result
}

// workCorpusReferenceRevisionIDs collects the citing revisions returned by the work reference read.
func (f *normalizedRevisionConformanceFixture) workCorpusReferenceRevisionIDs(t *testing.T) map[int64]bool {
	t.Helper()
	page, err := f.db.Work.ListCorpusReferences(context.Background(), work.CorpusReferenceFilter{
		RunID: f.runID, Sort: "id", Order: "ASC", Page: 1, PerPage: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	result := map[int64]bool{}
	for _, item := range page.Items {
		result[item.WorkRevisionID] = true
	}
	return result
}

// workArticleRevisionVisible reports whether the work detail read returns one revision.
func (f *normalizedRevisionConformanceFixture) workArticleRevisionVisible(t *testing.T, revisionID int64) bool {
	t.Helper()
	revision, err := f.db.Work.GetArticleRevision(context.Background(), revisionID, f.runID)
	if err != nil {
		t.Fatal(err)
	}
	return revision != nil
}

// revisionIDsFromRows collects the work_revision_id values from one JSON row list.
func revisionIDsFromRows(t *testing.T, body map[string]any) map[int64]bool {
	t.Helper()
	rows, ok := body["rows"].([]any)
	if !ok {
		t.Fatalf("response rows = %v", body["rows"])
	}
	result := map[int64]bool{}
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("response row = %v", raw)
		}
		result[int64(row["work_revision_id"].(float64))] = true
	}
	return result
}

// revisionSet builds a revision-ID set.
func revisionSet(ids ...int64) map[int64]bool {
	result := make(map[int64]bool, len(ids))
	for _, id := range ids {
		result[id] = true
	}
	return result
}

// assertRevisionSet compares one predicate copy's selected revisions.
func assertRevisionSet(t *testing.T, name string, got, want map[int64]bool) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s selected revisions=%v, want %v", name, sortedRevisionIDs(got), sortedRevisionIDs(want))
	}
	for id := range want {
		if !got[id] {
			t.Fatalf("%s selected revisions=%v, want %v", name, sortedRevisionIDs(got), sortedRevisionIDs(want))
		}
	}
}

// sortedRevisionIDs returns revision IDs in ascending order for diagnostics.
func sortedRevisionIDs(ids map[int64]bool) []int64 {
	result := make([]int64, 0, len(ids))
	for id := range ids {
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}
