-- name: InsertReviewNote :execresult
INSERT INTO review_notes (
    work_id,
    created_at
) VALUES (
    sqlc.arg(work_id),
    sqlc.arg(created_at)
);

-- name: InsertReviewNoteFirstVersion :execresult
INSERT INTO review_note_versions (
    note_id,
    created_in_context_id,
    state,
    body,
    created_at
) VALUES (
    sqlc.arg(note_id),
    sqlc.arg(created_in_context_id),
    'active',
    sqlc.arg(body),
    sqlc.arg(created_at)
);

-- name: InsertReviewNoteHead :exec
INSERT INTO review_context_note_heads (
    review_context_id,
    note_id,
    note_version_id
) VALUES (
    sqlc.arg(review_context_id),
    sqlc.arg(note_id),
    sqlc.arg(note_version_id)
);

-- name: GetReviewNoteHeadForUpdate :one
SELECT
    rc.pipeline_run_id,
    logical.work_id,
    work_head.work_revision_id,
    head.note_version_id,
    logical.created_at,
    pr.status,
    pr.visibility_state
FROM review_context_note_heads head
JOIN review_contexts rc ON rc.id = head.review_context_id
JOIN pipeline_runs pr ON pr.id = rc.pipeline_run_id
JOIN review_notes logical ON logical.id = head.note_id
JOIN review_context_work_heads work_head
    ON work_head.review_context_id = head.review_context_id
    AND work_head.work_id = logical.work_id
WHERE head.review_context_id = sqlc.arg(review_context_id)
  AND head.note_id = sqlc.arg(note_id);

-- name: InsertReviewNoteVersion :execresult
INSERT INTO review_note_versions (
    note_id,
    parent_version_id,
    created_in_context_id,
    state,
    body,
    created_at
) VALUES (
    sqlc.arg(note_id),
    sqlc.arg(parent_version_id),
    sqlc.arg(created_in_context_id),
    sqlc.arg(state),
    sqlc.arg(body),
    sqlc.arg(created_at)
);

-- name: UpdateReviewNoteHeadCAS :execresult
UPDATE review_context_note_heads
SET note_version_id = sqlc.arg(note_version_id)
WHERE review_context_id = sqlc.arg(review_context_id)
  AND note_id = sqlc.arg(note_id)
  AND note_version_id = sqlc.arg(expected_version_id);

-- name: GetReviewNoteHead :one
SELECT
    logical.id,
    logical.work_id,
    work_head.work_revision_id,
    logical.created_at,
    head.note_version_id
FROM review_context_note_heads head
JOIN review_notes logical ON logical.id = head.note_id
JOIN review_context_work_heads work_head
    ON work_head.review_context_id = head.review_context_id
    AND work_head.work_id = logical.work_id
WHERE head.review_context_id = sqlc.arg(review_context_id)
  AND head.note_id = sqlc.arg(note_id);

-- name: ListReviewNoteHeads :many
SELECT
    logical.id,
    logical.work_id,
    work_head.work_revision_id,
    logical.created_at,
    version.id AS version_id,
    version.note_id,
    version.parent_version_id,
    version.created_in_context_id,
    version.state,
    CAST(COALESCE(substr(version.body, 1, CAST(sqlc.arg(preview_bytes) AS INTEGER)), '') AS TEXT) AS body_preview,
    CAST(COALESCE(length(CAST(version.body AS BLOB)), 0) AS INTEGER) AS body_bytes,
    CAST(version.body IS NOT NULL AS INTEGER) AS has_body,
    version.created_at AS version_created_at,
    CAST(COALESCE(reviewer.username, '') AS TEXT) AS reviewer_username,
    CAST(COALESCE(reviewer.email, '') AS TEXT) AS reviewer_email,
    CAST((SELECT COUNT(*) FROM review_note_links link WHERE link.note_version_id = version.id) AS INTEGER) AS link_count
