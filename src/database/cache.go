// cache.go provides the repository for provider cache entries, run-cache
// usage tracking, and the cache-entry-to-payload-artifact link that
// backs the workspace cache layer.
package database

import (
	"database/sql"
	"fmt"
	"strings"

	"analysis/manifest"
)

// CacheEntry is a versioned raw provider response. A nil payload artifact is
// valid for negative responses such as an HTTP 404.
type CacheEntry struct {
	ID                 int64  `json:"id"`
	Provider           string `json:"provider"`
	Namespace          string `json:"namespace"`
	RequestFingerprint string `json:"request_fingerprint"`
	ResponseStatus     int    `json:"response_status"`
	PayloadArtifactID  *int64 `json:"payload_artifact_id,omitempty"`
	FetchedAt          string `json:"fetched_at"`
	ExpiresAt          string `json:"expires_at,omitempty"`
	ExtractorVersion   string `json:"extractor_version"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
}

// RunCacheUse records that a run consulted or consumed a global cache entry.
type RunCacheUse struct {
	ID            int64  `json:"id"`
	PipelineRunID int64  `json:"pipeline_run_id"`
	CacheEntryID  int64  `json:"cache_entry_id"`
	CacheLayer    string `json:"cache_layer"`
	Outcome       string `json:"outcome"`
	UsedAt        string `json:"used_at"`
}

// CacheEntryRepository provides persistence operations for cache entry records.
type CacheEntryRepository struct{ db *Database }

// RunCacheUseRepository provides persistence operations for run cache use records.
type RunCacheUseRepository struct{ db *Database }

// validateCacheEntry enforces provider, namespace, fingerprint, status, and payload invariants before persistence.
func validateCacheEntry(entry *CacheEntry) error {
	if entry == nil {
		return fmt.Errorf("cache entry is required")
	}
	for name, value := range map[string]string{
		"provider": entry.Provider, "namespace": entry.Namespace,
		"request fingerprint": entry.RequestFingerprint, "extractor version": entry.ExtractorVersion,
		"fetched at": entry.FetchedAt,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("cache entry %s is required", name)
		}
	}
	if entry.ResponseStatus < 100 || entry.ResponseStatus > 599 {
		return fmt.Errorf("cache entry response status %d is invalid", entry.ResponseStatus)
	}
	return nil
}

// Upsert atomically replaces the response for one provider request and
// extractor version. The stable row ID preserves references from run_cache_uses.
func (r *CacheEntryRepository) Upsert(entry *CacheEntry) (int64, error) {
	if err := validateCacheEntry(entry); err != nil {
		return 0, err
	}
	var payload any
	if entry.PayloadArtifactID != nil {
		payload = *entry.PayloadArtifactID
	}
	_, err := r.db.DB.Exec(`INSERT INTO cache_entries
        (provider, namespace, request_fingerprint, response_status, payload_artifact_id, fetched_at, expires_at, extractor_version, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
        ON CONFLICT(provider, namespace, request_fingerprint, extractor_version) DO UPDATE SET
          response_status=excluded.response_status,
          payload_artifact_id=excluded.payload_artifact_id,
          fetched_at=excluded.fetched_at,
          expires_at=excluded.expires_at,
          updated_at=datetime('now')`,
		entry.Provider, entry.Namespace, entry.RequestFingerprint, entry.ResponseStatus,
		payload, entry.FetchedAt, nullStr(entry.ExpiresAt), entry.ExtractorVersion)
	if err != nil {
		return 0, fmt.Errorf("upsert cache entry: %w", err)
	}
	var id int64
	err = r.db.DB.QueryRow(`SELECT id FROM cache_entries
        WHERE provider=? AND namespace=? AND request_fingerprint=? AND extractor_version=?`,
		entry.Provider, entry.Namespace, entry.RequestFingerprint, entry.ExtractorVersion).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("find upserted cache entry: %w", err)
	}
	return id, nil
}

// Get returns the exact versioned cache entry, regardless of expiry. Policy
// execution decides whether an expired entry is stale or may be reused.
func (r *CacheEntryRepository) Get(provider, namespace, fingerprint, extractorVersion string) (*CacheEntry, error) {
	return r.get(`SELECT id, provider, namespace, request_fingerprint,
        response_status, payload_artifact_id, fetched_at, expires_at,
        extractor_version, created_at, updated_at FROM cache_entries
        WHERE provider=? AND namespace=? AND request_fingerprint=? AND extractor_version=?`,
		provider, namespace, fingerprint, extractorVersion)
}

// GetGlobal returns an entry only after a run explicitly published it to the
// global layer. Entries written only to active_run remain private to that run.
func (r *CacheEntryRepository) GetGlobal(provider, namespace, fingerprint, extractorVersion string) (*CacheEntry, error) {
	return r.get(`SELECT ce.id, ce.provider, ce.namespace, ce.request_fingerprint,
        ce.response_status, ce.payload_artifact_id, ce.fetched_at, ce.expires_at,
        ce.extractor_version, ce.created_at, ce.updated_at
        FROM cache_entries ce JOIN run_cache_uses rcu ON rcu.cache_entry_id=ce.id
        WHERE rcu.cache_layer='global' AND ce.provider=? AND ce.namespace=?
          AND ce.request_fingerprint=? AND ce.extractor_version=?
        ORDER BY rcu.id DESC LIMIT 1`, provider, namespace, fingerprint, extractorVersion)
}

// get executes a cache-entry query and returns its nullable payload and expiry fields.
func (r *CacheEntryRepository) get(query string, args ...any) (*CacheEntry, error) {
	entry := &CacheEntry{}
	var payload sql.NullInt64
	var expires sql.NullString
	err := r.db.DB.QueryRow(query, args...).Scan(
		&entry.ID, &entry.Provider, &entry.Namespace, &entry.RequestFingerprint,
		&entry.ResponseStatus, &payload, &entry.FetchedAt, &expires,
		&entry.ExtractorVersion, &entry.CreatedAt, &entry.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get cache entry: %w", err)
	}
	if payload.Valid {
		entry.PayloadArtifactID = &payload.Int64
	}
	if expires.Valid {
		entry.ExpiresAt = expires.String
	}
	return entry, nil
}

// Create validates and inserts one run-scoped cache-use evidence record.
func (r *RunCacheUseRepository) Create(use *RunCacheUse) (int64, error) {
	if use == nil || use.PipelineRunID == 0 || use.CacheEntryID == 0 || strings.TrimSpace(use.CacheLayer) == "" {
		return 0, fmt.Errorf("pipeline run, cache entry, and cache layer are required")
	}
	if err := manifest.ValidateCacheOutcome(use.Outcome); err != nil {
		return 0, err
	}
	result, err := r.db.DB.Exec(`INSERT INTO run_cache_uses (pipeline_run_id, cache_entry_id, cache_layer, outcome)
        VALUES (?, ?, ?, ?)`, use.PipelineRunID, use.CacheEntryID, use.CacheLayer, use.Outcome)
	if err != nil {
		return 0, fmt.Errorf("create run cache use: %w", err)
	}
	return result.LastInsertId()
}

// ListByRun returns cache-use evidence for a run in insertion order.
func (r *RunCacheUseRepository) ListByRun(runID int64) ([]*RunCacheUse, error) {
	rows, err := r.db.DB.Query(`SELECT id, pipeline_run_id, cache_entry_id, cache_layer, outcome, used_at
        FROM run_cache_uses WHERE pipeline_run_id=? ORDER BY id`, runID)
	if err != nil {
		return nil, fmt.Errorf("list run cache uses: %w", err)
	}
	defer rows.Close()
	uses := make([]*RunCacheUse, 0)
	for rows.Next() {
		use := &RunCacheUse{}
		if err := rows.Scan(&use.ID, &use.PipelineRunID, &use.CacheEntryID, &use.CacheLayer, &use.Outcome, &use.UsedAt); err != nil {
			return nil, fmt.Errorf("scan run cache use: %w", err)
		}
		uses = append(uses, use)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate run cache uses: %w", err)
	}
	return uses, nil
}

// FindEntry returns the latest entry with this exact versioned key recorded
// for a run and cache layer. It supports active-run and named-prior-run reads.
func (r *RunCacheUseRepository) FindEntry(runID int64, layer, provider, namespace, fingerprint, extractorVersion string) (*CacheEntry, error) {
	return r.findEntry(`AND rcu.cache_layer=?`, []any{runID, layer, provider, namespace, fingerprint, extractorVersion})
}

// FindAnyEntry returns the latest exact cache key used by a prior run. Named
// prior-run policy reads use this because their source layer is provenance, not
// a restriction on how the source run originally obtained the response.
func (r *RunCacheUseRepository) FindAnyEntry(runID int64, provider, namespace, fingerprint, extractorVersion string) (*CacheEntry, error) {
	entry := &CacheEntry{}
	var payload sql.NullInt64
	var expires sql.NullString
	err := r.db.DB.QueryRow(`SELECT ce.id, ce.provider, ce.namespace, ce.request_fingerprint,
        ce.response_status, ce.payload_artifact_id, ce.fetched_at, ce.expires_at,
        ce.extractor_version, ce.created_at, ce.updated_at
        FROM run_cache_uses rcu JOIN cache_entries ce ON ce.id=rcu.cache_entry_id
        WHERE rcu.pipeline_run_id=? AND ce.provider=? AND ce.namespace=?
          AND ce.request_fingerprint=? AND (ce.extractor_version=? OR ce.extractor_version LIKE ?)
        ORDER BY rcu.id DESC LIMIT 1`, runID, provider, namespace, fingerprint, extractorVersion, extractorVersion+":run:%").Scan(
		&entry.ID, &entry.Provider, &entry.Namespace, &entry.RequestFingerprint,
		&entry.ResponseStatus, &payload, &entry.FetchedAt, &expires,
		&entry.ExtractorVersion, &entry.CreatedAt, &entry.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find prior-run cache entry: %w", err)
	}
	if payload.Valid {
		entry.PayloadArtifactID = &payload.Int64
	}
	if expires.Valid {
		entry.ExpiresAt = expires.String
	}
	return entry, nil
}

// findEntry returns the latest cache entry matching a run-layer predicate.
func (r *RunCacheUseRepository) findEntry(layerClause string, args []any) (*CacheEntry, error) {
	entry := &CacheEntry{}
	var payload sql.NullInt64
	var expires sql.NullString
	query := `SELECT ce.id, ce.provider, ce.namespace, ce.request_fingerprint,
        ce.response_status, ce.payload_artifact_id, ce.fetched_at, ce.expires_at,
        ce.extractor_version, ce.created_at, ce.updated_at
        FROM run_cache_uses rcu JOIN cache_entries ce ON ce.id=rcu.cache_entry_id
        WHERE rcu.pipeline_run_id=? ` + layerClause + ` AND ce.provider=? AND ce.namespace=?
          AND ce.request_fingerprint=? AND ce.extractor_version=?
	        ORDER BY rcu.id DESC LIMIT 1`
	err := r.db.DB.QueryRow(query, args...).Scan(
		&entry.ID, &entry.Provider, &entry.Namespace, &entry.RequestFingerprint,
		&entry.ResponseStatus, &payload, &entry.FetchedAt, &expires,
		&entry.ExtractorVersion, &entry.CreatedAt, &entry.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find run cache entry: %w", err)
	}
	if payload.Valid {
		entry.PayloadArtifactID = &payload.Int64
	}
	if expires.Valid {
		entry.ExpiresAt = expires.String
	}
	return entry, nil
}
