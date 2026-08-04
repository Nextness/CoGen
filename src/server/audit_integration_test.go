// audit_integration_test.go tests audit, artifact, and cache-use endpoints.
//
//go:build integration

package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// TestAPIDetailsArtifactsAndAudit verifies api details artifacts and audit.
func TestAPIDetailsArtifactsAndAudit(t *testing.T) {
	path, runID, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	handler := viewer.Handler()
	artifacts := viewerRequest(t, handler, "/api/runs/"+stringID(runID)+"/artifacts")
	if artifacts.Code != http.StatusOK || !strings.Contains(artifacts.Body.String(), "workspace_config") || !strings.Contains(artifacts.Body.String(), "resolved_manifest") || !strings.Contains(artifacts.Body.String(), "input_manifest") || !strings.Contains(artifacts.Body.String(), "research") || !strings.Contains(artifacts.Body.String(), "preflight") {
		t.Fatalf("run artifacts response: status=%d body=%s", artifacts.Code, artifacts.Body.String())
	}
	content := viewerRequest(t, handler, "/api/artifacts/1/content")
	if content.Code != http.StatusOK || content.Header().Get("Content-Disposition") != "attachment; filename=workspace-config-1.something" || content.Header().Get("X-Content-Type-Options") != "nosniff" || content.Header().Get("Cache-Control") != "no-store" || content.Body.String() != "workspace = {}" {
		t.Fatalf("artifact download response: status=%d disposition=%q body=%q", content.Code, content.Header().Get("Content-Disposition"), content.Body.String())
	}
	inspection := viewerRequest(t, handler, "/api/artifacts/2/inspect")
	if inspection.Code != http.StatusOK || !strings.Contains(inspection.Body.String(), "\"format\":\"json\"") || !strings.Contains(inspection.Body.String(), "manifest") {
		t.Fatalf("artifact inspection response: status=%d body=%s", inspection.Code, inspection.Body.String())
	}
	truncatedInspection := viewerRequest(t, handler, "/api/artifacts/2/inspect?preview_bytes=5")
	if truncatedInspection.Code != http.StatusOK || !strings.Contains(truncatedInspection.Body.String(), `"truncated":true`) || !strings.Contains(truncatedInspection.Body.String(), `"preview_byte_size":5`) || !strings.Contains(truncatedInspection.Body.String(), `"content":"{\"man"`) {
		t.Fatalf("bounded artifact inspection response: status=%d body=%s", truncatedInspection.Code, truncatedInspection.Body.String())
	}
	binary := viewerRequest(t, handler, "/api/artifacts/4/content")
	if binary.Code != http.StatusOK || binary.Header().Get("Content-Type") != "application/octet-stream" || binary.Header().Get("Content-Disposition") != "attachment; filename=artifact-4.bin" || string(binary.Body.Bytes()) != string([]byte{0, 1, 2}) {
		t.Fatalf("binary artifact download: status=%d headers=%v body=%v", binary.Code, binary.Header(), binary.Body.Bytes())
	}
	for _, path := range []string{"/api/artifacts/4/inspect", "/api/artifacts/5/content", "/api/artifacts/5/inspect", "/api/artifacts/999999/content"} {
		response := viewerRequest(t, handler, path)
		if response.Code != http.StatusBadRequest && response.Code != http.StatusNotFound {
			t.Errorf("GET %s: status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
	cachePage := viewerRequest(t, handler, "/api/runs/"+stringID(runID)+"/cache-uses?page=2&per_page=20&sort=id&order=asc")
	var cachePayload struct {
		Rows       []map[string]any `json:"rows"`
		Pagination struct {
			Page      int   `json:"page"`
			TotalRows int64 `json:"total_rows"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(cachePage.Body.Bytes(), &cachePayload); err != nil {
		t.Fatal(err)
	}
	if cachePage.Code != http.StatusOK || cachePayload.Pagination.Page != 2 || cachePayload.Pagination.TotalRows != 21 || len(cachePayload.Rows) != 1 {
		t.Fatalf("paginated cache response: status=%d payload=%#v body=%s", cachePage.Code, cachePayload, cachePage.Body.String())
	}
	validationAudit := viewerRequest(t, handler, "/api/audit?run_id="+stringID(runID)+"&category=validation&limit=1")
	if validationAudit.Code != http.StatusOK || !strings.Contains(validationAudit.Body.String(), "validation_changed") || !strings.Contains(validationAudit.Body.String(), `"total_events":1`) {
		t.Fatalf("filtered audit response: status=%d body=%s", validationAudit.Code, validationAudit.Body.String())
	}
	multiCategoryAudit := viewerRequest(t, handler, "/api/audit?run_id="+stringID(runID)+"&category=validation,enrichment&limit=25")
	if multiCategoryAudit.Code != http.StatusOK || !strings.Contains(multiCategoryAudit.Body.String(), "validation_changed") || !strings.Contains(multiCategoryAudit.Body.String(), "field_enriched") {
		t.Fatalf("multi-category audit response: status=%d body=%s", multiCategoryAudit.Code, multiCategoryAudit.Body.String())
	}
	firstAuditPage := viewerRequest(t, handler, "/api/audit?run_id="+stringID(runID)+"&limit=1")
	var auditPayload struct {
		Events     []map[string]any `json:"events"`
		HasMore    bool             `json:"has_more"`
		NextCursor int64            `json:"next_cursor"`
	}
	if err := json.Unmarshal(firstAuditPage.Body.Bytes(), &auditPayload); err != nil {
		t.Fatal(err)
	}
	if len(auditPayload.Events) != 1 || !auditPayload.HasMore || auditPayload.NextCursor < 1 {
		t.Fatalf("first audit cursor page: %#v body=%s", auditPayload, firstAuditPage.Body.String())
	}
	if auditPayload.Events[0]["action"] != "field_enriched" {
		t.Fatalf("audit page is not ordered by event time: %#v", auditPayload.Events[0])
	}
	secondAuditPage := viewerRequest(t, handler, "/api/audit?run_id="+stringID(runID)+"&limit=1&cursor="+stringID(auditPayload.NextCursor))
	var secondAuditPayload struct {
		Events []map[string]any `json:"events"`
	}
	if err := json.Unmarshal(secondAuditPage.Body.Bytes(), &secondAuditPayload); err != nil {
		t.Fatal(err)
	}
	if secondAuditPage.Code != http.StatusOK || len(secondAuditPayload.Events) != 1 || secondAuditPayload.Events[0]["id"] == float64(auditPayload.NextCursor) {
		t.Fatalf("second audit cursor page repeated the first page: status=%d payload=%#v body=%s", secondAuditPage.Code, secondAuditPayload, secondAuditPage.Body.String())
	}
	for _, path := range []string{
		"/api/audit?category=unknown",
		"/api/audit?category=validation,unknown",
		"/api/artifacts/2/inspect?preview_bytes=262145",
	} {
		response := viewerRequest(t, handler, path)
		if response.Code != http.StatusBadRequest {
			t.Errorf("GET %s: status=%d", path, response.Code)
		}
	}
}
