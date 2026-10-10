-- name: InsertRevisionIgnore :execresult
INSERT OR IGNORE INTO search_revisions (
    search_id,
    revision_label,
    config_artifact_hash,
    resolved_manifest_hash
) VALUES (
    sqlc.arg(search_id),
    sqlc.arg(revision_label),
    sqlc.arg(config_artifact_hash),
    sqlc.arg(resolved_manifest_hash)
);

-- name: UpdateRevisionHashes :exec
UPDATE search_revisions
SET
    config_artifact_hash = sqlc.arg(config_artifact_hash),
    resolved_manifest_hash = sqlc.arg(resolved_manifest_hash),
    updated_at = datetime('now')
WHERE id = sqlc.arg(id);

-- name: GetRevisionByID :one
SELECT
    id,
    search_id,
    revision_label,
    config_artifact_hash,
    resolved_manifest_hash,
    created_at,
    updated_at
FROM search_revisions
WHERE id = sqlc.arg(id);

-- name: GetRevisionBySearchAndLabel :one
SELECT
    id,
    search_id,
    revision_label,
    config_artifact_hash,
    resolved_manifest_hash,
    created_at,
    updated_at
FROM search_revisions
WHERE search_id = sqlc.arg(search_id)
  AND revision_label = sqlc.arg(revision_label);

-- name: ListRevisionsBySearch :many
SELECT
    id,
    search_id,
    revision_label,
    config_artifact_hash,
    resolved_manifest_hash,
    created_at,
    updated_at
FROM search_revisions
WHERE search_id = sqlc.arg(search_id)
ORDER BY id;
