//go:build integration

package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestReviewAPIInitializesAndMutatesMetadataOnly verifies the public review lifecycle, conflicts, parser errors, and PDF read-only ownership.
func TestReviewAPIInitializesAndMutatesMetadataOnly(t *testing.T) {
	fixture := newPDFViewerFixture(t)
	handler := fixture.server.Handler()
	var pdfRowsBefore int
	if err := fixture.server.pdfDB.QueryRow("SELECT COUNT(*) FROM pdf_documents").Scan(&pdfRowsBefore); err != nil {
		t.Fatal(err)
	}

	status, initial := requestJSON(t, handler, fmt.Sprintf("/api/runs/%d/articles/%d/review", fixture.runID, fixture.revisionID))
	if status != http.StatusOK || initial["context_initialized"] != false || initial["editable"] != false {
		t.Fatalf("initial review: status=%d body=%v", status, initial)
	}
	status, contextBody := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/review-context", fixture.runID), `{"parent_context_id":null}`, "")
	if status != http.StatusCreated || contextBody["context_initialized"] != true {
		t.Fatalf("create context: status=%d body=%v", status, contextBody)
	}
	for _, path := range []string{
		fmt.Sprintf("/api/runs/%d/review-context", fixture.runID),
		fmt.Sprintf("/api/runs/%d/review-context-candidates?scope=same_search&limit=10", fixture.runID),
	} {
		if status, body := requestJSON(t, handler, path); status != http.StatusOK {
			t.Fatalf("GET %s: status=%d body=%v", path, status, body)
		}
	}
	status, unavailable := mutationJSON(t, handler, http.MethodPut, fmt.Sprintf("/api/runs/%d/articles/%d/review", fixture.runID, fixture.notAvailableRevisionID), `{"expected_version_id":null,"status":"approved","sub_statuses":[],"reason":null}`, "")
	if status != http.StatusConflict || unavailable["error"].(map[string]any)["code"] != "pdf_unavailable" {
		t.Fatalf("unavailable PDF review: status=%d body=%v", status, unavailable)
	}
	status, wrongRun := mutationJSON(t, handler, http.MethodPut, fmt.Sprintf("/api/runs/%d/articles/%d/review", fixture.runID+1000, fixture.revisionID), `{"expected_version_id":null,"status":"approved","sub_statuses":[],"reason":null}`, "")
	if status != http.StatusNotFound {
		t.Fatalf("wrong run review: status=%d body=%v", status, wrongRun)
	}
	status, saved := mutationJSON(t, handler, http.MethodPut, fmt.Sprintf("/api/runs/%d/articles/%d/review", fixture.runID, fixture.revisionID), `{"expected_version_id":null,"status":"approved","sub_statuses":[],"reason":"Relevant"}`, "")
	if status != http.StatusOK || saved["changed"] != true {
		t.Fatalf("save review: status=%d body=%v", status, saved)
	}
	reviewVersionID := int64(saved["review"].(map[string]any)["version"].(map[string]any)["id"].(float64))
	status, articleDetail := requestJSON(t, handler, fmt.Sprintf("/api/articles/%d", fixture.revisionID))
	if status != http.StatusOK {
		t.Fatalf("article detail after review: status=%d body=%v", status, articleDetail)
	}
	foundReviewAudit := false
	for _, raw := range articleDetail["audit_events"].([]any) {
		if raw.(map[string]any)["action"] == "work_review_version_created" {
			foundReviewAudit = true
			break
		}
	}
	if !foundReviewAudit {
		t.Fatalf("article audit events omitted saved review decision: %v", articleDetail["audit_events"])
	}
	status, unchangedReview := mutationJSON(t, handler, http.MethodPut, fmt.Sprintf("/api/runs/%d/articles/%d/review", fixture.runID, fixture.revisionID), fmt.Sprintf(`{"expected_version_id":%d,"status":"approved","sub_statuses":[],"reason":"Relevant"}`, reviewVersionID), "")
	if status != http.StatusOK || unchangedReview["changed"] != false {
		t.Fatalf("unchanged review: status=%d body=%v", status, unchangedReview)
	}
	status, conflict := mutationJSON(t, handler, http.MethodPut, fmt.Sprintf("/api/runs/%d/articles/%d/review", fixture.runID, fixture.revisionID), `{"expected_version_id":null,"status":"removed","sub_statuses":["duplicate"],"reason":null}`, "")
	if status != http.StatusConflict || conflict["error"].(map[string]any)["code"] != "version_conflict" {
		t.Fatalf("review conflict: status=%d body=%v", status, conflict)
	}

	status, syntax := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/articles/%d/notes", fixture.runID, fixture.revisionID), `{"body":"[[ext:javascript:alert(1)]]"}`, "")
	if status != http.StatusBadRequest || syntax["error"].(map[string]any)["details"].(map[string]any)["syntax_errors"] == nil {
		t.Fatalf("note syntax response: status=%d body=%v", status, syntax)
	}
	status, noteBody := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/articles/%d/notes", fixture.runID, fixture.revisionID), `{"body":"See [[pdf:page=1|the PDF]]."}`, "")
	if status != http.StatusCreated {
		t.Fatalf("create note: status=%d body=%v", status, noteBody)
	}
	noteID := int64(noteBody["note"].(map[string]any)["id"].(float64))
	noteVersionID := int64(noteBody["note"].(map[string]any)["version"].(map[string]any)["id"].(float64))
	status, noteVersion := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/notes/%d/versions", fixture.runID, noteID), fmt.Sprintf(`{"expected_version_id":%d,"state":"active","body":"Edited [[pdf:page=1]]."}`, noteVersionID), "")
	if status != http.StatusCreated || noteVersion["changed"] != true {
		t.Fatalf("edit note: status=%d body=%v", status, noteVersion)
	}
	noteVersionID = int64(noteVersion["note"].(map[string]any)["version"].(map[string]any)["id"].(float64))
	for _, path := range []string{
		fmt.Sprintf("/api/runs/%d/articles/%d/review", fixture.runID, fixture.revisionID),
		fmt.Sprintf("/api/runs/%d/articles/%d/review/versions?limit=10", fixture.runID, fixture.revisionID),
		fmt.Sprintf("/api/runs/%d/articles/%d/notes?limit=10", fixture.runID, fixture.revisionID),
		fmt.Sprintf("/api/runs/%d/notes/%d", fixture.runID, noteID),
		fmt.Sprintf("/api/runs/%d/notes/%d/versions?limit=10", fixture.runID, noteID),
		fmt.Sprintf("/api/runs/%d/links/backlinks?target_type=pdf_page&target_id=1&limit=10", fixture.runID),
	} {
		if status, body := requestJSON(t, handler, path); status != http.StatusOK {
			t.Fatalf("GET %s: status=%d body=%v", path, status, body)
		}
	}
	status, deletedNote := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/notes/%d/versions", fixture.runID, noteID), fmt.Sprintf(`{"expected_version_id":%d,"state":"deleted","body":""}`, noteVersionID), "")
	if status != http.StatusCreated || deletedNote["changed"] != true {
		t.Fatalf("delete note: status=%d body=%v", status, deletedNote)
	}

	status, anchor := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/articles/%d/anchors", fixture.runID, fixture.revisionID), `{"anchor_id":"methods-1","page":1,"selected_text":"Methods","rectangles":[{"x":0.1,"y":0.2,"width":0.3,"height":0.1}]}`, "")
	if status != http.StatusCreated || anchor["anchor"] == nil {
		t.Fatalf("create anchor: status=%d body=%v", status, anchor)
	}
	anchorVersionID := int64(anchor["anchor"].(map[string]any)["version"].(map[string]any)["id"].(float64))
	for _, path := range []string{
		fmt.Sprintf("/api/runs/%d/articles/%d/anchors?limit=10", fixture.runID, fixture.revisionID),
		fmt.Sprintf("/api/runs/%d/anchors/methods-1/versions?limit=10", fixture.runID),
	} {
		if status, body := requestJSON(t, handler, path); status != http.StatusOK {
			t.Fatalf("GET %s: status=%d body=%v", path, status, body)
		}
	}
	status, unchangedAnchor := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/anchors/methods-1/versions", fixture.runID), fmt.Sprintf(`{"expected_version_id":%d,"state":"active","page":1,"selected_text":"Methods","rectangles":[{"x":0.1,"y":0.2,"width":0.3,"height":0.1}]}`, anchorVersionID), "")
	if status != http.StatusOK || unchangedAnchor["changed"] != false {
		t.Fatalf("unchanged anchor: status=%d body=%v", status, unchangedAnchor)
	}
	status, deletedAnchor := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/anchors/methods-1/versions", fixture.runID), fmt.Sprintf(`{"expected_version_id":%d,"state":"deleted","page":0,"selected_text":"","rectangles":[]}`, anchorVersionID), "")
	if status != http.StatusCreated || deletedAnchor["changed"] != true {
		t.Fatalf("delete anchor: status=%d body=%v", status, deletedAnchor)
	}
	status, evaluation := requestJSON(t, handler, fmt.Sprintf("/api/runs/%d/evaluation", fixture.runID))
	if status != http.StatusOK {
		t.Fatalf("evaluation: status=%d body=%v", status, evaluation)
	}
	rows := evaluation["rows"].([]any)
	found := false
	for _, raw := range rows {
		row := raw.(map[string]any)
		if int64(row["work_revision_id"].(float64)) == fixture.revisionID {
			found = row["review_status"] == "approved" && row["review_inherited"] == false
		}
	}
	if !found {
		t.Fatalf("evaluation review overlay missing: %v", rows)
	}
	var pdfRowsAfter int
	if err := fixture.server.pdfDB.QueryRow("SELECT COUNT(*) FROM pdf_documents").Scan(&pdfRowsAfter); err != nil {
		t.Fatal(err)
	}
	if pdfRowsAfter != pdfRowsBefore {
		t.Fatalf("review write changed PDF database rows: before=%d after=%d", pdfRowsBefore, pdfRowsAfter)
	}
}

