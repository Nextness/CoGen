-- name: UpsertRunWorkStage :exec
INSERT INTO run_work_stages (
    pipeline_run_id,
    work_id,
    stage_name,
    outcome,
    reason,
    updated_at
) VALUES (
    sqlc.arg(pipeline_run_id),
    sqlc.arg(work_id),
    sqlc.arg(stage_name),
    sqlc.arg(outcome),
    sqlc.arg(reason),
    datetime('now')
)
ON CONFLICT (pipeline_run_id, work_id, stage_name)
DO UPDATE SET
    outcome    = excluded.outcome,
    reason     = excluded.reason,
    updated_at = datetime('now');

-- name: GetRunWorkStageByRunWorkStage :one
SELECT
    id,
    pipeline_run_id,
    work_id,
    stage_name,
    outcome,
    reason,
    created_at,
    updated_at
FROM run_work_stages
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id)
  AND work_id = sqlc.arg(work_id)
  AND stage_name = sqlc.arg(stage_name);

-- name: ListRunWorkStagesByRunID :many
SELECT
    id,
    pipeline_run_id,
    work_id,
    stage_name,
    outcome,
    reason,
    created_at,
    updated_at
FROM run_work_stages
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id)
ORDER BY id;

-- name: ListRunWorkStagesByWorkID :many
SELECT
    id,
    pipeline_run_id,
    work_id,
    stage_name,
    outcome,
    reason,
    created_at,
    updated_at
FROM run_work_stages
WHERE work_id = sqlc.arg(work_id)
ORDER BY id;

-- name: CountRunWorkStagesByStageAndOutcome :one
SELECT COUNT(*) FROM run_work_stages
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id)
  AND stage_name = sqlc.arg(stage_name)
  AND outcome = sqlc.arg(outcome);
