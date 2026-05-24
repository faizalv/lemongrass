CREATE TABLE IF NOT EXISTS lg_file_hashes (
  project_id  BIGINT NOT NULL REFERENCES lg_projects(id) ON DELETE CASCADE,
  file_path   TEXT NOT NULL,
  hash        TEXT NOT NULL,
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (project_id, file_path)
);

ALTER TABLE lg_projects
  ADD COLUMN IF NOT EXISTS sync_interval TEXT NOT NULL DEFAULT 'off';
