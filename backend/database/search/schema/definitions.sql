-- sqlc-only declarations for the search table family.
-- These statements exist solely to compile the family's generated queries.
-- They are never executed by production and never participate in runtime
-- migrations or SOMETHING database versioning. Column shapes follow the
-- production metadata migrations V00001, V00006, V00007, and V00013.
CREATE TABLE searches (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    search_id TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE search_revisions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    search_id INTEGER NOT NULL,
    revision_label TEXT NOT NULL,
    config_artifact_hash TEXT NOT NULL,
    resolved_manifest_hash TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT,
    UNIQUE (search_id, revision_label)
);

CREATE TABLE execution_plans (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    search_revision_id INTEGER NOT NULL,
    execution_fingerprint TEXT NOT NULL,
    resolved_manifest_hash TEXT NOT NULL,
    input_manifest_hash TEXT NOT NULL DEFAULT '',
    enrichment_enabled INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (search_revision_id, execution_fingerprint)
);

-- External run table declared only so hierarchy totals and search summary
-- counts can read planned and completed attempts. The run family owns all run
-- mutations and lifecycle.
CREATE TABLE pipeline_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    status TEXT NOT NULL,
    execution_plan_id INTEGER
);
