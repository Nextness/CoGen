// cache.go implements the policy-driven provider cache layer
// (active-run, global, network, named-run reads; active-run and global
// writes) used by the workspace pipeline.
package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"analysis/database"
	"analysis/enrich"
	"analysis/manifest"
)

const cacheExtractorVersion = "workspace-cache-v1"

// cacheRequest identifies one provider request independently of its storage layer.
type cacheRequest struct {
	Provider  string
	Namespace string
	Identity  string
	URL       string
}

// cacheResponse records resolved payload bytes, status, layer, outcome, and artifact identity.
type cacheResponse struct {
	Body              []byte
	Status            int
	Layer             string
	Outcome           manifest.CacheOutcome
	PayloadArtifactID int64
}

// workspaceCache applies one resolved cache policy without making enrichment
// packages depend on SQLite. The same policy covers provider work, author, and
// name-search requests.
type workspaceCache struct {
	db     *database.Database
	runID  int64
	policy manifest.CachePolicy
}

// resolve follows the declared cache read order, validates reusable payloads, and records outcome evidence.
func (c *workspaceCache) resolve(ctx context.Context, request cacheRequest, fetch func(context.Context) *enrich.FetchResult, negative func([]byte) bool) (*cacheResponse, error) {
	if strings.TrimSpace(request.Provider) == "" || strings.TrimSpace(request.Namespace) == "" || strings.TrimSpace(request.Identity) == "" || strings.TrimSpace(request.URL) == "" {
		return nil, fmt.Errorf("cache request provider, namespace, identity, and URL are required")
	}
	fingerprint := cacheFingerprint(request)
	for _, layer := range c.policy.Reads {
		var (
			entry *database.CacheEntry
			err   error
		)
		switch {
		case layer == "active_run":
			entry, err = c.db.RunCacheUses.FindEntry(c.runID, "active_run", request.Provider, request.Namespace, fingerprint, c.extractorVersion("active_run"))
		case strings.HasPrefix(layer, "run:"):
			priorRunID, parseErr := strconv.ParseInt(strings.TrimPrefix(layer, "run:"), 10, 64)
			if parseErr != nil || priorRunID <= 0 {
				return nil, fmt.Errorf("invalid named cache layer %q", layer)
			}
			entry, err = c.db.RunCacheUses.FindAnyEntry(priorRunID, request.Provider, request.Namespace, fingerprint, cacheExtractorVersion)
		case layer == "global":
			entry, err = c.db.CacheEntries.GetGlobal(request.Provider, request.Namespace, fingerprint, cacheExtractorVersion)
		case layer == "network":
			return c.fetchAndRecord(ctx, request, fingerprint, layer, fetch, negative)
		default:
			return nil, fmt.Errorf("unsupported cache read layer %q", layer)
		}
		if err != nil {
			return nil, err
		}
		if entry == nil {
			if err := c.incrementMetric("cache_misses", request.Provider); err != nil {
				return nil, err
			}
			continue
		}
		if cacheEntryExpired(entry, time.Now().UTC()) {
			if err := c.recordUse(entry.ID, layer, manifest.CacheStale); err != nil {
				return nil, err
			}
			if err := c.incrementMetric("cache_stale", request.Provider); err != nil {
				return nil, err
			}
			continue
		}
		if entry.ResponseStatus == 404 {
			if err := c.recordUse(entry.ID, layer, manifest.CacheNegative); err != nil {
				return nil, err
			}
			if err := c.incrementMetric("cache_negative", request.Provider); err != nil {
				return nil, err
			}
			return &cacheResponse{Status: entry.ResponseStatus, Layer: layer, Outcome: manifest.CacheNegative}, nil
		}
		if entry.PayloadArtifactID == nil {
			return nil, fmt.Errorf("cache entry %d has no payload artifact", entry.ID)
		}
		body, err := c.readPayload(*entry.PayloadArtifactID)
		if err != nil {
			return nil, err
		}
		if err := enrich.ValidateProviderPayload(request.Provider, request.Namespace, body); err != nil {
			log.Warn("cached provider payload is not reusable", "cache_entry_id", entry.ID, "provider", request.Provider, "namespace", request.Namespace, "error", err)
			if metricErr := c.incrementMetric("cache_invalid_payloads", request.Provider); metricErr != nil {
				return nil, metricErr
			}
			continue
		}
		if err := c.recordUse(entry.ID, layer, manifest.CacheHit); err != nil {
			return nil, err
		}
		if err := c.incrementMetric("cache_hits", request.Provider); err != nil {
			return nil, err
		}
		if err := c.recordAudit(manifest.AuditCacheHit, request, layer, manifest.CacheHit, entry.ID); err != nil {
			return nil, err
		}
		return &cacheResponse{Body: body, Status: entry.ResponseStatus, Layer: layer, Outcome: manifest.CacheHit, PayloadArtifactID: *entry.PayloadArtifactID}, nil
	}
	return nil, fmt.Errorf("cache policy has no network layer and no reusable response for %s/%s", request.Provider, request.Identity)
}

