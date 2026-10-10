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

-- name: GetIdentityEvidenceStats :one
SELECT
    CAST(COUNT(*) AS INTEGER) AS resolutions,
    CAST(COALESCE(SUM(CASE WHEN status='orcid_is_unclear' THEN 1 ELSE 0 END), 0) AS INTEGER) AS unclear,
    CAST(COALESCE(SUM(CASE WHEN status='no_orcid_candidate' THEN 1 ELSE 0 END), 0) AS INTEGER) AS no_candidate,
    CAST(COALESCE(SUM(CASE WHEN status='provider_failed' THEN 1 ELSE 0 END), 0) AS INTEGER) AS provider_failed
FROM author_identity_resolutions
WHERE pipeline_run_id = sqlc.arg(pipeline_run_id);

-- name: CountIdentityCandidatesByRun :one
SELECT COUNT(*)
FROM author_identity_candidates candidate
JOIN author_identity_resolutions resolution ON resolution.id=candidate.identity_resolution_id
WHERE resolution.pipeline_run_id = sqlc.arg(pipeline_run_id);

-- name: IdentityResolutionExists :one
SELECT 1
FROM author_identity_resolutions
WHERE id = sqlc.arg(id)
  AND pipeline_run_id = sqlc.arg(pipeline_run_id);
