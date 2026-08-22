// evaluation.go provides the run evaluation endpoint that lists
// normalized articles for a selected run and overlays their PDF
// inventory state from the independently bound companion database.
package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
)

var evaluationSortFields = map[string]string{
	"title": "wr.title",
	"doi":   "w.doi",
}

var evaluationReviewStatuses = map[string]bool{
	"not_evaluated": true, "in_progress": true, "approved": true, "not_approved": true, "removed": true,
}

var evaluationReviewQualifiers = map[string]bool{
	"redacted": true, "unrelated": true, "out_of_scope": true, "duplicate": true,
	"retracted": true, "withdrawn": true, "superseded": true, "predatory_low_quality": true,
	"copyright_licensing": true, "not_peer_reviewed": true,
}

// runEvaluation lists the selected run's normalized articles and overlays
// their state from the independently bound PDF inventory.
func (s *Server) runEvaluation(w http.ResponseWriter, r *http.Request) {
	runID, err := positiveID(r.PathValue("id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	page, perPage, sortField, order, query, err := scopedRowsRequest(r, evaluationSortFields, "title",
		"pdf_status", "review_status", "review_source", "qualifier", "source", "reviewed", "current_revision_id")
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

	contextRecord, err := s.writeDB.Reviews.GetContextByRun(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	var contextID int64
	if contextRecord != nil {
		contextID = contextRecord.ID
	}
	run, err := s.loadReviewRun(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	runWritable := run.Status == "completed" && run.Visibility != "trashed"
	var proposedParent any
	if contextRecord == nil && runWritable {
		proposedParent, err = s.writeDB.Reviews.ProposeParent(ctx, runID)
		if err != nil {
			s.respond(w, r, nil, mapReviewError(err))
			return
		}
	}
	from := `FROM work_revisions wr JOIN works w ON w.id=wr.work_id
		LEFT JOIN review_context_work_heads review_head ON review_head.review_context_id=? AND review_head.work_id=wr.work_id
		LEFT JOIN work_review_versions review ON review.id=review_head.review_version_id`
	clauses := []string{"wr.pipeline_run_id=?", currentNormalizedRevisionPredicate("wr")}
	args := []any{contextID, runID}
	if query != "" {
		clauses = append(clauses, "(LOWER(COALESCE(wr.title, '')) LIKE ? OR LOWER(COALESCE(w.doi, '')) LIKE ?)")
		needle := "%" + strings.ToLower(query) + "%"
		args = append(args, needle, needle)
	}
	if source := strings.TrimSpace(r.URL.Query().Get("source")); source != "" {
		if source == "not_recorded" {
			clauses = append(clauses, "COALESCE(wr.source, '')='' ")
		} else {
			clauses = append(clauses, "wr.source=?")
			args = append(args, source)
		}
	}
	if status := r.URL.Query().Get("review_status"); status != "" {
		if !evaluationReviewStatuses[status] {
			s.respond(w, r, nil, badRequest("review_status is invalid"))
			return
		}
		clauses = append(clauses, "COALESCE(review.status, 'not_evaluated')=?")
		args = append(args, status)
	}
	if qualifier := r.URL.Query().Get("qualifier"); qualifier != "" {
		if !evaluationReviewQualifiers[qualifier] {
			s.respond(w, r, nil, badRequest("qualifier is invalid"))
			return
		}
		clauses = append(clauses, "EXISTS (SELECT 1 FROM work_review_version_substatuses sub WHERE sub.review_version_id=review.id AND sub.sub_status=?)")
		args = append(args, qualifier)
	}
	switch r.URL.Query().Get("review_source") {
	case "":
	case "this_context":
		clauses = append(clauses, "review.id IS NOT NULL AND review.created_in_context_id=?")
		args = append(args, contextID)
	case "inherited":
		clauses = append(clauses, "review.id IS NOT NULL AND review.created_in_context_id!=?")
		args = append(args, contextID)
	case "not_started":
		clauses = append(clauses, "review.id IS NULL")
	default:
		s.respond(w, r, nil, badRequest("review_source is invalid"))
		return
	}
	switch r.URL.Query().Get("reviewed") {
	case "":
	case "reviewed":
		clauses = append(clauses, "review.id IS NOT NULL AND review.status!='not_evaluated'")
	case "unreviewed":
		clauses = append(clauses, "(review.id IS NULL OR review.status='not_evaluated')")
	default:
		s.respond(w, r, nil, badRequest("reviewed is invalid"))
		return
	}
	availableDOIs, err := s.availablePDFDOIs(ctx)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	availableJSON, err := json.Marshal(availableDOIs)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if pdfStatus := r.URL.Query().Get("pdf_status"); pdfStatus != "" {
		if pdfStatus != "available" && pdfStatus != "not_available" {
			s.respond(w, r, nil, badRequest("pdf_status is invalid"))
			return
		}
		if pdfStatus == "available" {
			clauses = append(clauses, "w.doi IN (SELECT value FROM json_each(?))")
		} else {
			clauses = append(clauses, "(w.doi IS NULL OR w.doi NOT IN (SELECT value FROM json_each(?)))")
		}
		args = append(args, string(availableJSON))
	}
	where := strings.Join(clauses, " AND ")
	var currentRevisionID int64
	if raw := r.URL.Query().Get("current_revision_id"); raw != "" {
		currentRevisionID, err = positiveID(raw)
		if err != nil {
			s.respond(w, r, nil, badRequest("current_revision_id must be positive"))
			return
		}
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) "+from+" WHERE "+where, args...).Scan(&total); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	page = clampScopedPage(page, perPage, total)

	queryArgs := append(append([]any(nil), args...), perPage, (page-1)*perPage)
	rows, err := s.db.QueryContext(ctx, `SELECT wr.work_id, wr.id AS work_revision_id,
		wr.title, w.doi, wr.source, COALESCE(review.status, 'not_evaluated') AS review_status,
		CASE WHEN review.id IS NOT NULL AND review.created_in_context_id!=? THEN 1 ELSE 0 END AS review_inherited,
		review.id AS review_version_id, review.created_in_context_id AS review_created_in_context_id,
		COALESCE((SELECT json_group_array(sub_status) FROM (
			SELECT sub_status FROM work_review_version_substatuses WHERE review_version_id=review.id ORDER BY sub_status)), '[]') AS review_sub_statuses
		`+from+` WHERE `+where+` ORDER BY `+evaluationSortFields[sortField]+` `+order+`, wr.id `+order+` LIMIT ? OFFSET ?`,
		append([]any{contextID}, queryArgs...)...)
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
	summary, summaryErr := s.evaluationReviewSummary(ctx, runID, contextID, string(availableJSON))
	if err == nil {
		err = summaryErr
	}
	var navigation map[string]any
	if err == nil {
		navigation, err = s.evaluationQueueNavigation(ctx, runID, currentRevisionID, from, where, args, sortField, order)
	}
	for _, item := range items {
		if inherited, ok := item["review_inherited"].(int64); ok {
			item["review_inherited"] = inherited != 0
		}
		if raw, ok := item["review_sub_statuses"].(string); ok {
			var values []string
			if decodeErr := json.Unmarshal([]byte(raw), &values); decodeErr != nil {
				err = decodeErr
				break
			}
			item["review_sub_statuses"] = values
		} else if raw, ok := item["review_sub_statuses"].([]byte); ok {
			var values []string
			if decodeErr := json.Unmarshal(raw, &values); decodeErr != nil {
				err = decodeErr
				break
			}
			item["review_sub_statuses"] = values
		}
	}
	s.respond(w, r, map[string]any{
		"run_id": runID, "review_context_initialized": contextRecord != nil, "review_context": contextRecord,
		"review_summary": summary, "queue_navigation": navigation, "proposed_parent": proposedParent, "run_writable": runWritable,
		"columns": []string{"title", "doi", "source", "inventory_status", "inventoried_at", "review_status", "review_inherited", "review_sub_statuses"},
		"rows":    items,
		"pagination": scopedPagination(
			page, perPage, total, sortField, order,
		),
	}, err)
}

// evaluationQueueNavigation returns adjacent unreviewed revisions within the active queue filters.
func (s *Server) evaluationQueueNavigation(ctx context.Context, runID, currentRevisionID int64, from, where string, args []any, sortField, order string) (map[string]any, error) {
	unreviewedWhere := where + " AND (review.id IS NULL OR review.status='not_evaluated')"
	sortExpression := "COALESCE(CAST(" + evaluationSortFields[sortField] + " AS TEXT), '')"
	queryID := func(predicate, queryOrder string, extraArgs ...any) (any, error) {
		queryArgs := append(append([]any(nil), args...), extraArgs...)
		var revisionID int64
		err := s.db.QueryRowContext(ctx, `SELECT wr.id `+from+` WHERE `+unreviewedWhere+predicate+`
			ORDER BY `+sortExpression+` `+queryOrder+`, wr.id `+queryOrder+` LIMIT 1`, queryArgs...).Scan(&revisionID)
		if err == sql.ErrNoRows {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return revisionID, nil
	}
	if currentRevisionID == 0 {
		next, err := queryID("", order)
		if err != nil {
			return nil, err
		}
		reverse := "DESC"
		if order == "DESC" {
			reverse = "ASC"
		}
		previous, err := queryID("", reverse)
		return map[string]any{"previous_work_revision_id": previous, "next_work_revision_id": next}, err
	}

	var currentSortValue string
	err := s.db.QueryRowContext(ctx, `SELECT `+sortExpression+` FROM work_revisions wr JOIN works w ON w.id=wr.work_id
		WHERE wr.id=? AND wr.pipeline_run_id=? AND `+currentNormalizedRevisionPredicate("wr"), currentRevisionID, runID).Scan(&currentSortValue)
	if err == sql.ErrNoRows {
		return nil, notFound("current evaluation revision is not part of the selected run")
	}
	if err != nil {
		return nil, err
	}
	previousOperator := "<"
	nextOperator := ">"
	previousOrder := "DESC"
	nextOrder := "ASC"
	if order == "DESC" {
		previousOperator = ">"
		nextOperator = "<"
		previousOrder = "ASC"
		nextOrder = "DESC"
	}
	previousPredicate := " AND (" + sortExpression + previousOperator + "? OR (" + sortExpression + "=? AND wr.id" + previousOperator + "?))"
	nextPredicate := " AND (" + sortExpression + nextOperator + "? OR (" + sortExpression + "=? AND wr.id" + nextOperator + "?))"
	previous, err := queryID(previousPredicate, previousOrder, currentSortValue, currentSortValue, currentRevisionID)
	if err != nil {
		return nil, err
	}
	next, err := queryID(nextPredicate, nextOrder, currentSortValue, currentSortValue, currentRevisionID)
	return map[string]any{"previous_work_revision_id": previous, "next_work_revision_id": next}, err
}

// availablePDFDOIs returns the bounded identity projection used for evaluation inventory filters.
func (s *Server) availablePDFDOIs(ctx context.Context) ([]string, error) {
	if s.pdfDB == nil {
		return []string{}, nil
	}
	rows, err := s.pdfDB.QueryContext(ctx, `SELECT document.doi FROM pdf_documents document
		JOIN pdf_blobs blob ON blob.content_hash=document.content_hash WHERE document.status='available' ORDER BY document.doi`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]string, 0)
	for rows.Next() {
		var doi string
		if err := rows.Scan(&doi); err != nil {
			return nil, err
		}
		items = append(items, doi)
	}
	return items, rows.Err()
}

// evaluationReviewSummary returns invariant queue progress independently of page rows and filters.
func (s *Server) evaluationReviewSummary(ctx context.Context, runID, contextID int64, availableDOIsJSON string) (map[string]any, error) {
	var total, reviewed, unreviewed, availablePDFs int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*),
		COALESCE(SUM(CASE WHEN review.id IS NOT NULL AND review.status!='not_evaluated' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN review.id IS NULL OR review.status='not_evaluated' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN work.doi IN (SELECT value FROM json_each(?)) THEN 1 ELSE 0 END),0)
		FROM work_revisions revision
		JOIN works work ON work.id=revision.work_id
		LEFT JOIN review_context_work_heads head ON head.review_context_id=? AND head.work_id=revision.work_id
		LEFT JOIN work_review_versions review ON review.id=head.review_version_id
		WHERE revision.pipeline_run_id=? AND `+currentNormalizedRevisionPredicate("revision"), availableDOIsJSON, contextID, runID).
		Scan(&total, &reviewed, &unreviewed, &availablePDFs)
	if err != nil {
		return nil, err
	}
	baseFrom := `FROM work_revisions revision JOIN works work ON work.id=revision.work_id
		LEFT JOIN review_context_work_heads head ON head.review_context_id=? AND head.work_id=revision.work_id
		LEFT JOIN work_review_versions review ON review.id=head.review_version_id
		WHERE revision.pipeline_run_id=? AND ` + currentNormalizedRevisionPredicate("revision")
	statusFacets, err := s.evaluationFacet(ctx, `SELECT COALESCE(review.status, 'not_evaluated') AS value, COUNT(*) AS count `+baseFrom+`
		GROUP BY COALESCE(review.status, 'not_evaluated') ORDER BY value`, contextID, runID)
	if err != nil {
		return nil, err
	}
	sourceFacets, err := s.evaluationFacet(ctx, `SELECT COALESCE(NULLIF(revision.source, ''), 'not_recorded') AS value, COUNT(*) AS count `+baseFrom+`
		GROUP BY COALESCE(NULLIF(revision.source, ''), 'not_recorded') ORDER BY value`, contextID, runID)
	if err != nil {
		return nil, err
	}
	reviewSourceFacets, err := s.evaluationFacet(ctx, `SELECT CASE
		WHEN review.id IS NULL THEN 'not_started'
		WHEN review.created_in_context_id=? THEN 'this_context'
		ELSE 'inherited' END AS value, COUNT(*) AS count `+baseFrom+`
		GROUP BY value ORDER BY value`, contextID, contextID, runID)
	if err != nil {
		return nil, err
	}
	qualifierFacets, err := s.evaluationFacet(ctx, `SELECT sub.sub_status AS value, COUNT(*) AS count
		FROM work_revisions revision
		JOIN review_context_work_heads head ON head.review_context_id=? AND head.work_id=revision.work_id
		JOIN work_review_version_substatuses sub ON sub.review_version_id=head.review_version_id
		WHERE revision.pipeline_run_id=? AND `+currentNormalizedRevisionPredicate("revision")+`
		GROUP BY sub.sub_status ORDER BY sub.sub_status`, contextID, runID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"total": total, "reviewed": reviewed, "unreviewed": unreviewed,
		"pdf_available": availablePDFs, "pdf_not_available": total - availablePDFs,
		"percent_reviewed": percent(reviewed, total),
		"facets": map[string]any{
			"review_status": statusFacets, "source": sourceFacets, "review_source": reviewSourceFacets,
			"qualifier":  qualifierFacets,
			"pdf_status": []map[string]any{{"value": "available", "count": availablePDFs}, {"value": "not_available", "count": total - availablePDFs}},
		},
	}, nil
}

// evaluationFacet executes one bounded aggregate projection for queue filter choices.
func (s *Server) evaluationFacet(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return rowsAsMaps(rows)
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
