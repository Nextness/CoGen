// tables_integration_test.go tests the table browser and error handling
// for invalid table names, sort columns, and pagination limits.
//
//go:build integration

package server

import (
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
