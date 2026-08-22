// audit.go provides the read-only HTTP handlers for browsing pipeline
// audit events, artifacts, and their inline or blob-stored payloads
// through the local viewer.
package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

const maxInlineArtifactBytes = 256 * 1024
const defaultInlineArtifactPreviewBytes = 64 * 1024
const auditListPayloadBytes = 4 * 1024
const auditDetailPayloadBytes = 64 * 1024

// runAudit returns run-scoped and eligible global audit events with filters and facets.
func (s *Server) runAudit(w http.ResponseWriter, r *http.Request) {
	if err := validateKnownQuery(r, "limit", "cursor"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	runID, err := positiveID(r.PathValue("id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	var found int
	if err := s.db.QueryRowContext(ctx, "SELECT 1 FROM pipeline_runs WHERE id=?", runID).Scan(&found); err == sql.ErrNoRows {
		s.respond(w, r, nil, notFound("run not found"))
		return
	} else if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	limit, err := reviewLimit(r)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	cursor := int64(0)
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		cursor, err = positiveID(raw)
		if err != nil {
			s.respond(w, r, nil, badRequest("cursor must be a positive audit event ID"))
			return
		}
	}
	query := `SELECT id, occurred_at, actor, pipeline_run_id, entity_type, entity_id, action,
		before_json, after_json, metadata_json, correlation_id
		FROM audit_events WHERE pipeline_run_id=?`
	args := []any{runID}
	if cursor > 0 {
		query += " AND id<?"
		args = append(args, cursor)
	}
	query += " ORDER BY id DESC LIMIT ?"
	args = append(args, limit+1)
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
	boundAuditEventPayloads(items, auditListPayloadBytes)
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	var nextCursor any
	if hasMore {
		nextCursor = items[len(items)-1]["id"]
	}
	s.respond(w, r, map[string]any{
		"run_id": runID, "events": items, "has_more": hasMore, "next_cursor": nextCursor,
		"deprecated": true, "replacement": "/api/audit?run_id=" + strconv.FormatInt(runID, 10),
	}, nil)
}

// audit validates filters and returns a cursor-paginated audit timeline with summary and facets.
func (s *Server) audit(w http.ResponseWriter, r *http.Request) {
	if err := validateKnownQuery(r, "run_id", "entity_type", "entity_id", "action", "actor", "category", "stage", "outcome", "q", "limit", "cursor", "pdf_scope", "review_status", "review_reason", "review_substatus"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	clauses, args := make([]string, 0, 10), make([]any, 0, 10)
	if value := r.URL.Query().Get("entity_id"); value != "" {
		clauses = append(clauses, "entity_id=?")
		args = append(args, value)
	}
	for _, filter := range []struct{ parameter, column string }{
		{"entity_type", "entity_type"}, {"action", "action"}, {"actor", "actor"},
	} {
		values, err := auditMultiValues(r.URL.Query().Get(filter.parameter), filter.parameter)
		if err != nil {
			s.respond(w, r, nil, err)
			return
		}
		if len(values) > 0 {
			clause, valueArgs := auditInClause(filter.column, values)
			clauses = append(clauses, clause)
			args = append(args, valueArgs...)
		}
	}
	pdfSelected := false
	if category := r.URL.Query().Get("category"); category != "" {
		categories, err := auditMultiValues(category, "category")
		if err != nil {
			s.respond(w, r, nil, err)
			return
		}
		categoryClauses := make([]string, 0, len(categories))
		for _, selected := range categories {
			switch selected {
			case "pipeline":
				categoryClauses = append(categoryClauses, "(action LIKE 'pipeline_%' OR action IN ('plan_created','duplicate_plan_skipped','run_started','step_reused','run_completed','run_failed','run_trashed','run_restored','run_purged','revision_config_changed'))")
			case "enrichment":
				categoryClauses = append(categoryClauses, "action IN ('field_enriched','cache_hit','network_fetch')")
			case "validation":
				categoryClauses = append(categoryClauses, "action LIKE 'validation_%'")
			case "pdf":
				pdfSelected = true
				categoryClauses = append(categoryClauses, "action LIKE 'pdf_%'")
			case "review":
				categoryClauses = append(categoryClauses, "(action LIKE 'review_%' OR action LIKE 'work_review_%')")
			default:
				s.respond(w, r, nil, badRequest("category values must be pipeline, enrichment, validation, review, or pdf"))
				return
			}
		}
		clauses = append(clauses, "("+strings.Join(categoryClauses, " OR ")+")")
	}
	if stage := r.URL.Query().Get("stage"); stage != "" {
		clauses = append(clauses, "CASE WHEN json_valid(metadata_json) THEN COALESCE(json_extract(metadata_json, '$.stage'), json_extract(metadata_json, '$.stage_name'), '') ELSE '' END=?")
		args = append(args, stage)
	}
	if outcome := r.URL.Query().Get("outcome"); outcome != "" {
		clauses = append(clauses, "CASE WHEN json_valid(metadata_json) THEN COALESCE(json_extract(metadata_json, '$.outcome'), json_extract(metadata_json, '$.status'), '') ELSE '' END=?")
		args = append(args, outcome)
	}
	if status := strings.TrimSpace(r.URL.Query().Get("review_status")); status != "" {
		if len(status) > 100 {
			s.respond(w, r, nil, badRequest("review_status is too long"))
			return
		}
		clauses = append(clauses, "CASE WHEN json_valid(after_json) THEN COALESCE(json_extract(after_json, '$.status'), '') ELSE '' END=?")
		args = append(args, status)
	}
	if reason := strings.TrimSpace(r.URL.Query().Get("review_reason")); reason != "" {
		if len(reason) > 1000 {
			s.respond(w, r, nil, badRequest("review_reason is too long"))
			return
		}
		clauses = append(clauses, "CASE WHEN json_valid(after_json) THEN COALESCE(json_extract(after_json, '$.reason'), '') ELSE '' END=?")
		args = append(args, reason)
	}
	if substatus := strings.TrimSpace(r.URL.Query().Get("review_substatus")); substatus != "" {
		if len(substatus) > 100 {
			s.respond(w, r, nil, badRequest("review_substatus is too long"))
			return
		}
		clauses = append(clauses, "json_valid(after_json) AND EXISTS (SELECT 1 FROM json_each(after_json, '$.sub_statuses') WHERE value=?)")
		args = append(args, substatus)
	}
	if query := strings.TrimSpace(r.URL.Query().Get("q")); query != "" {
		clauses = append(clauses, "(LOWER(actor) LIKE ? OR LOWER(entity_type) LIKE ? OR LOWER(entity_id) LIKE ? OR LOWER(action) LIKE ?)")
		needle := "%" + strings.ToLower(query) + "%"
		args = append(args, needle, needle, needle, needle)
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	var scopeClause string
	var scopeArgs []any
	pdfScope := r.URL.Query().Get("pdf_scope")
	if pdfScope == "" {
		pdfScope = "run"
	}
	if pdfScope != "run" && pdfScope != "workspace" {
		s.respond(w, r, nil, badRequest("pdf_scope must be run or workspace"))
		return
	}
	if pdfScope == "workspace" && !pdfSelected {
		s.respond(w, r, nil, badRequest("pdf_scope=workspace requires the PDF category"))
		return
	}
	if raw := r.URL.Query().Get("run_id"); raw != "" {
		runID, err := positiveID(raw)
		if err != nil {
			s.respond(w, r, nil, err)
			return
		}
		if pdfSelected && pdfScope == "workspace" {
			scopeClause = "(pipeline_run_id=? OR (pipeline_run_id IS NULL AND action LIKE 'pdf_%'))"
			scopeArgs = []any{runID}
		} else if pdfSelected {
			scopeClause = `(pipeline_run_id=? OR (
				pipeline_run_id IS NULL AND action LIKE 'pdf_%' AND entity_type='work'
				AND EXISTS (SELECT 1 FROM work_revisions scoped_revision
					WHERE scoped_revision.pipeline_run_id=?
					AND CAST(scoped_revision.work_id AS TEXT)=audit_events.entity_id)))`
			scopeArgs = []any{runID, runID}
		} else {
			scopeClause = "pipeline_run_id=?"
			scopeArgs = []any{runID}
		}
		clauses = append(clauses, scopeClause)
		args = append(args, scopeArgs...)
		if err := s.requireRun(ctx, runID); err != nil {
			s.respond(w, r, nil, err)
			return
		}
	}
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := parseOptionalInt(raw, "limit")
		if err != nil || parsed < 1 || parsed > 100 {
			s.respond(w, r, nil, badRequest("limit must be between 1 and 100"))
			return
		}
		limit = int(parsed)
	}
	where := auditWhere(clauses)
	var summary any
	var facets any
	if r.URL.Query().Get("cursor") == "" {
		var summaryErr error
		summary, summaryErr = s.auditSummary(ctx, where, args)
		if summaryErr != nil {
			s.respond(w, r, nil, summaryErr)
			return
		}
		actorFacets, facetErr := s.auditFacet(ctx, "actor", scopeClause, scopeArgs)
		if facetErr != nil {
			s.respond(w, r, nil, facetErr)
			return
		}
		actionFacets, facetErr := s.auditFacet(ctx, "action", scopeClause, scopeArgs)
		if facetErr != nil {
			s.respond(w, r, nil, facetErr)
			return
		}
		entityFacets, facetErr := s.auditFacet(ctx, "entity_type", scopeClause, scopeArgs)
		if facetErr != nil {
			s.respond(w, r, nil, facetErr)
			return
		}
		facets = map[string]any{"actors": actorFacets, "actions": actionFacets, "entity_types": entityFacets}
	}
	queryClauses := append([]string(nil), clauses...)
	queryArgs := append([]any(nil), args...)
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		cursor, err := positiveID(raw)
		if err != nil {
			s.respond(w, r, nil, badRequest("cursor must be a positive audit event ID"))
			return
		}
		var occurredAt string
		if err := s.db.QueryRowContext(ctx, "SELECT occurred_at FROM audit_events WHERE id=?", cursor).Scan(&occurredAt); err != nil {
			if err == sql.ErrNoRows {
				s.respond(w, r, nil, badRequest("cursor must identify an audit event"))
			} else {
				s.respond(w, r, nil, err)
			}
			return
		}
		queryClauses = append(queryClauses, "(COALESCE(julianday(occurred_at), 0)<COALESCE(julianday(?), 0) OR (COALESCE(julianday(occurred_at), 0)=COALESCE(julianday(?), 0) AND id<?))")
		queryArgs = append(queryArgs, occurredAt, occurredAt, cursor)
	}
	query := "SELECT id, occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, before_json, after_json, metadata_json, correlation_id FROM audit_events" + auditWhere(queryClauses) + " ORDER BY COALESCE(julianday(occurred_at), 0) DESC, id DESC LIMIT ?"
	queryArgs = append(queryArgs, limit+1)
	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
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
	boundAuditEventPayloads(items, auditListPayloadBytes)
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	var nextCursor any
	if hasMore && len(items) > 0 {
		nextCursor = items[len(items)-1]["id"]
	}
	s.respond(w, r, map[string]any{
		"events": items, "has_more": hasMore, "next_cursor": nextCursor,
		"summary": summary,
		"facets":  facets,
		"scope":   map[string]any{"run_id": nullableRunScope(r.URL.Query().Get("run_id")), "pdf_scope": pdfScope},
	}, nil)
}

// auditRecordedData returns one privacy-scrubbed, byte-bounded payload only after explicit expansion.
func (s *Server) auditRecordedData(w http.ResponseWriter, r *http.Request) {
	if err := validateKnownQuery(r, "run_id"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	eventID, err := positiveID(r.PathValue("id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	runID, err := requiredQueryID(r, "run_id")
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
	row, err := s.oneRow(ctx, `SELECT id, before_json, after_json, metadata_json FROM audit_events
		WHERE id=? AND (pipeline_run_id=? OR (
			pipeline_run_id IS NULL AND action LIKE 'pdf_%' AND entity_type='work'
			AND EXISTS (SELECT 1 FROM work_revisions scoped_revision
				WHERE scoped_revision.pipeline_run_id=?
				AND CAST(scoped_revision.work_id AS TEXT)=audit_events.entity_id)))`, eventID, runID, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if row == nil {
		s.respond(w, r, nil, notFound("audit event not found"))
		return
	}
	payload := map[string]any{"event_id": eventID, "byte_limit": auditDetailPayloadBytes}
	truncated := make([]string, 0)
	remaining := auditDetailPayloadBytes
	for _, field := range []string{"metadata_json", "before_json", "after_json"} {
		label := strings.TrimSuffix(field, "_json")
		value, size, wasTruncated := safeAuditJSON(row[field], remaining)
		if wasTruncated {
			truncated = append(truncated, label)
			payload[label] = nil
			continue
		}
		payload[label] = value
		remaining -= size
	}
	payload["truncated_fields"] = truncated
	s.respond(w, r, payload, nil)
}

// boundAuditEventPayloads removes private fields and keeps timeline pages within a fixed payload budget per field.
func boundAuditEventPayloads(items []map[string]any, limit int) {
	for _, item := range items {
		available := false
		truncated := make([]string, 0)
		for _, field := range []string{"metadata_json", "before_json", "after_json"} {
			if raw, ok := item[field].(string); ok && raw != "" {
				available = true
			}
			value, _, wasTruncated := safeAuditJSON(item[field], limit)
			if wasTruncated {
				item[field] = nil
				truncated = append(truncated, strings.TrimSuffix(field, "_json"))
				continue
			}
			if value == nil {
				item[field] = nil
				continue
			}
			encoded, err := json.Marshal(value)
			if err != nil {
				item[field] = nil
				continue
			}
			item[field] = string(encoded)
		}
		item["recorded_data_available"] = available
		item["recorded_data_truncated_fields"] = truncated
	}
}

// safeAuditJSON decodes and recursively removes prose and contact fields before enforcing the byte budget.
func safeAuditJSON(raw any, limit int) (any, int, bool) {
	text, ok := raw.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return nil, 0, false
	}
	var value any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		value = map[string]any{"state": "invalid_json", "message": "Stored recorded data is not valid JSON."}
	}
	value = scrubAuditValue(value)
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, 0, true
	}
	if len(encoded) > limit {
		return nil, len(encoded), true
	}
	return value, len(encoded), false
}

// scrubAuditValue recursively omits review prose, selected text, and reviewer contact fields.
func scrubAuditValue(raw any) any {
	private := map[string]bool{"note_body": true, "body": true, "selected_text": true, "reviewer_email": true, "email": true}
	switch value := raw.(type) {
	case []any:
		result := make([]any, len(value))
		for index, item := range value {
			result[index] = scrubAuditValue(item)
		}
		return result
	case map[string]any:
		result := make(map[string]any, len(value))
		for key, item := range value {
			if private[strings.ToLower(key)] {
				continue
			}
			result[key] = scrubAuditValue(item)
		}
		return result
	default:
		return raw
	}
}

// auditMultiValues parses, deduplicates, and bounds a comma-separated audit facet filter.
func auditMultiValues(raw, parameter string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	seen := make(map[string]bool)
	values := make([]string, 0)
	for _, item := range strings.Split(raw, ",") {
		value := strings.TrimSpace(item)
		if value == "" || len(value) > 200 {
			return nil, badRequest(parameter + " contains an invalid value")
		}
		if !seen[value] {
			seen[value] = true
			values = append(values, value)
		}
		if len(values) > 100 {
			return nil, badRequest(parameter + " accepts at most 100 values")
		}
	}
	return values, nil
}

// auditInClause builds a parameterized SQL IN clause for validated audit facet values.
func auditInClause(column string, values []string) (string, []any) {
	markers := make([]string, len(values))
	args := make([]any, len(values))
	for index, value := range values {
		markers[index] = "?"
		args[index] = value
	}
	return column + " IN (" + strings.Join(markers, ",") + ")", args
}

// auditWhere joins audit predicates into an optional SQL WHERE clause.
func auditWhere(clauses []string) string {
	if len(clauses) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(clauses, " AND ")
}

// auditSummary counts filtered audit events by presentation category.
func (s *Server) auditSummary(ctx context.Context, where string, args []any) (map[string]any, error) {
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_events"+where, args...).Scan(&total); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT action, COUNT(*) AS count FROM audit_events"+where+" GROUP BY action ORDER BY count DESC, action", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	actions, err := rowsAsMaps(rows)
	if err != nil {
		return nil, err
	}
	return map[string]any{"total_events": total, "actions": actions}, nil
}

// auditFacet returns distinct non-empty values for an allowlisted audit column and run scope.
func (s *Server) auditFacet(ctx context.Context, column, scopeClause string, scopeArgs []any) ([]string, error) {
	query := "SELECT DISTINCT COALESCE(" + column + ", '') FROM audit_events"
	if scopeClause != "" {
		query += " WHERE " + scopeClause
	}
	query += " ORDER BY " + column + " LIMIT 101"
	rows, err := s.db.QueryContext(ctx, query, scopeArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]string, 0)
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		if value != "" {
			values = append(values, value)
		}
		if len(values) == 100 {
			break
		}
	}
	return values, rows.Err()
}

// nullableRunScope preserves an invariant null-or-string scope value in audit responses.
func nullableRunScope(raw string) any {
	if raw == "" {
		return nil
	}
	return raw
}

// auditRows returns audit event rows matching a caller-supplied parameterized condition.
func (s *Server) auditRows(ctx context.Context, condition string, args ...any) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, before_json, after_json, metadata_json, correlation_id FROM audit_events WHERE "+condition+" ORDER BY COALESCE(julianday(occurred_at), 0) DESC, id DESC", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return rowsAsMaps(rows)
}

