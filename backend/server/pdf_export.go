// pdf_export.go provides explicit single-PDF and selected-set export
// resources. It resolves exact selected-run article membership, validates
// expected source hashes, pins one companion read snapshot, records required
// export-preparation audit evidence, and returns original PDF bytes or a
// bounded streaming ZIP.
package server

import (
	viewrequest "analysis/backend/server/request"
	"archive/zip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"analysis/backend/database/work"
	"analysis/backend/manifest"
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

// pdfExportEvidence is the bounded operational PDF state recorded for one export.
type pdfExportEvidence struct {
	Status        string `json:"status"`
	ContentHash   string `json:"content_hash"`
	ByteSize      int64  `json:"byte_size"`
	InventoriedAt string `json:"inventoried_at,omitempty"`
	ChangeCounter int64  `json:"change_counter"`
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
	correlationID, err := newPDFExportKey()
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	event, err := pdfExportAuditEvent(article, document, runID, correlationID, "single")
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
	filename := pdfExportFilename(article, false)
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": filename})
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", disposition)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-PDF-Export-Content-Hash", document.ContentHash)
	w.Header().Set("X-PDF-Export-Correlation-Id", correlationID)
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
	byRevision := make(map[int64]*work.ExportArticle, len(articles))
	for _, article := range articles {
		byRevision[article.RevisionID] = article
	}
	resultManifest := pdfExportManifest{
		RunID: runID, IncludeComments: false, Requested: len(selectors),
		Files: make([]pdfExportManifestFile, 0, len(selectors)),
	}
	items := make([]pdfExportItem, 0, len(selectors))
	var aggregate int64
	for _, selector := range selectors {
		article := byRevision[selector.WorkRevisionID]
		_, document, exportErr := readPDFExport(ctx, snapshot, article, selector.ContentHash)
		if exportErr != nil {
			resultManifest.Files = append(resultManifest.Files, pdfExportSkippedFile(article, selector, exportErr))
			continue
		}
		if aggregate+document.ByteSize > maxPDFExportAggregate {
			s.respond(w, r, nil, pdfExportLimitExceeded("the selected PDFs exceed the aggregate export limit"))
			return
		}
		aggregate += document.ByteSize
		filename := pdfExportFilename(article, true)
		resultManifest.Files = append(resultManifest.Files, pdfExportManifestFile{
			WorkRevisionID: article.RevisionID, WorkID: article.WorkID,
			DOI: pdfExportDOI(article), Filename: filename,
			ContentHash: document.ContentHash, ByteSize: document.ByteSize, Status: "included",
		})
		items = append(items, pdfExportItem{Article: article, Document: document, Filename: filename})
	}
	resultManifest.Included = len(items)
	resultManifest.Skipped = len(resultManifest.Files) - len(items)
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
		event, err := pdfExportAuditEvent(item.Article, item.Document, runID, correlationID, "zip")
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
	w.Header().Set("X-PDF-Export-Correlation-Id", correlationID)
	w.WriteHeader(http.StatusOK)
	archive := zip.NewWriter(w)
	for _, item := range items {
		data, document, exportErr := readPDFExport(ctx, snapshot, item.Article, item.Document.ContentHash)
		if exportErr != nil || document.ContentHash != item.Document.ContentHash {
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
func pdfExportAuditEvent(article *work.ExportArticle, document *pdfstore.AvailableDocument, runID int64, correlationID, scope string) (*manifest.AuditEvent, error) {
	eventKey, err := newPDFExportKey()
	if err != nil {
		return nil, err
	}
	after, err := json.Marshal(pdfExportEvidence{
		Status: pdfstore.StatusAvailable, ContentHash: document.ContentHash,
		ByteSize: document.ByteSize, InventoriedAt: document.InventoriedAt,
		ChangeCounter: document.ChangeCounter,
	})
	if err != nil {
		return nil, err
	}
	metadata, err := json.Marshal(map[string]any{
		"source": "frontend_download", "scope": scope, "include_comments": false,
		"initiating_run_id": runID, "initiating_revision_id": article.RevisionID,
		"doi": pdfExportDOI(article), "event_key": eventKey,
	})
	if err != nil {
		return nil, err
	}
	return &manifest.AuditEvent{
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), Actor: "user",
		EntityType: "work", EntityID: strconv.FormatInt(article.WorkID, 10),
		Action: manifest.AuditPDFDocumentExportPrepared, AfterJSON: string(after),
		MetadataJSON: string(metadata), CorrelationID: correlationID,
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
// names include a stable work suffix so equal titles cannot collide.
func pdfExportFilename(article *work.ExportArticle, archive bool) string {
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
	if base == "" {
		return fmt.Sprintf("work-%d.pdf", workID)
	}
	suffix := ""
	if archive {
		suffix = fmt.Sprintf(" (work-%d)", workID)
	}
	base = truncatePDFExportTitle(base, maxPDFExportFilenameBytes-len(suffix)-len(".pdf"))
	base = strings.TrimRight(base, " .")
	if strings.Trim(base, "- .") == "" {
		return fmt.Sprintf("work-%d.pdf", workID)
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

// pdfCommentsUnavailable rejects comment-enabled exports until the annotation phase.
func pdfCommentsUnavailable() error {
	return &viewrequest.Problem{Status: http.StatusBadRequest, Code: "pdf_comments_unavailable", Message: "comment-enabled PDF export is not available in this phase"}
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
