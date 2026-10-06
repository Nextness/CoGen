// evaluation_dynamic.go retains the bounded dynamic evaluation list and queue
// navigation builders. Sort identifiers come from a closed map, filter values
// are bound, and the server validates enum values before calling the family.
package work

import (
	"fmt"
	"strings"
)

// evaluationSortFields is the closed allowlist of evaluation sort expressions.
var evaluationSortFields = map[string]string{
	"id":                "wr.id",
	"title":             "wr.title",
	"year":              "wr.year",
	"journal":           "wr.journal",
	"publisher":         "wr.publisher",
	"source":            "wr.source",
	"doi":               "w.doi",
	"validation_status": "validation.outcome",
	"citation_count":    "wr.citation_count",
	"reference_count":   "wr.reference_count",
	"created_at":        "wr.created_at",
}

// evaluationFrom is the fixed evaluation join tree. The first placeholder is
// the selected review context.
const evaluationFrom = `FROM work_revisions wr JOIN works w ON w.id=wr.work_id
	LEFT JOIN run_work_stages validation ON validation.pipeline_run_id=wr.pipeline_run_id
		AND validation.work_id=wr.work_id AND validation.stage_name='validate'
	LEFT JOIN review_context_work_heads review_head ON review_head.review_context_id=? AND review_head.work_id=wr.work_id
	LEFT JOIN work_review_versions review ON review.id=review_head.review_version_id`

// evaluationSelectColumns is the fixed evaluation projection.
const evaluationSelectColumns = `wr.id, wr.work_id, wr.title, wr.year, wr.journal, wr.publisher, wr.source, w.doi, validation.outcome AS validation_status, wr.citation_count, wr.reference_count, wr.producer_stage, wr.created_at, wr.abstract, wr.keywords, wr.keywords_plus, (SELECT GROUP_CONCAT(ao.citation_name, '; ') FROM authorships a JOIN author_occurrences ao ON ao.id=a.author_occurrence_id WHERE a.work_revision_id=wr.id ORDER BY a.author_order) AS authors`

// evaluationQueryScope is one built evaluation predicate and its bound arguments.
type evaluationQueryScope struct {
	from           string
	where          string
	args           []any
	sortExpression string
}

// evaluationScope builds the parameterized evaluation predicate from a filter.
func evaluationScope(filter EvaluationFilter) (*evaluationQueryScope, error) {
	sortExpression, ok := evaluationSortFields[filter.SortField]
	if !ok {
		return nil, fmt.Errorf("evaluation: unsupported sort field %q", filter.SortField)
	}
	clauses := []string{"wr.pipeline_run_id=?", normalizedRevisionPredicate("wr")}
	args := []any{filter.ContextID, filter.RunID}
	if filter.Query != "" {
		clauses = append(clauses, "(LOWER(COALESCE(wr.title, '')) LIKE ? OR LOWER(COALESCE(w.doi, '')) LIKE ? OR LOWER(COALESCE(wr.journal, '')) LIKE ? OR LOWER(COALESCE(wr.publisher, '')) LIKE ? OR LOWER(COALESCE(wr.source, '')) LIKE ?)")
		needle := "%" + strings.ToLower(filter.Query) + "%"
		args = append(args, needle, needle, needle, needle, needle)
	}
	if filter.Source != "" {
		if filter.Source == "not_recorded" {
			clauses = append(clauses, "COALESCE(wr.source, '')=''")
		} else {
			clauses = append(clauses, "wr.source=?")
			args = append(args, filter.Source)
		}
	}
	if filter.ReviewStatus != "" {
		clauses = append(clauses, "COALESCE(review.status, 'not_evaluated')=?")
		args = append(args, filter.ReviewStatus)
	}
	if filter.Qualifier != "" {
		clauses = append(clauses, "EXISTS (SELECT 1 FROM work_review_version_substatuses sub WHERE sub.review_version_id=review.id AND sub.sub_status=?)")
		args = append(args, filter.Qualifier)
	}
	switch filter.ReviewSource {
	case "":
	case "this_context":
		clauses = append(clauses, "review.id IS NOT NULL AND review.created_in_context_id=?")
		args = append(args, filter.ContextID)
	case "inherited":
		clauses = append(clauses, "review.id IS NOT NULL AND review.created_in_context_id!=?")
		args = append(args, filter.ContextID)
	case "not_started":
		clauses = append(clauses, "review.id IS NULL")
	default:
		return nil, fmt.Errorf("evaluation: invalid review_source %q", filter.ReviewSource)
	}
	switch filter.Reviewed {
	case "":
	case "reviewed":
		clauses = append(clauses, "review.id IS NOT NULL AND review.status!='not_evaluated'")
	case "unreviewed":
		clauses = append(clauses, "(review.id IS NULL OR review.status='not_evaluated')")
	default:
		return nil, fmt.Errorf("evaluation: invalid reviewed %q", filter.Reviewed)
	}
	if filter.PDFStatus != "" {
		if filter.PDFStatus == "available" {
			clauses = append(clauses, "w.doi IN (SELECT value FROM json_each(?))")
		} else {
			clauses = append(clauses, "(w.doi IS NULL OR w.doi NOT IN (SELECT value FROM json_each(?)))")
		}
		args = append(args, filter.AvailableDOIsJSON)
	}
	return &evaluationQueryScope{
		from:           evaluationFrom,
		where:          strings.Join(clauses, " AND "),
		args:           args,
		sortExpression: sortExpression,
	}, nil
}
