-- ==UP==
CREATE TABLE pdf_documents_inventory (
    doi            TEXT PRIMARY KEY CHECK (length(trim(doi)) > 0),
    status         TEXT NOT NULL CHECK (status IN ('not_available', 'available')),
    content_hash   TEXT REFERENCES pdf_blobs(content_hash),
    inventoried_at TEXT,
    updated_at     TEXT NOT NULL,
    CHECK (
        (status = 'available' AND content_hash IS NOT NULL AND inventoried_at IS NOT NULL) OR
        (status = 'not_available' AND content_hash IS NULL AND inventoried_at IS NULL)
    )
);

INSERT INTO pdf_documents_inventory (doi, status, content_hash, inventoried_at, updated_at)
SELECT doi,
       CASE WHEN status = 'downloaded' THEN 'available' ELSE 'not_available' END,
       CASE WHEN status = 'downloaded' THEN content_hash ELSE NULL END,
       CASE WHEN status = 'downloaded' THEN downloaded_at ELSE NULL END,
       updated_at
FROM pdf_documents;

DROP TABLE pdf_documents;
ALTER TABLE pdf_documents_inventory RENAME TO pdf_documents;
ALTER TABLE pdf_audit_outbox ADD COLUMN pipeline_run_id INTEGER;

-- ==DOWN==
ALTER TABLE pdf_audit_outbox DROP COLUMN pipeline_run_id;

CREATE TABLE pdf_documents_legacy (
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

INSERT INTO pdf_documents_legacy
    (doi, status, content_hash, source, downloaded_at, updated_at)
SELECT doi,
       CASE WHEN status = 'available' THEN 'downloaded' ELSE 'not_downloaded' END,
       content_hash,
       CASE WHEN status = 'available' THEN 'manual' ELSE NULL END,
       inventoried_at,
       updated_at
FROM pdf_documents;

DROP TABLE pdf_documents;
ALTER TABLE pdf_documents_legacy RENAME TO pdf_documents;
