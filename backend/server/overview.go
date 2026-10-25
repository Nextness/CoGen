// overview.go provides the run overview endpoint that returns pipeline
// metrics, enrichment breakdowns, stage outcomes, and per-source
// document counts for a selected run.
package server

import (
	viewrequest "analysis/backend/server/request"
	"analysis/internal/jsonvalue"
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"analysis/backend/database/run"
	"analysis/backend/database/source"
	"analysis/backend/server/metricsummary"
)

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
			"query":                   jsonvalue.Optional(row.Query),
			"expected_result_count":   jsonvalue.Optional(row.ExpectedResultCount),
			"observed_result_count":   jsonvalue.Optional(row.ObservedResultCount),
			"result_count_comparison": jsonvalue.Optional(row.ResultCountComparison),
			"export_date":             jsonvalue.Optional(row.ExportDate),
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
				corpusID, reviewErr = s.writeDB.Review.CorpusID(ctx)
				reviewWritable = reviewErr == nil
			}
		}
	}
	pdfBound := s.pdfDB != nil
	pdfReadable := false
	if pdfBound {
		pdfReadable = s.pdfDB.PingContext(ctx) == nil
	}
	pdfWritable := pdfBound && pdfReadable && s.pdfWritable
	pdfWriteReason := s.pdfWriteReason
	if pdfBound && !pdfReadable && pdfWriteReason == "" {
		pdfWriteReason = "PDF store is not readable"
	}
	pdfAuditPending := 0
	if s.pdfStore != nil {
		if count, err := s.pdfStore.PendingAuditCount(ctx); err == nil {
			pdfAuditPending = count
		}
	}
	s.auditRelayMu.Lock()
	auditRelayRunning := s.auditRelayCancel != nil
	auditRelayError := s.auditRelayError
	s.auditRelayMu.Unlock()
	s.respond(w, r, map[string]any{
		"readable":                metadataReadable,
		"metadata_readable":       metadataReadable,
		"table_count":             len(s.tables),
		"tables":                  s.tableNames(),
		"corpus_id":               corpusID,
		"review_writable":         reviewWritable,
		"pdf_store_bound":         pdfBound,
		"pdf_store_readable":      pdfReadable,
		"pdf_store_writable":      pdfWritable,
		"pdf_store_write_reason":  pdfWriteReason,
		"pdf_audit_pending":       pdfAuditPending,
		"pdf_audit_relay_running": auditRelayRunning,
		"pdf_audit_relay_error":   auditRelayError,
		"schema": map[string]any{
			"metadata": map[string]any{
				"actual": s.metadataActual,
				"target": s.metadataTarget,
				"ready":  s.metadataReady,
			},
			"pdf": map[string]any{
				"actual": s.pdfActual,
				"target": s.pdfTarget,
				"ready":  s.pdfReady,
			},
			"ready": s.schemaReady,
		},
		"review": map[string]any{
			"available":           reviewWritable,
			"metadata_writable":   reviewWritable,
			"pdf_store_bound":     pdfBound,
			"pdf_store_readable":  pdfReadable,
			"pdf_store_read_only": pdfBound && pdfReadable && !pdfWritable,
		},
	}, metadataErr)
}

// searches returns a bounded compatibility view of searches and their newest revisions.
func (s *Server) searches(w http.ResponseWriter, r *http.Request) {
	if err := viewrequest.KnownQuery(r); err != nil {
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
	if err := viewrequest.KnownQuery(r, "search_revision_id"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	id, err := viewrequest.RequiredID(r, "search_revision_id")
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
	if err := viewrequest.KnownQuery(r, "search_revision_id", "plan_id", "include_trashed"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	includeTrashed := r.URL.Query().Get("include_trashed") == "true"
	if raw := r.URL.Query().Get("include_trashed"); raw != "" && raw != "true" && raw != "false" {
		s.respond(w, r, nil, viewrequest.BadRequest("include_trashed must be true or false"))
		return
	}
	filter := run.LegacyRunFilter{IncludeTrashed: includeTrashed, Limit: legacyDiscoveryLimit + 1}
	if raw := r.URL.Query().Get("search_revision_id"); raw != "" {
		id, err := viewrequest.PositiveID(raw)
		if err != nil {
			s.respond(w, r, nil, err)
			return
		}
		filter.SearchRevisionID = id
	}
	if raw := r.URL.Query().Get("plan_id"); raw != "" {
		id, err := viewrequest.PositiveID(raw)
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
			"finished_at":        jsonvalue.Optional(row.FinishedAt),
			"status":             row.Status,
			"summary":            jsonvalue.Optional(row.Summary),
			"search_query":       jsonvalue.Optional(row.SearchQuery),
			"execution_plan_id":  jsonvalue.Optional(row.ExecutionPlanID),
			"attempt_number":     jsonvalue.Optional(row.AttemptNumber),
			"visibility_state":   row.VisibilityState,
			"trashed_at":         jsonvalue.Optional(row.TrashedAt),
			"trash_reason":       jsonvalue.Optional(row.TrashReason),
			"search_revision_id": jsonvalue.Optional(row.SearchRevisionID),
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
	runID, err := viewrequest.PositiveID(r.PathValue("id"))
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
		s.respond(w, r, nil, viewrequest.NotFound("run context not found"))
		return
	}
	runWritable := context.Run.Status == "completed" && context.Run.VisibilityState != "trashed"
	s.respond(w, r, runContextPayload(context, runWritable), nil)
}

// overview returns captured metrics, coverage, relationships, and source evidence for a run.
func (s *Server) overview(w http.ResponseWriter, r *http.Request) {
	runID, err := viewrequest.RequiredID(r, "run_id")
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
		s.respond(w, r, nil, viewrequest.NotFound("run not found"))
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
	report := metricsummary.Build(metrics)
	report["run_id"] = runID
	report["source_result_counts"] = sourceResultCounts
	report["source_filter_counts"] = sourceFilterCounts
	report["source_filter_diagnostics"] = sourceFilterDiagnostics
	report["current_coverage"] = coverage
	report["relationship_totals"] = breakdown
	s.respond(w, r, report, nil)

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
