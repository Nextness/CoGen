-- name: EvaluationReviewSummary :one
SELECT
    COUNT(*),
    CAST(COALESCE(SUM(CASE WHEN review.id IS NOT NULL AND review.status != 'not_evaluated' THEN 1 ELSE 0 END), 0) AS INTEGER),
    CAST(COALESCE(SUM(CASE WHEN review.id IS NULL OR review.status = 'not_evaluated' THEN 1 ELSE 0 END), 0) AS INTEGER),
    CAST(COALESCE(SUM(CASE WHEN work.doi IN (SELECT value FROM json_each(CAST(sqlc.arg(available_dois_json) AS TEXT))) THEN 1 ELSE 0 END), 0) AS INTEGER)
FROM work_revisions revision
JOIN works work ON work.id = revision.work_id
LEFT JOIN review_context_work_heads head ON head.review_context_id = sqlc.arg(context_id) AND head.work_id = revision.work_id
LEFT JOIN work_review_versions review ON review.id = head.review_version_id
WHERE revision.pipeline_run_id = sqlc.arg(pipeline_run_id)
  AND revision.producer_stage = 'normalize'
  AND revision.id = (SELECT MAX(normalized_candidate.id) FROM work_revisions normalized_candidate
      WHERE normalized_candidate.pipeline_run_id = revision.pipeline_run_id
        AND normalized_candidate.work_id = revision.work_id
        AND normalized_candidate.producer_stage = 'normalize')
  AND EXISTS (SELECT 1 FROM run_work_stages current_validation
      WHERE current_validation.pipeline_run_id = revision.pipeline_run_id
        AND current_validation.work_id = revision.work_id
        AND current_validation.stage_name = 'validate'
        AND current_validation.outcome = 'valid');

-- name: EvaluationStatusFacets :many
SELECT COALESCE(review.status, 'not_evaluated') AS value, COUNT(*) AS count
FROM work_revisions revision
JOIN works work ON work.id = revision.work_id
LEFT JOIN review_context_work_heads head ON head.review_context_id = sqlc.arg(context_id) AND head.work_id = revision.work_id
LEFT JOIN work_review_versions review ON review.id = head.review_version_id
WHERE revision.pipeline_run_id = sqlc.arg(pipeline_run_id)
  AND revision.producer_stage = 'normalize'
  AND revision.id = (SELECT MAX(normalized_candidate.id) FROM work_revisions normalized_candidate
      WHERE normalized_candidate.pipeline_run_id = revision.pipeline_run_id
        AND normalized_candidate.work_id = revision.work_id
        AND normalized_candidate.producer_stage = 'normalize')
  AND EXISTS (SELECT 1 FROM run_work_stages current_validation
      WHERE current_validation.pipeline_run_id = revision.pipeline_run_id
        AND current_validation.work_id = revision.work_id
        AND current_validation.stage_name = 'validate'
        AND current_validation.outcome = 'valid')
GROUP BY COALESCE(review.status, 'not_evaluated')
ORDER BY value;

-- name: EvaluationSourceFacets :many
SELECT CAST(COALESCE(NULLIF(revision.source, ''), 'not_recorded') AS TEXT) AS value, COUNT(*) AS count
FROM work_revisions revision
JOIN works work ON work.id = revision.work_id
LEFT JOIN review_context_work_heads head ON head.review_context_id = sqlc.arg(context_id) AND head.work_id = revision.work_id
LEFT JOIN work_review_versions review ON review.id = head.review_version_id
WHERE revision.pipeline_run_id = sqlc.arg(pipeline_run_id)
  AND revision.producer_stage = 'normalize'
  AND revision.id = (SELECT MAX(normalized_candidate.id) FROM work_revisions normalized_candidate
      WHERE normalized_candidate.pipeline_run_id = revision.pipeline_run_id
        AND normalized_candidate.work_id = revision.work_id
        AND normalized_candidate.producer_stage = 'normalize')
  AND EXISTS (SELECT 1 FROM run_work_stages current_validation
      WHERE current_validation.pipeline_run_id = revision.pipeline_run_id
        AND current_validation.work_id = revision.work_id
        AND current_validation.stage_name = 'validate'
        AND current_validation.outcome = 'valid')
GROUP BY COALESCE(NULLIF(revision.source, ''), 'not_recorded')
ORDER BY value;

-- name: EvaluationReviewSourceFacets :many
SELECT CAST(CASE
    WHEN review.id IS NULL THEN 'not_started'
    WHEN review.created_in_context_id = sqlc.arg(context_id) THEN 'this_context'
    ELSE 'inherited' END AS TEXT) AS value,
    COUNT(*) AS count
FROM work_revisions revision
JOIN works work ON work.id = revision.work_id
LEFT JOIN review_context_work_heads head ON head.review_context_id = sqlc.arg(context_id) AND head.work_id = revision.work_id
LEFT JOIN work_review_versions review ON review.id = head.review_version_id
WHERE revision.pipeline_run_id = sqlc.arg(pipeline_run_id)
  AND revision.producer_stage = 'normalize'
  AND revision.id = (SELECT MAX(normalized_candidate.id) FROM work_revisions normalized_candidate
      WHERE normalized_candidate.pipeline_run_id = revision.pipeline_run_id
        AND normalized_candidate.work_id = revision.work_id
        AND normalized_candidate.producer_stage = 'normalize')
  AND EXISTS (SELECT 1 FROM run_work_stages current_validation
      WHERE current_validation.pipeline_run_id = revision.pipeline_run_id
        AND current_validation.work_id = revision.work_id
        AND current_validation.stage_name = 'validate'
        AND current_validation.outcome = 'valid')
GROUP BY value
ORDER BY value;

-- name: EvaluationQualifierFacets :many
SELECT sub.sub_status AS value, COUNT(*) AS count
FROM work_revisions revision
JOIN review_context_work_heads head ON head.review_context_id = sqlc.arg(context_id) AND head.work_id = revision.work_id
JOIN work_review_version_substatuses sub ON sub.review_version_id = head.review_version_id
WHERE revision.pipeline_run_id = sqlc.arg(pipeline_run_id)
  AND revision.producer_stage = 'normalize'
  AND revision.id = (SELECT MAX(normalized_candidate.id) FROM work_revisions normalized_candidate
      WHERE normalized_candidate.pipeline_run_id = revision.pipeline_run_id
        AND normalized_candidate.work_id = revision.work_id
        AND normalized_candidate.producer_stage = 'normalize')
  AND EXISTS (SELECT 1 FROM run_work_stages current_validation
      WHERE current_validation.pipeline_run_id = revision.pipeline_run_id
        AND current_validation.work_id = revision.work_id
        AND current_validation.stage_name = 'validate'
        AND current_validation.outcome = 'valid')
GROUP BY sub.sub_status
ORDER BY sub.sub_status;
