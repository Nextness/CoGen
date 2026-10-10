// step.go provides the handwritten run-step data access layer over the
// generated statements. Stage status validation, terminal finish timestamps,
// reuse links, artifact links, and fingerprint persistence stay in Go.
package run

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	generated "analysis/database/run/internal/sql"
	"analysis/manifest"
)

// CreateStep inserts a new run step record. Returns the step ID.
func (s *Store) CreateStep(ctx context.Context, pipelineRunID int64, stepName string) (int64, error) {
	result, err := s.queries.InsertRunStep(ctx, generated.InsertRunStepParams{
		PipelineRunID: pipelineRunID,
		StepName:      stepName,
		StartedAt:     sql.NullString{String: runStepTimestamp(), Valid: true},
	})
	if err != nil {
		return 0, fmt.Errorf("create run step: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// UpdateStepStatus updates the status and optional finish time of a run step.
// The status must be a valid manifest.StageOutcome value. finished_at is only
// set for terminal statuses (completed, skipped, reused, failed).
func (s *Store) UpdateStepStatus(ctx context.Context, stepID int64, status string) error {
	if err := manifest.ValidateStageOutcome(status); err != nil {
		return err
	}

	// Only set finished_at for terminal statuses.
	finishedAt := runStepTimestamp()
	var finished sql.NullString
	if status != string(manifest.StagePending) && status != string(manifest.StageRunning) {
		finished = sql.NullString{String: finishedAt, Valid: true}
	}

	return s.queries.UpdateRunStepStatus(ctx, generated.UpdateRunStepStatusParams{
		StepStatus: status,
		FinishedAt: finished,
		ID:         stepID,
	})
}

// LinkStepReuse records that a step reused output from a prior run. It sets
// both the status to "reused" and the finished_at timestamp since reuse is a
// terminal stage outcome.
func (s *Store) LinkStepReuse(ctx context.Context, input StepReuseInput) error {
	return s.queries.LinkRunStepReuse(ctx, generated.LinkRunStepReuseParams{
		ReusedFromRunID: nullableInt64(input.ReusedFromRunID),
		FinishedAt:      sql.NullString{String: runStepTimestamp(), Valid: true},
		ID:              input.StepID,
	})
}

// LinkStepInputArtifact records that a step consumed a specific artifact as input.
func (s *Store) LinkStepInputArtifact(ctx context.Context, input StepArtifactInput) error {
	return s.queries.LinkRunStepInputArtifact(ctx, generated.LinkRunStepInputArtifactParams{
		InputArtifactID: nullableInt64(input.ArtifactID),
		ID:              input.StepID,
	})
}

// LinkStepOutputArtifact records that a step produced a specific artifact as output.
func (s *Store) LinkStepOutputArtifact(ctx context.Context, input StepArtifactInput) error {
	return s.queries.LinkRunStepOutputArtifact(ctx, generated.LinkRunStepOutputArtifactParams{
		OutputArtifactID: nullableInt64(input.ArtifactID),
		ID:               input.StepID,
	})
}

// SetStepFingerprints records the immutable stage input and output fingerprints
// used to decide whether this stage may be reused by a later attempt.
func (s *Store) SetStepFingerprints(ctx context.Context, input StepFingerprintInput) error {
	if err := s.queries.SetRunStepFingerprints(ctx, generated.SetRunStepFingerprintsParams{
		InputFingerprint:  input.InputFingerprint,
		OutputFingerprint: input.OutputFingerprint,
		ID:                input.StepID,
	}); err != nil {
		return fmt.Errorf("set run step fingerprints: %w", err)
	}
	return nil
}

// ListStepsByRun returns all steps for a given pipeline run, ordered by ID.
func (s *Store) ListStepsByRun(ctx context.Context, pipelineRunID int64) ([]*Step, error) {
	rows, err := s.queries.ListRunStepsByRun(ctx, pipelineRunID)
	if err != nil {
		return nil, err
	}
	var steps []*Step
	for _, row := range rows {
		steps = append(steps, stepFromGenerated(row))
	}
	return steps, nil
}

// stepFromGenerated maps one generated run-step row into an application step
// and makes nullable artifact links and timestamps explicit.
func stepFromGenerated(row generated.ListRunStepsByRunRow) *Step {
	step := &Step{
		ID:                row.ID,
		PipelineRunID:     row.PipelineRunID,
		StepName:          row.StepName,
		StepStatus:        row.StepStatus,
		InputFingerprint:  row.InputFingerprint,
		OutputFingerprint: row.OutputFingerprint,
	}
	if row.InputArtifactID.Valid {
		step.InputArtifactID = &row.InputArtifactID.Int64
	}
	if row.OutputArtifactID.Valid {
		step.OutputArtifactID = &row.OutputArtifactID.Int64
	}
	if row.ReusedFromRunID.Valid {
		step.ReusedFromRunID = &row.ReusedFromRunID.Int64
	}
	if row.StartedAt.Valid {
		step.StartedAt = row.StartedAt.String
	}
	if row.FinishedAt.Valid {
		step.FinishedAt = row.FinishedAt.String
	}
	return step
}

// runStepTimestamp returns a microsecond-precision UTC timestamp for persisted stage timing.
func runStepTimestamp() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000000Z")
}

// ListStepsForViewer returns run steps ordered by ID with their derived
// duration for the viewer stage timeline.
func (s *Store) ListStepsForViewer(ctx context.Context, pipelineRunID int64) ([]*ViewerStep, error) {
	rows, err := s.queries.ListRunStepsForViewer(ctx, pipelineRunID)
	if err != nil {
		return nil, fmt.Errorf("list run steps for viewer: %w", err)
	}
	steps := make([]*ViewerStep, 0, len(rows))
	for _, row := range rows {
		steps = append(steps, &ViewerStep{
			StepName:          row.StepName,
			StepStatus:        row.StepStatus,
			InputArtifactID:   nullableInt64Pointer(row.InputArtifactID),
			OutputArtifactID:  nullableInt64Pointer(row.OutputArtifactID),
			StartedAt:         nullableStringPointer(row.StartedAt),
			FinishedAt:        nullableStringPointer(row.FinishedAt),
			InputFingerprint:  row.InputFingerprint,
			OutputFingerprint: row.OutputFingerprint,
			DurationSeconds:   nullableFloat64(row.DurationSeconds),
		})
	}
	return steps, nil
}

// nullableFloat64 converts a scanned numeric duration into an optional value.
func nullableFloat64(value any) *float64 {
	switch typed := value.(type) {
	case nil:
		return nil
	case float64:
		return &typed
	case int64:
		result := float64(typed)
		return &result
	default:
		return nil
	}
}
