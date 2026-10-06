-- name: InsertWorkRevision :execresult
INSERT INTO work_revisions (
    work_id,
    pipeline_run_id,
    producer_stage,
    field_schema_version,
    payload_hash,
    title,
    abstract,
    year,
    journal,
    publisher,
    source,
    keywords,
    keywords_plus,
    citation_count,
    reference_count,
    extension_data
) VALUES (
    sqlc.arg(work_id),
    sqlc.arg(pipeline_run_id),
    sqlc.arg(producer_stage),
    sqlc.arg(field_schema_version),
    sqlc.arg(payload_hash),
    sqlc.arg(title),
    sqlc.arg(abstract),
    sqlc.arg(year),
    sqlc.arg(journal),
    sqlc.arg(publisher),
    sqlc.arg(source),
    sqlc.arg(keywords),
    sqlc.arg(keywords_plus),
    sqlc.arg(citation_count),
    sqlc.arg(reference_count),
    sqlc.arg(extension_data)
);

-- name: GetWorkRevisionByID :one
SELECT
    id,
    work_id,
    pipeline_run_id,
    field_schema_version,
    payload_hash,
    title,
    abstract,
    year,
    journal,
    publisher,
    source,
    keywords,
    keywords_plus,
    citation_count,
    reference_count,
    extension_data,
    producer_stage,
    created_at
FROM work_revisions
WHERE id = sqlc.arg(id);

-- name: ListWorkRevisionsByWorkID :many
SELECT
    id,
    work_id,
    pipeline_run_id,
    field_schema_version,
    payload_hash,
    title,
    abstract,
    year,
    journal,
    publisher,
    source,
    keywords,
    keywords_plus,
    citation_count,
    reference_count,
    extension_data,
    producer_stage,
    created_at
FROM work_revisions
WHERE work_id = sqlc.arg(work_id)
ORDER BY id;

-- name: ListWorkRevisionsByRunID :many
SELECT
    id,
    work_id,
    pipeline_run_id,
    field_schema_version,
    payload_hash,
    title,
    abstract,
    year,
    journal,
    publisher,
    source,
    keywords,
    keywords_plus,
    citation_count,
    reference_count,
    extension_data,
    producer_stage,
    created_at
FROM work_revisions
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id)
ORDER BY id;

-- name: CountWorkRevisionsByWorkID :one
SELECT COUNT(*) FROM work_revisions WHERE work_id = sqlc.arg(work_id);
