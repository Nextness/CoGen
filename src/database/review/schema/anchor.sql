-- name: ReviewAnchorLabelExists :one
SELECT EXISTS(
    SELECT 1 FROM review_anchors
    WHERE work_id = sqlc.arg(work_id)
      AND label = sqlc.arg(label)
);

-- name: InsertReviewAnchor :exec
INSERT INTO review_anchors (
    id,
    work_id,
    label,
    created_at
) VALUES (
    sqlc.arg(id),
    sqlc.arg(work_id),
    sqlc.arg(label),
    sqlc.arg(created_at)
);

-- name: InsertReviewAnchorFirstVersion :execresult
INSERT INTO review_anchor_versions (
    anchor_id,
    created_in_context_id,
    work_revision_id,
    pdf_content_hash,
    state,
    page,
    selected_text,
    rectangles_json,
    created_at
) VALUES (
    sqlc.arg(anchor_id),
    sqlc.arg(created_in_context_id),
    sqlc.arg(work_revision_id),
    sqlc.arg(pdf_content_hash),
    'active',
    sqlc.arg(page),
    sqlc.arg(selected_text),
    sqlc.arg(rectangles_json),
    sqlc.arg(created_at)
);

-- name: InsertReviewAnchorHead :exec
INSERT INTO review_context_anchor_heads (
    review_context_id,
    anchor_id,
    anchor_version_id
) VALUES (
    sqlc.arg(review_context_id),
    sqlc.arg(anchor_id),
    sqlc.arg(anchor_version_id)
);

-- name: GetReviewAnchorHeadForUpdate :one
SELECT
    rc.pipeline_run_id,
    logical.work_id,
    work_head.work_revision_id,
    head.anchor_version_id,
    CAST(COALESCE(logical.label, logical.id) AS TEXT) AS label,
    logical.created_at,
    pr.status,
    pr.visibility_state
FROM review_context_anchor_heads head
JOIN review_contexts rc ON rc.id = head.review_context_id
JOIN pipeline_runs pr ON pr.id = rc.pipeline_run_id
JOIN review_anchors logical ON logical.id = head.anchor_id
JOIN review_context_work_heads work_head
    ON work_head.review_context_id = head.review_context_id
    AND work_head.work_id = logical.work_id
WHERE head.review_context_id = sqlc.arg(review_context_id)
  AND head.anchor_id = sqlc.arg(anchor_id);

-- name: InsertReviewAnchorVersion :execresult
INSERT INTO review_anchor_versions (
    anchor_id,
    parent_version_id,
    created_in_context_id,
    work_revision_id,
    pdf_content_hash,
    state,
    page,
    selected_text,
    rectangles_json,
    created_at
) VALUES (
    sqlc.arg(anchor_id),
    sqlc.arg(parent_version_id),
    sqlc.arg(created_in_context_id),
    sqlc.arg(work_revision_id),
    sqlc.arg(pdf_content_hash),
    sqlc.arg(state),
    sqlc.arg(page),
    sqlc.arg(selected_text),
    sqlc.arg(rectangles_json),
    sqlc.arg(created_at)
);

-- name: UpdateReviewAnchorHeadCAS :execresult
UPDATE review_context_anchor_heads
SET anchor_version_id = sqlc.arg(anchor_version_id)
WHERE review_context_id = sqlc.arg(review_context_id)
  AND anchor_id = sqlc.arg(anchor_id)
  AND anchor_version_id = sqlc.arg(expected_version_id);

-- name: ListReviewAnchors :many
SELECT
    logical.id,
    CAST(COALESCE(logical.label, logical.id) AS TEXT) AS label,
    logical.work_id,
    logical.created_at,
    version.id AS version_id,
    version.anchor_id,
    version.parent_version_id,
    version.created_in_context_id,
    version.work_revision_id,
    version.pdf_content_hash,
    version.state,
    version.page,
    CAST(COALESCE(substr(version.selected_text, 1, CAST(sqlc.arg(preview_bytes) AS INTEGER)), '') AS TEXT) AS selected_text_preview,
    CAST(COALESCE(length(CAST(version.selected_text AS BLOB)), 0) AS INTEGER) AS selected_text_bytes,
    CAST(version.selected_text IS NOT NULL AS INTEGER) AS has_selected_text,
    version.rectangles_json,
    version.created_at AS version_created_at,
    CAST(COALESCE(reviewer.username, '') AS TEXT) AS reviewer_username,
    CAST(COALESCE(reviewer.email, '') AS TEXT) AS reviewer_email
FROM review_context_anchor_heads head
JOIN review_anchors logical ON logical.id = head.anchor_id
JOIN review_context_work_heads work_head
    ON work_head.review_context_id = head.review_context_id
    AND work_head.work_id = logical.work_id
