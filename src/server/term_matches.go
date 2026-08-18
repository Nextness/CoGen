// term_matches.go serves the stored per-run search-term inventories and
// per-revision field matches derived by the pipeline. It performs no query
// parsing and no matching; all reads are guarded so databases without the
// V00025 tables degrade to a null payload.
package server

import (
	"context"
	"strings"
)

// runSearchTerms returns the stored term inventory for one run ordered by id,
// plus the distinct term count. It returns nil data when the run has no stored
// terms or the table is absent.
func (s *Server) runSearchTerms(ctx context.Context, runID int64) ([]map[string]any, int64, error) {
	if !s.tableHasColumns("run_search_terms", "pipeline_run_id", "source_name", "term") {
		return nil, 0, nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT source_name, term FROM run_search_terms WHERE pipeline_run_id=? ORDER BY id`, runID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items, err := rowsAsMaps(rows)
	if err != nil {
		return nil, 0, err
	}
	var termTotal int64
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT term) FROM run_search_terms WHERE pipeline_run_id=?`, runID).Scan(&termTotal); err != nil {
		return nil, 0, err
	}
	return items, termTotal, nil
}

// revisionTermMatches returns the per-field matched terms for one revision.
func (s *Server) revisionTermMatches(ctx context.Context, runID, revisionID int64) (map[string][]string, error) {
	if !s.tableHasColumns("work_revision_term_matches", "pipeline_run_id", "work_revision_id", "field", "term") {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT field, term FROM work_revision_term_matches WHERE pipeline_run_id=? AND work_revision_id=? ORDER BY id`, runID, revisionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items, err := rowsAsMaps(rows)
	if err != nil {
		return nil, err
	}
	return groupTermMatches(items), nil
}

// revisionTermMatchesBulk returns per-field matched terms for a page of
// revisions. It short-circuits when the revision list is empty.
func (s *Server) revisionTermMatchesBulk(ctx context.Context, runID int64, revisionIDs []int64) (map[int64]map[string][]string, error) {
	if len(revisionIDs) == 0 {
		return map[int64]map[string][]string{}, nil
	}
	if !s.tableHasColumns("work_revision_term_matches", "pipeline_run_id", "work_revision_id", "field", "term") {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(revisionIDs)-1) + "?"
	args := make([]any, 0, len(revisionIDs)+1)
	args = append(args, runID)
	for _, id := range revisionIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT work_revision_id, field, term FROM work_revision_term_matches
		 WHERE pipeline_run_id=? AND work_revision_id IN (`+placeholders+`) ORDER BY id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items, err := rowsAsMaps(rows)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]map[string][]string)
	for _, item := range items {
		revisionID, _ := item["work_revision_id"].(int64)
		field, _ := item["field"].(string)
		term, _ := item["term"].(string)
		if result[revisionID] == nil {
			result[revisionID] = map[string][]string{}
		}
		result[revisionID][field] = append(result[revisionID][field], term)
	}
	return result, nil
}

// groupTermMatches groups (field, term) rows into per-field term lists.
func groupTermMatches(items []map[string]any) map[string][]string {
	result := map[string][]string{}
	for _, item := range items {
		field, _ := item["field"].(string)
		term, _ := item["term"].(string)
		result[field] = append(result[field], term)
	}
	return result
}

// detailTermMatches builds the full term-coverage payload for one revision,
// including per-term source attribution and the unmatched term list. It
// returns nil when the run has no stored terms.
func detailTermMatches(termRows []map[string]any, termTotal int64, revisionMatches map[string][]string) map[string]any {
	if len(termRows) == 0 {
		return nil
	}
	termsWithSources := map[string][]string{}
	sources := []string{}
	seenSources := map[string]bool{}
	orderedTerms := []string{}
	seenTerms := map[string]bool{}
	for _, row := range termRows {
		term, _ := row["term"].(string)
		source, _ := row["source_name"].(string)
		termsWithSources[term] = append(termsWithSources[term], source)
		if !seenSources[source] {
			seenSources[source] = true
			sources = append(sources, source)
		}
		if !seenTerms[term] {
			seenTerms[term] = true
			orderedTerms = append(orderedTerms, term)
		}
	}
	matched := matchedTermSet(revisionMatches)
	unmatched := []string{}
	for _, term := range orderedTerms {
		if !matched[term] {
			unmatched = append(unmatched, term)
		}
	}
	return map[string]any{
		"title":              emptyIfNil(revisionMatches["title"]),
		"abstract":           emptyIfNil(revisionMatches["abstract"]),
		"keywords":           emptyIfNil(revisionMatches["keywords"]),
		"keywords_plus":      emptyIfNil(revisionMatches["keywords_plus"]),
		"matched_total":      len(matched),
		"term_total":         termTotal,
		"sources":            sources,
		"terms_with_sources": termsWithSources,
		"unmatched":          unmatched,
	}
}

// rowTermMatches builds the compact term-coverage payload for one corpus row.
// It returns nil when the run has no stored terms.
func rowTermMatches(termRows []map[string]any, termTotal int64, revisionMatches map[string][]string) map[string]any {
	if len(termRows) == 0 {
		return nil
	}
	sources := []string{}
	seenSources := map[string]bool{}
	for _, row := range termRows {
		source, _ := row["source_name"].(string)
		if !seenSources[source] {
			seenSources[source] = true
			sources = append(sources, source)
		}
	}
	matched := matchedTermSet(revisionMatches)
	return map[string]any{
		"title":         emptyIfNil(revisionMatches["title"]),
		"abstract":      emptyIfNil(revisionMatches["abstract"]),
		"keywords":      emptyIfNil(revisionMatches["keywords"]),
		"keywords_plus": emptyIfNil(revisionMatches["keywords_plus"]),
		"matched_total": len(matched),
		"term_total":    termTotal,
		"sources":       sources,
	}
}

// matchedTermSet returns the distinct set of terms matched across all fields.
func matchedTermSet(revisionMatches map[string][]string) map[string]bool {
	matched := map[string]bool{}
	for _, terms := range revisionMatches {
		for _, term := range terms {
			matched[term] = true
		}
	}
	return matched
}

// emptyIfNil returns an empty slice so JSON renders [] instead of null.
func emptyIfNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
