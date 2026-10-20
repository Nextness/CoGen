-- Static prepare-osf export statements. The copy, pragma, integrity, and
-- publication orchestration stays handwritten in main.go; these named queries
-- only replace the statement text and row scanning.

-- name: GetPDFStoreBinding :one
SELECT relative_path
FROM pdf_store_binding
WHERE id = 1;

-- name: RedactReviewers :exec
UPDATE pipeline_run_reviewers
SET username = '', email = '';

-- name: RegenerateReviewCorpusID :exec
UPDATE review_settings
SET corpus_id = lower(hex(randomblob(16)))
WHERE id = 1;

-- name: ListWorkspaceConfigArtifacts :many
SELECT DISTINCT artifact.id, artifact.content_hash, artifact.content_type,
    blob.pipeline_run_id, blob.data
FROM run_artifacts link
JOIN artifacts artifact ON artifact.id = link.artifact_id
JOIN artifact_blobs blob ON blob.artifact_id = artifact.id
WHERE link.artifact_role = 'workspace_config'
ORDER BY artifact.id;

-- name: GetArtifactByHash :one
SELECT id
FROM artifacts
WHERE content_hash = sqlc.arg(content_hash);

-- name: InsertArtifact :execresult
INSERT INTO artifacts (content_hash, byte_size, content_type)
VALUES (sqlc.arg(content_hash), sqlc.arg(byte_size), sqlc.arg(content_type));

-- name: InsertArtifactBlob :exec
INSERT INTO artifact_blobs (artifact_id, pipeline_run_id, data)
VALUES (sqlc.arg(artifact_id), sqlc.arg(pipeline_run_id), sqlc.arg(data));

-- name: RewireRunArtifacts :exec
UPDATE run_artifacts
SET artifact_id = sqlc.arg(new_artifact_id)
WHERE artifact_id = sqlc.arg(old_artifact_id);

-- name: RewireSearchRevisions :exec
UPDATE search_revisions
SET config_artifact_hash = sqlc.arg(new_hash)
WHERE config_artifact_hash = sqlc.arg(old_hash);

-- name: RewireRunStepInputs :exec
UPDATE run_steps
SET input_artifact_id = sqlc.arg(new_artifact_id), input_fingerprint = sqlc.arg(new_hash)
WHERE input_artifact_id = sqlc.arg(old_artifact_id);

-- name: RewireRunStepOutputs :exec
UPDATE run_steps
SET output_artifact_id = sqlc.arg(new_artifact_id), output_fingerprint = sqlc.arg(new_hash)
WHERE output_artifact_id = sqlc.arg(old_artifact_id);

-- name: CountArtifactReferences :one
SELECT
    (SELECT COUNT(*) FROM run_artifacts WHERE artifact_id = sqlc.arg(artifact_id)) +
    (SELECT COUNT(*) FROM run_steps WHERE input_artifact_id = sqlc.arg(artifact_id) OR output_artifact_id = sqlc.arg(artifact_id)) +
    (SELECT COUNT(*) FROM cache_entries WHERE payload_artifact_id = sqlc.arg(artifact_id)) +
    (SELECT COUNT(*) FROM author_identity_candidates WHERE payload_artifact_id = sqlc.arg(artifact_id));

-- name: DeleteArtifactBlob :exec
DELETE FROM artifact_blobs
WHERE artifact_id = sqlc.arg(artifact_id);

-- name: DeleteArtifact :exec
DELETE FROM artifacts
WHERE id = sqlc.arg(id);

-- name: ListArtifactBlobs :many
SELECT artifact.content_hash, artifact.byte_size, blob.data
FROM artifacts artifact
JOIN artifact_blobs blob ON blob.artifact_id = artifact.id;
