-- sqlc-only declarations for the run table family.
-- These statements exist solely to compile the family's generated queries.
-- They are never executed by production and never participate in runtime
-- migrations or SOMETHING database versioning. Column shapes follow the
-- production metadata migrations V00001, V00008, and V00022.
CREATE TABLE pipeline_runs (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    step              TEXT NOT NULL,
    started_at        TEXT NOT NULL,
    finished_at       TEXT,
    status            TEXT NOT NULL DEFAULT 'running',
    summary           TEXT,
    search_query      TEXT,
    execution_plan_id INTEGER,
    attempt_number    INTEGER,
    visibility_state  TEXT NOT NULL DEFAULT 'active',
    trashed_at        TEXT,
    trash_reason      TEXT,
    created_at        TEXT DEFAULT (datetime('now'))
);

CREATE UNIQUE INDEX idx_pipeline_runs_attempt ON pipeline_runs(execution_plan_id, attempt_number);

CREATE TABLE pipeline_run_reviewers (
    pipeline_run_id INTEGER PRIMARY KEY NOT NULL,
    username        TEXT NOT NULL DEFAULT '' CHECK (length(username) <= 200),
    email           TEXT NOT NULL DEFAULT '' CHECK (length(email) <= 320),
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
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

CREATE TABLE pipeline_run_metrics (
    pipeline_run_id INTEGER NOT NULL,
    metric          TEXT NOT NULL,
    source          TEXT NOT NULL DEFAULT '',
    value           INTEGER NOT NULL CHECK (value >= 0),
    PRIMARY KEY (pipeline_run_id, metric, source)
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

CREATE TABLE review_contexts (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_run_id   INTEGER NOT NULL UNIQUE,
    parent_context_id INTEGER,
    created_at        TEXT NOT NULL DEFAULT (datetime('now'))
);

-- External search ancestry tables declared only so run overview and lifecycle
-- reads can return complete lineage. The search family owns their writes.
CREATE TABLE searches (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    search_id  TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE search_revisions (
    id                   INTEGER PRIMARY KEY AUTOINCREMENT,
    search_id            INTEGER NOT NULL,
    revision_label       TEXT NOT NULL,
    config_artifact_hash TEXT NOT NULL,
    resolved_manifest_hash TEXT NOT NULL,
    created_at           TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at           TEXT
);

CREATE TABLE execution_plans (
    id                   INTEGER PRIMARY KEY AUTOINCREMENT,
    search_revision_id   INTEGER NOT NULL,
    execution_fingerprint TEXT NOT NULL,
    resolved_manifest_hash TEXT NOT NULL,
    input_manifest_hash  TEXT NOT NULL DEFAULT '',
    enrichment_enabled   INTEGER NOT NULL DEFAULT 0,
    created_at           TEXT NOT NULL DEFAULT (datetime('now'))
);
