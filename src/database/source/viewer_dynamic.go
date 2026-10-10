// viewer_dynamic.go retains the bounded dynamic projections and predicates for
// the source family's viewer reads. The projection fragments and sort
// identifiers come from closed allowlists, and every user value is bound; no
// caller value is interpolated into SQL text.
package source

import (
	"fmt"
	"strings"
)

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

// resultCountsQuery builds the run source result-count projection. The
// optional fragments describe columns that older databases do not provide;
// they are fixed strings selected by validated booleans.
func resultCountsQuery(includeExportDate, includeResultCounts bool) string {
	dateColumn := "NULL AS export_date"
	if includeExportDate {
		dateColumn = "export_date"
	}
	countColumns := "NULL AS expected_result_count, NULL AS observed_result_count, NULL AS result_count_comparison, "
	if includeResultCounts {
		countColumns = "expected_result_count, observed_result_count, result_count_comparison, "
	}
	return "SELECT id, source_name, source_type, expected_file, query, " + countColumns + dateColumn +
		" FROM run_sources WHERE pipeline_run_id=? ORDER BY id"
}

// corpusRecordWhere builds the parameterized predicate for a source-record page.
func corpusRecordWhere(filter CorpusRecordFilter) (string, []any, error) {
	if _, ok := corpusRecordSortFields[filter.Sort]; !ok {
		return "", nil, fmt.Errorf("list corpus source records: unsupported sort field %q", filter.Sort)
	}
	where, args := scopedSearch("rs.pipeline_run_id=?", corpusRecordSearchFields, filter.RunID, filter.Query)
	return where, args, nil
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

// sqlDirection returns the validated ascending/descending SQL keyword.
func sqlDirection(order string) string {
	if strings.EqualFold(order, "DESC") {
		return "DESC"
	}
	return "ASC"
}
