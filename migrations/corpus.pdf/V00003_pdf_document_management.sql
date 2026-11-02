-- ==UP==

-- 1. Add change_counter to document registrations.
ALTER TABLE pdf_documents ADD COLUMN change_counter INTEGER NOT NULL DEFAULT 1 CHECK (change_counter >= 1);

-- 2. Protect registration identity and exactly-once counter advancement.
CREATE TRIGGER pdf_documents_abort_delete
BEFORE DELETE ON pdf_documents
BEGIN
    SELECT RAISE(ABORT, 'pdf_documents registrations cannot be deleted');
END;

CREATE TRIGGER pdf_documents_advance_counter
BEFORE UPDATE ON pdf_documents
BEGIN
    SELECT RAISE(ABORT, 'pdf_documents.change_counter must advance by exactly one when the selection changes')
    WHERE (NEW.content_hash IS NOT OLD.content_hash OR NEW.status IS NOT OLD.status)
      AND NEW.change_counter != OLD.change_counter + 1;

    SELECT RAISE(ABORT, 'pdf_documents.change_counter must not advance when the selection is unchanged')
    WHERE NEW.content_hash IS OLD.content_hash
      AND NEW.status IS OLD.status
      AND NEW.change_counter != OLD.change_counter;
END;

-- 3. Add the content-hash index for bounded checks during deletion.
CREATE INDEX idx_pdf_documents_content_hash ON pdf_documents(content_hash);

-- 4. Add nullable JSON-validated before/after columns to the outbox.
ALTER TABLE pdf_audit_outbox ADD COLUMN before_json TEXT CHECK (before_json IS NULL OR json_valid(before_json));
ALTER TABLE pdf_audit_outbox ADD COLUMN after_json TEXT CHECK (after_json IS NULL OR json_valid(after_json));

-- 5. Rebuild only pdf_download_attempts to remove its blob-retention foreign key.
-- Capture the current AUTOINCREMENT sequence so rebuilt IDs remain monotonic.
CREATE TEMP TABLE pdf_download_attempts_sequence AS
SELECT seq FROM sqlite_sequence WHERE name = 'pdf_download_attempts';

CREATE TABLE pdf_download_attempts_new (
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
    content_hash  TEXT  -- logical hash reference; no foreign key to pdf_blobs
);

INSERT INTO pdf_download_attempts_new
    (id, gather_run_id, doi, source, resolved_url, started_at, completed_at, outcome, http_status, error_class, error_message, content_hash)
SELECT id, gather_run_id, doi, source, resolved_url, started_at, completed_at, outcome, http_status, error_class, error_message, content_hash
FROM pdf_download_attempts;

DROP TABLE pdf_download_attempts;
ALTER TABLE pdf_download_attempts_new RENAME TO pdf_download_attempts;

UPDATE sqlite_sequence
SET seq = (SELECT seq FROM pdf_download_attempts_sequence)
WHERE name = 'pdf_download_attempts';

DROP TABLE pdf_download_attempts_sequence;

CREATE INDEX idx_pdf_download_attempts_run
    ON pdf_download_attempts(gather_run_id, id);
CREATE INDEX idx_pdf_download_attempts_doi
    ON pdf_download_attempts(doi, id);
CREATE INDEX idx_pdf_download_attempts_source_outcome
    ON pdf_download_attempts(source, outcome);

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

-- The DOWN section is retained for emergency disaster recovery only. It is not
-- an automatic downgrade workflow. Restoring the legacy attempt-to-blob foreign
-- key must fail safely when historical hashes no longer have blobs.
DROP TRIGGER IF EXISTS pdf_download_attempts_abort_delete;
DROP TRIGGER IF EXISTS pdf_download_attempts_abort_update;
DROP INDEX IF EXISTS idx_pdf_download_attempts_source_outcome;
DROP INDEX IF EXISTS idx_pdf_download_attempts_doi;
DROP INDEX IF EXISTS idx_pdf_download_attempts_run;

CREATE TEMP TABLE pdf_download_attempts_sequence AS
SELECT seq FROM sqlite_sequence WHERE name = 'pdf_download_attempts';

CREATE TABLE pdf_download_attempts_legacy (
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

INSERT INTO pdf_download_attempts_legacy
    (id, gather_run_id, doi, source, resolved_url, started_at, completed_at, outcome, http_status, error_class, error_message, content_hash)
SELECT id, gather_run_id, doi, source, resolved_url, started_at, completed_at, outcome, http_status, error_class, error_message, content_hash
FROM pdf_download_attempts;

DROP TABLE pdf_download_attempts;
ALTER TABLE pdf_download_attempts_legacy RENAME TO pdf_download_attempts;

UPDATE sqlite_sequence
SET seq = (SELECT seq FROM pdf_download_attempts_sequence)
WHERE name = 'pdf_download_attempts';

DROP TABLE pdf_download_attempts_sequence;

CREATE INDEX idx_pdf_download_attempts_run
    ON pdf_download_attempts(gather_run_id, id);
CREATE INDEX idx_pdf_download_attempts_doi
    ON pdf_download_attempts(doi, id);
CREATE INDEX idx_pdf_download_attempts_source_outcome
    ON pdf_download_attempts(source, outcome);

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

DROP INDEX IF EXISTS idx_pdf_documents_content_hash;
DROP TRIGGER IF EXISTS pdf_documents_advance_counter;
DROP TRIGGER IF EXISTS pdf_documents_abort_delete;
ALTER TABLE pdf_documents DROP COLUMN change_counter;

ALTER TABLE pdf_audit_outbox DROP COLUMN after_json;
ALTER TABLE pdf_audit_outbox DROP COLUMN before_json;
