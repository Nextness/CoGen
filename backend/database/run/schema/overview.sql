-- name: GetLatestRunWithAncestry :one
SELECT
    pr.id,
    pr.attempt_number,
    pr.started_at,
    pr.finished_at,
    pr.status,
    pr.visibility_state,
    s.id AS search_id,
    s.search_id AS search_name,
    sr.id AS search_revision_id,
    sr.revision_label,
    ep.id AS execution_plan_id
FROM pipeline_runs pr
LEFT JOIN execution_plans ep ON ep.id=pr.execution_plan_id
LEFT JOIN search_revisions sr ON sr.id=ep.search_revision_id
LEFT JOIN searches s ON s.id=sr.search_id
ORDER BY pr.id DESC
LIMIT 1;

-- name: GetSelectedAttempt :one
SELECT
    id,
    attempt_number,
    started_at,
    status,
    visibility_state
FROM pipeline_runs
WHERE id = sqlc.arg(id)
  AND execution_plan_id = sqlc.arg(execution_plan_id)
  AND visibility_state != 'trashed';

-- name: GetRunArtifactContext :one
SELECT
    s.search_id,
    sr.id AS search_revision_id,
    sr.revision_label,
    ep.id AS execution_plan_id,
    ep.execution_fingerprint,
    pr.id AS run_id,
    pr.attempt_number
FROM pipeline_runs pr
JOIN execution_plans ep ON ep.id=pr.execution_plan_id
JOIN search_revisions sr ON sr.id=ep.search_revision_id
JOIN searches s ON s.id=sr.search_id
WHERE pr.id = sqlc.arg(id);

-- name: ListHierarchyAttempts :many
SELECT
    id,
    execution_plan_id,
    attempt_number,
    started_at,
    finished_at,
    status,
    visibility_state
FROM pipeline_runs
WHERE execution_plan_id = sqlc.arg('execution_plan_id')
  AND visibility_state != 'trashed'
  AND (CAST(sqlc.arg('query') AS TEXT)='' OR CAST(id AS TEXT) LIKE CAST(sqlc.arg('pattern') AS TEXT)
    OR LOWER(status) LIKE CAST(sqlc.arg('pattern') AS TEXT) OR LOWER(started_at) LIKE CAST(sqlc.arg('pattern') AS TEXT))
  AND (CAST(sqlc.arg('cursor') AS INTEGER)=0 OR id < sqlc.arg('cursor'))
ORDER BY id DESC
LIMIT sqlc.arg('limit');

-- name: ListHierarchyRuns :many
SELECT
    pr.id,
    pr.attempt_number,
    pr.started_at,
    pr.finished_at,
    pr.status,
    pr.visibility_state,
    s.id AS search_id,
    s.search_id AS search_name,
    sr.id AS search_revision_id,
    sr.revision_label,
    ep.id AS execution_plan_id
FROM pipeline_runs pr
LEFT JOIN execution_plans ep ON ep.id=pr.execution_plan_id
LEFT JOIN search_revisions sr ON sr.id=ep.search_revision_id
LEFT JOIN searches s ON s.id=sr.search_id
WHERE (CAST(sqlc.arg('query') AS TEXT)='' OR LOWER(s.search_id) LIKE sqlc.arg('pattern')
    OR LOWER(sr.revision_label) LIKE sqlc.arg('pattern') OR CAST(pr.id AS TEXT) LIKE sqlc.arg('pattern'))
  AND (CAST(sqlc.arg('visibility') AS TEXT)='all'
    OR (CAST(sqlc.arg('visibility') AS TEXT)='active' AND pr.visibility_state!='trashed')
    OR (CAST(sqlc.arg('visibility') AS TEXT)='trashed' AND pr.visibility_state='trashed'))
  AND (CAST(sqlc.arg('status') AS TEXT)='' OR CAST(sqlc.arg('status') AS TEXT)='all' OR pr.status=sqlc.arg('status'))
  AND (CAST(sqlc.arg('started_after') AS TEXT)='' OR datetime(pr.started_at) >= datetime(sqlc.arg('started_after')))
  AND (CAST(sqlc.arg('started_before') AS TEXT)='' OR datetime(pr.started_at) < datetime(sqlc.arg('started_before')))
  AND (CAST(sqlc.arg('cursor') AS INTEGER)=0 OR pr.id < sqlc.arg('cursor'))
ORDER BY pr.id DESC
LIMIT sqlc.arg('limit');

-- name: ListLegacyRuns :many
SELECT
    pr.id,
    pr.step,
    pr.started_at,
    pr.finished_at,
    pr.status,
    pr.summary,
    pr.search_query,
    pr.execution_plan_id,
    pr.attempt_number,
    pr.visibility_state,
    pr.trashed_at,
    pr.trash_reason,
    ep.search_revision_id
FROM pipeline_runs pr
LEFT JOIN execution_plans ep ON ep.id=pr.execution_plan_id
WHERE (CAST(sqlc.arg('search_revision_id') AS INTEGER)=0 OR ep.search_revision_id=sqlc.arg('search_revision_id'))
  AND (CAST(sqlc.arg('plan_id') AS INTEGER)=0 OR pr.execution_plan_id=sqlc.arg('plan_id'))
  AND (CAST(sqlc.arg('include_trashed') AS INTEGER)=1 OR pr.visibility_state!='trashed')
ORDER BY pr.id DESC
LIMIT sqlc.arg('limit');
