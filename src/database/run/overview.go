// overview.go provides the handwritten run purge overview over the generated
// statements. It answers whether a run still owns shared artifacts, reusable
// stages, or review contexts before any purge decision is made.
package run

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	generated "analysis/database/run/internal/sql"
)

// CheckPurgeEligibility verifies that no other run shares artifacts or reusable
// stage outputs from the given run. It is the safety check before purge.
// Returns an error if no pipeline run with the given ID exists.
func (s *Store) CheckPurgeEligibility(ctx context.Context, runID int64) (*PurgeEligibility, error) {
	// Verify the run exists first.
	existing, err := s.GetByID(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("check purge eligibility: %w", err)
	}
	if existing == nil {
		return nil, fmt.Errorf("pipeline run %d not found", runID)
	}

	// Check whether any other run's steps reference this run's artifacts.
	// We must check both input_artifact_id and output_artifact_id because the
	// artifacts table is content-addressed with INSERT OR IGNORE: a different
	// run may "produce" the same artifact (same content hash) and thus reference
	// this run's artifact as either input or output.
	sharedArtifactCount, err := s.queries.CountSharedRunArtifacts(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("check shared artifacts: %w", err)
	}

	// Check whether any other run reuses stages from this run.
	reusedByCount, err := s.queries.CountRunsReusingRun(ctx, generated.CountRunsReusingRunParams{
		ReusedFromRunID: nullableInt64(runID),
		PipelineRunID:   runID,
	})
	if err != nil {
		return nil, fmt.Errorf("check reused by count: %w", err)
	}

	ownedReviewContexts, err := s.queries.CountOwnedReviewContexts(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("check owned review contexts: %w", err)
	}
	dependentReviewContexts, err := s.queries.CountDependentReviewContexts(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("check dependent review contexts: %w", err)
	}

	eligible := sharedArtifactCount == 0 && reusedByCount == 0 && ownedReviewContexts == 0 && dependentReviewContexts == 0

	return &PurgeEligibility{
		Eligible:                    eligible,
		SharedArtifactCount:         int(sharedArtifactCount),
		ReusedByCount:               int(reusedByCount),
		OwnedReviewContextCount:     int(ownedReviewContexts),
		DependentReviewContextCount: int(dependentReviewContexts),
	}, nil
}

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
