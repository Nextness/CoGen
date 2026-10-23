-- name: CurrentCoverage :one
SELECT
    COUNT(*),
    CAST(COALESCE(SUM(CASE WHEN journal IS NOT NULL AND journal != '' THEN 1 ELSE 0 END), 0) AS INTEGER)
FROM work_revisions
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id);

-- name: CountRunWorkRevisions :one
SELECT COUNT(*) FROM work_revisions
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id);

-- name: CountRunAnalysisReadyArticles :one
SELECT COUNT(*) FROM work_revisions wr
WHERE wr.pipeline_run_id = sqlc.arg(pipeline_run_id)
  AND wr.producer_stage = 'normalize'
  AND wr.id = (SELECT MAX(normalized_candidate.id) FROM work_revisions normalized_candidate
      WHERE normalized_candidate.pipeline_run_id = wr.pipeline_run_id
        AND normalized_candidate.work_id = wr.work_id
        AND normalized_candidate.producer_stage = 'normalize')
  AND EXISTS (SELECT 1 FROM run_work_stages current_validation
      WHERE current_validation.pipeline_run_id = wr.pipeline_run_id
        AND current_validation.work_id = wr.work_id
        AND current_validation.stage_name = 'validate'
        AND current_validation.outcome = 'valid');

-- name: CountRunAuthorships :one
SELECT COUNT(*) FROM authorships a
JOIN work_revisions wr ON wr.id = a.work_revision_id
WHERE wr.pipeline_run_id = sqlc.arg(pipeline_run_id)
  AND wr.producer_stage = 'normalize'
  AND wr.id = (SELECT MAX(normalized_candidate.id) FROM work_revisions normalized_candidate
      WHERE normalized_candidate.pipeline_run_id = wr.pipeline_run_id
        AND normalized_candidate.work_id = wr.work_id
        AND normalized_candidate.producer_stage = 'normalize')
  AND EXISTS (SELECT 1 FROM run_work_stages current_validation
      WHERE current_validation.pipeline_run_id = wr.pipeline_run_id
        AND current_validation.work_id = wr.work_id
        AND current_validation.stage_name = 'validate'
        AND current_validation.outcome = 'valid');

-- name: CountRunReferenceMentions :one
SELECT COUNT(*) FROM reference_mentions rm
JOIN work_revisions wr ON wr.id = rm.work_revision_id
WHERE wr.pipeline_run_id = sqlc.arg(pipeline_run_id)
  AND wr.producer_stage = 'normalize'
  AND wr.id = (SELECT MAX(normalized_candidate.id) FROM work_revisions normalized_candidate
      WHERE normalized_candidate.pipeline_run_id = wr.pipeline_run_id
        AND normalized_candidate.work_id = wr.work_id
        AND normalized_candidate.producer_stage = 'normalize')
  AND EXISTS (SELECT 1 FROM run_work_stages current_validation
      WHERE current_validation.pipeline_run_id = wr.pipeline_run_id
        AND current_validation.work_id = wr.work_id
        AND current_validation.stage_name = 'validate'
        AND current_validation.outcome = 'valid');

-- name: CountRunInternalCitations :one
SELECT COUNT(*) FROM reference_mentions rm
JOIN work_revisions wr ON wr.id = rm.work_revision_id
WHERE wr.pipeline_run_id = sqlc.arg(pipeline_run_id)
  AND rm.resolved_work_id IS NOT NULL
  AND wr.producer_stage = 'normalize'
  AND wr.id = (SELECT MAX(normalized_candidate.id) FROM work_revisions normalized_candidate
      WHERE normalized_candidate.pipeline_run_id = wr.pipeline_run_id
        AND normalized_candidate.work_id = wr.work_id
        AND normalized_candidate.producer_stage = 'normalize')
  AND EXISTS (SELECT 1 FROM run_work_stages current_validation
      WHERE current_validation.pipeline_run_id = wr.pipeline_run_id
        AND current_validation.work_id = wr.work_id
        AND current_validation.stage_name = 'validate'
        AND current_validation.outcome = 'valid');

-- name: RunStageSummaries :many
SELECT
    stage_name,
    outcome,
    COUNT(*) AS count,
    CAST(MIN(created_at) AS TEXT) AS first_recorded_at,
    CAST(MAX(updated_at) AS TEXT) AS last_recorded_at
FROM run_work_stages
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id)
GROUP BY stage_name, outcome
ORDER BY stage_name, outcome;
