-- ==UP==
CREATE TABLE IF NOT EXISTS pipeline_runs (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    step                TEXT NOT NULL,
    started_at          TEXT NOT NULL,
    finished_at         TEXT,
    status              TEXT NOT NULL DEFAULT 'running',
    summary             TEXT,
    search_query        TEXT,
    execution_plan_id   INTEGER REFERENCES execution_plans(id),
    attempt_number      INTEGER,
    visibility_state    TEXT NOT NULL DEFAULT 'active',
    trashed_at          TEXT,
    trash_reason        TEXT,
    created_at          TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS articles (
    id                   INTEGER PRIMARY KEY AUTOINCREMENT,
    doi                  TEXT UNIQUE,
    title                TEXT,
    abstract             TEXT,
    year                 INTEGER,
    keywords             TEXT,
    keywords_additional  TEXT,
    journal              TEXT,
    publisher            TEXT,
    source               TEXT,
    citation_count       INTEGER,
    cited_references     TEXT,
    reference_count      INTEGER DEFAULT 0,
    validation_status    TEXT,
    validation_reasons   TEXT,
    normalized_journal   TEXT,
    pipeline_run_id      INTEGER REFERENCES pipeline_runs(id),
    created_at           TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS authors (
    id                      INTEGER PRIMARY KEY AUTOINCREMENT,
    orcid                   TEXT UNIQUE,
    first_name              TEXT,
    last_name               TEXT,
    citation_name           TEXT,
    normalized_name         TEXT,
    extra                   TEXT,
    display_name            TEXT,
    works_count             INTEGER DEFAULT 0,
    cited_by_count          INTEGER DEFAULT 0,
    h_index                 INTEGER DEFAULT 0,
    i10_index               INTEGER DEFAULT 0,
    source                  TEXT,
    latest_known_institution TEXT,
    pipeline_run_id         INTEGER REFERENCES pipeline_runs(id),
    created_at              TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS article_authors (
    article_id   INTEGER NOT NULL REFERENCES articles(id),
    author_id    INTEGER NOT NULL REFERENCES authors(id),
    affiliation  TEXT,
    author_order INTEGER NOT NULL,
    PRIMARY KEY (article_id, author_id)
);

CREATE TABLE IF NOT EXISTS article_references (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    article_id      INTEGER NOT NULL REFERENCES articles(id),
    ref_doi         TEXT,
    ref_title       TEXT,
    ref_author      TEXT,
    ref_year        INTEGER,
    ref_source      TEXT,
    ref_article_id  INTEGER REFERENCES articles(id),
    enriched        INTEGER DEFAULT 0,
    pipeline_run_id INTEGER REFERENCES pipeline_runs(id),
    UNIQUE(article_id, ref_doi)
);

CREATE TABLE IF NOT EXISTS enrichment_log (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    article_doi     TEXT NOT NULL,
    source          TEXT NOT NULL,
    field           TEXT NOT NULL,
    original_value  TEXT,
    enriched_value  TEXT,
    enriched_at     TEXT NOT NULL DEFAULT (datetime('now')),
    pipeline_run_id INTEGER REFERENCES pipeline_runs(id)
);

CREATE INDEX IF NOT EXISTS idx_enrichment_log_doi ON enrichment_log(article_doi);
CREATE INDEX IF NOT EXISTS idx_enrichment_log_source ON enrichment_log(source);

CREATE TABLE IF NOT EXISTS searches (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    search_id   TEXT NOT NULL UNIQUE,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS search_revisions (
    id                      INTEGER PRIMARY KEY AUTOINCREMENT,
    search_id               INTEGER NOT NULL REFERENCES searches(id),
    revision_label          TEXT NOT NULL,
    config_artifact_hash    TEXT NOT NULL,
    resolved_manifest_hash  TEXT NOT NULL,
    created_at              TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (search_id, revision_label)
);

CREATE INDEX IF NOT EXISTS idx_search_revisions_search_id ON search_revisions(search_id);

CREATE TABLE IF NOT EXISTS execution_plans (
    id                      INTEGER PRIMARY KEY AUTOINCREMENT,
    search_revision_id      INTEGER NOT NULL REFERENCES search_revisions(id),
    execution_fingerprint   TEXT NOT NULL,
    resolved_manifest_hash  TEXT NOT NULL,
    created_at              TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (search_revision_id, execution_fingerprint)
);

CREATE INDEX IF NOT EXISTS idx_execution_plans_fingerprint ON execution_plans(execution_fingerprint);

CREATE UNIQUE INDEX IF NOT EXISTS idx_pipeline_runs_attempt ON pipeline_runs(execution_plan_id, attempt_number);

CREATE TABLE IF NOT EXISTS run_sources (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_run_id   INTEGER NOT NULL REFERENCES pipeline_runs(id),
    source_name       TEXT NOT NULL,
    source_type       TEXT NOT NULL,
    expected_file     TEXT NOT NULL,
    query             TEXT,
    requested_fields  TEXT,
    created_at        TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (pipeline_run_id, source_name)
);

CREATE TABLE IF NOT EXISTS source_records (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    run_source_id   INTEGER NOT NULL REFERENCES run_sources(id),
    record_index    INTEGER NOT NULL,
    raw_payload     TEXT NOT NULL,
    content_hash    TEXT NOT NULL,
    parse_status    TEXT NOT NULL DEFAULT 'pending',
    reject_reason   TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_source_records_run_source ON source_records(run_source_id);
CREATE INDEX IF NOT EXISTS idx_source_records_hash ON source_records(content_hash);

CREATE TABLE IF NOT EXISTS artifacts (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    content_hash    TEXT NOT NULL UNIQUE,
    byte_size       INTEGER NOT NULL,
    content_type    TEXT NOT NULL,
    stored_path     TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS run_steps (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_run_id     INTEGER NOT NULL REFERENCES pipeline_runs(id),
    step_name           TEXT NOT NULL,
    step_status         TEXT NOT NULL DEFAULT 'pending',
    input_artifact_id   INTEGER REFERENCES artifacts(id),
    output_artifact_id  INTEGER REFERENCES artifacts(id),
    reused_from_run_id  INTEGER REFERENCES pipeline_runs(id),
    started_at          TEXT,
    finished_at         TEXT,
    UNIQUE (pipeline_run_id, step_name)
);

CREATE INDEX IF NOT EXISTS idx_run_steps_artifact_in ON run_steps(input_artifact_id);
CREATE INDEX IF NOT EXISTS idx_run_steps_artifact_out ON run_steps(output_artifact_id);

CREATE TABLE IF NOT EXISTS pipeline_run_metrics (
    pipeline_run_id INTEGER NOT NULL REFERENCES pipeline_runs(id),
    metric          TEXT NOT NULL,
    source          TEXT NOT NULL DEFAULT '',
    value           INTEGER NOT NULL CHECK (value >= 0),
    PRIMARY KEY (pipeline_run_id, metric, source)
);

CREATE TABLE IF NOT EXISTS audit_events (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    occurred_at     TEXT NOT NULL,
    actor           TEXT NOT NULL,
    pipeline_run_id INTEGER REFERENCES pipeline_runs(id),
    entity_type     TEXT NOT NULL,
    entity_id       TEXT NOT NULL,
    action          TEXT NOT NULL,
    before_json     TEXT,
    after_json      TEXT,
    metadata_json   TEXT,
    correlation_id  TEXT
);

CREATE INDEX IF NOT EXISTS idx_audit_events_run ON audit_events(pipeline_run_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_entity ON audit_events(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_action ON audit_events(action);
CREATE INDEX IF NOT EXISTS idx_audit_events_correlation ON audit_events(correlation_id);

CREATE TRIGGER IF NOT EXISTS audit_events_abort_update
BEFORE UPDATE ON audit_events
BEGIN
    SELECT RAISE(ABORT, 'audit_events is append-only: updates are not allowed');
END;

CREATE TRIGGER IF NOT EXISTS audit_events_abort_delete
BEFORE DELETE ON audit_events
BEGIN
    SELECT RAISE(ABORT, 'audit_events is append-only: deletes are not allowed');
END;

CREATE TABLE IF NOT EXISTS works (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    doi         TEXT UNIQUE,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS work_identifiers (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    work_id     INTEGER NOT NULL REFERENCES works(id),
    namespace   TEXT NOT NULL,
    identifier  TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (namespace, identifier)
);

CREATE INDEX IF NOT EXISTS idx_work_identifiers_work_id ON work_identifiers(work_id);

CREATE TABLE IF NOT EXISTS work_revisions (
    id                      INTEGER PRIMARY KEY AUTOINCREMENT,
    work_id                 INTEGER NOT NULL REFERENCES works(id),
    pipeline_run_id         INTEGER NOT NULL REFERENCES pipeline_runs(id),
    field_schema_version    TEXT NOT NULL DEFAULT '1',
    payload_hash            TEXT NOT NULL,
    title                   TEXT,
    abstract                TEXT,
    year                    INTEGER,
    journal                 TEXT,
    publisher               TEXT,
    source                  TEXT,
    keywords                TEXT,
    keywords_plus           TEXT,
    citation_count          INTEGER,
    reference_count         INTEGER,
    extension_data          TEXT,
    producer_stage          TEXT NOT NULL,
    created_at              TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_work_revisions_work_id ON work_revisions(work_id);
CREATE INDEX IF NOT EXISTS idx_work_revisions_run_id ON work_revisions(pipeline_run_id);

CREATE TRIGGER IF NOT EXISTS work_revisions_abort_update
BEFORE UPDATE ON work_revisions
BEGIN
    SELECT RAISE(ABORT, 'work_revisions is append-only: updates are not allowed');
END;

CREATE TRIGGER IF NOT EXISTS work_revisions_abort_delete
BEFORE DELETE ON work_revisions
BEGIN
    SELECT RAISE(ABORT, 'work_revisions is append-only: deletes are not allowed');
END;

CREATE TABLE IF NOT EXISTS run_work_stages (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_run_id   INTEGER NOT NULL REFERENCES pipeline_runs(id),
    work_id           INTEGER NOT NULL REFERENCES works(id),
    stage_name        TEXT NOT NULL,
    outcome           TEXT NOT NULL,
    reason            TEXT,
    created_at        TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at        TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (pipeline_run_id, work_id, stage_name)
);

CREATE INDEX IF NOT EXISTS idx_run_work_stages_run_id ON run_work_stages(pipeline_run_id);
CREATE INDEX IF NOT EXISTS idx_run_work_stages_work_id ON run_work_stages(work_id);

-- ==DOWN==
DROP TABLE IF EXISTS run_work_stages;
DROP TRIGGER IF EXISTS work_revisions_abort_delete;
DROP TRIGGER IF EXISTS work_revisions_abort_update;
DROP TABLE IF EXISTS work_revisions;
DROP TABLE IF EXISTS work_identifiers;
DROP TABLE IF EXISTS works;
DROP TRIGGER IF EXISTS audit_events_abort_delete;
DROP TRIGGER IF EXISTS audit_events_abort_update;
DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS pipeline_run_metrics;
DROP TABLE IF EXISTS run_steps;
DROP TABLE IF EXISTS artifacts;
DROP TABLE IF EXISTS source_records;
DROP TABLE IF EXISTS run_sources;
DROP TABLE IF EXISTS execution_plans;
DROP TABLE IF EXISTS search_revisions;
DROP TABLE IF EXISTS searches;
DROP TABLE IF EXISTS enrichment_log;
DROP TABLE IF EXISTS article_references;
DROP TABLE IF EXISTS article_authors;
DROP TABLE IF EXISTS authors;
DROP TABLE IF EXISTS articles;
DROP TABLE IF EXISTS pipeline_runs;