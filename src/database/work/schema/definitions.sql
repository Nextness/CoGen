-- sqlc-only declarations for the work table family.
-- These statements exist solely to compile the family's generated queries.
-- They are never executed by production and never participate in runtime
-- migrations or SOMETHING database versioning. Column shapes follow the
-- production metadata migrations V00001, V00005, V00025, and V00027.
CREATE TABLE works (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    doi        TEXT UNIQUE,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE work_identifiers (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    work_id    INTEGER NOT NULL,
    namespace  TEXT NOT NULL,
    identifier TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (namespace, identifier)
);

CREATE TABLE work_revisions (
    id                   INTEGER PRIMARY KEY AUTOINCREMENT,
    work_id              INTEGER NOT NULL,
    pipeline_run_id      INTEGER NOT NULL,
    field_schema_version TEXT NOT NULL DEFAULT '1',
    payload_hash         TEXT NOT NULL,
    title                TEXT,
    abstract             TEXT,
    year                 INTEGER,
    journal              TEXT,
    publisher            TEXT,
    source               TEXT,
    keywords             TEXT,
    keywords_plus        TEXT,
    citation_count       INTEGER,
    reference_count      INTEGER,
    extension_data       TEXT,
    producer_stage       TEXT NOT NULL,
    created_at           TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE run_work_stages (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_run_id INTEGER NOT NULL,
    work_id         INTEGER NOT NULL,
    stage_name      TEXT NOT NULL,
    outcome         TEXT NOT NULL,
    reason          TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (pipeline_run_id, work_id, stage_name)
);

CREATE TABLE reference_mentions (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    work_revision_id INTEGER NOT NULL,
    resolved_work_id INTEGER,
    mention_order    INTEGER NOT NULL,
    raw_reference    TEXT,
    doi              TEXT,
    title            TEXT,
    author           TEXT,
    year             INTEGER,
    source           TEXT,
    created_at       TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (work_revision_id, mention_order)
);

CREATE TABLE run_search_terms (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_run_id INTEGER NOT NULL,
    source_name     TEXT NOT NULL,
    term            TEXT NOT NULL,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (pipeline_run_id, source_name, term)
);

CREATE TABLE work_revision_term_matches (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_run_id  INTEGER NOT NULL,
    work_revision_id INTEGER NOT NULL,
    field            TEXT NOT NULL CHECK (field IN ('title', 'abstract', 'keywords', 'keywords_plus')),
    term             TEXT NOT NULL,
    created_at       TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (pipeline_run_id, work_revision_id, field, term)
);

CREATE TABLE run_term_match_reconciliations (
    pipeline_run_id INTEGER PRIMARY KEY,
    reconciled_at   TEXT NOT NULL DEFAULT (datetime('now'))
);
