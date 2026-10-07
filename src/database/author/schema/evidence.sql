-- name: InsertAuthorIdentityCandidate :execresult
INSERT INTO author_identity_candidates (
    identity_resolution_id,
    candidate_orcid,
    provider_display_name,
    query_url,
    payload_artifact_id,
    provider_rank
) VALUES (
    sqlc.arg(identity_resolution_id),
    sqlc.arg(candidate_orcid),
    sqlc.arg(provider_display_name),
    sqlc.arg(query_url),
    sqlc.arg(payload_artifact_id),
    sqlc.arg(provider_rank)
);