// fetchAndRecord validates a network result, persists cacheable evidence, and records cache metrics and audit.
func (c *workspaceCache) fetchAndRecord(ctx context.Context, request cacheRequest, fingerprint, layer string, fetch func(context.Context) *enrich.FetchResult, negative func([]byte) bool) (*cacheResponse, error) {
	if err := c.incrementMetric("cache_network_fetches", request.Provider); err != nil {
		return nil, err
	}
	response := fetch(ctx)
	if response == nil {
		return nil, fmt.Errorf("network fetch %s/%s returned no result", request.Provider, request.Identity)
	}
	if err := c.recordAudit(manifest.AuditNetworkFetch, request, layer, manifest.CacheMiss, 0); err != nil {
		return nil, err
	}
	if response.Err != nil {
		return nil, fmt.Errorf("network fetch %s/%s: %w", request.Provider, request.Identity, response.Err)
	}
	status := response.StatusCode
	if status != 200 && status != 404 {
		return nil, fmt.Errorf("network fetch %s/%s returned status %d", request.Provider, request.Identity, status)
	}
	if status == 200 {
		if err := enrich.ValidateProviderPayload(request.Provider, request.Namespace, response.Body); err != nil {
			if metricErr := c.incrementMetric("cache_invalid_payloads", request.Provider); metricErr != nil {
				return nil, metricErr
			}
			return nil, fmt.Errorf("network fetch %s/%s returned invalid provider payload: %w", request.Provider, request.Identity, err)
		}
		if negative != nil && negative(response.Body) {
			status = 404
		}
	}
	if status == 404 && !cacheableNegative(request) {
		return &cacheResponse{Body: response.Body, Status: status, Layer: "network", Outcome: manifest.CacheMiss}, nil
	}
	entry := &database.CacheEntry{
		Provider: request.Provider, Namespace: request.Namespace, RequestFingerprint: fingerprint,
		ResponseStatus: status, FetchedAt: time.Now().UTC().Format(time.RFC3339Nano), ExtractorVersion: cacheExtractorVersion,
	}
	if status == 404 {
		entry.ExpiresAt = time.Now().UTC().Add(time.Duration(c.policy.NegativeTTLDays) * 24 * time.Hour).Format(time.RFC3339Nano)
	} else {
		artifactID, err := persistArtifact(c.db, c.runID, response.Body, "application/json")
		if err != nil {
			return nil, fmt.Errorf("persist cache payload: %w", err)
		}
		entry.PayloadArtifactID = &artifactID
	}
	outcome := manifest.CacheHit
	if status == 404 {
		outcome = manifest.CacheNegative
	}
	for _, writeLayer := range c.policy.Writes {
		copy := *entry
		copy.ExtractorVersion = c.extractorVersion(writeLayer)
		entryID, err := c.db.CacheEntries.Upsert(&copy)
		if err != nil {
			return nil, err
		}
		if err := c.recordUse(entryID, writeLayer, outcome); err != nil {
			return nil, err
		}
	}
	return &cacheResponse{Body: response.Body, Status: status, Layer: "network", Outcome: outcome, PayloadArtifactID: dereferenceInt64(entry.PayloadArtifactID)}, nil
}