// TestReviewDecisionAuditCapturesCompleteState verifies decision audit evidence records every changed review field.
func TestReviewDecisionAuditCapturesCompleteState(t *testing.T) {
	fixture := newPDFViewerFixture(t)
	handler := fixture.server.Handler()
	status, contextBody := mutationJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/runs/%d/review-context", fixture.runID), `{"parent_context_id":null}`, "")
	if status != http.StatusCreated || contextBody["context_initialized"] != true {
		t.Fatalf("create context: status=%d body=%v", status, contextBody)
	}
	status, first := mutationJSON(t, handler, http.MethodPut, fmt.Sprintf("/api/runs/%d/articles/%d/review", fixture.runID, fixture.revisionID), `{"expected_version_id":null,"status":"approved","sub_statuses":[],"reason":"Initially met the inclusion criteria"}`, "")
	if status != http.StatusOK || first["changed"] != true {
		t.Fatalf("first review: status=%d body=%v", status, first)
	}
	firstVersionID := int64(first["review"].(map[string]any)["version"].(map[string]any)["id"].(float64))
	status, second := mutationJSON(t, handler, http.MethodPut, fmt.Sprintf("/api/runs/%d/articles/%d/review", fixture.runID, fixture.revisionID), fmt.Sprintf(`{"expected_version_id":%d,"status":"not_approved","sub_statuses":["out_of_scope","not_peer_reviewed"],"reason":"Excluded after full-text review"}`, firstVersionID), "")
	if status != http.StatusOK || second["changed"] != true {
		t.Fatalf("second review: status=%d body=%v", status, second)
	}

	status, articleDetail := requestJSON(t, handler, fmt.Sprintf("/api/articles/%d", fixture.revisionID))
	if status != http.StatusOK {
		t.Fatalf("article detail: status=%d body=%v", status, articleDetail)
	}
	var before, after struct {
		Status      string   `json:"status"`
		Reason      *string  `json:"reason"`
		Substatuses []string `json:"sub_statuses"`
	}
	found := false
	for _, raw := range articleDetail["audit_events"].([]any) {
		event := raw.(map[string]any)
		if event["action"] != "work_review_version_created" {
			continue
		}
		if err := json.Unmarshal([]byte(event["after_json"].(string)), &after); err != nil || after.Status != "not_approved" {
			continue
		}
		if err := json.Unmarshal([]byte(event["before_json"].(string)), &before); err != nil {
			t.Fatalf("decode previous review audit state: %v", err)
		}
		found = true
		break
	}
	if !found {
		t.Fatalf("updated review audit event not found: %v", articleDetail["audit_events"])
	}
	if before.Status != "approved" || before.Reason == nil || *before.Reason != "Initially met the inclusion criteria" || len(before.Substatuses) != 0 {
		t.Fatalf("previous review audit state = %+v", before)
	}
	if after.Reason == nil || *after.Reason != "Excluded after full-text review" || len(after.Substatuses) != 2 || after.Substatuses[0] != "not_peer_reviewed" || after.Substatuses[1] != "out_of_scope" {
		t.Fatalf("new review audit state = %+v", after)
	}
}

