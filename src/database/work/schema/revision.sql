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

-- name: ListNormalizeRevisionsByRunID :many
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
  AND producer_stage = 'normalize'
ORDER BY id;

-- name: CountWorkRevisionsByWorkID :one
SELECT COUNT(*) FROM work_revisions WHERE work_id = sqlc.arg(work_id);

-- name: GetCurrentNormalizedRevisionWorkID :one
SELECT wr.work_id FROM work_revisions wr
WHERE wr.id = sqlc.arg(id)
  AND wr.pipeline_run_id = sqlc.arg(pipeline_run_id)
  AND wr.producer_stage = 'normalize'
  AND wr.id = (SELECT MAX(normalized_candidate.id) FROM work_revisions normalized_candidate
      WHERE normalized_candidate.pipeline_run_id = wr.pipeline_run_id
        AND normalized_candidate.work_id = wr.work_id
        AND normalized_candidate.producer_stage = 'normalize')
  AND EXISTS (SELECT 1 FROM run_work_stages current_validation
      WHERE current_validation.pipeline_run_id = wr.pipeline_run_id
        AND current_validation.work_id = wr.work_id
        AND current_validation.stage_name = 'validate'
        AND current_validation.outcome = 'valid');

-- name: CountCurrentNormalizedRevisionsByWork :one
SELECT COUNT(*) FROM work_revisions wr
WHERE wr.pipeline_run_id = sqlc.arg(pipeline_run_id)
  AND wr.work_id = sqlc.arg(work_id)
  AND wr.producer_stage = 'normalize'
  AND wr.id = (SELECT MAX(normalized_candidate.id) FROM work_revisions normalized_candidate
      WHERE normalized_candidate.pipeline_run_id = wr.pipeline_run_id
        AND normalized_candidate.work_id = wr.work_id
        AND normalized_candidate.producer_stage = 'normalize')
  AND EXISTS (SELECT 1 FROM run_work_stages current_validation
      WHERE current_validation.pipeline_run_id = wr.pipeline_run_id
        AND current_validation.work_id = wr.work_id
        AND current_validation.stage_name = 'validate'
        AND current_validation.outcome = 'valid');
