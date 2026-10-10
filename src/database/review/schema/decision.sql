-- name: GetWorkReviewHead :one
SELECT
    review_context_id,
    work_id,
    work_revision_id,
    review_version_id
FROM review_context_work_heads
WHERE review_context_id = sqlc.arg(review_context_id)
  AND work_revision_id = sqlc.arg(work_revision_id);

-- name: InsertWorkReviewVersion :execresult
INSERT INTO work_review_versions (
    work_id,
    work_revision_id,
    created_in_context_id,
    parent_version_id,
    status,
    reason,
    created_at
) VALUES (
    sqlc.arg(work_id),
    sqlc.arg(work_revision_id),
    sqlc.arg(created_in_context_id),
    sqlc.arg(parent_version_id),
    sqlc.arg(status),
    sqlc.arg(reason),
    sqlc.arg(created_at)
);

-- name: InsertWorkReviewSubstatus :exec
INSERT INTO work_review_version_substatuses (
    review_version_id,
    sub_status
) VALUES (
    sqlc.arg(review_version_id),
    sqlc.arg(sub_status)
);

-- name: UpdateWorkReviewHeadCASNull :execresult
UPDATE review_context_work_heads
SET review_version_id = sqlc.arg(review_version_id)
WHERE review_context_id = sqlc.arg(review_context_id)
  AND work_revision_id = sqlc.arg(work_revision_id)
  AND review_version_id IS NULL;

-- name: UpdateWorkReviewHeadCASValue :execresult
UPDATE review_context_work_heads
SET review_version_id = sqlc.arg(review_version_id)
WHERE review_context_id = sqlc.arg(review_context_id)
  AND work_revision_id = sqlc.arg(work_revision_id)
  AND review_version_id = sqlc.arg(expected_version_id);

-- name: GetMutableWorkReviewHead :one
SELECT
    rc.pipeline_run_id,
    head.work_id,
    head.review_version_id,
    pr.status,
    pr.visibility_state
FROM review_context_work_heads head
JOIN review_contexts rc ON rc.id = head.review_context_id
JOIN pipeline_runs pr ON pr.id = rc.pipeline_run_id
JOIN work_revisions wr
    ON wr.id = head.work_revision_id
    AND wr.pipeline_run_id = rc.pipeline_run_id
    AND wr.work_id = head.work_id
WHERE head.review_context_id = sqlc.arg(review_context_id)
  AND head.work_revision_id = sqlc.arg(work_revision_id);

-- name: ListWorkReviewVersionAncestry :many
WITH RECURSIVE ancestry(id) AS (
    SELECT review_head.review_version_id
    FROM review_context_work_heads review_head
    WHERE review_head.review_context_id = sqlc.arg(review_context_id)
      AND review_head.work_revision_id = sqlc.arg(work_revision_id)
      AND review_head.review_version_id IS NOT NULL
    UNION ALL
    SELECT version.parent_version_id
    FROM work_review_versions version
    JOIN ancestry ON version.id = ancestry.id
    WHERE version.parent_version_id IS NOT NULL
)
SELECT
    version.id,
    version.work_id,
    version.work_revision_id,
    version.created_in_context_id,
    version.parent_version_id,
    version.status,
    CAST(COALESCE(substr(version.reason, 1, CAST(sqlc.arg(preview_bytes) AS INTEGER)), '') AS TEXT) AS reason_preview,
    CAST(COALESCE(length(CAST(version.reason AS BLOB)), 0) AS INTEGER) AS reason_bytes,
    CAST(version.reason IS NOT NULL AS INTEGER) AS has_reason,
    version.created_at,
    CAST(COALESCE(reviewer.username, '') AS TEXT) AS reviewer_username,
    CAST(COALESCE(reviewer.email, '') AS TEXT) AS reviewer_email,
    CAST(COALESCE((SELECT json_group_array(sub_status) FROM (
        SELECT sub_status FROM work_review_version_substatuses
        WHERE review_version_id = version.id
        ORDER BY sub_status
    )), '[]') AS TEXT) AS substatuses_json
FROM ancestry
JOIN work_review_versions version ON version.id = ancestry.id
JOIN review_contexts version_context ON version_context.id = version.created_in_context_id
LEFT JOIN pipeline_run_reviewers reviewer ON reviewer.pipeline_run_id = version_context.pipeline_run_id
WHERE (CAST(sqlc.arg(cursor) AS INTEGER) = 0 OR version.id < CAST(sqlc.arg(cursor) AS INTEGER))
ORDER BY version.id DESC
LIMIT CAST(sqlc.arg(page_limit) AS INTEGER);

-- name: ReviewWorkReviewVersionInAncestry :one
WITH RECURSIVE ancestry(id) AS (
    SELECT review_head.review_version_id
    FROM review_context_work_heads review_head
    WHERE review_head.review_context_id = sqlc.arg(review_context_id)
      AND review_head.work_revision_id = sqlc.arg(work_revision_id)
      AND review_head.review_version_id IS NOT NULL
    UNION ALL
    SELECT version.parent_version_id
    FROM work_review_versions version
    JOIN ancestry ON version.id = ancestry.id
    WHERE version.parent_version_id IS NOT NULL
)
SELECT EXISTS(SELECT 1 FROM ancestry WHERE id = sqlc.arg(version_id));

-- name: GetWorkReviewVersionByID :one
SELECT
    version.id,
    version.work_id,
    version.work_revision_id,
    version.created_in_context_id,
    version.parent_version_id,
    version.status,
    version.reason,
    version.created_at,
    CAST(COALESCE(reviewer.username, '') AS TEXT) AS reviewer_username,
    CAST(COALESCE(reviewer.email, '') AS TEXT) AS reviewer_email
FROM work_review_versions version
JOIN review_contexts context ON context.id = version.created_in_context_id
LEFT JOIN pipeline_run_reviewers reviewer ON reviewer.pipeline_run_id = context.pipeline_run_id
WHERE version.id = sqlc.arg(id);

-- name: ListWorkReviewSubstatuses :many
SELECT sub_status
FROM work_review_version_substatuses
WHERE review_version_id = sqlc.arg(review_version_id)
ORDER BY sub_status;

-- name: CountReviewVersionAncestry :one
WITH RECURSIVE ancestry(id) AS (
    SELECT heads.review_version_id
    FROM review_context_work_heads heads
    WHERE heads.review_context_id = sqlc.arg(review_context_id)
      AND heads.work_id = sqlc.arg(work_id)
    UNION ALL
    SELECT version.parent_version_id
    FROM work_review_versions version
    JOIN ancestry ON ancestry.id=version.id
    WHERE version.parent_version_id IS NOT NULL
)
SELECT COUNT(*) FROM ancestry WHERE id IS NOT NULL;
