// pdf.go provides the PDF status and content endpoints that report
// the inventory state (available, not_available) and serve stored
// PDF bytes for a given work revision.
package server

import (
	"bytes"
	"context"
	"database/sql"
	"mime"
	"net/http"
	"strconv"
	"time"
)

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
	var data []byte
	var inventoriedAt string
	err = s.pdfDB.QueryRowContext(ctx, `SELECT b.data, d.inventoried_at FROM pdf_documents d
		JOIN pdf_blobs b ON b.content_hash=d.content_hash
		WHERE d.doi=? AND d.status='available'`, doi).Scan(&data, &inventoriedAt)
	if err == sql.ErrNoRows {
		s.respond(w, r, nil, notFound("PDF is not available"))
		return
	}
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	modified, _ := time.Parse(time.RFC3339Nano, inventoriedAt)
	w.Header().Set("Content-Type", "application/pdf")
	disposition := mime.FormatMediaType("inline", map[string]string{"filename": "work-" + strconv.FormatInt(workID, 10) + ".pdf"})
	w.Header().Set("Content-Disposition", disposition)
	http.ServeContent(w, r, "work-"+strconv.FormatInt(workID, 10)+".pdf", modified, bytes.NewReader(data))
}
