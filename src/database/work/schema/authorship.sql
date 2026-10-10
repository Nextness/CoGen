-- name: CountArticleAuthors :one
SELECT COUNT(*) FROM authorships
WHERE work_revision_id = sqlc.arg(work_revision_id);

-- name: ListArticleAuthors :many
SELECT
    a.id AS relation_id,
    ao.id,
    ao.person_id,
    ao.citation_name,
    ao.first_name,
    ao.last_name,
    ao.orcid,
    a.author_order,
    a.affiliation
FROM authorships a
JOIN author_occurrences ao ON ao.id = a.author_occurrence_id
WHERE a.work_revision_id = sqlc.arg(work_revision_id)
  AND a.id > sqlc.arg(cursor_id)
ORDER BY a.id
LIMIT sqlc.arg(row_limit);