// TestReviewMutationTransportGuards verifies content type, body bounds, unknown fields, trailing JSON, and origin checks.
func TestReviewMutationTransportGuards(t *testing.T) {
	fixture := newPDFViewerFixture(t)
	path := fmt.Sprintf("/api/runs/%d/review-context", fixture.runID)
	for _, test := range []struct {
		name, body, contentType, origin string
		want                            int
	}{
		{"media type", `{}`, "text/plain", "", http.StatusUnsupportedMediaType},
		{"unknown field", `{"unexpected":true}`, "application/json", "", http.StatusBadRequest},
		{"trailing value", `{"parent_context_id":null}{}`, "application/json", "", http.StatusBadRequest},
		{"origin", `{"parent_context_id":null}`, "application/json", "http://evil.test", http.StatusForbidden},
		{"oversized", `{"parent_context_id":null,"padding":"` + strings.Repeat("x", 524288) + `"}`, "application/json", "", http.StatusRequestEntityTooLarge},
	} {
		t.Run(test.name, func(t *testing.T) {
			status, _ := mutationJSON(t, fixture.server.Handler(), http.MethodPost, path, test.body, test.origin, test.contentType)
			if status != test.want {
				t.Fatalf("status=%d want=%d", status, test.want)
			}
		})
	}
}

