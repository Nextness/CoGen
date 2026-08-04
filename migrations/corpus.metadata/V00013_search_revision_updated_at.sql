-- ==UP==
-- Add updated_at column to search_revisions so we can track when a revision's
-- config or manifest hashes were last updated (e.g. when the workspace config
-- file changed but the revision label stayed the same).
ALTER TABLE search_revisions ADD COLUMN updated_at TEXT;
UPDATE search_revisions SET updated_at = created_at WHERE updated_at IS NULL;

-- ==DOWN==
-- SQLite does not support DROP COLUMN for columns added with ALTER TABLE
-- in older versions, but as of 3.35.0 it does. We use it here since the
-- baseline already requires a recent SQLite.
ALTER TABLE search_revisions DROP COLUMN updated_at;