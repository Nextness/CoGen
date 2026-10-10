-- Viewer hierarchy totals for the Home discovery summary. Counts planned and
-- completed attempts without transferring run lifecycle ownership.

-- name: GetHierarchyTotals :one
SELECT
    (SELECT COUNT(*) FROM searches) AS searches,
    (SELECT COUNT(*) FROM search_revisions) AS revisions,
    (SELECT COUNT(*) FROM execution_plans) AS plans,
    (SELECT COUNT(*) FROM pipeline_runs) AS runs,
    (SELECT COUNT(*) FROM pipeline_runs WHERE status='completed') AS completed_runs;

-- name: ListHierarchySearches :many
SELECT s.id, s.search_id, s.created_at,
    COUNT(DISTINCT sr.id) AS revision_count,
    COUNT(DISTINCT ep.id) AS plan_count,
    COUNT(DISTINCT pr.id) AS run_count,
    MAX(pr.id) AS latest_run_id,
    MAX((SELECT latest_pr.execution_plan_id FROM pipeline_runs latest_pr
        JOIN execution_plans latest_ep ON latest_ep.id=latest_pr.execution_plan_id
        JOIN search_revisions latest_sr ON latest_sr.id=latest_ep.search_revision_id
        WHERE latest_sr.search_id=s.id ORDER BY latest_pr.id DESC LIMIT 1)) AS latest_plan_id,
    MAX((SELECT latest_ep.search_revision_id FROM pipeline_runs latest_pr
        JOIN execution_plans latest_ep ON latest_ep.id=latest_pr.execution_plan_id
        JOIN search_revisions latest_sr ON latest_sr.id=latest_ep.search_revision_id
        WHERE latest_sr.search_id=s.id ORDER BY latest_pr.id DESC LIMIT 1)) AS latest_revision_id
FROM searches s
LEFT JOIN search_revisions sr ON sr.search_id=s.id
LEFT JOIN execution_plans ep ON ep.search_revision_id=sr.id
LEFT JOIN pipeline_runs pr ON pr.execution_plan_id=ep.id
WHERE (CAST(sqlc.arg('query') AS TEXT)='' OR LOWER(s.search_id) LIKE sqlc.arg('pattern') OR EXISTS (
        SELECT 1 FROM search_revisions matched_sr
        WHERE matched_sr.search_id=s.id AND LOWER(matched_sr.revision_label) LIKE sqlc.arg('pattern')))
  AND (CAST(sqlc.arg('cursor') AS INTEGER)=0 OR s.id < sqlc.arg('cursor'))
GROUP BY s.id
ORDER BY s.id DESC
LIMIT sqlc.arg('limit');

-- name: ListHierarchyRevisions :many
SELECT sr.id, sr.revision_label, sr.created_at,
    COUNT(DISTINCT ep.id) AS plan_count,
    COUNT(DISTINCT pr.id) AS run_count,
    MAX(pr.id) AS latest_run_id,
    MAX((SELECT latest_pr.execution_plan_id FROM pipeline_runs latest_pr
        JOIN execution_plans latest_ep ON latest_ep.id=latest_pr.execution_plan_id
        WHERE latest_ep.search_revision_id=sr.id ORDER BY latest_pr.id DESC LIMIT 1)) AS latest_plan_id
FROM search_revisions sr
LEFT JOIN execution_plans ep ON ep.search_revision_id=sr.id
LEFT JOIN pipeline_runs pr ON pr.execution_plan_id=ep.id
WHERE sr.search_id=sqlc.arg('search_id')
  AND (CAST(sqlc.arg('query') AS TEXT)='' OR LOWER(sr.revision_label) LIKE sqlc.arg('pattern') OR CAST(sr.id AS TEXT) LIKE sqlc.arg('pattern'))
  AND (CAST(sqlc.arg('cursor') AS INTEGER)=0 OR sr.id < sqlc.arg('cursor'))
GROUP BY sr.id
ORDER BY sr.id DESC
LIMIT sqlc.arg('limit');

-- name: ListHierarchyPlans :many
SELECT id, search_revision_id, execution_fingerprint, enrichment_enabled, created_at
FROM execution_plans
WHERE search_revision_id=sqlc.arg('search_revision_id')
  AND (CAST(sqlc.arg('query') AS TEXT)='' OR LOWER(execution_fingerprint) LIKE sqlc.arg('pattern') OR CAST(id AS TEXT) LIKE sqlc.arg('pattern'))
  AND (CAST(sqlc.arg('cursor') AS INTEGER)=0 OR id < sqlc.arg('cursor'))
ORDER BY id DESC
LIMIT sqlc.arg('limit');
