-- name: GetArticleRevision :one
SELECT
    wr.id,
    wr.work_id,
    wr.pipeline_run_id,
    wr.field_schema_version,
    wr.payload_hash,
    wr.title,
    wr.abstract,
    wr.year,
    wr.journal,
    wr.publisher,
    wr.source,
    wr.keywords,
    wr.keywords_plus,
    wr.citation_count,
    wr.reference_count,
    wr.extension_data,
    wr.producer_stage,
    wr.created_at,
    w.doi
FROM work_revisions wr
JOIN works w ON w.id = wr.work_id
WHERE wr.id = sqlc.arg(id)
  AND wr.pipeline_run_id = sqlc.arg(pipeline_run_id)
  AND (wr.producer_stage != 'normalize' OR (
      wr.producer_stage = 'normalize'
      AND wr.id = (SELECT MAX(normalized_candidate.id) FROM work_revisions normalized_candidate
          WHERE normalized_candidate.pipeline_run_id = wr.pipeline_run_id
            AND normalized_candidate.work_id = wr.work_id
            AND normalized_candidate.producer_stage = 'normalize')
      AND EXISTS (SELECT 1 FROM run_work_stages current_validation
          WHERE current_validation.pipeline_run_id = wr.pipeline_run_id
            AND current_validation.work_id = wr.work_id
            AND current_validation.stage_name = 'validate'
            AND current_validation.outcome = 'valid')));

-- name: GetArticleDetailWorkID :one
SELECT wr.work_id
FROM work_revisions wr
WHERE wr.id = sqlc.arg(id)
  AND wr.pipeline_run_id = sqlc.arg(pipeline_run_id)
  AND (wr.producer_stage != 'normalize' OR (
      wr.producer_stage = 'normalize'
      AND wr.id = (SELECT MAX(normalized_candidate.id) FROM work_revisions normalized_candidate
          WHERE normalized_candidate.pipeline_run_id = wr.pipeline_run_id
            AND normalized_candidate.work_id = wr.work_id
            AND normalized_candidate.producer_stage = 'normalize')
      AND EXISTS (SELECT 1 FROM run_work_stages current_validation
          WHERE current_validation.pipeline_run_id = wr.pipeline_run_id
            AND current_validation.work_id = wr.work_id
            AND current_validation.stage_name = 'validate'
            AND current_validation.outcome = 'valid')));

-- name: CountArticleReferences :one
SELECT COUNT(*) FROM reference_mentions
WHERE work_revision_id = sqlc.arg(work_revision_id);

-- name: ListArticleReferences :many
SELECT
    rm.id,
    rm.work_revision_id,
    rm.resolved_work_id,
    rm.mention_order,
    rm.doi,
    rm.title,
    rm.author,
    rm.year,
    rm.source,
    rm.created_at,
    target.id AS resolved_revision_id,
    target.title AS resolved_title
FROM reference_mentions rm
JOIN work_revisions source ON source.id = rm.work_revision_id
LEFT JOIN work_revisions target ON target.id = (
    SELECT candidate.id FROM work_revisions candidate
    WHERE candidate.work_id = rm.resolved_work_id
      AND candidate.pipeline_run_id = source.pipeline_run_id
      AND candidate.producer_stage = 'normalize'
      AND candidate.id = (SELECT MAX(normalized_candidate.id) FROM work_revisions normalized_candidate
          WHERE normalized_candidate.pipeline_run_id = candidate.pipeline_run_id
            AND normalized_candidate.work_id = candidate.work_id
            AND normalized_candidate.producer_stage = 'normalize')
      AND EXISTS (SELECT 1 FROM run_work_stages current_validation
          WHERE current_validation.pipeline_run_id = candidate.pipeline_run_id
            AND current_validation.work_id = candidate.work_id
            AND current_validation.stage_name = 'validate'
            AND current_validation.outcome = 'valid')
    LIMIT 1)
WHERE rm.work_revision_id = sqlc.arg(work_revision_id)
  AND rm.id > sqlc.arg(cursor_id)
ORDER BY rm.id
LIMIT sqlc.arg(row_limit);

-- name: CountArticleStages :one
SELECT COUNT(*) FROM run_work_stages
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id)
  AND work_id = sqlc.arg(work_id);

-- name: ListArticleStages :many
SELECT
    id,
    pipeline_run_id,
    work_id,
    stage_name,
    outcome,
    reason,
    created_at,
    updated_at
FROM run_work_stages
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id)
  AND work_id = sqlc.arg(work_id)
  AND id > sqlc.arg(cursor_id)
ORDER BY id
LIMIT sqlc.arg(row_limit);

-- name: GetReferenceDetail :one
SELECT
    rm.id,
    rm.work_revision_id,
    rm.resolved_work_id,
    rm.mention_order,
    rm.raw_reference,
    rm.doi,
    rm.title,
    rm.author,
    rm.year,
    rm.source,
    rm.created_at,
    wr.work_id,
    wr.title AS citing_title,
    wr.pipeline_run_id,
    target.id AS resolved_revision_id,
    target.title AS resolved_title
FROM reference_mentions rm
JOIN work_revisions wr ON wr.id = rm.work_revision_id
LEFT JOIN work_revisions target ON target.id = (
    SELECT candidate.id FROM work_revisions candidate
    WHERE candidate.work_id = rm.resolved_work_id
      AND candidate.pipeline_run_id = wr.pipeline_run_id
      AND candidate.producer_stage = 'normalize'
      AND candidate.id = (SELECT MAX(normalized_candidate.id) FROM work_revisions normalized_candidate
          WHERE normalized_candidate.pipeline_run_id = candidate.pipeline_run_id
            AND normalized_candidate.work_id = candidate.work_id
            AND normalized_candidate.producer_stage = 'normalize')
      AND EXISTS (SELECT 1 FROM run_work_stages current_validation
          WHERE current_validation.pipeline_run_id = candidate.pipeline_run_id
            AND current_validation.work_id = candidate.work_id
            AND current_validation.stage_name = 'validate'
            AND current_validation.outcome = 'valid')
    LIMIT 1)
WHERE rm.id = sqlc.arg(id)
  AND wr.pipeline_run_id = sqlc.arg(pipeline_run_id);
