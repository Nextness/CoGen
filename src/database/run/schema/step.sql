-- name: InsertRunStep :execresult
INSERT INTO run_steps (
    pipeline_run_id,
    step_name,
    started_at
) VALUES (
    sqlc.arg(pipeline_run_id),
    sqlc.arg(step_name),
    sqlc.arg(started_at)
);

-- name: UpdateRunStepStatus :exec
UPDATE run_steps
SET
    step_status = sqlc.arg(step_status),
    finished_at = sqlc.arg(finished_at)
WHERE id = sqlc.arg(id);

-- name: LinkRunStepReuse :exec
UPDATE run_steps
SET
    reused_from_run_id = sqlc.arg(reused_from_run_id),
    step_status = 'reused',
    finished_at = sqlc.arg(finished_at)
WHERE id = sqlc.arg(id);

-- name: LinkRunStepInputArtifact :exec
UPDATE run_steps
SET input_artifact_id = sqlc.arg(input_artifact_id)
WHERE id = sqlc.arg(id);

-- name: LinkRunStepOutputArtifact :exec
UPDATE run_steps
SET output_artifact_id = sqlc.arg(output_artifact_id)
WHERE id = sqlc.arg(id);

-- name: SetRunStepFingerprints :exec
UPDATE run_steps
SET
    input_fingerprint = sqlc.arg(input_fingerprint),
    output_fingerprint = sqlc.arg(output_fingerprint)
WHERE id = sqlc.arg(id);

-- name: ListRunStepsByRun :many
SELECT
    id,
    pipeline_run_id,
    step_name,
    step_status,
    input_artifact_id,
    output_artifact_id,
    reused_from_run_id,
    input_fingerprint,
    output_fingerprint,
    started_at,
    finished_at
FROM run_steps
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id)
ORDER BY id;

-- name: ListRunStepsForViewer :many
SELECT
    step_name,
    step_status,
    input_artifact_id,
    output_artifact_id,
    started_at,
    finished_at,
    input_fingerprint,
    output_fingerprint,
    CASE WHEN started_at IS NOT NULL AND finished_at IS NOT NULL
        THEN ROUND((julianday(finished_at)-julianday(started_at))*86400, 3)
        ELSE NULL END AS duration_seconds
FROM run_steps
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id)
ORDER BY id;
