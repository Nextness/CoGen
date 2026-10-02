-- sqlc-only declarations for the cache table family.
-- These statements exist solely to compile the family's generated queries.
-- They are never executed by production and never participate in runtime
-- migrations or SOMETHING database versioning.
CREATE TABLE cache_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider TEXT NOT NULL,
    namespace TEXT NOT NULL,
    request_fingerprint TEXT NOT NULL,
    response_status INTEGER NOT NULL,
    payload_artifact_id INTEGER,
    fetched_at TEXT NOT NULL,
    expires_at TEXT,
    extractor_version TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE run_cache_uses (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_run_id INTEGER NOT NULL,
    cache_entry_id INTEGER NOT NULL,
    cache_layer TEXT NOT NULL,
    outcome TEXT NOT NULL,
    used_at TEXT NOT NULL DEFAULT (datetime('now'))
);
