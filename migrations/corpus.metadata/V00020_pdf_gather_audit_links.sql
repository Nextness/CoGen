-- ==UP==
CREATE TABLE pdf_gather_audit_links (
    event_key      TEXT PRIMARY KEY,
    audit_event_id INTEGER NOT NULL REFERENCES audit_events(id),
    created_at     TEXT NOT NULL
);

-- ==DOWN==
DROP TABLE IF EXISTS pdf_gather_audit_links;
