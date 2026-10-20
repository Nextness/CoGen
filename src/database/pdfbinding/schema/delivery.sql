-- Delivered-event link queries. The metadata audit row and its delivery link
-- commit atomically in the caller's metadata transaction, and the link guards
-- retries so a crash between the metadata and companion commits cannot
-- duplicate an append-only audit row.

-- name: GetPDFAuditLink :one
SELECT audit_event_id
FROM pdf_audit_links
WHERE event_key = sqlc.arg(event_key);

-- name: PipelineRunExists :one
SELECT 1
FROM pipeline_runs
WHERE id = sqlc.arg(id);

-- name: InsertPDFMetadataAuditEvent :execresult
INSERT INTO audit_events (
    occurred_at, actor, pipeline_run_id, entity_type,
    entity_id, action, metadata_json, correlation_id
) VALUES (
    sqlc.arg(occurred_at), sqlc.arg(actor), sqlc.arg(pipeline_run_id), sqlc.arg(entity_type),
    sqlc.arg(entity_id), sqlc.arg(action), sqlc.arg(metadata_json), sqlc.arg(correlation_id)
);

-- name: InsertPDFAuditLink :exec
INSERT INTO pdf_audit_links (event_key, audit_event_id, created_at)
VALUES (sqlc.arg(event_key), sqlc.arg(audit_event_id), sqlc.arg(created_at));
