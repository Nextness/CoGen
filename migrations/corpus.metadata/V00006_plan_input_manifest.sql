-- ==UP==
-- Links each new execution plan to the content-addressed input manifest that
-- supplied its source-file hashes. Existing development plans predate this
-- provenance link and retain the empty default.
ALTER TABLE execution_plans ADD COLUMN input_manifest_hash TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_execution_plans_input_manifest_hash ON execution_plans(input_manifest_hash);
-- ==DOWN==
