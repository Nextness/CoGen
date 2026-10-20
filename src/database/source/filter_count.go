// filter_count.go provides the handwritten per-source filter count data access
// layer over the generated statements. The upsert replaces the stored JSON for
// one run and source without revalidating its contents.
package source

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	generated "analysis/database/source/internal/sql"
)

// SetFilterData upserts the filter data for a source in a run.
func (s *Store) SetFilterData(ctx context.Context, input FilterCountInput) error {
	if err := s.queries.UpsertSourceFilterCount(ctx, generated.UpsertSourceFilterCountParams{
		PipelineRunID: input.RunID,
		SourceName:    input.SourceName,
		FilterData:    input.FilterData,
	}); err != nil {
		return fmt.Errorf("set source filter count: %w", err)
	}
	return nil
}

// ListFilterCountsByRun returns filter data for all sources in a run, ordered by source name.
func (s *Store) ListFilterCountsByRun(ctx context.Context, pipelineRunID int64) ([]*FilterCount, error) {
	rows, err := s.queries.ListSourceFilterCountsByRun(ctx, pipelineRunID)
	if err != nil {
		return nil, err
	}
	var counts []*FilterCount
	for _, row := range rows {
		counts = append(counts, filterCountFromGenerated(row))
	}
	return counts, nil
}

// GetFilterCount returns filter data for a specific source in a run, or nil if not found.
func (s *Store) GetFilterCount(ctx context.Context, pipelineRunID int64, sourceName string) (*FilterCount, error) {
	row, err := s.queries.GetSourceFilterCount(ctx, generated.GetSourceFilterCountParams{
		PipelineRunID: pipelineRunID,
		SourceName:    sourceName,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return filterCountFromGenerated(row), nil
}

// filterCountFromGenerated maps one generated filter-count row into an application count.
func filterCountFromGenerated(row generated.SourceFilterCount) *FilterCount {
	return &FilterCount{
		ID:            row.ID,
		PipelineRunID: row.PipelineRunID,
		SourceName:    row.SourceName,
		FilterData:    row.FilterData,
	}
}
