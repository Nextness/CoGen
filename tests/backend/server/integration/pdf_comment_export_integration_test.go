// pdf_comment_export_integration_test.go verifies comment-enabled PDF exports
// against temporary production-migrated fixtures and the real annotation worker.

package server_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"

	"analysis/backend/database/review"
	"analysis/backend/pdfannotate"
	testhelpers "analysis/tests/support/go/backend/server"
)

// newPDFCommentFixture creates one completed review context with a matching
// anchor and saved note, and points the viewer at the real annotation worker.
func newPDFCommentFixture(t *testing.T) (testhelpers.PdfViewerFixture, int64, *review.Anchor, *review.Note) {
	t.Helper()
	fixture := testhelpers.NewPDFViewerFixture(t)
	fixture.Server.SetPDFAnnotationWorker(pdfannotate.NewWorkerAt(testhelpers.BuildAnalysisWorker(t)))
	ctx := context.Background()
	contextRecord, _, err := fixture.Server.Metadata.Review.CreateContext(ctx, fixture.RunID, nil)
	if err != nil {
		t.Fatal(err)
	}
	hash := exportHash(t, fixture, fixture.AvailableID)
	anchor, err := fixture.Server.Metadata.Review.CreateAnchor(ctx, contextRecord.ID, fixture.RevisionID, "methods-1", hash, 1, "Selected methods text", []review.AnchorRectangle{{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.1}})
	if err != nil {
		t.Fatal(err)
	}
	note, err := fixture.Server.Metadata.Review.CreateNote(ctx, contextRecord.ID, fixture.RevisionID, "Complete saved note body [[anchor:"+anchor.ID+"|methods]].")
	if err != nil {
		t.Fatal(err)
	}
	return fixture, contextRecord.ID, anchor, note
}

