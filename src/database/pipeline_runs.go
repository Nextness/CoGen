// pipeline_runs.go provides the PipelineRunRepository that manages
// pipeline run creation, status transitions, and completion tracking
// for the workspace pipeline lifecycle.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"analysis/manifest"
)

// PipelineRun represents a row in the pipeline_runs table.
type PipelineRun struct {
	ID              int64   `json:"id"`
	Step            string  `json:"step"`
	StartedAt       string  `json:"started_at"`
	FinishedAt      *string `json:"finished_at,omitempty"`
	Status          string  `json:"status"`
	Summary         *string `json:"summary,omitempty"`
	SearchQuery     *string `json:"search_query,omitempty"`
	ExecutionPlanID *int64  `json:"execution_plan_id,omitempty"`
	AttemptNumber   *int    `json:"attempt_number,omitempty"`
	VisibilityState string  `json:"visibility_state"`
	TrashedAt       *string `json:"trashed_at,omitempty"`
	TrashReason     *string `json:"trash_reason,omitempty"`
}

// AttemptAlreadyRunningError reports the active attempt that prevents another
// attempt for the same execution plan from starting.
type AttemptAlreadyRunningError struct {
	ExecutionPlanID int64
	PipelineRunID   int64
}

// Error returns the receiver's diagnostic message.
func (e *AttemptAlreadyRunningError) Error() string {
	return fmt.Sprintf("execution plan %d already has running attempt %d", e.ExecutionPlanID, e.PipelineRunID)
}

// PipelineRunRepository provides CRUD for the pipeline_runs table.
type PipelineRunRepository struct {
	db *Database
}

// StartRun records the start of a pipeline step. Returns the run ID.
// This is the legacy entry point; new code should use StartAttempt.
func (r *PipelineRunRepository) StartRun(step, searchQuery string) (int64, error) {
	res, err := r.db.DB.Exec(
		"INSERT INTO pipeline_runs (step, started_at, status, search_query) VALUES (?, ?, 'running', ?)",
		step, timestamp(), nullStr(searchQuery),
	)
	if err != nil {
		lg.Debug("pipeline run start failed", "step", step, "error", err)
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		lg.Debug("pipeline run started ID read failed", "step", step, "error", err)
		return 0, err
	}
	lg.Debug("pipeline run start successful", "step", step, "run_id", id)
	return id, nil
}

// StartAttempt records the start of a pipeline run attempt linked to an execution plan.
// It atomically computes the next attempt_number for the given plan and retries
// on transient UNIQUE constraint conflicts or SQLITE_BUSY.
// Returns the run ID and attempt number.
func (r *PipelineRunRepository) StartAttempt(executionPlanID int64, step, searchQuery string) (int64, int, error) {
	return r.startAttempt(executionPlanID, step, searchQuery, false)
}

// StartAttemptIfIdle starts a new attempt only when this execution plan has no
// running attempt. The check and insert share one transaction so callers cannot
// race from a separate read into duplicate active work.
func (r *PipelineRunRepository) StartAttemptIfIdle(executionPlanID int64, step, searchQuery string) (int64, int, error) {
	return r.startAttempt(executionPlanID, step, searchQuery, true)
}

// startAttempt atomically starts the next plan attempt, optionally rejecting an already-running attempt.
func (r *PipelineRunRepository) startAttempt(executionPlanID int64, step, searchQuery string, rejectRunning bool) (int64, int, error) {
	const maxRetries = 50
	var lastErr error

	for retry := 0; retry < maxRetries; retry++ {
		var runID int64
		var attemptNum int

		err := r.db.withTx(context.Background(), func(tx *sql.Tx) error {
			if rejectRunning {
				var runningID int64
				err := tx.QueryRow(
					"SELECT id FROM pipeline_runs WHERE execution_plan_id = ? AND status = 'running' ORDER BY attempt_number DESC LIMIT 1",
					executionPlanID,
				).Scan(&runningID)
				if err == nil {
					return &AttemptAlreadyRunningError{ExecutionPlanID: executionPlanID, PipelineRunID: runningID}
				}
				if err != sql.ErrNoRows {
					return fmt.Errorf("lookup running attempt: %w", err)
				}
			}

			// Atomically determine the next attempt number within the transaction.
			var maxAttempt sql.NullInt64
			err := tx.QueryRow(
				"SELECT MAX(attempt_number) FROM pipeline_runs WHERE execution_plan_id = ?",
				executionPlanID,
			).Scan(&maxAttempt)
			if err != nil {
				return fmt.Errorf("lookup attempt number: %w", err)
			}
			nextAttempt := 1
			if maxAttempt.Valid {
				nextAttempt = int(maxAttempt.Int64) + 1
			}

			res, err := tx.Exec(
				`INSERT INTO pipeline_runs
					(step, started_at, status, search_query, execution_plan_id, attempt_number)
				 VALUES (?, ?, 'running', ?, ?, ?)`,
				step, timestamp(), nullStr(searchQuery), executionPlanID, nextAttempt,
			)
			if err != nil {
				return err
			}
			id, err := res.LastInsertId()
			if err != nil {
				return fmt.Errorf("read inserted ID: %w", err)
			}
			runID = id
			attemptNum = nextAttempt
			return nil
		})

		if err == nil {
			lg.Debug("pipeline run attempt start successful",
				"step", step, "run_id", runID, "execution_plan_id", executionPlanID, "attempt", attemptNum)
			return runID, attemptNum, nil
		}

		// Retry on transient errors with backoff.
		if isRetryableError(err) {
			lastErr = err
			lg.Debug("pipeline run attempt retry",
				"step", step, "execution_plan_id", executionPlanID, "retry", retry+1, "error", err)
			// Small backoff to reduce contention.
			if retry < 10 {
				continue
			}
			time.Sleep(time.Duration(retry) * time.Millisecond)
			continue
		}

		lg.Debug("pipeline run attempt start failed",
			"step", step, "execution_plan_id", executionPlanID, "error", err)
		return 0, 0, err
	}

	lg.Debug("pipeline run attempt start failed after retries",
		"step", step, "execution_plan_id", executionPlanID, "retries", maxRetries, "last_error", lastErr)
	return 0, 0, fmt.Errorf("start attempt after %d retries: %w", maxRetries, lastErr)
}

