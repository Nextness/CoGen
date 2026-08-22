// identity_evidence.go provides the run identity-evidence endpoint
// that exposes ORCID name-search candidates and provider outcomes
// without asserting author identity.
package server

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"strings"
)

const identityCandidatePreviewLimit = 3

// runIdentityEvidence exposes name-derived ORCID evidence without presenting
// it as an author identity. The endpoint is unavailable for databases created
// before the evidence migration, preserving the viewer's read-only behavior.
func (s *Server) runIdentityEvidence(w http.ResponseWriter, r *http.Request) {
	runID, err := positiveID(r.PathValue("id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if !s.tableHasColumns("author_identity_resolutions", "pipeline_run_id", "author_occurrence_id", "status") || !s.tableHasColumns("author_identity_candidates", "identity_resolution_id", "candidate_orcid") {
		s.respond(w, r, nil, notFound("author identity evidence is unavailable for this database"))
		return
	}
	fields := map[string]string{
		"id": "r.id", "status": "r.status", "citation_name": "r.queried_citation_name",
		"article_title": "article_title", "doi": "doi", "candidate_count": "candidate_count", "resolved_at": "r.resolved_at",
	}
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
	from := `FROM author_identity_resolutions r
        JOIN author_occurrences ao ON ao.id=r.author_occurrence_id
        JOIN authorships a ON a.author_occurrence_id=ao.id
        JOIN work_revisions wr ON wr.id=a.work_revision_id AND wr.pipeline_run_id=r.pipeline_run_id
        JOIN works w ON w.id=wr.work_id
        LEFT JOIN author_identity_candidates c ON c.identity_resolution_id=r.id`
	where, args := scopedWhere("r.pipeline_run_id=?", "r.queried_citation_name, ao.citation_name, wr.title, w.doi", runID, query)
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(DISTINCT r.id) "+from+" WHERE "+where, args...).Scan(&total); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	totalPages := (total + int64(perPage) - 1) / int64(perPage)
	if totalPages == 0 {
		page = 1
	} else if int64(page) > totalPages {
		page = int(totalPages)
	}
	orderSQL := fields[sort] + " " + order
	if fields[sort] != "r.id" {
		orderSQL += ", r.id " + order
	}
	rows, err := s.db.QueryContext(ctx, `SELECT r.id AS resolution_id, r.status, r.provider, r.queried_citation_name,
        r.error_message, r.resolved_at, ao.id AS author_occurrence_id, ao.orcid AS observed_orcid,
        ao.person_id, MIN(wr.title) AS article_title, MIN(w.doi) AS doi,
        COUNT(DISTINCT c.id) AS candidate_count
		`+from+" WHERE "+where+" GROUP BY r.id ORDER BY "+orderSQL+" LIMIT ? OFFSET ?", append(args, perPage, (page-1)*perPage)...)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	items, err := rowsAsMaps(rows)
	rows.Close()
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if err := s.attachIdentityCandidatePreviews(ctx, items); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	stats, err := s.identityEvidenceStats(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	s.respond(w, r, map[string]any{
		"run_id":  runID,
		"columns": []string{"resolution_id", "status", "queried_citation_name", "article_title", "doi", "candidate_count", "resolved_at"},
		"rows":    items, "pagination": scopedPagination(page, perPage, total, sort, order), "stats": stats,
	}, nil)
}

// identityEvidenceStats counts candidate and resolution states for the selected context.
func (s *Server) identityEvidenceStats(ctx context.Context, runID int64) (map[string]int64, error) {
	var resolutions, unclear, noCandidate, providerFailed, candidates int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*),
        COALESCE(SUM(CASE WHEN status='orcid_is_unclear' THEN 1 ELSE 0 END), 0),
        COALESCE(SUM(CASE WHEN status='no_orcid_candidate' THEN 1 ELSE 0 END), 0),
        COALESCE(SUM(CASE WHEN status='provider_failed' THEN 1 ELSE 0 END), 0)
        FROM author_identity_resolutions WHERE pipeline_run_id=?`, runID).
		Scan(&resolutions, &unclear, &noCandidate, &providerFailed); err != nil {
		return nil, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM author_identity_candidates c
        JOIN author_identity_resolutions r ON r.id=c.identity_resolution_id
		WHERE r.pipeline_run_id=?`, runID).Scan(&candidates); err != nil {
		return nil, err
	}
	return map[string]int64{"resolutions": resolutions, "unclear": unclear, "no_candidate": noCandidate, "provider_failed": providerFailed, "candidates": candidates}, nil
}

