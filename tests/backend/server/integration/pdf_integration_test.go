// pdf_integration_test.go tests the PDF status and content HTTP endpoints with
// a temporary fixture-backed server.
//

package server_test

import "analysis/tests/support/go/testenv"

import testhelpers "analysis/tests/support/go/backend/server"
import "analysis/tests/support/go/pdffixture"

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"analysis/backend/pdfstore"
)

// TestPDFStatusEndpointsCoverEveryState verifies pdf status endpoints cover every state.
func TestPDFStatusEndpointsCoverEveryState(t *testing.T) {
	fixture := testhelpers.NewPDFViewerFixture(t)
	handler := fixture.Server.Handler()
	tests := []struct {
		id     int64
		status string
	}{
		{fixture.AvailableID, "available"},
		{fixture.NotAvailableID, "not_available"},
		{fixture.UnavailableID, "unavailable"},
	}
	for _, test := range tests {
		code, body := testhelpers.RequestJSON(t, handler, fmt.Sprintf("/api/works/%d/pdf-status", test.id))
		if code != http.StatusOK || body["status"] != test.status {
			t.Fatalf("work %d status response: code=%d body=%v", test.id, code, body)
		}
	}
	code, _ := testhelpers.RequestJSON(t, handler, "/api/works/999999/pdf-status")
	if code != http.StatusNotFound {
		t.Fatalf("missing work status code = %d", code)
	}
}

// TestPDFContentSupportsInlineRanges verifies pdf content supports inline ranges.
func TestPDFContentSupportsInlineRanges(t *testing.T) {
	fixture := testhelpers.NewPDFViewerFixture(t)
	handler := fixture.Server.Handler()
	path := fmt.Sprintf("/api/pdf/%d", fixture.AvailableID)
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
	code, _ := testhelpers.RequestJSON(t, handler, fmt.Sprintf("/api/pdf/%d", fixture.NotAvailableID))
	if code != http.StatusNotFound {
		t.Fatalf("failed document content code = %d", code)
	}
}

// TestArticleIncludesManualPDFAuditEvent verifies that article audit pagination includes manual PDF evidence.
func TestArticleIncludesManualPDFAuditEvent(t *testing.T) {
	fixture := testhelpers.NewPDFViewerFixture(t)
	handler := fixture.Server.Handler()
	code, article := testhelpers.RequestJSON(t, handler, fmt.Sprintf("/api/articles/%d?run_id=%d", fixture.RevisionID, fixture.RunID))
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
	code, audit := testhelpers.RequestJSON(t, handler, "/api/audit?category=pdf&run_id=1")
	if code != http.StatusOK || audit["summary"].(map[string]any)["total_events"].(float64) < 1 {
		t.Fatalf("PDF audit category response: code=%d body=%v", code, audit)
	}
}

// TestViewerPDFConnectionIsReadOnly verifies viewer pdf connection is read only.
func TestViewerPDFConnectionIsReadOnly(t *testing.T) {
	fixture := testhelpers.NewPDFViewerFixture(t)
	if _, err := fixture.Server.PDF.Exec(`INSERT INTO pdf_blobs
		(content_hash, byte_size, data, created_at) VALUES ('write-test', 1, x'00', '2026-01-01T00:00:00Z')`); err == nil {
		t.Fatal("viewer PDF connection accepted a write")
	}
}

