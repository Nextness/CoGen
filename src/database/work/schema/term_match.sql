-- name: UpsertRunTermReconciliation :exec
INSERT INTO run_term_match_reconciliations (pipeline_run_id)
VALUES (sqlc.arg(pipeline_run_id))
ON CONFLICT (pipeline_run_id)
DO UPDATE SET reconciled_at = datetime('now');

-- name: HasRunTermData :one
SELECT EXISTS(
    SELECT 1 FROM run_term_match_reconciliations
    WHERE pipeline_run_id = sqlc.arg(pipeline_run_id)
);

-- name: ListRunSearchTermsByRun :many
SELECT
    id,
    pipeline_run_id,
    source_name,
    term,
    created_at
FROM run_search_terms
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id)
ORDER BY id;

-- name: ListRevisionTermMatches :many
SELECT field, term
FROM work_revision_term_matches
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id)
  AND work_revision_id = sqlc.arg(work_revision_id)
ORDER BY id;

-- name: ListRevisionTermMatchesBulk :many
SELECT work_revision_id, field, term
FROM work_revision_term_matches
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id)
  AND work_revision_id IN (sqlc.slice(revision_ids))
ORDER BY id;

-- name: DeleteRunSearchTerms :exec
DELETE FROM run_search_terms WHERE pipeline_run_id = sqlc.arg(pipeline_run_id);

-- name: InsertRunSearchTerm :exec
INSERT OR IGNORE INTO run_search_terms (pipeline_run_id, source_name, term)
VALUES (
    sqlc.arg(pipeline_run_id),
    sqlc.arg(source_name),
    sqlc.arg(term)
);

-- name: DeleteRevisionTermMatches :exec
DELETE FROM work_revision_term_matches WHERE pipeline_run_id = sqlc.arg(pipeline_run_id);

-- name: InsertRevisionTermMatch :exec
INSERT OR IGNORE INTO work_revision_term_matches (pipeline_run_id, work_revision_id, field, term)
VALUES (
    sqlc.arg(pipeline_run_id),
    sqlc.arg(work_revision_id),
    sqlc.arg(field),
    sqlc.arg(term)
);
