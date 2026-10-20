-- name: InsertRunCacheUse :execresult
INSERT INTO run_cache_uses (
    pipeline_run_id,
    cache_entry_id,
    cache_layer,
    outcome
) VALUES (
    sqlc.arg(pipeline_run_id),
    sqlc.arg(cache_entry_id),
    sqlc.arg(cache_layer),
    sqlc.arg(outcome)
);

-- name: ListRunCacheUsesByRun :many
SELECT
    id,
    pipeline_run_id,
    cache_entry_id,
    cache_layer,
    outcome,
    used_at
FROM run_cache_uses
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id)
ORDER BY id;

-- name: FindRunCacheEntry :one
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
FROM run_cache_uses rcu
JOIN cache_entries ce ON ce.id = rcu.cache_entry_id
WHERE rcu.pipeline_run_id = sqlc.arg(pipeline_run_id)
  AND rcu.cache_layer = sqlc.arg(cache_layer)
  AND ce.provider = sqlc.arg(provider)
  AND ce.namespace = sqlc.arg(namespace)
  AND ce.request_fingerprint = sqlc.arg(request_fingerprint)
  AND ce.extractor_version = sqlc.arg(extractor_version)
ORDER BY rcu.id DESC
LIMIT 1;

-- name: FindAnyRunCacheEntry :one
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
FROM run_cache_uses rcu
JOIN cache_entries ce ON ce.id = rcu.cache_entry_id
WHERE rcu.pipeline_run_id = sqlc.arg(pipeline_run_id)
  AND ce.provider = sqlc.arg(provider)
  AND ce.namespace = sqlc.arg(namespace)
  AND ce.request_fingerprint = sqlc.arg(request_fingerprint)
  AND (
        ce.extractor_version = sqlc.arg(extractor_version)
        OR ce.extractor_version LIKE sqlc.arg(extractor_version_pattern)
    )
ORDER BY rcu.id DESC
LIMIT 1;
