// viewer.go provides the cache family's run-scoped cache-use evidence page.
// It owns the joined use-and-entry projection behind the run cache browser.
package cache

import (
	"context"
	"fmt"
)

// CacheUseFilter identifies one bounded, searchable page of run cache uses.
type CacheUseFilter struct {
	RunID   int64
	Query   string
	Sort    string
	Order   string
	Page    int
	PerPage int
}

// CacheUseRow is one run cache use joined with its immutable cache entry.
type CacheUseRow struct {
	ID                 int64
	CacheLayer         string
	Outcome            string
	UsedAt             string
	CacheEntryID       int64
	Provider           string
	Namespace          string
	RequestFingerprint string
	ResponseStatus     int64
	PayloadArtifactID  *int64
	FetchedAt          string
	ExpiresAt          *string
	ExtractorVersion   string
}

// CacheUsePage is one bounded page of run cache uses.
type CacheUsePage struct {
	Items []*CacheUseRow
	Total int64
}

// ListCacheUsesForRun returns one bounded page of cache-use evidence for a run
// with its joined immutable cache entry.
func (s *Store) ListCacheUsesForRun(ctx context.Context, filter CacheUseFilter) (*CacheUsePage, error) {
	where, args, err := cacheUseWhere(filter)
	if err != nil {
		return nil, err
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM run_cache_uses rcu JOIN cache_entries ce ON ce.id=rcu.cache_entry_id WHERE "+where, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count cache uses: %w", err)
	}
	page := clampPage(filter.Page, filter.PerPage, total)
	query := "SELECT rcu.id, rcu.cache_layer, rcu.outcome, rcu.used_at, ce.id AS cache_entry_id, ce.provider, ce.namespace, ce.request_fingerprint, ce.response_status, ce.payload_artifact_id, ce.fetched_at, ce.expires_at, ce.extractor_version " +
		"FROM run_cache_uses rcu JOIN cache_entries ce ON ce.id=rcu.cache_entry_id WHERE " + where +
		" ORDER BY " + cacheUseSortExpression(filter.Sort) + " " + sqlDirection(filter.Order) + ", rcu.id " + sqlDirection(filter.Order) + " LIMIT ? OFFSET ?"
	queryArgs := append(append([]any(nil), args...), filter.PerPage, (page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("list cache uses: %w", err)
	}
	defer rows.Close()
	items := make([]*CacheUseRow, 0)
	for rows.Next() {
		var item CacheUseRow
		if err := rows.Scan(&item.ID, &item.CacheLayer, &item.Outcome, &item.UsedAt, &item.CacheEntryID, &item.Provider, &item.Namespace, &item.RequestFingerprint, &item.ResponseStatus, &item.PayloadArtifactID, &item.FetchedAt, &item.ExpiresAt, &item.ExtractorVersion); err != nil {
			return nil, fmt.Errorf("list cache uses: %w", err)
		}
		items = append(items, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list cache uses: %w", err)
	}
	return &CacheUsePage{Items: items, Total: total}, nil
}

// clampPage maps an offset request past the end to the final populated page.
func clampPage(page, perPage int, total int64) int {
	totalPages := (total + int64(perPage) - 1) / int64(perPage)
	if totalPages == 0 {
		return 1
	}
	if int64(page) > totalPages {
		return int(totalPages)
	}
	return page
}
