-- name: InsertPersonByORCID :execresult
INSERT OR IGNORE INTO people (orcid)
VALUES (sqlc.arg(orcid));

-- name: GetPersonByID :one
SELECT
    id,
    orcid,
    created_at
FROM people
WHERE id = sqlc.arg(id);

-- name: GetPersonByORCID :one
SELECT
    id,
    orcid,
    created_at
FROM people
WHERE orcid = sqlc.arg(orcid);
