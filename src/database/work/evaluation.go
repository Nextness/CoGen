// evaluation.go provides the handwritten evaluation relationship reads over the
// generated summary/facet statements and the retained dynamic list and queue
// navigation builders in evaluation_dynamic.go.
package work

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	generated "analysis/database/work/internal/sql"
)

// ErrEvaluationRevisionNotFound reports that the current revision is not part
// of the selected run's analysis-ready queue.
var ErrEvaluationRevisionNotFound = errors.New("current evaluation revision is not part of the selected run")

// EvaluationReviewSummary returns invariant queue progress independently of
// page rows and filters.
func (s *Store) EvaluationReviewSummary(ctx context.Context, filter EvaluationFilter) (*EvaluationSummary, error) {
	row, err := s.queries.EvaluationReviewSummary(ctx, generated.EvaluationReviewSummaryParams{
		AvailableDoisJson: filter.AvailableDOIsJSON,
		ContextID:         filter.ContextID,
		PipelineRunID:     filter.RunID,
	})
	if err != nil {
		return nil, fmt.Errorf("evaluation review summary: %w", err)
	}
	summary := &EvaluationSummary{
		Total:           row.Count,
		Reviewed:        row.Column2,
		Unreviewed:      row.Column3,
		PDFAvailable:    row.Column4,
		PDFNotAvailable: row.Count - row.Column4,
		PercentReviewed: percent(row.Column2, row.Count),
	}
	statusFacets, err := s.queries.EvaluationStatusFacets(ctx, generated.EvaluationStatusFacetsParams{
		ContextID:     filter.ContextID,
		PipelineRunID: filter.RunID,
	})
	if err != nil {
		return nil, fmt.Errorf("evaluation status facets: %w", err)
	}
	sourceFacets, err := s.queries.EvaluationSourceFacets(ctx, generated.EvaluationSourceFacetsParams{
		ContextID:     filter.ContextID,
		PipelineRunID: filter.RunID,
	})
	if err != nil {
		return nil, fmt.Errorf("evaluation source facets: %w", err)
	}
	reviewSourceFacets, err := s.queries.EvaluationReviewSourceFacets(ctx, generated.EvaluationReviewSourceFacetsParams{
		ContextID:     filter.ContextID,
		PipelineRunID: filter.RunID,
	})
	if err != nil {
		return nil, fmt.Errorf("evaluation review source facets: %w", err)
	}
	qualifierFacets, err := s.queries.EvaluationQualifierFacets(ctx, generated.EvaluationQualifierFacetsParams{
		ContextID:     filter.ContextID,
		PipelineRunID: filter.RunID,
	})
	if err != nil {
		return nil, fmt.Errorf("evaluation qualifier facets: %w", err)
	}
	summary.Facets = EvaluationFacets{
		ReviewStatus: facetCountsFromStatus(statusFacets),
		Source:       facetCountsFromSource(sourceFacets),
		ReviewSource: facetCountsFromReviewSource(reviewSourceFacets),
		Qualifier:    facetCountsFromQualifier(qualifierFacets),
		PDFStatus: []*FacetCount{
			{Value: "available", Count: summary.PDFAvailable},
			{Value: "not_available", Count: summary.PDFNotAvailable},
		},
	}
	return summary, nil
}

