// corpus_integration_test.go tests the run-scoped corpus and stage endpoints.
//
//go:build integration

package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// TestRunScopedCorpusAndStages verifies run scoped corpus and stages.
func TestRunScopedCorpusAndStages(t *testing.T) {
	path, runID, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	handler := viewer.Handler()
	base := "/api/runs/" + stringID(runID)
	for _, collection := range []string{"articles", "authors", "references", "sources"} {
		response := viewerRequest(t, handler, base+"/corpus/"+collection+"?page=1&per_page=20&sort=id&order=asc")
		if response.Code != http.StatusOK {
			t.Errorf("run corpus %s: status=%d body=%s", collection, response.Code, response.Body.String())
			continue
		}
		var payload struct {
			RunID              int64            `json:"run_id"`
			Columns            []string         `json:"columns"`
			Rows               []map[string]any `json:"rows"`
			SourceResultCounts []map[string]any `json:"source_result_counts"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Errorf("decode run corpus %s: %v", collection, err)
			continue
		}
		if payload.RunID != runID || len(payload.Columns) == 0 || len(payload.Rows) == 0 {
			t.Errorf("unexpected run corpus %s payload: %#v", collection, payload)
		}
		if collection == "sources" && (len(payload.SourceResultCounts) != 1 || payload.SourceResultCounts[0]["result_count_comparison"] != "below") {
			t.Errorf("source count summary missing from source corpus payload: %#v", payload.SourceResultCounts)
		}
	}
	filtered := viewerRequest(t, handler, base+"/corpus/articles?q=article+one")
	if filtered.Code != http.StatusOK || !strings.Contains(filtered.Body.String(), "Article One") || strings.Contains(filtered.Body.String(), "Article Two") {
		t.Fatalf("run-scoped article search did not filter correctly: status=%d body=%s", filtered.Code, filtered.Body.String())
	}
	articles := viewerRequest(t, handler, base+"/corpus/articles")
	if articles.Code != http.StatusOK || !strings.Contains(articles.Body.String(), "Article One") || strings.Contains(articles.Body.String(), "Other Run Article") || strings.Contains(articles.Body.String(), "Article Two") {
		t.Fatalf("analysis-ready corpus included a non-normalized/other-run article: status=%d body=%s", articles.Code, articles.Body.String())
	}
	stages := viewerRequest(t, handler, base+"/stages?page=1&per_page=20&sort=id&order=asc")
	if stages.Code != http.StatusOK || !strings.Contains(stages.Body.String(), `"stage_name":"validate"`) || !strings.Contains(stages.Body.String(), `"stage_summaries"`) || !strings.Contains(stages.Body.String(), `"run_steps"`) || !strings.Contains(stages.Body.String(), `"duration_seconds":5`) {
		t.Fatalf("run stages response: status=%d body=%s", stages.Code, stages.Body.String())
	}
	for _, path := range []string{base + "/corpus/nope", base + "/corpus/articles?sort=id;drop", base + "/stages?per_page=21", "/api/runs/999999/corpus/articles", "/api/runs/999999/stages"} {
		response := viewerRequest(t, handler, path)
		if response.Code != http.StatusBadRequest && response.Code != http.StatusNotFound {
			t.Errorf("GET %s: status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}
