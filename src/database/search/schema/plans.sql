-- name: InsertPlanIgnore :execresult
INSERT OR IGNORE INTO execution_plans (
    search_revision_id,
    execution_fingerprint,
    resolved_manifest_hash,
    input_manifest_hash,
    enrichment_enabled
) VALUES (
    sqlc.arg(search_revision_id),
    sqlc.arg(execution_fingerprint),
    sqlc.arg(resolved_manifest_hash),
    sqlc.arg(input_manifest_hash),
    sqlc.arg(enrichment_enabled)
);

-- name: GetPlanByID :one
SELECT
    id,
    search_revision_id,
    execution_fingerprint,
    resolved_manifest_hash,
    input_manifest_hash,
    enrichment_enabled,
    created_at
FROM execution_plans
WHERE id = sqlc.arg(id);

-- name: GetPlanByFingerprint :one
SELECT
    id,
    search_revision_id,
    execution_fingerprint,
    resolved_manifest_hash,
    input_manifest_hash,
    enrichment_enabled,
    created_at
FROM execution_plans
WHERE search_revision_id = sqlc.arg(search_revision_id)
  AND execution_fingerprint = sqlc.arg(execution_fingerprint);

-- name: ListPlansBySearchRevision :many
SELECT
    id,
    search_revision_id,
    execution_fingerprint,
    resolved_manifest_hash,
    input_manifest_hash,
    enrichment_enabled,
    created_at
FROM execution_plans
WHERE search_revision_id = sqlc.arg(search_revision_id)
ORDER BY id;