// ListEvaluation returns one bounded page of analysis-ready articles with their
// review and validation state.
func (s *Store) ListEvaluation(ctx context.Context, filter EvaluationFilter) (*EvaluationPage, error) {
	scope, err := evaluationScope(filter)
	if err != nil {
		return nil, err
	}
	total, err := s.countQuery(ctx, "SELECT COUNT(*) "+scope.from+" WHERE "+scope.where, scope.args)
	if err != nil {
		return nil, fmt.Errorf("count evaluation: %w", err)
	}
	query := "SELECT " + evaluationSelectColumns + `, wr.id AS work_revision_id,
		COALESCE(review.status, 'not_evaluated') AS review_status,
		CASE WHEN review.id IS NOT NULL AND review.created_in_context_id != ? THEN 1 ELSE 0 END AS review_inherited,
		review.id AS review_version_id, review.created_in_context_id AS review_created_in_context_id,
		COALESCE((SELECT json_group_array(sub_status) FROM (
			SELECT sub_status FROM work_review_version_substatuses WHERE review_version_id=review.id ORDER BY sub_status)), '[]') AS review_sub_statuses
		` + scope.from + " WHERE " + scope.where +
		" ORDER BY COALESCE(" + scope.sortExpression + ", '') " + sqlDirection(filter.Order) + ", wr.id " + sqlDirection(filter.Order) + " LIMIT ? OFFSET ?"
	queryArgs := append(append([]any{filter.ContextID}, scope.args...), filter.PerPage, (clampPage(filter.Page, filter.PerPage, total)-1)*filter.PerPage)
	rows, err := s.queryRows(ctx, query, queryArgs)
	if err != nil {
		return nil, fmt.Errorf("list evaluation: %w", err)
	}
	items := make([]*EvaluationRow, 0, len(rows))
	for _, row := range rows {
		item, err := evaluationRowFromMap(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return &EvaluationPage{Items: items, Total: total}, nil
}

// EvaluationQueueNavigation returns adjacent unreviewed revisions within the
// active queue filters.
func (s *Store) EvaluationQueueNavigation(ctx context.Context, filter EvaluationFilter) (*QueueNavigation, error) {
	scope, err := evaluationScope(filter)
	if err != nil {
		return nil, err
	}
	unreviewedWhere := scope.where + " AND (review.id IS NULL OR review.status='not_evaluated')"
	queryID := func(predicate, queryOrder string, extraArgs ...any) (*int64, error) {
		queryArgs := append(append([]any(nil), scope.args...), extraArgs...)
		var revisionID int64
		err := s.db.QueryRowContext(ctx, "SELECT wr.id "+scope.from+" WHERE "+unreviewedWhere+predicate+
			" ORDER BY "+scope.sortExpression+" "+sqlDirection(queryOrder)+", wr.id "+sqlDirection(queryOrder)+" LIMIT 1", queryArgs...).Scan(&revisionID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return &revisionID, nil
	}
	if filter.CurrentRevisionID == 0 {
		next, err := queryID("", filter.Order)
		if err != nil {
			return nil, fmt.Errorf("evaluation queue navigation: %w", err)
		}
		reverse := "DESC"
		if sqlDirection(filter.Order) == "DESC" {
			reverse = "ASC"
		}
		previous, err := queryID("", reverse)
		if err != nil {
			return nil, fmt.Errorf("evaluation queue navigation: %w", err)
		}
		return &QueueNavigation{PreviousWorkRevisionID: previous, NextWorkRevisionID: next}, nil
	}

	var currentSortValue any
	err = s.db.QueryRowContext(ctx, "SELECT "+scope.sortExpression+` FROM work_revisions wr
		JOIN works w ON w.id=wr.work_id
		LEFT JOIN run_work_stages validation ON validation.pipeline_run_id=wr.pipeline_run_id
			AND validation.work_id=wr.work_id AND validation.stage_name='validate'
		WHERE wr.id=? AND wr.pipeline_run_id=? AND `+normalizedRevisionPredicate("wr"),
		filter.CurrentRevisionID, filter.RunID).Scan(&currentSortValue)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrEvaluationRevisionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("evaluation queue navigation: %w", err)
	}
	previousOperator, nextOperator := "<", ">"
	previousOrder, nextOrder := "DESC", "ASC"
	if sqlDirection(filter.Order) == "DESC" {
		previousOperator, nextOperator = ">", "<"
		previousOrder, nextOrder = "ASC", "DESC"
	}
	previousPredicate := " AND (" + scope.sortExpression + previousOperator + "? OR (" + scope.sortExpression + "=? AND wr.id" + previousOperator + "?))"
	nextPredicate := " AND (" + scope.sortExpression + nextOperator + "? OR (" + scope.sortExpression + "=? AND wr.id" + nextOperator + "?))"
	previous, err := queryID(previousPredicate, previousOrder, currentSortValue, currentSortValue, filter.CurrentRevisionID)
	if err != nil {
		return nil, fmt.Errorf("evaluation queue navigation: %w", err)
	}
	next, err := queryID(nextPredicate, nextOrder, currentSortValue, currentSortValue, filter.CurrentRevisionID)
	if err != nil {
		return nil, fmt.Errorf("evaluation queue navigation: %w", err)
	}
	return &QueueNavigation{PreviousWorkRevisionID: previous, NextWorkRevisionID: next}, nil
}

// evaluationRowFromMap maps one dynamic evaluation row into an application row.
func evaluationRowFromMap(row map[string]any) (*EvaluationRow, error) {
	item := &EvaluationRow{
		ID:               int64Value(row["id"]),
		WorkID:           int64Value(row["work_id"]),
		Title:            stringValue(row["title"]),
		Year:             int(int64Value(row["year"])),
		Journal:          stringValue(row["journal"]),
		Publisher:        stringValue(row["publisher"]),
		Source:           stringValue(row["source"]),
		DOI:              stringValue(row["doi"]),
		ValidationStatus: stringValue(row["validation_status"]),
		CitationCount:    int(int64Value(row["citation_count"])),
		ReferenceCount:   int(int64Value(row["reference_count"])),
		ProducerStage:    stringValue(row["producer_stage"]),
		CreatedAt:        stringValue(row["created_at"]),
		Abstract:         stringValue(row["abstract"]),
		Keywords:         stringValue(row["keywords"]),
		KeywordsPlus:     stringValue(row["keywords_plus"]),
		Authors:          stringValue(row["authors"]),
		WorkRevisionID:   int64Value(row["work_revision_id"]),
		ReviewStatus:     stringValue(row["review_status"]),
		ReviewInherited:  int64Value(row["review_inherited"]) != 0,
	}
	if value, ok := row["review_version_id"].(int64); ok {
		item.ReviewVersionID = &value
	}
	if value, ok := row["review_created_in_context_id"].(int64); ok {
		item.ReviewCreatedInContextID = &value
	}
	if raw := stringValue(row["review_sub_statuses"]); raw != "" {
		var values []string
		if err := json.Unmarshal([]byte(raw), &values); err != nil {
			return nil, fmt.Errorf("decode evaluation review sub-statuses: %w", err)
		}
		item.ReviewSubStatuses = values
	}
	return item, nil
}

// facetCountsFromStatus maps generated status facet rows.
func facetCountsFromStatus(rows []generated.EvaluationStatusFacetsRow) []*FacetCount {
	result := make([]*FacetCount, 0, len(rows))
	for _, row := range rows {
		result = append(result, &FacetCount{Value: row.Value, Count: row.Count})
	}
	return result
}

// facetCountsFromSource maps generated source facet rows.
func facetCountsFromSource(rows []generated.EvaluationSourceFacetsRow) []*FacetCount {
	result := make([]*FacetCount, 0, len(rows))
	for _, row := range rows {
		result = append(result, &FacetCount{Value: row.Value, Count: row.Count})
	}
	return result
}

// facetCountsFromReviewSource maps generated review-source facet rows.
func facetCountsFromReviewSource(rows []generated.EvaluationReviewSourceFacetsRow) []*FacetCount {
	result := make([]*FacetCount, 0, len(rows))
	for _, row := range rows {
		result = append(result, &FacetCount{Value: row.Value, Count: row.Count})
	}
	return result
}

// facetCountsFromQualifier maps generated qualifier facet rows.
func facetCountsFromQualifier(rows []generated.EvaluationQualifierFacetsRow) []*FacetCount {
	result := make([]*FacetCount, 0, len(rows))
	for _, row := range rows {
		result = append(result, &FacetCount{Value: row.Value, Count: row.Count})
	}
	return result
}

// percent returns value as a percentage of denominator, or nil when denominator is zero.
func percent(value, denominator int64) *float64 {
	if denominator == 0 {
		return nil
	}
	result := float64(value) * 100 / float64(denominator)
	return &result
}
