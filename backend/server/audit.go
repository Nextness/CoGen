// audit.go provides the read-only HTTP handlers for browsing pipeline
// audit events, artifacts, and their inline or blob-stored payloads
// through the local viewer.
package server

import (
	viewrequest "analysis/backend/server/request"
	"analysis/internal/artifactmedia"
	"analysis/internal/jsonvalue"
	"context"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"analysis/backend/database/artifact"
	"analysis/backend/database/audit"
	"analysis/backend/database/cache"
	"analysis/internal/textlimit"
)

const maxInlineArtifactBytes = 256 * 1024
const defaultInlineArtifactPreviewBytes = 64 * 1024
const auditListPayloadBytes = 4 * 1024
const auditDetailPayloadBytes = 64 * 1024

// audit validates filters and returns a cursor-paginated audit timeline with summary and facets.
func (s *Server) audit(w http.ResponseWriter, r *http.Request) {
	if err := viewrequest.KnownQuery(r, "run_id", "entity_type", "entity_id", "action", "actor", "category", "stage", "outcome", "q", "limit", "cursor", "pdf_scope", "review_status", "review_reason", "review_substatus"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	filter := audit.Filter{EntityID: r.URL.Query().Get("entity_id")}
	var err error
	if filter.EntityTypes, err = viewrequest.MultiValues(r.URL.Query().Get("entity_type"), "entity_type"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if filter.Actions, err = viewrequest.MultiValues(r.URL.Query().Get("action"), "action"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if filter.Actors, err = viewrequest.MultiValues(r.URL.Query().Get("actor"), "actor"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if filter.Categories, err = viewrequest.MultiValues(r.URL.Query().Get("category"), "category"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	for _, category := range filter.Categories {
		switch category {
		case "pipeline", "enrichment", "validation", "pdf", "review":
		default:
			s.respond(w, r, nil, viewrequest.BadRequest("category values must be pipeline, enrichment, validation, review, or pdf"))
			return
		}
		if category == "pdf" {
			filter.PDFSelected = true
		}
	}
	filter.Stage = r.URL.Query().Get("stage")
	filter.Outcome = r.URL.Query().Get("outcome")
	filter.ReviewStatus = strings.TrimSpace(r.URL.Query().Get("review_status"))
	if len(filter.ReviewStatus) > 100 {
		s.respond(w, r, nil, viewrequest.BadRequest("review_status is too long"))
		return
	}
	filter.ReviewReason = strings.TrimSpace(r.URL.Query().Get("review_reason"))
	if len(filter.ReviewReason) > 1000 {
		s.respond(w, r, nil, viewrequest.BadRequest("review_reason is too long"))
		return
	}
	filter.ReviewSubstatus = strings.TrimSpace(r.URL.Query().Get("review_substatus"))
	if len(filter.ReviewSubstatus) > 100 {
		s.respond(w, r, nil, viewrequest.BadRequest("review_substatus is too long"))
		return
	}
	filter.Query = strings.TrimSpace(r.URL.Query().Get("q"))
	if len(filter.Query) > viewrequest.SearchLimit {
		s.respond(w, r, nil, viewrequest.BadRequest("q must be at most 200 characters"))
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	filter.PDFScope = r.URL.Query().Get("pdf_scope")
	if filter.PDFScope == "" {
		filter.PDFScope = "run"
	}
	if filter.PDFScope != "run" && filter.PDFScope != "workspace" {
		s.respond(w, r, nil, viewrequest.BadRequest("pdf_scope must be run or workspace"))
		return
	}
	if filter.PDFScope == "workspace" && !filter.PDFSelected {
		s.respond(w, r, nil, viewrequest.BadRequest("pdf_scope=workspace requires the PDF category"))
		return
	}
	if raw := r.URL.Query().Get("run_id"); raw != "" {
		runID, err := viewrequest.PositiveID(raw)
		if err != nil {
			s.respond(w, r, nil, err)
			return
		}
		filter.RunID = runID
		if err := s.requireRun(ctx, runID); err != nil {
			s.respond(w, r, nil, err)
			return
		}
	}
	filter.Limit = 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := viewrequest.OptionalInt(raw, "limit")
		if err != nil || parsed < 1 || parsed > 100 {
			s.respond(w, r, nil, viewrequest.BadRequest("limit must be between 1 and 100"))
			return
		}
		filter.Limit = int(parsed)
	}
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		cursor, err := viewrequest.PositiveID(raw)
		if err != nil {
			s.respond(w, r, nil, viewrequest.BadRequest("cursor must be a positive audit event ID"))
			return
		}
		filter.Cursor = cursor
	}
	var summary any
	var facets any
	if filter.Cursor == 0 {
		summaryResult, err := s.auditStore.Summary(ctx, filter)
		if err != nil {
			s.respond(w, r, nil, err)
			return
		}
		summary = auditSummaryPayload(summaryResult)
		facetResult, err := s.auditStore.Facets(ctx, filter)
		if err != nil {
			s.respond(w, r, nil, err)
			return
		}
		facets = map[string]any{"actors": facetResult.Actors, "actions": facetResult.Actions, "entity_types": facetResult.EntityTypes}
	}
	events, err := s.auditStore.List(ctx, filter)
	if errors.Is(err, audit.ErrCursorNotFound) {
		s.respond(w, r, nil, viewrequest.BadRequest("cursor must identify an audit event"))
		return
	}
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	items := make([]map[string]any, 0, len(events))
	for _, event := range events {
		items = append(items, auditEventMap(event))
	}
	boundAuditEventPayloads(items, auditListPayloadBytes)
	hasMore := len(items) > filter.Limit
	if hasMore {
		items = items[:filter.Limit]
	}
	var nextCursor any
	if hasMore && len(items) > 0 {
		nextCursor = items[len(items)-1]["id"]
	}
	s.respond(w, r, map[string]any{
		"events": items, "has_more": hasMore, "next_cursor": nextCursor,
		"summary": summary,
		"facets":  facets,
		"scope":   map[string]any{"run_id": nullableRunScope(r.URL.Query().Get("run_id")), "pdf_scope": filter.PDFScope},
	}, nil)
}

// auditEventMap renders one audit family event as the viewer's JSON row shape.
func auditEventMap(event *audit.Event) map[string]any {
	item := map[string]any{
		"id":          event.ID,
		"occurred_at": event.OccurredAt,
		"actor":       event.Actor,
		"entity_type": event.EntityType,
		"entity_id":   event.EntityID,
		"action":      event.Action,
	}
	if event.PipelineRunID != nil {
		item["pipeline_run_id"] = *event.PipelineRunID
	} else {
		item["pipeline_run_id"] = nil
	}
	for key, value := range map[string]string{
		"before_json": event.BeforeJSON, "after_json": event.AfterJSON,
		"metadata_json": event.MetadataJSON, "correlation_id": event.CorrelationID,
	} {
		if value != "" {
			item[key] = value
		} else {
			item[key] = nil
		}
	}
	return item
}

// auditSummaryPayload renders the family summary as the viewer's JSON shape.
func auditSummaryPayload(summary *audit.Summary) map[string]any {
	actions := make([]map[string]any, 0, len(summary.Actions))
	for _, action := range summary.Actions {
		actions = append(actions, map[string]any{"action": action.Action, "count": action.Count})
	}
	return map[string]any{"total_events": summary.TotalEvents, "actions": actions}
}

// auditRecordedData returns one privacy-scrubbed, byte-bounded payload only after explicit expansion.
func (s *Server) auditRecordedData(w http.ResponseWriter, r *http.Request) {
	if err := viewrequest.KnownQuery(r, "run_id"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	eventID, err := viewrequest.PositiveID(r.PathValue("id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	runID, err := viewrequest.RequiredID(r, "run_id")
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
	data, err := s.auditStore.RecordedData(ctx, eventID, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if data == nil {
		s.respond(w, r, nil, viewrequest.NotFound("audit event not found"))
		return
	}
	payload := map[string]any{"event_id": eventID, "byte_limit": auditDetailPayloadBytes}
	truncated := make([]string, 0)
	remaining := auditDetailPayloadBytes
	for _, field := range []struct{ label, value string }{
		{"metadata", data.MetadataJSON}, {"before", data.BeforeJSON}, {"after", data.AfterJSON},
	} {
		value, size, wasTruncated := safeAuditJSON(field.value, remaining)
		if wasTruncated {
			truncated = append(truncated, field.label)
			payload[field.label] = nil
			continue
		}
		payload[field.label] = value
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

// nullableRunScope preserves an invariant null-or-string scope value in audit responses.
func nullableRunScope(raw string) any {
	if raw == "" {
		return nil
	}
	return raw
}

// runArtifacts returns artifact metadata linked to the selected run.
func (s *Server) runArtifacts(w http.ResponseWriter, r *http.Request) {
	runID, err := viewrequest.PositiveID(r.PathValue("id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	runContext, err := s.runStore.RunArtifactContext(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if runContext == nil {
		s.respond(w, r, nil, viewrequest.NotFound("run not found"))
		return
	}
	pageMode := r.URL.Query().Has("page") || r.URL.Query().Has("per_page")
	page, perPage, order := 1, 50, "ASC"
	cursor, limit := int64(0), 25
	searchQuery := strings.TrimSpace(r.URL.Query().Get("q"))
	if pageMode {
		fields := map[string]string{"id": "a.id"}
		parsedPage, parsedPerPage, _, parsedOrder, parsedQuery, parseErr := viewrequest.Rows(r, fields, "id", "role", "artifact_id")
		if parseErr != nil {
			s.respond(w, r, nil, parseErr)
			return
		}
		page, perPage, order, searchQuery = parsedPage, parsedPerPage, parsedOrder, parsedQuery
		limit = perPage
	} else {
		if err := viewrequest.KnownQuery(r, "limit", "cursor", "q", "role", "artifact_id"); err != nil {
			s.respond(w, r, nil, err)
			return
		}
		cursor, limit, err = reviewIDPage(r, "run_artifacts_"+stringID(runID))
		if err != nil {
			s.respond(w, r, nil, err)
			return
		}
	}
	role := strings.TrimSpace(r.URL.Query().Get("role"))
	focusID, err := optionalHierarchyID(r, "artifact_id")
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	result, err := s.artifact.ListRunArtifactEvidence(ctx, artifact.RunArtifactFilter{
		RunID: runID, Query: searchQuery, Role: role, CursorID: cursor, FocusID: focusID,
		Page: page, PerPage: perPage, PageMode: pageMode, Order: order, Limit: limit,
	})
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	items := make([]map[string]any, 0, len(result.Items))
	for _, record := range result.Items {
		hasBlob := int64(0)
		if record.HasBlob {
			hasBlob = 1
		}
		item := map[string]any{
			"id":                 record.ID,
			"content_hash":       record.ContentHash,
			"byte_size":          record.ByteSize,
			"content_type":       record.ContentType,
			"created_at":         record.CreatedAt,
			"has_blob":           hasBlob,
			"artifact_roles":     record.ArtifactRoles,
			"relationship_roles": record.RelationshipRoles,
			"produced_by_steps":  record.ProducedBySteps,
			"consumed_by_steps":  record.ConsumedBySteps,
		}
		item["preview_available"] = record.HasBlob && artifactmedia.Inspect(record.ContentType).Inline
		item["preview_limit_bytes"] = defaultInlineArtifactPreviewBytes
		items = append(items, item)
	}
	page = result.Page
	hasMore := result.HasMore
	var nextCursor any
	if !pageMode && hasMore {
		nextCursor = encodeCursor(reviewCursor{Kind: "run_artifacts_" + stringID(runID), ID: result.NextCursorID})
	}
	payload := map[string]any{
		"run_id": runID, "context": runArtifactContextRow(runContext), "artifacts": items,
		"has_more": hasMore, "next_cursor": nextCursor, "limit": limit,
		"filters": map[string]any{"q": searchQuery, "role": role, "artifact_id": nullablePositiveID(focusID)},
	}
	if pageMode {
		payload["pagination"] = viewrequest.Pagination(page, perPage, result.Total, "id", order)
	}
	s.respond(w, r, payload, nil)
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
	artifactID, err := viewrequest.PositiveID(r.PathValue("id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	content, err := s.artifact.GetContent(ctx, artifactID)
	if errors.Is(err, artifact.ErrNoBlob) {
		s.respond(w, r, nil, viewrequest.NotFound(err.Error()))
		return
	}
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if content == nil {
		s.respond(w, r, nil, viewrequest.NotFound("artifact not found"))
		return
	}
	storedSize := int64(len(content.Data))
	w.Header().Set("Content-Type", artifactmedia.Inspect(content.ContentType).MediaType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": artifactFilename(artifactID, content.Role, content.ContentType)}))
	w.Header().Set("Content-Length", strconv.FormatInt(storedSize, 10))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content.Data)
}

// artifactInspection returns bounded metadata and preview content for one artifact.
func (s *Server) artifactInspection(w http.ResponseWriter, r *http.Request) {
	if err := viewrequest.KnownQuery(r, "preview_bytes"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	artifactID, err := viewrequest.PositiveID(r.PathValue("id"))
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
			s.respond(w, r, nil, viewrequest.BadRequest("preview_bytes must be between 1 and 262144"))
			return
		}
		previewBytes = parsed
	}
	contentType, byteSize, blobSize, data, err := s.artifactPreviewBlob(ctx, artifactID, previewBytes)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if !artifactmedia.Inspect(contentType).Inline {
		s.respond(w, r, nil, viewrequest.BadRequest("artifact is not available for inline inspection; download it instead"))
		return
	}
	originalLength := len(data)
	data = textlimit.UTF8PrefixBytes(data, len(data))
	if originalLength > 0 && len(data) == 0 {
		s.respond(w, r, nil, viewrequest.BadRequest("artifact preview is not valid UTF-8; download it instead"))
		return
	}
	format := "text"
	if artifactmedia.Inspect(contentType).JSON {
		format = "json"
	}
	s.respond(w, r, map[string]any{
		"artifact_id":         artifactID,
		"content_type":        artifactmedia.Inspect(contentType).MediaType,
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
	preview, err := s.artifact.GetPreview(ctx, artifactID, previewBytes)
	if errors.Is(err, artifact.ErrNoBlob) {
		return "", 0, 0, nil, viewrequest.NotFound(err.Error())
	}
	if err != nil {
		return "", 0, 0, nil, err
	}
	if preview == nil {
		return "", 0, 0, nil, viewrequest.NotFound("artifact not found")
	}
	return preview.ContentType, preview.ByteSize, preview.BlobSize, preview.Data, nil
}

// artifactFilename derives a safe download filename from an artifact role and media type.
func artifactFilename(artifactID int64, role, contentType string) string {
	name := "artifact-" + strconv.FormatInt(artifactID, 10)
	switch role {
	case "workspace_config":
		name = "workspace-config-" + strconv.FormatInt(artifactID, 10)
	case "resolved_manifest":
		name = "resolved-manifest-" + strconv.FormatInt(artifactID, 10)
	case "input_manifest":
		name = "input-manifest-" + strconv.FormatInt(artifactID, 10)
	}
	mediaType := artifactmedia.Inspect(contentType).MediaType
	switch {
	case role == "workspace_config" || mediaType == "application/x-something-config":
		return name + ".something"
	case artifactmedia.Inspect(mediaType).JSON:
		return name + ".json"
	case strings.HasPrefix(mediaType, "text/"):
		return name + ".txt"
	default:
		return name + ".bin"
	}
}

// runCacheUses returns cache-use evidence recorded for the selected run.
func (s *Server) runCacheUses(w http.ResponseWriter, r *http.Request) {
	runID, err := viewrequest.PositiveID(r.PathValue("id"))
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
	page, perPage, sort, order, query, err := viewrequest.Rows(r, fields, "id")
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	result, err := s.cacheStore.ListCacheUsesForRun(ctx, cache.CacheUseFilter{
		RunID: runID, Query: query, Sort: sort, Order: order, Page: page, PerPage: perPage,
	})
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	items := make([]map[string]any, 0, len(result.Items))
	for _, record := range result.Items {
		items = append(items, map[string]any{
			"id":                  record.ID,
			"cache_layer":         record.CacheLayer,
			"outcome":             record.Outcome,
			"used_at":             record.UsedAt,
			"cache_entry_id":      record.CacheEntryID,
			"provider":            record.Provider,
			"namespace":           record.Namespace,
			"request_fingerprint": record.RequestFingerprint,
			"response_status":     record.ResponseStatus,
			"payload_artifact_id": jsonvalue.Optional(record.PayloadArtifactID),
			"fetched_at":          record.FetchedAt,
			"expires_at":          jsonvalue.Optional(record.ExpiresAt),
			"extractor_version":   record.ExtractorVersion,
		})
	}
	page = clampScopedPage(page, perPage, result.Total)
	columns := []string{"id", "cache_layer", "outcome", "used_at", "cache_entry_id", "provider", "namespace", "request_fingerprint", "response_status", "payload_artifact_id", "fetched_at", "expires_at", "extractor_version"}
	s.respond(w, r, map[string]any{
		"run_id": runID, "columns": columns, "rows": items, "cache_uses": items,
		"pagination": viewrequest.Pagination(page, perPage, result.Total, sort, order),
	}, nil)
}
