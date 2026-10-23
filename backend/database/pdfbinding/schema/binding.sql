-- Metadata-side companion PDF binding queries. The binding is a singleton row
-- that records the portable bundle-relative companion path.

-- name: GetPDFStoreBinding :one
SELECT relative_path
FROM pdf_store_binding
WHERE id = 1;

-- name: InsertPDFStoreBinding :exec
INSERT INTO pdf_store_binding (id, relative_path, configured_at, config_fingerprint)
VALUES (1, sqlc.arg(relative_path), sqlc.arg(configured_at), sqlc.arg(config_fingerprint));
