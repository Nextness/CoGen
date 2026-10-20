// overview.go provides generated run ancestry and selected-attempt projections.
package run

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	generated "analysis/database/run/internal/sql"
)

// LatestRunWithAncestry returns the newest run with complete search ancestry,
// or nil when no run exists.
func (s *Store) LatestRunWithAncestry(ctx context.Context) (*RunAncestry, error) {
	row, err := s.queries.GetLatestRunWithAncestry(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest run ancestry: %w", err)
	}
	return &RunAncestry{
		ID:               row.ID,
		AttemptNumber:    nullableIntPointer(row.AttemptNumber),
		StartedAt:        row.StartedAt,
		FinishedAt:       nullableStringPointer(row.FinishedAt),
		Status:           row.Status,
		VisibilityState:  row.VisibilityState,
		SearchID:         nullableInt64Pointer(row.SearchID),
		SearchName:       row.SearchName.String,
		SearchRevisionID: nullableInt64Pointer(row.SearchRevisionID),
		RevisionLabel:    row.RevisionLabel.String,
		ExecutionPlanID:  nullableInt64Pointer(row.ExecutionPlanID),
	}, nil
}

// SelectedAttempt returns one exact non-trashed attempt for the given plan, or
// nil when it is absent.
func (s *Store) SelectedAttempt(ctx context.Context, planID, selectedID int64) (*AttemptSummary, error) {
	row, err := s.queries.GetSelectedAttempt(ctx, generated.GetSelectedAttemptParams{
		ID:              selectedID,
		ExecutionPlanID: sql.NullInt64{Int64: planID, Valid: true},
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get selected attempt: %w", err)
	}
	return &AttemptSummary{
		ID:              row.ID,
		AttemptNumber:   nullableIntPointer(row.AttemptNumber),
		StartedAt:       row.StartedAt,
		Status:          row.Status,
		VisibilityState: row.VisibilityState,
	}, nil
}

// RunArtifactContext returns the canonical ancestry for one run, or nil when
// the run is absent.
func (s *Store) RunArtifactContext(ctx context.Context, runID int64) (*RunArtifactContext, error) {
	row, err := s.queries.GetRunArtifactContext(ctx, runID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get run artifact context: %w", err)
	}
	return &RunArtifactContext{
		SearchID:             row.SearchID,
		SearchRevisionID:     row.SearchRevisionID,
		SearchRevisionLabel:  row.RevisionLabel,
		ExecutionPlanID:      row.ExecutionPlanID,
		ExecutionFingerprint: row.ExecutionFingerprint,
		RunID:                row.RunID,
		AttemptNumber:        nullableIntPointer(row.AttemptNumber),
	}, nil
}
