-- ==UP==
CREATE TABLE pdf_store_binding (
    id                 INTEGER PRIMARY KEY CHECK (id = 1),
    relative_path      TEXT NOT NULL CHECK (length(trim(relative_path)) > 0),
    configured_at      TEXT NOT NULL,
    config_fingerprint TEXT NOT NULL CHECK (length(config_fingerprint) = 64)
);

-- ==DOWN==
DROP TABLE IF EXISTS pdf_store_binding;
