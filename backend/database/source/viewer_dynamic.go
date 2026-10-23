// viewer_dynamic.go retains the bounded dynamic source-record query. Sort
// identifiers come from a closed map, the search projection is fixed, every
// user value is bound, and the search, sort, order, and page inputs are
// validated and capped before assembly; no caller value is interpolated into
// SQL text.
package source

import (
	"fmt"
	"strings"
)

// maxViewerQueryLength bounds one viewer search string.
const maxViewerQueryLength = 200

// maxViewerPageSize bounds one viewer page request.
const maxViewerPageSize = 1000

// corpusRecordSortFields is the closed allowlist of source-record sort expressions.
var corpusRecordSortFields = map[string]string{
	"id":            "sr.id",
	"run_source_id": "sr.run_source_id",
	"source_name":   "rs.source_name",
	"source_type":   "rs.source_type",
	"record_index":  "sr.record_index",
	"parse_status":  "sr.parse_status",
	"reject_reason": "sr.reject_reason",
	"content_hash":  "sr.content_hash",
	"created_at":    "sr.created_at",
}

// corpusRecordSearchFields is the fixed searchable projection for source records.
var corpusRecordSearchFields = []string{"rs.source_name", "rs.source_type", "sr.parse_status", "sr.reject_reason", "sr.content_hash"}

// corpusRecordWhere builds the parameterized predicate for a source-record page.
func corpusRecordWhere(filter CorpusRecordFilter) (string, []any, error) {
	if err := validateCorpusRecordFilter(filter); err != nil {
		return "", nil, err
	}
	where, args := scopedSearch("rs.pipeline_run_id=?", corpusRecordSearchFields, filter.RunID, filter.Query)
	return where, args, nil
}

// validateCorpusRecordFilter validates the closed sort and order enums and the
// bounded search and page inputs before any SQL text is assembled.
func validateCorpusRecordFilter(filter CorpusRecordFilter) error {
	if _, ok := corpusRecordSortFields[filter.Sort]; !ok {
		return fmt.Errorf("list corpus source records: unsupported sort field %q", filter.Sort)
	}
	if err := validateViewerOrder("list corpus source records", filter.Order); err != nil {
		return err
	}
	if len(filter.Query) > maxViewerQueryLength {
		return fmt.Errorf("list corpus source records: query must be at most %d characters", maxViewerQueryLength)
	}
	if filter.Page < 1 {
		return fmt.Errorf("list corpus source records: page must be positive")
	}
	if filter.PerPage < 1 || filter.PerPage > maxViewerPageSize {
		return fmt.Errorf("list corpus source records: per_page must be between 1 and %d", maxViewerPageSize)
	}
	return nil
}

// corpusRecordSortExpression returns the allowlisted sort expression.
func corpusRecordSortExpression(sort string) string {
	return corpusRecordSortFields[sort]
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
