-- ==UP==
-- Keep the strong-identity registry usable even when a caller bypasses the
-- repository. Format and checksum validation remain repository-level because
-- SQLite has no project ORCID checksum function.
CREATE TRIGGER IF NOT EXISTS people_abort_blank_orcid_insert
BEFORE INSERT ON people
WHEN NEW.orcid IS NULL OR trim(NEW.orcid) = ''
BEGIN
    SELECT RAISE(ABORT, 'people.orcid must not be null, empty, or whitespace');
END;

CREATE TRIGGER IF NOT EXISTS people_abort_blank_orcid_update
BEFORE UPDATE OF orcid ON people
WHEN NEW.orcid IS NULL OR trim(NEW.orcid) = ''
BEGIN
    SELECT RAISE(ABORT, 'people.orcid must not be null, empty, or whitespace');
END;

-- ==DOWN==
DROP TRIGGER IF EXISTS people_abort_blank_orcid_update;
DROP TRIGGER IF EXISTS people_abort_blank_orcid_insert;
