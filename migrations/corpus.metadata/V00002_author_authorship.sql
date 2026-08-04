-- ==UP==
-- People: optional strong global identity for authors.
-- ORCID is the canonical strong identity signal.
-- A person record is created when a valid ORCID is first observed.
CREATE TABLE IF NOT EXISTS people (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    orcid       TEXT UNIQUE,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Author occurrences: observed author data at a point in time.
-- Each occurrence records the citation name, optional name components,
-- and the raw observed ORCID (if any).  An occurrence may optionally link
-- to a global person record when the ORCID is a known strong identity.
-- ORCID-less occurrences with the same name are never merged globally.
CREATE TABLE IF NOT EXISTS author_occurrences (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    person_id       INTEGER REFERENCES people(id),
    citation_name   TEXT NOT NULL,
    first_name      TEXT,
    last_name       TEXT,
    orcid           TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_author_occurrences_person ON author_occurrences(person_id);
CREATE INDEX IF NOT EXISTS idx_author_occurrences_orcid ON author_occurrences(orcid);

-- Authorships: links between immutable work_revisions and author_occurrences.
-- Each authorship preserves author order and optional affiliation for a
-- specific work revision.  Because work_revisions is append-only and
-- immutable, the authorship set for a given revision is also effectively
-- immutable — a later run creates a new revision with its own authorship set.
CREATE TABLE IF NOT EXISTS authorships (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    work_revision_id      INTEGER NOT NULL REFERENCES work_revisions(id),
    author_occurrence_id  INTEGER NOT NULL REFERENCES author_occurrences(id),
    author_order          INTEGER NOT NULL,
    affiliation           TEXT,
    created_at            TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (work_revision_id, author_occurrence_id),
    UNIQUE (work_revision_id, author_order)
);

CREATE INDEX IF NOT EXISTS idx_authorships_revision ON authorships(work_revision_id);
CREATE INDEX IF NOT EXISTS idx_authorships_occurrence ON authorships(author_occurrence_id);

-- ==DOWN==
DROP TABLE IF EXISTS authorships;
DROP TABLE IF EXISTS author_occurrences;
DROP TABLE IF EXISTS people;