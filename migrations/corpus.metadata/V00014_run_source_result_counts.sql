-- ==UP==
-- Preserve the count declared when source metadata was downloaded alongside
-- the raw record count observed when this run reads the export. Legacy rows
-- remain NULL because their original declared count is not recoverable.
ALTER TABLE run_sources ADD COLUMN expected_result_count INTEGER CHECK (expected_result_count >= 0);
ALTER TABLE run_sources ADD COLUMN observed_result_count INTEGER CHECK (observed_result_count >= 0);
ALTER TABLE run_sources ADD COLUMN result_count_comparison TEXT CHECK (result_count_comparison IN ('match', 'below', 'above'));

-- ==DOWN==
ALTER TABLE run_sources DROP COLUMN result_count_comparison;
ALTER TABLE run_sources DROP COLUMN observed_result_count;
ALTER TABLE run_sources DROP COLUMN expected_result_count;
