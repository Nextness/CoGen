-- name: CountSharedRunArtifacts :one
SELECT COUNT(DISTINCT other.artifact_id)
FROM (
    SELECT rs.input_artifact_id AS artifact_id
    FROM run_steps rs
    WHERE rs.pipeline_run_id != sqlc.arg(pipeline_run_id)
    UNION ALL
    SELECT rs.output_artifact_id AS artifact_id
    FROM run_steps rs
    WHERE rs.pipeline_run_id != sqlc.arg(pipeline_run_id)
) other
WHERE other.artifact_id IN (
    SELECT rs.output_artifact_id
    FROM run_steps rs
    WHERE rs.pipeline_run_id = sqlc.arg(pipeline_run_id)
);

-- name: CountRunsReusingRun :one
SELECT COUNT(DISTINCT pipeline_run_id)
FROM run_steps
WHERE reused_from_run_id = sqlc.arg(reused_from_run_id)
  AND pipeline_run_id != sqlc.arg(pipeline_run_id);

-- name: CountOwnedReviewContexts :one
SELECT COUNT(*)
FROM review_contexts
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id);

-- name: CountDependentReviewContexts :one
WITH RECURSIVE descendants(id) AS (
    SELECT child.id
    FROM review_contexts root
    JOIN review_contexts child ON child.parent_context_id = root.id
    WHERE root.pipeline_run_id = sqlc.arg(pipeline_run_id)
    UNION ALL
    SELECT child.id
    FROM review_contexts child
    JOIN descendants parent ON child.parent_context_id = parent.id
)
SELECT COUNT(*) FROM descendants;

-- name: GetLatestRunWithAncestry :one
SELECT
    pr.id,
    pr.attempt_number,
    pr.started_at,
    pr.finished_at,
    pr.status,
    pr.visibility_state,
    s.id AS search_id,
    s.search_id AS search_name,
    sr.id AS search_revision_id,
    sr.revision_label,
    ep.id AS execution_plan_id
FROM pipeline_runs pr
LEFT JOIN execution_plans ep ON ep.id=pr.execution_plan_id
LEFT JOIN search_revisions sr ON sr.id=ep.search_revision_id
LEFT JOIN searches s ON s.id=sr.search_id
ORDER BY pr.id DESC
LIMIT 1;

-- name: GetSelectedAttempt :one
SELECT
    id,
    attempt_number,
    started_at,
    status,
    visibility_state
FROM pipeline_runs
WHERE id = sqlc.arg(id)
  AND execution_plan_id = sqlc.arg(execution_plan_id)
  AND visibility_state != 'trashed';

-- name: GetRunArtifactContext :one
SELECT
    s.search_id,
    sr.id AS search_revision_id,
    sr.revision_label,
    ep.id AS execution_plan_id,
    ep.execution_fingerprint,
    pr.id AS run_id,
    pr.attempt_number
FROM pipeline_runs pr
JOIN execution_plans ep ON ep.id=pr.execution_plan_id
JOIN search_revisions sr ON sr.id=ep.search_revision_id
JOIN searches s ON s.id=sr.search_id
WHERE pr.id = sqlc.arg(id);
