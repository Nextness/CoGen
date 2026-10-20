// types.go defines the cache family application types. Generated sqlc row
// types stay inside the private internal/sql package and never cross this
// family boundary.
package cache

// Key identifies one exact provider response version.
type Key struct {
	Provider           string
	Namespace          string
	RequestFingerprint string
	ExtractorVersion   string
}

// Entry is a versioned raw provider response. A nil payload artifact is valid
// for negative responses such as an HTTP 404.
type Entry struct {
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

// Use records that a run consulted or consumed a global cache entry.
type Use struct {
	ID            int64  `json:"id"`
	PipelineRunID int64  `json:"pipeline_run_id"`
	CacheEntryID  int64  `json:"cache_entry_id"`
	CacheLayer    string `json:"cache_layer"`
	Outcome       string `json:"outcome"`
	UsedAt        string `json:"used_at"`
}
