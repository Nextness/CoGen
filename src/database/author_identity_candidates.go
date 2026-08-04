// author_identity_candidates.go provides the repository for writing
// author identity evidence records — ORCID name-search candidates,
// provider outcomes, and confirmed or rejected identity links.
package database

import (
	"fmt"
	"strings"
)

const (
	AuthorIdentityStatusORCIDUnclear     = "orcid_is_unclear"
	AuthorIdentityStatusNoORCIDCandidate = "no_orcid_candidate"
	AuthorIdentityStatusProviderFailed   = "provider_failed"
	AuthorIdentityStatusConfirmed        = "confirmed"
	AuthorIdentityStatusRejected         = "rejected"
)

// AuthorIdentityResolution records the result of evaluating one observed
// author occurrence against an identity provider. It is separate from people
// and author_occurrences because a name search alone is not identity proof.
type AuthorIdentityResolution struct {
	ID                  int64  `json:"id"`
	PipelineRunID       int64  `json:"pipeline_run_id"`
	AuthorOccurrenceID  int64  `json:"author_occurrence_id"`
	Status              string `json:"status"`
	Provider            string `json:"provider"`
	QueriedCitationName string `json:"queried_citation_name"`
	ErrorMessage        string `json:"error_message"`
	ResolvedAt          string `json:"resolved_at"`
	CreatedAt           string `json:"created_at"`
}

// AuthorIdentityCandidate is one provider-returned possible identity. It
// deliberately stores no person_id: a later reviewer may confirm or reject it
// without changing the evidence captured by this run.
type AuthorIdentityCandidate struct {
	ID                   int64  `json:"id"`
	IdentityResolutionID int64  `json:"identity_resolution_id"`
	CandidateORCID       string `json:"candidate_orcid"`
	ProviderDisplayName  string `json:"provider_display_name"`
	QueryURL             string `json:"query_url"`
	PayloadArtifactID    int64  `json:"payload_artifact_id"`
	ProviderRank         int    `json:"provider_rank"`
	CreatedAt            string `json:"created_at"`
}

// AuthorIdentityResolutionRepository owns uncertain identity evidence.
type AuthorIdentityResolutionRepository struct {
	db *Database
}

// Create validates and inserts one author identity resolution record.
func (r *AuthorIdentityResolutionRepository) Create(resolution *AuthorIdentityResolution) (int64, error) {
	if resolution == nil {
		return 0, fmt.Errorf("create author identity resolution: resolution is required")
	}
	if resolution.PipelineRunID == 0 || resolution.AuthorOccurrenceID == 0 {
		return 0, fmt.Errorf("create author identity resolution: pipeline_run_id and author_occurrence_id are required")
	}
	if !validAuthorIdentityStatus(resolution.Status) {
		return 0, fmt.Errorf("create author identity resolution: unsupported status %q", resolution.Status)
	}
	if strings.TrimSpace(resolution.Provider) == "" || strings.TrimSpace(resolution.QueriedCitationName) == "" || strings.TrimSpace(resolution.ResolvedAt) == "" {
		return 0, fmt.Errorf("create author identity resolution: provider, queried_citation_name, and resolved_at are required")
	}
	res, err := r.db.DB.Exec(`
		INSERT INTO author_identity_resolutions
			(pipeline_run_id, author_occurrence_id, status, provider, queried_citation_name, error_message, resolved_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		resolution.PipelineRunID, resolution.AuthorOccurrenceID, resolution.Status,
		resolution.Provider, resolution.QueriedCitationName, nullStr(resolution.ErrorMessage), resolution.ResolvedAt)
	if err != nil {
		return 0, fmt.Errorf("create author identity resolution: %w", err)
	}
	return res.LastInsertId()
}

// validAuthorIdentityStatus reports whether the supplied author identity status is supported.
func validAuthorIdentityStatus(status string) bool {
	switch status {
	case AuthorIdentityStatusORCIDUnclear, AuthorIdentityStatusNoORCIDCandidate,
		AuthorIdentityStatusProviderFailed, AuthorIdentityStatusConfirmed, AuthorIdentityStatusRejected:
		return true
	default:
		return false
	}
}

// AuthorIdentityCandidateRepository owns the immutable candidates attached to
// one uncertain identity resolution.
type AuthorIdentityCandidateRepository struct {
	db *Database
}

// Create validates and inserts one uncertain author identity candidate.
func (r *AuthorIdentityCandidateRepository) Create(candidate *AuthorIdentityCandidate) (int64, error) {
	if candidate == nil {
		return 0, fmt.Errorf("create author identity candidate: candidate is required")
	}
	if candidate.IdentityResolutionID == 0 || strings.TrimSpace(candidate.CandidateORCID) == "" || strings.TrimSpace(candidate.QueryURL) == "" || candidate.ProviderRank < 1 {
		return 0, fmt.Errorf("create author identity candidate: resolution, candidate_orcid, query_url, and positive provider_rank are required")
	}
	var payloadArtifactID any
	if candidate.PayloadArtifactID != 0 {
		payloadArtifactID = candidate.PayloadArtifactID
	}
	res, err := r.db.DB.Exec(`
		INSERT INTO author_identity_candidates
			(identity_resolution_id, candidate_orcid, provider_display_name, query_url, payload_artifact_id, provider_rank)
		VALUES (?, ?, ?, ?, ?, ?)`,
		candidate.IdentityResolutionID, strings.TrimSpace(candidate.CandidateORCID), nullStr(candidate.ProviderDisplayName), candidate.QueryURL, payloadArtifactID, candidate.ProviderRank)
	if err != nil {
		return 0, fmt.Errorf("create author identity candidate: %w", err)
	}
	return res.LastInsertId()
}
