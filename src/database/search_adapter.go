// search_adapter.go provides temporary forwarding adapters that keep the
// legacy flat search, revision, and execution-plan repository API working over
// the search family store. The adapters contain no SQL or second
// implementation; remove them after every caller migrates to Database.Search.
package database

import (
	"context"

	"analysis/database/search"
)

// Search represents a single research question (stable identity).
type Search struct {
	ID        int64  `json:"id"`
	SearchID  string `json:"search_id"`
	CreatedAt string `json:"created_at"`
}

// SearchRevision is a researcher-managed grouping for one version of a
// search's query intent. Its hashes describe the latest observed declaration;
// immutable historical configuration belongs to execution plans and attempts.
type SearchRevision struct {
	ID                   int64  `json:"id"`
	SearchID             int64  `json:"search_id"`
	RevisionLabel        string `json:"revision_label"`
	ConfigArtifactHash   string `json:"config_artifact_hash"`
	ResolvedManifestHash string `json:"resolved_manifest_hash"`
	CreatedAt            string `json:"created_at"`
	UpdatedAt            string `json:"updated_at,omitempty"`
}

// ExecutionPlan represents a unique fingerprint per search revision and input policy.
type ExecutionPlan struct {
	ID                   int64  `json:"id"`
	SearchRevisionID     int64  `json:"search_revision_id"`
	ExecutionFingerprint string `json:"execution_fingerprint"`
	ResolvedManifestHash string `json:"resolved_manifest_hash"`
	InputManifestHash    string `json:"input_manifest_hash"`
	EnrichmentEnabled    bool   `json:"enrichment_enabled"`
	CreatedAt            string `json:"created_at"`
}

// SearchRepository forwards the legacy search API to the search family store.
type SearchRepository struct{ db *Database }

// SearchRevisionRepository forwards the legacy search-revision API to the search family store.
type SearchRevisionRepository struct{ db *Database }

// ExecutionPlanRepository forwards the legacy execution-plan API to the search family store.
type ExecutionPlanRepository struct{ db *Database }

// Create inserts a new search by search_id. Returns the search ID.
// If the search_id already exists, returns the existing ID.
func (r *SearchRepository) Create(searchID string) (int64, error) {
	return r.db.Search.FindOrCreateSearch(context.Background(), searchID)
}

// GetByID returns a search by its primary key, or nil if not found.
func (r *SearchRepository) GetByID(id int64) (*Search, error) {
	found, err := r.db.Search.GetSearchByID(context.Background(), id)
	if err != nil {
		return nil, err
	}
	return searchFromFamily(found), nil
}

// GetBySearchID returns a search by its string identifier, or nil if not found.
func (r *SearchRepository) GetBySearchID(searchID string) (*Search, error) {
	found, err := r.db.Search.GetSearchBySearchID(context.Background(), searchID)
	if err != nil {
		return nil, err
	}
	return searchFromFamily(found), nil
}

// List returns all searches ordered by ID.
func (r *SearchRepository) List() ([]*Search, error) {
	found, err := r.db.Search.ListSearches(context.Background())
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, nil
	}
	legacy := make([]*Search, 0, len(found))
	for _, item := range found {
		legacy = append(legacy, searchFromFamily(item))
	}
	return legacy, nil
}

// Create inserts a new search revision. Returns the revision ID and whether
// its latest-declaration hashes were updated (false on first insert or
// identical hashes).
// If the same (search_id, revision_label) already exists with identical
// config and manifest hashes, returns the existing ID with updated=false.
// If the hashes differ, the existing row is updated with the new hashes and
// updated_at, and updated=true is returned. This allows the same revision
// label to track the latest configuration for a search.
func (r *SearchRevisionRepository) Create(searchID int64, revisionLabel string, configArtifactHash, resolvedManifestHash string) (int64, bool, error) {
	return r.db.Search.UpsertRevision(context.Background(), search.RevisionInput{
		SearchID:             searchID,
		RevisionLabel:        revisionLabel,
		ConfigArtifactHash:   configArtifactHash,
		ResolvedManifestHash: resolvedManifestHash,
	})
}

// GetByID returns a search revision by its primary key, or nil if not found.
func (r *SearchRevisionRepository) GetByID(id int64) (*SearchRevision, error) {
	found, err := r.db.Search.GetRevisionByID(context.Background(), id)
	if err != nil {
		return nil, err
	}
	return revisionFromFamily(found), nil
}

// GetBySearchAndRevision returns a revision for a given search and label, or nil if not found.
func (r *SearchRevisionRepository) GetBySearchAndRevision(searchID int64, revisionLabel string) (*SearchRevision, error) {
	found, err := r.db.Search.GetRevisionBySearchAndLabel(context.Background(), searchID, revisionLabel)
	if err != nil {
		return nil, err
	}
	return revisionFromFamily(found), nil
}

