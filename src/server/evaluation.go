// evaluation.go provides the run evaluation endpoint that lists
// normalized articles for a selected run and overlays their PDF
// inventory state from the independently bound companion database.
package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"analysis/database/work"
)

var evaluationSortFields = runCorpusDefinitions["articles"].sortFields

var evaluationReviewStatuses = map[string]bool{
	"not_evaluated": true,
	"in_progress":   true,
	"approved":      true,
	"not_approved":  true,
	"removed":       true,
}

var evaluationReviewQualifiers = map[string]bool{
	"redacted":              true,
	"unrelated":             true,
	"out_of_scope":          true,
	"duplicate":             true,
	"retracted":             true,
	"withdrawn":             true,
	"superseded":            true,
	"predatory_low_quality": true,
	"copyright_licensing":   true,
	"not_peer_reviewed":     true,
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
	hasPlan, err := s.runStore.HasExecutionPlan(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	runWritable := run.Status == "completed" && run.Visibility != "trashed" && hasPlan
	var proposedParent any
	if contextRecord == nil && runWritable {
		proposedParent, err = s.writeDB.Reviews.ProposeParent(ctx, runID)
		if err != nil {
			s.respond(w, r, nil, mapReviewError(err))
			return
		}
	}

	source := strings.TrimSpace(r.URL.Query().Get("source"))
	reviewStatus := r.URL.Query().Get("review_status")
	if reviewStatus != "" && !evaluationReviewStatuses[reviewStatus] {
		s.respond(w, r, nil, badRequest("review_status is invalid"))
		return
	}
	qualifier := r.URL.Query().Get("qualifier")
	if qualifier != "" && !evaluationReviewQualifiers[qualifier] {
		s.respond(w, r, nil, badRequest("qualifier is invalid"))
		return
	}
	reviewSource := r.URL.Query().Get("review_source")
	switch reviewSource {
	case "", "this_context", "inherited", "not_started":
	default:
		s.respond(w, r, nil, badRequest("review_source is invalid"))
		return
	}
	reviewed := r.URL.Query().Get("reviewed")
	switch reviewed {
	case "", "reviewed", "unreviewed":
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
	pdfStatus := r.URL.Query().Get("pdf_status")
	if pdfStatus != "" && pdfStatus != "available" && pdfStatus != "not_available" {
		s.respond(w, r, nil, badRequest("pdf_status is invalid"))
		return
	}
	var currentRevisionID int64
	if raw := r.URL.Query().Get("current_revision_id"); raw != "" {
		currentRevisionID, err = positiveID(raw)
		if err != nil {
			s.respond(w, r, nil, badRequest("current_revision_id must be positive"))
			return
		}
	}

	filter := work.EvaluationFilter{
		RunID: runID, ContextID: contextID, CurrentRevisionID: currentRevisionID,
		Query: query, Source: source, ReviewStatus: reviewStatus, Qualifier: qualifier,
		ReviewSource: reviewSource, Reviewed: reviewed, PDFStatus: pdfStatus,
		AvailableDOIsJSON: string(availableJSON), SortField: sortField, Order: order,
		Page: page, PerPage: perPage,
	}
	result, err := s.workStore.ListEvaluation(ctx, filter)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	items := make([]map[string]any, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, evaluationRow(item))
	}
	if err := s.overlayPDFInventory(ctx, items); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if err := s.attachArticleTermMatches(ctx, runID, items); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	summary, err := s.evaluationReviewSummary(ctx, filter)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	navigation, err := s.evaluationQueueNavigation(ctx, filter)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	page = clampScopedPage(page, perPage, result.Total)
	s.respond(w, r, map[string]any{
		"run_id": runID, "review_context_initialized": contextRecord != nil, "review_context": contextRecord,
		"review_summary": summary, "queue_navigation": navigation, "proposed_parent": proposedParent, "run_writable": runWritable,
		"collection": "articles",
		"columns":    append(append([]string(nil), runCorpusDefinitions["articles"].columns...), "inventory_status", "inventoried_at", "review_status", "review_inherited", "review_sub_statuses"),
		"rows":       items,
		"pagination": scopedPagination(page, perPage, result.Total, sortField, order),
	}, nil)
}

// evaluationQueueNavigation returns adjacent unreviewed revisions within the active queue filters.
func (s *Server) evaluationQueueNavigation(ctx context.Context, filter work.EvaluationFilter) (map[string]any, error) {
	navigation, err := s.workStore.EvaluationQueueNavigation(ctx, filter)
	if err != nil {
		if errors.Is(err, work.ErrEvaluationRevisionNotFound) {
			return nil, notFound("current evaluation revision is not part of the selected run")
		}
		return nil, err
	}
	result := map[string]any{"previous_work_revision_id": nil, "next_work_revision_id": nil}
	if navigation.PreviousWorkRevisionID != nil {
		result["previous_work_revision_id"] = *navigation.PreviousWorkRevisionID
	}
	if navigation.NextWorkRevisionID != nil {
		result["next_work_revision_id"] = *navigation.NextWorkRevisionID
	}
	return result, nil
}

// availablePDFDOIs returns the bounded identity projection used for evaluation inventory filters.
func (s *Server) availablePDFDOIs(ctx context.Context) ([]string, error) {
	if s.pdfDB == nil {
		return []string{}, nil
	}
	return s.pdfStore.AvailablePDFDOIs(ctx)
}

// evaluationReviewSummary returns invariant queue progress independently of page rows and filters.
func (s *Server) evaluationReviewSummary(ctx context.Context, filter work.EvaluationFilter) (map[string]any, error) {
	summary, err := s.workStore.EvaluationReviewSummary(ctx, filter)
	if err != nil {
		return nil, err
	}
	return evaluationSummaryPayload(summary), nil
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
