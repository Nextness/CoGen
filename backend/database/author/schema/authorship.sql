-- name: InsertAuthorship :execresult
INSERT INTO authorships (
    work_revision_id,
    author_occurrence_id,
    author_order,
    affiliation
) VALUES (
    sqlc.arg(work_revision_id),
    sqlc.arg(author_occurrence_id),
    sqlc.arg(author_order),
    sqlc.arg(affiliation)
);

-- name: ListAuthorshipsByRevisionID :many
SELECT
    id,
    work_revision_id,
    author_occurrence_id,
    author_order,
    affiliation,
    created_at
FROM authorships
WHERE work_revision_id = sqlc.arg(work_revision_id)
ORDER BY author_order;

-- name: ListAuthorshipsByOccurrenceID :many
SELECT
    id,
    work_revision_id,
    author_occurrence_id,
    author_order,
    affiliation,
    created_at
FROM authorships
WHERE author_occurrence_id = sqlc.arg(author_occurrence_id)
ORDER BY id;
