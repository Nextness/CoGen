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
