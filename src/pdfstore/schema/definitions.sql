-- sqlc-only declarations for the companion PDF store.
-- These statements exist solely to compile the companion read queries.
-- They are never executed by production and never participate in runtime
-- migrations or SOMETHING database versioning. Column shapes follow the
-- production companion migrations V00001 and V00002.
CREATE TABLE pdf_documents (
    doi            TEXT PRIMARY KEY,
    status         TEXT NOT NULL,
    content_hash   TEXT,
    inventoried_at TEXT,
    updated_at     TEXT NOT NULL
);

CREATE TABLE pdf_blobs (
    content_hash TEXT PRIMARY KEY,
    byte_size    INTEGER NOT NULL,
    data         BLOB NOT NULL,
    created_at   TEXT NOT NULL
);

CREATE TABLE pdf_audit_outbox (
    event_key       TEXT PRIMARY KEY,
    occurred_at     TEXT NOT NULL,
    actor           TEXT NOT NULL,
    pipeline_run_id INTEGER,
    entity_type     TEXT NOT NULL,
    entity_id       TEXT NOT NULL,
    action          TEXT NOT NULL,
    metadata_json   TEXT NOT NULL,
    correlation_id  TEXT NOT NULL,
    delivered_at    TEXT
);
