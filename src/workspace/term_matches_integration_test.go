// term_matches_integration_test.go tests the workspace term-match computation,
// persistence, and reconciliation against temporary databases with production
// migrations.
//go:build integration

package workspace

import (
	"path/filepath"
	"reflect"
	"testing"

	"analysis/article"
	"analysis/database"
	"analysis/manifest"
)

// TestComputeAndPersistRunTermMatches verifies the new-run computation and persistence.
func TestComputeAndPersistRunTermMatches(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "workspace.db"), filepath.Join("..", "..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	runID, err := db.PipelineRuns.StartRun("term matches", "")
	if err != nil {
		t.Fatal(err)
	}
	run := &Run{Manifest: &manifest.ResolvedManifest{
		Sources: []manifest.SourceManifest{
			{Name: "scopus", Query: `TITLE-ABS-KEY(("BPMN" OR "scheduling"))`},
			{Name: "wos", Query: `TS=("BPMN" OR "genetic algorithm")`},
		},
	}}
	workOne, err := db.Works.CreateByDOI("10.1000/one")
	if err != nil {
		t.Fatal(err)
	}
	revOne, err := db.WorkRevisions.Create(&database.WorkRevision{
		WorkID: workOne, PipelineRunID: runID, ProducerStage: database.ProducerStageNormalize, Title: "BPMN scheduling",
	})
	if err != nil {
		t.Fatal(err)
	}
	workTwo, err := db.Works.CreateByDOI("10.1000/two")
	if err != nil {
		t.Fatal(err)
	}
	revTwo, err := db.WorkRevisions.Create(&database.WorkRevision{
		WorkID: workTwo, PipelineRunID: runID, ProducerStage: database.ProducerStageNormalize, Title: "Unrelated",
	})
	if err != nil {
		t.Fatal(err)
	}
	articles := []*article.Article{
		{DOI: "10.1000/one", Title: "BPMN scheduling", Abstract: "genetic algorithm", Keywords: []string{"bpmn"}, KeywordsAdditional: []string{"scheduling"}},
		{DOI: "10.1000/two", Title: "Unrelated", Abstract: "", Keywords: nil, KeywordsAdditional: nil},
	}
	revisionIDs := map[string]int64{"10.1000/one": revOne, "10.1000/two": revTwo}
	termsBySource, matches := computeRunTermMatches(run, articles, revisionIDs)
	if err := persistRunTermMatches(db, runID, termsBySource, matches); err != nil {
		t.Fatal(err)
	}
	terms, err := db.TermMatches.GetRunTerms(runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(terms) != 4 {
		t.Fatalf("run_search_terms = %d, want 4: %+v", len(terms), terms)
	}
	got, err := db.TermMatches.GetRevisionMatches(runID, revOne)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"title":         {"BPMN", "scheduling"},
		"abstract":      {"genetic algorithm"},
		"keywords":      {"BPMN"},
		"keywords_plus": {"scheduling"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("revision one matches = %v, want %v", got, want)
	}
	gotTwo, err := db.TermMatches.GetRevisionMatches(runID, revTwo)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotTwo) != 0 {
		t.Fatalf("revision two matches = %v, want none", gotTwo)
	}
}

