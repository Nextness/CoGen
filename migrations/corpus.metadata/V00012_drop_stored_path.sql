-- ==UP==
-- Remove the stored_path column from the artifacts table. Artifact payload
-- data is now stored inline in the artifact_blobs table; the filesystem path
-- is no longer referenced by the application.
ALTER TABLE artifacts DROP COLUMN stored_path;
-- ==DOWN==
ALTER TABLE artifacts ADD COLUMN stored_path TEXT;