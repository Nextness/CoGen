// term_matches_integration_test.go tests the stored term-coverage payload on
// the article detail and run corpus endpoints.
//go:build integration

package server

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"analysis/database"
)

// termMatchesFixture is a viewer fixture with stored term data.
type termMatchesFixture struct {
	server          *Server
	runID           int64
	normalizedID    int64
	parseRevisionID int64
	emptyRunID      int64
	emptyRevisionID int64
}

// newTermMatchesFixture builds a viewer fixture with stored term data.
func newTermMatchesFixture(t *testing.T) termMatchesFixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "workspace.db")
	db, err := database.Open(path, filepath.Join("..", "..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	runID, err := db.PipelineRuns.StartRun("term matches", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.PipelineRuns.FinishRun(runID, "completed", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := db.RunSources.Create(runID, "scopus", "csv", "scopus.csv", `TITLE-ABS-KEY(("BPMN" OR "scheduling"))`, "", 0, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := db.RunSources.Create(runID, "wos", "bib", "wos.bib", `TS=("BPMN" OR "genetic algorithm" OR "metaheuristic")`, "", 0, ""); err != nil {
		t.Fatal(err)
	}
	workOne, err := db.Works.CreateByDOI("10.1000/term-one")
	if err != nil {
		t.Fatal(err)
	}
	normalizedID, err := db.WorkRevisions.Create(&database.WorkRevision{
		WorkID: workOne, PipelineRunID: runID, ProducerStage: database.ProducerStageNormalize,
		Title: "BPMN scheduling", Abstract: "genetic algorithm",
		Keywords: `["bpmn"]`, KeywordsPlus: `["scheduling"]`,
	})
	if err != nil {
		t.Fatal(err)
	}
	parseRevisionID, err := db.WorkRevisions.Create(&database.WorkRevision{
		WorkID: workOne, PipelineRunID: runID, ProducerStage: database.ProducerStageParse, Title: "BPMN scheduling raw",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.RunWorkStages.SetOutcome(runID, workOne, database.StageNameValidate, database.OutcomeValid, ""); err != nil {
		t.Fatal(err)
	}
	emptyRunID, err := db.PipelineRuns.StartRun("term matches empty", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.PipelineRuns.FinishRun(emptyRunID, "completed", ""); err != nil {
		t.Fatal(err)
	}
	emptyWork, err := db.Works.CreateByDOI("10.1000/term-empty")
	if err != nil {
		t.Fatal(err)
	}
	emptyRevisionID, err := db.WorkRevisions.Create(&database.WorkRevision{
		WorkID: emptyWork, PipelineRunID: emptyRunID, ProducerStage: database.ProducerStageNormalize, Title: "Empty run article",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.RunWorkStages.SetOutcome(emptyRunID, emptyWork, database.StageNameValidate, database.OutcomeValid, ""); err != nil {
		t.Fatal(err)
	}
	termsBySource := map[string][]string{"scopus": {"BPMN", "scheduling"}, "wos": {"BPMN", "genetic algorithm", "metaheuristic"}}
	matches := map[int64]map[string][]string{
		normalizedID: {"title": {"BPMN", "scheduling"}, "abstract": {"genetic algorithm"}, "keywords": {"BPMN"}, "keywords_plus": {"scheduling"}},
	}
	if err := db.TermMatches.ReplaceRunTermData(runID, termsBySource, matches); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = viewer.Close() })
	return termMatchesFixture{
		server: viewer, runID: runID, normalizedID: normalizedID, parseRevisionID: parseRevisionID,
		emptyRunID: emptyRunID, emptyRevisionID: emptyRevisionID,
	}
}

// TestArticleDetailTermMatches verifies the stored term-coverage payload on article detail.
func TestArticleDetailTermMatches(t *testing.T) {
	fx := newTermMatchesFixture(t)
	handler := fx.server.Handler()
	code, body := requestJSON(t, handler, "/api/articles/"+stringID(fx.normalizedID))
	if code != http.StatusOK {
		t.Fatalf("article detail status=%d body=%v", code, body)
	}
	matches, ok := body["term_matches"].(map[string]any)
	if !ok {
		t.Fatalf("term_matches missing or not an object: %#v", body["term_matches"])
	}
	if !reflect.DeepEqual(matches["title"], []any{"BPMN", "scheduling"}) {
		t.Fatalf("title matches = %v", matches["title"])
	}
	if !reflect.DeepEqual(matches["abstract"], []any{"genetic algorithm"}) {
		t.Fatalf("abstract matches = %v", matches["abstract"])
	}
	if !reflect.DeepEqual(matches["keywords"], []any{"BPMN"}) {
		t.Fatalf("keywords matches = %v", matches["keywords"])
	}
	if !reflect.DeepEqual(matches["keywords_plus"], []any{"scheduling"}) {
		t.Fatalf("keywords_plus matches = %v", matches["keywords_plus"])
	}
	if matches["matched_total"].(float64) != 3 {
		t.Fatalf("matched_total = %v, want 3", matches["matched_total"])
	}
	if matches["term_total"].(float64) != 4 {
		t.Fatalf("term_total = %v, want 4", matches["term_total"])
	}
	if !reflect.DeepEqual(matches["sources"], []any{"scopus", "wos"}) {
		t.Fatalf("sources = %v", matches["sources"])
	}
	termsWithSources, _ := matches["terms_with_sources"].(map[string]any)
	if !reflect.DeepEqual(termsWithSources["BPMN"], []any{"scopus", "wos"}) {
		t.Fatalf("terms_with_sources BPMN = %v", termsWithSources["BPMN"])
	}
	if !reflect.DeepEqual(matches["unmatched"], []any{"metaheuristic"}) {
		t.Fatalf("unmatched = %v", matches["unmatched"])
	}
}

// TestArticleDetailTermMatchesNullForNonNormalize verifies non-normalize revisions return null.
func TestArticleDetailTermMatchesNullForNonNormalize(t *testing.T) {
	fx := newTermMatchesFixture(t)
	handler := fx.server.Handler()
	code, body := requestJSON(t, handler, "/api/articles/"+stringID(fx.parseRevisionID))
	if code != http.StatusOK {
		t.Fatalf("article detail status=%d body=%v", code, body)
	}
	if body["term_matches"] != nil {
		t.Fatalf("term_matches = %#v, want null for a parse revision", body["term_matches"])
	}
}

// TestArticleDetailTermMatchesNullForEmptyRun verifies runs without stored data return null.
func TestArticleDetailTermMatchesNullForEmptyRun(t *testing.T) {
	fx := newTermMatchesFixture(t)
	handler := fx.server.Handler()
	code, body := requestJSON(t, handler, "/api/articles/"+stringID(fx.emptyRevisionID))
	if code != http.StatusOK {
		t.Fatalf("article detail status=%d body=%v", code, body)
	}
	if body["term_matches"] != nil {
		t.Fatalf("term_matches = %#v, want null for a run without stored terms", body["term_matches"])
	}
}

// TestRunCorpusArticlesTermMatches verifies corpus rows carry the compact payload.
func TestRunCorpusArticlesTermMatches(t *testing.T) {
	fx := newTermMatchesFixture(t)
	handler := fx.server.Handler()
	response := viewerRequest(t, handler, "/api/runs/"+stringID(fx.runID)+"/corpus/articles?page=1&per_page=20&sort=id&order=asc")
	if response.Code != http.StatusOK {
		t.Fatalf("run corpus status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Rows []map[string]any `json:"rows"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Rows) != 1 {
		t.Fatalf("corpus rows = %d, want 1", len(payload.Rows))
	}
	matches, ok := payload.Rows[0]["term_matches"].(map[string]any)
	if !ok {
		t.Fatalf("row term_matches missing or not an object: %#v", payload.Rows[0]["term_matches"])
	}
	if matches["matched_total"].(float64) != 3 || matches["term_total"].(float64) != 4 {
		t.Fatalf("row counts = %v/%v, want 3/4", matches["matched_total"], matches["term_total"])
	}
	if _, hasDetail := matches["unmatched"]; hasDetail {
		t.Fatalf("corpus row payload must not include unmatched: %#v", matches)
	}
}

// TestRunCorpusArticlesTermMatchesNullForEmptyRun verifies corpus rows for a run without stored data are null.
func TestRunCorpusArticlesTermMatchesNullForEmptyRun(t *testing.T) {
	fx := newTermMatchesFixture(t)
	handler := fx.server.Handler()
	response := viewerRequest(t, handler, "/api/runs/"+stringID(fx.emptyRunID)+"/corpus/articles?page=1&per_page=20&sort=id&order=asc")
	if response.Code != http.StatusOK {
		t.Fatalf("run corpus status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Rows []map[string]any `json:"rows"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Rows) != 1 {
		t.Fatalf("corpus rows = %d, want 1", len(payload.Rows))
	}
	if payload.Rows[0]["term_matches"] != nil {
		t.Fatalf("row term_matches = %#v, want null", payload.Rows[0]["term_matches"])
	}
}

// TestTermMatchesGuardedReadsOnUnmigratedDatabase verifies a database without
// the V00025 tables degrades to a null payload instead of failing.
func TestTermMatchesGuardedReadsOnUnmigratedDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace.db")
	db, err := database.Open(path, filepath.Join("..", "..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	runID, err := db.PipelineRuns.StartRun("unmigrated", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.PipelineRuns.FinishRun(runID, "completed", ""); err != nil {
		t.Fatal(err)
	}
	workID, err := db.Works.CreateByDOI("10.1000/unmigrated")
	if err != nil {
		t.Fatal(err)
	}
	revisionID, err := db.WorkRevisions.Create(&database.WorkRevision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: database.ProducerStageNormalize, Title: "Unmigrated article",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.RunWorkStages.SetOutcome(runID, workID, database.StageNameValidate, database.OutcomeValid, ""); err != nil {
		t.Fatal(err)
	}
	// Simulate a V00024-only database by dropping the V00025 tables.
	if _, err := db.DB.Exec("DROP TABLE work_revision_term_matches"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec("DROP TABLE run_search_terms"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	handler := viewer.Handler()
	code, body := requestJSON(t, handler, "/api/articles/"+stringID(revisionID))
	if code != http.StatusOK {
		t.Fatalf("article detail status=%d body=%v", code, body)
	}
	if body["term_matches"] != nil {
		t.Fatalf("term_matches = %#v, want null on an unmigrated database", body["term_matches"])
	}
	response := viewerRequest(t, handler, "/api/runs/"+stringID(runID)+"/corpus/articles")
	if response.Code != http.StatusOK {
		t.Fatalf("run corpus status=%d body=%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"term_matches":null`) {
		t.Fatalf("run corpus did not degrade to null term_matches: %s", response.Body.String())
	}
}
