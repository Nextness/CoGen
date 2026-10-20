// viewer_dynamic.go retains the bounded dynamic cache-use query. Sort
// identifiers come from a closed map, the search projection is fixed, every
// user value is bound, and the search, sort, order, and page inputs are
// validated and capped before assembly; no caller value is interpolated into
// SQL text.
package cache

import (
	"fmt"
	"strings"
)

// maxViewerQueryLength bounds one viewer search string.
const maxViewerQueryLength = 200

// maxViewerPageSize bounds one viewer page request.
const maxViewerPageSize = 1000

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
	if err := validateCacheUseFilter(filter); err != nil {
		return "", nil, err
	}
	where, args := scopedSearch("rcu.pipeline_run_id=?", cacheUseSearchFields, filter.RunID, filter.Query)
	return where, args, nil
}

// validateCacheUseFilter validates the closed sort and order enums and the
// bounded search and page inputs before any SQL text is assembled.
func validateCacheUseFilter(filter CacheUseFilter) error {
	if _, ok := cacheUseSortFields[filter.Sort]; !ok {
		return fmt.Errorf("list cache uses: unsupported sort field %q", filter.Sort)
	}
	if err := validateViewerOrder("list cache uses", filter.Order); err != nil {
		return err
	}
	if len(filter.Query) > maxViewerQueryLength {
		return fmt.Errorf("list cache uses: query must be at most %d characters", maxViewerQueryLength)
	}
	if filter.Page < 1 {
		return fmt.Errorf("list cache uses: page must be positive")
	}
	if filter.PerPage < 1 || filter.PerPage > maxViewerPageSize {
		return fmt.Errorf("list cache uses: per_page must be between 1 and %d", maxViewerPageSize)
	}
	return nil
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

// validateViewerOrder validates the closed ascending/descending order enum.
func validateViewerOrder(operation, order string) error {
	switch strings.ToUpper(order) {
	case "", "ASC", "DESC":
		return nil
	default:
		return fmt.Errorf("%s: order must be asc or desc", operation)
	}
}

// sqlDirection returns the validated ascending/descending SQL keyword.
func sqlDirection(order string) string {
	if strings.EqualFold(order, "DESC") {
		return "DESC"
	}
	return "ASC"
}
