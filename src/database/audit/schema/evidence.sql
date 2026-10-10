-- name: GetAuditEventRecordedData :one
SELECT
    event.id,
    event.before_json,
    event.after_json,
    event.metadata_json
FROM audit_events event
WHERE event.id = sqlc.arg(id)
  AND (
    event.pipeline_run_id = sqlc.arg(pipeline_run_id)
    OR (
        event.pipeline_run_id IS NULL
        AND event.action LIKE 'pdf_%'
        AND event.entity_type = 'work'
        AND EXISTS (
            SELECT 1 FROM work_revisions scoped_revision
            WHERE scoped_revision.pipeline_run_id = sqlc.arg(pipeline_run_id)
              AND CAST(scoped_revision.work_id AS TEXT) = event.entity_id
        )
    )
  );

-- name: ListEnrichmentSummary :many
SELECT DISTINCT
    CASE WHEN json_valid(event.metadata_json) THEN COALESCE(json_extract(event.metadata_json, '$.provider'), '') ELSE '' END AS provider,
    CASE WHEN json_valid(event.metadata_json) THEN COALESCE(json_extract(event.metadata_json, '$.field'), '') ELSE '' END AS field
FROM audit_events event
WHERE event.entity_type = 'work_revision'
  AND event.action = 'field_enriched'
  AND event.entity_id IN (
      SELECT CAST(revision.id AS TEXT) FROM work_revisions revision
      WHERE revision.work_id = sqlc.arg(work_id)
        AND revision.pipeline_run_id = sqlc.arg(pipeline_run_id)
  )
ORDER BY provider, field
LIMIT 101;

-- name: CountAuthorDetailEvents :one
SELECT COUNT(*)
FROM audit_events
WHERE entity_type='author_occurrence'
  AND entity_id = sqlc.arg('entity_id')
  AND pipeline_run_id = sqlc.arg('pipeline_run_id');

-- name: ListAuthorDetailEvents :many
SELECT
    id,
    occurred_at,
    actor,
    pipeline_run_id,
    entity_type,
    entity_id,
    action,
    before_json,
    after_json,
    metadata_json,
    correlation_id
FROM audit_events
WHERE entity_type='author_occurrence'
  AND entity_id = sqlc.arg('entity_id')
  AND pipeline_run_id = sqlc.arg('pipeline_run_id')
  AND (CAST(sqlc.arg('cursor_id') AS INTEGER)=0 OR id < sqlc.arg('cursor_id'))
ORDER BY id DESC
LIMIT sqlc.arg('limit');
