// pdf_export.go provides explicit single-PDF and selected-set export
// resources. It resolves exact selected-run article membership, validates
// expected source hashes, pins one companion read snapshot, optionally derives
// a separate comment-annotated copy through the bounded worker, records
// required export-preparation audit evidence, and returns original PDF bytes or
// a bounded streaming ZIP.
package server

import (
	viewrequest "analysis/backend/server/request"
	"archive/zip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"analysis/backend/database/review"
	"analysis/backend/database/work"
	"analysis/backend/manifest"
	"analysis/backend/pdfannotate"
	"analysis/backend/pdfstore"
)

const (
	maxPDFExportArticles       = 20
	maxPDFExportAggregate      = 100_000_000
	maxPDFExportManifestSize   = 1_000_000
	maxPDFExportFilenameBytes  = 240
	pdfExportConcurrency       = 2
	pdfExportDeadline          = 60 * time.Second
	pdfExportResponseDeadline  = 70 * time.Second
	pdfExportReadHeaderTimeout = 10 * time.Second
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
	WorkRevisionID    int64  `json:"work_revision_id"`
	WorkID            int64  `json:"work_id"`
	DOI               string `json:"doi,omitempty"`
	Filename          string `json:"filename,omitempty"`
	ContentHash       string `json:"content_hash,omitempty"`
	OutputContentHash string `json:"output_content_hash,omitempty"`
	ByteSize          int64  `json:"byte_size,omitempty"`
	OutputByteSize    int64  `json:"output_byte_size,omitempty"`
	Comments          int    `json:"comments,omitempty"`
	CommentsStatus    string `json:"comments_status,omitempty"`
	Status            string `json:"status"`
	Reason            string `json:"reason,omitempty"`
}

// pdfExportManifest is the bounded ZIP result manifest.
type pdfExportManifest struct {
	RunID           int64                   `json:"run_id"`
	IncludeComments bool                    `json:"include_comments"`
	Requested       int                     `json:"requested"`
	Included        int                     `json:"included"`
	Skipped         int                     `json:"skipped"`
	Annotated       int                     `json:"annotated"`
	Comments        int                     `json:"comments"`
	Files           []pdfExportManifestFile `json:"files"`
}

// pdfExportDerived is one validated exact output retained for response emission.
type pdfExportDerived struct {
	Data              []byte
	OutputContentHash string
	OutputByteSize    int64
	IncludeComments   bool
	Annotated         bool
	CommentCount      int
	CommentsStatus    string
	Evidence          *review.ExportEvidence
}

// pdfExportItem is one included export with its validated source metadata and
// either in-memory original bytes or one retained annotated temporary spool.
type pdfExportItem struct {
	Article  *work.ExportArticle
	Document *pdfstore.AvailableDocument
	Filename string
	Derived  *pdfExportDerived
	Spool    string
}

// pdfExportEvidence is the bounded operational PDF state recorded for one export.
type pdfExportEvidence struct {
	Status            string  `json:"status"`
	ContentHash       string  `json:"content_hash"`
	OutputContentHash string  `json:"output_content_hash,omitempty"`
	ByteSize          int64   `json:"byte_size"`
	OutputByteSize    int64   `json:"output_byte_size,omitempty"`
	InventoriedAt     string  `json:"inventoried_at,omitempty"`
	ChangeCounter     int64   `json:"change_counter"`
	IncludeComments   bool    `json:"include_comments"`
	CommentsStatus    string  `json:"comments_status"`
	CommentCount      int     `json:"comment_count,omitempty"`
	ReviewContextID   *int64  `json:"review_context_id,omitempty"`
	AnchorVersionIDs  []int64 `json:"anchor_version_ids,omitempty"`
	NoteVersionIDs    []int64 `json:"note_version_ids,omitempty"`
	ExcludedAnchors   int     `json:"excluded_anchors,omitempty"`
	ExcludedNotes     int     `json:"excluded_notes,omitempty"`
}

// pdfEvidenceReader reads immutable comment evidence from one pinned snapshot.
type pdfEvidenceReader interface {
	Evidence(context.Context, int64, int64, string) (*review.ExportEvidence, error)
}

