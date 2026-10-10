-- sqlc-only declarations for the author table family.
-- These statements exist solely to compile the family's generated queries.
-- They are never executed by production and never participate in runtime
-- migrations or SOMETHING database versioning. Column shapes follow the
-- production metadata migrations V00002 and V00016.
CREATE TABLE people (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    orcid      TEXT UNIQUE,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE author_occurrences (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    person_id     INTEGER,
    citation_name TEXT NOT NULL,
    first_name    TEXT,
    last_name     TEXT,
    orcid         TEXT,
    created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE authorships (
    id                   INTEGER PRIMARY KEY AUTOINCREMENT,
    work_revision_id     INTEGER NOT NULL,
    author_occurrence_id INTEGER NOT NULL,
    author_order         INTEGER NOT NULL,
    affiliation          TEXT,
    created_at           TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (work_revision_id, author_occurrence_id),
    UNIQUE (work_revision_id, author_order)
);

CREATE TABLE author_identity_resolutions (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_run_id       INTEGER NOT NULL,
    author_occurrence_id  INTEGER NOT NULL,
    status                TEXT NOT NULL CHECK (status IN (
        'orcid_is_unclear', 'no_orcid_candidate', 'provider_failed',
        'confirmed', 'rejected'
    )),
    provider              TEXT NOT NULL,
    queried_citation_name TEXT NOT NULL,
    error_message         TEXT,
    resolved_at           TEXT NOT NULL,
    created_at            TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (pipeline_run_id, author_occurrence_id, provider)
);

CREATE TABLE author_identity_candidates (
    id                      INTEGER PRIMARY KEY AUTOINCREMENT,
    identity_resolution_id  INTEGER NOT NULL,
    candidate_orcid         TEXT NOT NULL,
    provider_display_name   TEXT,
    query_url               TEXT NOT NULL,
    payload_artifact_id     INTEGER,
    provider_rank           INTEGER NOT NULL CHECK (provider_rank >= 1),
    created_at              TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (identity_resolution_id, candidate_orcid),
    UNIQUE (identity_resolution_id, provider_rank)
);

-- External immutable revision table declared only so run-scoped author
-- membership and identity-evidence reads can compile. The work family owns
-- revision writes.
CREATE TABLE work_revisions (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    work_id         INTEGER NOT NULL,
    pipeline_run_id INTEGER NOT NULL
);
