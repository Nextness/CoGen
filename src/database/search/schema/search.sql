-- name: InsertSearchIgnore :execresult
INSERT OR IGNORE INTO searches (search_id)
VALUES (sqlc.arg(search_id));

-- name: GetSearchIDBySearchID :one
SELECT id
FROM searches
WHERE search_id = sqlc.arg(search_id);

-- name: GetSearchByID :one
SELECT
    id,
    search_id,
    created_at
FROM searches
WHERE id = sqlc.arg(id);

-- name: GetSearchBySearchID :one
SELECT
    id,
    search_id,
    created_at
FROM searches
WHERE search_id = sqlc.arg(search_id);

-- name: ListSearches :many
SELECT
    id,
    search_id,
    created_at
FROM searches
ORDER BY id;

-- name: ListLegacySearches :many
WITH selected_searches AS (
    SELECT id, search_id, created_at
    FROM searches
    ORDER BY id DESC
    LIMIT sqlc.arg(limit)
)
SELECT
    s.id,
    s.search_id,
    s.created_at,
    sr.id AS revision_id,
    sr.revision_label,
    sr.config_artifact_hash,
    sr.resolved_manifest_hash,
    sr.created_at AS revision_created_at
FROM selected_searches s
LEFT JOIN search_revisions sr ON sr.search_id=s.id
    AND (SELECT COUNT(*) FROM search_revisions newer
        WHERE newer.search_id=s.id AND newer.id>sr.id) < sqlc.arg(revision_limit)
ORDER BY s.id DESC, sr.id DESC;