// exportArticlePDF prepares one validated PDF attachment, optionally derived
// with matching saved comments.
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
	if !s.acquirePDFExport(false) {
		w.Header().Set("Retry-After", "1")
		s.respond(w, r, nil, pdfExportBusy())
		return
	}
	defer s.releasePDFExport(false)
	snapshot, err := s.beginPDFExportSnapshot(ctx)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	defer snapshot.Close()
	article := articles[0]
	data, document, exportErr := readPDFExport(ctx, snapshot, article, request.ContentHash)
	if exportErr != nil {
		s.respond(w, r, nil, exportErr)
		return
	}
	reviewSnapshot, contextID, err := s.beginReviewExport(ctx, runID, request.IncludeComments)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if reviewSnapshot != nil {
		defer reviewSnapshot.Close()
	}
	derived, err := s.derivePDFExport(ctx, reviewSnapshot, contextID, article, document, data, request.IncludeComments)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if reviewSnapshot != nil {
		if err := reviewSnapshot.Close(); err != nil {
			s.respond(w, r, nil, err)
			return
		}
	}
	correlationID, err := newPDFExportKey()
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	event, err := pdfExportAuditEvent(article, document, derived, runID, correlationID, "single")
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if err := s.recordPDFExportAudit(ctx, []*manifest.AuditEvent{event}); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if err := snapshot.Close(); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	filename := pdfExportFilename(article, false, derived.Annotated)
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": filename})
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", disposition)
	w.Header().Set("Content-Length", strconv.Itoa(len(derived.Data)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-PDF-Export-Content-Hash", document.ContentHash)
	w.Header().Set("X-PDF-Export-Output-Hash", derived.OutputContentHash)
	w.Header().Set("X-PDF-Export-Comments", strconv.Itoa(derived.CommentCount))
	w.Header().Set("X-PDF-Export-Comments-Status", derived.CommentsStatus)
	w.Header().Set("X-PDF-Export-Correlation-Id", correlationID)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(derived.Data)
}

// exportPDFs prepares a bounded ZIP of validated PDF bytes with an explicit
// per-file manifest, optional matching saved comments, and partial-result headers.
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
	if !s.acquirePDFExport(true) {
		w.Header().Set("Retry-After", "1")
		s.respond(w, r, nil, pdfExportBusy())
		return
	}
	defer s.releasePDFExport(true)
	snapshot, err := s.beginPDFExportSnapshot(ctx)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	defer snapshot.Close()
	reviewSnapshot, contextID, err := s.beginReviewExport(ctx, runID, request.IncludeComments)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if reviewSnapshot != nil {
		defer reviewSnapshot.Close()
	}
	spools := make([]string, 0, len(selectors))
	defer cleanupPDFExportSpools(spools)
	byRevision := make(map[int64]*work.ExportArticle, len(articles))
	for _, article := range articles {
		byRevision[article.RevisionID] = article
	}
	resultManifest := pdfExportManifest{
		RunID: runID, IncludeComments: request.IncludeComments, Requested: len(selectors),
		Files: make([]pdfExportManifestFile, 0, len(selectors)),
	}
	items := make([]pdfExportItem, 0, len(selectors))
	var aggregate int64
	for _, selector := range selectors {
		article := byRevision[selector.WorkRevisionID]
		data, document, exportErr := readPDFExport(ctx, snapshot, article, selector.ContentHash)
		if exportErr != nil {
			resultManifest.Files = append(resultManifest.Files, pdfExportSkippedFile(article, selector, exportErr))
			continue
		}
		derived, deriveErr := s.derivePDFExport(ctx, reviewSnapshot, contextID, article, document, data, request.IncludeComments)
		if deriveErr != nil {
			resultManifest.Files = append(resultManifest.Files, pdfExportSkippedFile(article, selector, deriveErr))
			continue
		}
		outputSize := derived.OutputByteSize
		if aggregate+outputSize > maxPDFExportAggregate {
			s.respond(w, r, nil, pdfExportLimitExceeded("the selected PDFs exceed the aggregate export limit"))
			return
		}
		aggregate += outputSize
		filename := pdfExportFilename(article, true, derived.Annotated)
		item := pdfExportItem{Article: article, Document: document, Filename: filename, Derived: derived}
		spool, spoolErr := spoolPDFExport(derived.Data)
		if spoolErr != nil {
			log.Error("PDF export spool failed", "revision_id", article.RevisionID, "error", spoolErr)
			s.respond(w, r, nil, pdfExportStorageError())
			return
		}
		spools = append(spools, spool)
		item.Spool = spool
		derived.Data = nil
		resultManifest.Files = append(resultManifest.Files, pdfExportManifestFile{
			WorkRevisionID: article.RevisionID, WorkID: article.WorkID,
			DOI: pdfExportDOI(article), Filename: filename,
			ContentHash: document.ContentHash, OutputContentHash: derived.OutputContentHash,
			ByteSize: document.ByteSize, OutputByteSize: derived.OutputByteSize,
			Comments: derived.CommentCount, CommentsStatus: derived.CommentsStatus, Status: "included",
		})
		if derived.Annotated {
			resultManifest.Annotated++
		}
		resultManifest.Comments += derived.CommentCount
		items = append(items, item)
	}
	resultManifest.Included = len(items)
	resultManifest.Skipped = len(resultManifest.Files) - len(items)
	if reviewSnapshot != nil {
		if err := reviewSnapshot.Close(); err != nil {
			s.respond(w, r, nil, err)
			return
		}
	}
	if resultManifest.Included == 0 {
		s.respond(w, r, nil, pdfExportEmpty(resultManifest))
		return
	}
	manifestBytes, err := json.Marshal(resultManifest)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if len(manifestBytes) > maxPDFExportManifestSize {
		s.respond(w, r, nil, pdfExportLimitExceeded("the export manifest exceeds its bounded metadata limit"))
		return
	}
	correlationID, err := newPDFExportKey()
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	events := make([]*manifest.AuditEvent, 0, len(items))
	for _, item := range items {
		event, err := pdfExportAuditEvent(item.Article, item.Document, item.Derived, runID, correlationID, "zip")
		if err != nil {
			s.respond(w, r, nil, err)
			return
		}
		events = append(events, event)
	}
	if err := s.recordPDFExportAudit(ctx, events); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": fmt.Sprintf("run-%d-pdfs.zip", runID)}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-PDF-Export-Included", strconv.Itoa(resultManifest.Included))
	w.Header().Set("X-PDF-Export-Skipped", strconv.Itoa(resultManifest.Skipped))
	w.Header().Set("X-PDF-Export-Annotated", strconv.Itoa(resultManifest.Annotated))
	w.Header().Set("X-PDF-Export-Comments", strconv.Itoa(resultManifest.Comments))
	w.Header().Set("X-PDF-Export-Correlation-Id", correlationID)
	w.WriteHeader(http.StatusOK)
	archive := zip.NewWriter(w)
	for _, item := range items {
		header := &zip.FileHeader{Name: item.Filename, Method: zip.Store}
		header.SetMode(0o644)
		entry, err := archive.CreateHeader(header)
		if err != nil {
			_ = archive.Close()
			log.Error("PDF export ZIP entry failed", "revision_id", item.Article.RevisionID, "error", err)
			panic(http.ErrAbortHandler)
		}
		if err := writeSpooledPDF(entry, item); err != nil {
			_ = archive.Close()
			log.Error("PDF export spool stream failed", "revision_id", item.Article.RevisionID, "error", err)
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

// beginReviewExport resolves the selected run's existing review context and,
// when comments are requested and the context exists, pins one review snapshot.
func (s *Server) beginReviewExport(ctx context.Context, runID int64, include bool) (*review.ExportSnapshot, int64, error) {
	if !include || s.reviewStore == nil {
		return nil, 0, nil
	}
	contextRecord, err := s.reviewStore.GetContextByRun(ctx, runID)
	if err != nil {
		log.Error("PDF export review context read failed", "error", err)
		return nil, 0, &viewrequest.Problem{Status: http.StatusInternalServerError, Code: "pdf_annotation_failed", Message: "the saved comments could not be read"}
	}
	if contextRecord == nil {
		return nil, 0, nil
	}
	snapshot, err := s.reviewStore.BeginExportSnapshot(ctx)
	if err != nil {
		log.Error("PDF export review snapshot failed", "error", err)
		return nil, 0, &viewrequest.Problem{Status: http.StatusServiceUnavailable, Code: "pdf_annotation_unavailable", Message: "the saved comments are temporarily unavailable; retry shortly", Details: map[string]any{"retry_after_seconds": 1}}
	}
	return snapshot, contextRecord.ID, nil
}

// derivePDFExport returns exact original bytes or one validated annotated copy.
func (s *Server) derivePDFExport(ctx context.Context, reader pdfEvidenceReader, contextID int64, article *work.ExportArticle, document *pdfstore.AvailableDocument, data []byte, include bool) (*pdfExportDerived, error) {
	if !include {
		return &pdfExportDerived{Data: data, OutputContentHash: document.ContentHash, OutputByteSize: document.ByteSize, CommentsStatus: "off"}, nil
	}
	if reader == nil || contextID == 0 {
		return &pdfExportDerived{Data: data, OutputContentHash: document.ContentHash, OutputByteSize: document.ByteSize, IncludeComments: true, CommentsStatus: "no_review_context"}, nil
	}
	evidence, err := reader.Evidence(ctx, contextID, article.WorkID, document.ContentHash)
	if errors.Is(err, review.ErrExportContextNotFound) {
		return &pdfExportDerived{Data: data, OutputContentHash: document.ContentHash, OutputByteSize: document.ByteSize, IncludeComments: true, CommentsStatus: "no_review_context"}, nil
	}
	if err != nil {
		return nil, pdfAnnotationProblem(err)
	}
	annotations, err := pdfCommentAnnotations(evidence)
	if err != nil {
		return nil, pdfAnnotationProblem(err)
	}
	if len(annotations) == 0 {
		return &pdfExportDerived{Data: data, OutputContentHash: document.ContentHash, OutputByteSize: document.ByteSize, IncludeComments: true, CommentsStatus: "no_matching_comments", Evidence: evidence}, nil
	}
	if s.annotationWorker == nil {
		return nil, &viewrequest.Problem{Status: http.StatusServiceUnavailable, Code: "pdf_annotation_unavailable", Message: "the comment annotation worker is not configured"}
	}
	output, err := s.annotationWorker.Annotate(ctx, data, annotations)
	if err != nil {
		return nil, pdfAnnotationProblem(err)
	}
	digest := sha256.Sum256(output)
	outputHash := hex.EncodeToString(digest[:])
	if outputHash == document.ContentHash {
		return nil, &viewrequest.Problem{Status: http.StatusInternalServerError, Code: "pdf_annotation_failed", Message: "the annotated PDF did not change the source bytes; retry without comments"}
	}
	return &pdfExportDerived{
		Data: output, OutputContentHash: outputHash, OutputByteSize: int64(len(output)),
		IncludeComments: true, Annotated: true, CommentCount: len(annotations),
		CommentsStatus: "included", Evidence: evidence,
	}, nil
}

// pdfAnnotationProblem maps annotation limits and generation failures to stable problems.
func pdfAnnotationProblem(err error) error {
	var problem *viewrequest.Problem
	if errors.As(err, &problem) {
		return problem
	}
	switch {
	case errors.Is(err, review.ErrExportLimit), errors.Is(err, pdfannotate.ErrLimit):
		return pdfExportLimitExceeded("the saved comments exceed the bounded annotation export limit; retry without comments")
	case errors.Is(err, pdfannotate.ErrProtectedPDF):
		return &viewrequest.Problem{Status: http.StatusUnprocessableEntity, Code: "pdf_protected", Message: "the selected PDF is encrypted or permission-restricted; comments were not embedded"}
	case errors.Is(err, pdfannotate.ErrSignedPDF):
		return &viewrequest.Problem{Status: http.StatusUnprocessableEntity, Code: "pdf_signed", Message: "the selected PDF is digitally signed; comments were not embedded"}
	case errors.Is(err, pdfannotate.ErrMissingPage):
		return &viewrequest.Problem{Status: http.StatusUnprocessableEntity, Code: "pdf_page_missing", Message: "a saved comment references a page that does not exist in the selected PDF"}
	case errors.Is(err, pdfannotate.ErrInvalidGeometry):
		return &viewrequest.Problem{Status: http.StatusUnprocessableEntity, Code: "pdf_geometry_invalid", Message: "a saved comment has geometry that cannot be placed on the selected PDF"}
	default:
		log.Error("PDF annotation failed", "error", err)
		return &viewrequest.Problem{Status: http.StatusInternalServerError, Code: "pdf_annotation_failed", Message: "the saved comments could not be embedded; retry without comments"}
	}
}

// spoolPDFExport retains one exact derived PDF in a private temporary file.
func spoolPDFExport(data []byte) (string, error) {
	file, err := os.CreateTemp("", "cogen-pdf-export-*")
	if err != nil {
		return "", err
	}
	path := file.Name()
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		_ = os.Remove(path)
		return "", err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

// cleanupPDFExportSpools removes every retained derived PDF on any exit path.
func cleanupPDFExportSpools(paths []string) {
	for _, path := range paths {
		_ = os.Remove(path)
	}
}

// writeSpooledPDF streams one retained derivative and verifies its exact hash.
func writeSpooledPDF(destination io.Writer, item pdfExportItem) error {
	file, err := os.Open(item.Spool)
	if err != nil {
		return err
	}
	defer file.Close()
	digest := sha256.New()
	written, err := io.Copy(io.MultiWriter(destination, digest), file)
	if err != nil {
		return err
	}
	if written != item.Derived.OutputByteSize || hex.EncodeToString(digest.Sum(nil)) != item.Derived.OutputContentHash {
		return errors.New("retained PDF export bytes changed before emission")
	}
	return nil
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

// beginPDFExportSnapshot starts one pinned companion read transaction.
func (s *Server) beginPDFExportSnapshot(ctx context.Context) (*pdfstore.ExportSnapshot, error) {
	if s.pdfStore == nil || s.pdfDB == nil {
		return nil, &viewrequest.Problem{Status: http.StatusServiceUnavailable, Code: "pdf_store_unavailable", Message: "the PDF store is not readable"}
	}
	snapshot, err := s.pdfStore.BeginExportSnapshot(ctx)
	if err != nil {
		if isPDFStoreBusy(err) {
			return nil, &viewrequest.Problem{Status: http.StatusServiceUnavailable, Code: "pdf_store_busy", Message: "the PDF store is temporarily busy; retry shortly", Details: map[string]any{"retry_after_seconds": 1}}
		}
		return nil, err
	}
	return snapshot, nil
}

// readPDFExport reads and validates one selected PDF inside a pinned snapshot.
func readPDFExport(ctx context.Context, snapshot *pdfstore.ExportSnapshot, article *work.ExportArticle, expected string) ([]byte, *pdfstore.AvailableDocument, error) {
	if article == nil {
		return nil, nil, viewrequest.NotFound("article revision not found")
	}
	doi := pdfExportDOI(article)
	if doi == "" {
		return nil, nil, &viewrequest.Problem{Status: http.StatusConflict, Code: "pdf_doi_missing", Message: "the selected work has no normalized DOI for PDF inventory"}
	}
	document, data, err := snapshot.Available(ctx, doi)
	if errors.Is(err, pdfstore.ErrPDFIntegrity) {
		return nil, nil, &viewrequest.Problem{Status: http.StatusUnprocessableEntity, Code: "pdf_integrity_error", Message: "stored PDF content failed integrity validation"}
	}
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
	return data, document, nil
}

// pdfExportAuditEvent builds one bounded work-scoped export-preparation event.
func pdfExportAuditEvent(article *work.ExportArticle, document *pdfstore.AvailableDocument, derived *pdfExportDerived, runID int64, correlationID, scope string) (*manifest.AuditEvent, error) {
	eventKey, err := newPDFExportKey()
	if err != nil {
		return nil, err
	}
	evidence := pdfExportEvidence{
		Status: pdfstore.StatusAvailable, ContentHash: document.ContentHash,
		OutputContentHash: derived.OutputContentHash, ByteSize: document.ByteSize,
		OutputByteSize: derived.OutputByteSize, InventoriedAt: document.InventoriedAt,
		ChangeCounter: document.ChangeCounter, IncludeComments: derived.IncludeComments,
		CommentsStatus: derived.CommentsStatus, CommentCount: derived.CommentCount,
	}
	if derived.Evidence != nil {
		contextID := derived.Evidence.ContextID
		evidence.ReviewContextID = &contextID
		evidence.ExcludedAnchors = derived.Evidence.ExcludedAnchors
		evidence.ExcludedNotes = derived.Evidence.ExcludedNotes
		for _, anchor := range derived.Evidence.Anchors {
			evidence.AnchorVersionIDs = append(evidence.AnchorVersionIDs, anchor.VersionID)
		}
		for _, note := range derived.Evidence.Notes {
			evidence.NoteVersionIDs = append(evidence.NoteVersionIDs, note.VersionID)
		}
	}
	after, err := json.Marshal(evidence)
	if err != nil {
		return nil, err
	}
	metadata := map[string]any{
		"source": "frontend_download", "scope": scope, "include_comments": derived.IncludeComments,
		"initiating_run_id": runID, "initiating_revision_id": article.RevisionID,
		"doi": pdfExportDOI(article), "event_key": eventKey,
		"comments_status": derived.CommentsStatus, "comment_count": derived.CommentCount,
	}
	if derived.Evidence != nil {
		metadata["review_context_id"] = derived.Evidence.ContextID
		metadata["excluded_anchors"] = derived.Evidence.ExcludedAnchors
		metadata["excluded_notes"] = derived.Evidence.ExcludedNotes
	}
	encodedMetadata, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	return &manifest.AuditEvent{
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), Actor: "user",
		EntityType: "work", EntityID: strconv.FormatInt(article.WorkID, 10),
		Action: manifest.AuditPDFDocumentExportPrepared, AfterJSON: string(after),
		MetadataJSON: string(encodedMetadata), CorrelationID: correlationID,
	}, nil
}

// recordPDFExportAudit commits the required metadata-owned event batch before
// binary response headers are written.
func (s *Server) recordPDFExportAudit(ctx context.Context, events []*manifest.AuditEvent) error {
	if s.writeDB == nil {
		return &viewrequest.Problem{Status: http.StatusInternalServerError, Code: "pdf_audit_error", Message: "unable to record the PDF export audit event; no download was prepared"}
	}
	if err := s.writeDB.Audit.InsertBatch(ctx, events); err != nil {
		log.Error("PDF export audit insertion failed", "error", err)
		return &viewrequest.Problem{Status: http.StatusInternalServerError, Code: "pdf_audit_error", Message: "unable to record the PDF export audit event; no download was prepared"}
	}
	return nil
}

// newPDFExportKey returns one random lowercase hexadecimal event or correlation key.
func newPDFExportKey() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate PDF export key: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
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

// pdfExportFilename returns one readable, bounded attachment name. Archive
// names include a stable work suffix so equal titles cannot collide, and
// annotated names remain distinguishable from original downloads.
func pdfExportFilename(article *work.ExportArticle, archive, annotated bool) string {
	workID := int64(0)
	if article != nil {
		workID = article.WorkID
	}
	if workID < 1 {
		workID = 1
	}
	base := ""
	if article != nil && article.Title != nil {
		base = sanitizePDFExportTitle(*article.Title)
	}
	suffix := ""
	if annotated {
		suffix += " (annotated)"
	}
	if archive {
		suffix += fmt.Sprintf(" (work-%d)", workID)
	}
	if base == "" {
		annotatedSuffix := ""
		if annotated {
			annotatedSuffix = " (annotated)"
		}
		return fmt.Sprintf("work-%d%s.pdf", workID, annotatedSuffix)
	}
	base = truncatePDFExportTitle(base, maxPDFExportFilenameBytes-len(suffix)-len(".pdf"))
	base = strings.TrimRight(base, " .")
	if strings.Trim(base, "- .") == "" {
		return fmt.Sprintf("work-%d%s.pdf", workID, suffix)
	}
	return base + suffix + ".pdf"
}

// sanitizePDFExportTitle removes path separators, control characters, unsafe
// filename characters, and leading or trailing dots and spaces.
func sanitizePDFExportTitle(title string) string {
	name := strings.Map(func(character rune) rune {
		if unicode.IsControl(character) || unicode.IsSpace(character) {
			return ' '
		}
		if strings.ContainsRune(`/\:*?"<>|`, character) {
			return '-'
		}
		return character
	}, title)
	name = strings.Join(strings.Fields(name), " ")
	return strings.Trim(name, " .")
}

// truncatePDFExportTitle removes complete runes until the byte bound is met.
func truncatePDFExportTitle(value string, limit int) string {
	if limit < 1 {
		return ""
	}
	for len(value) > limit {
		_, size := utf8.DecodeLastRuneInString(value)
		value = value[:len(value)-size]
	}
	return value
}

// pdfExportStorageError returns one explicit temporary-spool failure.
func pdfExportStorageError() error {
	return &viewrequest.Problem{Status: http.StatusInternalServerError, Code: "pdf_export_storage_error", Message: "unable to retain the annotated PDF output; no download was prepared"}
}

// pdfExportLimitExceeded returns one explicit export-limit failure.
func pdfExportLimitExceeded(message string) error {
	return &viewrequest.Problem{Status: http.StatusRequestEntityTooLarge, Code: "pdf_export_limit", Message: message}
}

// pdfExportEmpty returns the all-failed selected-set failure with bounded per-file reasons.
func pdfExportEmpty(result pdfExportManifest) error {
	return &viewrequest.Problem{
		Status: http.StatusConflict, Code: "pdf_export_empty",
		Message: "no selected PDF could be exported",
		Details: map[string]any{
			"requested": result.Requested, "included": result.Included, "skipped": result.Skipped,
			"files": result.Files,
		},
	}
}

// pdfExportBusy returns the bounded explicit-export admission failure.
func pdfExportBusy() error {
	return &viewrequest.Problem{
		Status: http.StatusServiceUnavailable, Code: "pdf_export_busy",
		Message: "two PDF exports are already in progress; retry shortly",
		Details: map[string]any{"retry_after_seconds": 1},
	}
}

// acquirePDFExport reserves one explicit export response and, for a ZIP, the
// single archive/transformation slot.
func (s *Server) acquirePDFExport(archive bool) bool {
	if s.pdfExports == nil {
		return false
	}
	select {
	case s.pdfExports <- struct{}{}:
	default:
		return false
	}
	if !archive {
		return true
	}
	if s.pdfZIPExports == nil {
		s.releasePDFExport(false)
		return false
	}
	select {
	case s.pdfZIPExports <- struct{}{}:
		return true
	default:
		s.releasePDFExport(false)
		return false
	}
}

// releasePDFExport releases one explicit export response on every exit path.
func (s *Server) releasePDFExport(archive bool) {
	if archive {
		select {
		case <-s.pdfZIPExports:
		default:
		}
	}
	select {
	case <-s.pdfExports:
	default:
	}
}

// withPDFExportDeadlines applies finite body, response, and context deadlines
// to explicit export routes before the JSON response budget wrapper.
func withPDFExportDeadlines(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isPDFExportRequest(r) {
			next.ServeHTTP(w, r)
			return
		}
		controller := http.NewResponseController(w)
		now := time.Now()
		_ = controller.SetReadDeadline(now.Add(pdfExportReadHeaderTimeout))
		_ = controller.SetWriteDeadline(now.Add(pdfExportResponseDeadline))
		ctx, cancel := context.WithTimeout(r.Context(), pdfExportDeadline)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// isPDFExportRequest selects only the two explicit export resources.
func isPDFExportRequest(r *http.Request) bool {
	return r.Method == http.MethodPost && isPDFExportPath(r.URL.Path)
}
