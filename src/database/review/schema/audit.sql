-- name: InsertReviewAuditEvent :exec
INSERT INTO audit_events (
    occurred_at,
    actor,
    pipeline_run_id,
    entity_type,
    entity_id,
    action,
    before_json,
    after_json,
    metadata_json
) VALUES (
    sqlc.arg(occurred_at),
    'reviewer',
    sqlc.arg(pipeline_run_id),
    sqlc.arg(entity_type),
    sqlc.arg(entity_id),
    sqlc.arg(action),
    sqlc.arg(before_json),
    sqlc.arg(after_json),
    sqlc.arg(metadata_json)
);
