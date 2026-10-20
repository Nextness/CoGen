-- name: InsertRunArtifact :execresult
INSERT OR IGNORE INTO run_artifacts (pipeline_run_id, artifact_id, artifact_role)
VALUES (
    sqlc.arg(pipeline_run_id),
    sqlc.arg(artifact_id),
    sqlc.arg(artifact_role)
);

-- name: GetRunArtifactByRunAndRole :one
SELECT artifact_id
FROM run_artifacts
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id)
  AND artifact_role = sqlc.arg(artifact_role);
