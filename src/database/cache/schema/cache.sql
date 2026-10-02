-- name: InsertCacheEntry :execresult
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

-- name: LatestCacheEntry :one
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
  AND request_fingerprint = sqlc.arg(request_fingerprint)
  AND extractor_version = sqlc.arg(extractor_version)
ORDER BY id DESC
LIMIT 1;

-- name: GlobalCacheEntry :one
SELECT
    ce.id,
    ce.provider,
    ce.namespace,
    ce.request_fingerprint,
    ce.response_status,
    ce.payload_artifact_id,
    ce.fetched_at,
    ce.expires_at,
    ce.extractor_version,
    ce.created_at,
    ce.updated_at
FROM cache_entries ce
JOIN run_cache_uses rcu ON rcu.cache_entry_id = ce.id
WHERE rcu.cache_layer = 'global'
  AND ce.provider = sqlc.arg(provider)
  AND ce.namespace = sqlc.arg(namespace)
  AND ce.request_fingerprint = sqlc.arg(request_fingerprint)
  AND ce.extractor_version = sqlc.arg(extractor_version)
ORDER BY ce.id DESC, rcu.id DESC
LIMIT 1;
