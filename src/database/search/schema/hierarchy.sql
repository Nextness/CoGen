-- Viewer hierarchy totals for the Home discovery summary. Counts planned and
-- completed attempts without transferring run lifecycle ownership.

-- name: GetHierarchyTotals :one
SELECT
    (SELECT COUNT(*) FROM searches) AS searches,
    (SELECT COUNT(*) FROM search_revisions) AS revisions,
    (SELECT COUNT(*) FROM execution_plans) AS plans,
    (SELECT COUNT(*) FROM pipeline_runs) AS runs,
    (SELECT COUNT(*) FROM pipeline_runs WHERE status='completed') AS completed_runs;
