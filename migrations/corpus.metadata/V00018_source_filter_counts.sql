-- ==UP==
-- Preserve per-source filter stage data for each pipeline run.  Each row
-- stores the ordered filter stages (cumulative from NO_FILTER to the most
-- restrictive set) and their article counts as a JSON array.
-- filter_data format:
--   [{"filters": ["NO_FILTER"], "count": <int>},
--    {"filters": ["NO_FILTER","RANGE_10_YEARS",...], "count": <int>}, ...]
CREATE TABLE IF NOT EXISTS source_filter_counts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_run_id INTEGER NOT NULL REFERENCES pipeline_runs(id),
    source_name TEXT NOT NULL,
    filter_data TEXT NOT NULL DEFAULT '[]',
    UNIQUE(pipeline_run_id, source_name)
);

-- ==DOWN==
DROP TABLE IF EXISTS source_filter_counts;