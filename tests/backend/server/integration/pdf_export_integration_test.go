// pdf_export_integration_test.go verifies explicit single and selected-set PDF
// exports against temporary production-migrated fixtures.
package server_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	testhelpers "analysis/tests/support/go/backend/server"
	"analysis/tests/support/go/pdffixture"
)

// pdfExportRequest invokes one explicit export with a JSON selection.
func pdfExportRequest(t *testing.T, handler http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(encoded))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

// pdfExportErrorCode decodes one structured error code.
func pdfExportErrorCode(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode export error: %v\n%s", err, recorder.Body.String())
	}
	return body["error"].(map[string]any)["code"].(string)
}

// pdfExportZIP decodes one ZIP response and its manifest.
func pdfExportZIP(t *testing.T, recorder *httptest.ResponseRecorder) (map[string][]byte, map[string]any) {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(recorder.Body.Bytes()), int64(recorder.Body.Len()))
	if err != nil {
		t.Fatalf("open export ZIP: %v", err)
	}
	files := make(map[string][]byte)
	var manifest map[string]any
	for _, file := range reader.File {
		source, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(source)
		source.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[file.Name] = data
		if file.Name == "export-manifest.json" {
			if err := json.Unmarshal(data, &manifest); err != nil {
				t.Fatalf("decode export manifest: %v", err)
			}
		}
	}
	return files, manifest
}

// TestPDFSingleExportReturnsOriginalAttachment verifies exact bytes and headers.
func TestPDFSingleExportReturnsOriginalAttachment(t *testing.T) {
	fixture := testhelpers.NewPDFViewerFixture(t)
	handler := fixture.Server.Handler()
	status := pdfStatusForWork(t, handler, fixture.AvailableID)
	path := fmt.Sprintf("/api/runs/%d/articles/%d/pdf-export", fixture.RunID, fixture.RevisionID)
	recorder := pdfExportRequest(t, handler, path, map[string]any{"content_hash": status["content_hash"], "include_comments": false})
	if recorder.Code != http.StatusOK {
		t.Fatalf("single export status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if !bytes.Equal(recorder.Body.Bytes(), pdffixture.Bytes("Selectable viewer methods on page one", "Selectable viewer results on page two")) {
		t.Fatal("single export bytes differ from source")
	}
	if recorder.Header().Get("Content-Type") != "application/pdf" || !strings.HasPrefix(recorder.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("single export headers: %v", recorder.Header())
	}
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("single export cache control=%q", recorder.Header().Get("Cache-Control"))
	}
	after := pdfStatusForWork(t, handler, fixture.AvailableID)
	if after["content_hash"] != status["content_hash"] || after["change_counter"] != status["change_counter"] {
		t.Fatal("single export changed PDF selection")
	}
}

// TestPDFZIPExportReportsPartialResults verifies included and skipped entries.
func TestPDFZIPExportReportsPartialResults(t *testing.T) {
	fixture := testhelpers.NewPDFViewerFixture(t)
	handler := fixture.Server.Handler()
	status := pdfStatusForWork(t, handler, fixture.AvailableID)
	body := map[string]any{
		"articles": []map[string]any{
			{"work_revision_id": fixture.RevisionID, "content_hash": status["content_hash"]},
			{"work_revision_id": fixture.NotAvailableRevisionID, "content_hash": strings.Repeat("0", 64)},
			{"work_revision_id": fixture.RevisionID, "content_hash": status["content_hash"]},
		},
		"include_comments": false,
	}
	recorder := pdfExportRequest(t, handler, fmt.Sprintf("/api/runs/%d/pdf-exports", fixture.RunID), body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("ZIP export status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	files, manifest := pdfExportZIP(t, recorder)
	if len(files) != 2 {
		t.Fatalf("ZIP entries=%v", files)
	}
	if !bytes.Equal(files[fmt.Sprintf("work-%d.pdf", fixture.AvailableID)], pdffixture.Bytes("Selectable viewer methods on page one", "Selectable viewer results on page two")) {
		t.Fatal("ZIP PDF bytes differ from source")
	}
	if manifest["requested"] != float64(2) || manifest["included"] != float64(1) || manifest["skipped"] != float64(1) {
		t.Fatalf("manifest counts=%v", manifest)
	}
	if recorder.Header().Get("X-PDF-Export-Included") != "1" || recorder.Header().Get("X-PDF-Export-Skipped") != "1" {
		t.Fatalf("export headers: %v", recorder.Header())
	}
}

// TestPDFExportRejectsInvalidSelections verifies bounded and all-failed requests.
func TestPDFExportRejectsInvalidSelections(t *testing.T) {
	fixture := testhelpers.NewPDFViewerFixture(t)
	handler := fixture.Server.Handler()
	status := pdfStatusForWork(t, handler, fixture.AvailableID)
	singlePath := fmt.Sprintf("/api/runs/%d/articles/%d/pdf-export", fixture.RunID, fixture.RevisionID)
	changed := pdfExportRequest(t, handler, singlePath, map[string]any{"content_hash": strings.Repeat("0", 64)})
	if changed.Code != http.StatusConflict || pdfExportErrorCode(t, changed) != "pdf_changed" {
		t.Fatalf("changed single export status=%d body=%s", changed.Code, changed.Body.String())
	}
	comments := pdfExportRequest(t, handler, singlePath, map[string]any{"content_hash": status["content_hash"], "include_comments": true})
	if comments.Code != http.StatusBadRequest || pdfExportErrorCode(t, comments) != "pdf_comments_unavailable" {
		t.Fatalf("comments export status=%d body=%s", comments.Code, comments.Body.String())
	}
	outside := pdfExportRequest(t, handler, fmt.Sprintf("/api/runs/%d/articles/999999/pdf-export", fixture.RunID), map[string]any{"content_hash": status["content_hash"]})
	if outside.Code != http.StatusNotFound {
		t.Fatalf("outside-run export status=%d body=%s", outside.Code, outside.Body.String())
	}
	tooMany := make([]map[string]any, 0, 21)
	for id := 1; id <= 21; id++ {
		tooMany = append(tooMany, map[string]any{"work_revision_id": id, "content_hash": strings.Repeat("0", 64)})
	}
	limited := pdfExportRequest(t, handler, fmt.Sprintf("/api/runs/%d/pdf-exports", fixture.RunID), map[string]any{"articles": tooMany})
	if limited.Code != http.StatusRequestEntityTooLarge || pdfExportErrorCode(t, limited) != "pdf_export_limit" {
		t.Fatalf("limited export status=%d body=%s", limited.Code, limited.Body.String())
	}
	allFailed := pdfExportRequest(t, handler, fmt.Sprintf("/api/runs/%d/pdf-exports", fixture.RunID), map[string]any{
		"articles": []map[string]any{
			{"work_revision_id": fixture.RevisionID, "content_hash": strings.Repeat("0", 64)},
			{"work_revision_id": fixture.NotAvailableRevisionID, "content_hash": strings.Repeat("0", 64)},
		},
	})
	if allFailed.Code != http.StatusConflict || pdfExportErrorCode(t, allFailed) != "pdf_export_empty" {
		t.Fatalf("all-failed export status=%d body=%s", allFailed.Code, allFailed.Body.String())
	}
}
