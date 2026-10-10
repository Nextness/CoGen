// corpus_dynamic.go retains the bounded dynamic sort/search builders for the
// run-scoped corpus collections. Sort identifiers come from closed maps, every
// user value is bound, and the search, sort, order, and page inputs are
// validated and capped before assembly; no caller value is interpolated into
// SQL text.
package work

import (
	"fmt"
	"strings"
)

// maxViewerQueryLength bounds one viewer search string.
const maxViewerQueryLength = 200

// maxViewerPageSize bounds one viewer page request.
const maxViewerPageSize = 1000

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

// corpusAuthorSortFields is the closed allowlist of author sort expressions.
var corpusAuthorSortFields = map[string]string{
	"id":                "ao.id",
	"citation_name":     "ao.citation_name",
	"first_name":        "ao.first_name",
	"last_name":         "ao.last_name",
	"orcid":             "ao.orcid",
	"article_count":     "article_count",
	"affiliation_count": "affiliation_count",
	"created_at":        "ao.created_at",
}

// corpusAuthorSearchFields is the fixed searchable projection for authors.
var corpusAuthorSearchFields = []string{"ao.citation_name", "ao.first_name", "ao.last_name", "ao.orcid"}

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
	if err := validateCorpusPage("list corpus references", filter.Query, filter.Order, filter.Page, filter.PerPage); err != nil {
		return "", nil, err
	}
	where, args := scopedSearch("wr.pipeline_run_id=? AND "+normalizedRevisionPredicate("wr"), corpusReferenceSearchFields, filter.RunID, filter.Query)
	return where, args, nil
}

// corpusReferenceSortExpression returns the allowlisted sort expression.
func corpusReferenceSortExpression(sort string) string {
	return corpusReferenceSortFields[sort]
}

// corpusAuthorWhere builds the parameterized predicate for an author page.
func corpusAuthorWhere(filter CorpusAuthorFilter) (string, []any, error) {
	if _, ok := corpusAuthorSortFields[filter.Sort]; !ok {
		return "", nil, fmt.Errorf("list corpus authors: unsupported sort field %q", filter.Sort)
	}
	if err := validateCorpusPage("list corpus authors", filter.Query, filter.Order, filter.Page, filter.PerPage); err != nil {
		return "", nil, err
	}
	where, args := scopedSearch("wr.pipeline_run_id=? AND "+normalizedRevisionPredicate("wr"), corpusAuthorSearchFields, filter.RunID, filter.Query)
	return where, args, nil
}

// corpusAuthorSortExpression returns the allowlisted sort expression.
func corpusAuthorSortExpression(sort string) string {
	return corpusAuthorSortFields[sort]
}

// runStageWhere builds the parameterized predicate for a run-stage page.
func runStageWhere(filter RunStageFilter) (string, []any, error) {
	if _, ok := runStageSortFields[filter.Sort]; !ok {
		return "", nil, fmt.Errorf("list run stages: unsupported sort field %q", filter.Sort)
	}
	if err := validateCorpusPage("list run stages", filter.Query, filter.Order, filter.Page, filter.PerPage); err != nil {
		return "", nil, err
	}
	where, args := scopedSearch("rws.pipeline_run_id=?", runStageSearchFields, filter.RunID, filter.Query)
	return where, args, nil
}

// runStageSortExpression returns the allowlisted sort expression.
func runStageSortExpression(sort string) string {
	return runStageSortFields[sort]
}

// validateCorpusPage validates the bounded search, order, and page inputs
// before any SQL text is assembled.
func validateCorpusPage(operation, query, order string, page, perPage int) error {
	if err := validateViewerOrder(operation, order); err != nil {
		return err
	}
	if len(query) > maxViewerQueryLength {
		return fmt.Errorf("%s: query must be at most %d characters", operation, maxViewerQueryLength)
	}
	if page < 1 {
		return fmt.Errorf("%s: page must be positive", operation)
	}
	if perPage < 1 || perPage > maxViewerPageSize {
		return fmt.Errorf("%s: per_page must be between 1 and %d", operation, maxViewerPageSize)
	}
	return nil
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
