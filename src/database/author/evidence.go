// evidence.go provides the handwritten uncertain-identity candidate data
// access layer over the generated statements. Required-field and rank
// validation stay in Go around the generated insert.
package author

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	generated "analysis/database/author/internal/sql"
)

// CreateIdentityCandidate validates and inserts one uncertain author identity
// candidate. Candidates remain ranked provider evidence and never promote
// themselves to a confirmed person.
func (s *Store) CreateIdentityCandidate(ctx context.Context, candidate *IdentityCandidate) (int64, error) {
	if candidate == nil {
		return 0, fmt.Errorf("create author identity candidate: candidate is required")
	}
	if candidate.IdentityResolutionID == 0 || strings.TrimSpace(candidate.CandidateORCID) == "" || strings.TrimSpace(candidate.QueryURL) == "" || candidate.ProviderRank < 1 {
		return 0, fmt.Errorf("create author identity candidate: resolution, candidate_orcid, query_url, and positive provider_rank are required")
	}
	var payloadArtifactID sql.NullInt64
	if candidate.PayloadArtifactID != 0 {
		payloadArtifactID = sql.NullInt64{Int64: candidate.PayloadArtifactID, Valid: true}
	}
	result, err := s.queries.InsertAuthorIdentityCandidate(ctx, generated.InsertAuthorIdentityCandidateParams{
		IdentityResolutionID: candidate.IdentityResolutionID,
		CandidateOrcid:       strings.TrimSpace(candidate.CandidateORCID),
		ProviderDisplayName:  nullableString(candidate.ProviderDisplayName),
		QueryUrl:             candidate.QueryURL,
		PayloadArtifactID:    payloadArtifactID,
		ProviderRank:         int64(candidate.ProviderRank),
	})
	if err != nil {
		return 0, fmt.Errorf("create author identity candidate: %w", err)
	}
	return result.LastInsertId()
}

// IdentityEvidenceStats returns run-scoped resolution outcome and candidate counts.
func (s *Store) IdentityEvidenceStats(ctx context.Context, runID int64) (*IdentityEvidenceStats, error) {
	stats, err := s.queries.GetIdentityEvidenceStats(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("get identity evidence stats: %w", err)
	}
	candidates, err := s.queries.CountIdentityCandidatesByRun(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("count identity candidates: %w", err)
	}
	return &IdentityEvidenceStats{
		Resolutions:    stats.Resolutions,
		Unclear:        stats.Unclear,
		NoCandidate:    stats.NoCandidate,
		ProviderFailed: stats.ProviderFailed,
		Candidates:     candidates,
	}, nil
}

// IdentityResolutionExists reports whether one resolution belongs to the run.
func (s *Store) IdentityResolutionExists(ctx context.Context, resolutionID, runID int64) (bool, error) {
	if _, err := s.queries.IdentityResolutionExists(ctx, generated.IdentityResolutionExistsParams{
		ID:            resolutionID,
		PipelineRunID: runID,
	}); errors.Is(err, sql.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("check identity resolution: %w", err)
	}
	return true, nil
}
