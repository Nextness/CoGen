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

-- name: GetArtifactContent :one
SELECT
    a.content_type,
    CAST(COALESCE((
        SELECT artifact_role FROM run_artifacts
        WHERE artifact_id = a.id
        ORDER BY pipeline_run_id, artifact_role
        LIMIT 1
    ), '') AS TEXT) AS artifact_role,
    ab.data
FROM artifacts a
LEFT JOIN artifact_blobs ab ON ab.artifact_id = a.id
WHERE a.id = sqlc.arg(id);

-- name: GetArtifactPreview :one
SELECT
    a.content_type,
    a.byte_size,
    CASE WHEN ab.id IS NULL THEN 0 ELSE 1 END AS has_blob,
    CAST(COALESCE(length(CAST(ab.data AS BLOB)), 0) AS INTEGER) AS blob_size,
    CAST(substr(CAST(ab.data AS BLOB), 1, sqlc.arg(preview_bytes)) AS BLOB) AS data
FROM artifacts a
LEFT JOIN artifact_blobs ab ON ab.artifact_id = a.id
WHERE a.id = sqlc.arg(id);
