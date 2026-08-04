-- ==UP==
-- Makes the declared enrichment policy directly inspectable on execution plans.
ALTER TABLE execution_plans ADD COLUMN enrichment_enabled INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_execution_plans_enrichment_enabled ON execution_plans(enrichment_enabled);
-- ==DOWN==
