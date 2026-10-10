// identity_evidence.go provides the run identity-evidence endpoint
// that exposes ORCID name-search candidates and provider outcomes
// without asserting author identity.
package server

import (
	"context"
	"net/http"
	"strconv"

	"analysis/database/author"
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
	result, err := s.authorStore.ListIdentityEvidence(ctx, author.IdentityEvidenceFilter{
		RunID: runID, Query: query, Sort: sort, Order: order, Page: page, PerPage: perPage,
	})
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	items := make([]map[string]any, 0, len(result.Items))
	for _, row := range result.Items {
		items = append(items, identityEvidenceRow(row))
	}
	page = clampScopedPage(page, perPage, result.Total)
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
		"rows":    items, "pagination": scopedPagination(page, perPage, result.Total, sort, order), "stats": stats,
	}, nil)
}

// identityEvidenceRow renders one author family evidence row as the viewer's JSON row shape.
func identityEvidenceRow(row *author.IdentityEvidenceRow) map[string]any {
	return map[string]any{
		"resolution_id":         row.ResolutionID,
		"status":                row.Status,
		"provider":              row.Provider,
		"queried_citation_name": row.QueriedCitationName,
		"error_message":         nullableText(row.ErrorMessage),
		"resolved_at":           row.ResolvedAt,
		"author_occurrence_id":  row.AuthorOccurrenceID,
		"observed_orcid":        nullableText(row.ObservedORCID),
		"person_id":             nullableIDPointer(row.PersonID),
		"article_title":         nullableText(row.ArticleTitle),
		"doi":                   nullableText(row.DOI),
		"work_revision_id":      nullableIDPointer(row.WorkRevisionID),
		"evidence_revision_id":  nullableIDPointer(row.EvidenceRevisionID),
		"evidence_stage":        nullableText(row.EvidenceStage),
		"candidate_count":       row.CandidateCount,
	}
}

// identityEvidenceStats counts candidate and resolution states for the selected context.
func (s *Server) identityEvidenceStats(ctx context.Context, runID int64) (map[string]int64, error) {
	stats, err := s.authorStore.IdentityEvidenceStats(ctx, runID)
	if err != nil {
		return nil, err
	}
	return map[string]int64{
		"resolutions":     stats.Resolutions,
		"unclear":         stats.Unclear,
		"no_candidate":    stats.NoCandidate,
		"provider_failed": stats.ProviderFailed,
		"candidates":      stats.Candidates,
	}, nil
}

// attachIdentityCandidatePreviews batches a small ranked preview for every visible resolution.
func (s *Server) attachIdentityCandidatePreviews(ctx context.Context, resolutions []map[string]any) error {
	if len(resolutions) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(resolutions))
	for _, resolution := range resolutions {
		resolutionID, ok := resolution["resolution_id"].(int64)
		if !ok {
			return &apiProblem{Status: http.StatusInternalServerError, Code: "internal_error", Message: "identity evidence has an invalid resolution identifier"}
		}
		ids = append(ids, resolutionID)
		resolution["candidates"] = []map[string]any{}
	}
	previews, err := s.authorStore.ListCandidatePreviews(ctx, ids, identityCandidatePreviewLimit)
	if err != nil {
		return err
	}
	byResolution := make(map[int64][]map[string]any, len(ids))
	for _, preview := range previews {
		byResolution[preview.ResolutionID] = append(byResolution[preview.ResolutionID], identityCandidateRow(preview))
	}
	for _, resolution := range resolutions {
		resolutionID := resolution["resolution_id"].(int64)
		candidates := byResolution[resolutionID]
		if candidates == nil {
			candidates = []map[string]any{}
		}
		resolution["candidates"] = candidates
		count, _ := resolution["candidate_count"].(int64)
		resolution["candidate_preview_limit"] = identityCandidatePreviewLimit
		resolution["candidates_truncated"] = count > int64(len(candidates))
	}
	return nil
}

// identityCandidateRow renders one author family candidate as the viewer's JSON row shape.
func identityCandidateRow(candidate *author.IdentityCandidatePreview) map[string]any {
	return map[string]any{
		"id":                    candidate.ID,
		"candidate_orcid":       candidate.CandidateORCID,
		"provider_display_name": nullableText(candidate.ProviderDisplayName),
		"query_url":             candidate.QueryURL,
		"payload_artifact_id":   nullableIDPointer(candidate.PayloadArtifactID),
		"provider_rank":         nullableIDPointer(candidate.ProviderRank),
		"created_at":            candidate.CreatedAt,
	}
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
	exists, err := s.authorStore.IdentityResolutionExists(ctx, resolutionID, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if !exists {
		s.respond(w, r, nil, notFound("identity resolution not found"))
		return
	}
	candidates, err := s.authorStore.ListIdentityCandidates(ctx, author.IdentityCandidateFilter{
		ResolutionID: resolutionID, CursorRank: cursorRank, CursorID: cursor.ID, Limit: limit,
	})
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	hasMore := len(candidates) > limit
	if hasMore {
		candidates = candidates[:limit]
	}
	var nextCursor any
	if hasMore {
		last := candidates[len(candidates)-1]
		rank := int64(0)
		if last.ProviderRank != nil {
			rank = *last.ProviderRank
		}
		nextCursor = encodeCursor(reviewCursor{Kind: kind, ID: last.ID, Text: strconv.FormatInt(rank, 10)})
	}
	items := make([]map[string]any, 0, len(candidates))
	for _, candidate := range candidates {
		items = append(items, map[string]any{
			"id":                    candidate.ID,
			"candidate_orcid":       candidate.CandidateORCID,
			"provider_display_name": nullableText(candidate.ProviderDisplayName),
			"query_url":             candidate.QueryURL,
			"payload_artifact_id":   nullableIDPointer(candidate.PayloadArtifactID),
			"provider_rank":         nullableIDPointer(candidate.ProviderRank),
			"created_at":            candidate.CreatedAt,
		})
	}
	s.respond(w, r, map[string]any{
		"resolution_id": resolutionID, "items": items, "has_more": hasMore, "next_cursor": nextCursor, "limit": limit,
	}, nil)
}
