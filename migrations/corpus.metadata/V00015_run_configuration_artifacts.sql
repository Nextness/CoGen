-- ==UP==
-- Link each attempt to the exact workspace configuration, resolved manifest,
-- and input manifest it used. Artifacts are content-addressed and can be
-- shared, while these links retain the attempt-specific provenance role.
CREATE TABLE run_artifacts (
    pipeline_run_id INTEGER NOT NULL REFERENCES pipeline_runs(id),
    artifact_id      INTEGER NOT NULL REFERENCES artifacts(id),
    artifact_role    TEXT NOT NULL CHECK (artifact_role IN (
        'workspace_config', 'resolved_manifest', 'input_manifest'
    )),
    created_at       TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (pipeline_run_id, artifact_role)
);

CREATE INDEX idx_run_artifacts_artifact ON run_artifacts(artifact_id);

-- ==DOWN==
DROP TABLE run_artifacts;
