-- viewer.sql holds the source family's viewer result-count projections.
-- The four named variants are the finite schema-compatibility set: older
-- databases may lack the export_date and result-count columns, so each
-- combination projects the absent columns as NULL instead of failing.

-- name: ListResultCountsForRun :many
SELECT
    id,
    source_name,
    source_type,
    expected_file,
    query,
    expected_result_count,
    observed_result_count,
    result_count_comparison,
    export_date
FROM run_sources
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id)
ORDER BY id;

-- name: ListResultCountsForRunWithoutExportDate :many
SELECT
    id,
    source_name,
    source_type,
    expected_file,
    query,
    expected_result_count,
    observed_result_count,
    result_count_comparison,
    CAST(NULL AS TEXT) AS export_date
FROM run_sources
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id)
ORDER BY id;

-- name: ListResultCountsForRunWithoutResultCounts :many
SELECT
    id,
    source_name,
    source_type,
    expected_file,
    query,
    CAST(NULL AS INTEGER) AS expected_result_count,
    CAST(NULL AS INTEGER) AS observed_result_count,
    CAST(NULL AS TEXT) AS result_count_comparison,
    export_date
FROM run_sources
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id)
ORDER BY id;

-- name: ListResultCountsForRunWithoutResultCountsAndExportDate :many
SELECT
    id,
    source_name,
    source_type,
    expected_file,
    query,
    CAST(NULL AS INTEGER) AS expected_result_count,
    CAST(NULL AS INTEGER) AS observed_result_count,
    CAST(NULL AS TEXT) AS result_count_comparison,
    CAST(NULL AS TEXT) AS export_date
FROM run_sources
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id)
ORDER BY id;
