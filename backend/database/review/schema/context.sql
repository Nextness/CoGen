-- name: GetReviewCorpusID :one
SELECT corpus_id
FROM review_settings
WHERE id = 1;

-- name: GetReviewContextByRun :one
SELECT
    id,
    pipeline_run_id,
    parent_context_id,
    created_at
FROM review_contexts
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id);

-- name: GetReviewTarget :one
SELECT
    pr.id AS run_id,
    CAST(COALESCE(pr.execution_plan_id, 0) AS INTEGER) AS execution_plan_id,
    s.id AS search_db_id,
    pr.started_at,
    pr.status,
    pr.visibility_state
FROM pipeline_runs pr
JOIN execution_plans ep ON ep.id = pr.execution_plan_id
JOIN search_revisions sr ON sr.id = ep.search_revision_id
JOIN searches s ON s.id = sr.search_id
WHERE pr.id = sqlc.arg(run_id);

-- name: GetReviewParentContextRun :one
SELECT
    pr.id AS run_id,
    pr.started_at,
    pr.status,
    pr.visibility_state
FROM review_contexts rc
JOIN pipeline_runs pr ON pr.id = rc.pipeline_run_id
WHERE rc.id = sqlc.arg(context_id);

-- name: InsertReviewContext :execresult
INSERT INTO review_contexts (
    pipeline_run_id,
    parent_context_id,
    created_at
) VALUES (
    sqlc.arg(pipeline_run_id),
    sqlc.arg(parent_context_id),
    sqlc.arg(created_at)
);

-- name: InsertReviewContextWorkHeads :exec
INSERT INTO review_context_work_heads (
    review_context_id,
    work_id,
    work_revision_id,
    review_version_id
)
SELECT
    sqlc.arg(review_context_id),
    latest.work_id,
    latest.id,
    parent.review_version_id
FROM work_revisions latest
LEFT JOIN review_context_work_heads parent
    ON parent.review_context_id = sqlc.narg(parent_context_id)
    AND parent.work_id = latest.work_id
WHERE latest.pipeline_run_id = sqlc.arg(pipeline_run_id)
  AND latest.producer_stage = 'normalize'
  AND latest.id = (SELECT MAX(normalized_candidate.id) FROM work_revisions normalized_candidate
      WHERE normalized_candidate.pipeline_run_id = latest.pipeline_run_id
        AND normalized_candidate.work_id = latest.work_id
        AND normalized_candidate.producer_stage = 'normalize')
  AND EXISTS (SELECT 1 FROM run_work_stages current_validation
      WHERE current_validation.pipeline_run_id = latest.pipeline_run_id
        AND current_validation.work_id = latest.work_id
        AND current_validation.stage_name = 'validate'
        AND current_validation.outcome = 'valid');

-- name: InsertReviewContextNoteHeads :exec
INSERT INTO review_context_note_heads (
    review_context_id,
    note_id,
    note_version_id
)
SELECT
    sqlc.arg(review_context_id),
    parent.note_id,
    parent.note_version_id
FROM review_context_note_heads parent
JOIN review_notes note ON note.id = parent.note_id
JOIN review_context_work_heads target
    ON target.review_context_id = sqlc.arg(review_context_id)
    AND target.work_id = note.work_id
WHERE parent.review_context_id = sqlc.arg(parent_context_id);

-- name: InsertReviewContextAnchorHeads :exec
INSERT INTO review_context_anchor_heads (
    review_context_id,
    anchor_id,
    anchor_version_id
)
SELECT
    sqlc.arg(review_context_id),
    parent.anchor_id,
    parent.anchor_version_id
FROM review_context_anchor_heads parent
JOIN review_anchors anchor ON anchor.id = parent.anchor_id
JOIN review_context_work_heads target
    ON target.review_context_id = sqlc.arg(review_context_id)
    AND target.work_id = anchor.work_id
WHERE parent.review_context_id = sqlc.arg(parent_context_id);
