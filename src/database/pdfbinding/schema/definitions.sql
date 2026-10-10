-- sqlc-only declarations for the metadata PDF-binding family.
-- These statements exist solely to compile the family's generated queries.
-- They are never executed by production and never participate in runtime
-- migrations or SOMETHING database versioning. Column shapes follow the
-- production metadata migrations V00001, V00019, V00020, and V00021.
CREATE TABLE pdf_store_binding (
    id                 INTEGER PRIMARY KEY,
    relative_path      TEXT NOT NULL,
    configured_at      TEXT NOT NULL,
    config_fingerprint TEXT NOT NULL
);

CREATE TABLE pdf_audit_links (
    event_key      TEXT PRIMARY KEY,
    audit_event_id INTEGER NOT NULL,
    created_at     TEXT NOT NULL
);

-- The audit and run tables below are declared only so the delivered-event
-- relationship queries can be static named queries. The audit family owns
-- audit_events and the run family owns pipeline_runs.
CREATE TABLE audit_events (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    occurred_at     TEXT NOT NULL,
    actor           TEXT NOT NULL,
    pipeline_run_id INTEGER,
    entity_type     TEXT NOT NULL,
    entity_id       TEXT NOT NULL,
    action          TEXT NOT NULL,
    metadata_json   TEXT,
    correlation_id  TEXT
);

CREATE TABLE pipeline_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT
);
