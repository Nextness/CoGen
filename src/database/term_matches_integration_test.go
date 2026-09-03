// term_matches_integration_test.go tests the TermMatches repository against a
// temporary database with production migrations.
//go:build integration

package database

import (
	"reflect"
	"testing"
)

// TestTermMatchesReplaceRunTermDataIdempotent verifies replace semantics leave exactly one copy per row.
func TestTermMatchesReplaceRunTermDataIdempotent(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	runID, err := db.PipelineRuns.StartRun("term matches", "")
	if err != nil {
		t.Fatal(err)
	}
	workID, err := db.Works.CreateByDOI("10.1000/term-matches")
	if err != nil {
		t.Fatal(err)
	}
	revisionID, err := db.WorkRevisions.Create(&WorkRevision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: ProducerStageNormalize, Title: "Term match article",
	})
	if err != nil {
		t.Fatal(err)
	}
	termsBySource := map[string][]string{"scopus": {"BPMN", "scheduling"}, "wos": {"BPMN"}}
	matches := map[int64]map[string][]string{revisionID: {"title": {"BPMN"}, "keywords": {"scheduling"}}}

	for i := 0; i < 2; i++ {
		if err := db.TermMatches.ReplaceRunTermData(runID, termsBySource, matches); err != nil {
			t.Fatalf("ReplaceRunTermData pass %d: %v", i+1, err)
		}
	}

	var termRows int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM run_search_terms WHERE pipeline_run_id=?", runID).Scan(&termRows); err != nil {
		t.Fatal(err)
	}
	if termRows != 3 {
		t.Fatalf("run_search_terms rows = %d, want 3", termRows)
	}
	var matchRows int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM work_revision_term_matches WHERE pipeline_run_id=?", runID).Scan(&matchRows); err != nil {
		t.Fatal(err)
	}
	if matchRows != 2 {
		t.Fatalf("work_revision_term_matches rows = %d, want 2", matchRows)
	}
}

// TestTermMatchesGetRunTermsOrdered verifies ordered reads and empty results.
func TestTermMatchesGetRunTermsOrdered(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	runID, err := db.PipelineRuns.StartRun("term matches read", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.TermMatches.ReplaceRunTermData(runID, map[string][]string{"scopus": {"z", "a", "m"}}, nil); err != nil {
		t.Fatal(err)
	}
	terms, err := db.TermMatches.GetRunTerms(runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(terms) != 3 {
		t.Fatalf("GetRunTerms = %d terms, want 3", len(terms))
	}
	for i, want := range []string{"z", "a", "m"} {
		if terms[i].Term != want || terms[i].SourceName != "scopus" {
			t.Fatalf("GetRunTerms[%d] = %+v, want term %q source scopus", i, terms[i], want)
		}
	}

	missing, err := db.TermMatches.GetRunTerms(99999)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatalf("GetRunTerms(missing) = %d terms, want none", len(missing))
	}
}

// TestTermMatchesGetRevisionMatches verifies per-revision reads.
func TestTermMatchesGetRevisionMatches(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	runID, err := db.PipelineRuns.StartRun("term matches revision", "")
	if err != nil {
		t.Fatal(err)
	}
	workID, err := db.Works.CreateByDOI("10.1000/term-matches-revision")
	if err != nil {
		t.Fatal(err)
	}
	revisionID, err := db.WorkRevisions.Create(&WorkRevision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: ProducerStageNormalize, Title: "Term match revision",
	})
	if err != nil {
		t.Fatal(err)
	}
	matches := map[int64]map[string][]string{revisionID: {"title": {"BPMN"}, "abstract": {"scheduling"}}}
	if err := db.TermMatches.ReplaceRunTermData(runID, nil, matches); err != nil {
		t.Fatal(err)
	}
	got, err := db.TermMatches.GetRevisionMatches(runID, revisionID)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{"title": {"BPMN"}, "abstract": {"scheduling"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetRevisionMatches = %v, want %v", got, want)
	}
	missing, err := db.TermMatches.GetRevisionMatches(runID, 99999)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatalf("GetRevisionMatches(missing) = %v, want empty", missing)
	}
}

