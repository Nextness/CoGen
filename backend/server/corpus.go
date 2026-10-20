// corpus.go provides the run-scoped corpus endpoint that returns
// paginated work revisions for the selected pipeline run, with
// support for arbitrary schema-discovered table browsing.
package server

import (
	viewrequest "analysis/backend/server/request"
	"analysis/internal/jsonvalue"
	"context"
	"net/http"

	"analysis/backend/database/source"
	"analysis/backend/database/work"
)

// scopedRowsDefinition defines the safe projection and sorting for one corpus section.
type scopedRowsDefinition struct {
	columns    []string
	sortFields map[string]string
}

var runCorpusDefinitions = map[string]scopedRowsDefinition{
	"articles": {
		columns: []string{"id", "work_id", "title", "year", "journal", "publisher", "source", "doi", "validation_status", "citation_count", "reference_count", "producer_stage", "created_at", "abstract", "keywords", "keywords_plus", "authors"},
		sortFields: map[string]string{
			"id": "wr.id", "title": "wr.title", "year": "wr.year", "journal": "wr.journal", "publisher": "wr.publisher", "source": "wr.source", "doi": "w.doi", "validation_status": "validation.outcome", "citation_count": "wr.citation_count", "reference_count": "wr.reference_count", "created_at": "wr.created_at",
		},
	},
	"authors": {
		columns: []string{"id", "citation_name", "first_name", "last_name", "orcid", "person_id", "article_count", "affiliation_count", "created_at"},
		sortFields: map[string]string{
			"id": "ao.id", "citation_name": "ao.citation_name", "first_name": "ao.first_name", "last_name": "ao.last_name", "orcid": "ao.orcid", "article_count": "article_count", "affiliation_count": "affiliation_count", "created_at": "ao.created_at",
		},
	},
	"references": {
		columns: []string{"id", "work_revision_id", "mention_order", "doi", "title", "author", "year", "source", "resolved_work_id", "citing_title", "created_at"},
		sortFields: map[string]string{
			"id": "rm.id", "work_revision_id": "rm.work_revision_id", "mention_order": "rm.mention_order", "doi": "rm.doi", "title": "rm.title", "author": "rm.author", "year": "rm.year", "source": "rm.source", "resolved_work_id": "rm.resolved_work_id", "created_at": "rm.created_at",
		},
	},
	"sources": {
		columns: []string{"id", "run_source_id", "source_name", "source_type", "record_index", "parse_status", "reject_reason", "content_hash", "created_at"},
		sortFields: map[string]string{
			"id": "sr.id", "run_source_id": "sr.run_source_id", "source_name": "rs.source_name", "source_type": "rs.source_type", "record_index": "sr.record_index", "parse_status": "sr.parse_status", "reject_reason": "sr.reject_reason", "content_hash": "sr.content_hash", "created_at": "sr.created_at",
		},
	},
}

