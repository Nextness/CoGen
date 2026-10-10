// viewer_dynamic.go retains the bounded dynamic cache-use query. Sort
// identifiers come from a closed map, the search projection is fixed, and
// every user value is bound; no caller value is interpolated into SQL text.
package cache

import (
	"fmt"
	"strings"
)

// cacheUseSortFields is the closed allowlist of cache-use sort expressions.
var cacheUseSortFields = map[string]string{
	"id":                  "rcu.id",
	"cache_layer":         "rcu.cache_layer",
	"outcome":             "rcu.outcome",
	"used_at":             "rcu.used_at",
	"cache_entry_id":      "ce.id",
	"provider":            "ce.provider",
	"namespace":           "ce.namespace",
	"request_fingerprint": "ce.request_fingerprint",
	"response_status":     "ce.response_status",
	"payload_artifact_id": "ce.payload_artifact_id",
	"fetched_at":          "ce.fetched_at",
	"expires_at":          "ce.expires_at",
	"extractor_version":   "ce.extractor_version",
}

// cacheUseSearchFields is the fixed searchable projection for cache uses.
var cacheUseSearchFields = []string{"rcu.cache_layer", "rcu.outcome", "ce.provider", "ce.namespace", "ce.request_fingerprint"}

// cacheUseWhere builds the parameterized predicate for a cache-use page.
func cacheUseWhere(filter CacheUseFilter) (string, []any, error) {
	if _, ok := cacheUseSortFields[filter.Sort]; !ok {
		return "", nil, fmt.Errorf("list cache uses: unsupported sort field %q", filter.Sort)
	}
	where, args := scopedSearch("rcu.pipeline_run_id=?", cacheUseSearchFields, filter.RunID, filter.Query)
	return where, args, nil
}

// cacheUseSortExpression returns the allowlisted sort expression.
func cacheUseSortExpression(sort string) string {
	return cacheUseSortFields[sort]
}

// scopedSearch appends bound LIKE conditions for one optional search query.
func scopedSearch(base string, fields []string, runID int64, query string) (string, []any) {
	args := []any{runID}
	if query == "" {
		return base, args
	}
	conditions := make([]string, 0, len(fields))
	needle := "%" + strings.ToLower(query) + "%"
	for _, field := range fields {
		conditions = append(conditions, "LOWER(COALESCE("+field+", '')) LIKE ?")
		args = append(args, needle)
	}
	return base + " AND (" + strings.Join(conditions, " OR ") + ")", args
}

// sqlDirection returns the validated ascending/descending SQL keyword.
func sqlDirection(order string) string {
	if strings.EqualFold(order, "DESC") {
		return "DESC"
	}
	return "ASC"
}
