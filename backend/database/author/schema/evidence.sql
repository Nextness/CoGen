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

-- name: ListIdentityCandidates :many
SELECT
    id,
    candidate_orcid,
    provider_display_name,
    query_url,
    payload_artifact_id,
    provider_rank,
    created_at
FROM author_identity_candidates
WHERE identity_resolution_id = sqlc.arg('resolution_id')
  AND (CAST(sqlc.arg('cursor_id') AS INTEGER)=0
    OR COALESCE(provider_rank, 0) > sqlc.arg('cursor_rank')
    OR (COALESCE(provider_rank, 0) = sqlc.arg('cursor_rank') AND id > sqlc.arg('cursor_id')))
ORDER BY COALESCE(provider_rank, 0), id
LIMIT sqlc.arg('limit');

-- name: ListCandidatePreviews :many
SELECT
    preview.identity_resolution_id,
    preview.id,
    preview.candidate_orcid,
    preview.provider_display_name,
    preview.query_url,
    preview.payload_artifact_id,
    preview.provider_rank,
    preview.created_at
FROM (
    SELECT
        candidate.id,
        candidate.identity_resolution_id,
        candidate.candidate_orcid,
        candidate.provider_display_name,
        candidate.query_url,
        candidate.payload_artifact_id,
        candidate.provider_rank,
        candidate.created_at,
        ROW_NUMBER() OVER (PARTITION BY candidate.identity_resolution_id ORDER BY candidate.provider_rank, candidate.id) AS candidate_row
    FROM author_identity_candidates candidate
    WHERE candidate.identity_resolution_id IN (SELECT value FROM json_each(CAST(sqlc.arg('resolution_ids_json') AS TEXT)))
) preview
WHERE preview.candidate_row <= CAST(sqlc.arg('limit') AS INTEGER)
ORDER BY preview.identity_resolution_id, preview.provider_rank, preview.id;

-- name: CountAuthorIdentityEvidence :one
SELECT COUNT(DISTINCT r.id)
FROM author_identity_resolutions r
WHERE r.pipeline_run_id = sqlc.arg('pipeline_run_id')
  AND (r.author_occurrence_id = sqlc.arg('author_occurrence_id') OR EXISTS (
    SELECT 1 FROM authorships target_authorship
    JOIN author_occurrences target_author ON target_author.id=target_authorship.author_occurrence_id
    JOIN work_revisions target_revision ON target_revision.id=target_authorship.work_revision_id
    JOIN work_revisions evidence_revision ON evidence_revision.work_id=target_revision.work_id
        AND evidence_revision.pipeline_run_id=target_revision.pipeline_run_id
        AND evidence_revision.id<=target_revision.id
    JOIN authorships evidence_authorship ON evidence_authorship.work_revision_id=evidence_revision.id
        AND evidence_authorship.author_order=target_authorship.author_order
    JOIN author_occurrences evidence_author ON evidence_author.id=evidence_authorship.author_occurrence_id
    WHERE target_author.id = sqlc.arg('author_occurrence_id')
      AND target_revision.pipeline_run_id = r.pipeline_run_id
      AND evidence_author.id = r.author_occurrence_id
      AND evidence_author.citation_name IS target_author.citation_name
      AND evidence_author.first_name IS target_author.first_name
      AND evidence_author.last_name IS target_author.last_name
      AND evidence_author.orcid IS target_author.orcid));

-- name: ListAuthorIdentityEvidence :many
SELECT
    r.id AS resolution_id,
    r.id,
    r.pipeline_run_id,
    r.status,
    r.provider,
    r.queried_citation_name,
    r.error_message,
    r.resolved_at,
    COUNT(c.id) AS candidate_count
FROM author_identity_resolutions r
LEFT JOIN author_identity_candidates c ON c.identity_resolution_id=r.id
WHERE r.pipeline_run_id = sqlc.arg('pipeline_run_id')
  AND (r.author_occurrence_id = sqlc.arg('author_occurrence_id') OR EXISTS (
    SELECT 1 FROM authorships target_authorship
    JOIN author_occurrences target_author ON target_author.id=target_authorship.author_occurrence_id
    JOIN work_revisions target_revision ON target_revision.id=target_authorship.work_revision_id
    JOIN work_revisions evidence_revision ON evidence_revision.work_id=target_revision.work_id
        AND evidence_revision.pipeline_run_id=target_revision.pipeline_run_id
        AND evidence_revision.id<=target_revision.id
    JOIN authorships evidence_authorship ON evidence_authorship.work_revision_id=evidence_revision.id
        AND evidence_authorship.author_order=target_authorship.author_order
    JOIN author_occurrences evidence_author ON evidence_author.id=evidence_authorship.author_occurrence_id
    WHERE target_author.id = sqlc.arg('author_occurrence_id')
      AND target_revision.pipeline_run_id = r.pipeline_run_id
      AND evidence_author.id = r.author_occurrence_id
      AND evidence_author.citation_name IS target_author.citation_name
      AND evidence_author.first_name IS target_author.first_name
      AND evidence_author.last_name IS target_author.last_name
      AND evidence_author.orcid IS target_author.orcid))
  AND (CAST(sqlc.arg('cursor_id') AS INTEGER)=0 OR r.id < sqlc.arg('cursor_id'))
GROUP BY r.id
ORDER BY r.id DESC
LIMIT sqlc.arg('row_limit');
