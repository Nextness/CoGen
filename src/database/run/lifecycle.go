// lifecycle.go provides the handwritten run lifecycle over the generated
// statements: atomic attempt allocation, terminal completion, visibility
// transitions, and abandoned-attempt recovery. Timestamps, status validation,
// retry policy, and transaction boundaries stay in Go.
package run

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	generated "analysis/database/run/internal/sql"
	"analysis/manifest"
)

// visibilityReasonLimit bounds locally supplied lifecycle explanations.
const visibilityReasonLimit = 1000

// ErrRunActive reports that a running attempt cannot change visibility.
var ErrRunActive = errors.New("a running attempt cannot be moved to or restored from trash")

// StartAttempt records the start of a pipeline run attempt linked to an
// execution plan. It atomically computes the next attempt_number for the given
// plan and retries on transient UNIQUE constraint conflicts or SQLITE_BUSY.
// Returns the run ID and attempt number.
func (s *Store) StartAttempt(ctx context.Context, input StartAttemptInput) (int64, int, error) {
	return s.startAttempt(ctx, input, false)
}

// StartAttemptIfIdle starts a new attempt only when this execution plan has no
// running attempt. The check and insert share one transaction so callers cannot
// race from a separate read into duplicate active work.
func (s *Store) StartAttemptIfIdle(ctx context.Context, input StartAttemptInput) (int64, int, error) {
	return s.startAttempt(ctx, input, true)
}

// startAttempt atomically starts the next plan attempt, optionally rejecting an already-running attempt.
func (s *Store) startAttempt(ctx context.Context, input StartAttemptInput, rejectRunning bool) (int64, int, error) {
	const maxRetries = 50
	var lastErr error

	for retry := 0; retry < maxRetries; retry++ {
		var runID int64
		var attemptNum int

		err := s.withTx(ctx, func(queries *generated.Queries) error {
			if rejectRunning {
				runningID, err := queries.FindRunningAttemptByPlan(ctx, nullableInt64(input.ExecutionPlanID))
				if err == nil {
					return &AttemptAlreadyRunningError{ExecutionPlanID: input.ExecutionPlanID, PipelineRunID: runningID}
				}
				if !errors.Is(err, sql.ErrNoRows) {
					return fmt.Errorf("lookup running attempt: %w", err)
				}
			}

			// Atomically determine the next attempt number within the transaction.
			maxAttempt, err := queries.NextAttemptNumber(ctx, nullableInt64(input.ExecutionPlanID))
			if err != nil {
				return fmt.Errorf("lookup attempt number: %w", err)
			}
			nextAttempt := int(maxAttempt) + 1

			result, err := queries.InsertAttempt(ctx, generated.InsertAttemptParams{
				Step:            input.Step,
				StartedAt:       runTimestamp(),
				SearchQuery:     nullableString(input.SearchQuery),
				ExecutionPlanID: nullableInt64(input.ExecutionPlanID),
				AttemptNumber:   nullableInt64(int64(nextAttempt)),
			})
			if err != nil {
				return err
			}
			id, err := result.LastInsertId()
			if err != nil {
				return fmt.Errorf("read inserted ID: %w", err)
			}
			runID = id
			attemptNum = nextAttempt
			return nil
		})

		if err == nil {
			return runID, attemptNum, nil
		}

		// Retry on transient errors with backoff.
		if isRetryableError(err) {
			lastErr = err
			if retry < 10 {
				continue
			}
			time.Sleep(time.Duration(retry) * time.Millisecond)
			continue
		}

		return 0, 0, err
	}

	return 0, 0, fmt.Errorf("start attempt after %d retries: %w", maxRetries, lastErr)
}

