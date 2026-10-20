-- sqlc-only declarations for the prepare-osf export adapter.
-- These statements exist solely to compile the tool's generated export
-- queries. They are never executed by production and never participate in
-- runtime migrations or SOMETHING database versioning. Column shapes follow
-- the production metadata migrations V00001 through V00028.
CREATE TABLE pdf_store_binding (
    id            INTEGER PRIMARY KEY,
    relative_path TEXT NOT NULL
);

CREATE TABLE pipeline_run_reviewers (
    pipeline_run_id INTEGER PRIMARY KEY NOT NULL,
    username        TEXT NOT NULL,
    email           TEXT NOT NULL
);

CREATE TABLE review_settings (
    id        INTEGER PRIMARY KEY,
    corpus_id TEXT NOT NULL
);

CREATE TABLE artifacts (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    content_hash TEXT NOT NULL,
    byte_size    INTEGER NOT NULL,
    content_type TEXT NOT NULL
);

CREATE TABLE artifact_blobs (
    artifact_id     INTEGER NOT NULL,
    pipeline_run_id INTEGER NOT NULL,
    data            BLOB NOT NULL
);

CREATE TABLE run_artifacts (
    pipeline_run_id INTEGER NOT NULL,
    artifact_id     INTEGER NOT NULL,
    artifact_role   TEXT NOT NULL
);

CREATE TABLE search_revisions (
    id                   INTEGER PRIMARY KEY AUTOINCREMENT,
    config_artifact_hash TEXT NOT NULL
);

CREATE TABLE run_steps (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    input_artifact_id  INTEGER,
    input_fingerprint  TEXT NOT NULL,
    output_artifact_id INTEGER,
    output_fingerprint TEXT NOT NULL
);

CREATE TABLE cache_entries (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    payload_artifact_id INTEGER
);

CREATE TABLE author_identity_candidates (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    payload_artifact_id INTEGER
);
