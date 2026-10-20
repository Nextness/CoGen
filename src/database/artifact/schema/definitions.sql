-- sqlc-only declarations for the artifact table family.
-- These statements exist solely to compile the family's generated queries.
-- They are never executed by production and never participate in runtime
-- migrations or SOMETHING database versioning. Column shapes follow the
-- production metadata migrations V00001, V00008, V00011, V00012, V00015,
-- V00016, and V00028.
CREATE TABLE artifacts (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    content_hash TEXT NOT NULL UNIQUE,
    byte_size    INTEGER NOT NULL,
    content_type TEXT NOT NULL,
    created_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE artifact_blobs (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    artifact_id     INTEGER NOT NULL UNIQUE,
    pipeline_run_id INTEGER NOT NULL,
    data            BLOB NOT NULL,
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE run_artifacts (
    pipeline_run_id INTEGER NOT NULL,
    artifact_id     INTEGER NOT NULL,
    artifact_role   TEXT NOT NULL,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (pipeline_run_id, artifact_role)
);

CREATE TABLE run_steps (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_run_id    INTEGER NOT NULL,
    step_name          TEXT NOT NULL,
    step_status        TEXT NOT NULL DEFAULT 'pending',
    input_artifact_id  INTEGER,
    output_artifact_id INTEGER,
    reused_from_run_id INTEGER,
    started_at         TEXT,
    finished_at        TEXT,
    input_fingerprint  TEXT NOT NULL DEFAULT '',
    output_fingerprint TEXT NOT NULL DEFAULT '',
    UNIQUE (pipeline_run_id, step_name)
);

CREATE TABLE cache_entries (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    provider            TEXT NOT NULL,
    namespace           TEXT NOT NULL,
    request_fingerprint TEXT NOT NULL,
    response_status     INTEGER NOT NULL,
    payload_artifact_id INTEGER,
    fetched_at          TEXT NOT NULL,
    expires_at          TEXT,
    extractor_version   TEXT NOT NULL,
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at          TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE run_cache_uses (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_run_id INTEGER NOT NULL,
    cache_entry_id  INTEGER NOT NULL,
    cache_layer     TEXT NOT NULL,
    outcome         TEXT NOT NULL,
    used_at         TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE author_identity_resolutions (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_run_id       INTEGER NOT NULL,
    author_occurrence_id  INTEGER NOT NULL,
    status                TEXT NOT NULL,
    provider              TEXT NOT NULL,
    queried_citation_name TEXT NOT NULL,
    error_message         TEXT,
    resolved_at           TEXT NOT NULL,
    created_at            TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (pipeline_run_id, author_occurrence_id, provider)
);

CREATE TABLE author_identity_candidates (
    id                     INTEGER PRIMARY KEY AUTOINCREMENT,
    identity_resolution_id INTEGER NOT NULL,
    candidate_orcid        TEXT NOT NULL,
    provider_display_name  TEXT,
    query_url              TEXT NOT NULL,
    payload_artifact_id    INTEGER,
    provider_rank          INTEGER NOT NULL,
    created_at             TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (identity_resolution_id, candidate_orcid),
    UNIQUE (identity_resolution_id, provider_rank)
);