// dereferenceInt64 returns a pointed integer or zero for nil.
func dereferenceInt64(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

// cacheableNegative limits negative TTLs to provider responses whose 404 or
// validated empty-result semantics mean "not found" rather than a malformed
// endpoint, authorization issue, or transient provider failure.
func cacheableNegative(request cacheRequest) bool {
	switch request.Provider {
	case "crossref":
		return request.Namespace == "work_by_doi"
	case "openalex":
		return request.Namespace == "work_by_doi" || request.Namespace == "author_by_orcid"
	case "orcid":
		return request.Namespace == "author_by_orcid" || request.Namespace == "author_name_search"
	default:
		return false
	}
}

// extractorVersion returns the layer-specific extractor key used to isolate active-run entries.
func (c *workspaceCache) extractorVersion(layer string) string {
	if layer == "active_run" {
		return cacheExtractorVersion + ":run:" + strconv.FormatInt(c.runID, 10)
	}
	return cacheExtractorVersion
}

// incrementMetric increments a cache metric at both run-wide and provider scope.
func (c *workspaceCache) incrementMetric(metric, provider string) error {
	for _, source := range []string{"", provider} {
		current, err := c.db.Metrics.Get(c.runID, metric, source)
		if err != nil {
			return err
		}
		value := 1
		if current != nil {
			value += current.Value
		}
		if err := c.db.Metrics.Set(c.runID, metric, source, value); err != nil {
			return err
		}
	}
	return nil
}

// recordUse persists one run-to-cache-entry lookup outcome.
func (c *workspaceCache) recordUse(entryID int64, layer string, outcome manifest.CacheOutcome) error {
	_, err := c.db.RunCacheUses.Create(&database.RunCacheUse{PipelineRunID: c.runID, CacheEntryID: entryID, CacheLayer: layer, Outcome: string(outcome)})
	return err
}

// readPayload reads payload from the supplied source.
func (c *workspaceCache) readPayload(artifactID int64) ([]byte, error) {
	blob, err := c.db.ArtifactBlobs.GetByArtifactID(artifactID)
	if err != nil {
		return nil, err
	}
	if blob == nil {
		return nil, fmt.Errorf("cache payload artifact %d has no blob data", artifactID)
	}
	return blob.Data, nil
}

// recordAudit appends cache decision evidence for one provider request.
func (c *workspaceCache) recordAudit(action manifest.AuditAction, request cacheRequest, layer string, outcome manifest.CacheOutcome, entryID int64) error {
	metadata, err := json.Marshal(map[string]any{
		"provider": request.Provider, "namespace": request.Namespace, "identity": request.Identity,
		"cache_layer": layer, "cache_outcome": outcome, "cache_entry_id": entryID,
	})
	if err != nil {
		return err
	}
	_, err = c.db.AuditEvents.Insert(&manifest.AuditEvent{
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), Actor: "pipeline", PipelineRunID: c.runID,
		EntityType: "cache_request", EntityID: cacheFingerprint(request), Action: action,
		MetadataJSON: string(metadata), CorrelationID: "cache-" + strconv.FormatInt(c.runID, 10),
	})
	return err
}

// cacheFingerprint hashes every request-affecting field deterministically.
func cacheFingerprint(request cacheRequest) string {
	// Struct serialization is deterministic and includes every request-affecting
	// input. The extractor version remains part of the database uniqueness key.
	data, _ := json.Marshal(request)
	return contentHash(data)
}

// cacheEntryExpired reports whether a cache entry is past its parsed expiry, treating malformed expiry as stale.
func cacheEntryExpired(entry *database.CacheEntry, now time.Time) bool {
	if entry == nil || entry.ExpiresAt == "" {
		return false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if expiresAt, err := time.Parse(layout, entry.ExpiresAt); err == nil {
			return !expiresAt.After(now)
		}
	}
	// An unparseable expiry cannot safely be treated as reusable.
	return true
}
