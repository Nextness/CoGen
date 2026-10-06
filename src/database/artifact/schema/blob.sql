-- name: InsertArtifactBlob :execresult
INSERT OR IGNORE INTO artifact_blobs (artifact_id, pipeline_run_id, data)
VALUES (
    sqlc.arg(artifact_id),
    sqlc.arg(pipeline_run_id),
    sqlc.arg(data)
);

-- name: GetArtifactBlobByArtifactID :one
SELECT
    id,
    artifact_id,
    pipeline_run_id,
    data,
    created_at
FROM artifact_blobs
WHERE artifact_id = sqlc.arg(artifact_id);

-- name: GetArtifactBlobDataByArtifactID :one
SELECT data
FROM artifact_blobs
WHERE artifact_id = sqlc.arg(artifact_id);
