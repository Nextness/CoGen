// corpus.go provides the run-scoped corpus endpoint that returns
// paginated work revisions for the selected pipeline run, with
// support for arbitrary schema-discovered table browsing.
package server

import (
	"context"
	"database/sql"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// scopedRowsDefinition defines the safe projection, joins, filters, and sorting for one corpus section.
type scopedRowsDefinition struct {
	columns    []string
	from       string
	where      string
	groupBy    string
	search     string
	sortFields map[string]string
}

var runCorpusDefinitions = map[string]scopedRowsDefinition{
	"articles": {
		columns: []string{"id", "work_id", "title", "year", "journal", "publisher", "source", "doi", "validation_status", "citation_count", "reference_count", "producer_stage", "created_at", "abstract", "authors"},
		from: `FROM work_revisions wr
            JOIN works w ON w.id=wr.work_id
            LEFT JOIN run_work_stages validation ON validation.pipeline_run_id=wr.pipeline_run_id
                AND validation.work_id=wr.work_id AND validation.stage_name='validate'`,
		where:  "wr.pipeline_run_id=? AND wr.producer_stage='normalize' AND validation.outcome='valid'",
		search: "wr.title, w.doi, wr.journal, wr.publisher, wr.source",
		sortFields: map[string]string{
			"id": "wr.id", "title": "wr.title", "year": "wr.year", "journal": "wr.journal", "publisher": "wr.publisher", "source": "wr.source", "doi": "w.doi", "validation_status": "validation.outcome", "citation_count": "wr.citation_count", "reference_count": "wr.reference_count", "created_at": "wr.created_at",
		},
	},
	"authors": {
		columns: []string{"id", "citation_name", "first_name", "last_name", "orcid", "person_id", "article_count", "affiliation_count", "created_at"},
		from: `FROM author_occurrences ao
            JOIN authorships a ON a.author_occurrence_id=ao.id
            JOIN work_revisions wr ON wr.id=a.work_revision_id`,
		where:   "wr.pipeline_run_id=?",
		groupBy: "ao.id",
		search:  "ao.citation_name, ao.first_name, ao.last_name, ao.orcid",
		sortFields: map[string]string{
			"id": "ao.id", "citation_name": "ao.citation_name", "first_name": "ao.first_name", "last_name": "ao.last_name", "orcid": "ao.orcid", "article_count": "article_count", "affiliation_count": "affiliation_count", "created_at": "ao.created_at",
		},
	},
	"references": {
		columns: []string{"id", "work_revision_id", "mention_order", "doi", "title", "author", "year", "source", "resolved_work_id", "citing_title", "created_at"},
		from: `FROM reference_mentions rm
            JOIN work_revisions wr ON wr.id=rm.work_revision_id`,
		where:  "wr.pipeline_run_id=?",
		search: "rm.doi, rm.title, rm.author, rm.source, wr.title",
		sortFields: map[string]string{
			"id": "rm.id", "work_revision_id": "rm.work_revision_id", "mention_order": "rm.mention_order", "doi": "rm.doi", "title": "rm.title", "author": "rm.author", "year": "rm.year", "source": "rm.source", "resolved_work_id": "rm.resolved_work_id", "created_at": "rm.created_at",
		},
	},
	"sources": {
		columns: []string{"id", "run_source_id", "source_name", "source_type", "record_index", "parse_status", "reject_reason", "content_hash", "created_at"},
		from:    "FROM source_records sr JOIN run_sources rs ON rs.id=sr.run_source_id",
		where:   "rs.pipeline_run_id=?",
		search:  "rs.source_name, rs.source_type, sr.parse_status, sr.reject_reason, sr.content_hash",
		sortFields: map[string]string{
			"id": "sr.id", "run_source_id": "sr.run_source_id", "source_name": "rs.source_name", "source_type": "rs.source_type", "record_index": "sr.record_index", "parse_status": "sr.parse_status", "reject_reason": "sr.reject_reason", "content_hash": "sr.content_hash", "created_at": "sr.created_at",
		},
	},
}

// runCorpus returns one context-scoped corpus section for the selected run.
func (s *Server) runCorpus(w http.ResponseWriter, r *http.Request) {
	runID, err := positiveID(r.PathValue("id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	definition, ok := runCorpusDefinitions[r.PathValue("kind")]
	if !ok {
		s.respond(w, r, nil, notFound("corpus collection not found"))
		return
	}
	page, perPage, sort, order, query, err := scopedRowsRequest(r, definition.sortFields, definition.columns[0])
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
	where, args := scopedWhere(definition.where, definition.search, runID, query)
	countQuery := "SELECT COUNT(*) " + definition.from + " WHERE " + where
	if definition.groupBy != "" {
		countQuery = "SELECT COUNT(*) FROM (SELECT 1 " + definition.from + " WHERE " + where + " GROUP BY " + definition.groupBy + ")"
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	selectColumns := corpusSelectColumns(r.PathValue("kind"))
	querySQL := "SELECT " + selectColumns + " " + definition.from + " WHERE " + where
	if definition.groupBy != "" {
		querySQL += " GROUP BY " + definition.groupBy
	}
	querySQL += " ORDER BY " + definition.sortFields[sort] + " " + order + " LIMIT ? OFFSET ?"
	args = append(args, perPage, (page-1)*perPage)
	rows, err := s.db.QueryContext(ctx, querySQL, args...)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	defer rows.Close()
	items, err := rowsAsMaps(rows)
	payload := map[string]any{
		"run_id":     runID,
		"collection": r.PathValue("kind"),
		"columns":    definition.columns,
		"rows":       items,
		"pagination": scopedPagination(page, perPage, total, sort, order),
	}
	if err == nil && r.PathValue("kind") == "sources" {
		sourceResultCounts, sourceErr := s.sourceResultCounts(ctx, runID)
		if sourceErr != nil {
			err = sourceErr
		} else {
			payload["source_result_counts"] = sourceResultCounts
		}
	}
	s.respond(w, r, payload, err)
}

// corpusSelectColumns returns the fixed safe projection for a browsable corpus section.
func corpusSelectColumns(kind string) string {
	switch kind {
	case "articles":
		return "wr.id, wr.work_id, wr.title, wr.year, wr.journal, wr.publisher, wr.source, w.doi, validation.outcome AS validation_status, wr.citation_count, wr.reference_count, wr.producer_stage, wr.created_at, wr.abstract, (SELECT GROUP_CONCAT(ao.citation_name, '; ') FROM authorships a JOIN author_occurrences ao ON ao.id=a.author_occurrence_id WHERE a.work_revision_id=wr.id ORDER BY a.author_order) AS authors"
	case "authors":
		return "ao.id, ao.citation_name, ao.first_name, ao.last_name, ao.orcid, ao.person_id, COUNT(DISTINCT a.work_revision_id) AS article_count, COUNT(DISTINCT NULLIF(a.affiliation, '')) AS affiliation_count, ao.created_at"
	case "references":
		return "rm.id, rm.work_revision_id, rm.mention_order, rm.doi, rm.title, rm.author, rm.year, rm.source, rm.resolved_work_id, wr.title AS citing_title, rm.created_at"
	case "sources":
		return "sr.id, sr.run_source_id, rs.source_name, rs.source_type, sr.record_index, sr.parse_status, sr.reject_reason, sr.content_hash, sr.created_at"
	default:
		return ""
	}
}

// runStages returns detailed work-stage outcomes for the selected run.
func (s *Server) runStages(w http.ResponseWriter, r *http.Request) {
	runID, err := positiveID(r.PathValue("id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	fields := map[string]string{"id": "rws.id", "work_id": "rws.work_id", "stage_name": "rws.stage_name", "outcome": "rws.outcome", "reason": "rws.reason", "created_at": "rws.created_at", "updated_at": "rws.updated_at"}
	page, perPage, sort, order, query, err := scopedRowsRequest(r, fields, "id")
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
	where, args := scopedWhere("rws.pipeline_run_id=?", "rws.stage_name, rws.outcome, rws.reason, CAST(rws.work_id AS TEXT)", runID, query)
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM run_work_stages rws WHERE "+where, args...).Scan(&total); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	queryArgs := append(args, perPage, (page-1)*perPage)
	rows, err := s.db.QueryContext(ctx, "SELECT rws.id, rws.work_id, rws.stage_name, rws.outcome, rws.reason, rws.created_at, rws.updated_at FROM run_work_stages rws WHERE "+where+" ORDER BY "+fields[sort]+" "+order+" LIMIT ? OFFSET ?", queryArgs...)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	defer rows.Close()
	items, err := rowsAsMaps(rows)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	stageSummaries, err := s.runStageSummaries(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	steps, err := s.rows(ctx, `SELECT step_name, step_status, input_artifact_id, output_artifact_id,
		started_at, finished_at, input_fingerprint, output_fingerprint,
		CASE WHEN started_at IS NOT NULL AND finished_at IS NOT NULL
			THEN ROUND((julianday(finished_at)-julianday(started_at))*86400, 3)
			ELSE NULL END AS duration_seconds
		FROM run_steps WHERE pipeline_run_id=? ORDER BY id`, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	s.respond(w, r, map[string]any{
		"run_id": runID, "columns": []string{"id", "work_id", "stage_name", "outcome", "reason", "created_at", "updated_at"}, "rows": items,
		"pagination":      scopedPagination(page, perPage, total, sort, order),
		"stage_summaries": stageSummaries, "run_steps": steps,
	}, nil)
}

// runStageSummaries returns aggregate outcome counts by pipeline stage.
func (s *Server) runStageSummaries(ctx context.Context, runID int64) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT stage_name, outcome, COUNT(*) AS count,
		MIN(created_at) AS first_recorded_at, MAX(updated_at) AS last_recorded_at
		FROM run_work_stages WHERE pipeline_run_id=?
		GROUP BY stage_name, outcome ORDER BY stage_name, outcome`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records, err := rowsAsMaps(rows)
	if err != nil {
		return nil, err
	}
	byStage := make(map[string]map[string]any)
	for _, record := range records {
		stage, _ := record["stage_name"].(string)
		outcome, _ := record["outcome"].(string)
		count, _ := record["count"].(int64)
		summary := byStage[stage]
		if summary == nil {
			summary = map[string]any{
				"stage_name": stage, "total_records": int64(0), "outcomes": map[string]int64{},
				"first_recorded_at": record["first_recorded_at"], "last_recorded_at": record["last_recorded_at"],
			}
			byStage[stage] = summary
		}
		summary["total_records"] = summary["total_records"].(int64) + count
		summary["outcomes"].(map[string]int64)[outcome] = count
		if first, ok := record["first_recorded_at"].(string); ok {
			if current, ok := summary["first_recorded_at"].(string); !ok || first < current {
				summary["first_recorded_at"] = first
			}
		}
		if last, ok := record["last_recorded_at"].(string); ok {
			if current, ok := summary["last_recorded_at"].(string); !ok || last > current {
				summary["last_recorded_at"] = last
			}
		}
	}
	order := []string{"parse", "deduplicate", "enrich", "enrich_metadata", "enrich_identity", "validate", "normalize"}
	result := make([]map[string]any, 0, len(byStage))
	for _, stage := range order {
		if summary := byStage[stage]; summary != nil {
			result = append(result, summary)
			delete(byStage, stage)
		}
	}
	remaining := make([]string, 0, len(byStage))
	for stage := range byStage {
		remaining = append(remaining, stage)
	}
	sort.Strings(remaining)
	for _, stage := range remaining {
		result = append(result, byStage[stage])
	}
	return result, nil
}

// scopedRowsRequest parses and validates the context, filters, sorting, and pagination for a corpus request.
func scopedRowsRequest(r *http.Request, fields map[string]string, fallback string) (int, int, string, string, string, error) {
	if err := validateKnownQuery(r, "page", "per_page", "sort", "order", "q"); err != nil {
		return 0, 0, "", "", "", err
	}
	page, perPage := 1, 50
	if raw := r.URL.Query().Get("page"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			return 0, 0, "", "", "", badRequest("page must be a positive integer")
		}
		page = parsed
	}
	if raw := r.URL.Query().Get("per_page"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || !permittedPageSizes[parsed] {
			return 0, 0, "", "", "", badRequest("per_page must be one of 20, 50, 100, 200, 500")
		}
		perPage = parsed
	}
	sort := r.URL.Query().Get("sort")
	if sort == "" {
		sort = fallback
	}
	if _, ok := fields[sort]; !ok {
		return 0, 0, "", "", "", badRequest("sort must be a supported field")
	}
	order := strings.ToUpper(r.URL.Query().Get("order"))
	if order == "" {
		order = "ASC"
	}
	if order != "ASC" && order != "DESC" {
		return 0, 0, "", "", "", badRequest("order must be asc or desc")
	}
	return page, perPage, sort, order, strings.TrimSpace(r.URL.Query().Get("q")), nil
}

// scopedWhere builds the SQL predicate and arguments for a scoped corpus request.
func scopedWhere(base, searchable string, runID int64, query string) (string, []any) {
	args := []any{runID}
	if query == "" {
		return base, args
	}
	fields := strings.Split(searchable, ", ")
	conditions := make([]string, 0, len(fields))
	needle := "%" + strings.ToLower(query) + "%"
	for _, field := range fields {
		conditions = append(conditions, "LOWER(COALESCE("+field+", '')) LIKE ?")
		args = append(args, needle)
	}
	return base + " AND (" + strings.Join(conditions, " OR ") + ")", args
}

// scopedPagination returns validated page, page-size, offset, and limit values.
func scopedPagination(page, perPage int, total int64, sort, order string) map[string]any {
	return map[string]any{
		"page": page, "per_page": perPage, "total_rows": total,
		"total_pages": (total + int64(perPage) - 1) / int64(perPage),
		"has_next":    int64(page*perPage) < total, "sort": sort, "order": strings.ToLower(order),
	}
}

// requireRun requires a valid run value.
func (s *Server) requireRun(ctx context.Context, runID int64) error {
	var exists int
	if err := s.db.QueryRowContext(ctx, "SELECT 1 FROM pipeline_runs WHERE id=?", runID).Scan(&exists); err != nil {
		if err == sql.ErrNoRows {
			return notFound("run not found")
		}
		return err
	}
	return nil
}
