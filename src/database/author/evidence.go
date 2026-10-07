// evidence.go provides the handwritten uncertain-identity candidate data
// access layer over the generated statements. Required-field and rank
// validation stay in Go around the generated insert.
package author

import (
	"context"
	"database/sql"
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
