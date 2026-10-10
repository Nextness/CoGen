// author_adapter.go provides temporary forwarding adapters that keep the
// legacy flat person, author-occurrence, authorship, identity-resolution, and
// identity-candidate repository API working over the author family store. The
// adapters contain no SQL or second implementation; remove them after every
// caller migrates to Database.Author.
package database

import (
	"context"

	"analysis/database/author"
)

// Author identity resolution status vocabulary.
const (
	AuthorIdentityStatusORCIDUnclear     = author.IdentityStatusORCIDUnclear
	AuthorIdentityStatusNoORCIDCandidate = author.IdentityStatusNoORCIDCandidate
	AuthorIdentityStatusProviderFailed   = author.IdentityStatusProviderFailed
	AuthorIdentityStatusConfirmed        = author.IdentityStatusConfirmed
	AuthorIdentityStatusRejected         = author.IdentityStatusRejected
)

// Person represents an optional strong global identity for an author.
// ORCID is the canonical strong identity signal.
type Person struct {
	ID        int64  `json:"id"`
	ORCID     string `json:"orcid"`
	CreatedAt string `json:"created_at"`
}

// AuthorOccurrence represents observed author data at a point in time.
// An occurrence may optionally link to a global Person record when the ORCID
// is a known strong identity. ORCID-less occurrences with the same name are
// never merged globally.
type AuthorOccurrence struct {
	ID           int64  `json:"id"`
	PersonID     int64  `json:"person_id"`
	CitationName string `json:"citation_name"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	ORCID        string `json:"orcid"`
	CreatedAt    string `json:"created_at"`
}

// Authorship links an immutable work_revision to an author_occurrence,
// preserving author order and optional affiliation. The authorships table
// is append-only (database-level triggers enforce this), so a historical
// revision's authorship set is immutable. Corrections create a new
// work_revision with a new authorship set.
type Authorship struct {
	ID                 int64  `json:"id"`
	WorkRevisionID     int64  `json:"work_revision_id"`
	AuthorOccurrenceID int64  `json:"author_occurrence_id"`
	AuthorOrder        int    `json:"author_order"`
	Affiliation        string `json:"affiliation"`
	CreatedAt          string `json:"created_at"`
}

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

// PersonRepository forwards the legacy people API to the author family store.
type PersonRepository struct{ db *Database }

// AuthorOccurrenceRepository forwards the legacy author-occurrence API to the author family store.
type AuthorOccurrenceRepository struct{ db *Database }

// AuthorshipRepository forwards the legacy authorship API to the author family store.
type AuthorshipRepository struct{ db *Database }

// AuthorIdentityResolutionRepository forwards the legacy identity-resolution API to the author family store.
type AuthorIdentityResolutionRepository struct{ db *Database }

// AuthorIdentityCandidateRepository forwards the legacy identity-candidate API to the author family store.
type AuthorIdentityCandidateRepository struct{ db *Database }

// CreateByORCID inserts a new person by ORCID. If the ORCID already exists,
// returns the existing person ID (INSERT OR IGNORE semantics).
// The ORCID is normalized (lowercased, whitespace trimmed) before storage.
// A malformed ORCID or one that fails the ISO 7064 MOD 11-2 checksum is
// rejected: the people table is a strong identity registry, not a raw
// observation store.
func (r *PersonRepository) CreateByORCID(orcid string) (int64, error) {
	return r.db.Author.CreatePersonByORCID(context.Background(), orcid)
}

// GetByID returns a person by their primary key, or nil if not found.
func (r *PersonRepository) GetByID(id int64) (*Person, error) {
	found, err := r.db.Author.GetPersonByID(context.Background(), id)
	if err != nil {
		return nil, err
	}
	return personFromFamily(found), nil
}

// GetByORCID returns a person by their ORCID, or nil if not found.
// The ORCID is normalized the same way as CreateByORCID.
func (r *PersonRepository) GetByORCID(orcid string) (*Person, error) {
	found, err := r.db.Author.GetPersonByORCID(context.Background(), orcid)
	if err != nil {
		return nil, err
	}
	return personFromFamily(found), nil
}

// Create inserts a new author occurrence. If the ORCID is non-empty and
// passes format-and-checksum validation, the method looks up or creates a
// Person record and links the occurrence to it. Invalid or malformed ORCIDs
// are stored as raw observed values on the occurrence but do not create or
// link to a person record.
func (r *AuthorOccurrenceRepository) Create(ao *AuthorOccurrence) (int64, error) {
	if ao == nil {
		return r.db.Author.CreateOccurrence(context.Background(), nil)
	}
	family := occurrenceToFamily(ao)
	id, err := r.db.Author.CreateOccurrence(context.Background(), family)
	if err != nil {
		return 0, err
	}
	ao.PersonID = family.PersonID
	return id, nil
}

// GetByID returns an author occurrence by its primary key, or nil if not found.
func (r *AuthorOccurrenceRepository) GetByID(id int64) (*AuthorOccurrence, error) {
	found, err := r.db.Author.GetOccurrenceByID(context.Background(), id)
	if err != nil {
		return nil, err
	}
	return occurrenceFromFamily(found), nil
}

// GetByPersonID returns all author occurrences linked to a given person, in
// ID order.
func (r *AuthorOccurrenceRepository) GetByPersonID(personID int64) ([]*AuthorOccurrence, error) {
	found, err := r.db.Author.ListOccurrencesByPersonID(context.Background(), personID)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, nil
	}
	legacy := make([]*AuthorOccurrence, 0, len(found))
	for _, item := range found {
		legacy = append(legacy, occurrenceFromFamily(item))
	}
	return legacy, nil
}

// Create inserts a new authorship linking a work revision to an author
// occurrence with the given order and optional affiliation.
func (r *AuthorshipRepository) Create(a *Authorship) (int64, error) {
	if a == nil {
		return r.db.Author.CreateAuthorship(context.Background(), nil)
	}
	return r.db.Author.CreateAuthorship(context.Background(), authorshipToFamily(a))
}

// GetByRevisionID returns all authorships for a given work revision, ordered
// by author_order.
func (r *AuthorshipRepository) GetByRevisionID(revisionID int64) ([]*Authorship, error) {
	found, err := r.db.Author.ListAuthorshipsByRevisionID(context.Background(), revisionID)
	if err != nil {
		return nil, err
	}
	return authorshipsFromFamily(found), nil
}

// GetByOccurrenceID returns all authorships for a given author occurrence,
// ordered by ID.
func (r *AuthorshipRepository) GetByOccurrenceID(occurrenceID int64) ([]*Authorship, error) {
	found, err := r.db.Author.ListAuthorshipsByOccurrenceID(context.Background(), occurrenceID)
	if err != nil {
		return nil, err
	}
	return authorshipsFromFamily(found), nil
}

// Create validates and inserts one author identity resolution record.
func (r *AuthorIdentityResolutionRepository) Create(resolution *AuthorIdentityResolution) (int64, error) {
	if resolution == nil {
		return r.db.Author.CreateIdentityResolution(context.Background(), nil)
	}
	return r.db.Author.CreateIdentityResolution(context.Background(), identityResolutionToFamily(resolution))
}

// Create validates and inserts one uncertain author identity candidate.
func (r *AuthorIdentityCandidateRepository) Create(candidate *AuthorIdentityCandidate) (int64, error) {
	if candidate == nil {
		return r.db.Author.CreateIdentityCandidate(context.Background(), nil)
	}
	return r.db.Author.CreateIdentityCandidate(context.Background(), identityCandidateToFamily(candidate))
}

// personFromFamily maps an author family person into the legacy application type.
func personFromFamily(found *author.Person) *Person {
	if found == nil {
		return nil
	}
	return &Person{ID: found.ID, ORCID: found.ORCID, CreatedAt: found.CreatedAt}
}

// occurrenceToFamily maps a legacy occurrence into the author family application type.
func occurrenceToFamily(ao *AuthorOccurrence) *author.Occurrence {
	return &author.Occurrence{
		ID:           ao.ID,
		PersonID:     ao.PersonID,
		CitationName: ao.CitationName,
		FirstName:    ao.FirstName,
		LastName:     ao.LastName,
		ORCID:        ao.ORCID,
		CreatedAt:    ao.CreatedAt,
	}
}

// occurrenceFromFamily maps an author family occurrence into the legacy application type.
func occurrenceFromFamily(found *author.Occurrence) *AuthorOccurrence {
	if found == nil {
		return nil
	}
	return &AuthorOccurrence{
		ID:           found.ID,
		PersonID:     found.PersonID,
		CitationName: found.CitationName,
		FirstName:    found.FirstName,
		LastName:     found.LastName,
		ORCID:        found.ORCID,
		CreatedAt:    found.CreatedAt,
	}
}

// authorshipToFamily maps a legacy authorship into the author family application type.
func authorshipToFamily(a *Authorship) *author.Authorship {
	return &author.Authorship{
		ID:                 a.ID,
		WorkRevisionID:     a.WorkRevisionID,
		AuthorOccurrenceID: a.AuthorOccurrenceID,
		AuthorOrder:        a.AuthorOrder,
		Affiliation:        a.Affiliation,
		CreatedAt:          a.CreatedAt,
	}
}

// authorshipFromFamily maps an author family authorship into the legacy application type.
func authorshipFromFamily(found *author.Authorship) *Authorship {
	if found == nil {
		return nil
	}
	return &Authorship{
		ID:                 found.ID,
		WorkRevisionID:     found.WorkRevisionID,
		AuthorOccurrenceID: found.AuthorOccurrenceID,
		AuthorOrder:        found.AuthorOrder,
		Affiliation:        found.Affiliation,
		CreatedAt:          found.CreatedAt,
	}
}

// authorshipsFromFamily maps author family authorships into legacy application types.
// A nil input preserves the legacy nil slice identity.
func authorshipsFromFamily(found []*author.Authorship) []*Authorship {
	if found == nil {
		return nil
	}
	legacy := make([]*Authorship, 0, len(found))
	for _, item := range found {
		legacy = append(legacy, authorshipFromFamily(item))
	}
	return legacy
}

// identityResolutionToFamily maps a legacy identity resolution into the author family application type.
func identityResolutionToFamily(resolution *AuthorIdentityResolution) *author.IdentityResolution {
	return &author.IdentityResolution{
		ID:                  resolution.ID,
		PipelineRunID:       resolution.PipelineRunID,
		AuthorOccurrenceID:  resolution.AuthorOccurrenceID,
		Status:              resolution.Status,
		Provider:            resolution.Provider,
		QueriedCitationName: resolution.QueriedCitationName,
		ErrorMessage:        resolution.ErrorMessage,
		ResolvedAt:          resolution.ResolvedAt,
		CreatedAt:           resolution.CreatedAt,
	}
}

// identityCandidateToFamily maps a legacy identity candidate into the author family application type.
func identityCandidateToFamily(candidate *AuthorIdentityCandidate) *author.IdentityCandidate {
	return &author.IdentityCandidate{
		ID:                   candidate.ID,
		IdentityResolutionID: candidate.IdentityResolutionID,
		CandidateORCID:       candidate.CandidateORCID,
		ProviderDisplayName:  candidate.ProviderDisplayName,
		QueryURL:             candidate.QueryURL,
		PayloadArtifactID:    candidate.PayloadArtifactID,
		ProviderRank:         candidate.ProviderRank,
		CreatedAt:            candidate.CreatedAt,
	}
}
