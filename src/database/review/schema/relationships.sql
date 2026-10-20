-- name: GetReviewParentCandidateByPlan :one
SELECT
    rc.id AS context_id,
    pr.id AS pipeline_run_id,
    s.search_id,
    sr.revision_label,
    ep.id AS execution_plan_id,
    CAST(COALESCE(pr.attempt_number, 0) AS INTEGER) AS attempt_number,
    pr.started_at,
    CAST((SELECT COUNT(*) FROM review_context_work_heads parent_head
        WHERE parent_head.review_context_id = rc.id
          AND EXISTS (
              SELECT 1 FROM work_revisions target_wr
              WHERE target_wr.pipeline_run_id = sqlc.arg(target_run_id)
                AND target_wr.producer_stage = 'normalize'
                AND target_wr.id = (SELECT MAX(normalized_candidate.id) FROM work_revisions normalized_candidate
                    WHERE normalized_candidate.pipeline_run_id = target_wr.pipeline_run_id
                      AND normalized_candidate.work_id = target_wr.work_id
                      AND normalized_candidate.producer_stage = 'normalize')
                AND EXISTS (SELECT 1 FROM run_work_stages current_validation
                    WHERE current_validation.pipeline_run_id = target_wr.pipeline_run_id
                      AND current_validation.work_id = target_wr.work_id
                      AND current_validation.stage_name = 'validate'
                      AND current_validation.outcome = 'valid')
                AND target_wr.work_id = parent_head.work_id)) AS INTEGER) AS inherited_work_count
FROM review_contexts rc
JOIN pipeline_runs pr ON pr.id = rc.pipeline_run_id
JOIN execution_plans ep ON ep.id = pr.execution_plan_id
JOIN search_revisions sr ON sr.id = ep.search_revision_id
JOIN searches s ON s.id = sr.search_id
WHERE ep.id = sqlc.arg(plan_id)
  AND pr.status = 'completed'
  AND pr.visibility_state != 'trashed'
  AND (pr.started_at < sqlc.arg(target_started_at)
       OR (pr.started_at = sqlc.arg(target_started_at) AND pr.id < sqlc.arg(target_run_id)))
ORDER BY pr.started_at DESC, pr.id DESC
LIMIT 1;

-- name: GetReviewParentCandidateBySearch :one
SELECT
    rc.id AS context_id,
    pr.id AS pipeline_run_id,
    s.search_id,
    sr.revision_label,
    ep.id AS execution_plan_id,
    CAST(COALESCE(pr.attempt_number, 0) AS INTEGER) AS attempt_number,
    pr.started_at,
    CAST((SELECT COUNT(*) FROM review_context_work_heads parent_head
        WHERE parent_head.review_context_id = rc.id
          AND EXISTS (
              SELECT 1 FROM work_revisions target_wr
              WHERE target_wr.pipeline_run_id = sqlc.arg(target_run_id)
                AND target_wr.producer_stage = 'normalize'
                AND target_wr.id = (SELECT MAX(normalized_candidate.id) FROM work_revisions normalized_candidate
                    WHERE normalized_candidate.pipeline_run_id = target_wr.pipeline_run_id
                      AND normalized_candidate.work_id = target_wr.work_id
                      AND normalized_candidate.producer_stage = 'normalize')
                AND EXISTS (SELECT 1 FROM run_work_stages current_validation
                    WHERE current_validation.pipeline_run_id = target_wr.pipeline_run_id
                      AND current_validation.work_id = target_wr.work_id
                      AND current_validation.stage_name = 'validate'
                      AND current_validation.outcome = 'valid')
                AND target_wr.work_id = parent_head.work_id)) AS INTEGER) AS inherited_work_count
FROM review_contexts rc
JOIN pipeline_runs pr ON pr.id = rc.pipeline_run_id
JOIN execution_plans ep ON ep.id = pr.execution_plan_id
JOIN search_revisions sr ON sr.id = ep.search_revision_id
JOIN searches s ON s.id = sr.search_id
WHERE s.id = sqlc.arg(search_db_id)
  AND pr.status = 'completed'
  AND pr.visibility_state != 'trashed'
  AND (pr.started_at < sqlc.arg(target_started_at)
       OR (pr.started_at = sqlc.arg(target_started_at) AND pr.id < sqlc.arg(target_run_id)))
ORDER BY pr.started_at DESC, pr.id DESC
LIMIT 1;

-- name: ListReviewParentCandidates :many
SELECT
    rc.id AS context_id,
    pr.id AS pipeline_run_id,
    s.search_id,
    sr.revision_label,
    ep.id AS execution_plan_id,
    CAST(COALESCE(pr.attempt_number, 0) AS INTEGER) AS attempt_number,
    pr.started_at,
    CAST((SELECT COUNT(*) FROM review_context_work_heads parent_head
        WHERE parent_head.review_context_id = rc.id
          AND EXISTS (
              SELECT 1 FROM work_revisions target_wr
              WHERE target_wr.pipeline_run_id = sqlc.arg(target_run_id)
                AND target_wr.producer_stage = 'normalize'
                AND target_wr.id = (SELECT MAX(normalized_candidate.id) FROM work_revisions normalized_candidate
                    WHERE normalized_candidate.pipeline_run_id = target_wr.pipeline_run_id
                      AND normalized_candidate.work_id = target_wr.work_id
                      AND normalized_candidate.producer_stage = 'normalize')
                AND EXISTS (SELECT 1 FROM run_work_stages current_validation
                    WHERE current_validation.pipeline_run_id = target_wr.pipeline_run_id
                      AND current_validation.work_id = target_wr.work_id
                      AND current_validation.stage_name = 'validate'
                      AND current_validation.outcome = 'valid')
                AND target_wr.work_id = parent_head.work_id)) AS INTEGER) AS inherited_work_count
