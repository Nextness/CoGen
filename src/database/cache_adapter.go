// cache_adapter.go provides temporary forwarding adapters that keep the legacy
// flat cache repository API working over the cache family store. The adapters
// contain no SQL or second implementation; remove them after every caller
// migrates to Database.Cache.
package database

import (
	"context"

	"analysis/database/cache"
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

// CacheEntryRepository forwards the legacy cache-entry API to the cache family store.
type CacheEntryRepository struct{ db *Database }

// RunCacheUseRepository forwards the legacy run-cache-use API to the cache family store.
type RunCacheUseRepository struct{ db *Database }

// Upsert appends an immutable response version through the cache family store.
func (r *CacheEntryRepository) Upsert(entry *CacheEntry) (int64, error) {
	return r.db.Cache.AppendEntry(context.Background(), cacheEntryToFamily(entry))
}

// Get returns the latest response for an exact request and extractor, regardless of expiry. Policy
// execution decides whether an expired entry is stale or may be reused.
func (r *CacheEntryRepository) Get(provider, namespace, fingerprint, extractorVersion string) (*CacheEntry, error) {
	entry, err := r.db.Cache.LatestEntry(context.Background(), cache.Key{
		Provider: provider, Namespace: namespace, RequestFingerprint: fingerprint, ExtractorVersion: extractorVersion,
	})
	if err != nil {
		return nil, err
	}
	return cacheEntryFromFamily(entry), nil
}

// GetGlobal returns an entry only after a run explicitly published it to the
// global layer. Entries written only to active_run remain private to that run.
func (r *CacheEntryRepository) GetGlobal(provider, namespace, fingerprint, extractorVersion string) (*CacheEntry, error) {
	entry, err := r.db.Cache.GlobalEntry(context.Background(), cache.Key{
		Provider: provider, Namespace: namespace, RequestFingerprint: fingerprint, ExtractorVersion: extractorVersion,
	})
	if err != nil {
		return nil, err
	}
	return cacheEntryFromFamily(entry), nil
}

// Create validates and inserts one run-scoped cache-use evidence record through the cache family store.
func (r *RunCacheUseRepository) Create(use *RunCacheUse) (int64, error) {
	return r.db.Cache.AppendUse(context.Background(), cacheUseToFamily(use))
}

// ListByRun returns cache-use evidence for a run in insertion order.
func (r *RunCacheUseRepository) ListByRun(runID int64) ([]*RunCacheUse, error) {
	uses, err := r.db.Cache.ListUsesByRun(context.Background(), runID)
	if err != nil {
		return nil, err
	}
	legacy := make([]*RunCacheUse, 0, len(uses))
	for _, use := range uses {
		legacy = append(legacy, cacheUseFromFamily(use))
	}
	return legacy, nil
}

// FindEntry returns the latest entry with this exact versioned key recorded
// for a run and cache layer. It supports active-run and named-prior-run reads.
func (r *RunCacheUseRepository) FindEntry(runID int64, layer, provider, namespace, fingerprint, extractorVersion string) (*CacheEntry, error) {
	entry, err := r.db.Cache.FindEntry(context.Background(), runID, layer, cache.Key{
		Provider: provider, Namespace: namespace, RequestFingerprint: fingerprint, ExtractorVersion: extractorVersion,
	})
	if err != nil {
		return nil, err
	}
	return cacheEntryFromFamily(entry), nil
}

// FindAnyEntry returns the latest exact cache key used by a prior run. Named
// prior-run policy reads use this because their source layer is provenance, not
// a restriction on how the source run originally obtained the response.
func (r *RunCacheUseRepository) FindAnyEntry(runID int64, provider, namespace, fingerprint, extractorVersion string) (*CacheEntry, error) {
	entry, err := r.db.Cache.FindAnyEntry(context.Background(), runID, cache.Key{
		Provider: provider, Namespace: namespace, RequestFingerprint: fingerprint, ExtractorVersion: extractorVersion,
	})
	if err != nil {
		return nil, err
	}
	return cacheEntryFromFamily(entry), nil
}

// cacheEntryToFamily maps the legacy cache entry into the cache family application type.
func cacheEntryToFamily(entry *CacheEntry) *cache.Entry {
	if entry == nil {
		return nil
	}
	return &cache.Entry{
		ID:                 entry.ID,
		Provider:           entry.Provider,
		Namespace:          entry.Namespace,
		RequestFingerprint: entry.RequestFingerprint,
		ResponseStatus:     entry.ResponseStatus,
		PayloadArtifactID:  entry.PayloadArtifactID,
		FetchedAt:          entry.FetchedAt,
		ExpiresAt:          entry.ExpiresAt,
		ExtractorVersion:   entry.ExtractorVersion,
		CreatedAt:          entry.CreatedAt,
		UpdatedAt:          entry.UpdatedAt,
	}
}

// cacheEntryFromFamily maps a cache family entry into the legacy application type.
func cacheEntryFromFamily(entry *cache.Entry) *CacheEntry {
	if entry == nil {
		return nil
	}
	return &CacheEntry{
		ID:                 entry.ID,
		Provider:           entry.Provider,
		Namespace:          entry.Namespace,
		RequestFingerprint: entry.RequestFingerprint,
		ResponseStatus:     entry.ResponseStatus,
		PayloadArtifactID:  entry.PayloadArtifactID,
		FetchedAt:          entry.FetchedAt,
		ExpiresAt:          entry.ExpiresAt,
		ExtractorVersion:   entry.ExtractorVersion,
		CreatedAt:          entry.CreatedAt,
		UpdatedAt:          entry.UpdatedAt,
	}
}

// cacheUseToFamily maps the legacy run cache use into the cache family application type.
func cacheUseToFamily(use *RunCacheUse) *cache.Use {
	if use == nil {
		return nil
	}
	return &cache.Use{
		ID:            use.ID,
		PipelineRunID: use.PipelineRunID,
		CacheEntryID:  use.CacheEntryID,
		CacheLayer:    use.CacheLayer,
		Outcome:       use.Outcome,
		UsedAt:        use.UsedAt,
	}
}

// cacheUseFromFamily maps a cache family run cache use into the legacy application type.
func cacheUseFromFamily(use *cache.Use) *RunCacheUse {
	if use == nil {
		return nil
	}
	return &RunCacheUse{
		ID:            use.ID,
		PipelineRunID: use.PipelineRunID,
		CacheEntryID:  use.CacheEntryID,
		CacheLayer:    use.CacheLayer,
		Outcome:       use.Outcome,
		UsedAt:        use.UsedAt,
	}
}
