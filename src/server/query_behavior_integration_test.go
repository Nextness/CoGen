// query_behavior_integration_test.go pins the observable query behavior of the
// viewer API after the table-family cutover: per-route statement counts,
// streamed trailer counts, canceled reads, selected-context consistency, and
// the top-level JSON shapes the frontend consumes. The counts and shapes are
// pinned to the shared viewer fixtures, so a change means a handler started or
// stopped issuing SQL statements or changed its response contract.
//
//go:build integration

package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"sort"
	"strings"
	"testing"
)

// apiQueryCountCase pins one viewer route to its exact statement count.
type apiQueryCountCase struct {
	name string
	path string
	used int64
}

// TestAPIQueryCountsMatchRouteBudgets verifies every viewer route keeps its
// deterministic statement count and hard route ceiling after the handler
// cutover. The counts are pinned to the shared viewer fixtures; a changed count
// means a handler started or stopped issuing SQL statements.
func TestAPIQueryCountsMatchRouteBudgets(t *testing.T) {
	t.Run("metadata fixture", func(t *testing.T) {
		path, runID, revisionID, mentionID := viewerFixture(t)
		viewer, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer viewer.Close()
		run := stringID(runID)
		revision := stringID(revisionID)
		mention := stringID(mentionID)
		assertAPIQueryCounts(t, viewer.Handler(), []apiQueryCountCase{
			{"health", "/api/health", 2},
			{"hierarchy summary", "/api/hierarchy", 2},
			{"hierarchy runs", "/api/hierarchy?section=runs", 1},
			{"hierarchy searches", "/api/hierarchy?section=searches", 1},
			{"hierarchy revisions", "/api/hierarchy?section=revisions&search_id=1", 1},
			{"hierarchy plans", "/api/hierarchy?section=plans&search_revision_id=1", 1},
			{"hierarchy attempts", "/api/hierarchy?section=attempts&plan_id=1", 1},
			{"legacy searches", "/api/searches", 1},
			{"legacy plans", "/api/plans?search_revision_id=1", 1},
			{"legacy runs", "/api/runs?search_revision_id=1&plan_id=1", 1},
			{"run context", "/api/runs/" + run + "/context", 1},
			{"overview", "/api/overview?run_id=" + run, 10},
			{"run artifacts", "/api/runs/" + run + "/artifacts", 2},
			{"artifact inspection", "/api/artifacts/1/inspect", 1},
			{"run cache uses", "/api/runs/" + run + "/cache-uses", 3},
			{"corpus articles", "/api/runs/" + run + "/corpus/articles", 18},
			{"corpus authors", "/api/runs/" + run + "/corpus/authors", 3},
			{"corpus references", "/api/runs/" + run + "/corpus/references", 3},
			{"corpus sources", "/api/runs/" + run + "/corpus/sources", 4},
			{"evaluation", "/api/runs/" + run + "/evaluation", 18},
			{"identity evidence", "/api/runs/" + run + "/identity-evidence", 6},
			{"identity candidates", "/api/identity-resolutions/1/candidates?run_id=" + run, 2},
			{"run stages", "/api/runs/" + run + "/stages", 5},
			{"audit", "/api/audit?run_id=" + run, 7},
			{"audit recorded data", "/api/audit/1/recorded-data?run_id=" + run, 2},
			{"table discovery", "/api/tables", 0},
			{"table rows", "/api/tables/work_revisions", 2},
			{"article detail", "/api/articles/" + revision + "?run_id=" + run, 14},
			{"article authors", "/api/articles/" + revision + "/collections/authors?run_id=" + run, 3},
			{"author detail", "/api/authors/1?run_id=" + run, 7},
			{"author articles", "/api/authors/1/collections/articles?run_id=" + run, 3},
			{"reference detail", "/api/references/" + mention + "?run_id=" + run, 1},
			{"graph", "/api/graph?run_id=" + run + "&mode=article_author", 3},
			{"work PDF status", "/api/works/1/pdf-status", 1},
		})
	})

	t.Run("pdf fixture", func(t *testing.T) {
		fixture := newPDFViewerFixture(t)
		if _, _, err := fixture.server.writeDB.Reviews.CreateContext(t.Context(), fixture.runID, nil); err != nil {
			t.Fatal(err)
		}
		run := stringID(fixture.runID)
		revision := stringID(fixture.revisionID)
		assertAPIQueryCounts(t, fixture.server.Handler(), []apiQueryCountCase{
			{"review context", "/api/runs/" + run + "/review-context", 2},
			{"review context candidates", "/api/runs/" + run + "/review-context-candidates?scope=same_search", 2},
			{"article review", "/api/runs/" + run + "/articles/" + revision + "/review", 9},
			{"review versions", "/api/runs/" + run + "/articles/" + revision + "/review/versions", 3},
			{"article notes", "/api/runs/" + run + "/articles/" + revision + "/notes", 3},
			{"run notes", "/api/runs/" + run + "/notes", 3},
			{"article anchors", "/api/runs/" + run + "/articles/" + revision + "/anchors", 3},
			{"backlinks", "/api/runs/" + run + "/links/backlinks?target_type=article&target_id=10.1000/viewer-available", 3},
			{"work PDF status", "/api/works/" + stringID(fixture.availableID) + "/pdf-status", 2},
		})
	})
}

