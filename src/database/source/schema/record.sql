-- name: InsertSourceRecord :execresult
INSERT INTO source_records (
    run_source_id,
    record_index,
    raw_payload,
    content_hash
) VALUES (
    sqlc.arg(run_source_id),
    sqlc.arg(record_index),
    sqlc.arg(raw_payload),
    sqlc.arg(content_hash)
);

-- name: UpdateSourceRecordParseStatus :exec
UPDATE source_records
SET
    parse_status = sqlc.arg(parse_status),
    reject_reason = sqlc.arg(reject_reason)
WHERE id = sqlc.arg(id);

-- name: ListSourceRecordsBySource :many
SELECT
    id,
    run_source_id,
    record_index,
    raw_payload,
    content_hash,
    parse_status,
    reject_reason,
    created_at
FROM source_records
WHERE run_source_id = sqlc.arg(run_source_id)
ORDER BY record_index;

-- name: CountSourceRecordsBySource :one
SELECT COUNT(*)
FROM source_records
WHERE run_source_id = sqlc.arg(run_source_id);