// pdfAnnotationContents returns annotation contents and titles from one PDF.
func pdfAnnotationContents(t *testing.T, data []byte) ([]string, []string) {
	t.Helper()
	ctx, err := api.ReadAndValidate(bytes.NewReader(data), model.NewDefaultConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	var contents, titles []string
	for page := 1; page <= ctx.PageCount; page++ {
		dict, _, _, err := ctx.PageDict(page, false)
		if err != nil {
			t.Fatal(err)
		}
		object, found := dict.Find("Annots")
		if !found {
			continue
		}
		array, err := ctx.DereferenceArray(object)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range array {
			annotation, err := ctx.DereferenceDict(item)
			if err != nil {
				t.Fatal(err)
			}
			if value, found := annotation.Find("Contents"); found {
				text, err := ctx.DereferenceStringOrHexLiteral(value, model.V14, nil)
				if err != nil {
					t.Fatal(err)
				}
				contents = append(contents, text)
			}
			if value, found := annotation.Find("T"); found {
				text, err := ctx.DereferenceStringOrHexLiteral(value, model.V14, nil)
				if err != nil {
					t.Fatal(err)
				}
				titles = append(titles, text)
			}
		}
	}
	return contents, titles
}

// TestPDFCommentExportEmbedsMatchingSavedEvidence verifies derived output,
// headers, audit provenance, and unchanged source/review state.
func TestPDFCommentExportEmbedsMatchingSavedEvidence(t *testing.T) {
	fixture, contextID, anchor, note := newPDFCommentFixture(t)
	handler := fixture.Server.Handler()
	status := pdfStatusForWork(t, handler, fixture.AvailableID)
	sourceHash := status["content_hash"].(string)
	path := fmt.Sprintf("/api/runs/%d/articles/%d/pdf-export", fixture.RunID, fixture.RevisionID)
	recorder := pdfExportRequest(t, handler, path, map[string]any{"content_hash": sourceHash, "include_comments": true})
	if recorder.Code != http.StatusOK {
		t.Fatalf("comment export status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.HasPrefix(recorder.Header().Get("Content-Type"), "application/pdf") {
		t.Fatalf("content type=%q", recorder.Header().Get("Content-Type"))
	}
	if recorder.Header().Get("X-PDF-Export-Comments") != "1" || recorder.Header().Get("X-PDF-Export-Comments-Status") != "included" {
		t.Fatalf("comment headers=%v", recorder.Header())
	}
	if !strings.Contains(recorder.Header().Get("Content-Disposition"), "(annotated)") {
		t.Fatalf("annotated filename=%q", recorder.Header().Get("Content-Disposition"))
	}
	outputHash := recorder.Header().Get("X-PDF-Export-Output-Hash")
	digest := sha256.Sum256(recorder.Body.Bytes())
	if outputHash == sourceHash || outputHash != hex.EncodeToString(digest[:]) {
		t.Fatalf("output hash=%q source=%q actual=%q", outputHash, sourceHash, hex.EncodeToString(digest[:]))
	}
	contents, titles := pdfAnnotationContents(t, recorder.Body.Bytes())
	if len(contents) != 1 {
		t.Fatalf("annotation contents=%v", contents)
	}
	if !strings.Contains(contents[0], "Complete saved note body methods.") {
		t.Fatalf("note prose=%q", contents[0])
	}
	if len(titles) != 1 || titles[0] != "Fixture Reviewer" || strings.Contains(titles[0], "fixture@example.test") {
		t.Fatalf("annotation titles=%q", titles)
	}
	after := pdfStatusForWork(t, handler, fixture.AvailableID)
	if after["content_hash"] != sourceHash || after["change_counter"] != status["change_counter"] {
		t.Fatal("comment export changed the PDF selection")
	}
	currentAnchor, err := fixture.Server.Metadata.Review.GetAnchor(context.Background(), contextID, anchor.ID)
	if err != nil || currentAnchor.Version.ID != anchor.Version.ID {
		t.Fatalf("anchor head changed: %+v err=%v", currentAnchor, err)
	}
	currentNote, err := fixture.Server.Metadata.Review.GetNote(context.Background(), contextID, note.ID)
	if err != nil || currentNote.Version.ID != note.Version.ID {
		t.Fatalf("note head changed: %+v err=%v", currentNote, err)
	}
	var afterJSON, metadataJSON string
	if err := fixture.Server.Metadata.Conn.QueryRow(`SELECT after_json, metadata_json FROM audit_events WHERE action='pdf_document_export_prepared' ORDER BY id DESC LIMIT 1`).Scan(&afterJSON, &metadataJSON); err != nil {
		t.Fatal(err)
	}
	var evidence map[string]any
	if err := json.Unmarshal([]byte(afterJSON), &evidence); err != nil {
		t.Fatal(err)
	}
	if evidence["include_comments"] != true || evidence["comments_status"] != "included" || evidence["comment_count"] != float64(1) {
		t.Fatalf("audit evidence=%v", evidence)
	}
	if evidence["output_content_hash"] != outputHash || evidence["content_hash"] != sourceHash {
		t.Fatalf("audit hashes=%v", evidence)
	}
	if evidence["review_context_id"] != float64(contextID) {
		t.Fatalf("audit context=%v", evidence["review_context_id"])
	}
	if !strings.Contains(afterJSON, fmt.Sprint(note.Version.ID)) || !strings.Contains(afterJSON, fmt.Sprint(anchor.Version.ID)) {
		t.Fatalf("audit missing immutable ids: %s", afterJSON)
	}
	for _, forbidden := range []string{"Complete saved note body", "Selected methods text", "fixture@example.test"} {
		if strings.Contains(afterJSON, forbidden) || strings.Contains(metadataJSON, forbidden) {
			t.Fatalf("audit leaked %q: %s %s", forbidden, afterJSON, metadataJSON)
		}
	}
}

// TestPDFCommentExportOffReturnsOriginalBytes verifies the default path stays byte-identical.
func TestPDFCommentExportOffReturnsOriginalBytes(t *testing.T) {
	fixture, _, _, _ := newPDFCommentFixture(t)
	handler := fixture.Server.Handler()
	status := pdfStatusForWork(t, handler, fixture.AvailableID)
	path := fmt.Sprintf("/api/runs/%d/articles/%d/pdf-export", fixture.RunID, fixture.RevisionID)
	recorder := pdfExportRequest(t, handler, path, map[string]any{"content_hash": status["content_hash"], "include_comments": false})
	if recorder.Code != http.StatusOK {
		t.Fatalf("plain export status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	original := pdfBytes(t, handler, fixture.AvailableID)
	if !bytes.Equal(recorder.Body.Bytes(), original) {
		t.Fatal("plain export bytes differ from stored source")
	}
	if recorder.Header().Get("X-PDF-Export-Comments-Status") != "off" || recorder.Header().Get("X-PDF-Export-Output-Hash") != status["content_hash"] {
		t.Fatalf("plain export headers=%v", recorder.Header())
	}
	var metadataJSON string
	if err := fixture.Server.Metadata.Conn.QueryRow(`SELECT metadata_json FROM audit_events WHERE action='pdf_document_export_prepared' ORDER BY id DESC LIMIT 1`).Scan(&metadataJSON); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(metadataJSON, `"include_comments":false`) || strings.Contains(metadataJSON, `"review_context_id"`) {
		t.Fatalf("plain export audit=%s", metadataJSON)
	}
}

// TestPDFCommentExportWithoutContextReturnsOriginal verifies export never initializes review.
func TestPDFCommentExportWithoutContextReturnsOriginal(t *testing.T) {
	fixture := testhelpers.NewPDFViewerFixture(t)
	handler := fixture.Server.Handler()
	status := pdfStatusForWork(t, handler, fixture.AvailableID)
	path := fmt.Sprintf("/api/runs/%d/articles/%d/pdf-export", fixture.RunID, fixture.RevisionID)
	recorder := pdfExportRequest(t, handler, path, map[string]any{"content_hash": status["content_hash"], "include_comments": true})
	if recorder.Code != http.StatusOK {
		t.Fatalf("no-context export status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("X-PDF-Export-Comments-Status") != "no_review_context" || recorder.Header().Get("X-PDF-Export-Comments") != "0" {
		t.Fatalf("no-context headers=%v", recorder.Header())
	}
	if !bytes.Equal(recorder.Body.Bytes(), pdfBytes(t, handler, fixture.AvailableID)) {
		t.Fatal("no-context export changed source bytes")
	}
	contextRecord, err := fixture.Server.Metadata.Review.GetContextByRun(context.Background(), fixture.RunID)
	if err != nil || contextRecord != nil {
		t.Fatalf("export initialized a review context: %+v err=%v", contextRecord, err)
	}
}

// TestPDFZIPCommentExportRetainsExactOutput verifies one annotated ZIP entry,
// manifest provenance, and source preservation.
func TestPDFZIPCommentExportRetainsExactOutput(t *testing.T) {
	fixture, _, _, _ := newPDFCommentFixture(t)
	handler := fixture.Server.Handler()
	status := pdfStatusForWork(t, handler, fixture.AvailableID)
	body := map[string]any{
		"articles":         []map[string]any{{"work_revision_id": fixture.RevisionID, "content_hash": status["content_hash"]}},
		"include_comments": true,
	}
	recorder := pdfExportRequest(t, handler, fmt.Sprintf("/api/runs/%d/pdf-exports", fixture.RunID), body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("comment ZIP status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	files, manifest := pdfExportZIP(t, recorder)
	if manifest["annotated"] != float64(1) || manifest["comments"] != float64(1) {
		t.Fatalf("manifest=%v", manifest)
	}
	entries, ok := manifest["files"].([]any)
	if !ok || len(entries) != 1 {
		t.Fatalf("manifest files=%v", manifest["files"])
	}
	file := entries[0].(map[string]any)
	if file["comments_status"] != "included" || file["output_content_hash"] == status["content_hash"] {
		t.Fatalf("manifest file=%v", file)
	}
	annotated := files[fmt.Sprintf("Viewer PDF article (annotated) (work-%d).pdf", fixture.AvailableID)]
	if len(annotated) == 0 {
		t.Fatalf("ZIP files=%v", keysOf(files))
	}
	digest := sha256.Sum256(annotated)
	if hex.EncodeToString(digest[:]) != file["output_content_hash"] {
		t.Fatal("ZIP entry hash differs from manifest")
	}
	if bytes.Equal(annotated, pdfBytes(t, handler, fixture.AvailableID)) {
		t.Fatal("ZIP annotated entry equals source bytes")
	}
	if recorder.Header().Get("X-PDF-Export-Annotated") != "1" || recorder.Header().Get("X-PDF-Export-Comments") != "1" {
		t.Fatalf("ZIP comment headers=%v", recorder.Header())
	}
}

// TestPDFCommentExportFailureIsExplicit verifies generation failure never
// returns unannotated success for single or ZIP exports.
func TestPDFCommentExportFailureIsExplicit(t *testing.T) {
	fixture, _, _, _ := newPDFCommentFixture(t)
	script := filepath.Join(t.TempDir(), "failing-worker")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	fixture.Server.SetPDFAnnotationWorker(pdfannotate.NewWorkerAt(script))
	handler := fixture.Server.Handler()
	status := pdfStatusForWork(t, handler, fixture.AvailableID)
	path := fmt.Sprintf("/api/runs/%d/articles/%d/pdf-export", fixture.RunID, fixture.RevisionID)
	recorder := pdfExportRequest(t, handler, path, map[string]any{"content_hash": status["content_hash"], "include_comments": true})
	if recorder.Code != http.StatusInternalServerError || pdfExportErrorCode(t, recorder) != "pdf_annotation_failed" {
		t.Fatalf("single failure status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if strings.HasPrefix(recorder.Body.String(), "%PDF-") {
		t.Fatal("failed annotation wrote PDF bytes")
	}
	body := map[string]any{
		"articles":         []map[string]any{{"work_revision_id": fixture.RevisionID, "content_hash": status["content_hash"]}},
		"include_comments": true,
	}
	recorder = pdfExportRequest(t, handler, fmt.Sprintf("/api/runs/%d/pdf-exports", fixture.RunID), body)
	if recorder.Code != http.StatusConflict || pdfExportErrorCode(t, recorder) != "pdf_export_empty" {
		t.Fatalf("ZIP failure status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var problem map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	details := problem["error"].(map[string]any)["details"].(map[string]any)
	files := details["files"].([]any)
	if files[0].(map[string]any)["reason"] != "pdf_annotation_failed" {
		t.Fatalf("ZIP failure files=%v", files)
	}
}

// keysOf returns sorted ZIP entry names for diagnostics.
func keysOf(files map[string][]byte) []string {
	keys := make([]string, 0, len(files))
	for key := range files {
		keys = append(keys, key)
	}
	return keys
}