JOIN review_anchor_versions version
    ON version.id = head.anchor_version_id
    AND version.state = 'active'
JOIN review_contexts version_context ON version_context.id = version.created_in_context_id
LEFT JOIN pipeline_run_reviewers reviewer ON reviewer.pipeline_run_id = version_context.pipeline_run_id
WHERE head.review_context_id = sqlc.arg(review_context_id)
  AND work_head.work_revision_id = sqlc.arg(work_revision_id)
  AND (CAST(sqlc.arg(cursor) AS TEXT) = '' OR head.anchor_id > CAST(sqlc.arg(cursor) AS TEXT))
ORDER BY head.anchor_id
LIMIT CAST(sqlc.arg(page_limit) AS INTEGER);

-- name: GetReviewAnchorHead :one
SELECT
    logical.id,
    CAST(COALESCE(logical.label, logical.id) AS TEXT) AS label,
    logical.work_id,
    logical.created_at,
    head.anchor_version_id
FROM review_context_anchor_heads head
JOIN review_anchors logical ON logical.id = head.anchor_id
WHERE head.review_context_id = sqlc.arg(review_context_id)
  AND head.anchor_id = sqlc.arg(anchor_id);

-- name: ListReviewAnchorVersionAncestry :many
WITH RECURSIVE ancestry(id) AS (
    SELECT anchor_head.anchor_version_id
    FROM review_context_anchor_heads anchor_head
    WHERE anchor_head.review_context_id = sqlc.arg(review_context_id)
      AND anchor_head.anchor_id = sqlc.arg(anchor_id)
    UNION ALL
    SELECT version.parent_version_id
    FROM review_anchor_versions version
    JOIN ancestry ON version.id = ancestry.id
    WHERE version.parent_version_id IS NOT NULL
)
SELECT
    version.id,
    version.anchor_id,
    version.parent_version_id,
    version.created_in_context_id,
    version.work_revision_id,
    version.pdf_content_hash,
    version.state,
    version.page,
    CAST(COALESCE(substr(version.selected_text, 1, CAST(sqlc.arg(preview_bytes) AS INTEGER)), '') AS TEXT) AS selected_text_preview,
    CAST(COALESCE(length(CAST(version.selected_text AS BLOB)), 0) AS INTEGER) AS selected_text_bytes,
    CAST(version.selected_text IS NOT NULL AS INTEGER) AS has_selected_text,
    version.rectangles_json,
    version.created_at,
    CAST(COALESCE(reviewer.username, '') AS TEXT) AS reviewer_username,
    CAST(COALESCE(reviewer.email, '') AS TEXT) AS reviewer_email
FROM ancestry
JOIN review_anchor_versions version ON version.id = ancestry.id
JOIN review_contexts version_context ON version_context.id = version.created_in_context_id
LEFT JOIN pipeline_run_reviewers reviewer ON reviewer.pipeline_run_id = version_context.pipeline_run_id
WHERE (CAST(sqlc.arg(cursor) AS INTEGER) = 0 OR version.id < CAST(sqlc.arg(cursor) AS INTEGER))
ORDER BY version.id DESC
LIMIT CAST(sqlc.arg(page_limit) AS INTEGER);

-- name: ReviewAnchorVersionInAncestry :one
WITH RECURSIVE ancestry(id) AS (
    SELECT anchor_head.anchor_version_id
    FROM review_context_anchor_heads anchor_head
    WHERE anchor_head.review_context_id = sqlc.arg(review_context_id)
      AND anchor_head.anchor_id = sqlc.arg(anchor_id)
    UNION ALL
    SELECT version.parent_version_id
    FROM review_anchor_versions version
    JOIN ancestry ON version.id = ancestry.id
    WHERE version.parent_version_id IS NOT NULL
)
SELECT EXISTS(SELECT 1 FROM ancestry WHERE id = sqlc.arg(version_id));

-- name: GetReviewAnchorVersionByID :one
SELECT
    version.id,
    version.anchor_id,
    version.parent_version_id,
    version.created_in_context_id,
    version.work_revision_id,
    version.pdf_content_hash,
    version.state,
    version.page,
    version.selected_text,
    version.rectangles_json,
    version.created_at,
    CAST(COALESCE(reviewer.username, '') AS TEXT) AS reviewer_username,
    CAST(COALESCE(reviewer.email, '') AS TEXT) AS reviewer_email
FROM review_anchor_versions version
JOIN review_contexts context ON context.id = version.created_in_context_id
LEFT JOIN pipeline_run_reviewers reviewer ON reviewer.pipeline_run_id = context.pipeline_run_id
WHERE version.id = sqlc.arg(id);
