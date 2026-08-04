-- ==UP==
-- Track when source metadata was originally downloaded.  The date is declared
-- in the workspace config and preserved as provenance.  Legacy rows are NULL.
ALTER TABLE run_sources ADD COLUMN export_date TEXT;

-- ==DOWN==
ALTER TABLE run_sources DROP COLUMN export_date;