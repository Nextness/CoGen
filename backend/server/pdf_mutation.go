// pdf_mutation.go provides the bounded, conditional PDF upload and removal
// boundary. It resolves the selected run and normalized article revision,
// validates transport and mutation preconditions before retaining a body, and
// delegates all storage effects to the companion store's S03 transaction.
package server

import (
	viewrequest "analysis/backend/server/request"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"analysis/backend/pdfstore"
)

const (
	maxPDFUploadBytes         = pdfstore.DefaultMaxPDFBytes
	pdfUploadConcurrency      = 2
	pdfUploadBodyDeadline     = 60 * time.Second
	pdfUploadResponseDeadline = 75 * time.Second
)

// saveArticlePDF accepts one complete raw PDF body for an exact selected run
// and current normalized article revision.
func (s *Server) saveArticlePDF(w http.ResponseWriter, r *http.Request) {
	setMutableResponseHeaders(w)
	if err := validateSameOrigin(r); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if err := viewrequest.KnownQuery(r); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if err := requirePDFMediaType(r); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	declaredSize, declaredDigest, err := pdfUploadHeaders(r)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if r.ContentLength > maxPDFUploadBytes {
		s.respond(w, r, nil, requestTooLargePDF(maxPDFUploadBytes))
		return
	}
	runID, revisionID, err := pdfMutationIDs(r)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	run, workID, doi, document, err := s.pdfMutationTarget(ctx, runID, revisionID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	expectedCounter, err := pdfExpectedCounter(r, workID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if expectedCounter != document.ChangeCounter {
		s.respond(w, r, nil, s.pdfStateConflict(workID, doi, document, expectedCounter))
		return
	}
	if !s.acquirePDFUpload() {
		w.Header().Set("Retry-After", "1")
		s.respond(w, r, nil, pdfCapacityExhausted())
		return
	}
	defer s.releasePDFUpload()
	data, err := readPDFUploadBody(w, r, declaredSize, declaredDigest)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	writeCtx, writeCancel := queryContext(r)
	defer writeCancel()
	result, err := s.pdfWriteStore.Save(writeCtx, pdfstore.SaveInput{
		DOI: doi, WorkID: workID, ExpectedChangeCounter: expectedCounter,
		InitiatingRunID: runID, InitiatingRevisionID: revisionID, Data: data,
	})
	if err != nil {
		s.respond(w, r, nil, s.pdfMutationError(err, workID, doi))
		return
	}
	// The storage transaction has committed. A response lost after this point
	// is an uncertain outcome; the client must reconcile pdf-status instead of
	// retrying with a now-stale mutation ETag.
	status := s.pdfStatusFromDocument(workID, doi, &result.Document, result.ByteSize)
	status["actions"] = pdfActions(run, true, true, result.Document.Status, result.Document.ContentHash, doi)
	writeJSON(w, http.StatusOK, pdfMutationPayload(result.Changed, result.Operation, status, result.EventKey))
}

// removeArticlePDF clears the exact selected run/article PDF selection after
// validating the mutation precondition and an empty request body.
func (s *Server) removeArticlePDF(w http.ResponseWriter, r *http.Request) {
	setMutableResponseHeaders(w)
	if err := validateSameOrigin(r); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if err := viewrequest.KnownQuery(r); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if err := requireEmptyBody(r); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	runID, revisionID, err := pdfMutationIDs(r)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	run, workID, doi, document, err := s.pdfMutationTarget(ctx, runID, revisionID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	expectedCounter, err := pdfExpectedCounter(r, workID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if expectedCounter != document.ChangeCounter {
		s.respond(w, r, nil, s.pdfStateConflict(workID, doi, document, expectedCounter))
		return
	}
	writeCtx, writeCancel := queryContext(r)
	defer writeCancel()
	result, err := s.pdfWriteStore.Remove(writeCtx, pdfstore.RemoveInput{
		DOI: doi, WorkID: workID, ExpectedChangeCounter: expectedCounter,
		InitiatingRunID: runID, InitiatingRevisionID: revisionID,
	})
	if err != nil {
		s.respond(w, r, nil, s.pdfMutationError(err, workID, doi))
		return
	}
	// The removal has committed. A lost response is reconciled from pdf-status
	// before another conditional removal is attempted.
	status := s.pdfStatusFromDocument(workID, doi, &result.Document, 0)
	status["actions"] = pdfActions(run, true, true, result.Document.Status, result.Document.ContentHash, doi)
	writeJSON(w, http.StatusOK, pdfMutationPayload(result.Changed, result.Operation, status, result.EventKey))
}

// acquirePDFUpload reserves one of the server's two retained upload bodies.
func (s *Server) acquirePDFUpload() bool {
	if s.pdfUploads == nil {
		return false
	}
	select {
	case s.pdfUploads <- struct{}{}:
		return true
	default:
		return false
	}
}

// releasePDFUpload releases one retained upload body on every handler exit path.
func (s *Server) releasePDFUpload() {
	select {
	case <-s.pdfUploads:
	default:
	}
}

// withPDFUploadDeadlines applies finite body and response deadlines before the
// JSON response buffer, which does not expose the underlying response writer.
func withPDFUploadDeadlines(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isPDFUploadRequest(r) {
			next.ServeHTTP(w, r)
			return
		}
		controller := http.NewResponseController(w)
		now := time.Now()
		_ = controller.SetReadDeadline(now.Add(pdfUploadBodyDeadline))
		_ = controller.SetWriteDeadline(now.Add(pdfUploadResponseDeadline))
		ctx, cancel := context.WithTimeout(r.Context(), pdfUploadResponseDeadline)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// isPDFUploadRequest selects only the raw PUT resource that retains a body.
func isPDFUploadRequest(r *http.Request) bool {
	return r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/api/runs/") && strings.HasSuffix(r.URL.Path, "/pdf")
}

// pdfMutationIDs parses the exact run and work-revision route identifiers.
func pdfMutationIDs(r *http.Request) (int64, int64, error) {
	runID, err := viewrequest.PositiveID(r.PathValue("run_id"))
	if err != nil {
		return 0, 0, err
	}
	revisionID, err := viewrequest.PositiveID(r.PathValue("work_revision_id"))
	return runID, revisionID, err
}

// pdfMutationTarget resolves the selected run lifecycle, exact normalized
// revision membership, stable DOI, and current companion inventory state.
func (s *Server) pdfMutationTarget(ctx context.Context, runID, revisionID int64) (reviewRunRecord, int64, string, *pdfstore.Document, error) {
	run, err := s.loadReviewRun(ctx, runID)
	if err != nil {
		return reviewRunRecord{}, 0, "", nil, err
	}
	if run.Status != "completed" || run.Visibility == "trashed" {
		return reviewRunRecord{}, 0, "", nil, &viewrequest.Problem{
			Status: http.StatusConflict, Code: "run_not_mutable",
			Message: "only completed active runs accept PDF changes",
		}
	}
	workID, err := s.workStore.GetCurrentNormalizedRevisionWorkID(ctx, revisionID, runID)
	if errors.Is(err, sql.ErrNoRows) {
		return reviewRunRecord{}, 0, "", nil, viewrequest.NotFound("article revision does not belong to selected run")
	}
	if err != nil {
		return reviewRunRecord{}, 0, "", nil, err
	}
	work, err := s.workStore.GetWorkByID(ctx, workID)
	if err != nil {
		return reviewRunRecord{}, 0, "", nil, err
	}
	if work == nil {
		return reviewRunRecord{}, 0, "", nil, viewrequest.NotFound("work not found")
	}
	if work.DOI == "" {
		return reviewRunRecord{}, 0, "", nil, &viewrequest.Problem{
			Status: http.StatusConflict, Code: "pdf_doi_missing",
			Message: "the selected work has no normalized DOI for PDF inventory",
		}
	}
	if s.pdfStore == nil || s.pdfWriteStore == nil {
		return reviewRunRecord{}, 0, "", nil, s.pdfStoreUnavailable()
	}
	document, err := s.pdfStore.Document(ctx, work.DOI)
	if err != nil {
		return reviewRunRecord{}, 0, "", nil, err
	}
	if document == nil {
		return reviewRunRecord{}, 0, "", nil, &viewrequest.Problem{
			Status: http.StatusConflict, Code: "pdf_inventory_missing",
			Message: "the selected work has no normalized PDF inventory registration",
		}
	}
	return run, workID, work.DOI, document, nil
}

// pdfUploadHeaders validates the required declared size and lowercase SHA-256.
func pdfUploadHeaders(r *http.Request) (int64, string, error) {
	rawSize := strings.TrimSpace(r.Header.Get("X-PDF-Size"))
	if rawSize == "" {
		return 0, "", viewrequest.BadRequest("X-PDF-Size is required")
	}
	size, err := strconv.ParseInt(rawSize, 10, 64)
	if err != nil || size < 1 {
		return 0, "", viewrequest.BadRequest("X-PDF-Size must be a positive decimal byte count")
	}
	if size > maxPDFUploadBytes {
		return 0, "", requestTooLargePDF(maxPDFUploadBytes)
	}
	digest := strings.TrimSpace(r.Header.Get("X-PDF-SHA256"))
	if len(digest) != 64 || !lowercaseHex(digest) {
		return 0, "", viewrequest.BadRequest("X-PDF-SHA256 must be 64 lowercase hexadecimal characters")
	}
	if r.ContentLength >= 0 && r.ContentLength != size {
		return 0, "", pdfUploadMismatch("declared Content-Length does not match X-PDF-Size")
	}
	return size, digest, nil
}

// readPDFUploadBody retains one complete bounded body and verifies its
// declared byte count, SHA-256, and PDF signature before storage sees it.
func readPDFUploadBody(w http.ResponseWriter, r *http.Request, declaredSize int64, declaredDigest string) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPDFUploadBytes)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, requestTooLargePDF(maxPDFUploadBytes)
		}
		var timeout net.Error
		if (errors.As(err, &timeout) && timeout.Timeout()) || errors.Is(r.Context().Err(), context.DeadlineExceeded) {
			return nil, &viewrequest.Problem{Status: http.StatusRequestTimeout, Code: "request_timeout", Message: "PDF upload did not finish within the body deadline"}
		}
		return nil, pdfUploadMismatch("upload body ended before the complete declared PDF was received")
	}
	if int64(len(data)) != declaredSize {
		return nil, pdfUploadMismatch("received byte count does not match X-PDF-Size")
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != declaredDigest {
		return nil, pdfUploadMismatch("received SHA-256 does not match X-PDF-SHA256")
	}
	if _, err := pdfstore.ValidatePDF(data, maxPDFUploadBytes); err != nil {
		return nil, &viewrequest.Problem{
			Status: http.StatusUnprocessableEntity, Code: "pdf_validation_error",
			Message: "uploaded bytes are not a valid PDF file envelope",
		}
	}
	return data, nil
}

// pdfExpectedCounter parses one strong mutation ETag for the exact work.
func pdfExpectedCounter(r *http.Request, workID int64) (int64, error) {
	raw := strings.TrimSpace(r.Header.Get("If-Match"))
	if raw == "" {
		return 0, &viewrequest.Problem{Status: http.StatusPreconditionRequired, Code: "pdf_precondition_required", Message: "If-Match is required for PDF changes"}
	}
	if raw == "*" || strings.HasPrefix(raw, "W/") || strings.Contains(raw, ",") {
		return 0, viewrequest.BadRequest("If-Match must contain one strong PDF mutation ETag")
	}
	if len(raw) < 2 || raw[0] != '"' || raw[len(raw)-1] != '"' || strings.Contains(raw[1:len(raw)-1], `"`) {
		return 0, viewrequest.BadRequest("If-Match is not a valid PDF mutation ETag")
	}
	prefix := fmt.Sprintf("pdf-%d-", workID)
	value := raw[1 : len(raw)-1]
	if !strings.HasPrefix(value, prefix) {
		return 0, viewrequest.BadRequest("If-Match does not identify the selected work")
	}
	counter, err := strconv.ParseInt(strings.TrimPrefix(value, prefix), 16, 64)
	if err != nil || counter < 1 {
		return 0, viewrequest.BadRequest("If-Match is not a valid PDF mutation ETag")
	}
	return counter, nil
}

// pdfMutationETag formats one opaque strong mutation precondition.
func pdfMutationETag(workID, changeCounter int64) string {
	return fmt.Sprintf("\"pdf-%d-%x\"", workID, changeCounter)
}

// pdfStatusFromDocument builds the current bounded PDF state without querying
// again after a committed mutation.
func (s *Server) pdfStatusFromDocument(workID int64, doi string, document *pdfstore.Document, byteSize int64) map[string]any {
	status := map[string]any{
		"work_id":          workID,
		"doi":              doi,
		"eligible":         doi != "",
		"store_bound":      s.pdfDB != nil,
		"registered":       document != nil,
		"change_counter":   int64(0),
		"mutation_etag":    nil,
		"max_upload_bytes": int64(maxPDFUploadBytes),
		"storage_writable": s.pdfWritable && s.pdfWriteDB != nil,
		"storage_reason":   s.pdfWriteReason,
	}
	if writable := s.pdfWritable && s.pdfWriteDB != nil; writable {
		status["storage_reason"] = ""
	}
	if document == nil {
		status["status"] = pdfstore.StatusNotAvailable
		return status
	}
	status["status"] = document.Status
	status["change_counter"] = document.ChangeCounter
	status["mutation_etag"] = pdfMutationETag(workID, document.ChangeCounter)
	if document.Status == pdfstore.StatusAvailable {
		status["content_hash"] = document.ContentHash
		if document.InventoriedAt != "" {
			status["inventoried_at"] = document.InventoriedAt
		}
		if byteSize > 0 {
			status["byte_size"] = byteSize
		}
	}
	return status
}

// pdfActions reports mutation availability for one exact selected run/article.
func pdfActions(run reviewRunRecord, writable, registered bool, status, contentHash, doi string) map[string]any {
	actions := map[string]any{"upload": false, "replace": false, "remove": false, "download": false, "reason": ""}
	if status == pdfstore.StatusAvailable {
		actions["download"] = contentHash != ""
	}
	switch {
	case doi == "":
		actions["reason"] = "pdf_doi_missing"
	case run.Status != "completed":
		actions["reason"] = "run_not_completed"
	case run.Visibility == "trashed":
		actions["reason"] = "run_trashed"
	case !writable:
		actions["reason"] = "pdf_store_unavailable"
	case !registered:
		actions["reason"] = "pdf_inventory_missing"
	case status == pdfstore.StatusAvailable:
		actions["replace"] = true
		actions["remove"] = true
	case status == pdfstore.StatusNotAvailable:
		actions["upload"] = true
	default:
		actions["reason"] = "pdf_status_unavailable"
	}
	return actions
}

// pdfMutationPayload returns the stable successful mutation envelope.
func pdfMutationPayload(changed bool, operation pdfstore.MutationOperation, status map[string]any, eventKey string) map[string]any {
	delivery := "not_applicable"
	var key any
	if changed {
		delivery = "pending"
		if eventKey != "" {
			key = eventKey
		}
	}
	return map[string]any{
		"changed":        changed,
		"operation":      string(operation),
		"pdf_status":     status,
		"event_key":      key,
		"audit_delivery": delivery,
	}
}

// pdfMutationError maps storage failures to stable client-visible problems.
func (s *Server) pdfMutationError(err error, workID int64, doi string) error {
	var conflict *pdfstore.StateConflictError
	switch {
	case errors.As(err, &conflict):
		return s.pdfStateConflict(workID, doi, &conflict.State, conflict.Expected)
	case errors.Is(err, pdfstore.ErrPDFNotRegistered):
		return &viewrequest.Problem{Status: http.StatusConflict, Code: "pdf_inventory_missing", Message: "the selected work has no normalized PDF inventory registration"}
	case errors.Is(err, pdfstore.ErrPDFStateConflict):
		return &viewrequest.Problem{Status: http.StatusPreconditionFailed, Code: "pdf_state_conflict", Message: "the PDF selection changed; refresh and retry"}
	case errors.Is(err, pdfstore.ErrPDFIntegrity):
		return &viewrequest.Problem{Status: http.StatusUnprocessableEntity, Code: "pdf_integrity_error", Message: "stored PDF content failed integrity validation"}
	case errors.Is(err, pdfstore.ErrPDFInvalid):
		return &viewrequest.Problem{Status: http.StatusUnprocessableEntity, Code: "pdf_validation_error", Message: "uploaded bytes are not a valid PDF file envelope"}
	case isPDFStoreBusy(err):
		return &viewrequest.Problem{Status: http.StatusServiceUnavailable, Code: "pdf_store_busy", Message: "the PDF store is temporarily busy; retry shortly", Details: map[string]any{"retry_after_seconds": 1}}
	case errors.Is(err, context.DeadlineExceeded):
		return &viewrequest.Problem{Status: http.StatusServiceUnavailable, Code: "pdf_store_timeout", Message: "the PDF store operation did not finish in time; retry shortly", Details: map[string]any{"retry_after_seconds": 1}}
	default:
		log.Error("PDF mutation failed", "work_id", workID, "error", err)
		return &viewrequest.Problem{Status: http.StatusInternalServerError, Code: "pdf_storage_error", Message: "unable to store the PDF change"}
	}
}

// pdfStateConflict returns the stale-precondition envelope with bounded current state.
func (s *Server) pdfStateConflict(workID int64, doi string, document *pdfstore.Document, expected int64) error {
	status := s.pdfStatusFromDocument(workID, doi, document, 0)
	return &viewrequest.Problem{
		Status: http.StatusPreconditionFailed, Code: "pdf_state_conflict",
		Message: "the PDF selection changed; refresh and retry",
		Details: map[string]any{
			"expected_change_counter": expected,
			"current_pdf_status":      status,
		},
	}
}

// pdfStoreUnavailable returns the safe capability failure for mutation routes.
func (s *Server) pdfStoreUnavailable() error {
	reason := s.pdfWriteReason
	if reason == "" {
		reason = "PDF store is not writable"
	}
	return &viewrequest.Problem{
		Status: http.StatusServiceUnavailable, Code: "pdf_store_unavailable",
		Message: "PDF changes are unavailable: " + reason,
		Details: map[string]any{"reason": reason},
	}
}

// pdfCapacityExhausted returns the bounded upload admission failure.
func pdfCapacityExhausted() error {
	return &viewrequest.Problem{
		Status: http.StatusServiceUnavailable, Code: "pdf_capacity_exhausted",
		Message: "two PDF uploads are already in progress; retry shortly",
		Details: map[string]any{"retry_after_seconds": 1},
	}
}

// requestTooLargePDF returns the bounded upload-size failure.
func requestTooLargePDF(limit int) error {
	return &viewrequest.Problem{
		Status: http.StatusRequestEntityTooLarge, Code: "request_too_large",
		Message: fmt.Sprintf("PDF upload exceeds the %d-byte limit", limit),
	}
}

// pdfUploadMismatch returns the completed-upload identity failure.
func pdfUploadMismatch(message string) error {
	return &viewrequest.Problem{Status: http.StatusUnprocessableEntity, Code: "pdf_upload_mismatch", Message: message}
}

// requirePDFMediaType enforces the single raw PDF upload media type.
func requirePDFMediaType(r *http.Request) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/pdf" {
		return &viewrequest.Problem{Status: http.StatusUnsupportedMediaType, Code: "unsupported_media_type", Message: "Content-Type must be application/pdf"}
	}
	return nil
}

// requireEmptyBody rejects any DELETE payload before resolving storage state.
func requireEmptyBody(r *http.Request) error {
	if r.ContentLength > 0 {
		return viewrequest.BadRequest("DELETE body must be empty")
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, 1))
	if err != nil || len(data) != 0 {
		return viewrequest.BadRequest("DELETE body must be empty")
	}
	return nil
}

// validateSameOrigin enforces the local viewer origin for state-changing requests.
func validateSameOrigin(r *http.Request) error {
	if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+r.Host {
		return &viewrequest.Problem{Status: http.StatusForbidden, Code: "origin_rejected", Message: "request Origin must match the local viewer origin"}
	}
	return nil
}

// lowercaseHex reports whether value is exactly lowercase hexadecimal.
func lowercaseHex(value string) bool {
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

// isPDFStoreBusy recognizes bounded SQLite contention without exposing driver details.
func isPDFStoreBusy(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "database is locked") || strings.Contains(message, "sqlite_busy")
}
