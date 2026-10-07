// term_matches.go serves the stored per-run search-term inventories and
// per-revision field matches derived by the pipeline. It performs no query
// parsing and no matching; all reads are guarded so databases without the
// V00025 tables degrade to a null payload.
package server

import (
	"context"
)

// runSearchTerms returns the stored term inventory for one run ordered by id,
// plus the distinct term count. It returns nil data when the run has no stored
// terms or the table is absent.
func (s *Server) runSearchTerms(ctx context.Context, runID int64) ([]map[string]any, int64, error) {
	if !s.tableHasColumns("run_search_terms", "pipeline_run_id", "source_name", "term") {
		return nil, 0, nil
	}
	terms, err := s.workStore.ListRunTerms(ctx, runID)
	if err != nil {
		return nil, 0, err
	}
	items := make([]map[string]any, 0, len(terms))
	for _, term := range terms {
		items = append(items, map[string]any{"source_name": term.SourceName, "term": term.Term})
	}
	termTotal, err := s.workStore.CountDistinctRunTerms(ctx, runID)
	if err != nil {
		return nil, 0, err
	}
	return items, termTotal, nil
}

// revisionTermMatches returns the per-field matched terms for one revision.
func (s *Server) revisionTermMatches(ctx context.Context, runID, revisionID int64) (map[string][]string, error) {
	if !s.tableHasColumns("work_revision_term_matches", "pipeline_run_id", "work_revision_id", "field", "term") {
		return nil, nil
	}
	return s.workStore.GetRevisionMatches(ctx, runID, revisionID)
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
	return s.workStore.GetRevisionMatchesBulk(ctx, runID, revisionIDs)
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
