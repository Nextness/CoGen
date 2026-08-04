-- ==UP==
-- Versioned, content-addressed provider responses shared by all workspace
-- runs. Cache policy evaluation is implemented separately; these tables only
-- provide durable atomic storage and run-use provenance.
CREATE TABLE IF NOT EXISTS cache_entries (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    provider          TEXT NOT NULL,
    namespace         TEXT NOT NULL,
    request_fingerprint TEXT NOT NULL,
    response_status   INTEGER NOT NULL,
    payload_artifact_id INTEGER REFERENCES artifacts(id),
    fetched_at        TEXT NOT NULL,
    expires_at        TEXT,
    extractor_version TEXT NOT NULL,
    created_at        TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at        TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (provider, namespace, request_fingerprint, extractor_version)
);

CREATE INDEX IF NOT EXISTS idx_cache_entries_expiry ON cache_entries(expires_at);
CREATE INDEX IF NOT EXISTS idx_cache_entries_payload ON cache_entries(payload_artifact_id);

CREATE TABLE IF NOT EXISTS run_cache_uses (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_run_id INTEGER NOT NULL REFERENCES pipeline_runs(id),
    cache_entry_id  INTEGER NOT NULL REFERENCES cache_entries(id),
    cache_layer     TEXT NOT NULL,
    outcome         TEXT NOT NULL,
    used_at         TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_run_cache_uses_run ON run_cache_uses(pipeline_run_id);
CREATE INDEX IF NOT EXISTS idx_run_cache_uses_entry ON run_cache_uses(cache_entry_id);
-- ==DOWN==
