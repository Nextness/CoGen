-- name: InsertAuthorOccurrence :execresult
INSERT INTO author_occurrences (
    person_id,
    citation_name,
    first_name,
    last_name,
    orcid
) VALUES (
    sqlc.arg(person_id),
    sqlc.arg(citation_name),
    sqlc.arg(first_name),
    sqlc.arg(last_name),
    sqlc.arg(orcid)
);

-- name: GetAuthorOccurrenceByID :one
SELECT
    id,
    person_id,
    citation_name,
    first_name,
    last_name,
    orcid,
    created_at
FROM author_occurrences
WHERE id = sqlc.arg(id);

-- name: ListAuthorOccurrencesByPersonID :many
SELECT
    id,
    person_id,
    citation_name,
    first_name,
    last_name,
    orcid,
    created_at
FROM author_occurrences
WHERE person_id = sqlc.arg(person_id)
ORDER BY id;
