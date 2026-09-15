-- ==UP==
-- Preserve IDs and all retained payloads while removing request-key uniqueness.
-- Rebuild the child before dropping its former parent so foreign keys stay enabled.
ALTER TABLE cache_entries RENAME TO previous_cache_entries;
CREATE TABLE cache_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider TEXT NOT NULL,
    namespace TEXT NOT NULL,
    request_fingerprint TEXT NOT NULL,
    response_status INTEGER NOT NULL,
    payload_artifact_id INTEGER REFERENCES artifacts(id),
    fetched_at TEXT NOT NULL,
    expires_at TEXT,
    extractor_version TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
INSERT INTO cache_entries SELECT * FROM previous_cache_entries;
CREATE TABLE preserved_run_cache_uses (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_run_id INTEGER NOT NULL REFERENCES pipeline_runs(id),
    cache_entry_id INTEGER NOT NULL REFERENCES cache_entries(id),
    cache_layer TEXT NOT NULL,
    outcome TEXT NOT NULL,
    used_at TEXT NOT NULL DEFAULT (datetime('now'))
);
INSERT INTO preserved_run_cache_uses SELECT * FROM run_cache_uses;
DROP TABLE run_cache_uses;
DROP TABLE previous_cache_entries;
ALTER TABLE preserved_run_cache_uses RENAME TO run_cache_uses;
CREATE INDEX idx_run_cache_uses_run ON run_cache_uses(pipeline_run_id);
CREATE INDEX idx_run_cache_uses_entry ON run_cache_uses(cache_entry_id);
CREATE INDEX idx_cache_entries_expiry ON cache_entries(expires_at);
CREATE INDEX idx_cache_entries_payload ON cache_entries(payload_artifact_id);
CREATE INDEX idx_cache_entries_request ON cache_entries(provider, namespace, request_fingerprint, extractor_version, id);
CREATE TRIGGER cache_entries_immutable BEFORE UPDATE ON cache_entries
BEGIN
    SELECT RAISE(ABORT, 'cache responses are immutable');
END;
-- ==DOWN==
-- Response history cannot be collapsed back into unique request rows without losing evidence.
