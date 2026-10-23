-- Companion PDF store queries. Registration, insertion, inventory lookup,
-- blob reads, and outbox selection are typed; the cross-database delivery
-- protocol keeps its handwritten savepoints and metadata transaction.

-- name: GetPDFDocument :one
SELECT doi, status, content_hash, inventoried_at, updated_at
FROM pdf_documents
WHERE doi = sqlc.arg(doi);

-- name: InsertPDFDocument :execresult
INSERT OR IGNORE INTO pdf_documents (doi, status, updated_at)
VALUES (sqlc.arg(doi), 'not_available', sqlc.arg(updated_at));

-- name: GetPDFDocumentWithBlob :one
SELECT d.status, d.content_hash, b.byte_size, b.data
FROM pdf_documents d
LEFT JOIN pdf_blobs b ON b.content_hash = d.content_hash
WHERE d.doi = sqlc.arg(doi);

-- name: InsertPDFBlob :exec
INSERT OR IGNORE INTO pdf_blobs (content_hash, byte_size, data, created_at)
VALUES (sqlc.arg(content_hash), sqlc.arg(byte_size), sqlc.arg(data), sqlc.arg(created_at));

-- name: GetPDFBlobSize :one
SELECT byte_size
FROM pdf_blobs
WHERE content_hash = sqlc.arg(content_hash);

-- name: MarkPDFDocumentAvailable :execresult
UPDATE pdf_documents
SET status = 'available',
    content_hash = sqlc.arg(content_hash),
    inventoried_at = sqlc.arg(inventoried_at),
    updated_at = sqlc.arg(updated_at)
WHERE doi = sqlc.arg(doi)
  AND status = 'not_available';

-- name: GetAvailablePDFDocumentWithBlob :one
SELECT d.content_hash, b.byte_size, b.data
FROM pdf_documents d
JOIN pdf_blobs b ON b.content_hash = d.content_hash
WHERE d.doi = sqlc.arg(doi)
  AND d.status = 'available';

-- name: InsertPDFAuditOutboxEvent :exec
INSERT INTO pdf_audit_outbox (
    event_key, occurred_at, actor, pipeline_run_id, entity_type,
    entity_id, action, metadata_json, correlation_id
) VALUES (
    sqlc.arg(event_key), sqlc.arg(occurred_at), sqlc.arg(actor), sqlc.arg(pipeline_run_id),
    sqlc.arg(entity_type), sqlc.arg(entity_id), sqlc.arg(action), sqlc.arg(metadata_json),
    sqlc.arg(correlation_id)
);

-- name: ListPendingPDFAuditEvents :many
SELECT event_key, occurred_at, actor, pipeline_run_id, entity_type,
    entity_id, action, metadata_json, correlation_id
FROM pdf_audit_outbox
WHERE delivered_at IS NULL
ORDER BY occurred_at, event_key
LIMIT sqlc.arg(batch_limit);

-- name: ListPendingPDFAuditEventsAfter :many
SELECT event_key, occurred_at, actor, pipeline_run_id, entity_type,
    entity_id, action, metadata_json, correlation_id
FROM pdf_audit_outbox
WHERE delivered_at IS NULL
  AND (occurred_at > sqlc.arg(after_occurred_at)
       OR (occurred_at = sqlc.arg(after_occurred_at) AND event_key > sqlc.arg(after_event_key)))
ORDER BY occurred_at, event_key
LIMIT sqlc.arg(batch_limit);

-- name: MarkPDFAuditEventsDelivered :execresult
UPDATE pdf_audit_outbox
SET delivered_at = sqlc.arg(delivered_at)
WHERE delivered_at IS NULL
  AND event_key IN (sqlc.slice('event_keys'));

-- Viewer-facing companion PDF reads.

-- name: ListAvailablePDFDOIs :many
SELECT document.doi
FROM pdf_documents document
JOIN pdf_blobs blob ON blob.content_hash=document.content_hash
WHERE document.status='available'
ORDER BY document.doi;

-- name: ListAvailableDocumentInventory :many
SELECT document.doi, document.inventoried_at
FROM pdf_documents document
JOIN pdf_blobs blob ON blob.content_hash=document.content_hash
WHERE document.status='available'
  AND document.doi IN (sqlc.slice('dois'))
ORDER BY document.doi;

-- name: GetAvailablePDFDocument :one
SELECT
    d.content_hash,
    d.inventoried_at,
    b.byte_size
FROM pdf_documents d
JOIN pdf_blobs b ON b.content_hash=d.content_hash
WHERE d.doi = sqlc.arg(doi)
  AND d.status='available';

-- name: GetPDFBlobData :one
SELECT data
FROM pdf_blobs
WHERE content_hash = sqlc.arg(content_hash);
