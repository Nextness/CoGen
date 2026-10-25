// pdf_mutation_integration_test.go verifies the guarded run/article PDF save
// and removal boundary against the real server handler and companion store.
package server_test

import (
	serverhelpers "analysis/tests/support/go/backend/server"
	"analysis/tests/support/go/pdffixture"

	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// pdfMutationRequest sends one raw PDF mutation and decodes its JSON envelope.
func pdfMutationRequest(t *testing.T, handler http.Handler, method string, runID, revisionID int64, body []byte, size int64, digest, etag string) (int, map[string]any) {
	t.Helper()
	path := fmt.Sprintf("/api/runs/%d/articles/%d/pdf", runID, revisionID)
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/pdf")
	request.Header.Set("X-PDF-Size", fmt.Sprint(size))
	request.Header.Set("X-PDF-SHA256", digest)
	if etag != "" {
		request.Header.Set("If-Match", etag)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	var result map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode PDF mutation response: %v\n%s", err, recorder.Body.String())
	}
	return recorder.Code, result
}

// pdfStatusForWork reads one work's current PDF status payload.
func pdfStatusForWork(t *testing.T, handler http.Handler, workID int64) map[string]any {
	t.Helper()
	status, body := serverhelpers.RequestJSON(t, handler, fmt.Sprintf("/api/works/%d/pdf-status", workID))
	if status != http.StatusOK {
		t.Fatalf("PDF status = %d: %v", status, body)
	}
	return body
}

// pdfDigest returns the lowercase SHA-256 used by upload preconditions.
func pdfDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// pdfBytes reads one stored evidence response.
func pdfBytes(t *testing.T, handler http.Handler, workID int64) []byte {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/pdf/%d", workID), nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("PDF content = %d: %s", recorder.Code, recorder.Body.String())
	}
	return recorder.Body.Bytes()
}

// TestPDFMutationReplacesAndRemovesSelection verifies successful replacement,
// removal, and idempotent removal without review-context initialization.
func TestPDFMutationReplacesAndRemovesSelection(t *testing.T) {
	fixture := serverhelpers.NewPDFViewerFixture(t)
	handler := fixture.Server.Handler()
	status := pdfStatusForWork(t, handler, fixture.AvailableID)
	etag, _ := status["mutation_etag"].(string)
	if status["registered"] != true || status["change_counter"] != float64(2) || etag == "" {
		t.Fatalf("initial PDF status = %v", status)
	}
	replacement := pdffixture.Bytes("Replacement page one", "Replacement page two")
	code, body := pdfMutationRequest(t, handler, http.MethodPut, fixture.RunID, fixture.RevisionID, replacement, int64(len(replacement)), pdfDigest(replacement), etag)
	if code != http.StatusOK || body["changed"] != true || body["operation"] != "replaced" {
		t.Fatalf("replace response = %d %v", code, body)
	}
	if body["event_key"] == nil || body["audit_delivery"] != "pending" {
		t.Fatalf("replace audit envelope = %v", body)
	}
	if !bytes.Equal(pdfBytes(t, handler, fixture.AvailableID), replacement) {
		t.Fatal("replacement bytes were not readable")
	}
	updated := body["pdf_status"].(map[string]any)
	updatedETag, _ := updated["mutation_etag"].(string)
	code, removed := pdfMutationRequest(t, handler, http.MethodDelete, fixture.RunID, fixture.RevisionID, nil, 0, "", updatedETag)
	if code != http.StatusOK || removed["changed"] != true || removed["operation"] != "removed" {
		t.Fatalf("remove response = %d %v", code, removed)
	}
	if removed["pdf_status"].(map[string]any)["status"] != "not_available" {
		t.Fatalf("removed PDF status = %v", removed["pdf_status"])
	}
	removedETag := removed["pdf_status"].(map[string]any)["mutation_etag"].(string)
	code, unchanged := pdfMutationRequest(t, handler, http.MethodDelete, fixture.RunID, fixture.RevisionID, nil, 0, "", removedETag)
	if code != http.StatusOK || unchanged["changed"] != false || unchanged["operation"] != "unchanged" || unchanged["audit_delivery"] != "not_applicable" {
		t.Fatalf("idempotent remove response = %d %v", code, unchanged)
	}
	if recorder := httptest.NewRecorder(); func() int {
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/pdf/%d", fixture.AvailableID), nil))
		return recorder.Code
	}() != http.StatusNotFound {
		t.Fatalf("removed PDF content status = %d", recorder.Code)
	}
}

