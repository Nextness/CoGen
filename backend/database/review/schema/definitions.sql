-- sqlc-only declarations for the review table family.
-- These statements exist solely to compile the family's generated queries.
-- They are never executed by production and never participate in runtime
-- migrations or SOMETHING database versioning. Column shapes follow the
-- production metadata migrations V00001, V00005, V00022, V00023, V00024,
-- V00026, and V00027, including the V00026 anchor label column.
CREATE TABLE review_settings (
    id         INTEGER PRIMARY KEY CHECK (id = 1),
    corpus_id  TEXT NOT NULL,
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

CREATE TABLE review_context_note_heads (
    review_context_id INTEGER NOT NULL,
    note_id           INTEGER NOT NULL,
    note_version_id   INTEGER NOT NULL,
    PRIMARY KEY (review_context_id, note_id)
);

CREATE TABLE review_note_links (
    note_version_id INTEGER NOT NULL,
    ordinal         INTEGER NOT NULL CHECK (ordinal >= 1),
    target_type     TEXT NOT NULL CHECK (target_type IN ('note', 'article', 'pdf_page', 'anchor', 'ext')),
    raw_target      TEXT NOT NULL CHECK (length(raw_target) <= 2048),
    display_text    TEXT CHECK (display_text IS NULL OR length(display_text) <= 1024),
    utf16_position  INTEGER NOT NULL CHECK (utf16_position >= 0),
    utf16_length    INTEGER NOT NULL CHECK (utf16_length > 0),
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (note_version_id, ordinal)
);

CREATE TABLE review_anchors (
    id         TEXT PRIMARY KEY,
    work_id    INTEGER NOT NULL,
    label      TEXT,
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

CREATE TABLE review_context_anchor_heads (
    review_context_id INTEGER NOT NULL,
    anchor_id         TEXT NOT NULL,
    anchor_version_id INTEGER NOT NULL,
    PRIMARY KEY (review_context_id, anchor_id)
);

CREATE TABLE pipeline_run_reviewers (
    pipeline_run_id INTEGER PRIMARY KEY NOT NULL,
    username        TEXT NOT NULL DEFAULT '' CHECK (length(username) <= 200),
    email           TEXT NOT NULL DEFAULT '' CHECK (length(email) <= 320),
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
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

-- External relationship tables declared only so the family's review context,
-- decision, note, anchor, and backlink reads compile. The search, run, and
-- work families own these tables; the review family never writes them. Column
-- shapes follow the production metadata migrations V00001 and V00005.
CREATE TABLE pipeline_runs (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at        TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'running',
    execution_plan_id INTEGER,
    attempt_number    INTEGER,
    visibility_state  TEXT NOT NULL DEFAULT 'active'
);

CREATE TABLE execution_plans (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    search_revision_id INTEGER NOT NULL
);

CREATE TABLE search_revisions (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    search_id      INTEGER NOT NULL,
    revision_label TEXT NOT NULL
);

CREATE TABLE searches (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    search_id TEXT NOT NULL UNIQUE
);

CREATE TABLE works (
    id  INTEGER PRIMARY KEY AUTOINCREMENT,
    doi TEXT UNIQUE
);

CREATE TABLE work_revisions (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    work_id         INTEGER NOT NULL,
    pipeline_run_id INTEGER NOT NULL,
    producer_stage  TEXT NOT NULL
);

CREATE TABLE run_work_stages (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_run_id INTEGER NOT NULL,
    work_id         INTEGER NOT NULL,
    stage_name      TEXT NOT NULL,
    outcome         TEXT NOT NULL
);
