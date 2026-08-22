// graph_integration_test.go tests the graph endpoint with filters,
// truncation, and research-network mode.
//
//go:build integration

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"analysis/database"
)

// TestAPIDetailsGraphModes verifies api details graph modes.
func TestAPIDetailsGraphModes(t *testing.T) {
	path, runID, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	handler := viewer.Handler()
	for _, path := range []string{
		"/api/graph?run_id=" + stringID(runID) + "&mode=article_author",
		"/api/graph?run_id=" + stringID(runID) + "&mode=citation",
		"/api/graph?run_id=" + stringID(runID) + "&mode=article_reference",
		"/api/graph?run_id=" + stringID(runID) + "&mode=research_network",
		"/api/graph?run_id=" + stringID(runID) + "&q=One",
	} {
		response := viewerRequest(t, handler, path)
		if response.Code != http.StatusOK {
			t.Errorf("GET %s: status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
	researchNetwork := viewerRequest(t, handler, "/api/graph?run_id="+stringID(runID)+"&mode=research_network")
	for _, expected := range []string{`"type":"article"`, `"type":"author"`, `"type":"reference"`, `"type":"referenced_author"`, `"type":"coauthor"`, `"type":"reference_author"`, `"node_types"`, `"edge_types"`} {
		if !strings.Contains(researchNetwork.Body.String(), expected) {
			t.Errorf("research network response is missing %s: %s", expected, researchNetwork.Body.String())
		}
	}
	for _, path := range []string{
		"/api/graph?mode=article_author",
		"/api/graph?run_id=" + stringID(runID) + "&mode=unsupported",
		"/api/graph?run_id=" + stringID(runID) + "&article_limit=2001",
		"/api/graph?run_id=" + stringID(runID) + "&status=valid",
	} {
		response := viewerRequest(t, handler, path)
		if response.Code != http.StatusBadRequest {
			t.Errorf("GET %s: status=%d", path, response.Code)
		}
	}
}

// TestAPIGraphFiltersAndTruncation verifies api graph filters and truncation.
func TestAPIGraphFiltersAndTruncation(t *testing.T) {
	path, runID, _, _ := viewerFixture(t)
	db, err := database.Open(path, filepath.Join("..", "..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := db.DB.Exec("INSERT INTO works (doi) VALUES ('10.1/graph-truncation')")
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	workID, _ := result.LastInsertId()
	if _, err := db.DB.Exec("INSERT INTO work_revisions (work_id, pipeline_run_id, payload_hash, title, year, source, producer_stage) VALUES (?, ?, 'graph-truncation', 'Article Three', 2022, 'scopus', 'normalize')", workID, runID); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if _, err := db.DB.Exec("INSERT INTO run_work_stages (pipeline_run_id, work_id, stage_name, outcome) VALUES (?, ?, 'validate', 'valid')", runID, workID); err != nil {
		db.Close()
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
	base := "/api/graph?run_id=" + stringID(runID)
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
		response := viewerRequest(t, handler, base+test.query)
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
	response := viewerRequest(t, handler, base+"&article_limit=1")
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
	if !payload.Truncated || payload.TruncationReason != "article_limit" || payload.Limits["article_nodes"] != maxArticleLimit {
		t.Fatalf("graph did not report truncation metadata: %#v", payload)
	}
	full := viewerRequest(t, handler, base+"&mode=article_author")
	if full.Code != http.StatusOK || strings.Contains(full.Body.String(), "Article One raw import") {
		t.Fatalf("graph must expose only final normalized valid revisions: status=%d body=%s", full.Code, full.Body.String())
	}
	invalid := viewerRequest(t, handler, base+"&year_min=not-a-year")
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid numeric graph filter status=%d body=%s", invalid.Code, invalid.Body.String())
	}
}

// TestGraphEdgeBudgets verifies empty and exhausted relationship budgets without large fixtures.
func TestGraphEdgeBudgets(t *testing.T) {
	path, runID, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	request := httptest.NewRequest(http.MethodGet, "/api/graph?run_id="+stringID(runID), nil)
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