// TestPDFMutationUploadsUnavailableSelection verifies an unavailable registered
// selection becomes an upload without requiring review context.
func TestPDFMutationUploadsUnavailableSelection(t *testing.T) {
	fixture := serverhelpers.NewPDFViewerFixture(t)
	handler := fixture.Server.Handler()
	contextStatus, contextBody := serverhelpers.RequestJSON(t, handler, fmt.Sprintf("/api/runs/%d/review-context", fixture.RunID))
	if contextStatus != http.StatusOK || contextBody["context_initialized"] != false {
		t.Fatalf("review context fixture = %d %v", contextStatus, contextBody)
	}
	status := pdfStatusForWork(t, handler, fixture.NotAvailableID)
	data := pdffixture.Bytes("Uploaded page")
	code, body := pdfMutationRequest(t, handler, http.MethodPut, fixture.RunID, fixture.NotAvailableRevisionID, data, int64(len(data)), pdfDigest(data), status["mutation_etag"].(string))
	if code != http.StatusOK || body["changed"] != true || body["operation"] != "uploaded" {
		t.Fatalf("upload response = %d %v", code, body)
	}
	if !bytes.Equal(pdfBytes(t, handler, fixture.NotAvailableID), data) {
		t.Fatal("uploaded bytes were not readable")
	}
}

// TestPDFMutationRejectsStalePrecondition verifies a stale or identical stale
// save fails before changing the current selection.
func TestPDFMutationRejectsStalePrecondition(t *testing.T) {
	fixture := serverhelpers.NewPDFViewerFixture(t)
	handler := fixture.Server.Handler()
	original := pdfBytes(t, handler, fixture.AvailableID)
	status := pdfStatusForWork(t, handler, fixture.AvailableID)
	stale := fmt.Sprintf("\"pdf-%d-%x\"", fixture.AvailableID, int64(1))
	data := pdffixture.Bytes("Stale replacement")
	code, body := pdfMutationRequest(t, handler, http.MethodPut, fixture.RunID, fixture.RevisionID, data, int64(len(data)), pdfDigest(data), stale)
	if code != http.StatusPreconditionFailed || body["error"].(map[string]any)["code"] != "pdf_state_conflict" {
		t.Fatalf("stale response = %d %v", code, body)
	}
	if body["error"].(map[string]any)["details"].(map[string]any)["current_pdf_status"].(map[string]any)["change_counter"] != status["change_counter"] {
		t.Fatalf("stale details = %v", body)
	}
	code, body = pdfMutationRequest(t, handler, http.MethodPut, fixture.RunID, fixture.RevisionID, original, int64(len(original)), pdfDigest(original), stale)
	if code != http.StatusPreconditionFailed {
		t.Fatalf("stale identical response = %d %v", code, body)
	}
	if !bytes.Equal(pdfBytes(t, handler, fixture.AvailableID), original) {
		t.Fatal("stale mutation changed stored bytes")
	}
}

