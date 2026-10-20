-- name: InsertArtifact :execresult
INSERT OR IGNORE INTO artifacts (content_hash, byte_size, content_type)
VALUES (
    sqlc.arg(content_hash),
    sqlc.arg(byte_size),
    sqlc.arg(content_type)
);

-- name: GetArtifactByHash :one
SELECT
    id,
    content_hash,
    byte_size,
    content_type,
    created_at
FROM artifacts
WHERE content_hash = sqlc.arg(content_hash);

-- name: GetArtifactByID :one
SELECT
    id,
    content_hash,
    byte_size,
    content_type,
    created_at
FROM artifacts
WHERE id = sqlc.arg(id);

-- name: GetArtifactMetadataByHash :one
SELECT
    id,
    byte_size,
    content_type
FROM artifacts
WHERE content_hash = sqlc.arg(content_hash);

-- name: GetArtifactMetadataByID :one
SELECT
    byte_size,
    content_type
FROM artifacts
WHERE id = sqlc.arg(id);
