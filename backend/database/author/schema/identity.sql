-- name: InsertAuthorIdentityResolution :execresult
INSERT INTO author_identity_resolutions (
    pipeline_run_id,
    author_occurrence_id,
    status,
    provider,
    queried_citation_name,
    error_message,
    resolved_at
) VALUES (
    sqlc.arg(pipeline_run_id),
    sqlc.arg(author_occurrence_id),
    sqlc.arg(status),
    sqlc.arg(provider),
    sqlc.arg(queried_citation_name),
    sqlc.arg(error_message),
    sqlc.arg(resolved_at)
);
