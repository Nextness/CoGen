-- ==UP==
CREATE TABLE run_term_match_reconciliations (
    pipeline_run_id INTEGER PRIMARY KEY REFERENCES pipeline_runs(id),
    reconciled_at   TEXT NOT NULL DEFAULT (datetime('now'))
);
-- ==DOWN==
DROP TABLE run_term_match_reconciliations;