// runCorpus returns one context-scoped corpus section for the selected run.
func (s *Server) runCorpus(w http.ResponseWriter, r *http.Request) {
	switch r.PathValue("kind") {
	case "articles":
		s.runEvaluation(w, r)
		return
	case "authors":
		s.runCorpusAuthors(w, r)
		return
	case "references":
		s.runCorpusReferences(w, r)
		return
	case "sources":
	default:
		s.respond(w, r, nil, viewrequest.NotFound("corpus collection not found"))
		return
	}
	runID, err := viewrequest.PositiveID(r.PathValue("id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	definition := runCorpusDefinitions["sources"]
	page, perPage, sort, order, query, err := viewrequest.Rows(r, definition.sortFields, definition.columns[0])
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
	result, err := s.sourceStore.ListCorpusRecords(ctx, source.CorpusRecordFilter{
		RunID: runID, Query: query, Sort: sort, Order: order, Page: page, PerPage: perPage,
	})
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	items := make([]map[string]any, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, corpusSourceRow(item))
	}
	page = clampScopedPage(page, perPage, result.Total)
	payload := map[string]any{
		"run_id":     runID,
		"collection": "sources",
		"columns":    definition.columns,
		"rows":       items,
		"pagination": viewrequest.Pagination(page, perPage, result.Total, sort, order),
	}
	sourceResultCounts, sourceErr := s.sourceResultCounts(ctx, runID)
	if sourceErr != nil {
		err = sourceErr
	} else {
		payload["source_result_counts"] = sourceResultCounts
	}
	s.respond(w, r, payload, err)
}

// runCorpusAuthors returns one bounded page of run-scoped author occurrences.
func (s *Server) runCorpusAuthors(w http.ResponseWriter, r *http.Request) {
	runID, err := viewrequest.PositiveID(r.PathValue("id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	definition := runCorpusDefinitions["authors"]
	page, perPage, sort, order, query, err := viewrequest.Rows(r, definition.sortFields, definition.columns[0])
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
	result, err := s.workStore.ListCorpusAuthors(ctx, work.CorpusAuthorFilter{
		RunID: runID, Query: query, Sort: sort, Order: order, Page: page, PerPage: perPage,
	})
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	items := make([]map[string]any, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, corpusAuthorRow(item))
	}
	page = clampScopedPage(page, perPage, result.Total)
	s.respond(w, r, map[string]any{
		"run_id":     runID,
		"collection": "authors",
		"columns":    definition.columns,
		"rows":       items,
		"pagination": viewrequest.Pagination(page, perPage, result.Total, sort, order),
	}, nil)
}

// runCorpusReferences returns one bounded page of run-scoped reference mentions.
func (s *Server) runCorpusReferences(w http.ResponseWriter, r *http.Request) {
	runID, err := viewrequest.PositiveID(r.PathValue("id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	definition := runCorpusDefinitions["references"]
	page, perPage, sort, order, query, err := viewrequest.Rows(r, definition.sortFields, definition.columns[0])
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
	result, err := s.workStore.ListCorpusReferences(ctx, work.CorpusReferenceFilter{
		RunID: runID, Query: query, Sort: sort, Order: order, Page: page, PerPage: perPage,
	})
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	items := make([]map[string]any, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, corpusReferenceRow(item))
	}
	page = clampScopedPage(page, perPage, result.Total)
	s.respond(w, r, map[string]any{
		"run_id":     runID,
		"collection": "references",
		"columns":    definition.columns,
		"rows":       items,
		"pagination": viewrequest.Pagination(page, perPage, result.Total, sort, order),
	}, nil)
}

// attachArticleTermMatches adds bounded stored search-term evidence to article collection rows.
func (s *Server) attachArticleTermMatches(ctx context.Context, runID int64, items []map[string]any) error {
	termRows, termTotal, err := s.runSearchTerms(ctx, runID)
	if err != nil {
		return err
	}
	if len(termRows) == 0 {
		for _, item := range items {
			item["term_matches"] = nil
		}
		return nil
	}
	revisionIDs := make([]int64, 0, len(items))
	for _, item := range items {
		if id, ok := item["id"].(int64); ok {
			revisionIDs = append(revisionIDs, id)
		}
	}
	matches, err := s.revisionTermMatchesBulk(ctx, runID, revisionIDs)
	if err != nil {
		return err
	}
	for _, item := range items {
		id, _ := item["id"].(int64)
		item["term_matches"] = rowTermMatches(termRows, termTotal, matches[id])
	}
	return nil
}

// corpusSourceRow maps one source family record into the corpus row shape.
func corpusSourceRow(item *source.CorpusRecord) map[string]any {
	return map[string]any{
		"id":            item.ID,
		"run_source_id": item.RunSourceID,
		"source_name":   item.SourceName,
		"source_type":   item.SourceType,
		"record_index":  item.RecordIndex,
		"parse_status":  item.ParseStatus,
		"reject_reason": jsonvalue.Optional(item.RejectReason),
		"content_hash":  item.ContentHash,
		"created_at":    item.CreatedAt,
	}
}

// runStages returns detailed work-stage outcomes for the selected run.
func (s *Server) runStages(w http.ResponseWriter, r *http.Request) {
	runID, err := viewrequest.PositiveID(r.PathValue("id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	fields := map[string]string{
		"id":         "rws.id",
		"work_id":    "rws.work_id",
		"stage_name": "rws.stage_name",
		"outcome":    "rws.outcome",
		"reason":     "rws.reason",
		"created_at": "rws.created_at",
		"updated_at": "rws.updated_at",
	}
	page, perPage, sort, order, query, err := viewrequest.Rows(r, fields, "id")
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
	result, err := s.workStore.ListRunStages(ctx, work.RunStageFilter{
		RunID: runID, Query: query, Sort: sort, Order: order, Page: page, PerPage: perPage,
	})
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	items := make([]map[string]any, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, stageOutcomeRow(item))
	}
	page = clampScopedPage(page, perPage, result.Total)
	stageSummaries, err := s.runStageSummaries(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	steps, err := s.runStore.ListStepsForViewer(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	stepRows := make([]map[string]any, 0, len(steps))
	for _, step := range steps {
		stepRows = append(stepRows, viewerStepRow(step))
	}
	s.respond(w, r, map[string]any{
		"run_id": runID, "columns": []string{"id", "work_id", "stage_name", "outcome", "reason", "created_at", "updated_at"}, "rows": items,
		"pagination":      viewrequest.Pagination(page, perPage, result.Total, sort, order),
		"stage_summaries": stageSummaries, "run_steps": stepRows,
	}, nil)
}

// runStageSummaries returns aggregate outcome counts by pipeline stage.
func (s *Server) runStageSummaries(ctx context.Context, runID int64) ([]map[string]any, error) {
	summaries, err := s.workStore.RunStageSummaries(ctx, runID)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(summaries))
	for _, summary := range summaries {
		result = append(result, stageSummaryRow(summary))
	}
	return result, nil
}

// clampScopedPage maps an offset request past the end to the final populated page.
func clampScopedPage(page, perPage int, total int64) int {
	totalPages := (total + int64(perPage) - 1) / int64(perPage)
	if totalPages == 0 {
		return 1
	}
	if int64(page) > totalPages {
		return int(totalPages)
	}
	return page
}

// sqlOrderKeyword returns the validated ascending/descending SQL keyword for a
// request direction. Request parsing already restricts order to ASC or DESC;
// the default is defensive and keeps request input out of the SQL text.
func sqlOrderKeyword(order string) string {
	if order == "DESC" {
		return "DESC"
	}
	return "ASC"
}

// requireRun requires a valid run value.
func (s *Server) requireRun(ctx context.Context, runID int64) error {
	exists, err := s.runStore.Exists(ctx, runID)
	if err != nil {
		return err
	}
	if !exists {
		return viewrequest.NotFound("run not found")
	}
	return nil
}
