// identity.go provides the handwritten uncertain-identity resolution data
// access layer over the generated statements. Required-field and status
// vocabulary validation stay in Go around the generated insert.
package author

import (
	"context"
	"fmt"
	"strings"

	generated "analysis/database/author/internal/sql"
)

// CreateIdentityResolution validates and inserts one author identity
// resolution record.
func (s *Store) CreateIdentityResolution(ctx context.Context, resolution *IdentityResolution) (int64, error) {
	if resolution == nil {
		return 0, fmt.Errorf("create author identity resolution: resolution is required")
	}
	if resolution.PipelineRunID == 0 || resolution.AuthorOccurrenceID == 0 {
		return 0, fmt.Errorf("create author identity resolution: pipeline_run_id and author_occurrence_id are required")
	}
	if !validIdentityStatus(resolution.Status) {
		return 0, fmt.Errorf("create author identity resolution: unsupported status %q", resolution.Status)
	}
	if strings.TrimSpace(resolution.Provider) == "" || strings.TrimSpace(resolution.QueriedCitationName) == "" || strings.TrimSpace(resolution.ResolvedAt) == "" {
		return 0, fmt.Errorf("create author identity resolution: provider, queried_citation_name, and resolved_at are required")
	}
	result, err := s.queries.InsertAuthorIdentityResolution(ctx, generated.InsertAuthorIdentityResolutionParams{
		PipelineRunID:       resolution.PipelineRunID,
		AuthorOccurrenceID:  resolution.AuthorOccurrenceID,
		Status:              resolution.Status,
		Provider:            resolution.Provider,
		QueriedCitationName: resolution.QueriedCitationName,
		ErrorMessage:        nullableString(resolution.ErrorMessage),
		ResolvedAt:          resolution.ResolvedAt,
	})
	if err != nil {
		return 0, fmt.Errorf("create author identity resolution: %w", err)
	}
	return result.LastInsertId()
}

// validIdentityStatus reports whether the supplied author identity status is
// supported.
func validIdentityStatus(status string) bool {
	switch status {
	case IdentityStatusORCIDUnclear, IdentityStatusNoORCIDCandidate,
		IdentityStatusProviderFailed, IdentityStatusConfirmed, IdentityStatusRejected:
		return true
	default:
		return false
	}
}
