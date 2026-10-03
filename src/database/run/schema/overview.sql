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
