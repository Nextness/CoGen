-- name: InsertReviewer :exec
INSERT INTO pipeline_run_reviewers (
    pipeline_run_id,
    username,
    email,
    created_at
) VALUES (
    sqlc.arg(pipeline_run_id),
    sqlc.arg(username),
    sqlc.arg(email),
    sqlc.arg(created_at)
);

-- name: GetReviewerByRun :one
SELECT
    pipeline_run_id,
    username,
    email,
    created_at
FROM pipeline_run_reviewers
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id);
