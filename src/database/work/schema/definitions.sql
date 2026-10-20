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

-- External relationship tables declared only so the family's work-focused
-- relationship reads compile. The author, review, and audit families own these
-- tables; the work family never writes them. Column shapes follow the
-- production metadata migrations V00001, V00002, V00023, and V00024.
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

CREATE TABLE author_occurrences (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    person_id     INTEGER,
    citation_name TEXT NOT NULL,
    first_name    TEXT,
    last_name     TEXT,
    orcid         TEXT,
    created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE people (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    orcid      TEXT UNIQUE,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE review_contexts (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_run_id   INTEGER NOT NULL UNIQUE,
    parent_context_id INTEGER,
    created_at        TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE work_review_versions (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    work_id               INTEGER NOT NULL,
    work_revision_id      INTEGER NOT NULL,
    created_in_context_id INTEGER NOT NULL,
    parent_version_id     INTEGER,
    status                TEXT NOT NULL CHECK (status IN ('not_evaluated', 'in_progress', 'approved', 'not_approved', 'removed')),
    reason                TEXT CHECK (reason IS NULL OR length(reason) <= 32768),
    created_at            TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE work_review_version_substatuses (
    review_version_id INTEGER NOT NULL,
    sub_status        TEXT NOT NULL CHECK (sub_status IN ('redacted', 'unrelated', 'out_of_scope', 'duplicate', 'retracted', 'withdrawn', 'superseded', 'predatory_low_quality', 'copyright_licensing', 'not_peer_reviewed')),
    PRIMARY KEY (review_version_id, sub_status)
);

CREATE TABLE review_context_work_heads (
    review_context_id INTEGER NOT NULL,
    work_id           INTEGER NOT NULL,
    work_revision_id  INTEGER NOT NULL,
    review_version_id INTEGER,
    PRIMARY KEY (review_context_id, work_id),
    UNIQUE (review_context_id, work_revision_id)
);

CREATE TABLE review_notes (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    work_id    INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE review_note_versions (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    note_id               INTEGER NOT NULL,
    parent_version_id     INTEGER,
    created_in_context_id INTEGER NOT NULL,
    state                 TEXT NOT NULL CHECK (state IN ('active', 'deleted')),
    body                  TEXT,
    created_at            TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE review_anchors (
    id         TEXT PRIMARY KEY,
    work_id    INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE review_anchor_versions (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    anchor_id             TEXT NOT NULL,
    parent_version_id     INTEGER,
    created_in_context_id INTEGER NOT NULL,
    work_revision_id      INTEGER NOT NULL,
    pdf_content_hash      TEXT NOT NULL CHECK (length(pdf_content_hash) = 64),
    state                 TEXT NOT NULL CHECK (state IN ('active', 'deleted')),
    page                  INTEGER,
    selected_text         TEXT,
    rectangles_json       TEXT,
    created_at            TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE audit_events (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    occurred_at     TEXT NOT NULL,
    actor           TEXT NOT NULL,
    pipeline_run_id INTEGER,
    entity_type     TEXT NOT NULL,
    entity_id       TEXT NOT NULL,
    action          TEXT NOT NULL,
    before_json     TEXT,
    after_json      TEXT,
    metadata_json   TEXT,
    correlation_id  TEXT
);