// TestPDFContentRejectsSameSizeHashMismatch verifies cached PDF bytes are content-addressed.
func TestPDFContentRejectsSameSizeHashMismatch(t *testing.T) {
	fixture := testhelpers.NewPDFViewerFixture(t)
	if err := fixture.Server.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := pdfstore.Open(filepath.Join(filepath.Dir(fixture.MetadataPath), "corpus.pdf.db"), testenv.Path("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	var data []byte
	if err := store.DB.QueryRow(`SELECT b.data FROM pdf_documents d JOIN pdf_blobs b ON b.content_hash=d.content_hash WHERE d.doi='10.1000/viewer-available'`).Scan(&data); err != nil {
		store.Close()
		t.Fatal(err)
	}
	data[len(data)-1] ^= 1
	if _, err := store.DB.Exec(`DROP TRIGGER pdf_blobs_abort_update`); err != nil {
		store.Close()
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`UPDATE pdf_blobs SET data=? WHERE content_hash=(SELECT content_hash FROM pdf_documents WHERE doi='10.1000/viewer-available')`, data); err != nil {
		store.Close()
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`CREATE TRIGGER pdf_blobs_abort_update
		BEFORE UPDATE ON pdf_blobs
		BEGIN
			SELECT RAISE(ABORT, 'pdf_blobs is immutable: updates are not allowed');
		END`); err != nil {
		store.Close()
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	viewer, err := testhelpers.OpenViewer(t, fixture.MetadataPath)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	response := testhelpers.ViewerRequest(t, viewer.Handler(), fmt.Sprintf("/api/pdf/%d", fixture.AvailableID))
	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), "pdf_integrity_error") {
		t.Fatalf("corrupt PDF response: status=%d body=%s", response.Code, response.Body.String())
	}
}

// pdfContentRequest invokes one PDF evidence request with optional headers.
func pdfContentRequest(t *testing.T, handler http.Handler, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, nil)
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

// TestPDFContentHashPinPreservesReadSemantics verifies pinned GET, HEAD,
// range, conditional, and invalid-query behavior.
func TestPDFContentHashPinPreservesReadSemantics(t *testing.T) {
	fixture := testhelpers.NewPDFViewerFixture(t)
	handler := fixture.Server.Handler()
	status := pdfStatusForWork(t, handler, fixture.AvailableID)
	hash := status["content_hash"].(string)
	path := fmt.Sprintf("/api/pdf/%d?content_hash=%s", fixture.AvailableID, hash)
	full := pdfContentRequest(t, handler, http.MethodGet, path, nil)
	if full.Code != http.StatusOK || !bytes.HasPrefix(full.Body.Bytes(), []byte("%PDF-")) {
		t.Fatalf("pinned GET = %d %q", full.Code, full.Body.String())
	}
	if full.Header().Get("ETag") != `"`+hash+`"` {
		t.Fatalf("pinned ETag = %q", full.Header().Get("ETag"))
	}
	ranged := pdfContentRequest(t, handler, http.MethodGet, path, map[string]string{"Range": "bytes=0-4"})
	if ranged.Code != http.StatusPartialContent || ranged.Body.String() != "%PDF-" {
		t.Fatalf("pinned range = %d %q", ranged.Code, ranged.Body.String())
	}
	head := pdfContentRequest(t, handler, http.MethodHead, path, nil)
	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatalf("pinned HEAD = %d body=%d", head.Code, head.Body.Len())
	}
	conditional := pdfContentRequest(t, handler, http.MethodGet, path, map[string]string{"If-None-Match": `"` + hash + `"`})
	if conditional.Code != http.StatusNotModified {
		t.Fatalf("pinned conditional = %d", conditional.Code)
	}
	if invalid := pdfContentRequest(t, handler, http.MethodGet, fmt.Sprintf("/api/pdf/%d?content_hash=invalid", fixture.AvailableID), nil); invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid pin = %d", invalid.Code)
	}
	if unknown := pdfContentRequest(t, handler, http.MethodGet, fmt.Sprintf("/api/pdf/%d?unexpected=1", fixture.AvailableID), nil); unknown.Code != http.StatusBadRequest {
		t.Fatalf("unknown query = %d", unknown.Code)
	}
}

// TestPDFContentPinAndCacheObserveCommittedChange verifies replacement and
// removal invalidate cached bytes and reject stale pinned ranges.
func TestPDFContentPinAndCacheObserveCommittedChange(t *testing.T) {
	fixture := testhelpers.NewPDFViewerFixture(t)
	handler := fixture.Server.Handler()
	status := pdfStatusForWork(t, handler, fixture.AvailableID)
	oldHash := status["content_hash"].(string)
	oldPath := fmt.Sprintf("/api/pdf/%d?content_hash=%s", fixture.AvailableID, oldHash)
	if warm := pdfContentRequest(t, handler, http.MethodGet, oldPath, nil); warm.Code != http.StatusOK {
		t.Fatalf("cache warm = %d", warm.Code)
	}
	replacement := pdffixture.Bytes("Changed reader bytes")
	code, body := pdfMutationRequest(t, handler, http.MethodPut, fixture.RunID, fixture.RevisionID, replacement, int64(len(replacement)), pdfDigest(replacement), status["mutation_etag"].(string))
	if code != http.StatusOK {
		t.Fatalf("replacement = %d %v", code, body)
	}
	stale := pdfContentRequest(t, handler, http.MethodGet, oldPath, nil)
	if stale.Code != http.StatusConflict || !strings.Contains(stale.Body.String(), "pdf_changed") {
		t.Fatalf("stale pin after replace = %d %s", stale.Code, stale.Body.String())
	}
	unpinned := pdfContentRequest(t, handler, http.MethodGet, fmt.Sprintf("/api/pdf/%d", fixture.AvailableID), nil)
	if unpinned.Code != http.StatusOK || !bytes.Equal(unpinned.Body.Bytes(), replacement) {
		t.Fatalf("unpinned replacement read = %d bytes=%d", unpinned.Code, unpinned.Body.Len())
	}
	updated := body["pdf_status"].(map[string]any)
	newHash := updated["content_hash"].(string)
	newETag := updated["mutation_etag"].(string)
	newPath := fmt.Sprintf("/api/pdf/%d?content_hash=%s", fixture.AvailableID, newHash)
	if current := pdfContentRequest(t, handler, http.MethodGet, newPath, nil); current.Code != http.StatusOK || !bytes.Equal(current.Body.Bytes(), replacement) {
		t.Fatalf("new pin read = %d bytes=%d", current.Code, current.Body.Len())
	}
	code, removed := pdfMutationRequest(t, handler, http.MethodDelete, fixture.RunID, fixture.RevisionID, nil, 0, "", newETag)
	if code != http.StatusOK {
		t.Fatalf("removal = %d %v", code, removed)
	}
	removedPin := pdfContentRequest(t, handler, http.MethodGet, newPath, nil)
	if removedPin.Code != http.StatusConflict || !strings.Contains(removedPin.Body.String(), "pdf_changed") {
		t.Fatalf("stale pin after removal = %d %s", removedPin.Code, removedPin.Body.String())
	}
	absent := pdfContentRequest(t, handler, http.MethodGet, fmt.Sprintf("/api/pdf/%d", fixture.AvailableID), nil)
	if absent.Code != http.StatusNotFound {
		t.Fatalf("removed unpinned read = %d", absent.Code)
	}
}
