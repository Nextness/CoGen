// overview.go provides the handwritten run purge overview over the generated
// statements. It answers whether a run still owns shared artifacts, reusable
// stages, or review contexts before any purge decision is made.
package run

import (
	"context"
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
