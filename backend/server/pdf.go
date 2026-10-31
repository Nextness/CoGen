// pdf.go provides the PDF status and content endpoints that report
// the inventory state (available, not_available) and serve stored
// PDF bytes for a given work revision.
package server

import (
	viewrequest "analysis/backend/server/request"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"analysis/backend/pdfstore"
)

// cachedPDF retains one validated companion document for repeated browser range requests.
type cachedPDF struct {
	WorkID        int64
	ContentHash   string
	InventoriedAt string
	Data          []byte
}

// workPDFStatus returns normalized DOI inventory status for the requested work.
func (s *Server) workPDFStatus(w http.ResponseWriter, r *http.Request) {
	setMutableResponseHeaders(w)
	workID, err := viewrequest.PositiveID(r.PathValue("work_id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	status, err := s.pdfStatusForWork(ctx, workID)
	s.respond(w, r, status, err)
}

// pdfStatusForWork reads companion PDF inventory and mutation capability for
// one work. The change counter and opaque ETag are exposed separately from the
// content-hash byte ETag used by evidence reads.
func (s *Server) pdfStatusForWork(ctx context.Context, workID int64) (map[string]any, error) {
	work, err := s.workStore.GetWorkByID(ctx, workID)
	if err != nil {
		return nil, err
	}
	if work == nil {
		return nil, viewrequest.NotFound("work not found")
	}
	if work.DOI == "" {
		status := s.pdfStatusFromDocument(workID, "", nil, 0)
		status["status"] = "unavailable"
		return status, nil
	}
	if s.pdfDB == nil || s.pdfStore == nil {
		return s.pdfStatusFromDocument(workID, work.DOI, nil, 0), nil
	}
	document, err := s.pdfStore.Document(ctx, work.DOI)
	if err != nil {
		return nil, err
	}
	if document == nil {
		return s.pdfStatusFromDocument(workID, work.DOI, nil, 0), nil
	}
	status := s.pdfStatusFromDocument(workID, work.DOI, document, 0)
	if document.Status == pdfstore.StatusAvailable {
		available, err := s.pdfStore.AvailableDocument(ctx, work.DOI)
		if err != nil {
			return nil, err
		}
		if available != nil {
			status["content_hash"] = available.ContentHash
			status["inventoried_at"] = available.InventoriedAt
			status["byte_size"] = available.ByteSize
		}
	}
	return status, nil
}

// pdfStatusForArticle adds exact selected-run mutation availability without
// requiring review-context initialization.
func (s *Server) pdfStatusForArticle(run reviewRunRecord, workID int64, status map[string]any) map[string]any {
	contentHash, _ := status["content_hash"].(string)
	doi, _ := status["doi"].(string)
	registered, _ := status["registered"].(bool)
	status["actions"] = pdfActions(run, s.pdfWritable && s.pdfWriteDB != nil, registered, fmt.Sprint(status["status"]), contentHash, doi)
	return status
}

// workPDF streams the validated PDF associated with the requested work revision.
// A supplied content_hash pins the response to the bytes the embedded reader
// opened, so later range requests cannot combine two document selections.
func (s *Server) workPDF(w http.ResponseWriter, r *http.Request) {
	workID, err := viewrequest.PositiveID(r.PathValue("work_id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if err := viewrequest.KnownQuery(r, "content_hash"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	pinnedHash := strings.TrimSpace(r.URL.Query().Get("content_hash"))
	if pinnedHash != "" && (len(pinnedHash) != 64 || !lowercaseHex(pinnedHash)) {
		s.respond(w, r, nil, viewrequest.BadRequest("content_hash must be 64 lowercase hexadecimal characters"))
		return
	}
	if s.pdfDB == nil {
		s.respond(w, r, nil, viewrequest.NotFound("PDF store is not configured"))
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	work, err := s.workStore.GetWorkByID(ctx, workID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if work == nil || work.DOI == "" {
		s.respond(w, r, nil, viewrequest.NotFound("work or DOI not found"))
		return
	}
	document, err := s.pdfStore.AvailableDocument(ctx, work.DOI)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if document == nil {
		if pinnedHash != "" {
			s.respond(w, r, nil, s.pdfChanged(ctx, workID, work.DOI, pinnedHash))
			return
		}
		s.respond(w, r, nil, viewrequest.NotFound("PDF is not available"))
		return
	}
	contentHash, inventoriedAt, byteSize := document.ContentHash, document.InventoriedAt, document.ByteSize
	if pinnedHash != "" && pinnedHash != contentHash {
		s.respond(w, r, nil, s.pdfChanged(ctx, workID, work.DOI, pinnedHash))
		return
	}
	s.pdfCacheMu.Lock()
	var data []byte
	if s.pdfCache != nil && s.pdfCache.WorkID == workID && s.pdfCache.ContentHash == contentHash {
		// Recheck the current selection before reusing process-local bytes. This
		// is required because another server process may have committed a change.
		current, err := s.pdfStore.AvailableDocument(ctx, work.DOI)
		if err != nil {
			s.pdfCacheMu.Unlock()
			s.respond(w, r, nil, err)
			return
		}
		if current == nil || current.ContentHash != contentHash {
			s.pdfCacheMu.Unlock()
			s.respond(w, r, nil, s.pdfChanged(ctx, workID, work.DOI, pinnedHash))
			return
		}
		data = s.pdfCache.Data
	} else {
		stored, err := s.pdfStore.BlobData(ctx, contentHash)
		if err != nil {
			s.pdfCacheMu.Unlock()
			s.respond(w, r, nil, err)
			return
		}
		if stored == nil {
			s.pdfCacheMu.Unlock()
			s.respond(w, r, nil, s.pdfReadIntegrityError(ctx, workID, work.DOI, contentHash, pinnedHash))
			return
		}
		data = stored
		digest := sha256.Sum256(data)
		if int64(len(data)) != byteSize || len(data) < 5 || string(data[:5]) != "%PDF-" || contentHash != hex.EncodeToString(digest[:]) {
			s.pdfCacheMu.Unlock()
			s.respond(w, r, nil, s.pdfReadIntegrityError(ctx, workID, work.DOI, contentHash, pinnedHash))
			return
		}
		s.pdfCache = &cachedPDF{WorkID: workID, ContentHash: contentHash, InventoriedAt: inventoriedAt, Data: data}
	}
	s.pdfCacheMu.Unlock()
	modified, _ := time.Parse(time.RFC3339Nano, inventoriedAt)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("ETag", `"`+contentHash+`"`)
	w.Header().Set("Cache-Control", "private, max-age=0, must-revalidate")
	disposition := mime.FormatMediaType("inline", map[string]string{"filename": "work-" + strconv.FormatInt(workID, 10) + ".pdf"})
	w.Header().Set("Content-Disposition", disposition)
	http.ServeContent(w, r, "work-"+strconv.FormatInt(workID, 10)+".pdf", modified, bytes.NewReader(data))
}

// invalidatePDFCache clears one work's process-local document bytes after a
// committed selection change. Other server processes recheck selection before
// cache reuse instead of sharing an invalidation channel.
func (s *Server) invalidatePDFCache(workID int64) {
	s.pdfCacheMu.Lock()
	if s.pdfCache != nil && s.pdfCache.WorkID == workID {
		s.pdfCache = nil
	}
	s.pdfCacheMu.Unlock()
}

// pdfChanged reports a deliberate reader refresh conflict with bounded current state.
func (s *Server) pdfChanged(ctx context.Context, workID int64, doi, pinnedHash string) error {
	document, err := s.pdfStore.Document(ctx, doi)
	if err != nil {
		return err
	}
	return &viewrequest.Problem{
		Status: http.StatusConflict, Code: "pdf_changed",
		Message: "the selected PDF content changed; refresh the reader",
		Details: map[string]any{
			"pinned_content_hash": pinnedHash,
			"current_pdf_status":  s.pdfStatusFromDocument(workID, doi, document, 0),
		},
	}
}

// pdfReadIntegrityError distinguishes a concurrent selection change from
// genuine stored-content corruption after a read-time integrity failure.
func (s *Server) pdfReadIntegrityError(ctx context.Context, workID int64, doi, contentHash, pinnedHash string) error {
	current, err := s.pdfStore.AvailableDocument(ctx, doi)
	if err != nil {
		return err
	}
	if current == nil || current.ContentHash != contentHash {
		return s.pdfChanged(ctx, workID, doi, pinnedHash)
	}
	return &viewrequest.Problem{Status: http.StatusUnprocessableEntity, Code: "pdf_integrity_error", Message: "stored PDF content failed integrity validation"}
}
