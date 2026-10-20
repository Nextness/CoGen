-- name: InsertAuditEvent :execresult
INSERT INTO audit_events (
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
) VALUES (
    sqlc.arg(occurred_at),
    sqlc.arg(actor),
    sqlc.arg(pipeline_run_id),
    sqlc.arg(entity_type),
    sqlc.arg(entity_id),
    sqlc.arg(action),
    sqlc.arg(before_json),
    sqlc.arg(after_json),
    sqlc.arg(metadata_json),
    sqlc.arg(correlation_id)
);

-- name: ListAuditEventsByRun :many
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
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id)
ORDER BY id;

-- name: ListAuditEventsByEntity :many
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
WHERE entity_type = sqlc.arg(entity_type)
  AND entity_id = sqlc.arg(entity_id)
ORDER BY id;

-- name: GetAuditEventOccurredAt :one
SELECT occurred_at
FROM audit_events
WHERE id = sqlc.arg(id);