// FinishRun marks a pipeline run as completed (or failed).
// Supports both legacy runs and new attempt-based runs.
// It validates the status against the manifest lifecycle vocabulary and only
// sets finished_at for terminal statuses (completed, failed).
func (r *PipelineRunRepository) FinishRun(runID int64, status string, summary string) error {
	if err := manifest.ValidateAttemptStatus(status); err != nil {
		lg.Debug("pipeline run finish rejected", "run_id", runID, "status", status, "error", err)
		return err
	}

	// Only set finished_at for terminal statuses.
	now := timestamp()
	finishedAt := &now
	if status == string(manifest.AttemptRunning) {
		finishedAt = nil
	}

	_, err := r.db.DB.Exec(
		"UPDATE pipeline_runs SET finished_at = ?, status = ?, summary = ? WHERE id = ?",
		nullStrPtr(finishedAt), status, nullStr(summary), runID,
	)
	if err != nil {
		lg.Debug("pipeline run finish failed", "run_id", runID, "status", status, "error", err)
		return err
	}
	lg.Debug("pipeline run finish successful", "run_id", runID, "status", status)
	return nil
}

// Trash marks a pipeline run as trashed (soft-deleted).
func (r *PipelineRunRepository) Trash(runID int64, reason string) error {
	_, err := r.db.DB.Exec(
		"UPDATE pipeline_runs SET visibility_state = 'trashed', trashed_at = ?, trash_reason = ? WHERE id = ?",
		timestamp(), reason, runID,
	)
	if err != nil {
		lg.Debug("pipeline run trash failed", "run_id", runID, "error", err)
		return err
	}
	lg.Debug("pipeline run trash successful", "run_id", runID)
	return nil
}

// Restore sets a trashed pipeline run back to active visibility.
func (r *PipelineRunRepository) Restore(runID int64) error {
	_, err := r.db.DB.Exec(
		"UPDATE pipeline_runs SET visibility_state = 'active', trashed_at = NULL, trash_reason = NULL WHERE id = ?",
		runID,
	)
	if err != nil {
		lg.Debug("pipeline run restore failed", "run_id", runID, "error", err)
		return err
	}
	lg.Debug("pipeline run restore successful", "run_id", runID)
	return nil
}

// GetByID returns a pipeline run by its primary key, or nil if not found.
func (r *PipelineRunRepository) GetByID(runID int64) (*PipelineRun, error) {
	var pr PipelineRun
	var finishedAt, summary, searchQueryNull, trashedAt, trashReason sql.NullString
	var execPlanID sql.NullInt64
	var attemptNum sql.NullInt64
	err := r.db.DB.QueryRow(
		`SELECT id, step, started_at, finished_at, status, summary, search_query,
		        execution_plan_id, attempt_number, visibility_state, trashed_at, trash_reason
		 FROM pipeline_runs WHERE id = ?`, runID,
	).Scan(&pr.ID, &pr.Step, &pr.StartedAt, &finishedAt, &pr.Status, &summary, &searchQueryNull,
		&execPlanID, &attemptNum, &pr.VisibilityState, &trashedAt, &trashReason)
	if err == sql.ErrNoRows {
		lg.Debug("pipeline run query successful", "id", runID, "result", "not_found")
		return nil, nil
	}
	if err != nil {
		lg.Debug("pipeline run query failed", "id", runID, "error", err)
		return nil, err
	}

	if finishedAt.Valid {
		pr.FinishedAt = &finishedAt.String
	}
	if summary.Valid {
		pr.Summary = &summary.String
	}
	if searchQueryNull.Valid {
		pr.SearchQuery = &searchQueryNull.String
	}
	if execPlanID.Valid {
		pr.ExecutionPlanID = &execPlanID.Int64
	}
	if attemptNum.Valid {
		n := int(attemptNum.Int64)
		pr.AttemptNumber = &n
	}
	if trashedAt.Valid {
		pr.TrashedAt = &trashedAt.String
	}
	if trashReason.Valid {
		pr.TrashReason = &trashReason.String
	}
	lg.Debug("pipeline run query successful", "id", runID, "status", pr.Status, "result", "found")
	return &pr, nil
}