// assertAPIQueryCounts verifies each route's limit header, exact used count, and repeat determinism.
func assertAPIQueryCounts(t *testing.T, handler http.Handler, cases []apiQueryCountCase) {
	t.Helper()
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			response := viewerRequest(t, handler, test.path)
			if response.Code != http.StatusOK {
				t.Fatalf("GET %s status=%d body=%s", test.path, response.Code, response.Body.String())
			}
			if got := response.Header().Get("X-Query-Count-Limit"); got != fmt.Sprint(apiQueryBudget(routePath(test.path))) {
				t.Fatalf("GET %s X-Query-Count-Limit=%q, want %d", test.path, got, apiQueryBudget(routePath(test.path)))
			}
			if got := response.Header().Get("X-Query-Count-Used"); got != fmt.Sprint(test.used) {
				t.Fatalf("GET %s X-Query-Count-Used=%q, want %d", test.path, got, test.used)
			}
			repeated := viewerRequest(t, handler, test.path)
			if got := repeated.Header().Get("X-Query-Count-Used"); got != fmt.Sprint(test.used) {
				t.Fatalf("GET %s repeated X-Query-Count-Used=%q, want %d", test.path, got, test.used)
			}
		})
	}
}

// routePath returns the path portion of one test route without its query string.
func routePath(route string) string {
	path, _, _ := strings.Cut(route, "?")
	return path
}

// TestAPIStreamedQueryCountsUseTrailers verifies streamed PDF and artifact
// content deliver their statement count as a trailer instead of a header.
func TestAPIStreamedQueryCountsUseTrailers(t *testing.T) {
	path, _, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	artifact := viewerRequest(t, viewer.Handler(), "/api/artifacts/1/content")
	if artifact.Code != http.StatusOK {
		t.Fatalf("artifact content status=%d body=%s", artifact.Code, artifact.Body.String())
	}
	if got := artifact.Header().Get("X-Query-Count-Limit"); got != fmt.Sprint(apiQueryBudget("/api/artifacts/1/content")) {
		t.Fatalf("artifact content X-Query-Count-Limit=%q", got)
	}
	if got := artifact.Result().Trailer.Get("X-Query-Count-Used"); got != "1" {
		t.Fatalf("artifact content trailer X-Query-Count-Used=%q, want 1", got)
	}

	fixture := newPDFViewerFixture(t)
	pdf := viewerRequest(t, fixture.server.Handler(), "/api/pdf/"+stringID(fixture.availableID))
	if pdf.Code != http.StatusOK {
		t.Fatalf("PDF content status=%d body=%s", pdf.Code, pdf.Body.String())
	}
	if got := pdf.Header().Get("X-Query-Count-Limit"); got != fmt.Sprint(apiQueryBudget("/api/pdf/"+stringID(fixture.availableID))) {
		t.Fatalf("PDF content X-Query-Count-Limit=%q", got)
	}
	if got := pdf.Result().Trailer.Get("X-Query-Count-Used"); got != "3" {
		t.Fatalf("PDF content trailer X-Query-Count-Used=%q, want 3", got)
	}
}