// TestTermMatchesGetRevisionMatchesBulk verifies bulk reads and the empty short-circuit.
func TestTermMatchesGetRevisionMatchesBulk(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	runID, err := db.PipelineRuns.StartRun("term matches bulk", "")
	if err != nil {
		t.Fatal(err)
	}
	var revisionIDs []int64
	for _, doi := range []string{"10.1000/term-bulk-one", "10.1000/term-bulk-two"} {
		workID, err := db.Works.CreateByDOI(doi)
		if err != nil {
			t.Fatal(err)
		}
		revisionID, err := db.WorkRevisions.Create(&WorkRevision{
			WorkID: workID, PipelineRunID: runID, ProducerStage: ProducerStageNormalize, Title: "Bulk article",
		})
		if err != nil {
			t.Fatal(err)
		}
		revisionIDs = append(revisionIDs, revisionID)
	}
	matches := map[int64]map[string][]string{
		revisionIDs[0]: {"title": {"BPMN"}},
		revisionIDs[1]: {"keywords": {"scheduling"}},
	}
	if err := db.TermMatches.ReplaceRunTermData(runID, nil, matches); err != nil {
		t.Fatal(err)
	}
	got, err := db.TermMatches.GetRevisionMatchesBulk(runID, revisionIDs)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("GetRevisionMatchesBulk = %d revisions, want 2", len(got))
	}
	if !reflect.DeepEqual(got[revisionIDs[0]], map[string][]string{"title": {"BPMN"}}) {
		t.Fatalf("bulk revision 0 = %v", got[revisionIDs[0]])
	}
	if !reflect.DeepEqual(got[revisionIDs[1]], map[string][]string{"keywords": {"scheduling"}}) {
		t.Fatalf("bulk revision 1 = %v", got[revisionIDs[1]])
	}
	empty, err := db.TermMatches.GetRevisionMatchesBulk(runID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Fatalf("GetRevisionMatchesBulk(empty) = %v, want empty", empty)
	}
}

// TestTermMatchesReplaceRunTermDataOrdersMatchesDeterministically verifies map iteration cannot change stored term order.
func TestTermMatchesReplaceRunTermDataOrdersMatchesDeterministically(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	runID, err := db.PipelineRuns.StartRun("term matches ordering", "")
	if err != nil {
		t.Fatal(err)
	}
	workID, err := db.Works.CreateByDOI("10.1000/term-matches-ordering")
	if err != nil {
		t.Fatal(err)
	}
	revisionID, err := db.WorkRevisions.Create(&WorkRevision{WorkID: workID, PipelineRunID: runID, ProducerStage: ProducerStageNormalize})
	if err != nil {
		t.Fatal(err)
	}
	matches := map[int64]map[string][]string{revisionID: {"keywords": {"z", "a"}, "title": {"b", "a"}}}
	for pass := 0; pass < 2; pass++ {
		if err := db.TermMatches.ReplaceRunTermData(runID, nil, matches); err != nil {
			t.Fatal(err)
		}
		got, err := db.TermMatches.GetRevisionMatches(runID, revisionID)
		if err != nil {
			t.Fatal(err)
		}
		want := map[string][]string{"keywords": {"a", "z"}, "title": {"a", "b"}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("pass %d matches = %v, want %v", pass+1, got, want)
		}
	}
}

// TestTermMatchesHasRunTermDataDistinguishesAnEmptyCompletedResult verifies reconciliation state is independent of match count.
func TestTermMatchesHasRunTermDataDistinguishesAnEmptyCompletedResult(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	runID, err := db.PipelineRuns.StartRun("term matches count", "")
	if err != nil {
		t.Fatal(err)
	}
	reconciled, err := db.TermMatches.HasRunTermData(runID)
	if err != nil {
		t.Fatal(err)
	}
	if reconciled {
		t.Fatal("HasRunTermData before = true, want false")
	}
	if err := db.TermMatches.ReplaceRunTermData(runID, nil, nil); err != nil {
		t.Fatal(err)
	}
	reconciled, err = db.TermMatches.HasRunTermData(runID)
	if err != nil {
		t.Fatal(err)
	}
	if !reconciled {
		t.Fatal("HasRunTermData after = false, want true")
	}
}

// TestTermMatchesSchemaConstraints verifies foreign keys and the field vocabulary check.
func TestTermMatchesSchemaConstraints(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	if _, err := db.DB.Exec("INSERT INTO run_search_terms (pipeline_run_id, source_name, term) VALUES (999, 'scopus', 'x')"); err == nil {
		t.Fatal("expected foreign key failure for missing run")
	}
	runID, err := db.PipelineRuns.StartRun("term matches constraints", "")
	if err != nil {
		t.Fatal(err)
	}
	workID, err := db.Works.CreateByDOI("10.1000/term-matches-constraints")
	if err != nil {
		t.Fatal(err)
	}
	revisionID, err := db.WorkRevisions.Create(&WorkRevision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: ProducerStageNormalize, Title: "Constraint article",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec("INSERT INTO work_revision_term_matches (pipeline_run_id, work_revision_id, field, term) VALUES (?, ?, 'bogus', 'x')", runID, revisionID); err == nil {
		t.Fatal("expected CHECK failure for invalid field")
	}
	if _, err := db.DB.Exec("INSERT INTO work_revision_term_matches (pipeline_run_id, work_revision_id, field, term) VALUES (?, 999, 'title', 'x')", runID); err == nil {
		t.Fatal("expected foreign key failure for missing revision")
	}
}