// trash returns a bounded compatibility view of trashed runs.
func (s *Server) trash(w http.ResponseWriter, r *http.Request) {
	if err := validateKnownQuery(r); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	rows, err := s.db.QueryContext(ctx, `SELECT id, execution_plan_id, attempt_number, status, started_at, finished_at, trashed_at, trash_reason
		FROM pipeline_runs WHERE visibility_state='trashed' ORDER BY trashed_at DESC, id DESC LIMIT ?`, legacyDiscoveryLimit+1)
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
		"runs": items, "restore_allowed": false, "has_more": hasMore, "limit": legacyDiscoveryLimit,
		"deprecated": true, "replacement": "/api/hierarchy?section=runs&visibility=trashed",
	}, nil)
}

// runArtifacts returns artifact metadata linked to the selected run.
func (s *Server) runArtifacts(w http.ResponseWriter, r *http.Request) {
	if err := validateKnownQuery(r, "limit", "cursor", "q", "role", "artifact_id"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	runID, err := positiveID(r.PathValue("id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	type artifactContext struct {
		SearchID             string `json:"search_id"`
		SearchRevisionID     int64  `json:"search_revision_id"`
		SearchRevisionLabel  string `json:"search_revision_label"`
		ExecutionPlanID      int64  `json:"execution_plan_id"`
		ExecutionFingerprint string `json:"execution_fingerprint"`
		RunID                int64  `json:"run_id"`
		AttemptNumber        int64  `json:"attempt_number"`
	}
	var runContext artifactContext
	err = s.db.QueryRowContext(ctx, `SELECT s.search_id, sr.id, sr.revision_label,
            ep.id, ep.execution_fingerprint, pr.id, pr.attempt_number
        FROM pipeline_runs pr
        JOIN execution_plans ep ON ep.id=pr.execution_plan_id
        JOIN search_revisions sr ON sr.id=ep.search_revision_id
        JOIN searches s ON s.id=sr.search_id
        WHERE pr.id=?`, runID).Scan(
		&runContext.SearchID, &runContext.SearchRevisionID, &runContext.SearchRevisionLabel,
		&runContext.ExecutionPlanID, &runContext.ExecutionFingerprint, &runContext.RunID,
		&runContext.AttemptNumber,
	)
	if err == sql.ErrNoRows {
		s.respond(w, r, nil, notFound("run not found"))
		return
	}
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	cursor, limit, err := reviewIDPage(r, "run_artifacts_"+stringID(runID))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	searchQuery := strings.TrimSpace(r.URL.Query().Get("q"))
	role := strings.TrimSpace(r.URL.Query().Get("role"))
	focusID, err := optionalHierarchyID(r, "artifact_id")
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	const relationshipsSQL = `WITH artifact_relationships AS (
			SELECT artifact_id, 'run_role' AS relationship_role, artifact_role AS relationship_detail
			FROM run_artifacts WHERE pipeline_run_id=?
			UNION ALL
			SELECT input_artifact_id, 'step_input', step_name FROM run_steps
			WHERE pipeline_run_id=? AND input_artifact_id IS NOT NULL
			UNION ALL
			SELECT output_artifact_id, 'step_output', step_name FROM run_steps
			WHERE pipeline_run_id=? AND output_artifact_id IS NOT NULL
			UNION ALL
			SELECT ce.payload_artifact_id, 'cache_payload', ce.provider || ':' || ce.namespace
			FROM run_cache_uses use_record
			JOIN cache_entries ce ON ce.id=use_record.cache_entry_id
			WHERE use_record.pipeline_run_id=? AND ce.payload_artifact_id IS NOT NULL
			UNION ALL
			SELECT candidate.payload_artifact_id, 'identity_candidate_payload', resolution.provider
			FROM author_identity_resolutions resolution
			JOIN author_identity_candidates candidate ON candidate.identity_resolution_id=resolution.id
			WHERE resolution.pipeline_run_id=? AND candidate.payload_artifact_id IS NOT NULL
		), selected_artifacts AS (
			SELECT DISTINCT artifact_id FROM artifact_relationships
		)`
	query := relationshipsSQL + `
		SELECT a.id, a.content_hash, a.byte_size, a.content_type, a.created_at,
		       (ab.id IS NOT NULL) AS has_blob,
		       COALESCE((SELECT GROUP_CONCAT(role.artifact_role, ', ') FROM (
		           SELECT DISTINCT artifact_role FROM run_artifacts
		           WHERE pipeline_run_id=? AND artifact_id=a.id ORDER BY artifact_role
		       ) role), '') AS artifact_roles,
		       COALESCE((SELECT GROUP_CONCAT(relationship_role, ', ') FROM (
		           SELECT DISTINCT relationship_role FROM artifact_relationships
		           WHERE artifact_id=a.id ORDER BY relationship_role
		       )), '') AS relationship_roles,
		       COALESCE((SELECT GROUP_CONCAT(step_name, ', ') FROM (
		           SELECT DISTINCT step_name FROM run_steps
                   WHERE pipeline_run_id=? AND output_artifact_id=a.id
                   ORDER BY step_name
               )), '') AS produced_by_steps,
               COALESCE((SELECT GROUP_CONCAT(step_name, ', ') FROM (
                   SELECT DISTINCT step_name FROM run_steps
                   WHERE pipeline_run_id=? AND input_artifact_id=a.id
                   ORDER BY step_name
               )), '') AS consumed_by_steps
		FROM selected_artifacts selected
		JOIN artifacts a ON a.id=selected.artifact_id
		LEFT JOIN artifact_blobs ab ON ab.artifact_id=a.id
		WHERE a.id>?`
	args := []any{runID, runID, runID, runID, runID, runID, runID, runID, cursor}
	if searchQuery != "" {
		query += ` AND (LOWER(a.content_hash) LIKE ? OR LOWER(a.content_type) LIKE ?
			OR EXISTS (SELECT 1 FROM artifact_relationships searchable
				WHERE searchable.artifact_id=a.id AND (LOWER(searchable.relationship_role) LIKE ? OR LOWER(searchable.relationship_detail) LIKE ?)))`
		pattern := "%" + strings.ToLower(searchQuery) + "%"
		args = append(args, pattern, pattern, pattern, pattern)
	}
	if role != "" {
		query += ` AND EXISTS (SELECT 1 FROM artifact_relationships filtered_role
			WHERE filtered_role.artifact_id=a.id AND filtered_role.relationship_role=?)`
		args = append(args, role)
	}
	if cursor > 0 && focusID > 0 {
		query += " AND a.id!=?"
		args = append(args, focusID)
	}
	query += ` GROUP BY a.id, a.content_hash, a.byte_size, a.content_type, a.created_at, ab.id
		ORDER BY CASE WHEN a.id=? THEN 0 ELSE 1 END, a.id ASC LIMIT ?`
	args = append(args, focusID)
	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	defer rows.Close()
	items, err := rowsAsMaps(rows)
	if err == nil {
		for _, item := range items {
			contentType, _ := item["content_type"].(string)
			hasBlob, _ := item["has_blob"].(int64)
			item["preview_available"] = hasBlob != 0 && inlineArtifactContentType(contentType)
			item["preview_limit_bytes"] = defaultInlineArtifactPreviewBytes
		}
	}
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	var nextCursor any
	if hasMore {
		value := encodeReviewCursor(reviewCursor{Kind: "run_artifacts_" + stringID(runID), ID: items[len(items)-1]["id"].(int64)})
		nextCursor = value
	}
	s.respond(w, r, map[string]any{
		"run_id": runID, "context": runContext, "artifacts": items,
		"has_more": hasMore, "next_cursor": nextCursor, "limit": limit,
		"filters": map[string]any{"q": searchQuery, "role": role, "artifact_id": nullablePositiveID(focusID)},
	}, nil)
}

// nullablePositiveID preserves an invariant null-or-number response for optional focused records.
func nullablePositiveID(id int64) any {
	if id < 1 {
		return nil
	}
	return id
}

// artifactContent streams one stored artifact blob with a safe content disposition.
func (s *Server) artifactContent(w http.ResponseWriter, r *http.Request) {
	artifactID, err := positiveID(r.PathValue("id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	var contentType string
	var storedSize int64
	var hasBlob bool
	err = s.db.QueryRowContext(ctx, `SELECT a.content_type, ab.id IS NOT NULL,
		COALESCE(length(CAST(ab.data AS BLOB)), 0)
		FROM artifacts a LEFT JOIN artifact_blobs ab ON ab.artifact_id=a.id
		WHERE a.id=?`, artifactID).Scan(&contentType, &hasBlob, &storedSize)
	if err == sql.ErrNoRows {
		s.respond(w, r, nil, notFound("artifact not found"))
		return
	}
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if !hasBlob {
		s.respond(w, r, nil, notFound("artifact has no blob data"))
		return
	}
	w.Header().Set("Content-Type", normalizedArtifactContentType(contentType))
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": s.artifactFilename(ctx, artifactID, contentType)}))
	w.Header().Set("Content-Length", strconv.FormatInt(storedSize, 10))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	const chunkBytes = 64 * 1024
	for offset := int64(0); offset < storedSize; offset += chunkBytes {
		var chunk []byte
		if err := s.db.QueryRowContext(ctx, `SELECT substr(CAST(data AS BLOB), ?, ?)
			FROM artifact_blobs WHERE artifact_id=?`, offset+1, chunkBytes, artifactID).Scan(&chunk); err != nil {
			return
		}
		if _, err := w.Write(chunk); err != nil {
			return
		}
	}
}

// artifactInspection returns bounded metadata and preview content for one artifact.
func (s *Server) artifactInspection(w http.ResponseWriter, r *http.Request) {
	if err := validateKnownQuery(r, "preview_bytes"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	artifactID, err := positiveID(r.PathValue("id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	previewBytes := defaultInlineArtifactPreviewBytes
	if raw := r.URL.Query().Get("preview_bytes"); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed < 1 || parsed > maxInlineArtifactBytes {
			s.respond(w, r, nil, badRequest("preview_bytes must be between 1 and 262144"))
			return
		}
		previewBytes = parsed
	}
	contentType, byteSize, blobSize, data, err := s.artifactPreviewBlob(ctx, artifactID, previewBytes)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if !inlineArtifactContentType(contentType) {
		s.respond(w, r, nil, badRequest("artifact is not available for inline inspection; download it instead"))
		return
	}
	for trims := 0; trims < utf8.UTFMax && len(data) > 0 && !utf8.Valid(data); trims++ {
		data = data[:len(data)-1]
	}
	if !utf8.Valid(data) {
		s.respond(w, r, nil, badRequest("artifact preview is not valid UTF-8; download it instead"))
		return
	}
	format := "text"
	if jsonArtifactContentType(contentType) {
		format = "json"
	}
	s.respond(w, r, map[string]any{
		"artifact_id":         artifactID,
		"content_type":        normalizedArtifactContentType(contentType),
		"byte_size":           byteSize,
		"stored_byte_size":    blobSize,
		"preview_byte_size":   len(data),
		"preview_limit_bytes": previewBytes,
		"truncated":           blobSize > int64(len(data)),
		"format":              format,
		"content":             string(data),
	}, nil)
}

// artifactPreviewBlob reads a bounded artifact prefix together with its media type and total size.
func (s *Server) artifactPreviewBlob(ctx context.Context, artifactID int64, previewBytes int) (string, int64, int64, []byte, error) {
	var contentType string
	var byteSize, blobSize int64
	var hasBlob bool
	var data []byte
	err := s.db.QueryRowContext(ctx, `SELECT a.content_type, a.byte_size,
			ab.id IS NOT NULL, COALESCE(length(CAST(ab.data AS BLOB)), 0), substr(CAST(ab.data AS BLOB), 1, ?)
        FROM artifacts a
        LEFT JOIN artifact_blobs ab ON ab.artifact_id=a.id
        WHERE a.id=?`, previewBytes, artifactID).Scan(&contentType, &byteSize, &hasBlob, &blobSize, &data)
	if err == sql.ErrNoRows {
		return "", 0, 0, nil, notFound("artifact not found")
	}
	if err != nil {
		return "", 0, 0, nil, err
	}
	if !hasBlob {
		return "", 0, 0, nil, notFound("artifact has no blob data")
	}
	return contentType, byteSize, blobSize, data, nil
}

// artifactBlob returns the complete content-addressed blob for one run artifact.
func (s *Server) artifactBlob(ctx context.Context, artifactID int64) (string, int64, []byte, error) {
	var contentType string
	var byteSize int64
	var hasBlob bool
	var data []byte
	err := s.db.QueryRowContext(ctx, `SELECT a.content_type, a.byte_size,
            ab.id IS NOT NULL, ab.data
        FROM artifacts a
        LEFT JOIN artifact_blobs ab ON ab.artifact_id=a.id
        WHERE a.id=?`, artifactID).Scan(&contentType, &byteSize, &hasBlob, &data)
	if err == sql.ErrNoRows {
		return "", 0, nil, notFound("artifact not found")
	}
	if err != nil {
		return "", 0, nil, err
	}
	if !hasBlob {
		return "", 0, nil, notFound("artifact has no blob data")
	}
	return contentType, byteSize, data, nil
}

// normalizedArtifactContentType parses and lowercases an artifact media type without parameters.
func normalizedArtifactContentType(contentType string) string {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType == "" {
		return "application/octet-stream"
	}
	return mediaType
}

// jsonArtifactContentType reports whether a normalized media type carries JSON.
func jsonArtifactContentType(contentType string) bool {
	mediaType := normalizedArtifactContentType(contentType)
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

// inlineArtifactContentType reports whether a normalized media type is safe for inline display.
func inlineArtifactContentType(contentType string) bool {
	mediaType := normalizedArtifactContentType(contentType)
	if mediaType == "text/html" || mediaType == "application/xhtml+xml" {
		return false
	}
	return strings.HasPrefix(mediaType, "text/") || jsonArtifactContentType(mediaType) || mediaType == "application/x-something-config"
}

// artifactFilename derives a safe download filename from artifact metadata and media type.
func (s *Server) artifactFilename(ctx context.Context, artifactID int64, contentType string) string {
	var role string
	_ = s.db.QueryRowContext(ctx, `SELECT artifact_role FROM run_artifacts
        WHERE artifact_id=? ORDER BY pipeline_run_id, artifact_role LIMIT 1`, artifactID).Scan(&role)
	name := "artifact-" + strconv.FormatInt(artifactID, 10)
	switch role {
	case "workspace_config":
		name = "workspace-config-" + strconv.FormatInt(artifactID, 10)
	case "resolved_manifest":
		name = "resolved-manifest-" + strconv.FormatInt(artifactID, 10)
	case "input_manifest":
		name = "input-manifest-" + strconv.FormatInt(artifactID, 10)
	}
	mediaType := normalizedArtifactContentType(contentType)
	switch {
	case role == "workspace_config" || mediaType == "application/x-something-config":
		return name + ".something"
	case jsonArtifactContentType(mediaType):
		return name + ".json"
	case strings.HasPrefix(mediaType, "text/"):
		return name + ".txt"
	default:
		return name + ".bin"
	}
}

// runCacheUses returns cache-use evidence recorded for the selected run.
func (s *Server) runCacheUses(w http.ResponseWriter, r *http.Request) {
	runID, err := positiveID(r.PathValue("id"))
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
	fields := map[string]string{
		"id": "rcu.id", "cache_layer": "rcu.cache_layer", "outcome": "rcu.outcome", "used_at": "rcu.used_at",
		"cache_entry_id": "ce.id", "provider": "ce.provider", "namespace": "ce.namespace", "request_fingerprint": "ce.request_fingerprint",
		"response_status": "ce.response_status", "payload_artifact_id": "ce.payload_artifact_id", "fetched_at": "ce.fetched_at", "expires_at": "ce.expires_at", "extractor_version": "ce.extractor_version",
	}
	page, perPage, sort, order, query, err := scopedRowsRequest(r, fields, "id")
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	where, args := scopedWhere("rcu.pipeline_run_id=?", "rcu.cache_layer, rcu.outcome, ce.provider, ce.namespace, ce.request_fingerprint", runID, query)
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM run_cache_uses rcu JOIN cache_entries ce ON ce.id=rcu.cache_entry_id WHERE `+where, args...).Scan(&total); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	page = clampScopedPage(page, perPage, total)
	args = append(args, perPage, (page-1)*perPage)
	rows, err := s.db.QueryContext(ctx, `SELECT rcu.id, rcu.cache_layer, rcu.outcome, rcu.used_at,
        ce.id AS cache_entry_id, ce.provider, ce.namespace, ce.request_fingerprint,
        ce.response_status, ce.payload_artifact_id, ce.fetched_at, ce.expires_at, ce.extractor_version
        FROM run_cache_uses rcu JOIN cache_entries ce ON ce.id=rcu.cache_entry_id
		WHERE `+where+` ORDER BY `+stableScopedOrder(fields[sort], "rcu.id", order)+` LIMIT ? OFFSET ?`, args...)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	defer rows.Close()
	items, err := rowsAsMaps(rows)
	columns := []string{"id", "cache_layer", "outcome", "used_at", "cache_entry_id", "provider", "namespace", "request_fingerprint", "response_status", "payload_artifact_id", "fetched_at", "expires_at", "extractor_version"}
	s.respond(w, r, map[string]any{
		"run_id": runID, "columns": columns, "rows": items, "cache_uses": items,
		"pagination": scopedPagination(page, perPage, total, sort, order),
	}, err)
}
