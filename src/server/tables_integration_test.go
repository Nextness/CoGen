// tables_integration_test.go tests the table browser and error handling
// for invalid table names, sort columns, and pagination limits.
//
//go:build integration

package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// TestAPITableBrowserErrors verifies api table browser errors.
func TestAPITableBrowserErrors(t *testing.T) {
	path, _, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	handler := viewer.Handler()
	for _, path := range []string{"/api/tables/nope", "/api/tables/works?per_page=21", "/api/tables/works?sort=id;DROP%20TABLE%20works", "/api/tables/works?order=sideways"} {
		response := viewerRequest(t, handler, path)
		if response.Code != http.StatusBadRequest && response.Code != http.StatusNotFound {
			t.Errorf("GET %s: status=%d body=%s", path, response.Code, response.Body.String())
		}
		if !strings.Contains(response.Body.String(), `"error"`) {
			t.Errorf("GET %s did not return JSON API error", path)
		}
	}
}

// TestAPITableBrowserUsesSafeBoundedProjection verifies request-time counts, redaction, binary omission, and page canonicalization.
func TestAPITableBrowserUsesSafeBoundedProjection(t *testing.T) {
	path, _, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	handler := viewer.Handler()

	status, discovery := requestJSON(t, handler, "/api/tables")
	if status != http.StatusOK {
		t.Fatalf("table discovery status=%d body=%v", status, discovery)
	}
	encodedDiscovery, err := json.Marshal(discovery)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encodedDiscovery), "row_count") {
		t.Fatalf("table discovery must not expose cached startup counts: %s", encodedDiscovery)
	}
	if strings.Contains(string(encodedDiscovery), `"name":"data"`) {
		t.Fatalf("binary artifact data must not be discoverable as a browsable field: %s", encodedDiscovery)
	}

	status, page := requestJSON(t, handler, "/api/tables/source_records?page=999&per_page=20&sort=parse_status&order=asc")
	if status != http.StatusOK {
		t.Fatalf("table page status=%d body=%v", status, page)
	}
	pagination := page["pagination"].(map[string]any)
	if pagination["page"] != pagination["total_pages"] {
		t.Fatalf("out-of-range page was not canonicalized: %v", pagination)
	}
	rows := page["rows"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["raw_payload"] != "[redacted]" {
		t.Fatalf("sensitive payload was not redacted: %v", rows)
	}
	truncated := page["truncated_fields"].(map[string]any)
	if _, ok := truncated["raw_payload"]; !ok {
		t.Fatalf("redaction metadata is missing: %v", truncated)
	}
}