// TestPDFMutationTransportAndMembership verifies required headers, origin,
// media, query, and exact revision membership failures.
func TestPDFMutationTransportAndMembership(t *testing.T) {
	fixture := serverhelpers.NewPDFViewerFixture(t)
	handler := fixture.Server.Handler()
	status := pdfStatusForWork(t, handler, fixture.AvailableID)
	etag := status["mutation_etag"].(string)
	data := pdffixture.Bytes("Transport test")
	digest := pdfDigest(data)
	path := fmt.Sprintf("/api/runs/%d/articles/%d/pdf", fixture.RunID, fixture.RevisionID)
	for _, test := range []struct {
		name        string
		method      string
		body        []byte
		contentType string
		origin      string
		size        string
		digest      string
		etag        string
		query       string
		want        int
	}{
		{name: "missing precondition", method: http.MethodPut, body: data, contentType: "application/pdf", size: fmt.Sprint(len(data)), digest: digest, want: http.StatusPreconditionRequired},
		{name: "weak precondition", method: http.MethodPut, body: data, contentType: "application/pdf", size: fmt.Sprint(len(data)), digest: digest, etag: "W/" + etag, want: http.StatusBadRequest},
		{name: "wildcard precondition", method: http.MethodPut, body: data, contentType: "application/pdf", size: fmt.Sprint(len(data)), digest: digest, etag: "*", want: http.StatusBadRequest},
		{name: "missing size", method: http.MethodPut, body: data, contentType: "application/pdf", digest: digest, etag: etag, want: http.StatusBadRequest},
		{name: "uppercase digest", method: http.MethodPut, body: data, contentType: "application/pdf", size: fmt.Sprint(len(data)), digest: strings.ToUpper(digest), etag: etag, want: http.StatusBadRequest},
		{name: "media type", method: http.MethodPut, body: data, contentType: "application/json", size: fmt.Sprint(len(data)), digest: digest, etag: etag, want: http.StatusUnsupportedMediaType},
		{name: "origin", method: http.MethodPut, body: data, contentType: "application/pdf", origin: "http://evil.test", size: fmt.Sprint(len(data)), digest: digest, etag: etag, want: http.StatusForbidden},
		{name: "query", method: http.MethodPut, body: data, contentType: "application/pdf", size: fmt.Sprint(len(data)), digest: digest, etag: etag, query: "?path=/tmp/x", want: http.StatusBadRequest},
		{name: "delete body", method: http.MethodDelete, body: []byte("x"), contentType: "application/pdf", etag: etag, want: http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, path+test.query, bytes.NewReader(test.body))
			request.Header.Set("Content-Type", test.contentType)
			if test.size != "" {
				request.Header.Set("X-PDF-Size", test.size)
			}
			if test.digest != "" {
				request.Header.Set("X-PDF-SHA256", test.digest)
			}
			if test.etag != "" {
				request.Header.Set("If-Match", test.etag)
			}
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != test.want {
				t.Fatalf("status = %d want %d: %s", recorder.Code, test.want, recorder.Body.String())
			}
		})
	}
	missingPath := fmt.Sprintf("/api/runs/%d/articles/999999/pdf", fixture.RunID)
	request := httptest.NewRequest(http.MethodPut, missingPath, bytes.NewReader(data))
	request.Header.Set("Content-Type", "application/pdf")
	request.Header.Set("X-PDF-Size", fmt.Sprint(len(data)))
	request.Header.Set("X-PDF-SHA256", digest)
	request.Header.Set("If-Match", etag)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("missing revision status = %d: %s", recorder.Code, recorder.Body.String())
	}
	if !bytes.Equal(pdfBytes(t, handler, fixture.AvailableID), func() []byte {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/pdf/%d", fixture.AvailableID), nil))
		return recorder.Body.Bytes()
	}()) {
		t.Fatal("rejected transport changed stored bytes")
	}
}