// FinishRun marks a pipeline run as completed (or failed). It validates the
// status against the manifest lifecycle vocabulary and only sets finished_at
// for terminal statuses (completed, failed).
func (s *Store) FinishRun(ctx context.Context, runID int64, status, summary string) error {
	if err := manifest.ValidateAttemptStatus(status); err != nil {
		return err
	}

	// Only set finished_at for terminal statuses.
	finishedAt := runTimestamp()
	var finished sql.NullString
	if status != string(manifest.AttemptRunning) {
		finished = sql.NullString{String: finishedAt, Valid: true}
	}

	return s.queries.FinishRun(ctx, generated.FinishRunParams{
		FinishedAt: finished,
		Status:     status,
		Summary:    nullableString(summary),
		ID:         runID,
	})
}

// Trash marks a pipeline run as trashed (soft-deleted).
func (s *Store) Trash(ctx context.Context, runID int64, reason string) error {
	return s.queries.TrashRun(ctx, generated.TrashRunParams{
		TrashedAt:   sql.NullString{String: runTimestamp(), Valid: true},
		TrashReason: sql.NullString{String: reason, Valid: true},
		ID:          runID,
	})
}

// Restore sets a trashed pipeline run back to active visibility.
func (s *Store) Restore(ctx context.Context, runID int64) error {
	return s.queries.RestoreRun(ctx, runID)
}

