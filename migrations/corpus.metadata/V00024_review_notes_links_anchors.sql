-- ==UP==
CREATE TABLE review_notes (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    work_id    INTEGER NOT NULL REFERENCES works(id),
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE review_note_versions (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    note_id               INTEGER NOT NULL REFERENCES review_notes(id),
    parent_version_id     INTEGER REFERENCES review_note_versions(id),
    created_in_context_id INTEGER NOT NULL REFERENCES review_contexts(id),
    state                 TEXT NOT NULL CHECK (state IN ('active', 'deleted')),
    body                  TEXT,
    created_at            TEXT NOT NULL DEFAULT (datetime('now')),
    CHECK ((state = 'active' AND body IS NOT NULL AND length(trim(body)) > 0) OR (state = 'deleted' AND body IS NULL))
);

CREATE INDEX idx_review_note_versions_note ON review_note_versions(note_id, id);
CREATE INDEX idx_review_note_versions_parent ON review_note_versions(parent_version_id);

CREATE TABLE review_context_note_heads (
    review_context_id INTEGER NOT NULL REFERENCES review_contexts(id),
    note_id           INTEGER NOT NULL REFERENCES review_notes(id),
    note_version_id   INTEGER NOT NULL REFERENCES review_note_versions(id),
    PRIMARY KEY (review_context_id, note_id)
);

CREATE INDEX idx_review_context_note_heads_version ON review_context_note_heads(note_version_id);

CREATE TABLE review_note_links (
    note_version_id INTEGER NOT NULL REFERENCES review_note_versions(id),
    ordinal         INTEGER NOT NULL CHECK (ordinal >= 1),
    target_type     TEXT NOT NULL CHECK (target_type IN ('note', 'article', 'pdf_page', 'anchor', 'ext')),
    raw_target      TEXT NOT NULL CHECK (length(raw_target) <= 2048),
    display_text    TEXT CHECK (display_text IS NULL OR length(display_text) <= 1024),
    utf16_position  INTEGER NOT NULL CHECK (utf16_position >= 0),
    utf16_length    INTEGER NOT NULL CHECK (utf16_length > 0),
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (note_version_id, ordinal)
);

CREATE INDEX idx_review_note_links_target ON review_note_links(target_type, raw_target, note_version_id);

CREATE TABLE review_anchors (
    id         TEXT PRIMARY KEY,
    work_id    INTEGER NOT NULL REFERENCES works(id),
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_review_anchors_work ON review_anchors(work_id, id);

CREATE TABLE review_anchor_versions (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    anchor_id             TEXT NOT NULL REFERENCES review_anchors(id),
    parent_version_id     INTEGER REFERENCES review_anchor_versions(id),
    created_in_context_id INTEGER NOT NULL REFERENCES review_contexts(id),
    work_revision_id      INTEGER NOT NULL REFERENCES work_revisions(id),
    pdf_content_hash      TEXT NOT NULL CHECK (length(pdf_content_hash) = 64),
    state                 TEXT NOT NULL CHECK (state IN ('active', 'deleted')),
    page                  INTEGER,
    selected_text         TEXT,
    rectangles_json       TEXT,
    created_at            TEXT NOT NULL DEFAULT (datetime('now')),
    CHECK ((state = 'active' AND page >= 1 AND selected_text IS NOT NULL AND rectangles_json IS NOT NULL) OR (state = 'deleted' AND page IS NULL AND selected_text IS NULL AND rectangles_json IS NULL))
);

CREATE INDEX idx_review_anchor_versions_anchor ON review_anchor_versions(anchor_id, id);
CREATE INDEX idx_review_anchor_versions_parent ON review_anchor_versions(parent_version_id);

CREATE TABLE review_context_anchor_heads (
    review_context_id INTEGER NOT NULL REFERENCES review_contexts(id),
    anchor_id         TEXT NOT NULL REFERENCES review_anchors(id),
    anchor_version_id INTEGER NOT NULL REFERENCES review_anchor_versions(id),
    PRIMARY KEY (review_context_id, anchor_id)
);

CREATE INDEX idx_review_context_anchor_heads_version ON review_context_anchor_heads(anchor_version_id);

CREATE TRIGGER review_notes_abort_update BEFORE UPDATE ON review_notes BEGIN SELECT RAISE(ABORT, 'review_notes is append-only: updates are not allowed'); END;
CREATE TRIGGER review_notes_abort_delete BEFORE DELETE ON review_notes BEGIN SELECT RAISE(ABORT, 'review_notes is append-only: deletes are not allowed'); END;
CREATE TRIGGER review_note_versions_abort_update BEFORE UPDATE ON review_note_versions BEGIN SELECT RAISE(ABORT, 'review_note_versions is append-only: updates are not allowed'); END;
CREATE TRIGGER review_note_versions_abort_delete BEFORE DELETE ON review_note_versions BEGIN SELECT RAISE(ABORT, 'review_note_versions is append-only: deletes are not allowed'); END;
CREATE TRIGGER review_note_links_abort_update BEFORE UPDATE ON review_note_links BEGIN SELECT RAISE(ABORT, 'review_note_links is append-only: updates are not allowed'); END;
CREATE TRIGGER review_note_links_abort_delete BEFORE DELETE ON review_note_links BEGIN SELECT RAISE(ABORT, 'review_note_links is append-only: deletes are not allowed'); END;
CREATE TRIGGER review_anchors_abort_update BEFORE UPDATE ON review_anchors BEGIN SELECT RAISE(ABORT, 'review_anchors is append-only: updates are not allowed'); END;
CREATE TRIGGER review_anchors_abort_delete BEFORE DELETE ON review_anchors BEGIN SELECT RAISE(ABORT, 'review_anchors is append-only: deletes are not allowed'); END;
CREATE TRIGGER review_anchor_versions_abort_update BEFORE UPDATE ON review_anchor_versions BEGIN SELECT RAISE(ABORT, 'review_anchor_versions is append-only: updates are not allowed'); END;
CREATE TRIGGER review_anchor_versions_abort_delete BEFORE DELETE ON review_anchor_versions BEGIN SELECT RAISE(ABORT, 'review_anchor_versions is append-only: deletes are not allowed'); END;

-- ==DOWN==
DROP TRIGGER review_anchor_versions_abort_delete;
DROP TRIGGER review_anchor_versions_abort_update;
DROP TRIGGER review_anchors_abort_delete;
DROP TRIGGER review_anchors_abort_update;
DROP TRIGGER review_note_links_abort_delete;
DROP TRIGGER review_note_links_abort_update;
DROP TRIGGER review_note_versions_abort_delete;
DROP TRIGGER review_note_versions_abort_update;
DROP TRIGGER review_notes_abort_delete;
DROP TRIGGER review_notes_abort_update;
DROP INDEX idx_review_context_anchor_heads_version;
DROP TABLE review_context_anchor_heads;
DROP INDEX idx_review_anchor_versions_parent;
DROP INDEX idx_review_anchor_versions_anchor;
DROP TABLE review_anchor_versions;
DROP INDEX idx_review_anchors_work;
DROP TABLE review_anchors;
DROP INDEX idx_review_note_links_target;
DROP TABLE review_note_links;
DROP INDEX idx_review_context_note_heads_version;
DROP TABLE review_context_note_heads;
DROP INDEX idx_review_note_versions_parent;
DROP INDEX idx_review_note_versions_note;
DROP TABLE review_note_versions;
DROP TABLE review_notes;
