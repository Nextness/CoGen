// identity_evidence.go provides the run identity-evidence endpoint
// that exposes ORCID name-search candidates and provider outcomes
// without asserting author identity.
package server

import (
	"context"
	"fmt"
	"net/http"
)

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
	rows, err := s.db.QueryContext(ctx, `SELECT r.id AS resolution_id, r.status, r.provider, r.queried_citation_name,
        r.error_message, r.resolved_at, ao.id AS author_occurrence_id, ao.orcid AS observed_orcid,
        ao.person_id, MIN(wr.title) AS article_title, MIN(w.doi) AS doi,
        COUNT(DISTINCT c.id) AS candidate_count
        `+from+" WHERE "+where+" GROUP BY r.id ORDER BY "+fields[sort]+" "+order+" LIMIT ? OFFSET ?", append(args, perPage, (page-1)*perPage)...)
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
	if err := s.attachIdentityCandidates(ctx, items); err != nil {
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

// attachIdentityCandidates groups stored identity candidates under their resolution records.
func (s *Server) attachIdentityCandidates(ctx context.Context, resolutions []map[string]any) error {
	for _, resolution := range resolutions {
		resolutionID, ok := resolution["resolution_id"].(int64)
		if !ok {
			return fmt.Errorf("identity evidence has an invalid resolution identifier")
		}
		rows, err := s.rows(ctx, `SELECT id, candidate_orcid, provider_display_name, query_url,
            payload_artifact_id, provider_rank, created_at
            FROM author_identity_candidates WHERE identity_resolution_id=?
            ORDER BY provider_rank, id LIMIT 100`, resolutionID)
		if err != nil {
			return err
		}
		resolution["candidates"] = rows
		if count, ok := resolution["candidate_count"].(int64); ok {
			resolution["candidates_truncated"] = count > int64(len(rows))
		}
	}
	return nil
}