// attachIdentityCandidatePreviews batches a small ranked preview for every visible resolution.
func (s *Server) attachIdentityCandidatePreviews(ctx context.Context, resolutions []map[string]any) error {
	if len(resolutions) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(resolutions))
	byID := make(map[int64]map[string]any, len(resolutions))
	for _, resolution := range resolutions {
		resolutionID, ok := resolution["resolution_id"].(int64)
		if !ok {
			return &apiProblem{Status: http.StatusInternalServerError, Code: "internal_error", Message: "identity evidence has an invalid resolution identifier"}
		}
		ids = append(ids, resolutionID)
		byID[resolutionID] = resolution
		resolution["candidates"] = []map[string]any{}
	}
	markers := make([]string, len(ids))
	args := make([]any, len(ids)+1)
	for index, id := range ids {
		markers[index] = "?"
		args[index] = id
	}
	args[len(ids)] = identityCandidatePreviewLimit
	rows, err := s.db.QueryContext(ctx, `SELECT identity_resolution_id, id, candidate_orcid,
		provider_display_name, query_url, payload_artifact_id, provider_rank, created_at
		FROM (SELECT candidate.*,
			ROW_NUMBER() OVER (PARTITION BY identity_resolution_id ORDER BY provider_rank, id) AS candidate_row
			FROM author_identity_candidates candidate
			WHERE identity_resolution_id IN (`+strings.Join(markers, ",")+`))
		WHERE candidate_row<=? ORDER BY identity_resolution_id, provider_rank, id`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var resolutionID, id int64
		var candidateORCID, queryURL, createdAt string
		var displayName sql.NullString
		var payloadID, rank sql.NullInt64
		if err := rows.Scan(&resolutionID, &id, &candidateORCID, &displayName, &queryURL, &payloadID, &rank, &createdAt); err != nil {
			return err
		}
		resolution := byID[resolutionID]
		candidates := resolution["candidates"].([]map[string]any)
		resolution["candidates"] = append(candidates, map[string]any{
			"id": id, "candidate_orcid": candidateORCID, "provider_display_name": nullableString(displayName),
			"query_url": queryURL, "payload_artifact_id": nullableInt64(payloadID), "provider_rank": nullableInt64(rank), "created_at": createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, resolution := range resolutions {
		count, _ := resolution["candidate_count"].(int64)
		resolution["candidate_preview_limit"] = identityCandidatePreviewLimit
		resolution["candidates_truncated"] = count > int64(len(resolution["candidates"].([]map[string]any)))
	}
	return nil
}

// identityCandidates returns one cursor-paginated ranked candidate page for a run-owned resolution.
func (s *Server) identityCandidates(w http.ResponseWriter, r *http.Request) {
	if err := validateKnownQuery(r, "run_id", "limit", "cursor"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	runID, err := hierarchyRequiredID(r, "run_id")
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	resolutionID, err := positiveID(r.PathValue("id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	limit, err := reviewLimit(r)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	kind := "identity_candidates_" + stringID(runID) + "_" + stringID(resolutionID)
	cursor, err := decodeReviewCursor(r.URL.Query().Get("cursor"), kind)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	cursorRank := int64(0)
	if r.URL.Query().Get("cursor") != "" && cursor.Text == "" {
		s.respond(w, r, nil, badRequest("cursor is invalid for this collection"))
		return
	}
	if cursor.Text != "" {
		cursorRank, err = strconv.ParseInt(cursor.Text, 10, 64)
		if err != nil || cursorRank < 0 || cursor.ID < 1 {
			s.respond(w, r, nil, badRequest("cursor is invalid for this collection"))
			return
		}
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM author_identity_resolutions
		WHERE id=? AND pipeline_run_id=?`, resolutionID, runID).Scan(&exists); err == sql.ErrNoRows {
		s.respond(w, r, nil, notFound("identity resolution not found"))
		return
	} else if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	query := `SELECT id, candidate_orcid, provider_display_name, query_url,
		payload_artifact_id, provider_rank, created_at
		FROM author_identity_candidates WHERE identity_resolution_id=?`
	args := []any{resolutionID}
	if cursor.Text != "" {
		query += " AND (COALESCE(provider_rank, 0)>? OR (COALESCE(provider_rank, 0)=? AND id>?))"
		args = append(args, cursorRank, cursorRank, cursor.ID)
	}
	query += " ORDER BY COALESCE(provider_rank, 0), id LIMIT ?"
	args = append(args, limit+1)
	items, err := s.rows(ctx, query, args...)
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
		last := items[len(items)-1]
		rank, _ := last["provider_rank"].(int64)
		id, _ := last["id"].(int64)
		nextCursor = encodeReviewCursor(reviewCursor{Kind: kind, ID: id, Text: strconv.FormatInt(rank, 10)})
	}
	s.respond(w, r, map[string]any{
		"resolution_id": resolutionID, "items": items, "has_more": hasMore, "next_cursor": nextCursor, "limit": limit,
	}, nil)
}
