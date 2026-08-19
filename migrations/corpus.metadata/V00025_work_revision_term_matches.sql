-- ==UP==
CREATE TABLE run_search_terms (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_run_id INTEGER NOT NULL REFERENCES pipeline_runs(id),
    source_name     TEXT NOT NULL,
    term            TEXT NOT NULL,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (pipeline_run_id, source_name, term)
);

CREATE TABLE work_revision_term_matches (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_run_id  INTEGER NOT NULL REFERENCES pipeline_runs(id),
    work_revision_id INTEGER NOT NULL REFERENCES work_revisions(id),
    field            TEXT NOT NULL CHECK (field IN ('title', 'abstract', 'keywords', 'keywords_plus')),
    term             TEXT NOT NULL,
    created_at       TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (pipeline_run_id, work_revision_id, field, term)
);

CREATE INDEX idx_work_revision_term_matches_run_field_term
    ON work_revision_term_matches (pipeline_run_id, field, term);
-- ==DOWN==
DROP INDEX idx_work_revision_term_matches_run_field_term;
DROP TABLE work_revision_term_matches;
DROP TABLE run_search_terms;