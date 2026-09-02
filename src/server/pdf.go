// pdf.go provides the PDF status and content endpoints that report
// the inventory state (available, not_available) and serve stored
// PDF bytes for a given work revision.
package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"mime"
	"net/http"
	"strconv"
	"time"
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
	workID, err := positiveID(r.PathValue("work_id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	status, err := s.pdfStatusForWork(ctx, workID)
	s.respond(w, r, status, err)
}

// pdfStatusForWork reads companion PDF availability metadata for one work revision.
func (s *Server) pdfStatusForWork(ctx context.Context, workID int64) (map[string]any, error) {
	var doi sql.NullString
	if err := s.db.QueryRowContext(ctx, "SELECT doi FROM works WHERE id=?", workID).Scan(&doi); err == sql.ErrNoRows {
		return nil, notFound("work not found")
	} else if err != nil {
		return nil, err
	}
	if !doi.Valid || doi.String == "" {
		return map[string]any{"work_id": workID, "status": "unavailable", "eligible": false, "store_bound": s.pdfDB != nil}, nil
	}
	base := map[string]any{"work_id": workID, "doi": doi.String, "eligible": true, "store_bound": s.pdfDB != nil}
	if s.pdfDB == nil {
		base["status"] = "not_available"
		return base, nil
	}
	var contentHash, inventoriedAt sql.NullString
	var byteSize sql.NullInt64
	err := s.pdfDB.QueryRowContext(ctx, `SELECT d.content_hash,
		d.inventoried_at, b.byte_size
		FROM pdf_documents d JOIN pdf_blobs b ON b.content_hash=d.content_hash
		WHERE d.doi=? AND d.status='available'`, doi.String).Scan(
		&contentHash, &inventoriedAt, &byteSize)
	if err == sql.ErrNoRows {
		base["status"] = "not_available"
		return base, nil
	}
	if err != nil {
		return nil, err
	}
	base["status"] = "available"
	base["content_hash"] = nullableValue(contentHash)
	base["inventoried_at"] = nullableValue(inventoriedAt)
	if byteSize.Valid {
		base["byte_size"] = byteSize.Int64
	} else {
		base["byte_size"] = nil
	}
	return base, nil
}

// nullableValue converts a nullable SQL string to either its value or nil.
func nullableValue(value sql.NullString) any {
	if value.Valid {
		return value.String
	}
	return nil
}

// workPDF streams the validated PDF associated with the requested work revision.
func (s *Server) workPDF(w http.ResponseWriter, r *http.Request) {
	workID, err := positiveID(r.PathValue("work_id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if s.pdfDB == nil {
		s.respond(w, r, nil, notFound("PDF store is not configured"))
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	var doi string
	if err := s.db.QueryRowContext(ctx, "SELECT doi FROM works WHERE id=? AND doi IS NOT NULL", workID).Scan(&doi); err == sql.ErrNoRows {
		s.respond(w, r, nil, notFound("work or DOI not found"))
		return
	} else if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	var contentHash, inventoriedAt string
	var byteSize int64
	err = s.pdfDB.QueryRowContext(ctx, `SELECT d.content_hash, d.inventoried_at, b.byte_size FROM pdf_documents d
		JOIN pdf_blobs b ON b.content_hash=d.content_hash
		WHERE d.doi=? AND d.status='available'`, doi).Scan(&contentHash, &inventoriedAt, &byteSize)
	if err == sql.ErrNoRows {
		s.respond(w, r, nil, notFound("PDF is not available"))
		return
	}
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	s.pdfCacheMu.Lock()
	var data []byte
	if s.pdfCache != nil && s.pdfCache.WorkID == workID && s.pdfCache.ContentHash == contentHash {
		data = s.pdfCache.Data
	} else {
		err = s.pdfDB.QueryRowContext(ctx, "SELECT data FROM pdf_blobs WHERE content_hash=?", contentHash).Scan(&data)
		if err == sql.ErrNoRows {
			s.pdfCacheMu.Unlock()
			s.respond(w, r, nil, notFound("PDF content is not available"))
			return
		}
		if err != nil {
			s.pdfCacheMu.Unlock()
			s.respond(w, r, nil, err)
			return
		}
		digest := sha256.Sum256(data)
		if int64(len(data)) != byteSize || len(data) < 5 || string(data[:5]) != "%PDF-" || contentHash != hex.EncodeToString(digest[:]) {
			s.pdfCacheMu.Unlock()
			s.respond(w, r, nil, &apiProblem{Status: http.StatusUnprocessableEntity, Code: "pdf_integrity_error", Message: "stored PDF content failed integrity validation"})
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
