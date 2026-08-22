// audit_integration_test.go tests audit, artifact, and cache-use endpoints.
//
//go:build integration

package server

import (
	"encoding/json"
	"net/http"
	"net/url"
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
	if artifacts.Code != http.StatusOK || !strings.Contains(artifacts.Body.String(), "workspace_config") || !strings.Contains(artifacts.Body.String(), "resolved_manifest") || !strings.Contains(artifacts.Body.String(), "input_manifest") || !strings.Contains(artifacts.Body.String(), "research") || !strings.Contains(artifacts.Body.String(), "preflight") || !strings.Contains(artifacts.Body.String(), "cache_payload") || !strings.Contains(artifacts.Body.String(), "identity_candidate_payload") {
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

// TestRunArtifactsPaginatesEveryRelationshipAndFocusesAnExactArtifact verifies the complete bounded inventory contract.
func TestRunArtifactsPaginatesEveryRelationshipAndFocusesAnExactArtifact(t *testing.T) {
	path, runID, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	var focusedID int64
	for index := 0; index < 30; index++ {
		artifact, err := viewer.writeDB.DB.Exec("INSERT INTO artifacts (content_hash, byte_size, content_type) VALUES (?, 0, 'text/plain')", "paged-artifact-"+stringID(int64(index)))
		if err != nil {
			t.Fatal(err)
		}
		artifactID, _ := artifact.LastInsertId()
		focusedID = artifactID
		if _, err := viewer.writeDB.DB.Exec("INSERT INTO run_steps (pipeline_run_id, step_name, step_status, output_artifact_id) VALUES (?, ?, 'completed', ?)", runID, "paged-step-"+stringID(int64(index)), artifactID); err != nil {
			t.Fatal(err)
		}
	}
	handler := viewer.Handler()
	status, first := requestJSON(t, handler, "/api/runs/"+stringID(runID)+"/artifacts?limit=25")
	if status != http.StatusOK || len(first["artifacts"].([]any)) != 25 || first["has_more"] != true {
		t.Fatalf("first artifact page status=%d body=%v", status, first)
	}
	cursor := first["next_cursor"].(string)
	status, second := requestJSON(t, handler, "/api/runs/"+stringID(runID)+"/artifacts?limit=25&cursor="+url.QueryEscape(cursor))
	if status != http.StatusOK || len(second["artifacts"].([]any)) < 1 {
		t.Fatalf("second artifact page status=%d body=%v", status, second)
	}
	status, focused := requestJSON(t, handler, "/api/runs/"+stringID(runID)+"/artifacts?limit=25&artifact_id="+stringID(focusedID))
	if status != http.StatusOK || int64(focused["artifacts"].([]any)[0].(map[string]any)["id"].(float64)) != focusedID {
		t.Fatalf("focused artifact was not sequenced first: status=%d body=%v", status, focused)
	}
	status, cache := requestJSON(t, handler, "/api/runs/"+stringID(runID)+"/artifacts?role=cache_payload&limit=25")
	if status != http.StatusOK || len(cache["artifacts"].([]any)) != 1 {
		t.Fatalf("cache artifact relationship filter status=%d body=%v", status, cache)
	}
}

// TestAuditSeparatesReviewPagesAndRunScopedPDFEvidence verifies category support, first-page metadata, and PDF membership isolation.
func TestAuditSeparatesReviewPagesAndRunScopedPDFEvidence(t *testing.T) {
	path, runID, revisionID, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	var workID int64
	if err := viewer.writeDB.DB.QueryRow("SELECT work_id FROM work_revisions WHERE id=?", revisionID).Scan(&workID); err != nil {
		t.Fatal(err)
	}
	otherWork, err := viewer.writeDB.DB.Exec("INSERT INTO works (doi) VALUES ('10.1/audit-other')")
	if err != nil {
		t.Fatal(err)
	}
	otherWorkID, _ := otherWork.LastInsertId()
	if _, err := viewer.writeDB.DB.Exec(`INSERT INTO audit_events
		(occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, metadata_json)
		VALUES
		(datetime('now'), 'reviewer', ?, 'work_review_version', '1', 'work_review_version_created', '{"status":"approved"}'),
		(datetime('now', '-1 second'), 'reviewer', ?, 'review_note', '1', 'review_note_created', '{}'),
		(datetime('now'), 'pdf-store', NULL, 'work', ?, 'pdf_document_inventoried', '{}'),
		(datetime('now'), 'pdf-store', NULL, 'work', ?, 'pdf_document_inventoried', '{}')`, runID, runID, stringID(workID), stringID(otherWorkID)); err != nil {
		t.Fatal(err)
	}
	handler := viewer.Handler()

	status, review := requestJSON(t, handler, "/api/audit?run_id="+stringID(runID)+"&category=review&limit=1")
	if status != http.StatusOK || len(review["events"].([]any)) != 1 || review["summary"] == nil || review["facets"] == nil {
		t.Fatalf("review first page status=%d body=%v", status, review)
	}
	cursor := review["next_cursor"]
	if cursor != nil {
		status, older := requestJSON(t, handler, "/api/audit?run_id="+stringID(runID)+"&category=review&limit=1&cursor="+stringID(int64(cursor.(float64))))
		if status != http.StatusOK || older["summary"] != nil || older["facets"] != nil {
			t.Fatalf("older audit page recomputed first-page metadata: status=%d body=%v", status, older)
		}
	}

	status, pdfRun := requestJSON(t, handler, "/api/audit?run_id="+stringID(runID)+"&category=pdf&limit=100")
	if status != http.StatusOK {
		t.Fatalf("run PDF audit status=%d body=%v", status, pdfRun)
	}
	for _, raw := range pdfRun["events"].([]any) {
		event := raw.(map[string]any)
		if event["entity_id"] == stringID(otherWorkID) {
			t.Fatalf("unrelated global PDF event contaminated run audit: %v", event)
		}
	}
	status, workspacePDF := requestJSON(t, handler, "/api/audit?run_id="+stringID(runID)+"&category=pdf&pdf_scope=workspace&limit=100")
	if status != http.StatusOK || len(workspacePDF["events"].([]any)) <= len(pdfRun["events"].([]any)) {
		t.Fatalf("workspace PDF scope did not expose separate global history: status=%d body=%v", status, workspacePDF)
	}
}

// TestAuditRecordedDataIsLazyBoundedAndPrivate verifies structured review filtering and the explicit payload endpoint.
func TestAuditRecordedDataIsLazyBoundedAndPrivate(t *testing.T) {
	path, runID, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	before, err := json.Marshal(map[string]any{"status": "in_progress", "reason": "previous", "note_body": "private note", "reviewer_email": "private@example.test"})
	if err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(map[string]any{"status": "approved", "reason": "verified", "sub_statuses": []string{"duplicate"}, "selected_text": "private selection"})
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := json.Marshal(map[string]any{"provider": "review", "oversized": strings.Repeat("x", auditDetailPayloadBytes+1)})
	if err != nil {
		t.Fatal(err)
	}
	result, err := viewer.writeDB.DB.Exec(`INSERT INTO audit_events
		(occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, before_json, after_json, metadata_json)
		VALUES (datetime('now', '+1 minute'), 'reviewer', ?, 'work_review_version', '999', 'work_review_version_created', ?, ?, ?)`, runID, string(before), string(after), string(metadata))
	if err != nil {
		t.Fatal(err)
	}
	eventID, _ := result.LastInsertId()
	handler := viewer.Handler()
	status, page := requestJSON(t, handler, "/api/audit?run_id="+stringID(runID)+"&category=review&review_status=approved&review_reason=verified&review_substatus=duplicate&limit=25")
	if status != http.StatusOK || len(page["events"].([]any)) != 1 {
		t.Fatalf("structured review audit status=%d body=%v", status, page)
	}
	encodedPage, _ := json.Marshal(page)
	if strings.Contains(string(encodedPage), "private note") || strings.Contains(string(encodedPage), "private@example.test") || strings.Contains(string(encodedPage), "private selection") || strings.Contains(string(encodedPage), strings.Repeat("x", auditListPayloadBytes)) {
		t.Fatalf("audit list exposed private or oversized recorded data: %s", encodedPage)
	}
	event := page["events"].([]any)[0].(map[string]any)
	if event["recorded_data_available"] != true || len(event["recorded_data_truncated_fields"].([]any)) != 1 {
		t.Fatalf("audit list truncation metadata=%v", event)
	}

	status, recorded := requestJSON(t, handler, "/api/audit/"+stringID(eventID)+"/recorded-data?run_id="+stringID(runID))
	if status != http.StatusOK || recorded["event_id"] != float64(eventID) {
		t.Fatalf("recorded data status=%d body=%v", status, recorded)
	}
	encodedRecorded, _ := json.Marshal(recorded)
	if strings.Contains(string(encodedRecorded), "private note") || strings.Contains(string(encodedRecorded), "private@example.test") || strings.Contains(string(encodedRecorded), "private selection") {
		t.Fatalf("recorded data exposed private fields: %s", encodedRecorded)
	}
	if len(recorded["truncated_fields"].([]any)) != 1 || recorded["after"].(map[string]any)["status"] != "approved" {
		t.Fatalf("recorded data bounds or structured state=%v", recorded)
	}
	wrongRun := viewerRequest(t, handler, "/api/audit/"+stringID(eventID)+"/recorded-data?run_id="+stringID(runID+1))
	if wrongRun.Code != http.StatusNotFound {
		t.Fatalf("cross-run recorded data status=%d body=%s", wrongRun.Code, wrongRun.Body.String())
	}
}