// ListByPlan returns all runs for a given execution plan, ordered by attempt number.
func (r *PipelineRunRepository) ListByPlan(executionPlanID int64) ([]*PipelineRun, error) {
	rows, err := r.db.DB.Query(
		`SELECT id, step, started_at, finished_at, status, summary, search_query,
		        execution_plan_id, attempt_number, visibility_state, trashed_at, trash_reason
		 FROM pipeline_runs WHERE execution_plan_id = ? ORDER BY attempt_number`,
		executionPlanID,
	)
	if err != nil {
		lg.Debug("pipeline run list by plan failed", "execution_plan_id", executionPlanID, "error", err)
		return nil, err
	}
	defer rows.Close()

	return scanPipelineRuns(rows)
}

// ListByVisibility returns all runs with a given visibility state, ordered by ID.
func (r *PipelineRunRepository) ListByVisibility(visibilityState string) ([]*PipelineRun, error) {
	rows, err := r.db.DB.Query(
		`SELECT id, step, started_at, finished_at, status, summary, search_query,
		        execution_plan_id, attempt_number, visibility_state, trashed_at, trash_reason
		 FROM pipeline_runs WHERE visibility_state = ? ORDER BY id`,
		visibilityState,
	)
	if err != nil {
		lg.Debug("pipeline run list by visibility failed", "visibility", visibilityState, "error", err)
		return nil, err
	}
	defer rows.Close()

	return scanPipelineRuns(rows)
}

// scanPipelineRuns decodes pipeline runs from a database row.
func scanPipelineRuns(rows *sql.Rows) ([]*PipelineRun, error) {
	var result []*PipelineRun
	for rows.Next() {
		var pr PipelineRun
		var finishedAt, summary, searchQueryNull, trashedAt, trashReason sql.NullString
		var execPlanID sql.NullInt64
		var attemptNum sql.NullInt64
		if err := rows.Scan(
			&pr.ID, &pr.Step, &pr.StartedAt, &finishedAt, &pr.Status, &summary, &searchQueryNull,
			&execPlanID, &attemptNum, &pr.VisibilityState, &trashedAt, &trashReason,
		); err != nil {
			lg.Debug("pipeline run scan failed", "scanned", len(result), "error", err)
			return nil, err
		}
		if finishedAt.Valid {
			pr.FinishedAt = &finishedAt.String
		}
		if summary.Valid {
			pr.Summary = &summary.String
		}
		if searchQueryNull.Valid {
			pr.SearchQuery = &searchQueryNull.String
		}
		if execPlanID.Valid {
			pr.ExecutionPlanID = &execPlanID.Int64
		}
		if attemptNum.Valid {
			n := int(attemptNum.Int64)
			pr.AttemptNumber = &n
		}
		if trashedAt.Valid {
			pr.TrashedAt = &trashedAt.String
		}
		if trashReason.Valid {
			pr.TrashReason = &trashReason.String
		}
		result = append(result, &pr)
	}
	if err := rows.Err(); err != nil {
		lg.Debug("pipeline run iteration failed", "scanned", len(result), "error", err)
		return nil, err
	}
	lg.Debug("pipeline run scan successful", "runs", len(result))
	return result, nil
}

// scanAll converts arbitrary SQL rows to maps keyed by column name.
func scanAll(rows *sql.Rows) ([]map[string]any, error) {
	cols, err := rows.Columns()
	if err != nil {
		lg.Debug("generic row column lookup failed", "error", err)
		return nil, err
	}
	var result []map[string]any
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			lg.Debug("generic row scan failed", "rows_scanned", len(result), "error", err)
			return nil, err
		}
		m := make(map[string]any, len(cols))
		for i, col := range cols {
			m[col] = vals[i]
		}
		result = append(result, m)
	}
	if err := rows.Err(); err != nil {
		lg.Debug("generic row iteration failed", "rows_scanned", len(result), "error", err)
		return nil, err
	}
	lg.Debug("generic row scan successful", "rows", len(result), "columns", len(cols))
	return result, nil
}

// isRetryableError returns true if the error is a transient SQLite error that
// can be retried: UNIQUE constraint violation or SQLITE_BUSY (database locked).
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// modernc.org/sqlite reports UNIQUE constraint violations as
	// "UNIQUE constraint failed: <table>.<column>"
	if strings.Contains(msg, "UNIQUE constraint failed") {
		return true
	}
	// modernc.org/sqlite reports database lock as "database is locked" or
	// "SQLITE_BUSY".
	if strings.Contains(msg, "database is locked") || strings.Contains(msg, "SQLITE_BUSY") {
		return true
	}
	return false
}

// withTx runs fn inside a transaction, rolling back on error and committing on success.
func (d *Database) withTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := d.DB.BeginTx(ctx, nil)
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
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// nullStrPtr returns nil if s is nil, otherwise returns a sql.NullString with the value.
func nullStrPtr(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}
