-- ==UP==
CREATE TABLE pdf_blobs (
    content_hash TEXT PRIMARY KEY CHECK (length(content_hash) = 64),
    byte_size    INTEGER NOT NULL CHECK (byte_size > 0),
    data         BLOB NOT NULL CHECK (length(data) = byte_size),
    created_at   TEXT NOT NULL
);

CREATE TABLE pdf_gather_runs (
    id                   INTEGER PRIMARY KEY AUTOINCREMENT,
    correlation_id       TEXT NOT NULL UNIQUE,
    started_at           TEXT NOT NULL,
    completed_at         TEXT,
    status               TEXT NOT NULL CHECK (status IN ('running', 'completed', 'failed')),
    settings_fingerprint TEXT NOT NULL CHECK (length(settings_fingerprint) = 64),
    settings_json        TEXT NOT NULL CHECK (json_valid(settings_json)),
    selected_workspaces  TEXT NOT NULL CHECK (json_valid(selected_workspaces))
);

CREATE TABLE pdf_documents (
    doi                TEXT PRIMARY KEY CHECK (length(trim(doi)) > 0),
    status             TEXT NOT NULL CHECK (status IN ('not_downloaded', 'attempting', 'downloaded', 'failed')),
    content_hash       TEXT REFERENCES pdf_blobs(content_hash),
    source             TEXT,
    error_class        TEXT,
    error_message      TEXT CHECK (error_message IS NULL OR length(error_message) <= 1000),
    attempt_started_at TEXT,
    lease_token        TEXT,
    downloaded_at      TEXT,
    verified_at        TEXT,
    updated_at         TEXT NOT NULL,
    CHECK (
        (status = 'downloaded' AND content_hash IS NOT NULL AND downloaded_at IS NOT NULL) OR
        (status != 'downloaded' AND content_hash IS NULL)
    ),
    CHECK (
        (status = 'attempting' AND attempt_started_at IS NOT NULL AND lease_token IS NOT NULL) OR
        (status != 'attempting' AND lease_token IS NULL)
    )
);

CREATE TABLE pdf_download_attempts (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    gather_run_id INTEGER NOT NULL REFERENCES pdf_gather_runs(id),
    doi           TEXT NOT NULL,
    source        TEXT,
    resolved_url  TEXT CHECK (resolved_url IS NULL OR length(resolved_url) <= 2048),
    started_at    TEXT NOT NULL,
    completed_at  TEXT,
    outcome       TEXT NOT NULL,
    http_status   INTEGER,
    error_class   TEXT,
    error_message TEXT CHECK (error_message IS NULL OR length(error_message) <= 1000),
    content_hash  TEXT REFERENCES pdf_blobs(content_hash)
);

CREATE INDEX idx_pdf_download_attempts_run
    ON pdf_download_attempts(gather_run_id, id);
CREATE INDEX idx_pdf_download_attempts_doi
    ON pdf_download_attempts(doi, id);
CREATE INDEX idx_pdf_download_attempts_source_outcome
    ON pdf_download_attempts(source, outcome);

CREATE TABLE pdf_audit_outbox (
    event_key      TEXT PRIMARY KEY,
    occurred_at    TEXT NOT NULL,
    actor          TEXT NOT NULL,
    entity_type    TEXT NOT NULL,
    entity_id      TEXT NOT NULL,
    action         TEXT NOT NULL,
    metadata_json  TEXT NOT NULL CHECK (json_valid(metadata_json)),
    correlation_id TEXT NOT NULL,
    delivered_at   TEXT
);

CREATE INDEX idx_pdf_audit_outbox_undelivered
    ON pdf_audit_outbox(delivered_at, occurred_at);

CREATE TRIGGER pdf_blobs_abort_update
BEFORE UPDATE ON pdf_blobs
BEGIN
    SELECT RAISE(ABORT, 'pdf_blobs is immutable: updates are not allowed');
END;

CREATE TRIGGER pdf_download_attempts_abort_update
BEFORE UPDATE ON pdf_download_attempts
BEGIN
    SELECT RAISE(ABORT, 'pdf_download_attempts is append-only: updates are not allowed');
END;

CREATE TRIGGER pdf_download_attempts_abort_delete
BEFORE DELETE ON pdf_download_attempts
BEGIN
    SELECT RAISE(ABORT, 'pdf_download_attempts is append-only: deletes are not allowed');
END;

-- ==DOWN==
DROP TRIGGER IF EXISTS pdf_download_attempts_abort_delete;
DROP TRIGGER IF EXISTS pdf_download_attempts_abort_update;
DROP TRIGGER IF EXISTS pdf_blobs_abort_update;
DROP TABLE IF EXISTS pdf_audit_outbox;
DROP INDEX IF EXISTS idx_pdf_download_attempts_source_outcome;
DROP INDEX IF EXISTS idx_pdf_download_attempts_doi;
DROP INDEX IF EXISTS idx_pdf_download_attempts_run;
DROP TABLE IF EXISTS pdf_download_attempts;
DROP TABLE IF EXISTS pdf_documents;
DROP TABLE IF EXISTS pdf_gather_runs;
DROP TABLE IF EXISTS pdf_blobs;
