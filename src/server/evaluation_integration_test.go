// evaluation_integration_test.go tests the evaluation endpoint and
// its integration with the PDF inventory.
//
//go:build integration

package server

import (
	"fmt"
	"net/http"
	"testing"
)

// TestEvaluationListsOnlyNormalizedArticlesWithInventoryState verifies evaluation lists only normalized articles with inventory state.
func TestEvaluationListsOnlyNormalizedArticlesWithInventoryState(t *testing.T) {
	fixture := newPDFViewerFixture(t)
	handler := fixture.server.Handler()
	code, body := requestJSON(t, handler, fmt.Sprintf(
		"/api/runs/%d/evaluation?page=1&per_page=20&sort=title&order=asc", fixture.runID,
	))
	if code != http.StatusOK {
		t.Fatalf("evaluation response: code=%d body=%v", code, body)
	}
	pagination := body["pagination"].(map[string]any)
	if pagination["total_rows"].(float64) != 2 {
		t.Fatalf("evaluation pagination = %#v, want two normalized articles", pagination)
	}
	if body["review_context_initialized"] != false || body["review_context"] != nil {
		t.Fatalf("uninitialized review context = %#v", body)
	}
	summary := body["review_summary"].(map[string]any)
	if summary["total"] != float64(2) || summary["reviewed"] != float64(0) || summary["unreviewed"] != float64(2) || summary["pdf_available"] != float64(1) {
		t.Fatalf("uninitialized review summary = %#v", summary)
	}
	initialNavigation := body["queue_navigation"].(map[string]any)
	if initialNavigation["next_work_revision_id"] == nil || initialNavigation["previous_work_revision_id"] == nil {
		t.Fatalf("initial unreviewed navigation = %#v", initialNavigation)
	}
	rows := body["rows"].([]any)
	statuses := make(map[string]map[string]any, len(rows))
	for _, raw := range rows {
		row := raw.(map[string]any)
		statuses[row["inventory_status"].(string)] = row
	}
	if statuses["available"] == nil || statuses["available"]["inventoried_at"] == nil {
		t.Fatalf("available evaluation row = %#v", statuses["available"])
	}
	if statuses["not_available"] == nil || statuses["not_available"]["inventoried_at"] != nil {
		t.Fatalf("not-available evaluation row = %#v", statuses["not_available"])
	}
	code, filtered := requestJSON(t, fixture.server.Handler(), fmt.Sprintf(
		"/api/runs/%d/evaluation?q=without&per_page=20", fixture.runID,
	))
	if code != http.StatusOK || filtered["pagination"].(map[string]any)["total_rows"].(float64) != 1 {
		t.Fatalf("filtered evaluation response: code=%d body=%v", code, filtered)
	}

	status, created := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/review-context", fixture.runID), `{"parent_context_id":null}`, "")
	if status != http.StatusCreated || created["context_initialized"] != true {
		t.Fatalf("initialize review context: status=%d body=%v", status, created)
	}
	status, decision := mutationJSON(t, handler, http.MethodPut, fmt.Sprintf("/api/runs/%d/articles/%d/review", fixture.runID, fixture.revisionID), `{"expected_version_id":null,"status":"not_approved","sub_statuses":["duplicate"],"reason":"Checked"}`, "")
	if status != http.StatusOK || decision["changed"] != true {
		t.Fatalf("save review decision: status=%d body=%v", status, decision)
	}

	for _, query := range []string{"review_status=not_approved", "qualifier=duplicate", "reviewed=reviewed", "pdf_status=available", "review_source=this_context"} {
		code, filtered = requestJSON(t, handler, fmt.Sprintf("/api/runs/%d/evaluation?%s&per_page=20", fixture.runID, query))
		if code != http.StatusOK || filtered["pagination"].(map[string]any)["total_rows"] != float64(1) {
			t.Errorf("evaluation filter %q: code=%d body=%#v", query, code, filtered)
		}
	}
	code, filtered = requestJSON(t, handler, fmt.Sprintf("/api/runs/%d/evaluation?current_revision_id=%d&per_page=20", fixture.runID, fixture.revisionID))
	navigation := filtered["queue_navigation"].(map[string]any)
	if code != http.StatusOK || int64(navigation["next_work_revision_id"].(float64)) == fixture.revisionID || navigation["previous_work_revision_id"] != nil {
		t.Fatalf("adjacent unreviewed navigation: code=%d body=%#v", code, filtered)
	}
	code, filtered = requestJSON(t, handler, fmt.Sprintf("/api/runs/%d/evaluation?q=no-match&page=999&per_page=20", fixture.runID))
	if code != http.StatusOK || filtered["review_context_initialized"] != true || filtered["pagination"].(map[string]any)["total_rows"] != float64(0) {
		t.Fatalf("empty initialized evaluation page: code=%d body=%#v", code, filtered)
	}
	code, filtered = requestJSON(t, handler, fmt.Sprintf("/api/runs/%d/evaluation?review_status=unknown", fixture.runID))
	if code != http.StatusBadRequest {
		t.Fatalf("invalid review status: code=%d body=%#v", code, filtered)
	}
}
