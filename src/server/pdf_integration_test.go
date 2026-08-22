// pdf_integration_test.go tests the PDF status and content HTTP endpoints with
// a temporary fixture-backed server.
//
//go:build integration

package server

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestPDFStatusEndpointsCoverEveryState verifies pdf status endpoints cover every state.
func TestPDFStatusEndpointsCoverEveryState(t *testing.T) {
	fixture := newPDFViewerFixture(t)
	handler := fixture.server.Handler()
	tests := []struct {
		id     int64
		status string
	}{
		{fixture.availableID, "available"},
		{fixture.notAvailableID, "not_available"},
		{fixture.unavailableID, "unavailable"},
	}
	for _, test := range tests {
		code, body := requestJSON(t, handler, fmt.Sprintf("/api/works/%d/pdf-status", test.id))
		if code != http.StatusOK || body["status"] != test.status {
			t.Fatalf("work %d status response: code=%d body=%v", test.id, code, body)
		}
	}
	code, _ := requestJSON(t, handler, "/api/works/999999/pdf-status")
	if code != http.StatusNotFound {
		t.Fatalf("missing work status code = %d", code)
	}
}

// TestPDFContentSupportsInlineRanges verifies pdf content supports inline ranges.
func TestPDFContentSupportsInlineRanges(t *testing.T) {
	fixture := newPDFViewerFixture(t)
	handler := fixture.server.Handler()
	path := fmt.Sprintf("/api/pdf/%d", fixture.availableID)
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Header.Set("Range", "bytes=0-4")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	response := recorder.Result()
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusPartialContent || string(body) != "%PDF-" {
		t.Fatalf("range response: status=%d body=%q", response.StatusCode, body)
	}
	if response.Header.Get("Content-Type") != "application/pdf" || !strings.HasPrefix(response.Header.Get("Content-Disposition"), "inline") {
		t.Fatalf("PDF headers: %v", response.Header)
	}
	code, _ := requestJSON(t, handler, fmt.Sprintf("/api/pdf/%d", fixture.notAvailableID))
	if code != http.StatusNotFound {
		t.Fatalf("failed document content code = %d", code)
	}
}

// TestArticleIncludesManualPDFAuditEvent verifies that article audit pagination includes manual PDF evidence.
func TestArticleIncludesManualPDFAuditEvent(t *testing.T) {
	fixture := newPDFViewerFixture(t)
	handler := fixture.server.Handler()
	code, article := requestJSON(t, handler, fmt.Sprintf("/api/articles/%d?run_id=%d", fixture.revisionID, fixture.runID))
	if code != http.StatusOK {
		t.Fatalf("article code=%d body=%v", code, article)
	}
	if article["pdf_status"].(map[string]any)["status"] != "available" {
		t.Fatalf("article PDF status = %#v", article["pdf_status"])
	}
	events := article["audit_events"].(map[string]any)["items"].([]any)
	found := false
	for _, raw := range events {
		if raw.(map[string]any)["action"] == "pdf_document_inventoried" {
			found = true
		}
	}
	if !found {
		t.Fatalf("article PDF audit event missing: %#v", events)
	}
	code, audit := requestJSON(t, handler, "/api/audit?category=pdf&run_id=1")
	if code != http.StatusOK || audit["summary"].(map[string]any)["total_events"].(float64) < 1 {
		t.Fatalf("PDF audit category response: code=%d body=%v", code, audit)
	}
}

// TestViewerPDFConnectionIsReadOnly verifies viewer pdf connection is read only.
func TestViewerPDFConnectionIsReadOnly(t *testing.T) {
	fixture := newPDFViewerFixture(t)
	if _, err := fixture.server.pdfDB.Exec(`INSERT INTO pdf_blobs
		(content_hash, byte_size, content, created_at) VALUES ('write-test', 1, x'00', '2026-01-01T00:00:00Z')`); err == nil {
		t.Fatal("viewer PDF connection accepted a write")
	}
}
