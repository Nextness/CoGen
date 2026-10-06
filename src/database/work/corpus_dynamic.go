// corpus_dynamic.go retains the bounded dynamic sort/search builders for the
// run-scoped corpus collections. Sort identifiers come from closed maps and
// every user value is bound; no caller value is interpolated into SQL text.
package work

import (
	"fmt"
	"strings"
)

// corpusReferenceSortFields is the closed allowlist of reference sort expressions.
var corpusReferenceSortFields = map[string]string{
	"id":               "rm.id",
	"work_revision_id": "rm.work_revision_id",
	"mention_order":    "rm.mention_order",
	"doi":              "rm.doi",
	"title":            "rm.title",
	"author":           "rm.author",
	"year":             "rm.year",
	"source":           "rm.source",
	"resolved_work_id": "rm.resolved_work_id",
	"created_at":       "rm.created_at",
}

// corpusReferenceSearchFields is the fixed searchable projection for references.
var corpusReferenceSearchFields = []string{"rm.doi", "rm.title", "rm.author", "rm.source", "wr.title"}

// runStageSortFields is the closed allowlist of run-stage sort expressions.
var runStageSortFields = map[string]string{
	"id":         "rws.id",
	"work_id":    "rws.work_id",
	"stage_name": "rws.stage_name",
	"outcome":    "rws.outcome",
	"reason":     "rws.reason",
	"created_at": "rws.created_at",
	"updated_at": "rws.updated_at",
}

// runStageSearchFields is the fixed searchable projection for run stages.
var runStageSearchFields = []string{"rws.stage_name", "rws.outcome", "rws.reason", "CAST(rws.work_id AS TEXT)"}

// corpusReferenceWhere builds the parameterized predicate for a reference page.
func corpusReferenceWhere(filter CorpusReferenceFilter) (string, []any, error) {
	if _, ok := corpusReferenceSortFields[filter.Sort]; !ok {
		return "", nil, fmt.Errorf("list corpus references: unsupported sort field %q", filter.Sort)
	}
	where, args := scopedSearch("wr.pipeline_run_id=? AND "+normalizedRevisionPredicate("wr"), corpusReferenceSearchFields, filter.RunID, filter.Query)
	return where, args, nil
}

// corpusReferenceSortExpression returns the allowlisted sort expression.
func corpusReferenceSortExpression(sort string) string {
	return corpusReferenceSortFields[sort]
}

// runStageWhere builds the parameterized predicate for a run-stage page.
func runStageWhere(filter RunStageFilter) (string, []any, error) {
	if _, ok := runStageSortFields[filter.Sort]; !ok {
		return "", nil, fmt.Errorf("list run stages: unsupported sort field %q", filter.Sort)
	}
	where, args := scopedSearch("rws.pipeline_run_id=?", runStageSearchFields, filter.RunID, filter.Query)
	return where, args, nil
}

// runStageSortExpression returns the allowlisted sort expression.
func runStageSortExpression(sort string) string {
	return runStageSortFields[sort]
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