// TestCanceledReadRequestFailsWithoutPartialJSON verifies a canceled read
// request stops before touching the database and returns the stable error
// envelope instead of a partial document.
func TestCanceledReadRequestFailsWithoutPartialJSON(t *testing.T) {
	path, runID, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest(http.MethodGet, "/api/runs/"+stringID(runID)+"/context", nil).WithContext(canceled)
	recorder := httptest.NewRecorder()
	viewer.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("canceled read status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"code":"internal_error"`) {
		t.Fatalf("canceled read body=%s", recorder.Body.String())
	}
}

// TestSelectedRunContextIsConsistentAcrossEndpoints verifies the run context
// selected by the viewer is the same run, plan, and revision reported by the
// run-scoped reads and the hierarchy.
func TestSelectedRunContextIsConsistentAcrossEndpoints(t *testing.T) {
	path, runID, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	handler := viewer.Handler()
	run := stringID(runID)

	status, context := requestJSON(t, handler, "/api/runs/"+run+"/context")
	if status != http.StatusOK {
		t.Fatalf("run context status=%d body=%v", status, context)
	}
	revision := context["revision"].(map[string]any)
	plan := context["plan"].(map[string]any)
	selectedRun := context["run"].(map[string]any)
	revisionID := int64(revision["id"].(float64))
	planID := int64(plan["id"].(float64))
	if selectedRun["id"] != float64(runID) || selectedRun["execution_plan_id"] != float64(planID) {
		t.Fatalf("run context run=%v plan=%v", selectedRun, plan)
	}

	for _, route := range []string{
		"/api/overview?run_id=" + run,
		"/api/runs/" + run + "/corpus/articles",
		"/api/runs/" + run + "/evaluation",
		"/api/runs/" + run + "/stages",
	} {
		status, body := requestJSON(t, handler, route)
		if status != http.StatusOK {
			t.Fatalf("GET %s status=%d body=%v", route, status, body)
		}
		if body["run_id"] != float64(runID) {
			t.Errorf("GET %s run_id=%v, want %d", route, body["run_id"], runID)
		}
	}

	status, attempts := requestJSON(t, handler, "/api/hierarchy?section=attempts&plan_id="+stringID(planID))
	if status != http.StatusOK {
		t.Fatalf("hierarchy attempts status=%d body=%v", status, attempts)
	}
	found := false
	for _, raw := range attempts["items"].([]any) {
		item := raw.(map[string]any)
		if item["id"] == float64(runID) {
			found = true
			if item["execution_plan_id"] != float64(planID) {
				t.Errorf("hierarchy attempt execution_plan_id=%v, want %d", item["execution_plan_id"], planID)
			}
		}
	}
	if !found {
		t.Errorf("hierarchy attempts did not include run %d", runID)
	}

	status, runs := requestJSON(t, handler, "/api/runs?search_revision_id="+stringID(revisionID)+"&plan_id="+stringID(planID))
	if status != http.StatusOK {
		t.Fatalf("legacy runs status=%d body=%v", status, runs)
	}
	found = false
	for _, raw := range runs["runs"].([]any) {
		item := raw.(map[string]any)
		if item["id"] == float64(runID) {
			found = true
			if item["search_revision_id"] != float64(revisionID) {
				t.Errorf("legacy run search_revision_id=%v, want %d", item["search_revision_id"], revisionID)
			}
		}
	}
	if !found {
		t.Errorf("legacy runs did not include run %d", runID)
	}
}

