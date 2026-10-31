// pdf_export_integration_test.go verifies explicit single and selected-set PDF
// exports against temporary production-migrated fixtures.
package server_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"analysis/backend/database/work"
	"analysis/backend/pdfstore"
	testhelpers "analysis/tests/support/go/backend/server"
	"analysis/tests/support/go/pdffixture"
	"analysis/tests/support/go/testenv"
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
	return pdfExportZIPBytes(t, recorder.Body.Bytes())
}

// pdfExportZIPBytes decodes one ZIP body and its manifest.
func pdfExportZIPBytes(t *testing.T, data []byte) (map[string][]byte, map[string]any) {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
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
	if !strings.Contains(recorder.Header().Get("Content-Disposition"), "Viewer PDF article.pdf") {
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
	if !bytes.Equal(files[fmt.Sprintf("Viewer PDF article (work-%d).pdf", fixture.AvailableID)], pdffixture.Bytes("Selectable viewer methods on page one", "Selectable viewer results on page two")) {
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
	if comments.Code != http.StatusOK || comments.Header().Get("X-PDF-Export-Comments-Status") != "no_review_context" {
		t.Fatalf("comments export status=%d headers=%v body=%s", comments.Code, comments.Header(), comments.Body.String())
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

// pdfExportNameFixture adds Unicode, colliding, and missing-title exports to
// one completed viewer run.
type pdfExportNameFixture struct {
	Viewer      testhelpers.PdfViewerFixture
	UnicodeID   int64
	CollideAID  int64
	CollideBID  int64
	MissingID   int64
	UnicodeRev  int64
	CollideARev int64
	CollideBRev int64
	MissingRev  int64
}

// newPDFExportNameFixture seeds selected-run articles with export naming edge cases.
func newPDFExportNameFixture(t *testing.T) pdfExportNameFixture {
	t.Helper()
	viewer := testhelpers.NewPDFViewerFixture(t)
	ctx := context.Background()
	create := func(doi, title string) (int64, int64) {
		t.Helper()
		workID, err := viewer.Server.Metadata.Work.CreateWorkByDOI(ctx, doi)
		if err != nil {
			t.Fatal(err)
		}
		revisionID, err := viewer.Server.Metadata.Work.CreateRevision(ctx, &work.Revision{
			WorkID: workID, PipelineRunID: viewer.RunID, ProducerStage: work.ProducerStageNormalize, Title: title,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := viewer.Server.Metadata.Work.SetStageOutcome(ctx, work.StageOutcomeInput{
			RunID: viewer.RunID, WorkID: workID, StageName: work.StageNameValidate, Outcome: work.OutcomeValid,
		}); err != nil {
			t.Fatal(err)
		}
		return workID, revisionID
	}
	unicodeID, unicodeRev := create("10.1000/export-unicode", "A study: paths / Unicode α?")
	collideAID, collideARev := create("10.1000/export-collide-a", "Same title")
	collideBID, collideBRev := create("10.1000/export-collide-b", "Same title")
	missingID, missingRev := create("10.1000/export-missing-title", "")
	store, err := pdfstore.Open(filepath.Join(filepath.Dir(viewer.MetadataPath), "corpus.pdf.db"), testenv.Path("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for index, item := range []struct {
		doi    string
		workID int64
	}{
		{"10.1000/export-unicode", unicodeID},
		{"10.1000/export-collide-a", collideAID},
		{"10.1000/export-collide-b", collideBID},
		{"10.1000/export-missing-title", missingID},
	} {
		if _, err := store.Register(ctx, item.doi, item.workID, viewer.RunID); err != nil {
			t.Fatal(err)
		}
		data := pdffixture.Bytes(fmt.Sprintf("Export naming fixture %d", index))
		if _, err := store.Save(ctx, pdfstore.SaveInput{DOI: item.doi, WorkID: item.workID, ExpectedChangeCounter: 1, Data: data}); err != nil {
			t.Fatal(err)
		}
	}
	return pdfExportNameFixture{
		Viewer: viewer, UnicodeID: unicodeID, CollideAID: collideAID, CollideBID: collideBID, MissingID: missingID,
		UnicodeRev: unicodeRev, CollideARev: collideARev, CollideBRev: collideBRev, MissingRev: missingRev,
	}
}

// exportHash reads one current source hash through the viewer status resource.
func exportHash(t *testing.T, fixture testhelpers.PdfViewerFixture, workID int64) string {
	t.Helper()
	return pdfStatusForWork(t, fixture.Server.Handler(), workID)["content_hash"].(string)
}

// TestPDFExportUsesSafeNamesAndRequiredAudit verifies Unicode bounds, collision
// suffixes, missing-title fallback, one ZIP correlation, and metadata-only audit.
func TestPDFExportUsesSafeNamesAndRequiredAudit(t *testing.T) {
	fixture := newPDFExportNameFixture(t)
	handler := fixture.Viewer.Server.Handler()
	body := map[string]any{"articles": []map[string]any{
		{"work_revision_id": fixture.UnicodeRev, "content_hash": exportHash(t, fixture.Viewer, fixture.UnicodeID)},
		{"work_revision_id": fixture.CollideARev, "content_hash": exportHash(t, fixture.Viewer, fixture.CollideAID)},
		{"work_revision_id": fixture.CollideBRev, "content_hash": exportHash(t, fixture.Viewer, fixture.CollideBID)},
		{"work_revision_id": fixture.MissingRev, "content_hash": exportHash(t, fixture.Viewer, fixture.MissingID)},
	}}
	recorder := pdfExportRequest(t, handler, fmt.Sprintf("/api/runs/%d/pdf-exports", fixture.Viewer.RunID), body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("named ZIP status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	files, manifest := pdfExportZIP(t, recorder)
	wantUnicode := fmt.Sprintf("A study- paths - Unicode α- (work-%d).pdf", fixture.UnicodeID)
	wantCollideA := fmt.Sprintf("Same title (work-%d).pdf", fixture.CollideAID)
	wantCollideB := fmt.Sprintf("Same title (work-%d).pdf", fixture.CollideBID)
	wantMissing := fmt.Sprintf("work-%d.pdf", fixture.MissingID)
	for _, name := range []string{wantUnicode, wantCollideA, wantCollideB, wantMissing, "export-manifest.json"} {
		if _, ok := files[name]; !ok {
			t.Fatalf("ZIP missing %q in %v", name, files)
		}
		if len(name) > 240 {
			t.Fatalf("ZIP entry name exceeds 240 bytes: %q", name)
		}
	}
	if manifest["included"] != float64(4) || manifest["skipped"] != float64(0) {
		t.Fatalf("manifest=%v", manifest)
	}
	var count, correlations, nullRuns int
	if err := fixture.Viewer.Server.Metadata.Conn.QueryRow(`SELECT COUNT(*), COUNT(DISTINCT correlation_id), SUM(pipeline_run_id IS NULL)
		FROM audit_events WHERE action='pdf_document_export_prepared'`).Scan(&count, &correlations, &nullRuns); err != nil {
		t.Fatal(err)
	}
	if count != 4 || correlations != 1 || nullRuns != 4 {
		t.Fatalf("export audit count=%d correlations=%d null_runs=%d", count, correlations, nullRuns)
	}
	var outbox int
	if err := fixture.Viewer.Server.PDF.QueryRow(`SELECT COUNT(*) FROM pdf_audit_outbox WHERE action='pdf_document_export_prepared'`).Scan(&outbox); err != nil {
		t.Fatal(err)
	}
	if outbox != 0 {
		t.Fatalf("export wrote %d PDF outbox events", outbox)
	}
}

// TestPDFExportRequiredAuditFailureWritesNoBinary verifies audit failure happens before headers.
func TestPDFExportRequiredAuditFailureWritesNoBinary(t *testing.T) {
	fixture := testhelpers.NewPDFViewerFixture(t)
	handler := fixture.Server.Handler()
	if _, err := fixture.Server.Metadata.Conn.Exec(`CREATE TRIGGER fail_pdf_export_audit
		BEFORE INSERT ON audit_events
		WHEN NEW.action='pdf_document_export_prepared'
		BEGIN
			SELECT RAISE(ABORT, 'fixture export audit failure');
		END`); err != nil {
		t.Fatal(err)
	}
	defer fixture.Server.Metadata.Conn.Exec(`DROP TRIGGER fail_pdf_export_audit`)
	status := pdfStatusForWork(t, handler, fixture.AvailableID)
	recorder := pdfExportRequest(t, handler, fmt.Sprintf("/api/runs/%d/articles/%d/pdf-export", fixture.RunID, fixture.RevisionID), map[string]any{"content_hash": status["content_hash"]})
	if recorder.Code != http.StatusInternalServerError || pdfExportErrorCode(t, recorder) != "pdf_audit_error" {
		t.Fatalf("audit failure status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if strings.HasPrefix(recorder.Header().Get("Content-Type"), "application/pdf") || strings.HasPrefix(recorder.Body.String(), "%PDF-") {
		t.Fatal("audit failure wrote a PDF response")
	}
}
