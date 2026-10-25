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
func (s *Server) workPDF(w http.ResponseWriter, r *http.Request) {
	workID, err := viewrequest.PositiveID(r.PathValue("work_id"))
	if err != nil {
		s.respond(w, r, nil, err)
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
		s.respond(w, r, nil, viewrequest.NotFound("PDF is not available"))
		return
	}
	contentHash, inventoriedAt, byteSize := document.ContentHash, document.InventoriedAt, document.ByteSize
	s.pdfCacheMu.Lock()
	var data []byte
	if s.pdfCache != nil && s.pdfCache.WorkID == workID && s.pdfCache.ContentHash == contentHash {
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
			s.respond(w, r, nil, viewrequest.NotFound("PDF content is not available"))
			return
		}
		data = stored
		digest := sha256.Sum256(data)
		if int64(len(data)) != byteSize || len(data) < 5 || string(data[:5]) != "%PDF-" || contentHash != hex.EncodeToString(digest[:]) {
			s.pdfCacheMu.Unlock()
			s.respond(w, r, nil, &viewrequest.Problem{Status: http.StatusUnprocessableEntity, Code: "pdf_integrity_error", Message: "stored PDF content failed integrity validation"})
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
