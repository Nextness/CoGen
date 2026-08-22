// current_revision_integration_test.go verifies every analysis-ready consumer selects the same normalized revision.

//go:build integration

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	"analysis/database"
)

// TestCurrentNormalizedRevisionIsConsistentAcrossConsumers verifies a later duplicate normalize revision replaces the earlier revision everywhere.
func TestCurrentNormalizedRevisionIsConsistentAcrossConsumers(t *testing.T) {
	fixture := newPDFViewerFixture(t)
	db, err := database.Open(fixture.metadataPath, filepath.Join("..", "..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	latestID, err := db.WorkRevisions.Create(&database.WorkRevision{
		WorkID: fixture.availableID, PipelineRunID: fixture.runID, ProducerStage: database.ProducerStageNormalize,
		Title: "Replacement normalized revision",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec("INSERT INTO run_search_terms (pipeline_run_id, source_name, term) VALUES (?, 'fixture', 'legacy')", fixture.runID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec("INSERT INTO work_revision_term_matches (pipeline_run_id, work_revision_id, field, term) VALUES (?, ?, 'title', 'legacy')", fixture.runID, fixture.revisionID); err != nil {
		t.Fatal(err)
	}

	reviewContext, created, err := db.Reviews.CreateContext(context.Background(), fixture.runID, nil)
	if err != nil || !created {
		t.Fatalf("create review context: context=%+v created=%v err=%v", reviewContext, created, err)
	}
	var headRevisionID int64
	if err := db.DB.QueryRow(`SELECT work_revision_id FROM review_context_work_heads
		WHERE review_context_id=? AND work_id=?`, reviewContext.ID, fixture.availableID).Scan(&headRevisionID); err != nil {
		t.Fatal(err)
	}
	if headRevisionID != latestID {
		t.Fatalf("review head revision=%d, want latest %d", headRevisionID, latestID)
	}

	handler := fixture.server.Handler()
	for name, path := range map[string]string{
		"corpus":     fmt.Sprintf("/api/runs/%d/corpus/articles?per_page=20", fixture.runID),
		"evaluation": fmt.Sprintf("/api/runs/%d/evaluation?per_page=20", fixture.runID),
	} {
		status, body := requestJSON(t, handler, path)
		if status != http.StatusOK {
			t.Fatalf("%s response: status=%d body=%v", name, status, body)
		}
		rows := body["rows"].([]any)
		matching := 0
		for _, raw := range rows {
			row := raw.(map[string]any)
			if row["work_id"] == float64(fixture.availableID) {
				matching++
				if row["work_revision_id"] != nil && row["work_revision_id"] != float64(latestID) {
					t.Errorf("%s work_revision_id=%v, want %d", name, row["work_revision_id"], latestID)
				}
				if row["id"] != nil && row["id"] != float64(latestID) {
					t.Errorf("%s id=%v, want %d", name, row["id"], latestID)
				}
				if matches, ok := row["term_matches"].(map[string]any); ok && matches["matched_total"] != float64(0) {
					t.Errorf("%s reused stale term matches: %v", name, matches)
				}
			}
		}
		if matching != 1 {
			t.Errorf("%s returned %d rows for work %d, want one", name, matching, fixture.availableID)
		}
	}

	graphResponse := viewerRequest(t, handler, fmt.Sprintf("/api/graph?run_id=%d&mode=article_author", fixture.runID))
	if graphResponse.Code != http.StatusOK {
		t.Fatalf("graph response: status=%d body=%s", graphResponse.Code, graphResponse.Body.String())
	}
	var graph struct {
		Nodes []map[string]any `json:"nodes"`
	}
	if err := json.Unmarshal(graphResponse.Body.Bytes(), &graph); err != nil {
		t.Fatal(err)
	}
	matchingNodes := 0
	for _, node := range graph.Nodes {
		if node["type"] == "article" && node["work_id"] == float64(fixture.availableID) {
			matchingNodes++
			if node["revision_id"] != float64(latestID) {
				t.Errorf("graph revision=%v, want %d", node["revision_id"], latestID)
			}
		}
	}
	if matchingNodes != 1 {
		t.Fatalf("graph returned %d nodes for work %d, want one", matchingNodes, fixture.availableID)
	}

	if response := viewerRequest(t, handler, fmt.Sprintf("/api/articles/%d?run_id=%d", fixture.revisionID, fixture.runID)); response.Code != http.StatusNotFound {
		t.Errorf("stale article detail status=%d, want 404", response.Code)
	}
	if response := viewerRequest(t, handler, fmt.Sprintf("/api/articles/%d?run_id=%d", latestID, fixture.runID)); response.Code != http.StatusOK {
		t.Errorf("latest article detail status=%d body=%s", response.Code, response.Body.String())
	}
}
