-- name: InsertProbeEntry :execresult
INSERT INTO cache_entries (
    provider,
    namespace,
    request_fingerprint,
    response_status,
    payload_artifact_id,
    fetched_at,
    expires_at,
    extractor_version
) VALUES (
    sqlc.arg(provider),
    sqlc.arg(namespace),
    sqlc.arg(request_fingerprint),
    sqlc.arg(response_status),
    sqlc.arg(payload_artifact_id),
    sqlc.arg(fetched_at),
    sqlc.arg(expires_at),
    sqlc.arg(extractor_version)
);

-- name: LatestProbeEntry :one
SELECT
    id,
    provider,
    namespace,
    request_fingerprint,
    response_status,
    payload_artifact_id,
    fetched_at,
    expires_at,
    extractor_version,
    created_at,
    updated_at
FROM cache_entries
WHERE provider = sqlc.arg(provider)
  AND namespace = sqlc.arg(namespace)
ORDER BY id DESC
LIMIT 1;

-- name: ListProbeEntries :many
SELECT
    id,
    provider,
    namespace,
    request_fingerprint,
    response_status,
    payload_artifact_id,
    fetched_at,
    expires_at,
    extractor_version,
    created_at,
    updated_at
FROM cache_entries
WHERE provider = sqlc.arg(provider)
ORDER BY id;
