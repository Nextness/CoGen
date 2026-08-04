-- ==UP==
ALTER TABLE pdf_gather_audit_links RENAME TO pdf_audit_links;

-- ==DOWN==
ALTER TABLE pdf_audit_links RENAME TO pdf_gather_audit_links;