// TestPDFMutationRejectsUploadIdentityMismatch verifies size, digest,
// truncation, and signature failures never reach storage.
func TestPDFMutationRejectsUploadIdentityMismatch(t *testing.T) {
	fixture := serverhelpers.NewPDFViewerFixture(t)
	handler := fixture.Server.Handler()
	original := pdfBytes(t, handler, fixture.AvailableID)
	etag := pdfStatusForWork(t, handler, fixture.AvailableID)["mutation_etag"].(string)
	data := pdffixture.Bytes("Identity mismatch")
	digest := pdfDigest(data)
	invalidSignature := append([]byte(nil), data...)
	invalidSignature[0] = 'X'
	for _, test := range []struct {
		name   string
		body   []byte
		size   int64
		digest string
		want   string
	}{
		{name: "size", body: data, size: int64(len(data)) + 1, digest: digest, want: "pdf_upload_mismatch"},
		{name: "digest", body: data, size: int64(len(data)), digest: strings.Repeat("0", 64), want: "pdf_upload_mismatch"},
		{name: "truncated", body: data[:len(data)-16], size: int64(len(data)), digest: digest, want: "pdf_upload_mismatch"},
		{name: "signature", body: invalidSignature, size: int64(len(invalidSignature)), digest: pdfDigest(invalidSignature), want: "pdf_validation_error"},
	} {
		t.Run(test.name, func(t *testing.T) {
			code, body := pdfMutationRequest(t, handler, http.MethodPut, fixture.RunID, fixture.RevisionID, test.body, test.size, test.digest, etag)
			if code != http.StatusUnprocessableEntity || body["error"].(map[string]any)["code"] != test.want {
				t.Fatalf("response = %d %v", code, body)
			}
			if !bytes.Equal(pdfBytes(t, handler, fixture.AvailableID), original) {
				t.Fatal("rejected upload changed stored bytes")
			}
		})
	}
}

// TestPDFMutationRejectsInactiveRun verifies lifecycle policy without a review-context prerequisite.
func TestPDFMutationRejectsInactiveRun(t *testing.T) {
	fixture := serverhelpers.NewPDFViewerFixture(t)
	if _, err := fixture.Server.Metadata.Conn.Exec(`UPDATE pipeline_runs SET status='running' WHERE id=?`, fixture.RunID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = fixture.Server.Metadata.Conn.Exec(`UPDATE pipeline_runs SET status='completed' WHERE id=?`, fixture.RunID)
	})
	status := pdfStatusForWork(t, fixture.Server.Handler(), fixture.AvailableID)
	data := pdffixture.Bytes("Inactive run")
	code, body := pdfMutationRequest(t, fixture.Server.Handler(), http.MethodPut, fixture.RunID, fixture.RevisionID, data, int64(len(data)), pdfDigest(data), status["mutation_etag"].(string))
	if code != http.StatusConflict || body["error"].(map[string]any)["code"] != "run_not_mutable" {
		t.Fatalf("inactive run response = %d %v", code, body)
	}
}

// TestPDFMutationStatusReportsCapabilities verifies status and article detail
// expose mutation state without conflating it with review editability.
func TestPDFMutationStatusReportsCapabilities(t *testing.T) {
	fixture := serverhelpers.NewPDFViewerFixture(t)
	status := pdfStatusForWork(t, fixture.Server.Handler(), fixture.AvailableID)
	if status["registered"] != true || status["max_upload_bytes"] != float64(20_000_000) || status["storage_writable"] != true || status["storage_reason"] != "" {
		t.Fatalf("PDF status capabilities = %v", status)
	}
	if _, ok := status["mutation_etag"].(string); !ok {
		t.Fatalf("PDF status mutation ETag = %v", status["mutation_etag"])
	}
	code, article := serverhelpers.RequestJSON(t, fixture.Server.Handler(), fmt.Sprintf("/api/articles/%d?run_id=%d", fixture.RevisionID, fixture.RunID))
	if code != http.StatusOK {
		t.Fatalf("article status = %d", code)
	}
	actions := article["pdf_status"].(map[string]any)["actions"].(map[string]any)
	if actions["replace"] != true || actions["remove"] != true || actions["download"] != true {
		t.Fatalf("article PDF actions = %v", actions)
	}
	if article["review_context_initialized"] != false {
		t.Fatal("article fixture unexpectedly initialized review context")
	}
}

// TestPDFMutationResponseIsNoStore verifies status and mutation responses are not cacheable.
func TestPDFMutationResponseIsNoStore(t *testing.T) {
	fixture := serverhelpers.NewPDFViewerFixture(t)
	response := serverhelpers.ViewerRequest(t, fixture.Server.Handler(), fmt.Sprintf("/api/works/%d/pdf-status", fixture.AvailableID))
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status cache control = %q", response.Header().Get("Cache-Control"))
	}
}
