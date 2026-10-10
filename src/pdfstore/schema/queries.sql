-- Viewer-facing companion PDF reads. Registration, insertion, and outbox
-- delivery keep their handwritten orchestration and savepoint protocol.

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
