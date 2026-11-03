-- name: MarkPDFDocumentAvailable :execresult
UPDATE pdf_documents
SET available = true
WHERE id = sqlc.arg(id);

-- name: GetPDFDocument :one
SELECT id, available
FROM pdf_documents
WHERE id = sqlc.arg(id);
