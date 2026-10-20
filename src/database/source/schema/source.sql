-- name: InsertRunSource :execresult
INSERT INTO run_sources (
    pipeline_run_id,
    source_name,
    source_type,
    expected_file,
    query,
    requested_fields,
    expected_result_count,
    export_date
) VALUES (
    sqlc.arg(pipeline_run_id),
    sqlc.arg(source_name),
    sqlc.arg(source_type),
    sqlc.arg(expected_file),
    sqlc.arg(query),
    sqlc.arg(requested_fields),
    sqlc.arg(expected_result_count),
    sqlc.arg(export_date)
);

-- name: ListRunSourcesByRun :many
SELECT
    id,
    pipeline_run_id,
    source_name,
    source_type,
    expected_file,
    query,
    requested_fields,
    expected_result_count,
    observed_result_count,
    result_count_comparison,
    export_date,
    created_at
FROM run_sources
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id)
ORDER BY id;

-- name: SetRunSourceObservedResultCount :exec
UPDATE run_sources
SET
    observed_result_count = sqlc.arg(observed_result_count),
    result_count_comparison = sqlc.arg(result_count_comparison)
WHERE id = sqlc.arg(id);
