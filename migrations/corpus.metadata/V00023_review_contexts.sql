-- ==UP==
CREATE TABLE review_contexts (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_run_id   INTEGER NOT NULL UNIQUE REFERENCES pipeline_runs(id),
    parent_context_id INTEGER REFERENCES review_contexts(id),
    created_at        TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_review_contexts_parent ON review_contexts(parent_context_id);

CREATE TABLE work_review_versions (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    work_id               INTEGER NOT NULL REFERENCES works(id),
    work_revision_id      INTEGER NOT NULL REFERENCES work_revisions(id),
    created_in_context_id INTEGER NOT NULL REFERENCES review_contexts(id),
    parent_version_id     INTEGER REFERENCES work_review_versions(id),
    status                TEXT NOT NULL CHECK (status IN ('not_evaluated', 'in_progress', 'approved', 'not_approved', 'removed')),
    reason                TEXT CHECK (reason IS NULL OR length(reason) <= 32768),
    created_at            TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_work_review_versions_work ON work_review_versions(work_id, id);
CREATE INDEX idx_work_review_versions_parent ON work_review_versions(parent_version_id);
CREATE INDEX idx_work_review_versions_context ON work_review_versions(created_in_context_id, id);

CREATE TABLE work_review_version_substatuses (
    review_version_id INTEGER NOT NULL REFERENCES work_review_versions(id),
    sub_status        TEXT NOT NULL CHECK (sub_status IN ('redacted', 'unrelated', 'out_of_scope', 'duplicate', 'retracted', 'withdrawn', 'superseded', 'predatory_low_quality', 'copyright_licensing', 'not_peer_reviewed')),
    PRIMARY KEY (review_version_id, sub_status)
);

CREATE TABLE review_context_work_heads (
    review_context_id INTEGER NOT NULL REFERENCES review_contexts(id),
    work_id           INTEGER NOT NULL REFERENCES works(id),
    work_revision_id  INTEGER NOT NULL REFERENCES work_revisions(id),
    review_version_id INTEGER REFERENCES work_review_versions(id),
    PRIMARY KEY (review_context_id, work_id),
    UNIQUE (review_context_id, work_revision_id)
);

CREATE INDEX idx_review_context_work_heads_version ON review_context_work_heads(review_version_id);

CREATE TRIGGER review_contexts_abort_update BEFORE UPDATE ON review_contexts BEGIN SELECT RAISE(ABORT, 'review_contexts is append-only: updates are not allowed'); END;
CREATE TRIGGER review_contexts_abort_delete BEFORE DELETE ON review_contexts BEGIN SELECT RAISE(ABORT, 'review_contexts is append-only: deletes are not allowed'); END;
CREATE TRIGGER work_review_versions_abort_update BEFORE UPDATE ON work_review_versions BEGIN SELECT RAISE(ABORT, 'work_review_versions is append-only: updates are not allowed'); END;
CREATE TRIGGER work_review_versions_abort_delete BEFORE DELETE ON work_review_versions BEGIN SELECT RAISE(ABORT, 'work_review_versions is append-only: deletes are not allowed'); END;
CREATE TRIGGER work_review_version_substatuses_abort_update BEFORE UPDATE ON work_review_version_substatuses BEGIN SELECT RAISE(ABORT, 'work_review_version_substatuses is append-only: updates are not allowed'); END;
CREATE TRIGGER work_review_version_substatuses_abort_delete BEFORE DELETE ON work_review_version_substatuses BEGIN SELECT RAISE(ABORT, 'work_review_version_substatuses is append-only: deletes are not allowed'); END;

-- ==DOWN==
DROP TRIGGER work_review_version_substatuses_abort_delete;
DROP TRIGGER work_review_version_substatuses_abort_update;
DROP TRIGGER work_review_versions_abort_delete;
DROP TRIGGER work_review_versions_abort_update;
DROP TRIGGER review_contexts_abort_delete;
DROP TRIGGER review_contexts_abort_update;
DROP INDEX idx_review_context_work_heads_version;
DROP TABLE review_context_work_heads;
DROP TABLE work_review_version_substatuses;
DROP INDEX idx_work_review_versions_context;
DROP INDEX idx_work_review_versions_parent;
DROP INDEX idx_work_review_versions_work;
DROP TABLE work_review_versions;
DROP INDEX idx_review_contexts_parent;
DROP TABLE review_contexts;
