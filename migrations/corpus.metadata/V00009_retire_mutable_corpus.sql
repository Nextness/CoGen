-- ==UP==
-- Phase 3.6 removes the transition-only mutable corpus. Workspace runs retain
-- source records, immutable revisions, authorships, reference mentions,
-- artifacts, metrics, and audit events instead.
DROP TABLE IF EXISTS article_authors;
DROP TABLE IF EXISTS article_references;
DROP TABLE IF EXISTS enrichment_log;
DROP TABLE IF EXISTS authors;
DROP TABLE IF EXISTS articles;
-- ==DOWN==
