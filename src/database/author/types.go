// types.go defines the author family application types and vocabulary.
// Generated sqlc row types stay inside the private internal/sql package and
// never cross this family boundary.
package author

// Person is an optional strong global identity for an author. ORCID is the
// canonical strong identity signal.
type Person struct {
	ID        int64  `json:"id"`
	ORCID     string `json:"orcid"`
	CreatedAt string `json:"created_at"`
}

// Occurrence is observed author data at a point in time. An occurrence may
// optionally link to a global Person record when the ORCID is a known strong
// identity. ORCID-less occurrences with the same name are never merged
// globally.
type Occurrence struct {
	ID           int64  `json:"id"`
	PersonID     int64  `json:"person_id"`
	CitationName string `json:"citation_name"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	ORCID        string `json:"orcid"`
	CreatedAt    string `json:"created_at"`
}

// Authorship links an immutable work revision to an author occurrence,
// preserving author order and optional affiliation. The authorships table is
// append-only, so a historical revision's authorship set is immutable.
type Authorship struct {
	ID                 int64  `json:"id"`
	WorkRevisionID     int64  `json:"work_revision_id"`
	AuthorOccurrenceID int64  `json:"author_occurrence_id"`
	AuthorOrder        int    `json:"author_order"`
	Affiliation        string `json:"affiliation"`
	CreatedAt          string `json:"created_at"`
}

// IdentityResolution records the result of evaluating one observed author
// occurrence against an identity provider. It is separate from people and
// occurrences because a name search alone is not identity proof.
type IdentityResolution struct {
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

// IdentityCandidate is one provider-returned possible identity. It
// deliberately stores no person_id: a later reviewer may confirm or reject it
// without changing the evidence captured by this run.
type IdentityCandidate struct {
	ID                   int64  `json:"id"`
	IdentityResolutionID int64  `json:"identity_resolution_id"`
	CandidateORCID       string `json:"candidate_orcid"`
	ProviderDisplayName  string `json:"provider_display_name"`
	QueryURL             string `json:"query_url"`
	PayloadArtifactID    int64  `json:"payload_artifact_id"`
	ProviderRank         int    `json:"provider_rank"`
	CreatedAt            string `json:"created_at"`
}

// Identity resolution status vocabulary.
const (
	IdentityStatusORCIDUnclear     = "orcid_is_unclear"
	IdentityStatusNoORCIDCandidate = "no_orcid_candidate"
	IdentityStatusProviderFailed   = "provider_failed"
	IdentityStatusConfirmed        = "confirmed"
	IdentityStatusRejected         = "rejected"
)
