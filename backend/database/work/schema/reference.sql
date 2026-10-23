-- name: InsertReferenceMention :execresult
INSERT INTO reference_mentions (
    work_revision_id,
    resolved_work_id,
    mention_order,
    raw_reference,
    doi,
    title,
    author,
    year,
    source
) VALUES (
    sqlc.arg(work_revision_id),
    sqlc.arg(resolved_work_id),
    sqlc.arg(mention_order),
    sqlc.arg(raw_reference),
    sqlc.arg(doi),
    sqlc.arg(title),
    sqlc.arg(author),
    sqlc.arg(year),
    sqlc.arg(source)
);

-- name: GetReferenceMentionByID :one
SELECT
    id,
    work_revision_id,
    resolved_work_id,
    mention_order,
    raw_reference,
    doi,
    title,
    author,
    year,
    source,
    created_at
FROM reference_mentions
WHERE id = sqlc.arg(id);

-- name: ListReferenceMentionsByRevisionID :many
SELECT
    id,
    work_revision_id,
    resolved_work_id,
    mention_order,
    raw_reference,
    doi,
    title,
    author,
    year,
    source,
    created_at
FROM reference_mentions
WHERE work_revision_id = sqlc.arg(work_revision_id)
ORDER BY mention_order, id;

-- name: ListReferenceMentionsByResolvedWorkID :many
SELECT
    id,
    work_revision_id,
    resolved_work_id,
    mention_order,
    raw_reference,
    doi,
    title,
    author,
    year,
    source,
    created_at
FROM reference_mentions
WHERE resolved_work_id = sqlc.arg(resolved_work_id)
ORDER BY id;
