// graph_dynamic.go retains the bounded dynamic graph relationship builders.
// Filter columns and operators come from closed lists, every user value is
// bound, and the caller supplies the already validated row limits.
package work

import (
	"fmt"
	"strings"
)

// graphArticleWhere builds the parameterized graph article predicate.
func graphArticleWhere(filter GraphFilter) (string, []any, error) {
	clauses := []string{"wr.pipeline_run_id=?", normalizedRevisionPredicate("wr")}
	args := []any{filter.RunID}
	if filter.Query != "" {
		clauses = append(clauses, "(lower(COALESCE(wr.title,'')) LIKE lower(?) OR lower(COALESCE(w.doi,'')) LIKE lower(?))")
		like := "%" + filter.Query + "%"
		args = append(args, like, like)
	}
	if filter.Source != "" {
		clauses = append(clauses, "wr.source=?")
		args = append(args, filter.Source)
	}
	for _, bound := range []struct {
		value    *int64
		column   string
		operator string
	}{
		{filter.YearMin, "wr.year", ">="},
		{filter.YearMax, "wr.year", "<="},
		{filter.CitationMin, "wr.citation_count", ">="},
		{filter.CitationMax, "wr.citation_count", "<="},
		{filter.ReferenceMin, "wr.reference_count", ">="},
		{filter.ReferenceMax, "wr.reference_count", "<="},
	} {
		if bound.value != nil {
			clauses = append(clauses, bound.column+bound.operator+"?")
			args = append(args, *bound.value)
		}
	}
	if filter.Author != "" || filter.ORCID != "" {
		conditions := make([]string, 0, 2)
		authorArgs := make([]any, 0, 2)
		if filter.Author != "" {
			conditions = append(conditions, "lower(COALESCE(ao.citation_name,'')) LIKE lower(?)")
			authorArgs = append(authorArgs, "%"+filter.Author+"%")
		}
		if filter.ORCID != "" {
			conditions = append(conditions, "ao.orcid=?")
			authorArgs = append(authorArgs, filter.ORCID)
		}
		clauses = append(clauses, "EXISTS (SELECT 1 FROM authorships a JOIN author_occurrences ao ON ao.id=a.author_occurrence_id WHERE a.work_revision_id=wr.id AND "+strings.Join(conditions, " AND ")+")")
		args = append(args, authorArgs...)
	}
	if filter.Reference != "" {
		like := "%" + filter.Reference + "%"
		clauses = append(clauses, "EXISTS (SELECT 1 FROM reference_mentions rm WHERE rm.work_revision_id=wr.id AND (lower(COALESCE(rm.doi,'')) LIKE lower(?) OR lower(COALESCE(rm.title,'')) LIKE lower(?) OR lower(COALESCE(rm.author,'')) LIKE lower(?)))")
		args = append(args, like, like, like)
	}
	return strings.Join(clauses, " AND "), args, nil
}

// graphAuthorshipQuery builds the bounded article-author relationship query.
func graphAuthorshipQuery(revisionIDs []int64, authorLimit, rowLimit int) (string, []any, error) {
	if authorLimit < 1 || rowLimit < 1 {
		return "", nil, fmt.Errorf("list graph authorships: limits must be positive")
	}
	articlePlaceholders, articleArgs := placeholders(revisionIDs)
	query := `WITH eligible_authors AS (
		SELECT a.author_occurrence_id FROM authorships a
		WHERE a.work_revision_id IN (` + articlePlaceholders + `)
		GROUP BY a.author_occurrence_id ORDER BY MIN(a.id), a.author_occurrence_id LIMIT ?
	) SELECT a.work_revision_id, ao.id AS author_id, ao.citation_name, ao.orcid, a.author_order, a.affiliation
		FROM authorships a JOIN eligible_authors ea ON ea.author_occurrence_id=a.author_occurrence_id
		JOIN author_occurrences ao ON ao.id=a.author_occurrence_id
		WHERE a.work_revision_id IN (` + articlePlaceholders + `) ORDER BY a.id LIMIT ?`
	args := make([]any, 0, len(articleArgs)*2+2)
	args = append(args, articleArgs...)
	args = append(args, authorLimit)
	args = append(args, articleArgs...)
	args = append(args, rowLimit)
	return query, args, nil
}

// graphCitationQuery builds the bounded resolved citation relationship query.
func graphCitationQuery(revisionIDs, workIDs []int64, rowLimit int) (string, []any, error) {
	if rowLimit < 1 {
		return "", nil, fmt.Errorf("list graph citations: limit must be positive")
	}
	articlePlaceholders, articleArgs := placeholders(revisionIDs)
	workPlaceholders, workArgs := placeholders(workIDs)
	query := `SELECT rm.work_revision_id, rm.resolved_work_id FROM reference_mentions rm
		WHERE rm.work_revision_id IN (` + articlePlaceholders + `) AND rm.resolved_work_id IN (` + workPlaceholders + `) ORDER BY rm.id LIMIT ?`
	args := make([]any, 0, len(articleArgs)+len(workArgs)+1)
	args = append(args, articleArgs...)
	args = append(args, workArgs...)
	args = append(args, rowLimit)
	return query, args, nil
}

// graphReferenceQuery builds the bounded article-reference relationship query.
func graphReferenceQuery(revisionIDs []int64, rowLimit int) (string, []any, error) {
	if rowLimit < 1 {
		return "", nil, fmt.Errorf("list graph references: limit must be positive")
	}
	articlePlaceholders, articleArgs := placeholders(revisionIDs)
	query := `SELECT rm.id AS reference_id, rm.work_revision_id, rm.doi, rm.title, rm.author, rm.year, rm.source
		FROM reference_mentions rm WHERE rm.work_revision_id IN (` + articlePlaceholders + `) ORDER BY rm.id LIMIT ?`
	args := make([]any, 0, len(articleArgs)+1)
	args = append(args, articleArgs...)
	args = append(args, rowLimit)
	return query, args, nil
}

// placeholders returns a comma-separated SQL placeholder list and matching identifier arguments.
func placeholders(ids []int64) (string, []any) {
	parts, args := make([]string, len(ids)), make([]any, len(ids))
	for index, id := range ids {
		parts[index] = "?"
		args[index] = id
	}
	return strings.Join(parts, ","), args
}
