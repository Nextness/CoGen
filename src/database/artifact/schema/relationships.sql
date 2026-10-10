-- name: ListRunArtifacts :many
WITH artifact_relationships AS (
    SELECT ra.artifact_id, 'run_role' AS relationship_role, ra.artifact_role AS relationship_detail
    FROM run_artifacts ra
    WHERE ra.pipeline_run_id = sqlc.arg(pipeline_run_id)
    UNION ALL
    SELECT rs.input_artifact_id, 'step_input', rs.step_name
    FROM run_steps rs
    WHERE rs.pipeline_run_id = sqlc.arg(pipeline_run_id)
      AND rs.input_artifact_id IS NOT NULL
    UNION ALL
    SELECT rs.output_artifact_id, 'step_output', rs.step_name
    FROM run_steps rs
    WHERE rs.pipeline_run_id = sqlc.arg(pipeline_run_id)
      AND rs.output_artifact_id IS NOT NULL
    UNION ALL
    SELECT ce.payload_artifact_id, 'cache_payload', ce.provider || ':' || ce.namespace
    FROM run_cache_uses use_record
    JOIN cache_entries ce ON ce.id = use_record.cache_entry_id
    WHERE use_record.pipeline_run_id = sqlc.arg(pipeline_run_id)
      AND ce.payload_artifact_id IS NOT NULL
    UNION ALL
    SELECT candidate.payload_artifact_id, 'identity_candidate_payload', resolution.provider
    FROM author_identity_resolutions resolution
    JOIN author_identity_candidates candidate ON candidate.identity_resolution_id = resolution.id
    WHERE resolution.pipeline_run_id = sqlc.arg(pipeline_run_id)
      AND candidate.payload_artifact_id IS NOT NULL
)
SELECT
    a.id,
    a.content_hash,
    a.byte_size,
    a.content_type,
    a.created_at,
    relationship.relationship_role,
    relationship.relationship_detail
FROM artifact_relationships relationship
JOIN artifacts a ON a.id = relationship.artifact_id
ORDER BY a.id, relationship.relationship_role, relationship.relationship_detail;
