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
