-- name: InsertRun :execresult
INSERT INTO pipeline_runs (
    step,
    started_at,
    status,
    search_query
) VALUES (
    sqlc.arg(step),
    sqlc.arg(started_at),
    'running',
    sqlc.arg(search_query)
);

-- name: GetRunByID :one
SELECT
    id,
    step,
    started_at,
    finished_at,
    status,
    summary,
    search_query,
    execution_plan_id,
    attempt_number,
    visibility_state,
    trashed_at,
    trash_reason
FROM pipeline_runs
WHERE id = sqlc.arg(id);

-- name: ListRunsByPlan :many
SELECT
    id,
    step,
    started_at,
    finished_at,
    status,
    summary,
    search_query,
    execution_plan_id,
    attempt_number,
    visibility_state,
    trashed_at,
    trash_reason
FROM pipeline_runs
WHERE execution_plan_id = sqlc.arg(execution_plan_id)
ORDER BY attempt_number;

-- name: ListRunsByVisibility :many
SELECT
    id,
    step,
    started_at,
    finished_at,
    status,
    summary,
    search_query,
    execution_plan_id,
    attempt_number,
    visibility_state,
    trashed_at,
    trash_reason
FROM pipeline_runs
WHERE visibility_state = sqlc.arg(visibility_state)
ORDER BY id;

-- name: RunExists :one
SELECT 1
FROM pipeline_runs
WHERE id = sqlc.arg(id);

-- name: RunHasExecutionPlan :one
SELECT execution_plan_id IS NOT NULL AS has_execution_plan
FROM pipeline_runs
WHERE id = sqlc.arg(id);
