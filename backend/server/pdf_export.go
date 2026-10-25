// pdf_export.go provides explicit single-PDF and selected-set export
// resources. It resolves exact selected-run article membership, validates
// expected source hashes, and returns original PDF bytes or a bounded ZIP.
package server

import (
	viewrequest "analysis/backend/server/request"
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"analysis/backend/database/work"
	"analysis/backend/pdfstore"
)

const (
	maxPDFExportArticles     = 20
	maxPDFExportAggregate    = 100_000_000
	maxPDFExportManifestSize = 1_000_000
)

// pdfExportSelector identifies one expected article revision and source hash.
type pdfExportSelector struct {
	WorkRevisionID int64  `json:"work_revision_id"`
	ContentHash    string `json:"content_hash"`
}

// pdfExportSelection is the bounded selected-set export request.
type pdfExportSelection struct {
	Articles        []pdfExportSelector `json:"articles"`
	IncludeComments bool                `json:"include_comments"`
}

// pdfSingleExportRequest is the bounded single-article export request.
type pdfSingleExportRequest struct {
	ContentHash     string `json:"content_hash"`
	IncludeComments bool   `json:"include_comments"`
}

// pdfExportManifestFile is one bounded per-file ZIP result.
type pdfExportManifestFile struct {
	WorkRevisionID int64  `json:"work_revision_id"`
	WorkID         int64  `json:"work_id"`
	DOI            string `json:"doi,omitempty"`
	Filename       string `json:"filename,omitempty"`
	ContentHash    string `json:"content_hash,omitempty"`
	ByteSize       int64  `json:"byte_size,omitempty"`
	Status         string `json:"status"`
	Reason         string `json:"reason,omitempty"`
}

// pdfExportManifest is the bounded ZIP result manifest.
type pdfExportManifest struct {
	RunID           int64                   `json:"run_id"`
	IncludeComments bool                    `json:"include_comments"`
	Requested       int                     `json:"requested"`
	Included        int                     `json:"included"`
	Skipped         int                     `json:"skipped"`
	Files           []pdfExportManifestFile `json:"files"`
}

// pdfExportItem is one included export with its validated source metadata.
type pdfExportItem struct {
	Article  *work.ExportArticle
	Document *pdfstore.AvailableDocument
	Filename string
}

