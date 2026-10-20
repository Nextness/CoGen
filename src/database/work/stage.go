// stage.go provides the handwritten run-stage outcome data access layer over
// the generated statements. Stage/outcome vocabulary validation stays in Go
// around the generated upsert.
package work

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	generated "analysis/database/work/internal/sql"
)

// SetStageOutcome inserts or updates a stage outcome for a given work in a run.
// The upsert preserves the row identity (id, created_at) across progressive
// outcome updates (for example pending to parsed). Stage names and outcomes
// are validated against known stage/outcome pairs; impossible combinations are
// rejected.
func (s *Store) SetStageOutcome(ctx context.Context, input StageOutcomeInput) error {
	if input.StageName == "" {
		return fmt.Errorf("set run work stage outcome: stage_name is required")
	}
	if input.Outcome == "" {
		return fmt.Errorf("set run work stage outcome: outcome is required")
	}
	if !validStageName(input.StageName) {
		return fmt.Errorf("set run work stage outcome: invalid stage_name %q", input.StageName)
	}
	if !validStageOutcomeForStage(input.StageName, input.Outcome) {
		return fmt.Errorf("set run work stage outcome: outcome %q is not valid for stage %q", input.Outcome, input.StageName)
	}

	if err := s.queries.UpsertRunWorkStage(ctx, generated.UpsertRunWorkStageParams{
		PipelineRunID: input.RunID,
		WorkID:        input.WorkID,
		StageName:     input.StageName,
		Outcome:       input.Outcome,
		Reason:        nullableString(input.Reason),
	}); err != nil {
		return fmt.Errorf("set run work stage outcome: %w", err)
	}
	return nil
}

// GetStageOutcome returns the stage outcome for a specific work, run, and
// stage name, or nil if not found.
func (s *Store) GetStageOutcome(ctx context.Context, runID, workID int64, stageName string) (*StageOutcome, error) {
	row, err := s.queries.GetRunWorkStageByRunWorkStage(ctx, generated.GetRunWorkStageByRunWorkStageParams{
		PipelineRunID: runID,
		WorkID:        workID,
		StageName:     stageName,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return stageOutcomeFromGenerated(row), nil
}

// ListStageOutcomesByRun returns all stage outcomes for a given pipeline run,
// ordered by ID.
func (s *Store) ListStageOutcomesByRun(ctx context.Context, runID int64) ([]*StageOutcome, error) {
	rows, err := s.queries.ListRunWorkStagesByRunID(ctx, runID)
	if err != nil {
		return nil, err
	}
	var outcomes []*StageOutcome
	for _, row := range rows {
		outcomes = append(outcomes, stageOutcomeFromGenerated(row))
	}
	return outcomes, nil
}

// ListStageOutcomesByWork returns all stage outcomes across runs for a given
// work, ordered by ID.
func (s *Store) ListStageOutcomesByWork(ctx context.Context, workID int64) ([]*StageOutcome, error) {
	rows, err := s.queries.ListRunWorkStagesByWorkID(ctx, workID)
	if err != nil {
		return nil, err
	}
	var outcomes []*StageOutcome
	for _, row := range rows {
		outcomes = append(outcomes, stageOutcomeFromGenerated(row))
	}
	return outcomes, nil
}

// CountStageOutcomes returns the number of works that reached a given stage
// outcome within a pipeline run. Useful for dashboard funnel counts.
func (s *Store) CountStageOutcomes(ctx context.Context, runID int64, stageName, outcome string) (int, error) {
	count, err := s.queries.CountRunWorkStagesByStageAndOutcome(ctx, generated.CountRunWorkStagesByStageAndOutcomeParams{
		PipelineRunID: runID,
		StageName:     stageName,
		Outcome:       outcome,
	})
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// stageOutcomeFromGenerated maps one generated stage row into an application
// stage outcome and makes the nullable reason explicit.
func stageOutcomeFromGenerated(row generated.RunWorkStage) *StageOutcome {
	outcome := &StageOutcome{
		ID:            row.ID,
		PipelineRunID: row.PipelineRunID,
		WorkID:        row.WorkID,
		StageName:     row.StageName,
		Outcome:       row.Outcome,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
	if row.Reason.Valid {
		outcome.Reason = row.Reason.String
	}
	return outcome
}

// Valid stage names for run_work_stages.
var validStageNames = map[string]bool{
	StageNameParse:          true,
	StageNameDeduplicate:    true,
	StageNameValidate:       true,
	StageNameEnrich:         true,
	StageNameEnrichMetadata: true,
	StageNameEnrichIdentity: true,
	StageNameNormalize:      true,
}

// Allowed outcomes per stage. Each stage can only report outcomes that make
// sense for that pipeline step.
var allowedStageOutcomes = map[string]map[string]bool{
	StageNameParse: {
		OutcomeParsed:  true,
		OutcomeSkipped: true,
		OutcomePending: true,
	},
	StageNameDeduplicate: {
		OutcomeDuplicate:    true,
		OutcomeDeduplicated: true,
		OutcomeSkipped:      true,
		OutcomePending:      true,
	},
	StageNameValidate: {
		OutcomeValid:     true,
		OutcomeDiscarded: true,
		OutcomeSkipped:   true,
		OutcomePending:   true,
	},
	StageNameEnrich: {
		OutcomeEnriched: true,
		OutcomeSkipped:  true,
		OutcomePending:  true,
	},
	StageNameEnrichMetadata: {
		OutcomeEnriched: true,
		OutcomeSkipped:  true,
		OutcomePending:  true,
	},
	StageNameEnrichIdentity: {
		OutcomeEnriched: true,
		OutcomeFailed:   true,
		OutcomeSkipped:  true,
		OutcomePending:  true,
	},
	StageNameNormalize: {
		OutcomeNormalized: true,
		OutcomeSkipped:    true,
		OutcomePending:    true,
	},
}

// validStageName reports whether the supplied stage name is supported.
func validStageName(stage string) bool {
	return validStageNames[stage]
}

// validStageOutcomeForStage returns true if outcome is allowed for the given stage.
func validStageOutcomeForStage(stageName, outcome string) bool {
	allowed, ok := allowedStageOutcomes[stageName]
	if !ok {
		return false
	}
	return allowed[outcome]
}