FROM review_context_note_heads head
JOIN review_context_work_heads work_head ON work_head.review_context_id = head.review_context_id
JOIN review_notes logical ON logical.id = head.note_id AND logical.work_id = work_head.work_id
JOIN review_note_versions version ON version.id = head.note_version_id
JOIN review_contexts version_context ON version_context.id = version.created_in_context_id
LEFT JOIN pipeline_run_reviewers reviewer ON reviewer.pipeline_run_id = version_context.pipeline_run_id
WHERE head.review_context_id = sqlc.arg(review_context_id)
  AND (CAST(sqlc.arg(cursor) AS INTEGER) = 0 OR head.note_id < CAST(sqlc.arg(cursor) AS INTEGER))
  AND (CAST(sqlc.arg(state_filter) AS TEXT) = 'all'
       OR (CAST(sqlc.arg(state_filter) AS TEXT) = 'active' AND version.state = 'active')
       OR (CAST(sqlc.arg(state_filter) AS TEXT) = 'removed' AND version.state = 'deleted'))
  AND (CAST(sqlc.arg(has_work_revision) AS INTEGER) = 0
       OR work_head.work_revision_id = CAST(sqlc.arg(work_revision_id) AS INTEGER))
  AND (CAST(sqlc.arg(has_query) AS INTEGER) = 0
       OR (version.body LIKE sqlc.arg(query_pattern) ESCAPE '\'))
ORDER BY head.note_id DESC
LIMIT CAST(sqlc.arg(page_limit) AS INTEGER);

-- name: ListReviewNoteVersionAncestry :many
WITH RECURSIVE ancestry(id) AS (
    SELECT note_head.note_version_id
    FROM review_context_note_heads note_head
    WHERE note_head.review_context_id = sqlc.arg(review_context_id)
      AND note_head.note_id = sqlc.arg(note_id)
    UNION ALL
    SELECT version.parent_version_id
    FROM review_note_versions version
    JOIN ancestry ON version.id = ancestry.id
    WHERE version.parent_version_id IS NOT NULL
)
SELECT
    version.id,
    version.note_id,
    version.parent_version_id,
    version.created_in_context_id,
    version.state,
    CAST(COALESCE(substr(version.body, 1, CAST(sqlc.arg(preview_bytes) AS INTEGER)), '') AS TEXT) AS body_preview,
    CAST(COALESCE(length(CAST(version.body AS BLOB)), 0) AS INTEGER) AS body_bytes,
    CAST(version.body IS NOT NULL AS INTEGER) AS has_body,
    version.created_at,
    CAST(COALESCE(reviewer.username, '') AS TEXT) AS reviewer_username,
    CAST(COALESCE(reviewer.email, '') AS TEXT) AS reviewer_email,
    CAST((SELECT COUNT(*) FROM review_note_links link WHERE link.note_version_id = version.id) AS INTEGER) AS link_count
FROM ancestry
JOIN review_note_versions version ON version.id = ancestry.id
JOIN review_contexts version_context ON version_context.id = version.created_in_context_id
LEFT JOIN pipeline_run_reviewers reviewer ON reviewer.pipeline_run_id = version_context.pipeline_run_id
WHERE (CAST(sqlc.arg(cursor) AS INTEGER) = 0 OR version.id < CAST(sqlc.arg(cursor) AS INTEGER))
ORDER BY version.id DESC
LIMIT CAST(sqlc.arg(page_limit) AS INTEGER);

-- name: ReviewNoteVersionInAncestry :one
WITH RECURSIVE ancestry(id) AS (
    SELECT note_head.note_version_id
    FROM review_context_note_heads note_head
    WHERE note_head.review_context_id = sqlc.arg(review_context_id)
      AND note_head.note_id = sqlc.arg(note_id)
    UNION ALL
    SELECT version.parent_version_id
    FROM review_note_versions version
    JOIN ancestry ON version.id = ancestry.id
    WHERE version.parent_version_id IS NOT NULL
)
SELECT EXISTS(SELECT 1 FROM ancestry WHERE id = sqlc.arg(version_id));

-- name: GetReviewNoteVersionByID :one
SELECT
    version.id,
    version.note_id,
    version.parent_version_id,
    version.created_in_context_id,
    version.state,
    version.body,
    version.created_at,
    CAST(COALESCE(reviewer.username, '') AS TEXT) AS reviewer_username,
    CAST(COALESCE(reviewer.email, '') AS TEXT) AS reviewer_email
FROM review_note_versions version
JOIN review_contexts context ON context.id = version.created_in_context_id
LEFT JOIN pipeline_run_reviewers reviewer ON reviewer.pipeline_run_id = context.pipeline_run_id
WHERE version.id = sqlc.arg(id);

-- name: InsertReviewNoteLink :exec
INSERT INTO review_note_links (
    note_version_id,
    ordinal,
    target_type,
    raw_target,
    display_text,
    utf16_position,
    utf16_length,
    created_at
) VALUES (
    sqlc.arg(note_version_id),
    sqlc.arg(ordinal),
    sqlc.arg(target_type),
    sqlc.arg(raw_target),
    sqlc.arg(display_text),
    sqlc.arg(utf16_position),
    sqlc.arg(utf16_length),
    sqlc.arg(created_at)
);

-- name: ListReviewNoteLinks :many
SELECT
    ordinal,
    target_type,
    raw_target,
    display_text,
    utf16_position,
    utf16_length
FROM review_note_links
WHERE note_version_id = sqlc.arg(note_version_id)
ORDER BY ordinal;
