// overview.go provides the run overview endpoint that returns pipeline
// metrics, enrichment breakdowns, stage outcomes, and per-source
// document counts for a selected run.
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"analysis/database/run"
	"analysis/database/source"
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
	rows, err := s.sourceStore.ListResultCountsForRun(ctx, source.ResultCountFilter{
		RunID:               runID,
		IncludeExportDate:   s.tableHasColumns("run_sources", "export_date"),
		IncludeResultCounts: s.tableHasColumns("run_sources", "expected_result_count", "observed_result_count", "result_count_comparison"),
	})
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"id":                      row.ID,
			"source_name":             row.SourceName,
			"source_type":             row.SourceType,
			"expected_file":           row.ExpectedFile,
			"query":                   nullableText(row.Query),
			"expected_result_count":   nullableIDPointer(row.ExpectedResultCount),
			"observed_result_count":   nullableIDPointer(row.ObservedResultCount),
			"result_count_comparison": nullableText(row.ResultCountComparison),
			"export_date":             nullableText(row.ExportDate),
		})
	}
	return items, nil
}

// sourceFilterCounts decodes stored per-source filter stages and reports malformed evidence without exposing its raw content.
func (s *Server) sourceFilterCounts(ctx context.Context, runID int64) ([]map[string]any, []map[string]any, error) {
	if !s.tableHasColumns("source_filter_counts") {
		return nil, nil, nil
	}
	counts, err := s.sourceStore.ListFilterCountsByRun(ctx, runID)
	if err != nil {
		return nil, nil, err
	}
	result := make([]map[string]any, 0, len(counts))
	diagnostics := make([]map[string]any, 0)
	for _, item := range counts {
		sourceName := item.SourceName
		filterDataRaw := item.FilterData
		if filterDataRaw == "" || filterDataRaw == "[]" {
			continue
		}
		var filterStages []struct {
			Filters []string `json:"filters"`
			Count   *int64   `json:"count"`
		}
		if err := json.Unmarshal([]byte(filterDataRaw), &filterStages); err != nil {
			diagnostics = append(diagnostics, map[string]any{
				"source":  sourceName,
				"state":   "invalid",
				"code":    "invalid_json",
				"message": "Stored source-filter evidence is not valid JSON.",
			})
			continue
		}
		for index, stage := range filterStages {
			if len(stage.Filters) == 0 || stage.Count == nil || *stage.Count < 0 {
				diagnostics = append(diagnostics, map[string]any{
					"source":      sourceName,
					"state":       "invalid",
					"code":        "invalid_stage",
					"stage_index": index,
					"message":     "Stored source-filter evidence has an invalid filter list or count.",
				})
				continue
			}
			result = append(result, map[string]any{
				"source":  sourceName,
				"filters": stage.Filters,
				"count":   *stage.Count,
				"state":   "recorded",
			})
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
		if err := s.writeDB.PingContext(ctx); err == nil {
			if writable, err := s.writeDB.Writable(ctx); err == nil && writable {
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
		"readable":           metadataReadable,
		"metadata_readable":  metadataReadable,
		"table_count":        len(s.tables),
		"tables":             s.tableNames(),
		"corpus_id":          corpusID,
		"review_writable":    reviewWritable,
		"pdf_store_bound":    pdfBound,
		"pdf_store_readable": pdfReadable,
		"review": map[string]any{
			"available":           reviewWritable,
			"metadata_writable":   reviewWritable,
			"pdf_store_bound":     pdfBound,
			"pdf_store_readable":  pdfReadable,
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
	rows, err := s.searchStore.ListLegacySearches(ctx, legacyDiscoveryLimit+1)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	type revision struct {
		ID                   int64  `json:"id"`
		Label                string `json:"label"`
		ConfigArtifactHash   string `json:"config_artifact_hash"`
		ResolvedManifestHash string `json:"resolved_manifest_hash"`
		CreatedAt            string `json:"created_at"`
	}
	type searchRecord struct {
		ID                 int64      `json:"id"`
		SearchID           string     `json:"search_id"`
		CreatedAt          string     `json:"created_at"`
		Revisions          []revision `json:"revisions"`
		RevisionsTruncated bool       `json:"revisions_truncated"`
	}
	byID := map[int64]*searchRecord{}
	ordered := make([]*searchRecord, 0)
	for _, row := range rows {
		existing := byID[row.ID]
		if existing == nil {
			existing = &searchRecord{ID: row.ID, SearchID: row.SearchID, CreatedAt: row.CreatedAt}
			byID[row.ID] = existing
			ordered = append(ordered, existing)
		}
		if row.RevisionID == nil {
			continue
		}
		if len(existing.Revisions) < legacyDiscoveryLimit {
			existing.Revisions = append(existing.Revisions, revision{
				ID:                   *row.RevisionID,
				Label:                row.RevisionLabel,
				ConfigArtifactHash:   row.ConfigArtifactHash,
				ResolvedManifestHash: row.ResolvedManifestHash,
				CreatedAt:            row.RevisionCreatedAt,
			})
		} else {
			existing.RevisionsTruncated = true
		}
	}
	hasMore := len(ordered) > legacyDiscoveryLimit
	if hasMore {
		ordered = ordered[:legacyDiscoveryLimit]
	}
	s.respond(w, r, map[string]any{
		"searches":    ordered,
		"has_more":    hasMore,
		"limit":       legacyDiscoveryLimit,
		"deprecated":  true,
		"replacement": "/api/hierarchy?section=searches",
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
	plans, err := s.searchStore.ListLegacyPlans(ctx, id, legacyDiscoveryLimit+1)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	items := make([]map[string]any, 0, len(plans))
	for _, plan := range plans {
		enrichmentEnabled := int64(0)
		if plan.EnrichmentEnabled {
			enrichmentEnabled = 1
		}
		items = append(items, map[string]any{
			"id":                     plan.ID,
			"search_revision_id":     plan.SearchRevisionID,
			"execution_fingerprint":  plan.ExecutionFingerprint,
			"resolved_manifest_hash": plan.ResolvedManifestHash,
			"input_manifest_hash":    plan.InputManifestHash,
			"enrichment_enabled":     enrichmentEnabled,
			"created_at":             plan.CreatedAt,
		})
	}
	hasMore := len(items) > legacyDiscoveryLimit
	if hasMore {
		items = items[:legacyDiscoveryLimit]
	}
	s.respond(w, r, map[string]any{
		"plans":       items,
		"has_more":    hasMore,
		"limit":       legacyDiscoveryLimit,
		"deprecated":  true,
		"replacement": "/api/hierarchy?section=plans&search_revision_id=" + strconv.FormatInt(id, 10),
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
	filter := run.LegacyRunFilter{IncludeTrashed: includeTrashed, Limit: legacyDiscoveryLimit + 1}
	if raw := r.URL.Query().Get("search_revision_id"); raw != "" {
		id, err := positiveID(raw)
		if err != nil {
			s.respond(w, r, nil, err)
			return
		}
		filter.SearchRevisionID = id
	}
	if raw := r.URL.Query().Get("plan_id"); raw != "" {
		id, err := positiveID(raw)
		if err != nil {
			s.respond(w, r, nil, err)
			return
		}
		filter.PlanID = id
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	rows, err := s.runStore.ListLegacyRuns(ctx, filter)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"id":                 row.ID,
			"step":               row.Step,
			"started_at":         row.StartedAt,
			"finished_at":        nullableText(row.FinishedAt),
			"status":             row.Status,
			"summary":            nullableText(row.Summary),
			"search_query":       nullableText(row.SearchQuery),
			"execution_plan_id":  nullableIDPointer(row.ExecutionPlanID),
			"attempt_number":     nullableIDPointer(row.AttemptNumber),
			"visibility_state":   row.VisibilityState,
			"trashed_at":         nullableText(row.TrashedAt),
			"trash_reason":       nullableText(row.TrashReason),
			"search_revision_id": nullableIDPointer(row.SearchRevisionID),
		})
	}
	hasMore := len(items) > legacyDiscoveryLimit
	if hasMore {
		items = items[:legacyDiscoveryLimit]
	}
	s.respond(w, r, map[string]any{
		"runs":        items,
		"has_more":    hasMore,
		"limit":       legacyDiscoveryLimit,
		"deprecated":  true,
		"replacement": "/api/hierarchy?section=runs",
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
	context, err := s.runStore.RunContext(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if context == nil {
		s.respond(w, r, nil, notFound("run context not found"))
		return
	}
	runWritable := context.Run.Status == "completed" && context.Run.VisibilityState != "trashed"
	s.respond(w, r, runContextPayload(context, runWritable), nil)
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
	exists, err := s.runStore.Exists(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if !exists {
		s.respond(w, r, nil, notFound("run not found"))
		return
	}
	metricRows, err := s.runStore.ListMetricsByRun(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	metrics := make([]map[string]any, 0, len(metricRows))
	for _, metric := range metricRows {
		metrics = append(metrics, map[string]any{
			"metric": metric.Metric,
			"source": metric.Source,
			"value":  int64(metric.Value),
		})
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
		item := map[string]any{
			"metric":    metric["metric"],
			"source":    metric["source"],
			"available": true,
			"state":     "recorded",
			"value":     metric["value"],
		}
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
			metricSummary = append(metricSummary, map[string]any{
				"metric":    name,
				"source":    "",
				"available": false,
				"state":     "unavailable",
			})
		}
	}
	for _, metric := range metrics {
		if metric["source"] != "" {
			metricSummary = append(metricSummary, map[string]any{
				"metric":    metric["metric"],
				"source":    metric["source"],
				"available": true,
				"state":     "recorded",
				"value":     metric["value"],
			})
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
			result[name] = map[string]any{
				"available": false,
				"state":     "unavailable",
			}
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
		item := map[string]any{
			"available": true,
			"state":     "recorded",
			"value":     value,
		}
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
		result[source] = map[string]any{
			"available": true,
			"state":     "recorded",
			"value":     metric["value"],
		}
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
			result[field][status] = map[string]any{
				"available": false,
				"state":     "unavailable",
			}
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
		result[field][status] = map[string]any{
			"available": true,
			"state":     "recorded",
			"value":     metric["value"],
		}
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
	coverage, err := s.workStore.CurrentCoverage(ctx, runID)
	if err != nil {
		return nil, err
	}
	result := map[string]any{}
	result["work_revisions"] = map[string]any{"value": coverage.WorkRevisions, "available": true, "state": "derived"}
	result["journal_coverage"] = map[string]any{"value": coverage.JournalCoverage, "denominator": coverage.WorkRevisions, "percentage": percent(coverage.JournalCoverage, coverage.WorkRevisions), "available": true, "state": "derived"}
	return result, nil
}

// relationshipTotals counts canonical works, authorships, references, and resolved citations for a run.
func (s *Server) relationshipTotals(ctx context.Context, runID int64) (map[string]any, error) {
	totals, err := s.workStore.RelationshipTotals(ctx, runID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"work_revisions":          map[string]any{"value": totals.WorkRevisions, "available": true, "state": "derived"},
		"analysis_ready_articles": map[string]any{"value": totals.AnalysisReadyArticles, "available": true, "state": "derived"},
		"authorships":             map[string]any{"value": totals.Authorships, "available": true, "state": "derived"},
		"reference_mentions":      map[string]any{"value": totals.ReferenceMentions, "available": true, "state": "derived"},
		"internal_citations":      map[string]any{"value": totals.InternalCitations, "available": true, "state": "derived"},
	}, nil
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
