-- ==UP==
-- Append-only enforcement for author_occurrences and authorships.
-- These tables record immutable historical authorship data linked to
-- work_revisions.  Once a row exists, it must not be updated or deleted;
-- corrections are expressed by creating a new work_revision with a new
-- authorship set, exactly as work_revisions itself is append-only.

CREATE TRIGGER IF NOT EXISTS author_occurrences_abort_update
BEFORE UPDATE ON author_occurrences
BEGIN
    SELECT RAISE(ABORT, 'author_occurrences is append-only: updates are not allowed');
END;

CREATE TRIGGER IF NOT EXISTS author_occurrences_abort_delete
BEFORE DELETE ON author_occurrences
BEGIN
    SELECT RAISE(ABORT, 'author_occurrences is append-only: deletes are not allowed');
END;

CREATE TRIGGER IF NOT EXISTS authorships_abort_update
BEFORE UPDATE ON authorships
BEGIN
    SELECT RAISE(ABORT, 'authorships is append-only: updates are not allowed');
END;

CREATE TRIGGER IF NOT EXISTS authorships_abort_delete
BEFORE DELETE ON authorships
BEGIN
    SELECT RAISE(ABORT, 'authorships is append-only: deletes are not allowed');
END;

-- ==DOWN==
DROP TRIGGER IF EXISTS authorships_abort_delete;
DROP TRIGGER IF EXISTS authorships_abort_update;
DROP TRIGGER IF EXISTS author_occurrences_abort_delete;
DROP TRIGGER IF EXISTS author_occurrences_abort_update;