FROM review_contexts rc
JOIN pipeline_runs pr ON pr.id = rc.pipeline_run_id
JOIN execution_plans ep ON ep.id = pr.execution_plan_id
JOIN search_revisions sr ON sr.id = ep.search_revision_id
JOIN searches s ON s.id = sr.search_id
WHERE pr.status = 'completed'
  AND pr.visibility_state != 'trashed'
  AND (pr.started_at < sqlc.arg(target_started_at)
       OR (pr.started_at = sqlc.arg(target_started_at) AND pr.id < sqlc.arg(target_run_id)))
  AND (CAST(sqlc.arg(scope_all) AS INTEGER) = 1 OR s.id = CAST(sqlc.arg(search_db_id) AS INTEGER))
  AND (CAST(sqlc.arg(has_cursor) AS INTEGER) = 0
       OR pr.started_at < CAST(sqlc.arg(cursor_started_at) AS TEXT)
       OR (pr.started_at = CAST(sqlc.arg(cursor_started_at) AS TEXT) AND pr.id < CAST(sqlc.arg(cursor_run_id) AS INTEGER)))
  AND (CAST(sqlc.arg(has_query) AS INTEGER) = 0
       OR s.search_id LIKE CAST(sqlc.arg(query_pattern) AS TEXT)
       OR sr.revision_label LIKE CAST(sqlc.arg(query_pattern) AS TEXT))
ORDER BY pr.started_at DESC, pr.id DESC
LIMIT CAST(sqlc.arg(page_limit) AS INTEGER);

-- name: ResolveReviewLinkNote :one
SELECT work_head.work_revision_id
FROM review_context_note_heads note_head
JOIN review_notes logical ON logical.id = note_head.note_id
JOIN review_context_work_heads work_head
    ON work_head.review_context_id = note_head.review_context_id
    AND work_head.work_id = logical.work_id
WHERE note_head.review_context_id = sqlc.arg(review_context_id)
  AND note_head.note_id = sqlc.arg(note_id);

-- name: ResolveReviewLinkArticle :one
SELECT head.work_revision_id
FROM review_context_work_heads head
JOIN works work ON work.id = head.work_id
WHERE head.review_context_id = sqlc.arg(review_context_id)
  AND work.doi = sqlc.arg(doi);

-- name: ResolveReviewLinkPDFPage :one
SELECT work_head.work_revision_id
FROM review_notes logical
JOIN review_context_work_heads work_head
    ON work_head.review_context_id = sqlc.arg(review_context_id)
    AND work_head.work_id = logical.work_id
WHERE logical.id = sqlc.arg(note_id);

-- name: ResolveReviewLinkAnchor :one
SELECT
    work_head.work_revision_id,
    version.page
FROM review_context_anchor_heads anchor_head
JOIN review_anchors logical ON logical.id = anchor_head.anchor_id
JOIN review_anchor_versions version
    ON version.id = anchor_head.anchor_version_id
    AND version.state = 'active'
JOIN review_context_work_heads work_head
    ON work_head.review_context_id = anchor_head.review_context_id
    AND work_head.work_id = logical.work_id
WHERE anchor_head.review_context_id = sqlc.arg(review_context_id)
  AND anchor_head.anchor_id = sqlc.arg(anchor_id);

-- name: ListReviewBacklinks :many
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
    CAST((SELECT COUNT(*) FROM review_note_links all_links WHERE all_links.note_version_id = version.id) AS INTEGER) AS link_count
FROM review_context_note_heads head
JOIN review_note_versions version ON version.id = head.note_version_id AND version.state = 'active'
JOIN review_notes logical ON logical.id = head.note_id
JOIN review_context_work_heads work_head
    ON work_head.review_context_id = head.review_context_id
    AND work_head.work_id = logical.work_id
JOIN review_contexts version_context ON version_context.id = version.created_in_context_id
LEFT JOIN pipeline_run_reviewers reviewer ON reviewer.pipeline_run_id = version_context.pipeline_run_id
JOIN review_note_links link ON link.note_version_id = head.note_version_id
WHERE head.review_context_id = sqlc.arg(review_context_id)
  AND link.target_type = sqlc.arg(target_type)
  AND link.raw_target = sqlc.arg(target_id)
  AND (CAST(sqlc.arg(source_work_id) AS INTEGER) = 0
       OR logical.work_id = CAST(sqlc.arg(source_work_id) AS INTEGER))
  AND (CAST(sqlc.arg(cursor) AS INTEGER) = 0 OR head.note_id < CAST(sqlc.arg(cursor) AS INTEGER))
GROUP BY logical.id, version.id
ORDER BY head.note_id DESC
LIMIT CAST(sqlc.arg(page_limit) AS INTEGER);
