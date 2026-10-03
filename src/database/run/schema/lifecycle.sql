-- name: FindRunningAttemptByPlan :one
SELECT id
FROM pipeline_runs
WHERE execution_plan_id = sqlc.arg(execution_plan_id)
  AND status = 'running'
ORDER BY attempt_number DESC
LIMIT 1;

-- name: NextAttemptNumber :one
SELECT CAST(COALESCE(MAX(attempt_number), 0) AS INTEGER) AS max_attempt_number
FROM pipeline_runs
WHERE execution_plan_id = sqlc.arg(execution_plan_id);

-- name: InsertAttempt :execresult
INSERT INTO pipeline_runs (
    step,
    started_at,
    status,
    search_query,
    execution_plan_id,
    attempt_number
) VALUES (
    sqlc.arg(step),
    sqlc.arg(started_at),
    'running',
    sqlc.arg(search_query),
    sqlc.arg(execution_plan_id),
    sqlc.arg(attempt_number)
);

-- name: FinishRun :exec
UPDATE pipeline_runs
SET
    finished_at = sqlc.arg(finished_at),
    status = sqlc.arg(status),
    summary = sqlc.arg(summary)
WHERE id = sqlc.arg(id);

-- name: TrashRun :exec
UPDATE pipeline_runs
SET
    visibility_state = 'trashed',
    trashed_at = sqlc.arg(trashed_at),
    trash_reason = sqlc.arg(trash_reason)
WHERE id = sqlc.arg(id);

-- name: RestoreRun :exec
UPDATE pipeline_runs
SET
    visibility_state = 'active',
    trashed_at = NULL,
    trash_reason = NULL
WHERE id = sqlc.arg(id);

-- name: RecoverRun :execresult
UPDATE pipeline_runs
SET
    status = 'failed',
    finished_at = sqlc.arg(finished_at),
    summary = 'abandoned attempt recovered by operator'
WHERE id = sqlc.arg(id)
  AND status = 'running';

-- name: FailRunningSteps :exec
UPDATE run_steps
SET
    step_status = 'failed',
    finished_at = sqlc.arg(finished_at)
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id)
  AND step_status IN ('pending', 'running');
