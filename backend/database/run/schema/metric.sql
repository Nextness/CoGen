-- name: UpsertRunMetric :exec
INSERT OR REPLACE INTO pipeline_run_metrics (
    pipeline_run_id,
    metric,
    source,
    value
) VALUES (
    sqlc.arg(pipeline_run_id),
    sqlc.arg(metric),
    sqlc.arg(source),
    sqlc.arg(value)
);

-- name: GetRunMetric :one
SELECT
    pipeline_run_id,
    metric,
    source,
    value
FROM pipeline_run_metrics
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id)
  AND metric = sqlc.arg(metric)
  AND source = sqlc.arg(source);

-- name: ListRunMetricsByRun :many
SELECT
    pipeline_run_id,
    metric,
    source,
    value
FROM pipeline_run_metrics
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id)
ORDER BY metric, source;
