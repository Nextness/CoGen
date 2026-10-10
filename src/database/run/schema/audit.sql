-- name: InsertRunRecoveryAudit :exec
INSERT INTO audit_events (
    occurred_at,
    actor,
    pipeline_run_id,
    entity_type,
    entity_id,
    action,
    metadata_json,
    correlation_id
) VALUES (
    sqlc.arg(occurred_at),
    'operator',
    sqlc.arg(pipeline_run_id),
    'pipeline_run',
    sqlc.arg(entity_id),
    sqlc.arg(action),
    '{"reason":"abandoned_attempt_recovery"}',
    sqlc.arg(correlation_id)
);

-- name: InsertRunVisibilityAudit :exec
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
    'local_user',
    sqlc.arg(pipeline_run_id),
    'pipeline_run',
    sqlc.arg(entity_id),
    sqlc.arg(action),
    sqlc.arg(before_json),
    sqlc.arg(after_json),
    sqlc.arg(metadata_json)
);
