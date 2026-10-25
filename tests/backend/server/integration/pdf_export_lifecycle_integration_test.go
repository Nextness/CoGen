// pdf_export_lifecycle_integration_test.go verifies export bounds, admission,
// deadlines, read-only companion support, pinned snapshots, and stream aborts.
package server_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"analysis/backend/database/work"
	"analysis/backend/pdfstore"
	testhelpers "analysis/tests/support/go/backend/server"
	"analysis/tests/support/go/pdffixture"
	"analysis/tests/support/go/testenv"
	"analysis/tests/support/go/testfixture"
)

// bulkExportArticle identifies one seeded PDF selected for a bound check.
type bulkExportArticle struct {
	WorkID      int64
	RevisionID  int64
	ContentHash string
}

// seedBulkPDFExports adds count available selected-run articles with one
// shared large PDF body so aggregate byte limits can be exercised cheaply.
func seedBulkPDFExports(t *testing.T, viewer testhelpers.PdfViewerFixture, count int, size int) []bulkExportArticle {
	t.Helper()
	ctx := context.Background()
	store, err := pdfstore.Open(filepath.Join(filepath.Dir(viewer.MetadataPath), "corpus.pdf.db"), testenv.Path("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	data := append([]byte("%PDF-1.7\n"), bytes.Repeat([]byte("x"), size)...)
	items := make([]bulkExportArticle, 0, count)
	for index := 0; index < count; index++ {
		doi := fmt.Sprintf("10.1000/export-bulk-%02d", index)
		workID, err := viewer.Server.Metadata.Work.CreateWorkByDOI(ctx, doi)
		if err != nil {
			t.Fatal(err)
		}
		revisionID, err := viewer.Server.Metadata.Work.CreateRevision(ctx, &work.Revision{
			WorkID: workID, PipelineRunID: viewer.RunID, ProducerStage: work.ProducerStageNormalize,
			Title: fmt.Sprintf("Bulk export article %02d", index),
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := viewer.Server.Metadata.Work.SetStageOutcome(ctx, work.StageOutcomeInput{
			RunID: viewer.RunID, WorkID: workID, StageName: work.StageNameValidate, Outcome: work.OutcomeValid,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Register(ctx, doi, workID, viewer.RunID); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Save(ctx, pdfstore.SaveInput{DOI: doi, WorkID: workID, ExpectedChangeCounter: 1, Data: data}); err != nil {
			t.Fatal(err)
		}
		document, err := store.Document(ctx, doi)
		if err != nil || document == nil {
			t.Fatalf("read bulk export document: %v", err)
		}
		items = append(items, bulkExportArticle{WorkID: workID, RevisionID: revisionID, ContentHash: document.ContentHash})
	}
	return items
}

// TestPDFExportRejectsAggregateByteLimit verifies the 100 MB output cap before headers.
func TestPDFExportRejectsAggregateByteLimit(t *testing.T) {
	fixture := testhelpers.NewPDFViewerFixture(t)
	items := seedBulkPDFExports(t, fixture, 20, 5_000_000)
	articles := make([]map[string]any, 0, len(items))
	for _, item := range items {
		articles = append(articles, map[string]any{"work_revision_id": item.RevisionID, "content_hash": item.ContentHash})
	}
	recorder := pdfExportRequest(t, fixture.Server.Handler(), fmt.Sprintf("/api/runs/%d/pdf-exports", fixture.RunID), map[string]any{"articles": articles})
	if recorder.Code != http.StatusRequestEntityTooLarge || pdfExportErrorCode(t, recorder) != "pdf_export_limit" {
		t.Fatalf("aggregate byte limit status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Header().Get("Content-Type"), "zip") {
		t.Fatal("aggregate byte limit wrote ZIP headers")
	}
}

// TestPDFExportDeadlineWrapper verifies explicit export deadlines are finite.
func TestPDFExportDeadlineWrapper(t *testing.T) {
	fixture := testhelpers.NewPDFViewerFixture(t)
	status := pdfStatusForWork(t, fixture.Server.Handler(), fixture.AvailableID)
	writer := &deadlineResponseWriter{header: make(http.Header)}
	body := fmt.Sprintf(`{"content_hash":%q}`, status["content_hash"])
	request := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/runs/%d/articles/%d/pdf-export", fixture.RunID, fixture.RevisionID), strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	fixture.Server.Handler().ServeHTTP(writer, request)
	if writer.read.IsZero() || writer.write.IsZero() || writer.write.Sub(writer.read) <= 0 {
		t.Fatalf("export deadlines = read %v write %v", writer.read, writer.write)
	}
}

// TestPDFExportAdmissionBounds verifies two active responses and one ZIP slot.
func TestPDFExportAdmissionBounds(t *testing.T) {
	fixture := testhelpers.NewPDFViewerFixture(t)
	handler := fixture.Server.Handler()
	status := pdfStatusForWork(t, handler, fixture.AvailableID)
	singleBody := fmt.Sprintf(`{"content_hash":%q}`, status["content_hash"])
	zipBody := fmt.Sprintf(`{"articles":[{"work_revision_id":%d,"content_hash":%q}]}`, fixture.RevisionID, status["content_hash"])
	singleWriter := newBlockingExportWriter()
	zipWriter := newBlockingExportWriter()
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		request := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/runs/%d/articles/%d/pdf-export", fixture.RunID, fixture.RevisionID), strings.NewReader(singleBody))
		request.Header.Set("Content-Type", "application/json")
		handler.ServeHTTP(singleWriter, request)
	}()
	go func() {
		defer wait.Done()
		request := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/runs/%d/pdf-exports", fixture.RunID), strings.NewReader(zipBody))
		request.Header.Set("Content-Type", "application/json")
		handler.ServeHTTP(zipWriter, request)
	}()
	for name, writer := range map[string]*blockingExportWriter{"single": singleWriter, "zip": zipWriter} {
		select {
		case <-writer.started:
		case <-time.After(5 * time.Second):
			t.Fatalf("%s export did not reach response streaming", name)
		}
	}
	third := pdfExportRequest(t, handler, fmt.Sprintf("/api/runs/%d/articles/%d/pdf-export", fixture.RunID, fixture.RevisionID), map[string]any{"content_hash": status["content_hash"]})
	if third.Code != http.StatusServiceUnavailable || pdfExportErrorCode(t, third) != "pdf_export_busy" {
		t.Fatalf("admission bound status=%d body=%s", third.Code, third.Body.String())
	}
	close(singleWriter.release)
	close(zipWriter.release)
	done := make(chan struct{})
	go func() {
		wait.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("export admission test did not release")
	}
}

// blockingExportWriter blocks the first response body write until released.
type blockingExportWriter struct {
	header  http.Header
	status  int
	started chan struct{}
	release chan struct{}
	once    sync.Once
	body    bytes.Buffer
}

// newBlockingExportWriter creates one blocked response writer.
func newBlockingExportWriter() *blockingExportWriter {
	return &blockingExportWriter{header: make(http.Header), started: make(chan struct{}), release: make(chan struct{})}
}

// Header returns the response header map.
func (w *blockingExportWriter) Header() http.Header { return w.header }

// WriteHeader records the response status.
func (w *blockingExportWriter) WriteHeader(status int) { w.status = status }

// Write blocks the first body write and then records all output.
func (w *blockingExportWriter) Write(data []byte) (int, error) {
	w.once.Do(func() { close(w.started) })
	<-w.release
	return w.body.Write(data)
}

// SetReadDeadline satisfies the response controller interface.
func (w *blockingExportWriter) SetReadDeadline(time.Time) error { return nil }

// SetWriteDeadline satisfies the response controller interface.
func (w *blockingExportWriter) SetWriteDeadline(time.Time) error { return nil }

// TestPDFExportWorksWithReadOnlyCompanion verifies explicit export needs no write capability.
func TestPDFExportWorksWithReadOnlyCompanion(t *testing.T) {
	fixture := testhelpers.NewPDFViewerFixture(t)
	handler := fixture.Server.Handler()
	status := pdfStatusForWork(t, handler, fixture.AvailableID)
	want := pdffixture.Bytes("Selectable viewer methods on page one", "Selectable viewer results on page two")
	companion := filepath.Join(filepath.Dir(fixture.MetadataPath), "corpus.pdf.db")
	if err := fixture.Server.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(companion, 0o444); err != nil {
		t.Fatal(err)
	}
	viewer, err := testhelpers.OpenViewer(t, fixture.MetadataPath)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	if viewer.PDFStoreWritable() {
		t.Fatal("read-only companion was reported writable")
	}
	recorder := pdfExportRequest(t, viewer.Handler(), fmt.Sprintf("/api/runs/%d/articles/%d/pdf-export", fixture.RunID, fixture.RevisionID), map[string]any{"content_hash": status["content_hash"]})
	if recorder.Code != http.StatusOK || !bytes.Equal(recorder.Body.Bytes(), want) {
		t.Fatalf("read-only export status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

// TestPDFExportPinsSnapshotDuringReplacement verifies one archive cannot mix versions.
func TestPDFExportPinsSnapshotDuringReplacement(t *testing.T) {
	fixture := newPDFExportNameFixture(t)
	handler := fixture.Viewer.Server.Handler()
	firstHash := exportHash(t, fixture.Viewer, fixture.UnicodeID)
	secondHash := exportHash(t, fixture.Viewer, fixture.CollideAID)
	firstBytes := pdfBytes(t, handler, fixture.UnicodeID)
	secondBytes := pdfBytes(t, handler, fixture.CollideAID)
	writer := newBlockingExportWriter()
	body := fmt.Sprintf(`{"articles":[{"work_revision_id":%d,"content_hash":%q},{"work_revision_id":%d,"content_hash":%q}]}`,
		fixture.UnicodeRev, firstHash, fixture.CollideARev, secondHash)
	done := make(chan struct{})
	go func() {
		defer close(done)
		request := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/runs/%d/pdf-exports", fixture.Viewer.RunID), strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		handler.ServeHTTP(writer, request)
	}()
	select {
	case <-writer.started:
	case <-time.After(5 * time.Second):
		t.Fatal("pinned export did not reach response streaming")
	}
	store, err := pdfstore.Open(filepath.Join(filepath.Dir(fixture.Viewer.MetadataPath), "corpus.pdf.db"), testenv.Path("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	replacement := pdffixture.Bytes("Replacement during export")
	if _, err := store.Save(context.Background(), pdfstore.SaveInput{
		DOI: "10.1000/export-collide-a", WorkID: fixture.CollideAID, ExpectedChangeCounter: 2, Data: replacement,
	}); err != nil {
		store.Close()
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	close(writer.release)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("pinned export did not finish")
	}
	files, manifest := pdfExportZIPBytes(t, writer.body.Bytes())
	if manifest["included"] != float64(2) {
		t.Fatalf("pinned manifest=%v", manifest)
	}
	for _, raw := range manifest["files"].([]any) {
		item := raw.(map[string]any)
		filename := item["filename"].(string)
		switch int64(item["work_revision_id"].(float64)) {
		case fixture.UnicodeRev:
			if !bytes.Equal(files[filename], firstBytes) {
				t.Fatal("first pinned export changed")
			}
		case fixture.CollideARev:
			if !bytes.Equal(files[filename], secondBytes) {
				t.Fatal("second pinned export mixed replacement bytes")
			}
		}
	}
	changed := pdfExportRequest(t, handler, fmt.Sprintf("/api/runs/%d/articles/%d/pdf-export", fixture.Viewer.RunID, fixture.CollideARev), map[string]any{"content_hash": secondHash})
	if changed.Code != http.StatusConflict || pdfExportErrorCode(t, changed) != "pdf_changed" {
		t.Fatalf("stale post-export hash status=%d body=%s", changed.Code, changed.Body.String())
	}
	newHash := pdfDigest(replacement)
	current := pdfExportRequest(t, handler, fmt.Sprintf("/api/runs/%d/articles/%d/pdf-export", fixture.Viewer.RunID, fixture.CollideARev), map[string]any{"content_hash": newHash})
	if current.Code != http.StatusOK || !bytes.Equal(current.Body.Bytes(), replacement) {
		t.Fatalf("replacement export status=%d", current.Code)
	}
}

// failingExportWriter fails every response body write.
type failingExportWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
}

// Header returns the response header map.
func (w *failingExportWriter) Header() http.Header { return w.header }

// WriteHeader records the response status.
func (w *failingExportWriter) WriteHeader(status int) { w.status = status }

// Write records the prefix and returns a transport failure.
func (w *failingExportWriter) Write(data []byte) (int, error) {
	_, _ = w.body.Write(data)
	return 0, fmt.Errorf("fixture stream interruption")
}

// SetReadDeadline satisfies the response controller interface.
func (w *failingExportWriter) SetReadDeadline(time.Time) error { return nil }

// SetWriteDeadline satisfies the response controller interface.
func (w *failingExportWriter) SetWriteDeadline(time.Time) error { return nil }

// TestPDFExportStreamInterruptionAborts verifies failed emission never appends JSON.
func TestPDFExportStreamInterruptionAborts(t *testing.T) {
	fixture := testhelpers.NewPDFViewerFixture(t)
	status := pdfStatusForWork(t, fixture.Server.Handler(), fixture.AvailableID)
	writer := &failingExportWriter{header: make(http.Header)}
	body := fmt.Sprintf(`{"articles":[{"work_revision_id":%d,"content_hash":%q}]}`, fixture.RevisionID, status["content_hash"])
	request := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/runs/%d/pdf-exports", fixture.RunID), strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	func() {
		defer func() {
			recovered := recover()
			if recovered != http.ErrAbortHandler {
				t.Fatalf("stream interruption panic = %#v", recovered)
			}
		}()
		fixture.Server.Handler().ServeHTTP(writer, request)
	}()
	if bytes.Contains(writer.body.Bytes(), []byte(`"error"`)) || bytes.Contains(writer.body.Bytes(), []byte("unable to read workspace data")) {
		t.Fatalf("stream interruption appended JSON: %s", writer.body.String())
	}
}

// TestPDFExportRejectsCorruptContentAndKeepsPartialArchive verifies stored
// identity failures never become attachments while valid ZIP entries remain.
func TestPDFExportRejectsCorruptContentAndKeepsPartialArchive(t *testing.T) {
	fixture := newPDFExportNameFixture(t)
	handler := fixture.Viewer.Server.Handler()
	firstHash := exportHash(t, fixture.Viewer, fixture.UnicodeID)
	secondHash := exportHash(t, fixture.Viewer, fixture.CollideAID)
	firstBytes := pdfBytes(t, handler, fixture.UnicodeID)
	secondBytes := pdfBytes(t, handler, fixture.CollideAID)
	corrupt := append([]byte(nil), secondBytes...)
	corrupt[len(corrupt)-1] ^= 1
	conn, err := testfixture.ConnectExisting(filepath.Join(filepath.Dir(fixture.Viewer.MetadataPath), "corpus.pdf.db"), "sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Conn.Exec("DROP TRIGGER pdf_blobs_abort_update"); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Conn.Exec("UPDATE pdf_blobs SET data=? WHERE content_hash=?", corrupt, secondHash); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Conn.Exec(`CREATE TRIGGER pdf_blobs_abort_update
		BEFORE UPDATE ON pdf_blobs
		BEGIN
			SELECT RAISE(ABORT, 'pdf_blobs is immutable: updates are not allowed');
		END`); err != nil {
		t.Fatal(err)
	}

	single := pdfExportRequest(t, handler, fmt.Sprintf("/api/runs/%d/articles/%d/pdf-export", fixture.Viewer.RunID, fixture.CollideARev), map[string]any{"content_hash": secondHash})
	if single.Code != http.StatusUnprocessableEntity || pdfExportErrorCode(t, single) != "pdf_integrity_error" {
		t.Fatalf("corrupt single export status=%d body=%s", single.Code, single.Body.String())
	}
	if strings.HasPrefix(single.Header().Get("Content-Type"), "application/pdf") {
		t.Fatal("corrupt single export wrote PDF headers")
	}
	partial := pdfExportRequest(t, handler, fmt.Sprintf("/api/runs/%d/pdf-exports", fixture.Viewer.RunID), map[string]any{
		"articles": []map[string]any{
			{"work_revision_id": fixture.UnicodeRev, "content_hash": firstHash},
			{"work_revision_id": fixture.CollideARev, "content_hash": secondHash},
		},
	})
	if partial.Code != http.StatusOK {
		t.Fatalf("partial corrupt export status=%d body=%s", partial.Code, partial.Body.String())
	}
	files, manifest := pdfExportZIP(t, partial)
	if manifest["included"] != float64(1) || manifest["skipped"] != float64(1) {
		t.Fatalf("partial corrupt manifest=%v", manifest)
	}
	skipped := manifest["files"].([]any)[1].(map[string]any)
	if skipped["reason"] != "pdf_integrity_error" {
		t.Fatalf("partial corrupt reason=%v", skipped)
	}
	for name, data := range files {
		if name == "export-manifest.json" {
			continue
		}
		if !bytes.Equal(data, firstBytes) {
			t.Fatalf("partial corrupt archive entry %q changed", name)
		}
	}
}
