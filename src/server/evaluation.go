// evaluation.go provides the run evaluation endpoint that lists
// normalized articles for a selected run and overlays their PDF
// inventory state from the independently bound companion database.
package server

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
)

var evaluationSortFields = map[string]string{
	"title": "wr.title",
	"doi":   "w.doi",
}

// runEvaluation lists the selected run's normalized articles and overlays
// their state from the independently bound PDF inventory.
func (s *Server) runEvaluation(w http.ResponseWriter, r *http.Request) {
	runID, err := positiveID(r.PathValue("id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	page, perPage, sortField, order, query, err := scopedRowsRequest(r, evaluationSortFields, "title")
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	if err := s.requireRun(ctx, runID); err != nil {
		s.respond(w, r, nil, err)
		return
	}

	where, args := scopedWhere(
		"wr.pipeline_run_id=? AND wr.producer_stage='normalize'",
		"wr.title, w.doi", runID, query,
	)
	from := "FROM work_revisions wr JOIN works w ON w.id=wr.work_id"
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) "+from+" WHERE "+where, args...).Scan(&total); err != nil {
		s.respond(w, r, nil, err)
		return
	}

	queryArgs := append(args, perPage, (page-1)*perPage)
	rows, err := s.db.QueryContext(ctx, `SELECT wr.work_id, wr.id AS work_revision_id,
		wr.title, w.doi, wr.source `+from+` WHERE `+where+` ORDER BY `+
		evaluationSortFields[sortField]+` `+order+` LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	items, err := rowsAsMaps(rows)
	closeErr := rows.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = s.overlayPDFInventory(ctx, items)
	}
	s.respond(w, r, map[string]any{
		"run_id":  runID,
		"columns": []string{"title", "doi", "source", "inventory_status", "inventoried_at"},
		"rows":    items,
		"pagination": scopedPagination(
			page, perPage, total, sortField, order,
		),
	}, err)
}

// overlayPDFInventory overlays companion PDF availability onto evaluation rows by normalized DOI.
func (s *Server) overlayPDFInventory(ctx context.Context, items []map[string]any) error {
	for _, item := range items {
		item["inventory_status"] = "not_available"
		item["inventoried_at"] = nil
	}
	if s.pdfDB == nil || len(items) == 0 {
		return nil
	}

	dois := make([]any, 0, len(items))
	byDOI := make(map[string]map[string]any, len(items))
	for _, item := range items {
		doi, _ := item["doi"].(string)
		if doi == "" {
			continue
		}
		dois = append(dois, doi)
		byDOI[doi] = item
	}
	if len(dois) == 0 {
		return nil
	}

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(dois)), ",")
	rows, err := s.pdfDB.QueryContext(ctx, `SELECT d.doi, d.inventoried_at
		FROM pdf_documents d
		JOIN pdf_blobs b ON b.content_hash=d.content_hash
		WHERE d.status='available' AND d.doi IN (`+placeholders+`)`, dois...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var doi string
		var inventoriedAt sql.NullString
		if err := rows.Scan(&doi, &inventoriedAt); err != nil {
			return err
		}
		if item := byDOI[doi]; item != nil {
			item["inventory_status"] = "available"
			if inventoriedAt.Valid {
				item["inventoried_at"] = inventoriedAt.String
			}
		}
	}
	return rows.Err()
}
