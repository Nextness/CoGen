// overview.go provides the run overview endpoint that returns pipeline
// metrics, enrichment breakdowns, stage outcomes, and per-source
// document counts for a selected run.
package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

var knownRunMetrics = []string{
	"input_records",
	"parsed_articles",
	"deduplicated_articles",
	"duplicate_articles",
	"valid_articles",
	"discarded_articles",
	"enrichment_skipped",
	"enrichment_candidates",
	"enriched_article_updates",
	"enriched_fields_total",
	"enriched_fields_title",
	"enriched_fields_abstract",
	"enriched_fields_publisher",
	"enriched_fields_citation_count",
	"enriched_fields_references",
	"enriched_fields_authors",
	"enriched_fields_author_orcid",
	"enriched_fields_author_first_name",
	"enriched_fields_author_last_name",
	"enriched_fields_author_citation_name",
	"enriched_fields_author_affiliation",
	"normalized_articles_processed",
	"normalization_fields_processed",
	"normalization_fields_changed",
	"normalization_fields_already_canonical",
	"normalization_fields_unavailable",
	"cache_hits",
	"cache_misses",
	"cache_negative",
	"cache_stale",
	"cache_network_fetches",
	"cache_invalid_payloads",
}

const legacyDiscoveryLimit = 100

// sourceResultCounts returns the stored source inventory and result-count evidence for a run.
func (s *Server) sourceResultCounts(ctx context.Context, runID int64) ([]map[string]any, error) {
	dateColumn := "NULL AS export_date"
	if s.tableHasColumns("run_sources", "export_date") {
		dateColumn = "export_date"
	}
	countColumns := fmt.Sprintf(`NULL AS expected_result_count, NULL AS observed_result_count,
        NULL AS result_count_comparison, %s`, dateColumn)
	if s.tableHasColumns("run_sources", "expected_result_count", "observed_result_count", "result_count_comparison") {
		countColumns = fmt.Sprintf("expected_result_count, observed_result_count, result_count_comparison, %s", dateColumn)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, source_name, source_type, expected_file, query, `+countColumns+`
        FROM run_sources WHERE pipeline_run_id=? ORDER BY id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return rowsAsMaps(rows)
}

// sourceFilterCounts decodes stored per-source filter stages and reports malformed evidence without exposing its raw content.
func (s *Server) sourceFilterCounts(ctx context.Context, runID int64) ([]map[string]any, []map[string]any, error) {
	if !s.tableHasColumns("source_filter_counts") {
		return nil, nil, nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT source_name, filter_data FROM source_filter_counts WHERE pipeline_run_id=? ORDER BY source_name`, runID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	items, err := rowsAsMaps(rows)
	if err != nil {
		return nil, nil, err
	}
	result := make([]map[string]any, 0, len(items))
	diagnostics := make([]map[string]any, 0)
	for _, item := range items {
		sourceName, _ := item["source_name"].(string)
		filterDataRaw, _ := item["filter_data"].(string)
		if filterDataRaw == "" || filterDataRaw == "[]" {
			continue
		}
		var filterStages []struct {
			Filters []string `json:"filters"`
			Count   *int64   `json:"count"`
		}
		if err := json.Unmarshal([]byte(filterDataRaw), &filterStages); err != nil {
			diagnostics = append(diagnostics, map[string]any{"source": sourceName, "state": "invalid", "code": "invalid_json", "message": "Stored source-filter evidence is not valid JSON."})
			continue
		}
		for index, stage := range filterStages {
			if len(stage.Filters) == 0 || stage.Count == nil || *stage.Count < 0 {
				diagnostics = append(diagnostics, map[string]any{"source": sourceName, "state": "invalid", "code": "invalid_stage", "stage_index": index, "message": "Stored source-filter evidence has an invalid filter list or count."})
				continue
			}
			result = append(result, map[string]any{"source": sourceName, "filters": stage.Filters, "count": *stage.Count, "state": "recorded"})
		}
	}
	return result, diagnostics, nil
}

// health reports database readability and the discovered table inventory.
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := queryContext(r)
	defer cancel()
	metadataErr := s.db.PingContext(ctx)
	metadataReadable := metadataErr == nil
	corpusID := ""
	reviewWritable := false
	if metadataReadable && s.writeDB != nil {
		var queryOnly int
		if err := s.writeDB.DB.PingContext(ctx); err == nil {
			if err := s.writeDB.DB.QueryRowContext(ctx, "PRAGMA query_only").Scan(&queryOnly); err == nil && queryOnly == 0 {
				var reviewErr error
				corpusID, reviewErr = s.writeDB.Reviews.CorpusID(ctx)
				reviewWritable = reviewErr == nil
			}
		}
	}
	pdfBound := s.pdfDB != nil
	pdfReadable := false
	if pdfBound {
		pdfReadable = s.pdfDB.PingContext(ctx) == nil
	}
	s.respond(w, r, map[string]any{
		"readable": metadataReadable, "metadata_readable": metadataReadable,
		"table_count": len(s.tables), "tables": s.tableNames(), "corpus_id": corpusID,
		"review_writable": reviewWritable, "pdf_store_bound": pdfBound, "pdf_store_readable": pdfReadable,
		"review": map[string]any{
			"available": reviewWritable, "metadata_writable": reviewWritable,
			"pdf_store_bound": pdfBound, "pdf_store_readable": pdfReadable,
			"pdf_store_read_only": pdfBound && pdfReadable,
		},
	}, metadataErr)
}

// tableNames returns discovered table names in deterministic order.
func (s *Server) tableNames() []string {
	names := make([]string, 0, len(s.tables))
	for name := range s.tables {
		names = append(names, name)
	}
	// Table discovery orders its query. This fallback is intentionally tiny to
	// avoid exposing map iteration order through the API.
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j] < names[j-1]; j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}
	return names
}

// searches returns a bounded compatibility view of searches and their newest revisions.
func (s *Server) searches(w http.ResponseWriter, r *http.Request) {
	if err := validateKnownQuery(r); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	rows, err := s.db.QueryContext(ctx, `WITH selected_searches AS (
		SELECT id, search_id, created_at FROM searches ORDER BY id DESC LIMIT ?
	), ranked_revisions AS (
		SELECT sr.id, sr.search_id, sr.revision_label, sr.config_artifact_hash,
			sr.resolved_manifest_hash, sr.created_at,
			ROW_NUMBER() OVER (PARTITION BY sr.search_id ORDER BY sr.id DESC) AS row_number
		FROM search_revisions sr JOIN selected_searches selected ON selected.id=sr.search_id
	)
		SELECT s.id, s.search_id, s.created_at,
        sr.id AS revision_id, sr.revision_label, sr.config_artifact_hash,
        sr.resolved_manifest_hash, sr.created_at AS revision_created_at
		FROM selected_searches s
		LEFT JOIN ranked_revisions sr ON sr.search_id=s.id AND sr.row_number<=?
		ORDER BY s.id DESC, sr.id DESC`, legacyDiscoveryLimit+1, legacyDiscoveryLimit+1)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	defer rows.Close()
	type revision struct {
		ID                   int64  `json:"id"`
		Label                string `json:"label"`
		ConfigArtifactHash   string `json:"config_artifact_hash"`
		ResolvedManifestHash string `json:"resolved_manifest_hash"`
		CreatedAt            string `json:"created_at"`
	}
	type search struct {
		ID                 int64      `json:"id"`
		SearchID           string     `json:"search_id"`
		CreatedAt          string     `json:"created_at"`
		Revisions          []revision `json:"revisions"`
		RevisionsTruncated bool       `json:"revisions_truncated"`
	}
	byID := map[int64]*search{}
	ordered := make([]*search, 0)
	for rows.Next() {
		var item search
		var revisionID sql.NullInt64
		var rev revision
		var nullable [4]sql.NullString
		if err := rows.Scan(&item.ID, &item.SearchID, &item.CreatedAt, &revisionID, &nullable[0], &nullable[1], &nullable[2], &nullable[3]); err != nil {
			s.respond(w, r, nil, err)
			return
		}
		existing := byID[item.ID]
		if existing == nil {
			existing = &item
			byID[item.ID] = existing
			ordered = append(ordered, existing)
		}
		if revisionID.Valid && len(existing.Revisions) < legacyDiscoveryLimit {
			rev.ID = revisionID.Int64
			rev.Label = nullable[0].String
			rev.ConfigArtifactHash = nullable[1].String
			rev.ResolvedManifestHash = nullable[2].String
			rev.CreatedAt = nullable[3].String
			existing.Revisions = append(existing.Revisions, rev)
		} else if revisionID.Valid {
			existing.RevisionsTruncated = true
		}
	}
	if err := rows.Err(); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	hasMore := len(ordered) > legacyDiscoveryLimit
	if hasMore {
		ordered = ordered[:legacyDiscoveryLimit]
	}
	s.respond(w, r, map[string]any{
		"searches": ordered, "has_more": hasMore, "limit": legacyDiscoveryLimit,
		"deprecated": true, "replacement": "/api/hierarchy?section=searches",
	}, nil)
}

// plans returns a bounded compatibility view of execution plans for one revision.
func (s *Server) plans(w http.ResponseWriter, r *http.Request) {
	if err := validateKnownQuery(r, "search_revision_id"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	id, err := requiredQueryID(r, "search_revision_id")
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	rows, err := s.db.QueryContext(ctx, `SELECT id, search_revision_id, execution_fingerprint,
        resolved_manifest_hash, input_manifest_hash, enrichment_enabled, created_at
		FROM execution_plans WHERE search_revision_id=? ORDER BY id DESC LIMIT ?`, id, legacyDiscoveryLimit+1)
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
	hasMore := len(items) > legacyDiscoveryLimit
	if hasMore {
		items = items[:legacyDiscoveryLimit]
	}
	s.respond(w, r, map[string]any{
		"plans": items, "has_more": hasMore, "limit": legacyDiscoveryLimit,
		"deprecated": true, "replacement": "/api/hierarchy?section=plans&search_revision_id=" + strconv.FormatInt(id, 10),
	}, nil)
}

// runs returns pipeline attempts filtered by research context and visibility.
func (s *Server) runs(w http.ResponseWriter, r *http.Request) {
	if err := validateKnownQuery(r, "search_revision_id", "plan_id", "include_trashed"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	includeTrashed := r.URL.Query().Get("include_trashed") == "true"
	if raw := r.URL.Query().Get("include_trashed"); raw != "" && raw != "true" && raw != "false" {
		s.respond(w, r, nil, badRequest("include_trashed must be true or false"))
		return
	}
	args := make([]any, 0, 2)
	clauses := make([]string, 0, 2)
	if raw := r.URL.Query().Get("search_revision_id"); raw != "" {
		id, err := positiveID(raw)
		if err != nil {
			s.respond(w, r, nil, err)
			return
		}
		clauses = append(clauses, "ep.search_revision_id=?")
		args = append(args, id)
	}
	if raw := r.URL.Query().Get("plan_id"); raw != "" {
		id, err := positiveID(raw)
		if err != nil {
			s.respond(w, r, nil, err)
			return
		}
		clauses = append(clauses, "pr.execution_plan_id=?")
		args = append(args, id)
	}
	if !includeTrashed {
		clauses = append(clauses, "pr.visibility_state != 'trashed'")
	}
	query := `SELECT pr.id, pr.step, pr.started_at, pr.finished_at, pr.status, pr.summary,
        pr.search_query, pr.execution_plan_id, pr.attempt_number, pr.visibility_state,
        pr.trashed_at, pr.trash_reason, ep.search_revision_id
        FROM pipeline_runs pr LEFT JOIN execution_plans ep ON ep.id=pr.execution_plan_id`
	if len(clauses) != 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY pr.id DESC LIMIT ?"
	args = append(args, legacyDiscoveryLimit+1)
	ctx, cancel := queryContext(r)
	defer cancel()
	rows, err := s.db.QueryContext(ctx, query, args...)
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
	hasMore := len(items) > legacyDiscoveryLimit
	if hasMore {
		items = items[:legacyDiscoveryLimit]
	}
	s.respond(w, r, map[string]any{
		"runs": items, "has_more": hasMore, "limit": legacyDiscoveryLimit,
		"deprecated": true, "replacement": "/api/hierarchy?section=runs",
	}, nil)
}

// runContext returns the canonical complete ancestry and lifecycle for one run.
func (s *Server) runContext(w http.ResponseWriter, r *http.Request) {
	runID, err := positiveID(r.PathValue("id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()

	type searchContext struct {
		ID        int64  `json:"id"`
		SearchID  string `json:"search_id"`
		CreatedAt string `json:"created_at"`
	}
	type revisionContext struct {
		ID                   int64  `json:"id"`
		SearchID             int64  `json:"search_id"`
		Label                string `json:"label"`
		ConfigArtifactHash   string `json:"config_artifact_hash"`
		ResolvedManifestHash string `json:"resolved_manifest_hash"`
		CreatedAt            string `json:"created_at"`
	}
	type planContext struct {
		ID                   int64  `json:"id"`
		SearchRevisionID     int64  `json:"search_revision_id"`
		ExecutionFingerprint string `json:"execution_fingerprint"`
		ResolvedManifestHash string `json:"resolved_manifest_hash"`
		InputManifestHash    string `json:"input_manifest_hash"`
		EnrichmentEnabled    bool   `json:"enrichment_enabled"`
		CreatedAt            string `json:"created_at"`
	}
	type runContext struct {
		ID              int64   `json:"id"`
		ExecutionPlanID int64   `json:"execution_plan_id"`
		Step            string  `json:"step"`
		StartedAt       string  `json:"started_at"`
		FinishedAt      *string `json:"finished_at"`
		Status          string  `json:"status"`
		Summary         *string `json:"summary"`
		AttemptNumber   int64   `json:"attempt_number"`
		VisibilityState string  `json:"visibility_state"`
		TrashedAt       *string `json:"trashed_at"`
		TrashReason     *string `json:"trash_reason"`
	}

	var search searchContext
	var revision revisionContext
	var plan planContext
	var run runContext
	var reviewContextID sql.NullInt64
	err = s.db.QueryRowContext(ctx, `SELECT
		s.id, s.search_id, s.created_at,
		sr.id, sr.search_id, sr.revision_label, sr.config_artifact_hash, sr.resolved_manifest_hash, sr.created_at,
		ep.id, ep.search_revision_id, ep.execution_fingerprint, ep.resolved_manifest_hash, ep.input_manifest_hash, ep.enrichment_enabled, ep.created_at,
		pr.id, pr.execution_plan_id, pr.step, pr.started_at, pr.finished_at, pr.status, pr.summary,
		pr.attempt_number, pr.visibility_state, pr.trashed_at, pr.trash_reason, rc.id
		FROM pipeline_runs pr
		JOIN execution_plans ep ON ep.id=pr.execution_plan_id
		JOIN search_revisions sr ON sr.id=ep.search_revision_id
		JOIN searches s ON s.id=sr.search_id
		LEFT JOIN review_contexts rc ON rc.pipeline_run_id=pr.id
		WHERE pr.id=?`, runID).Scan(
		&search.ID, &search.SearchID, &search.CreatedAt,
		&revision.ID, &revision.SearchID, &revision.Label, &revision.ConfigArtifactHash, &revision.ResolvedManifestHash, &revision.CreatedAt,
		&plan.ID, &plan.SearchRevisionID, &plan.ExecutionFingerprint, &plan.ResolvedManifestHash, &plan.InputManifestHash, &plan.EnrichmentEnabled, &plan.CreatedAt,
		&run.ID, &run.ExecutionPlanID, &run.Step, &run.StartedAt, &run.FinishedAt, &run.Status, &run.Summary,
		&run.AttemptNumber, &run.VisibilityState, &run.TrashedAt, &run.TrashReason, &reviewContextID,
	)
	if err == sql.ErrNoRows {
		s.respond(w, r, nil, notFound("run context not found"))
		return
	}
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}

	runWritable := run.Status == "completed" && run.VisibilityState != "trashed"
	var contextID any
	if reviewContextID.Valid {
		contextID = reviewContextID.Int64
	}
	s.respond(w, r, map[string]any{
		"search": search, "revision": revision, "plan": plan, "run": run,
		"lifecycle": map[string]any{
			"status": run.Status, "visibility_state": run.VisibilityState, "review_writable": runWritable,
		},
		"review": map[string]any{
			"initialized": reviewContextID.Valid, "context_id": contextID, "run_writable": runWritable,
		},
	}, nil)
}

// overview returns captured metrics, coverage, relationships, and source evidence for a run.
func (s *Server) overview(w http.ResponseWriter, r *http.Request) {
	runID, err := requiredQueryID(r, "run_id")
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	var exists int
	if err := s.db.QueryRowContext(ctx, "SELECT 1 FROM pipeline_runs WHERE id=?", runID).Scan(&exists); err == sql.ErrNoRows {
		s.respond(w, r, nil, notFound("run not found"))
		return
	} else if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	rows, err := s.db.QueryContext(ctx, "SELECT metric, source, value FROM pipeline_run_metrics WHERE pipeline_run_id=? ORDER BY metric, source", runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	defer rows.Close()
	metrics, err := rowsAsMaps(rows)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	metricValues := map[string]int64{}
	for _, metric := range metrics {
		if source, _ := metric["source"].(string); source == "" {
			switch v := metric["value"].(type) {
			case int64:
				metricValues[metric["metric"].(string)] = v
			case int:
				metricValues[metric["metric"].(string)] = int64(v)
			}
		}
	}
	byName := make(map[string]map[string]any, len(metrics))
	for _, metric := range metrics {
		item := map[string]any{"metric": metric["metric"], "source": metric["source"], "available": true, "state": "recorded", "value": metric["value"]}
		if metric["source"] == "" {
			denominator, ok := metricDenominator(metric["metric"].(string), metricValues)
			if ok && denominator > 0 {
				item["denominator"] = denominator
				item["percentage"] = float64(metric["value"].(int64)) * 100 / float64(denominator)
			}
		}
		if metric["source"] == "" {
			byName[metric["metric"].(string)] = item
		}
	}
	metricSummary := make([]map[string]any, 0, len(knownRunMetrics)+len(metrics))
	for _, name := range knownRunMetrics {
		if item, ok := byName[name]; ok {
			metricSummary = append(metricSummary, item)
		} else {
			metricSummary = append(metricSummary, map[string]any{"metric": name, "source": "", "available": false, "state": "unavailable"})
		}
	}
	for _, metric := range metrics {
		if metric["source"] != "" {
			metricSummary = append(metricSummary, map[string]any{"metric": metric["metric"], "source": metric["source"], "available": true, "state": "recorded", "value": metric["value"]})
		}
	}
	coverage, err := s.currentCoverage(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	breakdown, err := s.relationshipTotals(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	sourceResultCounts, err := s.sourceResultCounts(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	sourceFilterCounts, sourceFilterDiagnostics, err := s.sourceFilterCounts(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	s.respond(w, r, map[string]any{
		"run_id":                        runID,
		"captured_metrics":              metricSummary,
		"retention_funnel":              metricGroup(byName, "input_records", "parsed_articles", "deduplicated_articles", "valid_articles", "discarded_articles"),
		"source_breakdown":              sourceBreakdown(metrics, metricValues),
		"source_result_counts":          sourceResultCounts,
		"source_filter_counts":          sourceFilterCounts,
		"source_filter_diagnostics":     sourceFilterDiagnostics,
		"validation_breakdown":          metricGroup(byName, "valid_articles", "discarded_articles"),
		"cache_breakdown":               metricGroup(byName, "cache_hits", "cache_misses", "cache_negative", "cache_stale", "cache_network_fetches", "cache_invalid_payloads"),
		"enrichment_breakdown":          metricGroup(byName, "enrichment_skipped", "enrichment_candidates", "enriched_article_updates"),
		"enrichment_field_breakdown":    enrichmentFieldBreakdown(byName),
		"enrichment_provider_breakdown": enrichmentProviderBreakdown(metrics),
		"normalization_breakdown":       metricGroup(byName, "normalized_articles_processed", "normalization_fields_processed", "normalization_fields_changed", "normalization_fields_already_canonical", "normalization_fields_unavailable"),
		"normalization_field_breakdown": normalizationFieldBreakdown(metrics),
		"current_coverage":              coverage,
		"relationship_totals":           breakdown,
	}, nil)
}

// metricGroup selects named metrics and marks absent captures as unavailable.
func metricGroup(metrics map[string]map[string]any, names ...string) map[string]any {
	result := make(map[string]any, len(names))
	for _, name := range names {
		if metric, ok := metrics[name]; ok {
			result[name] = metric
		} else {
			result[name] = map[string]any{"available": false, "state": "unavailable"}
		}
	}
	return result
}

// sourceBreakdown calculates each source's share of captured input records.
func sourceBreakdown(metrics []map[string]any, totals map[string]int64) map[string]any {
	result := map[string]any{}
	for _, metric := range metrics {
		source, _ := metric["source"].(string)
		if source == "" || metric["metric"] != "input_records" {
			continue
		}
		value, _ := metric["value"].(int64)
		item := map[string]any{"available": true, "state": "recorded", "value": value}
		if denominator := totals["input_records"]; denominator > 0 {
			item["denominator"] = denominator
			item["percentage"] = float64(value) * 100 / float64(denominator)
		}
		result[source] = item
	}
	return result
}

// enrichmentFieldBreakdown extracts per-field enrichment counts from the
// byName metric map. Metrics named "enriched_fields_<field>" with no source
// are included.
func enrichmentFieldBreakdown(byName map[string]map[string]any) map[string]any {
	result := make(map[string]any)
	prefix := "enriched_fields_"
	for name, item := range byName {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		field := strings.TrimPrefix(name, prefix)
		if field == "total" {
			continue
		}
		result[field] = item
	}
	return result
}

// enrichmentProviderBreakdown extracts per-provider enrichment counts from the
// raw metrics list. Metrics named "enriched_fields" with a non-empty source
// are included.
func enrichmentProviderBreakdown(metrics []map[string]any) map[string]any {
	result := make(map[string]any)
	for _, metric := range metrics {
		m, _ := metric["metric"].(string)
		source, _ := metric["source"].(string)
		if m != "enriched_fields" || source == "" {
			continue
		}
		result[source] = map[string]any{"available": true, "state": "recorded", "value": metric["value"]}
	}
	return result
}

// normalizationFieldBreakdown groups normalization outcome metrics by field and derives percentages.
func normalizationFieldBreakdown(metrics []map[string]any) map[string]map[string]any {
	fields := []string{"publisher", "journal", "author_name", "affiliation"}
	statuses := []string{"processed", "changed", "already_canonical", "unavailable"}
	result := make(map[string]map[string]any, len(fields))
	for _, field := range fields {
		result[field] = make(map[string]any, len(statuses))
		for _, status := range statuses {
			result[field][status] = map[string]any{"available": false, "state": "unavailable"}
		}
	}
	for _, metric := range metrics {
		field, _ := metric["source"].(string)
		if _, ok := result[field]; !ok {
			continue
		}
		name, _ := metric["metric"].(string)
		const prefix = "normalization_fields_"
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		status := strings.TrimPrefix(name, prefix)
		if _, ok := result[field][status]; !ok {
			continue
		}
		result[field][status] = map[string]any{"available": true, "state": "recorded", "value": metric["value"]}
	}
	for _, field := range fields {
		processed, _ := result[field]["processed"].(map[string]any)
		processedValue, _ := processed["value"].(int64)
		if processedValue == 0 {
			continue
		}
		for _, status := range statuses[1:] {
			item := result[field][status].(map[string]any)
			if item["available"] != true {
				continue
			}
			item["denominator"] = processedValue
			item["percentage"] = float64(item["value"].(int64)) * 100 / float64(processedValue)
		}
	}
	return result
}

// metricDenominator returns the captured population against which a metric is measured.
func metricDenominator(metric string, values map[string]int64) (int64, bool) {
	switch metric {
	case "input_records", "parsed_articles", "deduplicated_articles":
		return values["input_records"], values["input_records"] > 0
	case "valid_articles", "discarded_articles", "enrichment_candidates", "enriched_article_updates":
		return values["deduplicated_articles"], values["deduplicated_articles"] > 0
	case "normalized_articles_processed":
		return values["valid_articles"], values["valid_articles"] > 0
	case "normalization_fields_changed", "normalization_fields_already_canonical", "normalization_fields_unavailable":
		return values["normalization_fields_processed"], values["normalization_fields_processed"] > 0
	}
	return 0, false
}

// currentCoverage returns work-revision and journal coverage for a run.
func (s *Server) currentCoverage(ctx context.Context, runID int64) (map[string]any, error) {
	result := map[string]any{}
	var total, normalized int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*), COALESCE(SUM(CASE WHEN journal IS NOT NULL AND journal != '' THEN 1 ELSE 0 END),0) FROM work_revisions WHERE pipeline_run_id=?", runID).Scan(&total, &normalized); err != nil {
		return nil, err
	}
	result["work_revisions"] = map[string]any{"value": total, "available": true, "state": "derived"}
	result["journal_coverage"] = map[string]any{"value": normalized, "denominator": total, "percentage": percent(normalized, total), "available": true, "state": "derived"}
	return result, nil
}

// relationshipTotals counts canonical works, authorships, references, and resolved citations for a run.
func (s *Server) relationshipTotals(ctx context.Context, runID int64) (map[string]any, error) {
	queries := map[string]string{
		"work_revisions":          "SELECT COUNT(*) FROM work_revisions WHERE pipeline_run_id=?",
		"analysis_ready_articles": "SELECT COUNT(*) FROM work_revisions wr WHERE wr.pipeline_run_id=? AND " + currentNormalizedRevisionPredicate("wr"),
		"authorships":             "SELECT COUNT(*) FROM authorships a JOIN work_revisions wr ON wr.id=a.work_revision_id WHERE wr.pipeline_run_id=? AND " + currentNormalizedRevisionPredicate("wr"),
		"reference_mentions":      "SELECT COUNT(*) FROM reference_mentions rm JOIN work_revisions wr ON wr.id=rm.work_revision_id WHERE wr.pipeline_run_id=? AND " + currentNormalizedRevisionPredicate("wr"),
		"internal_citations":      "SELECT COUNT(*) FROM reference_mentions rm JOIN work_revisions wr ON wr.id=rm.work_revision_id WHERE wr.pipeline_run_id=? AND rm.resolved_work_id IS NOT NULL AND " + currentNormalizedRevisionPredicate("wr"),
	}
	result := map[string]any{}
	for name, query := range queries {
		var count int64
		if err := s.db.QueryRowContext(ctx, query, runID).Scan(&count); err != nil {
			return nil, err
		}
		result[name] = map[string]any{"value": count, "available": true, "state": "derived"}
	}
	return result, nil
}

// percent returns value as a percentage of denominator, or nil when denominator is zero.
func percent(value, denominator int64) *float64 {
	if denominator == 0 {
		return nil
	}
	result := float64(value) * 100 / float64(denominator)
	return &result
}

// requiredQueryID validates the endpoint query allowlist and returns one required positive identifier.
func requiredQueryID(r *http.Request, name string) (int64, error) {
	if err := validateKnownQuery(r, name); err != nil {
		return 0, err
	}
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return 0, badRequest(name + " is required")
	}
	return positiveID(raw)
}

// validateKnownQuery rejects semicolon syntax and query parameters outside the endpoint allowlist.
func validateKnownQuery(r *http.Request, allowed ...string) error {
	if strings.Contains(r.URL.RawQuery, ";") {
		return badRequest("query parameters must not contain semicolons")
	}
	known := map[string]bool{}
	for _, name := range allowed {
		known[name] = true
	}
	for name := range r.URL.Query() {
		if !known[name] {
			return badRequest("unknown query parameter: " + name)
		}
	}
	return nil
}

// parseOptionalInt parses a named decimal query value for an endpoint diagnostic.
func parseOptionalInt(raw, name string) (int64, error) {
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, badRequest(name + " must be an integer")
	}
	return value, nil
}
