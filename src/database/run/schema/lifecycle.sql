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

-- name: GetRunVisibility :one
SELECT
    status,
    visibility_state
FROM pipeline_runs
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

-- name: GetRunContext :one
SELECT
    s.id AS search_pk,
    s.search_id AS search_name,
    s.created_at AS search_created_at,
    sr.id AS revision_pk,
    sr.search_id AS revision_search_id,
    sr.revision_label,
    sr.config_artifact_hash,
    sr.resolved_manifest_hash,
    sr.created_at AS revision_created_at,
    ep.id AS plan_pk,
    ep.search_revision_id AS plan_search_revision_id,
    ep.execution_fingerprint,
    ep.resolved_manifest_hash AS plan_resolved_manifest_hash,
    ep.input_manifest_hash,
    ep.enrichment_enabled,
    ep.created_at AS plan_created_at,
    pr.id AS run_pk,
    pr.execution_plan_id AS run_execution_plan_id,
    pr.step,
    pr.started_at,
    pr.finished_at,
    pr.status,
    pr.summary,
    pr.attempt_number,
    pr.visibility_state,
    pr.trashed_at,
    pr.trash_reason,
    rc.id AS review_context_id
FROM pipeline_runs pr
JOIN execution_plans ep ON ep.id=pr.execution_plan_id
JOIN search_revisions sr ON sr.id=ep.search_revision_id
JOIN searches s ON s.id=sr.search_id
LEFT JOIN review_contexts rc ON rc.pipeline_run_id=pr.id
WHERE pr.id = sqlc.arg(id);
