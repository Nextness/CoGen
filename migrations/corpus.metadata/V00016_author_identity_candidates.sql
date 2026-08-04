-- ==UP==
-- Name-based provider matches are evidence, not identities. A resolution is
-- scoped to the immutable author occurrence observed in one pipeline run;
-- candidates retain the provider response that requires human review.
CREATE TABLE author_identity_resolutions (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_run_id       INTEGER NOT NULL REFERENCES pipeline_runs(id),
    author_occurrence_id  INTEGER NOT NULL REFERENCES author_occurrences(id),
    status                TEXT NOT NULL CHECK (status IN (
        'orcid_is_unclear', 'no_orcid_candidate', 'provider_failed',
        'confirmed', 'rejected'
    )),
    provider              TEXT NOT NULL,
    queried_citation_name TEXT NOT NULL,
    error_message         TEXT,
    resolved_at           TEXT NOT NULL,
    created_at            TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (pipeline_run_id, author_occurrence_id, provider)
);

CREATE INDEX idx_author_identity_resolutions_run ON author_identity_resolutions(pipeline_run_id);
CREATE INDEX idx_author_identity_resolutions_occurrence ON author_identity_resolutions(author_occurrence_id);

CREATE TABLE author_identity_candidates (
    id                      INTEGER PRIMARY KEY AUTOINCREMENT,
    identity_resolution_id  INTEGER NOT NULL REFERENCES author_identity_resolutions(id),
    candidate_orcid         TEXT NOT NULL,
    provider_display_name   TEXT,
    query_url               TEXT NOT NULL,
    payload_artifact_id     INTEGER REFERENCES artifacts(id),
    provider_rank           INTEGER NOT NULL CHECK (provider_rank >= 1),
    created_at              TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (identity_resolution_id, candidate_orcid),
    UNIQUE (identity_resolution_id, provider_rank)
);

CREATE INDEX idx_author_identity_candidates_resolution ON author_identity_candidates(identity_resolution_id);
CREATE INDEX idx_author_identity_candidates_orcid ON author_identity_candidates(candidate_orcid);

-- ==DOWN==
DROP TABLE author_identity_candidates;
DROP TABLE author_identity_resolutions;
