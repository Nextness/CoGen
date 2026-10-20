-- name: InsertWorkByDOI :execresult
INSERT OR IGNORE INTO works (doi)
VALUES (sqlc.arg(doi));

-- name: InsertWorkWithoutDOI :execresult
INSERT INTO works (doi)
VALUES (NULL);

-- name: GetWorkByID :one
SELECT
    id,
    doi,
    created_at
FROM works
WHERE id = sqlc.arg(id);

-- name: GetWorkByDOI :one
SELECT
    id,
    doi,
    created_at
FROM works
WHERE doi = sqlc.arg(doi);

-- name: ListWorksByIDs :many
SELECT
    id,
    doi,
    created_at
FROM works
WHERE id IN (sqlc.slice(ids))
ORDER BY id;

-- name: CountWorks :one
SELECT COUNT(*) FROM works;

-- name: InsertWorkIdentifier :execresult
INSERT OR IGNORE INTO work_identifiers (work_id, namespace, identifier)
VALUES (
    sqlc.arg(work_id),
    sqlc.arg(namespace),
    sqlc.arg(identifier)
);

-- name: GetWorkIdentifierByID :one
SELECT
    id,
    work_id,
    namespace,
    identifier,
    created_at
FROM work_identifiers
WHERE id = sqlc.arg(id);

-- name: GetWorkIdentifierByNamespaceAndIdentifier :one
SELECT
    id,
    work_id,
    namespace,
    identifier,
    created_at
FROM work_identifiers
WHERE namespace = sqlc.arg(namespace)
  AND identifier = sqlc.arg(identifier);

-- name: ListWorkIdentifiersByWorkID :many
SELECT
    id,
    work_id,
    namespace,
    identifier,
    created_at
FROM work_identifiers
WHERE work_id = sqlc.arg(work_id)
ORDER BY id;

-- name: CountWorkIdentifiersByWorkID :one
SELECT COUNT(*) FROM work_identifiers WHERE work_id = sqlc.arg(work_id);
