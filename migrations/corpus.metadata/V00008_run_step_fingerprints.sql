-- ==UP==
ALTER TABLE run_steps ADD COLUMN input_fingerprint TEXT NOT NULL DEFAULT '';
ALTER TABLE run_steps ADD COLUMN output_fingerprint TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_run_steps_input_fingerprint ON run_steps(input_fingerprint);
-- ==DOWN==
