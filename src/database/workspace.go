// workspace.go provides the repositories for searches, search revisions,
// source records, and the workspace-level query that binds a pipeline
// run to its declared search iteration.
package database

import (
	"database/sql"
	"fmt"
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

// SearchRepository provides CRUD for the searches table.
type SearchRepository struct {
	db *Database
}

// Create inserts a new search by search_id. Returns the search ID.
// If the search_id already exists, returns the existing ID.
func (r *SearchRepository) Create(searchID string) (int64, error) {
	res, err := r.db.DB.Exec(
		"INSERT OR IGNORE INTO searches (search_id) VALUES (?)",
		searchID,
	)
	if err != nil {
		lg.Debug("search creation failed", "search_id", searchID, "error", err)
		return 0, fmt.Errorf("create search: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		lg.Debug("search creation result read failed", "search_id", searchID, "error", err)
		return 0, err
	}
	if rowsAffected > 0 {
		id, err := res.LastInsertId()
		if err != nil {
			lg.Debug("search inserted ID read failed", "search_id", searchID, "error", err)
			return 0, err
		}
		lg.Debug("search creation successful", "search_id", searchID, "id", id, "result", "inserted")
		return id, nil
	}

	// Already exists - look up existing ID
	var id int64
	err = r.db.DB.QueryRow("SELECT id FROM searches WHERE search_id = ?", searchID).Scan(&id)
	if err != nil {
		lg.Debug("search existing ID lookup failed", "search_id", searchID, "error", err)
		return 0, err
	}
	lg.Debug("search creation successful", "search_id", searchID, "id", id, "result", "already_existing")
	return id, nil
}

// GetByID returns a search by its primary key, or nil if not found.
func (r *SearchRepository) GetByID(id int64) (*Search, error) {
	var s Search
	err := r.db.DB.QueryRow(
		"SELECT id, search_id, created_at FROM searches WHERE id = ?", id,
	).Scan(&s.ID, &s.SearchID, &s.CreatedAt)
	if err == sql.ErrNoRows {
		lg.Debug("search query successful", "id", id, "result", "not_found")
		return nil, nil
	}
	if err != nil {
		lg.Debug("search query failed", "id", id, "error", err)
		return nil, err
	}
	lg.Debug("search query successful", "id", id, "search_id", s.SearchID, "result", "found")
	return &s, nil
}

// GetBySearchID returns a search by its string identifier, or nil if not found.
func (r *SearchRepository) GetBySearchID(searchID string) (*Search, error) {
	var s Search
	err := r.db.DB.QueryRow(
		"SELECT id, search_id, created_at FROM searches WHERE search_id = ?", searchID,
	).Scan(&s.ID, &s.SearchID, &s.CreatedAt)
	if err == sql.ErrNoRows {
		lg.Debug("search query successful", "search_id", searchID, "result", "not_found")
		return nil, nil
	}
	if err != nil {
		lg.Debug("search query failed", "search_id", searchID, "error", err)
		return nil, err
	}
	lg.Debug("search query successful", "search_id", searchID, "id", s.ID, "result", "found")
	return &s, nil
}

// List returns all searches ordered by ID.
func (r *SearchRepository) List() ([]*Search, error) {
	rows, err := r.db.DB.Query("SELECT id, search_id, created_at FROM searches ORDER BY id")
	if err != nil {
		lg.Debug("search list query failed", "error", err)
		return nil, err
	}
	defer rows.Close()

	var result []*Search
	for rows.Next() {
		var s Search
		if err := rows.Scan(&s.ID, &s.SearchID, &s.CreatedAt); err != nil {
			lg.Debug("search list row scan failed", "scanned", len(result), "error", err)
			return nil, err
		}
		result = append(result, &s)
	}
	if err := rows.Err(); err != nil {
		lg.Debug("search list iteration failed", "scanned", len(result), "error", err)
		return nil, err
	}
	lg.Debug("search list query successful", "searches", len(result))
	return result, nil
}

// SearchRevisionRepository provides CRUD for the search_revisions table.
type SearchRevisionRepository struct {
	db *Database
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
	res, err := r.db.DB.Exec(
		`INSERT OR IGNORE INTO search_revisions
			(search_id, revision_label, config_artifact_hash, resolved_manifest_hash)
		 VALUES (?, ?, ?, ?)`,
		searchID, revisionLabel, configArtifactHash, resolvedManifestHash,
	)
	if err != nil {
		lg.Debug("search revision creation failed",
			"search_id", searchID, "revision", revisionLabel, "error", err)
		return 0, false, fmt.Errorf("create search revision: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		lg.Debug("search revision creation result read failed",
			"search_id", searchID, "revision", revisionLabel, "error", err)
		return 0, false, err
	}
	if rowsAffected > 0 {
		id, err := res.LastInsertId()
		if err != nil {
			lg.Debug("search revision inserted ID read failed",
				"search_id", searchID, "revision", revisionLabel, "error", err)
			return 0, false, err
		}
		lg.Debug("search revision creation successful",
			"search_id", searchID, "revision", revisionLabel, "id", id, "result", "inserted")
		return id, false, nil
	}

	// Already exists - check if hashes match.
	existing, err := r.GetBySearchAndRevision(searchID, revisionLabel)
	if err != nil {
		lg.Debug("search revision existing lookup failed",
			"search_id", searchID, "revision", revisionLabel, "error", err)
		return 0, false, err
	}
	if existing == nil {
		lg.Debug("search revision creation failed",
			"search_id", searchID, "revision", revisionLabel, "reason", "insert_skipped_but_not_found")
		return 0, false, fmt.Errorf("create search revision: insert skipped but existing row not found")
	}
	if existing.ConfigArtifactHash == configArtifactHash && existing.ResolvedManifestHash == resolvedManifestHash {
		lg.Debug("search revision creation successful",
			"search_id", searchID, "revision", revisionLabel, "id", existing.ID, "result", "already_existing")
		return existing.ID, false, nil
	}

	// Hashes differ - update the existing row with new hashes.
	_, err = r.db.DB.Exec(
		`UPDATE search_revisions
		 SET config_artifact_hash = ?, resolved_manifest_hash = ?,
		     updated_at = datetime('now')
		 WHERE id = ?`,
		configArtifactHash, resolvedManifestHash, existing.ID,
	)
	if err != nil {
		lg.Debug("search revision update failed",
			"search_id", searchID, "revision", revisionLabel, "id", existing.ID, "error", err)
		return 0, false, fmt.Errorf("update search revision hashes: %w", err)
	}
	lg.Debug("search revision hashes updated",
		"search_id", searchID, "revision", revisionLabel, "id", existing.ID,
		"old_config_hash", existing.ConfigArtifactHash,
		"new_config_hash", configArtifactHash,
		"old_manifest_hash", existing.ResolvedManifestHash,
		"new_manifest_hash", resolvedManifestHash,
		"result", "updated")
	return existing.ID, true, nil
}

// GetByID returns a search revision by its primary key, or nil if not found.
func (r *SearchRevisionRepository) GetByID(id int64) (*SearchRevision, error) {
	var sr SearchRevision
	var updatedAt sql.NullString
	err := r.db.DB.QueryRow(
		`SELECT id, search_id, revision_label, config_artifact_hash,
		        resolved_manifest_hash, created_at, updated_at
		 FROM search_revisions WHERE id = ?`, id,
	).Scan(&sr.ID, &sr.SearchID, &sr.RevisionLabel, &sr.ConfigArtifactHash, &sr.ResolvedManifestHash, &sr.CreatedAt, &updatedAt)
	if err == sql.ErrNoRows {
		lg.Debug("search revision query successful", "id", id, "result", "not_found")
		return nil, nil
	}
	if err != nil {
		lg.Debug("search revision query failed", "id", id, "error", err)
		return nil, err
	}
	sr.UpdatedAt = updatedAt.String
	lg.Debug("search revision query successful", "id", id, "revision", sr.RevisionLabel, "result", "found")
	return &sr, nil
}

// GetBySearchAndRevision returns a revision for a given search and label, or nil if not found.
func (r *SearchRevisionRepository) GetBySearchAndRevision(searchID int64, revisionLabel string) (*SearchRevision, error) {
	var sr SearchRevision
	var updatedAt sql.NullString
	err := r.db.DB.QueryRow(
		`SELECT id, search_id, revision_label, config_artifact_hash,
		        resolved_manifest_hash, created_at, updated_at
		 FROM search_revisions WHERE search_id = ? AND revision_label = ?`,
		searchID, revisionLabel,
	).Scan(&sr.ID, &sr.SearchID, &sr.RevisionLabel, &sr.ConfigArtifactHash, &sr.ResolvedManifestHash, &sr.CreatedAt, &updatedAt)
	if err == sql.ErrNoRows {
		lg.Debug("search revision query successful", "search_id", searchID, "revision", revisionLabel, "result", "not_found")
		return nil, nil
	}
	if err != nil {
		lg.Debug("search revision query failed", "search_id", searchID, "revision", revisionLabel, "error", err)
		return nil, err
	}
	sr.UpdatedAt = updatedAt.String
	lg.Debug("search revision query successful", "search_id", searchID, "revision", revisionLabel, "result", "found")
	return &sr, nil
}

// ListBySearch returns all revisions for a search, ordered by ID.
func (r *SearchRevisionRepository) ListBySearch(searchID int64) ([]*SearchRevision, error) {
	rows, err := r.db.DB.Query(
		`SELECT id, search_id, revision_label, config_artifact_hash,
		        resolved_manifest_hash, created_at, updated_at
		 FROM search_revisions WHERE search_id = ? ORDER BY id`,
		searchID,
	)
	if err != nil {
		lg.Debug("search revision list query failed", "search_id", searchID, "error", err)
		return nil, err
	}
	defer rows.Close()

	var result []*SearchRevision
	for rows.Next() {
		var sr SearchRevision
		var updatedAt sql.NullString
		if err := rows.Scan(&sr.ID, &sr.SearchID, &sr.RevisionLabel, &sr.ConfigArtifactHash, &sr.ResolvedManifestHash, &sr.CreatedAt, &updatedAt); err != nil {
			lg.Debug("search revision list row scan failed", "search_id", searchID, "scanned", len(result), "error", err)
			return nil, err
		}
		sr.UpdatedAt = updatedAt.String
		result = append(result, &sr)
	}
	if err := rows.Err(); err != nil {
		lg.Debug("search revision list iteration failed", "search_id", searchID, "scanned", len(result), "error", err)
		return nil, err
	}
	lg.Debug("search revision list query successful", "search_id", searchID, "revisions", len(result))
	return result, nil
}

// ExecutionPlanRepository provides CRUD for the execution_plans table.
type ExecutionPlanRepository struct {
	db *Database
}

// Create inserts a new execution plan. Returns the plan ID.
// If an identical plan (same search_revision_id + execution_fingerprint) already
// exists with the same resolved_manifest_hash, returns the existing ID.
// If the manifest hash differs, returns an error: the fingerprint is reserved
// for a different resolved configuration and cannot be reused.
func (r *ExecutionPlanRepository) Create(searchRevisionID int64, fingerprint, manifestHash string) (int64, error) {
	return r.createWithPolicy(searchRevisionID, fingerprint, manifestHash, "", false)
}

// CreateWithInputManifest creates a plan linked to the frozen input manifest
// used to calculate its execution fingerprint.
func (r *ExecutionPlanRepository) CreateWithInputManifest(searchRevisionID int64, fingerprint, manifestHash, inputManifestHash string, enrichmentEnabled bool) (int64, error) {
	if inputManifestHash == "" {
		return 0, fmt.Errorf("create execution plan: input manifest hash is required")
	}
	return r.createWithPolicy(searchRevisionID, fingerprint, manifestHash, inputManifestHash, enrichmentEnabled)
}

// createWithPolicy inserts or reuses an execution plan with the supplied manifest and enrichment policy hashes.
func (r *ExecutionPlanRepository) createWithPolicy(searchRevisionID int64, fingerprint, manifestHash, inputManifestHash string, enrichmentEnabled bool) (int64, error) {
	res, err := r.db.DB.Exec(
		`INSERT OR IGNORE INTO execution_plans
			(search_revision_id, execution_fingerprint, resolved_manifest_hash, input_manifest_hash, enrichment_enabled)
		 VALUES (?, ?, ?, ?, ?)`,
		searchRevisionID, fingerprint, manifestHash, inputManifestHash, enrichmentEnabled,
	)
	if err != nil {
		lg.Debug("execution plan creation failed",
			"search_revision_id", searchRevisionID, "fingerprint", fingerprint, "error", err)
		return 0, fmt.Errorf("create execution plan: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		lg.Debug("execution plan creation result read failed",
			"search_revision_id", searchRevisionID, "fingerprint", fingerprint, "error", err)
		return 0, err
	}
	if rowsAffected > 0 {
		id, err := res.LastInsertId()
		if err != nil {
			lg.Debug("execution plan inserted ID read failed",
				"search_revision_id", searchRevisionID, "fingerprint", fingerprint, "error", err)
			return 0, err
		}
		lg.Debug("execution plan creation successful",
			"search_revision_id", searchRevisionID, "fingerprint", fingerprint, "id", id, "result", "inserted")
		return id, nil
	}

	// Already exists - verify manifest hash matches
	existing, err := r.GetByFingerprint(searchRevisionID, fingerprint)
	if err != nil {
		lg.Debug("execution plan existing lookup failed",
			"search_revision_id", searchRevisionID, "fingerprint", fingerprint, "error", err)
		return 0, err
	}
	if existing == nil {
		lg.Debug("execution plan creation failed",
			"search_revision_id", searchRevisionID, "fingerprint", fingerprint, "reason", "insert_skipped_but_not_found")
		return 0, fmt.Errorf("create execution plan: insert skipped but existing row not found")
	}
	if existing.ResolvedManifestHash != manifestHash || (inputManifestHash != "" && existing.InputManifestHash != inputManifestHash) || (inputManifestHash != "" && existing.EnrichmentEnabled != enrichmentEnabled) {
		lg.Debug("execution plan creation rejected",
			"search_revision_id", searchRevisionID, "fingerprint", fingerprint,
			"existing_manifest_hash", existing.ResolvedManifestHash,
			"incoming_manifest_hash", manifestHash,
			"reason", "manifest_hash_mismatch")
		return 0, fmt.Errorf("execution fingerprint %q already exists for search revision %d with different manifest or input", fingerprint, searchRevisionID)
	}
	lg.Debug("execution plan creation successful",
		"search_revision_id", searchRevisionID, "fingerprint", fingerprint, "id", existing.ID, "result", "already_existing")
	return existing.ID, nil
}

// GetByID returns an execution plan by its primary key, or nil if not found.
func (r *ExecutionPlanRepository) GetByID(id int64) (*ExecutionPlan, error) {
	var ep ExecutionPlan
	err := r.db.DB.QueryRow(
		`SELECT id, search_revision_id, execution_fingerprint,
		        resolved_manifest_hash, input_manifest_hash, enrichment_enabled, created_at
		 FROM execution_plans WHERE id = ?`, id,
	).Scan(&ep.ID, &ep.SearchRevisionID, &ep.ExecutionFingerprint, &ep.ResolvedManifestHash, &ep.InputManifestHash, &ep.EnrichmentEnabled, &ep.CreatedAt)
	if err == sql.ErrNoRows {
		lg.Debug("execution plan query successful", "id", id, "result", "not_found")
		return nil, nil
	}
	if err != nil {
		lg.Debug("execution plan query failed", "id", id, "error", err)
		return nil, err
	}
	lg.Debug("execution plan query successful", "id", id, "fingerprint", ep.ExecutionFingerprint, "result", "found")
	return &ep, nil
}

// GetByFingerprint returns an execution plan matching the given search revision and fingerprint, or nil if not found.
func (r *ExecutionPlanRepository) GetByFingerprint(searchRevisionID int64, fingerprint string) (*ExecutionPlan, error) {
	var ep ExecutionPlan
	err := r.db.DB.QueryRow(
		`SELECT id, search_revision_id, execution_fingerprint,
		        resolved_manifest_hash, input_manifest_hash, enrichment_enabled, created_at
		 FROM execution_plans WHERE search_revision_id = ? AND execution_fingerprint = ?`,
		searchRevisionID, fingerprint,
	).Scan(&ep.ID, &ep.SearchRevisionID, &ep.ExecutionFingerprint, &ep.ResolvedManifestHash, &ep.InputManifestHash, &ep.EnrichmentEnabled, &ep.CreatedAt)
	if err == sql.ErrNoRows {
		lg.Debug("execution plan query successful", "search_revision_id", searchRevisionID, "fingerprint", fingerprint, "result", "not_found")
		return nil, nil
	}
	if err != nil {
		lg.Debug("execution plan query failed", "search_revision_id", searchRevisionID, "fingerprint", fingerprint, "error", err)
		return nil, err
	}
	lg.Debug("execution plan query successful", "search_revision_id", searchRevisionID, "fingerprint", fingerprint, "result", "found")
	return &ep, nil
}

// ListBySearchRevision returns all execution plans for a given search revision, ordered by ID.
func (r *ExecutionPlanRepository) ListBySearchRevision(searchRevisionID int64) ([]*ExecutionPlan, error) {
	rows, err := r.db.DB.Query(
		`SELECT id, search_revision_id, execution_fingerprint,
		        resolved_manifest_hash, input_manifest_hash, enrichment_enabled, created_at
		 FROM execution_plans WHERE search_revision_id = ? ORDER BY id`,
		searchRevisionID,
	)
	if err != nil {
		lg.Debug("execution plan list query failed", "search_revision_id", searchRevisionID, "error", err)
		return nil, err
	}
	defer rows.Close()

	var result []*ExecutionPlan
	for rows.Next() {
		var ep ExecutionPlan
		if err := rows.Scan(&ep.ID, &ep.SearchRevisionID, &ep.ExecutionFingerprint, &ep.ResolvedManifestHash, &ep.InputManifestHash, &ep.EnrichmentEnabled, &ep.CreatedAt); err != nil {
			lg.Debug("execution plan list row scan failed", "search_revision_id", searchRevisionID, "scanned", len(result), "error", err)
			return nil, err
		}
		result = append(result, &ep)
	}
	if err := rows.Err(); err != nil {
		lg.Debug("execution plan list iteration failed", "search_revision_id", searchRevisionID, "scanned", len(result), "error", err)
		return nil, err
	}
	lg.Debug("execution plan list query successful", "search_revision_id", searchRevisionID, "plans", len(result))
	return result, nil
}
