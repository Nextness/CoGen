// term_match.go provides the handwritten per-run search-term inventory and
// per-revision term-match data access layer over the generated statements.
// Replacement stays atomic in one transaction even for valid empty results.
package work

import (
	"context"
	"fmt"
	"sort"

	generated "analysis/database/work/internal/sql"
)

// ReplaceRunTermData replaces both the term inventory and the revision matches
// for one run in a single transaction, preserving per-run atomicity. The
// reconciliation marker is written even for valid empty results.
func (s *Store) ReplaceRunTermData(ctx context.Context, input ReplaceTermDataInput) error {
	return s.withTx(ctx, func(queries *generated.Queries) error {
		if err := replaceRunTerms(ctx, queries, input.RunID, input.TermsBySource); err != nil {
			return err
		}
		if err := replaceRunMatches(ctx, queries, input.RunID, input.Matches); err != nil {
			return err
		}
		if err := queries.UpsertRunTermReconciliation(ctx, input.RunID); err != nil {
			return fmt.Errorf("record term reconciliation: %w", err)
		}
		return nil
	})
}

// HasRunTermData reports whether term reconciliation completed, including valid
// empty results.
func (s *Store) HasRunTermData(ctx context.Context, runID int64) (bool, error) {
	found, err := s.queries.HasRunTermData(ctx, runID)
	if err != nil {
		return false, fmt.Errorf("check run term reconciliation: %w", err)
	}
	return found, nil
}

// ListRunTerms returns the stored term inventory for one run ordered by ID.
func (s *Store) ListRunTerms(ctx context.Context, runID int64) ([]RunTerm, error) {
	rows, err := s.queries.ListRunSearchTermsByRun(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("list run search terms: %w", err)
	}
	var terms []RunTerm
	for _, row := range rows {
		terms = append(terms, RunTerm{
			ID:            row.ID,
			PipelineRunID: row.PipelineRunID,
			SourceName:    row.SourceName,
			Term:          row.Term,
			CreatedAt:     row.CreatedAt,
		})
	}
	return terms, nil
}

// GetRevisionMatches returns the per-field matched terms for one revision.
func (s *Store) GetRevisionMatches(ctx context.Context, runID, revisionID int64) (map[string][]string, error) {
	rows, err := s.queries.ListRevisionTermMatches(ctx, generated.ListRevisionTermMatchesParams{
		PipelineRunID:  runID,
		WorkRevisionID: revisionID,
	})
	if err != nil {
		return nil, fmt.Errorf("list revision term matches: %w", err)
	}
	result := map[string][]string{}
	for _, row := range rows {
		result[row.Field] = append(result[row.Field], row.Term)
	}
	return result, nil
}

// GetRevisionMatchesBulk returns per-field matched terms for a page of
// revisions. It short-circuits when the revision list is empty.
func (s *Store) GetRevisionMatchesBulk(ctx context.Context, runID int64, revisionIDs []int64) (map[int64]map[string][]string, error) {
	if len(revisionIDs) == 0 {
		return map[int64]map[string][]string{}, nil
	}
	rows, err := s.queries.ListRevisionTermMatchesBulk(ctx, generated.ListRevisionTermMatchesBulkParams{
		PipelineRunID: runID,
		RevisionIds:   revisionIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("list revision term matches bulk: %w", err)
	}
	result := make(map[int64]map[string][]string)
	for _, row := range rows {
		if result[row.WorkRevisionID] == nil {
			result[row.WorkRevisionID] = map[string][]string{}
		}
		result[row.WorkRevisionID][row.Field] = append(result[row.WorkRevisionID][row.Field], row.Term)
	}
	return result, nil
}

// replaceRunTerms deletes and reinserts the term inventory for one run.
func replaceRunTerms(ctx context.Context, queries *generated.Queries, runID int64, termsBySource map[string][]string) error {
	if err := queries.DeleteRunSearchTerms(ctx, runID); err != nil {
		return fmt.Errorf("clear run search terms: %w", err)
	}
	sources := make([]string, 0, len(termsBySource))
	for source := range termsBySource {
		sources = append(sources, source)
	}
	sort.Strings(sources)
	for _, source := range sources {
		for _, term := range termsBySource[source] {
			if err := queries.InsertRunSearchTerm(ctx, generated.InsertRunSearchTermParams{
				PipelineRunID: runID,
				SourceName:    source,
				Term:          term,
			}); err != nil {
				return fmt.Errorf("insert run search term: %w", err)
			}
		}
	}
	return nil
}

// replaceRunMatches deletes and reinserts the revision matches for one run.
func replaceRunMatches(ctx context.Context, queries *generated.Queries, runID int64, matches map[int64]map[string][]string) error {
	if err := queries.DeleteRevisionTermMatches(ctx, runID); err != nil {
		return fmt.Errorf("clear revision term matches: %w", err)
	}
	revisionIDs := make([]int64, 0, len(matches))
	for revisionID := range matches {
		revisionIDs = append(revisionIDs, revisionID)
	}
	sort.Slice(revisionIDs, func(i, j int) bool { return revisionIDs[i] < revisionIDs[j] })
	for _, revisionID := range revisionIDs {
		fields := matches[revisionID]
		fieldNames := make([]string, 0, len(fields))
		for field := range fields {
			fieldNames = append(fieldNames, field)
		}
		sort.Strings(fieldNames)
		for _, field := range fieldNames {
			terms := append([]string(nil), fields[field]...)
			sort.Strings(terms)
			for _, term := range terms {
				if err := queries.InsertRevisionTermMatch(ctx, generated.InsertRevisionTermMatchParams{
					PipelineRunID:  runID,
					WorkRevisionID: revisionID,
					Field:          field,
					Term:           term,
				}); err != nil {
					return fmt.Errorf("insert revision term match: %w", err)
				}
			}
		}
	}
	return nil
}
