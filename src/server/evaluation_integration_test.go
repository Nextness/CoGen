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
	code, body := requestJSON(t, fixture.server.Handler(), fmt.Sprintf(
		"/api/runs/%d/evaluation?page=1&per_page=20&sort=title&order=asc", fixture.runID,
	))
	if code != http.StatusOK {
		t.Fatalf("evaluation response: code=%d body=%v", code, body)
	}
	pagination := body["pagination"].(map[string]any)
	if pagination["total_rows"].(float64) != 2 {
		t.Fatalf("evaluation pagination = %#v, want two normalized articles", pagination)
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
}