// UpdateVisibility validates the requested state, rejects a running attempt,
// updates the run, and appends matching audit evidence in one transaction. A
// run already in the requested state reports Changed=false without writing.
// An absent run returns nil, nil.
func (s *Store) UpdateVisibility(ctx context.Context, input VisibilityInput) (*VisibilityResult, error) {
	if input.VisibilityState != VisibilityActive && input.VisibilityState != VisibilityTrashed {
		return nil, fmt.Errorf("visibility_state must be active or trashed")
	}
	if len([]byte(input.Reason)) > visibilityReasonLimit {
		return nil, fmt.Errorf("reason must not exceed 1000 UTF-8 bytes")
	}

	var result *VisibilityResult
	err := s.withTx(ctx, func(queries *generated.Queries) error {
		current, err := queries.GetRunVisibility(ctx, input.RunID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if current.VisibilityState == input.VisibilityState {
			result = &VisibilityResult{VisibilityState: current.VisibilityState, Changed: false}
			return nil
		}
		if current.Status == string(manifest.AttemptRunning) {
			return ErrRunActive
		}

		occurredAt := time.Now().UTC().Format(time.RFC3339Nano)
		action := manifest.AuditRunRestored
		if input.VisibilityState == VisibilityTrashed {
			action = manifest.AuditRunTrashed
			if err := queries.TrashRun(ctx, generated.TrashRunParams{
				TrashedAt:   sql.NullString{String: occurredAt, Valid: true},
				TrashReason: sql.NullString{String: input.Reason, Valid: true},
				ID:          input.RunID,
			}); err != nil {
				return err
			}
		} else if err := queries.RestoreRun(ctx, input.RunID); err != nil {
			return err
		}

		beforeJSON, _ := json.Marshal(map[string]any{"visibility_state": current.VisibilityState})
		afterJSON, _ := json.Marshal(map[string]any{"visibility_state": input.VisibilityState})
		metadataJSON, _ := json.Marshal(map[string]any{"source": "local_viewer"})
		if err := queries.InsertRunVisibilityAudit(ctx, generated.InsertRunVisibilityAuditParams{
			OccurredAt:    occurredAt,
			PipelineRunID: nullableInt64(input.RunID),
			EntityID:      fmt.Sprint(input.RunID),
			Action:        string(action),
			BeforeJson:    sql.NullString{String: string(beforeJSON), Valid: true},
			AfterJson:     sql.NullString{String: string(afterJSON), Valid: true},
			MetadataJson:  sql.NullString{String: string(metadataJSON), Valid: true},
		}); err != nil {
			return err
		}
		result = &VisibilityResult{VisibilityState: input.VisibilityState, Changed: true}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// RecoverAbandoned atomically fails a running attempt and records recovery
// audit; the caller must hold exclusive pipeline ownership.
func (s *Store) RecoverAbandoned(ctx context.Context, runID int64) error {
	if runID <= 0 {
		return fmt.Errorf("recovery requires a positive run ID")
	}
	return s.withTx(ctx, func(queries *generated.Queries) error {
		result, err := queries.RecoverRun(ctx, generated.RecoverRunParams{
			FinishedAt: sql.NullString{String: runTimestamp(), Valid: true},
			ID:         runID,
		})
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if count != 1 {
			return fmt.Errorf("run %d is not a running attempt", runID)
		}
		if err := queries.FailRunningSteps(ctx, generated.FailRunningStepsParams{
			FinishedAt:    sql.NullString{String: runTimestamp(), Valid: true},
			PipelineRunID: runID,
		}); err != nil {
			return err
		}
		return insertRecoveryAudit(ctx, queries, runID)
	})
}

// withTx runs fn inside a transaction over the generated queries, rolling back
// on error and committing on success.
func (s *Store) withTx(ctx context.Context, fn func(*generated.Queries) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		// Rollback on panic; if fn returned an error the tx is already rolled back.
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()
	if err := fn(s.queries.WithTx(tx)); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// isRetryableError returns true if the error is a transient SQLite error that
// can be retried: UNIQUE constraint violation or SQLITE_BUSY (database locked).
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// modernc.org/sqlite reports constraint and lock failures through these messages.
	if strings.Contains(msg, "UNIQUE constraint failed") {
		return true
	}
	return strings.Contains(msg, "database is locked") || strings.Contains(msg, "SQLITE_BUSY")
}

// RunContext returns the complete ancestry and lifecycle projection for one
// run, or nil when the run does not exist.
func (s *Store) RunContext(ctx context.Context, runID int64) (*RunContext, error) {
	row, err := s.queries.GetRunContext(ctx, runID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get run context: %w", err)
	}
	return &RunContext{
		Search: RunContextSearch{
			ID:        row.SearchPk,
			SearchID:  row.SearchName,
			CreatedAt: row.SearchCreatedAt,
		},
		Revision: RunContextRevision{
			ID:                   row.RevisionPk,
			SearchID:             row.RevisionSearchID,
			Label:                row.RevisionLabel,
			ConfigArtifactHash:   row.ConfigArtifactHash,
			ResolvedManifestHash: row.ResolvedManifestHash,
			CreatedAt:            row.RevisionCreatedAt,
		},
		Plan: RunContextPlan{
			ID:                   row.PlanPk,
			SearchRevisionID:     row.PlanSearchRevisionID,
			ExecutionFingerprint: row.ExecutionFingerprint,
			ResolvedManifestHash: row.PlanResolvedManifestHash,
			InputManifestHash:    row.InputManifestHash,
			EnrichmentEnabled:    row.EnrichmentEnabled != 0,
			CreatedAt:            row.PlanCreatedAt,
		},
		Run: RunContextRun{
			ID:              row.RunPk,
			ExecutionPlanID: row.RunExecutionPlanID.Int64,
			Step:            row.Step,
			StartedAt:       row.StartedAt,
			FinishedAt:      nullableStringPointer(row.FinishedAt),
			Status:          row.Status,
			Summary:         nullableStringPointer(row.Summary),
			AttemptNumber:   row.AttemptNumber.Int64,
			VisibilityState: row.VisibilityState,
			TrashedAt:       nullableStringPointer(row.TrashedAt),
			TrashReason:     nullableStringPointer(row.TrashReason),
		},
		ReviewContextID: nullableInt64Pointer(row.ReviewContextID),
	}, nil
}

// GetVisibility returns the status and visibility state for one run, or nil
// when the run is absent.
func (s *Store) GetVisibility(ctx context.Context, runID int64) (*Visibility, error) {
	row, err := s.queries.GetRunVisibility(ctx, runID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get run visibility: %w", err)
	}
	return &Visibility{Status: row.Status, VisibilityState: row.VisibilityState}, nil
}
