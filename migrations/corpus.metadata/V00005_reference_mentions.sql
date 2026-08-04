-- ==UP==
-- Reference mentions are the immutable citation snapshot for one work revision.
-- Identical external references intentionally remain distinct mentions; order
-- preserves their position in the source record.
CREATE TABLE IF NOT EXISTS reference_mentions (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    work_revision_id  INTEGER NOT NULL REFERENCES work_revisions(id),
    resolved_work_id  INTEGER REFERENCES works(id),
    mention_order     INTEGER NOT NULL,
    raw_reference     TEXT,
    doi               TEXT,
    title             TEXT,
    author            TEXT,
    year              INTEGER,
    source            TEXT,
    created_at        TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (work_revision_id, mention_order)
);

CREATE INDEX IF NOT EXISTS idx_reference_mentions_revision ON reference_mentions(work_revision_id);
CREATE INDEX IF NOT EXISTS idx_reference_mentions_resolved_work ON reference_mentions(resolved_work_id);

CREATE TRIGGER IF NOT EXISTS reference_mentions_abort_update
BEFORE UPDATE ON reference_mentions
BEGIN
    SELECT RAISE(ABORT, 'reference_mentions is append-only: updates are not allowed');
END;

CREATE TRIGGER IF NOT EXISTS reference_mentions_abort_delete
BEFORE DELETE ON reference_mentions
BEGIN
    SELECT RAISE(ABORT, 'reference_mentions is append-only: deletes are not allowed');
END;

-- ==DOWN==
DROP TRIGGER IF EXISTS reference_mentions_abort_delete;
DROP TRIGGER IF EXISTS reference_mentions_abort_update;
DROP TABLE IF EXISTS reference_mentions;