// TestAPIResponseShapesMatchFrontendFixtures verifies every viewer route keeps
// the exact top-level JSON keys the frontend contract consumes.
func TestAPIResponseShapesMatchFrontendFixtures(t *testing.T) {
	path, runID, revisionID, mentionID := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	handler := viewer.Handler()
	run := stringID(runID)
	revision := stringID(revisionID)
	mention := stringID(mentionID)

	cases := []struct {
		name string
		path string
		keys []string
	}{
		{"health", "/api/health", []string{"corpus_id", "metadata_readable", "pdf_store_bound", "pdf_store_readable", "readable", "review", "review_writable", "table_count", "tables"}},
		{"hierarchy summary", "/api/hierarchy", []string{"latest_run", "totals", "version"}},
		{"hierarchy runs", "/api/hierarchy?section=runs", []string{"has_more", "items", "limit", "next_cursor", "version"}},
		{"hierarchy searches", "/api/hierarchy?section=searches", []string{"has_more", "items", "limit", "next_cursor", "version"}},
		{"hierarchy revisions", "/api/hierarchy?section=revisions&search_id=1", []string{"has_more", "items", "limit", "next_cursor", "version"}},
		{"hierarchy plans", "/api/hierarchy?section=plans&search_revision_id=1", []string{"has_more", "items", "limit", "next_cursor", "version"}},
		{"hierarchy attempts", "/api/hierarchy?section=attempts&plan_id=1", []string{"has_more", "items", "limit", "next_cursor", "version"}},
		{"legacy searches", "/api/searches", []string{"deprecated", "has_more", "limit", "replacement", "searches"}},
		{"legacy plans", "/api/plans?search_revision_id=1", []string{"deprecated", "has_more", "limit", "plans", "replacement"}},
		{"legacy runs", "/api/runs?search_revision_id=1&plan_id=1", []string{"deprecated", "has_more", "limit", "replacement", "runs"}},
		{"run context", "/api/runs/" + run + "/context", []string{"lifecycle", "plan", "review", "revision", "run", "search"}},
		{"overview", "/api/overview?run_id=" + run, []string{"cache_breakdown", "captured_metrics", "current_coverage", "enrichment_breakdown", "enrichment_field_breakdown", "enrichment_provider_breakdown", "normalization_breakdown", "normalization_field_breakdown", "relationship_totals", "retention_funnel", "run_id", "source_breakdown", "source_filter_counts", "source_filter_diagnostics", "source_result_counts", "validation_breakdown"}},
		{"run artifacts", "/api/runs/" + run + "/artifacts", []string{"artifacts", "context", "filters", "has_more", "limit", "next_cursor", "run_id"}},
		{"artifact inspection", "/api/artifacts/1/inspect", []string{"artifact_id", "byte_size", "content", "content_type", "format", "preview_byte_size", "preview_limit_bytes", "stored_byte_size", "truncated"}},
		{"run cache uses", "/api/runs/" + run + "/cache-uses", []string{"cache_uses", "columns", "pagination", "rows", "run_id"}},
		{"corpus articles", "/api/runs/" + run + "/corpus/articles", []string{"collection", "columns", "pagination", "proposed_parent", "queue_navigation", "review_context", "review_context_initialized", "review_summary", "rows", "run_id", "run_writable"}},
		{"corpus authors", "/api/runs/" + run + "/corpus/authors", []string{"collection", "columns", "pagination", "rows", "run_id"}},
		{"corpus references", "/api/runs/" + run + "/corpus/references", []string{"collection", "columns", "pagination", "rows", "run_id"}},
		{"corpus sources", "/api/runs/" + run + "/corpus/sources", []string{"collection", "columns", "pagination", "rows", "run_id", "source_result_counts"}},
		{"evaluation", "/api/runs/" + run + "/evaluation", []string{"collection", "columns", "pagination", "proposed_parent", "queue_navigation", "review_context", "review_context_initialized", "review_summary", "rows", "run_id", "run_writable"}},
		{"identity evidence", "/api/runs/" + run + "/identity-evidence", []string{"columns", "pagination", "rows", "run_id", "stats"}},
		{"identity candidates", "/api/identity-resolutions/1/candidates?run_id=" + run, []string{"has_more", "items", "limit", "next_cursor", "resolution_id"}},
		{"run stages", "/api/runs/" + run + "/stages", []string{"columns", "pagination", "rows", "run_id", "run_steps", "stage_summaries"}},
		{"audit", "/api/audit?run_id=" + run, []string{"events", "facets", "has_more", "next_cursor", "scope", "summary"}},
		{"audit recorded data", "/api/audit/1/recorded-data?run_id=" + run, []string{"after", "before", "byte_limit", "event_id", "metadata", "truncated_fields"}},
		{"table discovery", "/api/tables", []string{"tables"}},
		{"table rows", "/api/tables/work_revisions", []string{"limits", "pagination", "rows", "table", "truncated_fields"}},
		{"article detail", "/api/articles/" + revision + "?run_id=" + run, []string{"article", "audit_events", "authors", "enrichment_summary", "pdf_status", "references", "review_context", "review_context_initialized", "stage_outcomes", "term_matches"}},
		{"article authors", "/api/articles/" + revision + "/collections/authors?run_id=" + run, []string{"has_more", "items", "limit", "next_cursor", "total"}},
		{"author detail", "/api/authors/1?run_id=" + run, []string{"articles", "audit_events", "author", "identity_evidence"}},
		{"author articles", "/api/authors/1/collections/articles?run_id=" + run, []string{"has_more", "items", "limit", "next_cursor", "total"}},
		{"reference detail", "/api/references/" + mention + "?run_id=" + run, []string{"reference"}},
		{"graph", "/api/graph?run_id=" + run + "&mode=article_author", []string{"counts", "edges", "filters", "limits", "nodes", "truncated", "truncation_reason"}},
		{"work PDF status", "/api/works/1/pdf-status", []string{"doi", "eligible", "status", "store_bound", "work_id"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			status, body := requestJSON(t, handler, test.path)
			if status != http.StatusOK {
				t.Fatalf("GET %s status=%d body=%v", test.path, status, body)
			}
			assertTopLevelKeys(t, test.path, body, test.keys)
		})
	}

	t.Run("pdf fixture", func(t *testing.T) {
		fixture := newPDFViewerFixture(t)
		if _, _, err := fixture.server.writeDB.Reviews.CreateContext(t.Context(), fixture.runID, nil); err != nil {
			t.Fatal(err)
		}
		pdfHandler := fixture.server.Handler()
		pdfRun := stringID(fixture.runID)
		pdfRevision := stringID(fixture.revisionID)
		pdfCases := []struct {
			name string
			path string
			keys []string
		}{
			{"review context", "/api/runs/" + pdfRun + "/review-context", []string{"context", "context_initialized", "run_id", "run_writable"}},
			{"review context candidates", "/api/runs/" + pdfRun + "/review-context-candidates?scope=same_search", []string{"has_more", "items", "limit", "next_cursor", "rows"}},
			{"article review", "/api/runs/" + pdfRun + "/articles/" + pdfRevision + "/review", []string{"context", "context_initialized", "editability", "editable", "pdf_status", "review", "run_id", "state", "summary_counts", "work_id", "work_revision_id"}},
			{"review versions", "/api/runs/" + pdfRun + "/articles/" + pdfRevision + "/review/versions", []string{"has_more", "items", "limit", "next_cursor", "versions"}},
			{"article notes", "/api/runs/" + pdfRun + "/articles/" + pdfRevision + "/notes", []string{"has_more", "items", "limit", "next_cursor", "notes"}},
			{"run notes", "/api/runs/" + pdfRun + "/notes", []string{"has_more", "items", "limit", "next_cursor", "notes"}},
			{"article anchors", "/api/runs/" + pdfRun + "/articles/" + pdfRevision + "/anchors", []string{"anchors", "has_more", "items", "limit", "next_cursor"}},
			{"backlinks", "/api/runs/" + pdfRun + "/links/backlinks?target_type=article&target_id=10.1000/viewer-available", []string{"backlinks", "has_more", "items", "limit", "next_cursor"}},
			{"work PDF status", "/api/works/" + stringID(fixture.availableID) + "/pdf-status", []string{"byte_size", "content_hash", "doi", "eligible", "inventoried_at", "status", "store_bound", "work_id"}},
		}
		for _, test := range pdfCases {
			t.Run(test.name, func(t *testing.T) {
				status, body := requestJSON(t, pdfHandler, test.path)
				if status != http.StatusOK {
					t.Fatalf("GET %s status=%d body=%v", test.path, status, body)
				}
				assertTopLevelKeys(t, test.path, body, test.keys)
			})
		}
	})
}

// assertTopLevelKeys verifies one JSON object exposes exactly the expected top-level keys.
func assertTopLevelKeys(t *testing.T, path string, body map[string]any, expected []string) {
	t.Helper()
	keys := make([]string, 0, len(body))
	for key := range body {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if !slices.Equal(keys, expected) {
		t.Errorf("GET %s top-level keys = %v, want %v", path, keys, expected)
	}
}
