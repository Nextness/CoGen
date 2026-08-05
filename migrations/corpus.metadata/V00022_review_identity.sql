-- ==UP==
CREATE TABLE pipeline_run_reviewers (
    pipeline_run_id INTEGER PRIMARY KEY NOT NULL REFERENCES pipeline_runs(id),
    username        TEXT NOT NULL DEFAULT '' CHECK (length(username) <= 200),
    email           TEXT NOT NULL DEFAULT '' CHECK (length(email) <= 320),
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

INSERT INTO pipeline_run_reviewers (pipeline_run_id, username, email)
SELECT id, '', '' FROM pipeline_runs;

CREATE TABLE review_settings (
    id         INTEGER PRIMARY KEY CHECK (id = 1),
    corpus_id  TEXT NOT NULL UNIQUE CHECK (length(corpus_id) = 32),
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

INSERT INTO review_settings (id, corpus_id) VALUES (1, lower(hex(randomblob(16))));

-- ==DOWN==
DROP TABLE review_settings;
DROP TABLE pipeline_run_reviewers;
