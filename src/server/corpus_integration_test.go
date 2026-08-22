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
	if _, err := viewer.writeDB.DB.Exec("INSERT INTO run_steps (pipeline_run_id, step_name, step_status, started_at, finished_at) VALUES (?, 'subsecond', 'completed', '2026-08-22T00:00:00.100000Z', '2026-08-22T00:00:00.350000Z')", runID); err != nil {
		t.Fatal(err)
	}
	stages := viewerRequest(t, handler, base+"/stages?page=1&per_page=20&sort=id&order=asc")
	if stages.Code != http.StatusOK || !strings.Contains(stages.Body.String(), `"stage_name":"validate"`) || !strings.Contains(stages.Body.String(), `"stage_summaries"`) || !strings.Contains(stages.Body.String(), `"run_steps"`) || !strings.Contains(stages.Body.String(), `"duration_seconds":5`) || !strings.Contains(stages.Body.String(), `"duration_seconds":0.25`) {
		t.Fatalf("run stages response: status=%d body=%s", stages.Code, stages.Body.String())
	}
	for _, path := range []string{base + "/corpus/nope", base + "/corpus/articles?sort=id;drop", base + "/stages?per_page=21", "/api/runs/999999/corpus/articles", "/api/runs/999999/stages"} {
		response := viewerRequest(t, handler, path)
		if response.Code != http.StatusBadRequest && response.Code != http.StatusNotFound {
			t.Errorf("GET %s: status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}

// TestCorpusPaginationUsesStableTiesAndClamps verifies deterministic complete traversal and populated out-of-range pages.
func TestCorpusPaginationUsesStableTiesAndClamps(t *testing.T) {
	path, runID, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	for index := 0; index < 41; index++ {
		work, err := viewer.writeDB.DB.Exec("INSERT INTO works (doi) VALUES (?)", "10.1/tied-"+stringID(int64(index)))
		if err != nil {
			t.Fatal(err)
		}
		workID, err := work.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := viewer.writeDB.DB.Exec(`INSERT INTO work_revisions
			(work_id, pipeline_run_id, payload_hash, title, producer_stage)
			VALUES (?, ?, ?, 'Tied title', 'normalize')`, workID, runID, "tied-"+stringID(int64(index))); err != nil {
			t.Fatal(err)
		}
		if _, err := viewer.writeDB.DB.Exec(`INSERT INTO run_work_stages
			(pipeline_run_id, work_id, stage_name, outcome) VALUES (?, ?, 'validate', 'valid')`, runID, workID); err != nil {
			t.Fatal(err)
		}
	}

	handler := viewer.Handler()
	seen := make(map[int64]bool)
	for page := 1; page <= 3; page++ {
		response := viewerRequest(t, handler, "/api/runs/"+stringID(runID)+"/corpus/articles?page="+stringID(int64(page))+"&per_page=20&sort=title&order=asc")
		if response.Code != http.StatusOK {
			t.Fatalf("corpus page %d status=%d body=%s", page, response.Code, response.Body.String())
		}
		var payload struct {
			Rows       []map[string]any `json:"rows"`
			Pagination struct {
				Page       int `json:"page"`
				TotalPages int `json:"total_pages"`
			} `json:"pagination"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Pagination.Page != page || payload.Pagination.TotalPages != 3 {
			t.Fatalf("corpus page metadata = %+v", payload.Pagination)
		}
		for _, row := range payload.Rows {
			id := int64(row["id"].(float64))
			if seen[id] {
				t.Fatalf("corpus revision %d appeared on multiple pages", id)
			}
			seen[id] = true
		}
	}
	if len(seen) != 42 {
		t.Fatalf("traversed %d corpus revisions, want 42", len(seen))
	}
	outOfRange := viewerRequest(t, handler, "/api/runs/"+stringID(runID)+"/corpus/articles?page=999&per_page=20&sort=title&order=asc")
	if outOfRange.Code != http.StatusOK {
		t.Fatalf("out-of-range corpus status=%d body=%s", outOfRange.Code, outOfRange.Body.String())
	}
	var clamped struct {
		Rows       []map[string]any `json:"rows"`
		Pagination struct {
			Page int `json:"page"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(outOfRange.Body.Bytes(), &clamped); err != nil {
		t.Fatal(err)
	}
	if clamped.Pagination.Page != 3 || len(clamped.Rows) != 2 {
		t.Fatalf("out-of-range corpus page=%d rows=%d, want page 3 with 2 rows", clamped.Pagination.Page, len(clamped.Rows))
	}
}