// TestReconcileStoredTermMatchesBackfills verifies the reuse-path backfill
// including the keyword raw-text fallback and JSON-null treatment.
func TestReconcileStoredTermMatchesBackfills(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "workspace.db"), filepath.Join("..", "..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	runID, err := db.PipelineRuns.StartRun("reconcile", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.PipelineRuns.FinishRun(runID, "completed", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := db.RunSources.Create(runID, "scopus", "csv", "scopus.csv", `TITLE-ABS-KEY(("BPMN" OR "scheduling"))`, "", 0, ""); err != nil {
		t.Fatal(err)
	}
	workOne, err := db.Works.CreateByDOI("10.1000/backfill-one")
	if err != nil {
		t.Fatal(err)
	}
	revOne, err := db.WorkRevisions.Create(&database.WorkRevision{
		WorkID: workOne, PipelineRunID: runID, ProducerStage: database.ProducerStageNormalize,
		Title: "BPMN scheduling", Abstract: "genetic algorithm",
		Keywords: "genetic algorithm; BPMN", KeywordsPlus: "null",
	})
	if err != nil {
		t.Fatal(err)
	}
	workTwo, err := db.Works.CreateByDOI("10.1000/backfill-two")
	if err != nil {
		t.Fatal(err)
	}
	revTwo, err := db.WorkRevisions.Create(&database.WorkRevision{
		WorkID: workTwo, PipelineRunID: runID, ProducerStage: database.ProducerStageNormalize, Title: "Unrelated",
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := reconcileStoredTermMatches(db); err != nil {
		t.Fatal(err)
	}
	terms, err := db.TermMatches.GetRunTerms(runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(terms) != 2 {
		t.Fatalf("run_search_terms = %d, want 2: %+v", len(terms), terms)
	}
	got, err := db.TermMatches.GetRevisionMatches(runID, revOne)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"title":    {"BPMN", "scheduling"},
		"keywords": {"BPMN"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("revision one matches = %v, want %v", got, want)
	}
	gotTwo, err := db.TermMatches.GetRevisionMatches(runID, revTwo)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotTwo) != 0 {
		t.Fatalf("revision two matches = %v, want none", gotTwo)
	}

	// A second pass must not duplicate rows.
	if err := reconcileStoredTermMatches(db); err != nil {
		t.Fatal(err)
	}
	var termRows, matchRows int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM run_search_terms WHERE pipeline_run_id=?", runID).Scan(&termRows); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM work_revision_term_matches WHERE pipeline_run_id=?", runID).Scan(&matchRows); err != nil {
		t.Fatal(err)
	}
	if termRows != 2 || matchRows != 3 {
		t.Fatalf("after second pass term_rows=%d match_rows=%d, want 2 and 3", termRows, matchRows)
	}
}

// TestReconcileStoredTermMatchesNullQueries verifies runs with NULL queries produce no rows and no error.
func TestReconcileStoredTermMatchesNullQueries(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "workspace.db"), filepath.Join("..", "..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	runID, err := db.PipelineRuns.StartRun("reconcile null", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.PipelineRuns.FinishRun(runID, "completed", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := db.RunSources.Create(runID, "scopus", "csv", "scopus.csv", "", "", 0, ""); err != nil {
		t.Fatal(err)
	}
	if err := reconcileStoredTermMatches(db); err != nil {
		t.Fatal(err)
	}
	terms, err := db.TermMatches.GetRunTerms(runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(terms) != 0 {
		t.Fatalf("run_search_terms = %+v, want none", terms)
	}
}

// TestReconcileStoredTermMatchesNoRevisionsStillStoresTerms verifies a run with
// queries but no normalize revisions still receives its term inventory.
func TestReconcileStoredTermMatchesNoRevisionsStillStoresTerms(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "workspace.db"), filepath.Join("..", "..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	runID, err := db.PipelineRuns.StartRun("reconcile no revisions", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.PipelineRuns.FinishRun(runID, "completed", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := db.RunSources.Create(runID, "scopus", "csv", "scopus.csv", `TITLE-ABS-KEY(("BPMN" OR "scheduling"))`, "", 0, ""); err != nil {
		t.Fatal(err)
	}
	if err := reconcileStoredTermMatches(db); err != nil {
		t.Fatal(err)
	}
	terms, err := db.TermMatches.GetRunTerms(runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(terms) != 2 {
		t.Fatalf("run_search_terms = %d, want 2: %+v", len(terms), terms)
	}
	count, err := db.TermMatches.CountRunTermData(runID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("match rows = %d, want 0", count)
	}
}

// TestReconcileStoredTermMatchesBestEffortLogsFailure verifies a reconciliation
// failure is contained by the best-effort wrapper and does not panic.
func TestReconcileStoredTermMatchesBestEffortLogsFailure(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "workspace.db"), filepath.Join("..", "..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	runID, err := db.PipelineRuns.StartRun("reconcile failure", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.PipelineRuns.FinishRun(runID, "completed", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec("DROP TABLE work_revision_term_matches"); err != nil {
		t.Fatal(err)
	}
	if err := reconcileStoredTermMatches(db); err == nil {
		t.Fatal("reconcileStoredTermMatches should fail when the match table is missing")
	}
	reconcileStoredTermMatchesBestEffort(db)
}
