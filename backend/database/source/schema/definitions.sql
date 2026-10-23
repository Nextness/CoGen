-- sqlc-only declarations for the source table family.
-- These statements exist solely to compile the family's generated queries.
-- They are never executed by production and never participate in runtime
-- migrations or SOMETHING database versioning. Column shapes follow the
-- production metadata migrations V00001, V00014, V00017, and V00018.
CREATE TABLE run_sources (
    id                      INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_run_id         INTEGER NOT NULL,
    source_name             TEXT NOT NULL,
    source_type             TEXT NOT NULL,
    expected_file           TEXT NOT NULL,
    query                   TEXT,
    requested_fields        TEXT,
    expected_result_count   INTEGER,
    observed_result_count   INTEGER,
    result_count_comparison TEXT,
    export_date             TEXT,
    created_at              TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (pipeline_run_id, source_name)
);

CREATE TABLE source_records (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    run_source_id INTEGER NOT NULL,
    record_index  INTEGER NOT NULL,
    raw_payload   TEXT NOT NULL,
    content_hash  TEXT NOT NULL,
    parse_status  TEXT NOT NULL DEFAULT 'pending',
    reject_reason TEXT,
    created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE source_filter_counts (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_run_id INTEGER NOT NULL,
    source_name     TEXT NOT NULL,
    filter_data     TEXT NOT NULL DEFAULT '[]',
    UNIQUE (pipeline_run_id, source_name)
);