// TestReviewReadValidation verifies bounded pagination, target validation, and the local server configuration contract.
func TestReviewReadValidation(t *testing.T) {
	fixture := newPDFViewerFixture(t)
	handler := fixture.server.Handler()
	if !fixture.server.PDFStoreBound() {
		t.Fatal("fixture did not bind its companion PDF store")
	}
	configured := fixture.server.HTTPServer("127.0.0.1:8080")
	if configured.Addr != "127.0.0.1:8080" || configured.Handler == nil || configured.ReadHeaderTimeout == 0 {
		t.Fatalf("HTTP server configuration = %+v", configured)
	}
	for _, path := range []string{
		"/api/runs/invalid/review-context",
		fmt.Sprintf("/api/runs/%d/review-context-candidates?limit=0", fixture.runID),
		fmt.Sprintf("/api/runs/%d/review-context-candidates?cursor=invalid", fixture.runID),
		fmt.Sprintf("/api/runs/%d/articles/%d/review/versions?cursor=0", fixture.runID, fixture.revisionID),
		fmt.Sprintf("/api/runs/%d/articles/%d/notes?limit=101", fixture.runID, fixture.revisionID),
		fmt.Sprintf("/api/runs/%d/articles/%d/anchors?limit=invalid", fixture.runID, fixture.revisionID),
		fmt.Sprintf("/api/runs/%d/links/backlinks?target_type=invalid&target_id=1", fixture.runID),
	} {
		if status, body := requestJSON(t, handler, path); status != http.StatusBadRequest {
			t.Errorf("GET %s: status=%d body=%v", path, status, body)
		}
	}
}

// TestLoopbackAuthorityRejectsRebindingHost verifies the HTTP server boundary accepts only its exact bound authority.
func TestLoopbackAuthorityRejectsRebindingHost(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := enforceLoopbackAuthority("127.0.0.1:8080", next)
	for host, want := range map[string]int{"127.0.0.1:8080": http.StatusNoContent, "localhost:8080": http.StatusBadRequest, "127.0.0.1:9999": http.StatusBadRequest, "example.test:8080": http.StatusBadRequest} {
		request := httptest.NewRequest(http.MethodGet, "http://"+host+"/", nil)
		request.Host = host
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != want {
			t.Errorf("Host %q status=%d want=%d", host, recorder.Code, want)
		}
	}
}

// mutationJSON invokes one review mutation and decodes its object response for assertions.
func mutationJSON(t *testing.T, handler http.Handler, method, path, body, origin string, contentTypes ...string) (int, map[string]any) {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	contentType := "application/json"
	if len(contentTypes) != 0 {
		contentType = contentTypes[0]
	}
	request.Header.Set("Content-Type", contentType)
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	var result map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode mutation response: %v\n%s", err, recorder.Body.String())
	}
	return recorder.Code, result
}
