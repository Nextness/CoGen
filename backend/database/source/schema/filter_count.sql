-- name: UpsertSourceFilterCount :exec
INSERT INTO source_filter_counts (
    pipeline_run_id,
    source_name,
    filter_data
) VALUES (
    sqlc.arg(pipeline_run_id),
    sqlc.arg(source_name),
    sqlc.arg(filter_data)
)
ON CONFLICT(pipeline_run_id, source_name) DO UPDATE SET
    filter_data = excluded.filter_data;

-- name: ListSourceFilterCountsByRun :many
SELECT
    id,
    pipeline_run_id,
    source_name,
    filter_data
FROM source_filter_counts
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id)
ORDER BY source_name;

-- name: GetSourceFilterCount :one
SELECT
    id,
    pipeline_run_id,
    source_name,
    filter_data
FROM source_filter_counts
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id)
  AND source_name = sqlc.arg(source_name);
