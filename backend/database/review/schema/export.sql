-- name: GetReviewExportContext :one
SELECT id
FROM review_contexts
WHERE id = sqlc.arg(review_context_id);

-- name: CountReviewExportAnchors :one
SELECT COUNT(*)
FROM review_context_anchor_heads head
JOIN review_anchors logical ON logical.id = head.anchor_id
JOIN review_anchor_versions version ON version.id = head.anchor_version_id
WHERE head.review_context_id = sqlc.arg(review_context_id)
  AND logical.work_id = sqlc.arg(work_id)
  AND version.state = 'active';

-- name: CountReviewExportNotes :one
SELECT COUNT(*)
FROM review_context_note_heads head
JOIN review_notes logical ON logical.id = head.note_id
JOIN review_note_versions version ON version.id = head.note_version_id
WHERE head.review_context_id = sqlc.arg(review_context_id)
  AND logical.work_id = sqlc.arg(work_id)
  AND version.state = 'active';

-- name: ListReviewExportAnchors :many
SELECT
    logical.id,
    CAST(COALESCE(logical.label, logical.id) AS TEXT) AS label,
    logical.work_id,
    logical.created_at,
    version.id AS version_id,
    version.work_revision_id,
    version.pdf_content_hash,
    version.page,
    version.selected_text,
    version.rectangles_json,
    version.created_at AS version_created_at,
    CAST(COALESCE(reviewer.username, '') AS TEXT) AS reviewer_username
FROM review_context_anchor_heads head
JOIN review_anchors logical ON logical.id = head.anchor_id
JOIN review_context_work_heads work_head
    ON work_head.review_context_id = head.review_context_id
    AND work_head.work_id = logical.work_id
JOIN review_anchor_versions version ON version.id = head.anchor_version_id
JOIN review_contexts version_context ON version_context.id = version.created_in_context_id
LEFT JOIN pipeline_run_reviewers reviewer ON reviewer.pipeline_run_id = version_context.pipeline_run_id
WHERE head.review_context_id = sqlc.arg(review_context_id)
  AND logical.work_id = sqlc.arg(work_id)
  AND version.state = 'active'
  AND version.pdf_content_hash = sqlc.arg(pdf_content_hash)
  AND version.page IS NOT NULL
ORDER BY head.anchor_id
LIMIT CAST(sqlc.arg(page_limit) AS INTEGER);

-- name: ListReviewExportNotes :many
SELECT
    logical.id,
    logical.work_id,
    work_head.work_revision_id,
    logical.created_at,
    version.id AS version_id,
    version.body,
    version.created_at AS version_created_at,
    CAST(COALESCE(reviewer.username, '') AS TEXT) AS reviewer_username
FROM review_context_note_heads head
JOIN review_notes logical ON logical.id = head.note_id
JOIN review_context_work_heads work_head
    ON work_head.review_context_id = head.review_context_id
    AND work_head.work_id = logical.work_id
JOIN review_note_versions version ON version.id = head.note_version_id
JOIN review_contexts version_context ON version_context.id = version.created_in_context_id
LEFT JOIN pipeline_run_reviewers reviewer ON reviewer.pipeline_run_id = version_context.pipeline_run_id
WHERE head.review_context_id = sqlc.arg(review_context_id)
  AND logical.work_id = sqlc.arg(work_id)
  AND version.state = 'active'
  AND EXISTS (
      SELECT 1
      FROM review_note_links link
      WHERE link.note_version_id = version.id
        AND link.target_type IN ('anchor', 'pdf_page')
  )
ORDER BY head.note_id
LIMIT CAST(sqlc.arg(page_limit) AS INTEGER);

-- name: ListReviewExportNoteLinks :many
SELECT
    link.note_version_id,
    link.ordinal,
    link.target_type,
    link.raw_target
FROM review_note_links link
JOIN review_context_note_heads head ON head.note_version_id = link.note_version_id
JOIN review_notes logical ON logical.id = head.note_id
WHERE head.review_context_id = sqlc.arg(review_context_id)
  AND logical.work_id = sqlc.arg(work_id)
  AND link.target_type IN ('anchor', 'pdf_page')
ORDER BY link.note_version_id, link.ordinal
LIMIT CAST(sqlc.arg(page_limit) AS INTEGER);
