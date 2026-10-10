-- sqlc-only declarations for the audit table family.
-- These statements exist solely to compile the family's generated queries.
-- They are never executed by production and never participate in runtime
-- migrations or SOMETHING database versioning. Column shapes follow the
-- production metadata migrations V00001 and V00020.
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

CREATE TABLE work_revisions (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    work_id         INTEGER NOT NULL,
    pipeline_run_id INTEGER NOT NULL
);

-- The review tables below are declared only so the article detail event
-- predicate can be a static named query. Column shapes follow the production
-- metadata migrations V00020 through V00028.
CREATE TABLE review_contexts (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_run_id INTEGER NOT NULL
);

CREATE TABLE work_review_versions (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    work_id          INTEGER NOT NULL,
    work_revision_id INTEGER NOT NULL
);

CREATE TABLE review_notes (
    id      INTEGER PRIMARY KEY AUTOINCREMENT,
    work_id INTEGER NOT NULL
);

CREATE TABLE review_note_versions (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    note_id               INTEGER NOT NULL,
    created_in_context_id INTEGER NOT NULL
);

CREATE TABLE review_anchors (
    id      TEXT PRIMARY KEY,
    work_id INTEGER NOT NULL
);

CREATE TABLE review_anchor_versions (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    anchor_id             TEXT NOT NULL,
    created_in_context_id INTEGER NOT NULL
);
