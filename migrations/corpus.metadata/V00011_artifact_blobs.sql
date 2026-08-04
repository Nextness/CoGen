-- ==UP==
-- Artifact blob storage: content-addressed blob data, stored inline in the
-- database. One row per unique artifact (deduplicated via artifact_id, matching
-- the artifacts table's content_hash UNIQUE invariant). The ./artifacts/
-- directory and artifacts.stored_path column are no longer supported after this
-- migration.
CREATE TABLE IF NOT EXISTS artifact_blobs (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    artifact_id     INTEGER NOT NULL UNIQUE REFERENCES artifacts(id),
    pipeline_run_id INTEGER NOT NULL REFERENCES pipeline_runs(id),
    data            BLOB NOT NULL,
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_artifact_blobs_run     ON artifact_blobs(pipeline_run_id);
CREATE INDEX IF NOT EXISTS idx_artifact_blobs_artifact ON artifact_blobs(artifact_id);
-- ==DOWN==
DROP TABLE IF EXISTS artifact_blobs;
