-- sqlc-only declarations for the generator feature probe.
-- The probe reads and writes the production cache_entries table so it can run
-- against a database created by the production migration runner. These
-- statements are never executed by production and never participate in runtime
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
