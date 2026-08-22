// hierarchy_integration_test.go verifies bounded Home discovery and hierarchy filtering.
//
//go:build integration

package server

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"analysis/database"
)

// TestHierarchyProvidesBoundedIndependentHomeSections verifies paging, filters, ancestry, and cursor ownership at workspace scale.
func TestHierarchyProvidesBoundedIndependentHomeSections(t *testing.T) {
	path, _, _, _ := viewerFixture(t)
	db, err := database.OpenExisting(path)
	if err != nil {
		t.Fatal(err)
	}
	for index := 1; index <= 105; index++ {
		search, err := db.DB.Exec("INSERT INTO searches (search_id) VALUES (?)", fmt.Sprintf("scale-search-%03d", index))
		if err != nil {
			t.Fatal(err)
		}
		searchID, _ := search.LastInsertId()
		revision, err := db.DB.Exec("INSERT INTO search_revisions (search_id, revision_label, config_artifact_hash, resolved_manifest_hash) VALUES (?, ?, ?, ?)", searchID, fmt.Sprintf("scale-r-%03d", index), fmt.Sprintf("config-%03d", index), fmt.Sprintf("manifest-%03d", index))
		if err != nil {
			t.Fatal(err)
		}
		revisionID, _ := revision.LastInsertId()
		plan, err := db.DB.Exec("INSERT INTO execution_plans (search_revision_id, execution_fingerprint, resolved_manifest_hash, input_manifest_hash, enrichment_enabled) VALUES (?, ?, ?, ?, 0)", revisionID, fmt.Sprintf("fingerprint-%03d", index), fmt.Sprintf("manifest-%03d", index), fmt.Sprintf("input-%03d", index))
		if err != nil {
			t.Fatal(err)
		}
		planID, _ := plan.LastInsertId()
		if _, err := db.DB.Exec("INSERT INTO pipeline_runs (step, started_at, finished_at, status, execution_plan_id, attempt_number) VALUES ('workspace', ?, ?, 'completed', ?, 1)", fmt.Sprintf("2026-07-%02d 10:00:00", (index%28)+1), fmt.Sprintf("2026-07-%02d 10:05:00", (index%28)+1), planID); err != nil {
			t.Fatal(err)
		}
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

	status, summary := requestJSON(t, handler, "/api/hierarchy?section=summary")
	if status != http.StatusOK || summary["version"] != "1" {
		t.Fatalf("summary status=%d body=%v", status, summary)
	}
	totals := summary["totals"].(map[string]any)
	if totals["searches"].(float64) != 106 || totals["runs"].(float64) != 107 {
		t.Fatalf("unexpected hierarchy totals: %v", totals)
	}

	status, first := requestJSON(t, handler, "/api/hierarchy?section=searches")
	if status != http.StatusOK {
		t.Fatalf("first search page status=%d body=%v", status, first)
	}
	firstItems := first["items"].([]any)
	if len(firstItems) != hierarchyPageLimit || first["has_more"] != true {
		t.Fatalf("first search page was not bounded: %v", first)
	}
	cursor := first["next_cursor"].(string)
	status, second := requestJSON(t, handler, "/api/hierarchy?section=searches&cursor="+url.QueryEscape(cursor))
	if status != http.StatusOK {
		t.Fatalf("second search page status=%d body=%v", status, second)
	}
	seen := make(map[float64]bool)
	for _, raw := range firstItems {
		seen[raw.(map[string]any)["id"].(float64)] = true
	}
	for _, raw := range second["items"].([]any) {
		id := raw.(map[string]any)["id"].(float64)
		if seen[id] {
			t.Fatalf("search %v appeared on both cursor pages", id)
		}
	}

	status, filtered := requestJSON(t, handler, "/api/hierarchy?section=runs&q=scale-search-005&visibility=all")
	if status != http.StatusOK || len(filtered["items"].([]any)) != 1 {
		t.Fatalf("filtered run page status=%d body=%v", status, filtered)
	}
	filteredRun := filtered["items"].([]any)[0].(map[string]any)
	if filteredRun["search_name"] != "scale-search-005" || filteredRun["search_revision_id"] == nil || filteredRun["execution_plan_id"] == nil {
		t.Fatalf("filtered run lacks complete ancestry: %v", filteredRun)
	}
	searchID := int64(filteredRun["search_id"].(float64))
	revisionID := int64(filteredRun["search_revision_id"].(float64))
	planID := int64(filteredRun["execution_plan_id"].(float64))
	runID := int64(filteredRun["id"].(float64))
	for _, request := range []string{
		fmt.Sprintf("/api/hierarchy?section=searches&q=no-match&selected_id=%d", searchID),
		fmt.Sprintf("/api/hierarchy?section=revisions&search_id=%d&q=no-match&selected_id=%d", searchID, revisionID),
		fmt.Sprintf("/api/hierarchy?section=plans&search_revision_id=%d&q=no-match&selected_id=%d", revisionID, planID),
		fmt.Sprintf("/api/hierarchy?section=attempts&plan_id=%d&q=no-match&selected_id=%d", planID, runID),
	} {
		status, selected := requestJSON(t, handler, request)
		if status != http.StatusOK || selected["selected_item"] == nil {
			t.Fatalf("exact selected option %s status=%d body=%v", request, status, selected)
		}
	}

	status, trashed := requestJSON(t, handler, "/api/hierarchy?section=runs&visibility=trashed")
	if status != http.StatusOK || len(trashed["items"].([]any)) != 1 {
		t.Fatalf("trashed run filter status=%d body=%v", status, trashed)
	}

	status, _ = requestJSON(t, handler, "/api/hierarchy?section=searches&q=changed&cursor="+url.QueryEscape(cursor))
	if status != http.StatusBadRequest {
		t.Fatalf("cursor reused across filters returned status %d", status)
	}
	status, _ = requestJSON(t, handler, "/api/hierarchy?section=runs&started_after=08/01/2026")
	if status != http.StatusBadRequest {
		t.Fatalf("invalid calendar filter returned status %d", status)
	}

	for _, request := range []struct {
		path, collection string
	}{
		{"/api/searches", "searches"},
		{"/api/runs?include_trashed=true", "runs"},
	} {
		status, legacy := requestJSON(t, handler, request.path)
		if status != http.StatusOK || legacy["deprecated"] != true || legacy["has_more"] != true {
			t.Fatalf("legacy discovery %s was not explicitly deprecated and bounded: status=%d body=%v", request.path, status, legacy)
		}
		if len(legacy[request.collection].([]any)) != legacyDiscoveryLimit {
			t.Fatalf("legacy discovery %s returned %d rows, want %d", request.path, len(legacy[request.collection].([]any)), legacyDiscoveryLimit)
		}
	}
}