// ListBySearch returns all revisions for a search, ordered by ID.
func (r *SearchRevisionRepository) ListBySearch(searchID int64) ([]*SearchRevision, error) {
	found, err := r.db.Search.ListRevisionsBySearch(context.Background(), searchID)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, nil
	}
	legacy := make([]*SearchRevision, 0, len(found))
	for _, item := range found {
		legacy = append(legacy, revisionFromFamily(item))
	}
	return legacy, nil
}

// Create inserts a new execution plan. Returns the plan ID.
// If an identical plan (same search_revision_id + execution_fingerprint) already
// exists with the same resolved_manifest_hash, returns the existing ID.
// If the manifest hash differs, returns an error: the fingerprint is reserved
// for a different resolved configuration and cannot be reused.
func (r *ExecutionPlanRepository) Create(searchRevisionID int64, fingerprint, manifestHash string) (int64, error) {
	return r.db.Search.FindOrCreatePlan(context.Background(), search.PlanInput{
		SearchRevisionID:     searchRevisionID,
		ExecutionFingerprint: fingerprint,
		ResolvedManifestHash: manifestHash,
	})
}

// CreateWithInputManifest creates a plan linked to the frozen input manifest
// used to calculate its execution fingerprint.
func (r *ExecutionPlanRepository) CreateWithInputManifest(searchRevisionID int64, fingerprint, manifestHash, inputManifestHash string, enrichmentEnabled bool) (int64, error) {
	return r.db.Search.FindOrCreatePlanWithInputManifest(context.Background(), search.PlanInput{
		SearchRevisionID:     searchRevisionID,
		ExecutionFingerprint: fingerprint,
		ResolvedManifestHash: manifestHash,
		InputManifestHash:    inputManifestHash,
		EnrichmentEnabled:    enrichmentEnabled,
	})
}

// GetByID returns an execution plan by its primary key, or nil if not found.
func (r *ExecutionPlanRepository) GetByID(id int64) (*ExecutionPlan, error) {
	found, err := r.db.Search.GetPlanByID(context.Background(), id)
	if err != nil {
		return nil, err
	}
	return planFromFamily(found), nil
}

// GetByFingerprint returns an execution plan matching the given search revision and fingerprint, or nil if not found.
func (r *ExecutionPlanRepository) GetByFingerprint(searchRevisionID int64, fingerprint string) (*ExecutionPlan, error) {
	found, err := r.db.Search.GetPlanByFingerprint(context.Background(), searchRevisionID, fingerprint)
	if err != nil {
		return nil, err
	}
	return planFromFamily(found), nil
}

// ListBySearchRevision returns all execution plans for a given search revision, ordered by ID.
func (r *ExecutionPlanRepository) ListBySearchRevision(searchRevisionID int64) ([]*ExecutionPlan, error) {
	found, err := r.db.Search.ListPlansBySearchRevision(context.Background(), searchRevisionID)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, nil
	}
	legacy := make([]*ExecutionPlan, 0, len(found))
	for _, item := range found {
		legacy = append(legacy, planFromFamily(item))
	}
	return legacy, nil
}

// searchFromFamily maps a search family search into the legacy application type.
func searchFromFamily(found *search.Search) *Search {
	if found == nil {
		return nil
	}
	return &Search{ID: found.ID, SearchID: found.SearchID, CreatedAt: found.CreatedAt}
}

// revisionFromFamily maps a search family revision into the legacy application type.
func revisionFromFamily(found *search.Revision) *SearchRevision {
	if found == nil {
		return nil
	}
	return &SearchRevision{
		ID:                   found.ID,
		SearchID:             found.SearchID,
		RevisionLabel:        found.RevisionLabel,
		ConfigArtifactHash:   found.ConfigArtifactHash,
		ResolvedManifestHash: found.ResolvedManifestHash,
		CreatedAt:            found.CreatedAt,
		UpdatedAt:            found.UpdatedAt,
	}
}

// planFromFamily maps a search family plan into the legacy application type.
func planFromFamily(found *search.Plan) *ExecutionPlan {
	if found == nil {
		return nil
	}
	return &ExecutionPlan{
		ID:                   found.ID,
		SearchRevisionID:     found.SearchRevisionID,
		ExecutionFingerprint: found.ExecutionFingerprint,
		ResolvedManifestHash: found.ResolvedManifestHash,
		InputManifestHash:    found.InputManifestHash,
		EnrichmentEnabled:    found.EnrichmentEnabled,
		CreatedAt:            found.CreatedAt,
	}
}