// exportArticlePDF prepares one validated original-byte PDF attachment.
func (s *Server) exportArticlePDF(w http.ResponseWriter, r *http.Request) {
	setMutableResponseHeaders(w)
	runID, revisionID, err := pdfMutationIDs(r)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	var request pdfSingleExportRequest
	if err := decodeMutationJSON(w, r, &request); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if request.IncludeComments {
		s.respond(w, r, nil, pdfCommentsUnavailable())
		return
	}
	if err := validatePDFContentHash(request.ContentHash); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx := r.Context()
	if err := s.requireRun(ctx, runID); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	articles, err := s.workStore.ListExportArticles(ctx, runID, []int64{revisionID})
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if len(articles) != 1 {
		s.respond(w, r, nil, viewrequest.NotFound("article revision does not belong to selected run"))
		return
	}
	article := articles[0]
	data, document, exportErr := s.readPDFExport(ctx, article, request.ContentHash)
	if exportErr != nil {
		s.respond(w, r, nil, exportErr)
		return
	}
	filename := pdfExportFilename(article, false)
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": filename})
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", disposition)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-PDF-Export-Content-Hash", document.ContentHash)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// exportPDFs prepares a bounded ZIP of validated original PDF bytes with an
// explicit per-file manifest and partial-result headers.
func (s *Server) exportPDFs(w http.ResponseWriter, r *http.Request) {
	setMutableResponseHeaders(w)
	runID, err := viewrequest.PositiveID(r.PathValue("run_id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	var request pdfExportSelection
	if err := decodeMutationJSON(w, r, &request); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if request.IncludeComments {
		s.respond(w, r, nil, pdfCommentsUnavailable())
		return
	}
	selectors, err := boundedPDFExportSelectors(request.Articles)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx := r.Context()
	if err := s.requireRun(ctx, runID); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	revisionIDs := make([]int64, 0, len(selectors))
	for _, selector := range selectors {
		revisionIDs = append(revisionIDs, selector.WorkRevisionID)
	}
	articles, err := s.workStore.ListExportArticles(ctx, runID, revisionIDs)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if len(articles) != len(selectors) {
		s.respond(w, r, nil, viewrequest.NotFound("one or more article revisions do not belong to selected run"))
		return
	}
	byRevision := make(map[int64]*work.ExportArticle, len(articles))
	for _, article := range articles {
		byRevision[article.RevisionID] = article
	}
	manifest := pdfExportManifest{
		RunID: runID, IncludeComments: false, Requested: len(selectors),
		Files: make([]pdfExportManifestFile, 0, len(selectors)),
	}
	items := make([]pdfExportItem, 0, len(selectors))
	var aggregate int64
	for _, selector := range selectors {
		article := byRevision[selector.WorkRevisionID]
		document, exportErr := s.preflightPDFExport(ctx, article, selector.ContentHash)
		if exportErr != nil {
			manifest.Files = append(manifest.Files, pdfExportSkippedFile(article, selector, exportErr))
			continue
		}
		if aggregate+document.ByteSize > maxPDFExportAggregate {
			s.respond(w, r, nil, pdfExportLimitExceeded("the selected PDFs exceed the aggregate export limit"))
			return
		}
		aggregate += document.ByteSize
		filename := pdfExportFilename(article, true)
		manifest.Files = append(manifest.Files, pdfExportManifestFile{
			WorkRevisionID: article.RevisionID, WorkID: article.WorkID,
			DOI: pdfExportDOI(article), Filename: filename,
			ContentHash: document.ContentHash, ByteSize: document.ByteSize, Status: "included",
		})
		items = append(items, pdfExportItem{Article: article, Document: document, Filename: filename})
	}
	manifest.Included = len(items)
	manifest.Skipped = len(manifest.Files) - len(items)
	if manifest.Included == 0 {
		s.respond(w, r, nil, pdfExportEmpty(manifest))
		return
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if len(manifestBytes) > maxPDFExportManifestSize {
		s.respond(w, r, nil, pdfExportLimitExceeded("the export manifest exceeds its bounded metadata limit"))
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": fmt.Sprintf("run-%d-pdfs.zip", runID)}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-PDF-Export-Included", strconv.Itoa(manifest.Included))
	w.Header().Set("X-PDF-Export-Skipped", strconv.Itoa(manifest.Skipped))
	w.WriteHeader(http.StatusOK)
	archive := zip.NewWriter(w)
	for _, item := range items {
		data, _, exportErr := s.readPDFExport(ctx, item.Article, item.Document.ContentHash)
		if exportErr != nil {
			_ = archive.Close()
			log.Error("PDF export stream failed after headers", "revision_id", item.Article.RevisionID, "error", exportErr)
			panic(http.ErrAbortHandler)
		}
		header := &zip.FileHeader{Name: item.Filename, Method: zip.Store}
		header.SetMode(0o644)
		entry, err := archive.CreateHeader(header)
		if err != nil {
			_ = archive.Close()
			log.Error("PDF export ZIP entry failed", "revision_id", item.Article.RevisionID, "error", err)
			panic(http.ErrAbortHandler)
		}
		if _, err := entry.Write(data); err != nil {
			_ = archive.Close()
			log.Error("PDF export ZIP write failed", "revision_id", item.Article.RevisionID, "error", err)
			panic(http.ErrAbortHandler)
		}
	}
	entry, err := archive.CreateHeader(&zip.FileHeader{Name: "export-manifest.json", Method: zip.Store})
	if err != nil {
		_ = archive.Close()
		log.Error("PDF export manifest entry failed", "error", err)
		panic(http.ErrAbortHandler)
	}
	if _, err := entry.Write(manifestBytes); err != nil {
		_ = archive.Close()
		log.Error("PDF export manifest write failed", "error", err)
		panic(http.ErrAbortHandler)
	}
	if err := archive.Close(); err != nil {
		log.Error("PDF export ZIP close failed", "error", err)
		panic(http.ErrAbortHandler)
	}
}

// boundedPDFExportSelectors validates and deduplicates a selected-set request.
func boundedPDFExportSelectors(raw []pdfExportSelector) ([]pdfExportSelector, error) {
	if len(raw) == 0 {
		return nil, viewrequest.BadRequest("articles must contain at least one selection")
	}
	seen := make(map[int64]string, len(raw))
	items := make([]pdfExportSelector, 0, len(raw))
	for _, item := range raw {
		if item.WorkRevisionID < 1 {
			return nil, viewrequest.BadRequest("work_revision_id must be a positive integer")
		}
		if err := validatePDFContentHash(item.ContentHash); err != nil {
			return nil, err
		}
		if existing, ok := seen[item.WorkRevisionID]; ok {
			if existing != item.ContentHash {
				return nil, viewrequest.BadRequest("duplicate work_revision_id values must use the same content_hash")
			}
			continue
		}
		seen[item.WorkRevisionID] = item.ContentHash
		items = append(items, item)
		if len(items) > maxPDFExportArticles {
			return nil, pdfExportLimitExceeded(fmt.Sprintf("an export selects at most %d unique article revisions", maxPDFExportArticles))
		}
	}
	return items, nil
}

// validatePDFContentHash requires one lowercase SHA-256 source identity.
func validatePDFContentHash(value string) error {
	if len(value) != 64 || !lowercaseHex(value) {
		return viewrequest.BadRequest("content_hash must be 64 lowercase hexadecimal characters")
	}
	return nil
}

// preflightPDFExport validates one selected PDF without retaining its bytes.
func (s *Server) preflightPDFExport(ctx context.Context, article *work.ExportArticle, expected string) (*pdfstore.AvailableDocument, error) {
	data, document, err := s.readPDFExport(ctx, article, expected)
	_ = data
	return document, err
}

// readPDFExport reads and validates one selected PDF against its expected hash.
func (s *Server) readPDFExport(ctx context.Context, article *work.ExportArticle, expected string) ([]byte, *pdfstore.AvailableDocument, error) {
	if article == nil {
		return nil, nil, viewrequest.NotFound("article revision not found")
	}
	doi := pdfExportDOI(article)
	if doi == "" {
		return nil, nil, &viewrequest.Problem{Status: http.StatusConflict, Code: "pdf_doi_missing", Message: "the selected work has no normalized DOI for PDF inventory"}
	}
	if s.pdfStore == nil || s.pdfDB == nil {
		return nil, nil, &viewrequest.Problem{Status: http.StatusServiceUnavailable, Code: "pdf_store_unavailable", Message: "the PDF store is not readable"}
	}
	document, err := s.pdfStore.AvailableDocument(ctx, doi)
	if err != nil {
		return nil, nil, err
	}
	if document == nil {
		return nil, nil, &viewrequest.Problem{Status: http.StatusConflict, Code: "pdf_unavailable", Message: "the selected PDF is not available"}
	}
	if document.ContentHash != expected {
		return nil, document, &viewrequest.Problem{
			Status: http.StatusConflict, Code: "pdf_changed",
			Message: "the selected PDF content changed; refresh the selection before exporting",
			Details: map[string]any{"expected_content_hash": expected, "current_content_hash": document.ContentHash},
		}
	}
	if document.ByteSize < 1 || document.ByteSize > pdfstore.DefaultMaxPDFBytes {
		return nil, nil, &viewrequest.Problem{Status: http.StatusUnprocessableEntity, Code: "pdf_integrity_error", Message: "stored PDF content failed integrity validation"}
	}
	data, err := s.pdfStore.BlobData(ctx, document.ContentHash)
	if err != nil {
		return nil, nil, err
	}
	if data == nil {
		return nil, nil, &viewrequest.Problem{Status: http.StatusUnprocessableEntity, Code: "pdf_integrity_error", Message: "stored PDF content failed integrity validation"}
	}
	digest, err := pdfstore.ValidatePDF(data, pdfstore.DefaultMaxPDFBytes)
	if err != nil || digest != document.ContentHash || int64(len(data)) != document.ByteSize {
		return nil, nil, &viewrequest.Problem{Status: http.StatusUnprocessableEntity, Code: "pdf_integrity_error", Message: "stored PDF content failed integrity validation"}
	}
	return data, document, nil
}

// pdfExportSkippedFile renders one stable per-file ZIP failure result.
func pdfExportSkippedFile(article *work.ExportArticle, selector pdfExportSelector, err error) pdfExportManifestFile {
	result := pdfExportManifestFile{
		WorkRevisionID: selector.WorkRevisionID, Status: "skipped", Reason: "pdf_export_failed",
	}
	if article != nil {
		result.WorkID = article.WorkID
		result.DOI = pdfExportDOI(article)
	}
	var problem *viewrequest.Problem
	if errors.As(err, &problem) {
		result.Reason = problem.Code
	}
	return result
}

// pdfExportDOI returns the selected article's normalized DOI.
func pdfExportDOI(article *work.ExportArticle) string {
	if article == nil || article.DOI == nil {
		return ""
	}
	return strings.TrimSpace(*article.DOI)
}

// pdfExportFilename returns one readable, bounded attachment name.
func pdfExportFilename(article *work.ExportArticle, archive bool) string {
	workID := int64(0)
	if article != nil {
		workID = article.WorkID
	}
	if workID < 1 {
		workID = 1
	}
	return fmt.Sprintf("work-%d.pdf", workID)
}

// pdfCommentsUnavailable rejects comment-enabled exports until the annotation phase.
func pdfCommentsUnavailable() error {
	return &viewrequest.Problem{Status: http.StatusBadRequest, Code: "pdf_comments_unavailable", Message: "comment-enabled PDF export is not available in this phase"}
}

// pdfExportLimitExceeded returns one explicit export-limit failure.
func pdfExportLimitExceeded(message string) error {
	return &viewrequest.Problem{Status: http.StatusRequestEntityTooLarge, Code: "pdf_export_limit", Message: message}
}

// pdfExportEmpty returns the all-failed selected-set failure with bounded per-file reasons.
func pdfExportEmpty(manifest pdfExportManifest) error {
	return &viewrequest.Problem{
		Status: http.StatusConflict, Code: "pdf_export_empty",
		Message: "no selected PDF could be exported",
		Details: map[string]any{
			"requested": manifest.Requested, "included": manifest.Included, "skipped": manifest.Skipped,
			"files": manifest.Files,
		},
	}
}
