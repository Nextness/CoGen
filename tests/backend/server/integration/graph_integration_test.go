// graph_integration_test.go tests the graph endpoint with filters,
// truncation, and research-network mode.
//

package server_test

import "analysis/tests/support/go/testfixture"

import "analysis/tests/support/go/testenv"

import subject "analysis/backend/server"
import testhelpers "analysis/tests/support/go/backend/server"

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"strings"
	"testing"
)

// TestAPIDetailsGraphModes verifies api details graph modes.
func TestAPIDetailsGraphModes(t *testing.T) {
	path, runID, _, _ := testhelpers.ViewerFixture(t)
	viewer, err := subject.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	handler := viewer.Handler()
	for _, path := range []string{
		"/api/graph?run_id=" + subject.stringID(runID) + "&mode=article_author",
		"/api/graph?run_id=" + subject.stringID(runID) + "&mode=citation",
		"/api/graph?run_id=" + subject.stringID(runID) + "&mode=article_reference",
		"/api/graph?run_id=" + subject.stringID(runID) + "&mode=research_network",
		"/api/graph?run_id=" + subject.stringID(runID) + "&q=One",
	} {
		response := testhelpers.ViewerRequest(t, handler, path)
		if response.Code != http.StatusOK {
			t.Errorf("GET %s: status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
	researchNetwork := testhelpers.ViewerRequest(t, handler, "/api/graph?run_id="+subject.stringID(runID)+"&mode=research_network")
	for _, expected := range []string{`"type":"article"`, `"type":"author"`, `"type":"reference"`, `"type":"referenced_author"`, `"type":"coauthor"`, `"type":"reference_author"`, `"node_types"`, `"edge_types"`} {
		if !strings.Contains(researchNetwork.Body.String(), expected) {
			t.Errorf("research network response is missing %s: %s", expected, researchNetwork.Body.String())
		}
	}
	for _, path := range []string{
		"/api/graph?mode=article_author",
		"/api/graph?run_id=" + subject.stringID(runID) + "&mode=unsupported",
		"/api/graph?run_id=" + subject.stringID(runID) + "&article_limit=2001",
		"/api/graph?run_id=" + subject.stringID(runID) + "&status=valid",
	} {
		response := testhelpers.ViewerRequest(t, handler, path)
		if response.Code != http.StatusBadRequest {
			t.Errorf("GET %s: status=%d", path, response.Code)
		}
	}
}

// TestAPIGraphFiltersAndTruncation verifies api graph filters and truncation.
func TestAPIGraphFiltersAndTruncation(t *testing.T) {
	path, runID, _, _ := testhelpers.ViewerFixture(t)
	db, err := testfixture.Open(path, testenv.Path("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := db.Conn.Exec("INSERT INTO works (doi) VALUES ('10.1/graph-truncation')")
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	workID, _ := result.LastInsertId()
	if _, err := db.Conn.Exec("INSERT INTO work_revisions (work_id, pipeline_run_id, payload_hash, title, year, source, producer_stage) VALUES (?, ?, 'graph-truncation', 'Article Three', 2022, 'scopus', 'normalize')", workID, runID); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if _, err := db.Conn.Exec("INSERT INTO run_work_stages (pipeline_run_id, work_id, stage_name, outcome) VALUES (?, ?, 'validate', 'valid')", runID, workID); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	viewer, err := subject.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	handler := viewer.Handler()
	base := "/api/graph?run_id=" + subject.stringID(runID)
	for _, test := range []struct {
		query string
		nodes int
	}{
		{"&q=Article+One", 1},
		{"&author=Ada", 1},
		{"&orcid=0000-0002-1825-0097", 1},
		{"&reference=External", 1},
		{"&source=scopus", 2},
		{"&year_min=2024&year_max=2024", 1},
		{"&citation_min=5&citation_max=5", 1},
		{"&reference_min=1&reference_max=1", 1},
	} {
		response := testhelpers.ViewerRequest(t, handler, base+test.query)
		if response.Code != http.StatusOK {
			t.Errorf("GET %s: status=%d body=%s", base+test.query, response.Code, response.Body.String())
			continue
		}
		var payload struct {
			Nodes []map[string]any `json:"nodes"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		articles := 0
		for _, node := range payload.Nodes {
			if node["type"] == "article" {
				articles++
			}
		}
		if articles != test.nodes {
			t.Errorf("GET %s returned %d article nodes, want %d", base+test.query, articles, test.nodes)
		}
	}
	response := testhelpers.ViewerRequest(t, handler, base+"&article_limit=1")
	if response.Code != http.StatusOK {
		t.Fatalf("truncation request: status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Truncated        bool           `json:"truncated"`
		TruncationReason string         `json:"truncation_reason"`
		Limits           map[string]int `json:"limits"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Truncated || payload.TruncationReason != "article_limit" || payload.Limits["article_nodes"] != subject.maxArticleLimit {
		t.Fatalf("graph did not report truncation metadata: %#v", payload)
	}
	full := testhelpers.ViewerRequest(t, handler, base+"&mode=article_author")
	if full.Code != http.StatusOK || strings.Contains(full.Body.String(), "Article One raw import") {
		t.Fatalf("graph must expose only final normalized valid revisions: status=%d body=%s", full.Code, full.Body.String())
	}
	invalid := testhelpers.ViewerRequest(t, handler, base+"&year_min=not-a-year")
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid numeric graph filter status=%d body=%s", invalid.Code, invalid.Body.String())
	}
}

// TestGraphEdgeBudgets verifies empty and exhausted relationship budgets without large fixtures.
func TestGraphEdgeBudgets(t *testing.T) {
	path, runID, _, _ := testhelpers.ViewerFixture(t)
	viewer, err := subject.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	request := httptest.NewRequest(http.MethodGet, "/api/graph?run_id="+subject.stringID(runID), nil)
	articles, _, err := viewer.graphArticles(request.Context(), request, runID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) == 0 {
		t.Fatal("fixture has no graph articles")
	}
	nodes, edges, truncated, err := viewer.graphEdgesWithinBudget(request.Context(), "article_author", nil, -1, -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 0 || len(edges) != 0 || truncated {
		t.Fatalf("empty graph nodes=%d edges=%d truncated=%v", len(nodes), len(edges), truncated)
	}
	nodes, edges, truncated, err = viewer.graphEdgesWithinBudget(request.Context(), "article_author", articles, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != len(articles) || len(edges) != 0 || !truncated {
		t.Fatalf("related budget graph nodes=%d articles=%d edges=%d truncated=%v", len(nodes), len(articles), len(edges), truncated)
	}
	_, edges, truncated, err = viewer.graphEdgesWithinBudget(request.Context(), "article_author", articles, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 0 || !truncated {
		t.Fatalf("edge budget graph edges=%d truncated=%v", len(edges), truncated)
	}
}

// TestCitationGraphFiltersExternalTargetsBeforeApplyingEdgeBudget verifies irrelevant citations cannot consume the renderable edge limit.
func TestCitationGraphFiltersExternalTargetsBeforeApplyingEdgeBudget(t *testing.T) {
	path, runID, _, _ := testhelpers.ViewerFixture(t)
	viewer, err := subject.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()

	createArticle := func(doi, hash, title string) (int64, int64) {
		t.Helper()
		work, err := viewer.writeDB.Conn.Exec("INSERT INTO works (doi) VALUES (?)", doi)
		if err != nil {
			t.Fatal(err)
		}
		workID, _ := work.LastInsertId()
		revision, err := viewer.writeDB.Conn.Exec("INSERT INTO work_revisions (work_id, pipeline_run_id, payload_hash, title, producer_stage) VALUES (?, ?, ?, ?, 'normalize')", workID, runID, hash, title)
		if err != nil {
			t.Fatal(err)
		}
		revisionID, _ := revision.LastInsertId()
		return workID, revisionID
	}
	sourceWorkID, sourceRevisionID := createArticle("10.1/graph-source", "graph-source", "Graph source")
	targetWorkID, targetRevisionID := createArticle("10.1/graph-target", "graph-target", "Graph target")
	for index := 0; index < 3; index++ {
		work, err := viewer.writeDB.Conn.Exec("INSERT INTO works (doi) VALUES (?)", "10.1/graph-external-"+subject.stringID(int64(index)))
		if err != nil {
			t.Fatal(err)
		}
		externalWorkID, _ := work.LastInsertId()
		if _, err := viewer.writeDB.Conn.Exec("INSERT INTO reference_mentions (work_revision_id, resolved_work_id, mention_order) VALUES (?, ?, ?)", sourceRevisionID, externalWorkID, index+1); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := viewer.writeDB.Conn.Exec("INSERT INTO reference_mentions (work_revision_id, resolved_work_id, mention_order) VALUES (?, ?, 4)", sourceRevisionID, targetWorkID); err != nil {
		t.Fatal(err)
	}
	articles := []map[string]any{
		{"id": sourceRevisionID, "work_id": sourceWorkID, "title": "Graph source", "doi": "10.1/graph-source"},
		{"id": targetRevisionID, "work_id": targetWorkID, "title": "Graph target", "doi": "10.1/graph-target"},
	}
	_, edges, truncated, err := viewer.graphEdgesWithinBudget(t.Context(), "citation", articles, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if truncated || len(edges) != 1 || edges[0]["source"] != "article:"+subject.stringID(sourceRevisionID) || edges[0]["target"] != "article:"+subject.stringID(targetRevisionID) {
		t.Fatalf("citation edge selection = edges=%v truncated=%v", edges, truncated)
	}
}
